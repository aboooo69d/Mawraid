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

	providerType := &entities.ProviderType{
		Name: "Gas Supplier",
		Slug: "gas-supplier",
	}
	if err := userRepo.CreateProviderType(ctx, providerType); err != nil {
		t.Fatalf("failed to seed provider type: %v", err)
	}

	t.Run("Fail When Provider Phone Is Already Taken", func(t *testing.T) {
		req := userDto.CreateProviderRequest{
			UserBase: userDto.UserBase{
				Phone:    existingPhone,
				Password: "Password123!",
			},
			Name:           "Water Tanker Co",
			Description:    "Clean drinking water distribution",
			ProviderTypeID: providerType.ID,
		}

		_, err := authService.RegisterProvider(ctx, req)
		if err != dto.ErrPhoneAlreadyExists {
			t.Fatalf("expected ErrPhoneAlreadyExists, got %v", err)
		}
	})

	t.Run("Fail When Provider Type Does Not Exist", func(t *testing.T) {
		startAt := time.Now()
		endAt := startAt.Add(8 * time.Hour)

		req := userDto.CreateProviderRequest{
			UserBase: userDto.UserBase{
				Phone:    "779876543",
				Password: "SecurePassword123!",
			},
			Name:           "Unknown Type Provider",
			Description:    "Should fail because provider type does not exist",
			StartAt:        startAt,
			EndAt:          endAt,
			ProviderTypeID: 9999,
		}

		_, err := authService.RegisterProvider(ctx, req)
		if err != userDto.ErrProviderTypeNotFound {
			t.Fatalf("expected ErrProviderTypeNotFound, got %v", err)
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
			Name:           "Gas Cylinder Supply",
			Description:    "Household cooking gas delivery",
			StartAt:        startAt,
			EndAt:          endAt,
			ProviderTypeID: providerType.ID,
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

		if res.ProviderTypeID != providerType.ID {
			t.Fatalf("expected provider_type_id %d, got %d", providerType.ID, res.ProviderTypeID)
		}
		if res.ProviderType.ID != providerType.ID || res.ProviderType.Name != providerType.Name || res.ProviderType.Slug != providerType.Slug {
			t.Fatalf("expected provider_type %+v, got %+v", providerType, res.ProviderType)
		}
	})
}

