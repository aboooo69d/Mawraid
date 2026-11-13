package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

// TestSessionMiddleware tests session extraction and unauthorized rejection.
func TestSessionMiddleware(t *testing.T) {
	cookieStore := cookie.NewStore([]byte("secret-session-key-32-bytes!!12"))

	setupApp := func() *gin.Engine {
		r := gin.New()
		r.Use(sessions.Sessions("mysession", cookieStore))
		return r
	}

	t.Run("Reject Request Without Active Session (Returns 401 Unauthorized)", func(t *testing.T) {
		r := setupApp()
		r.GET("/api/profile", middleware.SessionHandler(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "authorized"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/profile", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}

		var resp respons.Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Error == nil || resp.Error.Code != errs_code.ErrUnauthorized {
			t.Fatalf("expected error code %d, got %v", errs_code.ErrUnauthorized, resp.Error)
		}
	})

	t.Run("Allow Request With Valid Session and Inject Context Values", func(t *testing.T) {
		r := setupApp()

		// Route to simulate user login by setting session
		r.POST("/login-test", func(c *gin.Context) {
			session := sessions.Default(c)
			session.Set("user_id", "user-uuid-1234")
			session.Set("role", "consumer")
			_ = session.Save()
			c.Status(http.StatusOK)
		})

		// Protected route requiring active session
		r.GET("/protected-test", middleware.SessionHandler(), func(c *gin.Context) {
			userId := c.GetString("user_id")
			role := c.GetString("role")
			c.JSON(http.StatusOK, gin.H{
				"user_id": userId,
				"role":    role,
			})
		})

		// 1. Log in and get session cookie
		wLogin := httptest.NewRecorder()
		reqLogin, _ := http.NewRequest(http.MethodPost, "/login-test", nil)
		r.ServeHTTP(wLogin, reqLogin)

		cookieHeader := wLogin.Header().Get("Set-Cookie")
		if cookieHeader == "" {
			t.Fatal("expected session cookie to be set")
		}

		// 2. Make authenticated request using the session cookie
		wAuth := httptest.NewRecorder()
		reqAuth, _ := http.NewRequest(http.MethodGet, "/protected-test", nil)
		reqAuth.Header.Set("Cookie", cookieHeader)
		r.ServeHTTP(wAuth, reqAuth)

		if wAuth.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", wAuth.Code)
		}

		var data map[string]string
		if err := json.Unmarshal(wAuth.Body.Bytes(), &data); err != nil {
			t.Fatalf("failed to parse auth response: %v", err)
		}

		if data["user_id"] != "user-uuid-1234" || data["role"] != "consumer" {
			t.Fatalf("context values mismatch: got %v", data)
		}
	})
}

