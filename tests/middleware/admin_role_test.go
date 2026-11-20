package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

// TestRequireRoleAndRequireAdmin tests RBAC authorization filters.
func TestRequireRoleAndRequireAdmin(t *testing.T) {
	setupApp := func(role string) *gin.Engine {
		r := gin.New()
		// Mock authentication layer setting context
		r.Use(func(c *gin.Context) {
			if role != "" {
				c.Set("user_id", "test-user-id")
				c.Set("role", role)
			}
			c.Next()
		})
		return r
	}

	t.Run("Reject Request When No Role in Context (401 Unauthorized)", func(t *testing.T) {
		r := setupApp("")
		r.GET("/admin/dashboard", middleware.RequireAdmin(), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("Reject Non-Admin Role (Returns 403 Forbidden)", func(t *testing.T) {
		r := setupApp("consumer") // consumer role
		r.GET("/admin/dashboard", middleware.RequireAdmin(), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", w.Code)
		}

		var resp respons.Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Error == nil || resp.Error.Code != errs_code.ErrForbidden {
			t.Fatalf("expected error code %d, got %v", errs_code.ErrForbidden, resp.Error)
		}
	})

	t.Run("Allow Access for User with Admin Role", func(t *testing.T) {
		r := setupApp("admin")
		r.GET("/admin/dashboard", middleware.RequireAdmin(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"access": "granted"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})
}
