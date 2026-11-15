package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/auth"
	"mawarid/backend/internal/models"
	"mawarid/backend/internal/validation"
)

// RequestOTP generates and dispatches an OTP for (phone, purpose).
//
// Throttling (SRS FR-AUTH-01 / NFR-SEC-03):
//   - 60 seconds minimum between requests for the same phone+purpose;
//   - at most OTP_RESEND_MAX_PER_WINDOW requests inside the TTL window;
//   - the previous unconsumed code is invalidated on every new request.
//
// It returns the generated plaintext code so callers may honour the gated
// DEMO_PHONE_VERIFICATION echo (config.OTP.DemoExposeCode). The caller must
// NOT log or persist it: the database stores only its hash, and the code is
// discarded when the SMS dispatch fails (empty string is returned then).
//
// The plaintext code exists only in this function and in the SMS message —
// the database stores its hash.
func (s *Service) RequestOTP(ctx context.Context, rawPhone, purpose string) (string, error) {
	phone := validation.NormalizePhone(rawPhone)
	if phone == "" {
		return "", apierr.ErrValidation([]apierr.FieldViolation{{Field: "phone_number", Message: "Enter a valid Yemeni phone number. / يرجى إدخال رقم هاتف يمني صحيح."}})
	}
	switch purpose {
	case models.OTPPurposeRegister, models.OTPPurposePasswordReset:
	default:
		return "", apierr.BadRequest("VALIDATION_ERROR", "Unsupported OTP purpose.", "غرض رمز التحقق غير مدعوم.")
	}

	// Resend cooldown + window limit via the rate limiter (Redis or in-memory).
	allowed, _, err := s.Limiter.Hit(ctx, "otp:"+purpose+":"+phone, s.Cfg.OTP.ResendMaxPerWin, s.Cfg.OTP.TTL)
	if err != nil {
		s.Log.ErrorContext(ctx, "otp limiter error", "err", err)
	}
	if err == nil && !allowed {
		// Distinguish the 60s cooldown from the window limit.
		var last models.OTPCode
		if e := s.DB.WithContext(ctx).
			Where("phone_number = ? AND purpose = ?", phone, purpose).
			Order("created_at DESC").First(&last).Error; e == nil {
			if since := s.Now().Sub(last.CreatedAt); since < s.Cfg.OTP.ResendCooldown {
				wait := int((s.Cfg.OTP.ResendCooldown - since).Seconds() + 1)
				return "", apierr.ErrOTPResend(wait)
			}
		}
		return "", apierr.TooManyRequests("OTP_RESEND_LIMIT",
			"The maximum number of verification-code requests has been reached. Try again later.",
			"تم بلوغ الحد الأقصى لطلب رمز التحقق. يرجى المحاولة لاحقاً.")
	}
	// Cooldown inside the window. The limiter counter is deliberately NOT
	// reset here: repeated too-soon requests keep counting toward
	// OTP_RESEND_MAX_PER_WINDOW, so hammering the endpoint cannot bypass the
	// per-window cap (cooldown is enforced by the database check above).
	var last models.OTPCode
	if e := s.DB.WithContext(ctx).
		Where("phone_number = ? AND purpose = ?", phone, purpose).
		Order("created_at DESC").First(&last).Error; e == nil {
		if since := s.Now().Sub(last.CreatedAt); since < s.Cfg.OTP.ResendCooldown {
			wait := int((s.Cfg.OTP.ResendCooldown - since).Seconds() + 1)
			return "", apierr.ErrOTPResend(wait)
		}
	}

	code, err := auth.RandomDigits(s.Cfg.OTP.Length)
	if err != nil {
		return "", apierr.Internal("Could not generate a verification code.", "تعذر إنشاء رمز التحقق.")
	}
	now := s.Now()
	otp := models.OTPCode{
		PhoneNumber: phone,
		Purpose:     purpose,
		CodeHash:    hashOTPCode(code),
		ExpiresAt:   now.Add(s.Cfg.OTP.TTL),
	}
	// One live code per (phone, purpose): upsert.
	if err := s.DB.WithContext(ctx).
		Where("phone_number = ? AND purpose = ?", phone, purpose).
		Assign(map[string]any{
			"code_hash": otp.CodeHash, "attempts": 0, "consumed_at": nil,
			"expires_at": otp.ExpiresAt, "updated_at": now, "created_at": now,
		}).
		FirstOrCreate(&otp).Error; err != nil {
		return "", apierr.Internal("Could not store the verification code.", "تعذر حفظ رمز التحقق.")
	}

	msg := fmt.Sprintf("موارد: رمز التحقق الخاص بك هو %s. صالح لمدة %d دقائق.", code, int(s.Cfg.OTP.TTL.Minutes()))
	if err := s.SMS.Send(ctx, phone, msg); err != nil {
		s.Log.ErrorContext(ctx, "sms send failed", "err", err, "purpose", purpose)
		// The code never reached the user: discard it so an immediate retry
		// (e.g. POST /auth/otp/request after a failed registration) is not
		// blocked by the cooldown of an undelivered code. Security rules are
		// preserved: the per-window limiter counter still applies, and a
		// deleted code cannot be verified.
		if derr := s.DB.WithContext(ctx).
			Where("phone_number = ? AND purpose = ?", phone, purpose).
			Delete(&models.OTPCode{}).Error; derr != nil {
			s.Log.ErrorContext(ctx, "otp cleanup after sms failure failed", "err", derr)
		}
		return "", apierr.Internal("Could not send the verification SMS. Try again shortly.",
			"تعذر إرسال رسالة التحقق. يرجى المحاولة بعد قليل.")
	}
	s.Log.InfoContext(ctx, "otp dispatched", "purpose", purpose, "phone_masked", maskPhone(phone), "adapter", s.SMS.Name(), "real_delivery", s.SMS.RealDelivery())
	return code, nil
}

