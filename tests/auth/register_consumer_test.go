package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/auth/dto"
	"github.com/najm-x/nova/modules/auth/service"
	userDto "github.com/najm-x/nova/modules/user/dto"
)

// TestRegisterConsumerScenarios tests consumer profile registration.
func TestRegisterConsumerScenarios(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	roleRepo := newMockRoleRepo()
	sms := newMockSmsProvider()
	otpRepo := newMockOtpRepo()
	otpService := service.NewOtpService(otpRepo, userRepo, sms)

	authService := service.NewAuthService(userRepo, roleRepo, otpService, nil, sms)

	existingPhone := "771112233"
	_ = userRepo.CreateUser(ctx, &entities.User{
		ID:    uuid.New(),
		Phone: existingPhone,
	})

	t.Run("Fail When Phone Number Is Already Registered (409 Conflict)", func(t *testing.T) {
		req := userDto.CreateConsumerRequest{
			UserBase: userDto.UserBase{
				Phone:    existingPhone,
				Password: "Password123!",
			},
			Name:     "Existing Citizen",
			IdNumber: "0101010101",
			IsHome:   true,
		}

		_, err := authService.RegisterConsumer(ctx, req)
		if err != dto.ErrPhoneAlreadyExists {
			t.Fatalf("expected ErrPhoneAlreadyExists, got %v", err)
		}
	})

	t.Run("Successful Registration for New Consumer", func(t *testing.T) {
		familyCount := 4
		req := userDto.CreateConsumerRequest{
			UserBase: userDto.UserBase{
				Phone:    "772223344",
				Password: "StrongPassword123!",
			},
			Name:        "New Consumer",
			IdNumber:    "1020304050",
			IdImage:     "consumers/id/uuid.jpg.enc",
			IsHome:      true,
			FamilyCount: &familyCount,
		}

		res, err := authService.RegisterConsumer(ctx, req)
		if err != nil {
			t.Fatalf("unexpected registration error: %v", err)
		}

		if res == nil {
			t.Fatal("expected non-nil response")
		}

		if res.Phone != "772223344" {
			t.Fatalf("expected phone 772223344, got %s", res.Phone)
		}

		if res.IsVerified != false {
			t.Fatal("new registered user must initially be unverified")
		}
	})
}

