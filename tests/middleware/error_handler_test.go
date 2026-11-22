package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/logger"
	"github.com/najm-x/nova/pkg/respons"
)

// TestErrorHandlerMiddleware tests global error translation and response envelope.
func TestErrorHandlerMiddleware(t *testing.T) {
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

	t.Run("Pass-through when no error occurs", func(t *testing.T) {
		logBuf := setupLogCapture(t)
		r := gin.New()
		r.Use(middleware.ErrorHandler())
		r.GET("/ok", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/ok", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if logBuf.Len() != 0 {
			t.Fatalf("expected no log output, got %q", logBuf.String())
		}
	})

	t.Run("Handle AppError with Custom Status and Code", func(t *testing.T) {
		logBuf := setupLogCapture(t)
		r := gin.New()
		r.Use(middleware.ErrorHandler())
		r.GET("/custom-error", func(c *gin.Context) {
			_ = c.Error(&respons.AppError{
				Status:  http.StatusConflict,
				Code:    errs_code.ErrPhoneAlreadyExists,
				Message: "phone number already in use",
			})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/custom-error", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, w.Code)
		}

		var resp respons.Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.Success != false {
			t.Fatal("expected success to be false")
		}

		if resp.Error == nil || resp.Error.Code != errs_code.ErrPhoneAlreadyExists {
			t.Fatalf("expected error code %d, got %v", errs_code.ErrPhoneAlreadyExists, resp.Error)
		}

		entry := decodeLogEntry(t, logBuf)
		if entry["level"] != "WARN" {
			t.Fatalf("expected log level %q, got %v", "WARN", entry["level"])
		}
		if entry["middleware"] != "error_handler" {
			t.Fatalf("expected middleware %q, got %v", "error_handler", entry["middleware"])
		}
		if entry["status"] != float64(http.StatusConflict) {
			t.Fatalf("expected status %d, got %v", http.StatusConflict, entry["status"])
		}
		if entry["code"] != float64(errs_code.ErrPhoneAlreadyExists) {
			t.Fatalf("expected code %d, got %v", errs_code.ErrPhoneAlreadyExists, entry["code"])
		}
		if entry["error"] != "phone number already in use" {
			t.Fatalf("expected error %q, got %v", "phone number already in use", entry["error"])
		}
	})

	t.Run("Handle 5xx AppError Logs at Error Level", func(t *testing.T) {
		logBuf := setupLogCapture(t)
		r := gin.New()
		r.Use(middleware.ErrorHandler())
		r.GET("/app-500", func(c *gin.Context) {
			_ = c.Error(&respons.AppError{
				Status:  http.StatusInternalServerError,
				Code:    errs_code.ErrInternalServer,
				Message: "storage unavailable",
			})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/app-500", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}

		entry := decodeLogEntry(t, logBuf)
		if entry["level"] != "ERROR" {
			t.Fatalf("expected log level %q, got %v", "ERROR", entry["level"])
		}
		if entry["status"] != float64(http.StatusInternalServerError) {
			t.Fatalf("expected status %d, got %v", http.StatusInternalServerError, entry["status"])
		}
	})

	t.Run("Fallback Unhandled Error to 500 Internal Server Error", func(t *testing.T) {
		logBuf := setupLogCapture(t)
		r := gin.New()
		r.Use(middleware.ErrorHandler())
		r.GET("/panic-or-raw-error", func(c *gin.Context) {
			_ = c.Error(errors.New("database connection broken"))
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/panic-or-raw-error", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", w.Code)
		}

		var resp respons.Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.Error == nil || resp.Error.Code != errs_code.ErrInternalServer {
			t.Fatalf("expected error code %d, got %v", errs_code.ErrInternalServer, resp.Error)
		}
		if resp.Error.Message != "internal server error" {
			t.Fatalf("expected 'internal server error', got %q", resp.Error.Message)
		}

		entry := decodeLogEntry(t, logBuf)
		if entry["level"] != "ERROR" {
			t.Fatalf("expected log level %q, got %v", "ERROR", entry["level"])
		}
		if entry["middleware"] != "error_handler" {
			t.Fatalf("expected middleware %q, got %v", "error_handler", entry["middleware"])
		}
		if entry["status"] != float64(http.StatusInternalServerError) {
			t.Fatalf("expected status %d, got %v", http.StatusInternalServerError, entry["status"])
		}
		if entry["code"] != float64(errs_code.ErrInternalServer) {
			t.Fatalf("expected code %d, got %v", errs_code.ErrInternalServer, entry["code"])
		}
		if entry["error"] != "database connection broken" {
			t.Fatalf("expected error %q, got %v", "database connection broken", entry["error"])
		}
	})
}