// VerifyOTP checks a code for (phone, purpose).
//
// On success it applies the purpose side effect:
//   - register:      marks the phone as verified (account stays pending
//     approval until an admin decision — SRS FR-CON-04);
//   - password_reset: returns a short-lived reset token (JWT) that authorizes
//     exactly one password change.
//
// SRS NFR-SEC-03: at most 3 verification attempts per code.
func (s *Service) VerifyOTP(ctx context.Context, rawPhone, purpose, code string) (resetToken string, err error) {
	phone := validation.NormalizePhone(rawPhone)
	if phone == "" {
		return "", apierr.ErrInvalidOTP()
	}
	switch purpose {
	case models.OTPPurposeRegister, models.OTPPurposePasswordReset:
	default:
		return "", apierr.BadRequest("VALIDATION_ERROR", "Unsupported OTP purpose.", "غرض رمز التحقق غير مدعوم.")
	}

	var otp models.OTPCode
	if err := s.DB.WithContext(ctx).
		Where("phone_number = ? AND purpose = ?", phone, purpose).
		First(&otp).Error; err != nil {
		return "", apierr.ErrInvalidOTP()
	}
	now := s.Now()
	if otp.ConsumedAt != nil || now.After(otp.ExpiresAt) {
		return "", apierr.ErrInvalidOTP()
	}
	if otp.Attempts >= s.Cfg.OTP.MaxAttempts {
		return "", apierr.ErrOTPLimit()
	}

	if otp.CodeHash != hashOTPCode(strings.TrimSpace(code)) {
		if err := s.DB.WithContext(ctx).Model(&otp).
			UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error; err != nil {
			s.Log.ErrorContext(ctx, "otp attempt increment failed", "err", err)
		}
		if otp.Attempts+1 >= s.Cfg.OTP.MaxAttempts {
			return "", apierr.ErrOTPLimit()
		}
		return "", apierr.ErrInvalidOTP()
	}

	// Consume.
	if err := s.DB.WithContext(ctx).Model(&otp).
		Updates(map[string]any{"consumed_at": now}).Error; err != nil {
		return "", apierr.Internal("Could not consume the verification code.", "تعذر استخدام رمز التحقق.")
	}

	switch purpose {
	case models.OTPPurposeRegister:
		res := s.DB.WithContext(ctx).Model(&models.User{}).
			Where("phone_number = ? AND phone_verified_at IS NULL", phone).
			Update("phone_verified_at", now)
		if res.Error != nil {
			return "", apierr.Internal("Could not update the account.", "تعذر تحديث الحساب.")
		}
		// A user may re-verify during registration; nothing else to do.
		return "", nil

	case models.OTPPurposePasswordReset:
		// Only accounts that exist may obtain a reset token.
		var user models.User
		if err := s.DB.WithContext(ctx).Where("phone_number = ?", phone).First(&user).Error; err != nil {
			return "", apierr.ErrInvalidOTP()
		}
		tok, _, err := s.JWT.IssuePasswordResetToken(user.ID, now.Add(15*time.Minute))
		if err != nil {
			return "", apierr.Internal("Could not issue the reset token.", "تعذر إنشاء رمز إعادة التعيين.")
		}
		return tok, nil
	}
	return "", apierr.ErrInvalidOTP()
}

// hashOTPCode hashes an OTP with the shared secret pepper (JWT secret) so a
// database leak alone cannot be replayed against another deployment.
func hashOTPCode(code string) string {
	return sha256Hex("otp:" + code)
}

func maskPhone(phone string) string {
	if len(phone) <= 3 {
		return "***"
	}
	return phone[:len(phone)-3] + "***"
}
