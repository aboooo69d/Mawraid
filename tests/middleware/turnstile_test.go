package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/config"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestTurnstileMiddleware tests Cloudflare Turnstile token verification scenarios.
func TestTurnstileMiddleware(t *testing.T) {
	cfg := &config.Config{
		Turnstile: config.TurnstileConfig{
			SecretKey: "test-secret-key",
			Hostnames: []string{"example.com", "localhost"},
		},
	}

	setupRouter := func(c *config.Config) *gin.Engine {
		r := gin.New()
		r.Use(middleware.CfTurnstileHanduler(c))
		r.GET("/protected", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		return r
	}

	t.Run("Reject Request with Missing Header (Returns 403 and No Panic)", func(t *testing.T) {
		r := setupRouter(cfg)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)

		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
		}

		var resp respons.Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Success != false {
			t.Fatal("expected success=false")
		}

		if resp.Error == nil || resp.Error.Code != errs_code.ErrTurnstileToken {
			t.Fatalf("expected error code %d, got %v", errs_code.ErrTurnstileToken, resp.Error)
		}
	})

	t.Run("Reject Request with Token Exceeding Max Length", func(t *testing.T) {
		r := setupRouter(cfg)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)

		hugeToken := strings.Repeat("A", 2049)
		req.Header.Set("cf-turnstile-response", hugeToken)

		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
		}
	})

	t.Run("Reject Request When Configuration is Incomplete", func(t *testing.T) {
		emptyCfg := &config.Config{
			Turnstile: config.TurnstileConfig{
				SecretKey: "", // missing secret key
				Hostnames: []string{},
			},
		}

		r := setupRouter(emptyCfg)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("cf-turnstile-response", "any-token")

		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
		}
	})
}

