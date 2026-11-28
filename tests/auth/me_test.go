package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/najm-x/nova/config"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/modules/auth/dto"
	"github.com/najm-x/nova/modules/auth/handler"
	"github.com/najm-x/nova/modules/auth/router"
	"github.com/najm-x/nova/modules/auth/service"
)

func TestMeServiceScenarios(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	roleRepo := newMockRoleRepo()
	sms := newMockSmsProvider()

	authSvc := service.NewAuthService(userRepo, roleRepo, nil, nil, sms)

	consumerID := uuid.New()
	consumerUser := &entities.User{
		ID:         consumerID,
		Phone:      "771234567",
		RoleID:     1,
		Role:       entities.Role{ID: 1, Name: "consumer"},
		IsVerified: true,
		Status:     entities.StatusActive,
		Consumer: &entities.Consumer{
			UserID:   consumerID,
			Name:     "Test Consumer",
			IdNumber: "123456789",
			IsHome:   true,
		},
	}
	_ = userRepo.CreateUser(ctx, consumerUser)

	providerID := uuid.New()
	providerUser := &entities.User{
		ID:         providerID,
		Phone:      "772345678",
		RoleID:     2,
		Role:       entities.Role{ID: 2, Name: "provider"},
		IsVerified: true,
		Status:     entities.StatusActive,
		Provider: &entities.Provider{
			UserID:         providerID,
			Name:           "Test Provider",
			Description:    "Water supplier",
			ProviderTypeID: 1,
			ProviderType:   entities.ProviderType{Name: "Water", Slug: "water"},
		},
	}
	_ = userRepo.CreateUser(ctx, providerUser)

	t.Run("Fail When UserID Is Empty Or Invalid", func(t *testing.T) {
		if _, err := authSvc.Me(ctx, ""); err != dto.ErrUnauthorized {
			t.Fatalf("expected ErrUnauthorized for empty id, got %v", err)
		}
		if _, err := authSvc.Me(ctx, "not-a-uuid"); err != dto.ErrUnauthorized {
			t.Fatalf("expected ErrUnauthorized for invalid uuid, got %v", err)
		}
	})

	t.Run("Fail When User Does Not Exist", func(t *testing.T) {
		_, err := authSvc.Me(ctx, uuid.New().String())
		if err != dto.ErrUserNotFound {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("Success For Consumer User", func(t *testing.T) {
		res, err := authSvc.Me(ctx, consumerID.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ID != consumerID.String() || res.Phone != "771234567" || res.RoleName != "consumer" {
			t.Fatalf("unexpected base response: %+v", res)
		}
		if res.Consumer == nil || res.Consumer.Name != "Test Consumer" {
			t.Fatalf("expected consumer details, got %+v", res.Consumer)
		}
		if res.Provider != nil {
			t.Fatalf("expected nil provider for consumer user")
		}
	})

	t.Run("Success For Provider User", func(t *testing.T) {
		res, err := authSvc.Me(ctx, providerID.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ID != providerID.String() || res.RoleName != "provider" {
			t.Fatalf("unexpected base response: %+v", res)
		}
		if res.Provider == nil || res.Provider.Name != "Test Provider" || res.Provider.ProviderType.Slug != "water" {
			t.Fatalf("expected provider details, got %+v", res.Provider)
		}
		if res.Consumer != nil {
			t.Fatalf("expected nil consumer for provider user")
		}
	})
}

func TestAuthMeEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	userRepo := newMockUserRepo()
	roleRepo := newMockRoleRepo()
	sms := newMockSmsProvider()

	userID := uuid.New()
	_ = userRepo.CreateUser(ctx, &entities.User{
		ID:         userID,
		Phone:      "771112233",
		RoleID:     1,
		Role:       entities.Role{ID: 1, Name: "consumer"},
		IsVerified: true,
		Status:     entities.StatusActive,
		Consumer: &entities.Consumer{
			UserID:   userID,
			Name:     "Session User",
			IdNumber: "987654321",
		},
	})

	authSvc := service.NewAuthService(userRepo, roleRepo, nil, nil, sms)
	authH := handler.NewAuthHandler(authSvc, nil, nil)

	r := gin.New()
	store := cookie.NewStore([]byte("test-secret"))
	r.Use(sessions.Sessions("session", store))
	r.Use(middleware.ErrorHandler())

	// Helper route to populate a session cookie for testing.
	r.POST("/test-session", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("user_id", userID.String())
		s.Set("role", "consumer")
		_ = s.Save()
		c.Status(http.StatusOK)
	})

	v1 := r.Group("/api/v1")
	router.RegisterAuthRoutes(v1, authH, &config.Config{})

	t.Run("Unauthorized Without Session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Returns Authenticated User With Valid Session", func(t *testing.T) {
		loginReq := httptest.NewRequest(http.MethodPost, "/test-session", nil)
		loginW := httptest.NewRecorder()
		r.ServeHTTP(loginW, loginReq)

		cookies := loginW.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatalf("expected session cookie to be set")
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var body struct {
			Success bool `json:"success"`
			Data    struct {
				ID       string `json:"id"`
				Phone    string `json:"phone"`
				RoleName string `json:"role_name"`
				Consumer *struct {
					Name string `json:"name"`
				} `json:"consumer"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !body.Success || body.Data.ID != userID.String() || body.Data.Phone != "771112233" || body.Data.Consumer == nil || body.Data.Consumer.Name != "Session User" {
			t.Fatalf("unexpected response payload: %s", w.Body.String())
		}
	})
}
