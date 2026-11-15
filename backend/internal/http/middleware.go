package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/models"
	"mawarid/backend/internal/service"
)

// ---------------------------------------------------------------------------
// Middleware chain (outermost first):
//   CORS → request id → panic recovery → access logging → [auth] → handler
// ---------------------------------------------------------------------------

// wrap applies the universal middleware chain to every route.
func (s *Server) wrap(h http.Handler) http.Handler {
	return s.cors(s.requestID(s.recovery(s.accessLog(h))))
}

// ---------------------------------------------------------------------------
// Request id (correlation) — echoed as x-request-id on every response.
// ---------------------------------------------------------------------------

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if id == "" || len(id) > 64 || !isSafeID(id) {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(withRequestIDValue(r.Context(), id)))
	})
}

func isSafeID(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return len(s) > 0
}

// ---------------------------------------------------------------------------
// CORS — only configured origins; credentials are never used (Bearer tokens).
// ---------------------------------------------------------------------------

func (s *Server) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range s.cfg.HTTP.CORSOrigins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[strings.TrimRight(origin, "/")] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			if origin != "" && !allowed[strings.TrimRight(origin, "/")] {
				s.writeJSON(w, r, http.StatusForbidden, map[string]any{"error": apierr.ErrForbiddenRole().Envelope(RequestID(r.Context()))})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Panic recovery — never leaks stack traces to clients.
// ---------------------------------------------------------------------------

func (s *Server) recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.ErrorContext(r.Context(), "panic recovered",
					"request_id", RequestID(r.Context()),
					"panic", fmt.Sprintf("%v", rec),
					"stack", string(debug.Stack()))
				gen := apierr.Internal("Something went wrong. Please try again.",
					"حدث خطأ غير متوقع. يرجى المحاولة لاحقاً.")
				s.writeJSON(w, r, gen.Status, map[string]any{"error": gen.Envelope(RequestID(r.Context()))})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Access logging — method, path (query stripped), status, duration.
// Request/response bodies are NEVER logged (NFR-SEC-02: no PII in logs).
// ---------------------------------------------------------------------------

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.InfoContext(r.Context(), "http request",
			"request_id", RequestID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path, // query stripped on purpose
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// ---------------------------------------------------------------------------
// Authentication (JWT bearer) + account state gate + RBAC
// ---------------------------------------------------------------------------

// authenticate validates the access token and loads the caller's current
// account state from the database (so suspensions apply immediately, before
// the short-lived access token expires).
//
// Gate rules:
//   - suspended / inactive  → 403 ACCOUNT_SUSPENDED (immediate lockout)
//   - pending / rejected / active → allowed (account endpoints must remain
//     reachable so the client can read status, upload documents, log out).
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if header == "" {
			s.writeError(w, r, apierr.ErrUnauthorizedNoToken())
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			s.writeError(w, r, apierr.ErrUnauthorizedNoToken())
			return
		}
		claims, err := s.svc.JWT.VerifyAccessToken(strings.TrimSpace(parts[1]))
		if err != nil {
			s.writeError(w, r, apierr.ErrUnauthorizedNoToken())
			return
		}

		var user models.User
		if err := s.svc.DB.WithContext(r.Context()).Preload("Role").
			First(&user, claims.UserID).Error; err != nil {
			s.writeError(w, r, apierr.ErrUnauthorizedNoToken())
			return
		}
		if !user.IsActive || user.Status == models.AccountStatusSuspended {
			s.writeError(w, r, apierr.ErrAccountSuspended())
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, user.ID)
		ctx = context.WithValue(ctx, ctxUserRole, user.Role.Name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireRole enforces RBAC for the wrapped handler.
func (s *Server) requireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := CurrentUserRole(r.Context())
			for _, want := range roles {
				if role == want {
					next.ServeHTTP(w, r)
					return
				}
			}
			s.writeError(w, r, apierr.ErrForbiddenRole())
		})
	}
}

// ---------------------------------------------------------------------------
// Session metadata (IP / user agent) for sessions, login attempts, audit.
// ---------------------------------------------------------------------------

func sessionMeta(r *http.Request) service.SessionMeta {
	return service.SessionMeta{
		UserAgent: truncate(r.UserAgent(), 255),
		IP:        clientIP(r),
	}
}

// clientIP prefers the first X-Forwarded-For entry (behind the dev proxy) and
// falls back to the socket address.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return truncate(first, 45)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return truncate(r.RemoteAddr, 45)
	}
	return host
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
