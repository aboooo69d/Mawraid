package middleware_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

// TestErrorHandlerMiddleware tests global error translation and response envelope.
func TestErrorHandlerMiddleware(t *testing.T) {
	t.Run("Pass-through when no error occurs", func(t *testing.T) {
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
	})

	t.Run("Handle AppError with Custom Status and Code", func(t *testing.T) {
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
	})

	t.Run("Fallback Unhandled Error to 500 Internal Server Error", func(t *testing.T) {
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
	})
}

