package service

import (
	"context"
	"crypto/rand"
	"math/big"

	"github.com/najm-x/nova/modules/auth/repo"
	errsCode "github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
	smsSender "github.com/najm-x/nova/pkg/sms_sender"
)

var (
	// 429 Too Many Requests - أخطاء تجاوز الحدود الزمنية والمحاولات للـ OTP
	ErrCooldownActive = &respons.AppError{
		Status:  429,
		Code:    errsCode.ErrCooldownActive,
		Message: "please wait a minute before requesting a new code", // "please wait a minute before requesting a new code"
	}
	ErrDailyLimitReached = &respons.AppError{
		Status:  429,
		Code:    errsCode.ErrDailyLimitReached,
		Message: "daily OTP request limit reached", // "daily OTP request limit reached"
	}
	ErrVerifyLimitReached = &respons.AppError{
		Status:  429,
		Code:    errsCode.ErrVerifyLimitReached,
		Message: "maximum verification attempts reached, request a new code", // "maximum verification attempts reached, request a new code"
	}

	// 400 Bad Request - الرمز غير صحيح أو منتهي
	ErrInvalidOtp = &respons.AppError{
		Status:  400,
		Code:    errsCode.ErrInvalidOtp,
		Message: "رمز التحقق غير صحيح أو منتهي الصلاحية", // "invalid or expired verification code"
	}
)

const (
	MaxDailyOtpRequests = 5
	MaxVerifyAttempts   = 3
	OtpLength           = 6
)

type OtpService interface {
	SendOtp(c context.Context, phone string) (string, error)
	VerifyOtp(c context.Context, phone string, code string) error
}

type otpService struct {
	otpRepo repo.OtpRepo
	sms     smsSender.Provider
}

func NewOtpService(otpRepo repo.OtpRepo, sms smsSender.Provider) OtpService {
	return &otpService{
		otpRepo: otpRepo,
		sms:     sms,
	}
}

func (s *otpService) SendOtp(c context.Context, phone string) (string, error) {

	if s.otpRepo.GetIsCooldown(c, phone) {
		return "", ErrCooldownActive
	}

	if s.otpRepo.GetDailyLimit(c, phone) >= MaxDailyOtpRequests {
		return "", ErrDailyLimitReached
	}

	code, err := generateSecureOtp(OtpLength)
	if err != nil {
		return "", err
	}

	if err := s.otpRepo.Save(c, phone, code); err != nil {
		return "", err
	}
	go func() {
		s.sms.SendSMS(phone, code)
	}()

	return code, nil
}

func (s *otpService) VerifyOtp(c context.Context, phone string, code string) error {

	if s.otpRepo.GetVerifyLimit(c, phone) >= MaxVerifyAttempts {
		return ErrVerifyLimitReached
	}

	isValid := s.otpRepo.Verify(c, phone, code)
	if !isValid {
		return ErrInvalidOtp
	}

	return nil
}

func generateSecureOtp(length int) (string, error) {
	const digits = "0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		result[i] = digits[num.Int64()]
	}
	return string(result), nil
}
