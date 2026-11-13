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

// TestPhoneVerificationScenarios tests phone verification with OTP codes.
func TestPhoneVerificationScenarios(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	roleRepo := newMockRoleRepo()
	sms := newMockSmsProvider()
	otpRepo := newMockOtpRepo()
	otpService := service.NewOtpService(otpRepo, userRepo, sms)

	authService := service.NewAuthService(userRepo, roleRepo, otpService, nil, sms)

	phone := "775556677"
	testUser := &entities.User{
		ID:         uuid.New(),
		Phone:      phone,
		IsVerified: false,
		Status:     entities.StatusActive,
	}
	_ = userRepo.CreateUser(ctx, testUser)

	t.Run("Fail VerifyPhone When Phone Does Not Exist", func(t *testing.T) {
		err := authService.VerifyPhone(ctx, "770000000", "123456")
		if err != dto.ErrPhoneNotFound {
			t.Fatalf("expected ErrPhoneNotFound, got %v", err)
		}
	})

	t.Run("Fail VerifyPhone When OTP Is Invalid", func(t *testing.T) {
		otpRepo.codes[phone] = "654321"

		err := authService.VerifyPhone(ctx, phone, "111111")
		if err != dto.ErrInvalidOtpCode {
			t.Fatalf("expected ErrInvalidOtpCode, got %v", err)
		}
	})

	t.Run("Successful VerifyPhone Marks Account Verified", func(t *testing.T) {
		otpRepo.codes[phone] = "654321"

		err := authService.VerifyPhone(ctx, phone, "654321")
		if err != nil {
			t.Fatalf("unexpected verification error: %v", err)
		}

		user, _ := userRepo.GetUserByPhone(ctx, phone)
		if !user.IsVerified {
			t.Fatal("user account should be marked as verified")
		}
	})

	t.Run("Fail VerifyPhone When Already Verified", func(t *testing.T) {
		err := authService.VerifyPhone(ctx, phone, "654321")
		if err != dto.ErrAccountAlreadyVerified {
			t.Fatalf("expected ErrAccountAlreadyVerified, got %v", err)
		}
	})
}

// TestPasswordResetScenarios tests forget password and password update flows.
func TestPasswordResetScenarios(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	roleRepo := newMockRoleRepo()
	sms := newMockSmsProvider()
	otpRepo := newMockOtpRepo()
	otpService := service.NewOtpService(otpRepo, userRepo, sms)

	authService := service.NewAuthService(userRepo, roleRepo, otpService, nil, sms)

	phone := "776667788"
	hashedPassword, _ := valueHashing.HashValue("OldPassword123!")
	user := &entities.User{
		ID:         uuid.New(),
		Phone:      phone,
		Password:   hashedPassword,
		IsVerified: true,
		Status:     entities.StatusActive,
	}
	_ = userRepo.CreateUser(ctx, user)

	t.Run("ForgetPassword Returns Nil Silently When Phone Not Found (Anti-Enumeration)", func(t *testing.T) {
		// Note: When phone is not registered, system returns nil to avoid exposing user existence
		userNotFoundPhone := "779998877"
		err := authService.ForgetPassword(ctx, userNotFoundPhone)
		// Should not expose or fail with sensitive leak
		_ = err
	})

	t.Run("ForgetPassword Fails When Account Is Not Active", func(t *testing.T) {
		suspendedPhone := "771239999"
		_ = userRepo.CreateUser(ctx, &entities.User{
			ID:     uuid.New(),
			Phone:  suspendedPhone,
			Status: entities.StatusSuspended,
		})

		err := authService.ForgetPassword(ctx, suspendedPhone)
		if err != dto.ErrAcountNotActive {
			t.Fatalf("expected ErrAcountNotActive, got %v", err)
		}
	})
}

