// Package http implements the REST layer of the Mawarid backend.
//
// Contract highlights (must match api/openapi.yaml and the frontend client
// lib/api/client.ts):
//   - success responses are plain JSON objects (no wrapper);
//   - failures use the {error:{code,message,message_ar,request_id,details}}
//     envelope with an x-request-id header on every response;
//   - Arabic messages satisfy SRS NFR-USA-02.
package http

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/config"
	"mawarid/backend/internal/service"
)

// Server carries the HTTP dependencies.
type Server struct {
	cfg   *config.Config
	svc   *service.Service
	admin *service.AdminService
	prov  *service.ProviderService
	log   *slog.Logger
	mux   *http.ServeMux
}

// New builds the Server and registers every route (see router.go).
func New(cfg *config.Config, svc *service.Service, admin *service.AdminService, prov *service.ProviderService, log *slog.Logger) *Server {
	s := &Server{cfg: cfg, svc: svc, admin: admin, prov: prov, log: log, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler returns the fully wrapped HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

// ---------------------------------------------------------------------------
// Context helpers
// ---------------------------------------------------------------------------

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxUserID
	ctxUserRole
)

// withRequestIDValue stores the correlation id in the context.
func withRequestIDValue(ctx stdcontext.Context, id string) stdcontext.Context {
	return stdcontext.WithValue(ctx, ctxRequestID, id)
}

// RequestID returns the correlation id of a request ("" when absent).
func RequestID(ctx stdcontext.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	return ""
}

// CurrentUserID returns the authenticated user id (0 for anonymous).
func CurrentUserID(ctx stdcontext.Context) uint {
	if v, ok := ctx.Value(ctxUserID).(uint); ok {
		return v
	}
	return 0
}

// CurrentUserRole returns the authenticated role ("" for anonymous).
func CurrentUserRole(ctx stdcontext.Context) string {
	if v, ok := ctx.Value(ctxUserRole).(string); ok {
		return v
	}
	return ""
}

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

// writeJSON writes a success payload with the request id header.
func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	if id := RequestID(r.Context()); id != "" {
		w.Header().Set("X-Request-Id", id)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// writeError renders an error through the standard envelope. Unexpected
// errors are logged server-side (full detail) and returned as a generic 500 so
// internals never leak to clients.
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apierr.Error
	if errors.As(err, &ae) {
		s.writeJSON(w, r, ae.Status, map[string]any{"error": ae.Envelope(RequestID(r.Context()))})
		return
	}
	s.log.ErrorContext(r.Context(), "unhandled error",
		"request_id", RequestID(r.Context()), "err", err)
	gen := apierr.Internal("Something went wrong. Please try again.",
		"حدث خطأ غير متوقع. يرجى المحاولة لاحقاً.")
	s.writeJSON(w, r, gen.Status, map[string]any{"error": gen.Envelope(RequestID(r.Context()))})
}

// decodeJSON decodes a single JSON object from the request body (max 1 MB).
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	return decodeJSONBody(w, r, dst, false)
}

// decodeJSONOptional is decodeJSON for endpoints whose body is optional
// (e.g. logout): an empty body is accepted and leaves dst zero-valued, while
// malformed JSON, trailing content and oversized bodies are still rejected.
func decodeJSONOptional(w http.ResponseWriter, r *http.Request, dst any) error {
	return decodeJSONBody(w, r, dst, true)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any, allowEmpty bool) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apierr.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE",
				"Request body is too large.", "حجم الطلب كبير جداً.")
		}
		if errors.Is(err, io.EOF) {
			if allowEmpty {
				return nil
			}
			return apierr.BadRequest("VALIDATION_ERROR", "A JSON request body is required.", "جسم الطلب JSON مطلوب.")
		}
		return apierr.BadRequest("INVALID_JSON", "Malformed JSON request body.", "صيغة JSON غير صالحة.")
	}
	// Reject trailing content after the first object.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return apierr.BadRequest("INVALID_JSON", "Request body must contain a single JSON object.",
			"يجب أن يحتوي جسم الطلب على كائن JSON واحد.")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// pagination reads limit/offset with sane caps.
func pagination(r *http.Request) (limit, offset int) {
	limit = queryInt(r, "limit", 20)
	offset = queryInt(r, "offset", 0)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func queryInt(r *http.Request, key string, def int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// pathID parses an {id} path parameter as uint.
func pathID(r *http.Request, name string) (uint, error) {
	raw := r.PathValue(name)
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		return 0, apierr.ErrValidation([]apierr.FieldViolation{{
			Field: name, Message: "Invalid identifier. / المعرّف غير صالح.",
		}})
	}
	return uint(n), nil
}
