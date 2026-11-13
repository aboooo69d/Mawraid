package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/auth/dto"
	"github.com/najm-x/nova/modules/auth/service"
	userDto "github.com/najm-x/nova/modules/user/dto"
)

// TestRegisterProviderScenarios tests service provider registration and uniqueness constraints.
func TestRegisterProviderScenarios(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	roleRepo := newMockRoleRepo()
	sms := newMockSmsProvider()
	otpRepo := newMockOtpRepo()
	otpService := service.NewOtpService(otpRepo, userRepo, sms)

	authService := service.NewAuthService(userRepo, roleRepo, otpService, nil, sms)

	existingPhone := "773334455"
	_ = userRepo.CreateUser(ctx, &entities.User{
		ID:    uuid.New(),
		Phone: existingPhone,
	})

	t.Run("Fail When Provider Phone Is Already Taken", func(t *testing.T) {
		req := userDto.CreateProviderRequest{
			UserBase: userDto.UserBase{
				Phone:    existingPhone,
				Password: "Password123!",
			},
			Name:        "Water Tanker Co",
			Description: "Clean drinking water distribution",
		}

		_, err := authService.RegisterProvider(ctx, req)
		if err != dto.ErrPhoneAlreadyExists {
			t.Fatalf("expected ErrPhoneAlreadyExists, got %v", err)
		}
	})

	t.Run("Successful Registration for New Provider", func(t *testing.T) {
		startAt := time.Now()
		endAt := startAt.Add(8 * time.Hour)

		req := userDto.CreateProviderRequest{
			UserBase: userDto.UserBase{
				Phone:    "774445566",
				Password: "SecurePassword123!",
			},
			Name:        "Gas Cylinder Supply",
			Description: "Household cooking gas delivery",
			StartAt:     startAt,
			EndAt:       endAt,
		}

		res, err := authService.RegisterProvider(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res == nil || res.Phone != "774445566" {
			t.Fatalf("expected phone 774445566, got %v", res)
		}

		if res.IsVerified != false {
			t.Fatal("provider must be unverified initially")
		}
	})
}

