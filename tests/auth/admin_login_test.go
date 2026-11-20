package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/auth/dto"
	"github.com/najm-x/nova/modules/auth/service"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	valueHashing "github.com/najm-x/nova/pkg/value_hashing"
)

// TestAdminLoginAndGetMe validates admin-only login enforcement and session profile retrieval.
func TestAdminLoginAndGetMe(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	roles := newMockRoleRepo()
	sms := newMockSmsProvider()

	authService := service.NewAuthService(userRepo, roles, nil, nil, sms)

	rawPassword := "AdminSecret123!"
	hashedPassword, _ := valueHashing.HashValue(rawPassword)

	// 1. Create an Admin User
	adminUser := &entities.User{
		ID:         uuid.New(),
		Phone:      "777111222",
		Password:   hashedPassword,
		RoleID:     3,
		Role:       entities.Role{ID: 3, Name: roleRepo.RoleAdmin, DisplayName: "Admin"},
		IsVerified: true,
		Status:     entities.StatusActive,
	}
	_ = userRepo.CreateUser(ctx, adminUser)

	// 2. Create a Regular Consumer User
	consumerUser := &entities.User{
		ID:         uuid.New(),
		Phone:      "777333444",
		Password:   hashedPassword,
		RoleID:     1,
		Role:       entities.Role{ID: 1, Name: roleRepo.RoleConsumer, DisplayName: "Consumer"},
		IsVerified: true,
		Status:     entities.StatusActive,
	}
	_ = userRepo.CreateUser(ctx, consumerUser)

	t.Run("Fail AdminLogin for Regular Consumer (403 Forbidden)", func(t *testing.T) {
		req := dto.UserLoginRequest{
			Phone:    "777333444",
			Password: rawPassword,
		}

		_, err := authService.AdminLogin(ctx, req)
		if err != dto.ErrAdminAccessRequired {
			t.Fatalf("expected ErrAdminAccessRequired, got %v", err)
		}
	})

	t.Run("Successful AdminLogin for Admin User", func(t *testing.T) {
		req := dto.UserLoginRequest{
			Phone:    "777111222",
			Password: rawPassword,
		}

		res, err := authService.AdminLogin(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error during admin login: %v", err)
		}

		if res == nil || res.RoleName != roleRepo.RoleAdmin {
			t.Fatalf("expected role %s, got %v", roleRepo.RoleAdmin, res)
		}
	})

	t.Run("Successful GetMe for Authenticated User", func(t *testing.T) {
		res, err := authService.GetMe(ctx, adminUser.ID.String())
		if err != nil {
			t.Fatalf("unexpected error in GetMe: %v", err)
		}

		if res.ID != adminUser.ID.String() || res.Phone != adminUser.Phone {
			t.Fatalf("expected user details to match, got %v", res)
		}
	})

	t.Run("Fail GetMe for Non-existent User ID", func(t *testing.T) {
		_, err := authService.GetMe(ctx, uuid.NewString())
		if err != dto.ErrUserNotFound {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}
