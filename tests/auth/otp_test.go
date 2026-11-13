package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/auth/dto"
	"github.com/najm-x/nova/modules/auth/service"
)

// mockOtpRepo is an in-memory mock for authRepo.OtpRepo.
type mockOtpRepo struct {
	cooldowns    map[string]bool
	dailyLimits  map[string]int
	verifyLimits map[string]int
	codes        map[string]string
}

func newMockOtpRepo() *mockOtpRepo {
	return &mockOtpRepo{
		cooldowns:    make(map[string]bool),
		dailyLimits:  make(map[string]int),
		verifyLimits: make(map[string]int),
		codes:        make(map[string]string),
	}
}

func (m *mockOtpRepo) Save(c context.Context, phone string, code string) error {
	m.codes[phone] = code
	m.cooldowns[phone] = true
	m.dailyLimits[phone]++
	return nil
}

func (m *mockOtpRepo) Verify(c context.Context, phone string, code string) bool {
	m.verifyLimits[phone]++
	saved, ok := m.codes[phone]
	return ok && saved == code
}

func (m *mockOtpRepo) GetIsCooldown(c context.Context, phone string) bool {
	return m.cooldowns[phone]
}

func (m *mockOtpRepo) GetDailyLimit(c context.Context, phone string) int {
	return m.dailyLimits[phone]
}

func (m *mockOtpRepo) GetVerifyLimit(c context.Context, phone string) int {
	return m.verifyLimits[phone]
}

// TestOtpServiceScenarios tests rate limiting, daily caps, and verification attempts for OTP.
func TestOtpServiceScenarios(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	sms := newMockSmsProvider()

	existingPhone := "771234567"
	_ = userRepo.CreateUser(ctx, &entities.User{
		ID:         uuid.New(),
		Phone:      existingPhone,
		IsVerified: false,
		Status:     entities.StatusActive,
	})

	t.Run("Fail SendOtp When Cooldown Is Active", func(t *testing.T) {
		otpRepo := newMockOtpRepo()
		otpRepo.cooldowns[existingPhone] = true

		otpSvc := service.NewOtpService(otpRepo, userRepo, sms)
		err := otpSvc.SendOtp(ctx, existingPhone)
		if err != service.ErrCooldownActive {
			t.Fatalf("expected ErrCooldownActive, got %v", err)
		}
	})

	t.Run("Fail SendOtp When Daily Limit (5 Requests) Is Reached", func(t *testing.T) {
		otpRepo := newMockOtpRepo()
		otpRepo.dailyLimits[existingPhone] = service.MaxDailyOtpRequests

		otpSvc := service.NewOtpService(otpRepo, userRepo, sms)
		err := otpSvc.SendOtp(ctx, existingPhone)
		if err != service.ErrDailyLimitReached {
			t.Fatalf("expected ErrDailyLimitReached, got %v", err)
		}
	})

	t.Run("Fail SendOtp With InternalServer When SMS Provider Fails", func(t *testing.T) {
		otpRepo := newMockOtpRepo()
		failingSms := newMockSmsProvider()
		failingSms.shouldFail = true

		otpSvc := service.NewOtpService(otpRepo, userRepo, failingSms)
		err := otpSvc.SendOtp(ctx, existingPhone)
		if err != dto.ErrInternalServer {
			t.Fatalf("expected ErrInternalServer on SMS failure, got %v", err)
		}
	})

	t.Run("Fail VerifyOtp When Verification Limit (3 Attempts) Is Reached", func(t *testing.T) {
		otpRepo := newMockOtpRepo()
		otpRepo.verifyLimits[existingPhone] = service.MaxVerifyAttempts

		otpSvc := service.NewOtpService(otpRepo, userRepo, sms)
		err := otpSvc.VerifyOtp(ctx, existingPhone, "123456")
		if err != service.ErrVerifyLimitReached {
			t.Fatalf("expected ErrVerifyLimitReached, got %v", err)
		}
	})

	t.Run("Fail VerifyOtp When Code Is Wrong", func(t *testing.T) {
		otpRepo := newMockOtpRepo()
		otpRepo.codes[existingPhone] = "999888"

		otpSvc := service.NewOtpService(otpRepo, userRepo, sms)
		err := otpSvc.VerifyOtp(ctx, existingPhone, "111222")
		if err != service.ErrInvalidOtp {
			t.Fatalf("expected ErrInvalidOtp, got %v", err)
		}
	})

	t.Run("Successful VerifyOtp When Code Matches", func(t *testing.T) {
		otpRepo := newMockOtpRepo()
		otpRepo.codes[existingPhone] = "123456"

		otpSvc := service.NewOtpService(otpRepo, userRepo, sms)
		err := otpSvc.VerifyOtp(ctx, existingPhone, "123456")
		if err != nil {
			t.Fatalf("unexpected verification error: %v", err)
		}
	})
}

