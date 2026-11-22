package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/config"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/logger"
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

	setupLogCapture := func(t *testing.T) *bytes.Buffer {
		t.Helper()
		var buf bytes.Buffer
		prev := logger.Log
		logger.Log = slog.New(slog.NewJSONHandler(&buf, nil))
		t.Cleanup(func() {
			logger.Log = prev
		})
		return &buf
	}

	decodeLogEntry := func(t *testing.T, buf *bytes.Buffer) map[string]any {
		t.Helper()
		var entry map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
			t.Fatalf("failed to decode log output %q: %v", buf.String(), err)
		}
		return entry
	}

	t.Run("Reject Request with Missing Header (Returns 403 and No Panic)", func(t *testing.T) {
		logBuf := setupLogCapture(t)
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

		entry := decodeLogEntry(t, logBuf)
		if entry["reason"] != "missing_token" {
			t.Fatalf("expected log reason %q, got %v", "missing_token", entry["reason"])
		}
	})

	t.Run("Reject Request with Token Exceeding Max Length", func(t *testing.T) {
		logBuf := setupLogCapture(t)
		r := setupRouter(cfg)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)

		hugeToken := strings.Repeat("A", 2049)
		req.Header.Set("cf-turnstile-response", hugeToken)

		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
		}

		entry := decodeLogEntry(t, logBuf)
		if entry["reason"] != "token_too_long" {
			t.Fatalf("expected log reason %q, got %v", "token_too_long", entry["reason"])
		}
		if entry["token_length"] != float64(2049) {
			t.Fatalf("expected token_length 2049, got %v", entry["token_length"])
		}
	})

	t.Run("Reject Request When Configuration is Incomplete", func(t *testing.T) {
		logBuf := setupLogCapture(t)
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

		entry := decodeLogEntry(t, logBuf)
		if entry["reason"] != "invalid_configuration" {
			t.Fatalf("expected log reason %q, got %v", "invalid_configuration", entry["reason"])
		}
		if entry["missing_secret_key"] != true {
			t.Fatalf("expected missing_secret_key=true, got %v", entry["missing_secret_key"])
		}
		if entry["hostnames_count"] != float64(0) {
			t.Fatalf("expected hostnames_count=0, got %v", entry["hostnames_count"])
		}
	})
}

