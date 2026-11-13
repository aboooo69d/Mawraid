package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/auth/dto"
	"github.com/najm-x/nova/modules/auth/service"
	valueHashing "github.com/najm-x/nova/pkg/value_hashing"
)

// TestLoginServiceScenarios validates all authentication and status check branches in Login.
func TestLoginServiceScenarios(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	roleRepo := newMockRoleRepo()
	sms := newMockSmsProvider()

	authService := service.NewAuthService(userRepo, roleRepo, nil, nil, sms)

	rawPassword := "Password123!"
	hashedPassword, _ := valueHashing.HashValue(rawPassword)

	testUser := &entities.User{
		ID:         uuid.New(),
		Phone:      "771234567",
		Password:   hashedPassword,
		RoleID:     1,
		Role:       entities.Role{ID: 1, Name: "consumer"},
		IsVerified: true,
		Status:     entities.StatusActive,
	}
	_ = userRepo.CreateUser(ctx, testUser)

	t.Run("Fail When Phone Does Not Exist", func(t *testing.T) {
		req := dto.UserLoginRequest{
			Phone:    "779999999",
			Password: rawPassword,
		}
		_, err := authService.Login(ctx, req)
		if err != dto.ErrPhoneNotFound {
			t.Fatalf("expected ErrPhoneNotFound, got %v", err)
		}
	})

	t.Run("Fail When Password Is Incorrect", func(t *testing.T) {
		req := dto.UserLoginRequest{
			Phone:    "771234567",
			Password: "WrongPassword!",
		}
		_, err := authService.Login(ctx, req)
		if err != dto.ErrInvalidCredentials {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("Fail When Account Is Not Verified", func(t *testing.T) {
		unverifiedUser := &entities.User{
			ID:         uuid.New(),
			Phone:      "770000001",
			Password:   hashedPassword,
			RoleID:     1,
			IsVerified: false,
			Status:     entities.StatusActive,
		}
		_ = userRepo.CreateUser(ctx, unverifiedUser)

		req := dto.UserLoginRequest{
			Phone:    "770000001",
			Password: rawPassword,
		}
		_, err := authService.Login(ctx, req)
		if err != dto.ErrAcountNotVerify {
			t.Fatalf("expected ErrAcountNotVerify, got %v", err)
		}
	})

	t.Run("Fail When Account Status Is Pending", func(t *testing.T) {
		pendingUser := &entities.User{
			ID:         uuid.New(),
			Phone:      "770000002",
			Password:   hashedPassword,
			RoleID:     1,
			IsVerified: true,
			Status:     entities.StatusPending,
		}
		_ = userRepo.CreateUser(ctx, pendingUser)

		req := dto.UserLoginRequest{
			Phone:    "770000002",
			Password: rawPassword,
		}
		_, err := authService.Login(ctx, req)
		if err != dto.ErrAccountPending {
			t.Fatalf("expected ErrAccountPending, got %v", err)
		}
	})

	t.Run("Fail When Account Status Is Docs Required", func(t *testing.T) {
		docsUser := &entities.User{
			ID:         uuid.New(),
			Phone:      "770000003",
			Password:   hashedPassword,
			RoleID:     1,
			IsVerified: true,
			Status:     entities.StatusDocs,
		}
		_ = userRepo.CreateUser(ctx, docsUser)

		req := dto.UserLoginRequest{
			Phone:    "770000003",
			Password: rawPassword,
		}
		_, err := authService.Login(ctx, req)
		if err != dto.ErrAccountDocs {
			t.Fatalf("expected ErrAccountDocs, got %v", err)
		}
	})

	t.Run("Fail When Account Status Is Rejected", func(t *testing.T) {
		rejectedUser := &entities.User{
			ID:         uuid.New(),
			Phone:      "770000004",
			Password:   hashedPassword,
			RoleID:     1,
			IsVerified: true,
			Status:     entities.StatusRejected,
		}
		_ = userRepo.CreateUser(ctx, rejectedUser)

		req := dto.UserLoginRequest{
			Phone:    "770000004",
			Password: rawPassword,
		}
		_, err := authService.Login(ctx, req)
		if err != dto.ErrAccountRejected {
			t.Fatalf("expected ErrAccountRejected, got %v", err)
		}
	})

	t.Run("Fail When Account Status Is Suspended", func(t *testing.T) {
		suspendedUser := &entities.User{
			ID:         uuid.New(),
			Phone:      "770000005",
			Password:   hashedPassword,
			RoleID:     1,
			IsVerified: true,
			Status:     entities.StatusSuspended,
		}
		_ = userRepo.CreateUser(ctx, suspendedUser)

		req := dto.UserLoginRequest{
			Phone:    "770000005",
			Password: rawPassword,
		}
		_, err := authService.Login(ctx, req)
		if err != dto.StatusSuspended {
			t.Fatalf("expected StatusSuspended, got %v", err)
		}
	})

	t.Run("Successful Login for Verified Active User", func(t *testing.T) {
		req := dto.UserLoginRequest{
			Phone:    "771234567",
			Password: rawPassword,
		}
		res, err := authService.Login(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == nil || res.Phone != "771234567" {
			t.Fatalf("unexpected user response: %v", res)
		}
	})
}

