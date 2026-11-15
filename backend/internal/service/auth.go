package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/models"
	"mawarid/backend/internal/validation"
)

// ---------------------------------------------------------------------------
// Input / output shapes
// ---------------------------------------------------------------------------

// RegisterConsumerInput — Sprint 1 MVP citizen registration: EXACTLY three
// fields (approved MVP requirement). The national-ID / usage-type fields of the
// earlier SRS draft are gone: the identity number is collected later only
// through the optional POST /account/verification flow, and legacy clients that
// still send id_number / usage_types simply have those keys ignored by the
// JSON decoder (unknown fields are never validated).
type RegisterConsumerInput struct {
	FullName    string `json:"full_name"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
}

// RegisterProviderInput — SRS 1.2 scenario 1 / FR-PRV-01..03.
type RegisterProviderInput struct {
	FullName            string `json:"full_name"`
	PhoneNumber         string `json:"phone_number"`
	Password            string `json:"password"`
	ProviderName        string `json:"provider_name"`
	ProviderDescription string `json:"provider_description"`
	StartTime           string `json:"start_time"` // HH:MM
	EndTime             string `json:"end_time"`   // HH:MM, must be > start_time
	City                string `json:"city"`
	District            string `json:"district"`
	Street              string `json:"street"`
}

// LoginInput — SRS URS 1.3 scenario 1.
type LoginInput struct {
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
}

// RegistrationResult pairs the created user with the OTP dispatch outcome.
//
// The account is committed BEFORE the registration OTP is dispatched: when
// OTPSent is false the account EXISTS and the client must recover through
// POST /auth/otp/request — registering again would hit
// 409 ACCOUNT_ALREADY_EXISTS.
//
// DemoOTPCode is non-empty only when the gated DEMO_PHONE_VERIFICATION
// configuration (config.OTP.DemoExposeCode) is enabled AND the code was
// dispatched successfully; handlers echo it as `dev_otp_code`. It is empty
// in every other case (production refuses that configuration outright).
type RegistrationResult struct {
	User        UserResponse
	OTPSent     bool
	DemoOTPCode string
}

// TokenResponse is the login/refresh payload.
type TokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int          `json:"expires_in"` // seconds until access token expiry
	User         UserResponse `json:"user"`
}

// UserResponse is the public user representation (never includes hashes).
type UserResponse struct {
	ID                 uint                      `json:"id"`
	FullName           string                    `json:"full_name"`
	PhoneNumber        string                    `json:"phone_number"`
	Role               string                    `json:"role"`
	Status             models.AccountStatus      `json:"status"`
	VerificationStatus models.VerificationStatus `json:"verification_status"`
	PhoneVerified      bool                      `json:"phone_verified"`
	IDNumber           *string                   `json:"id_number,omitempty"`
	IDImageURL         *string                   `json:"id_image_url,omitempty"`
	CreatedAt          time.Time                 `json:"created_at"`
	// Dashboard routes the client to after login (SRS FR-AUTH-02).
	Dashboard string `json:"dashboard"`
}

func dashboardFor(role string) string {
	switch role {
	case models.RoleAdmin:
		return "/admin"
	case models.RoleProvider:
		return "/provider/dashboard"
	case models.RoleDriver:
		return "/driver"
	default:
		return "/home"
	}
}

// toUserResponse builds the public view.
func toUserResponse(u *models.User) UserResponse {
	phoneVerified := u.PhoneVerifiedAt != nil
	return UserResponse{
		ID: u.ID, FullName: u.FullName, PhoneNumber: u.PhoneNumber,
		Role: u.Role.Name, Status: u.Status, VerificationStatus: u.VerificationStatus,
		PhoneVerified: phoneVerified, IDNumber: u.IDNumber, IDImageURL: u.IDImageURL,
		CreatedAt: u.CreatedAt, Dashboard: dashboardFor(u.Role.Name),
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

// RegisterConsumer creates a CITIZEN account and dispatches the registration
// OTP. Sprint 1 MVP (approved requirement): registration takes EXACTLY three
// fields — full name, phone number, password. No national ID is collected here
// (the optional identity flow later uses POST /account/verification), and the
// citizen needs NO admin approval: the account is created active immediately.
// Admin approval remains mandatory for providers (see RegisterProvider).
//
// Login is still gated by phone verification: PHONE_NOT_VERIFIED (403) until
// the registration OTP is confirmed, so an unverified number can never sign in.
//
// OTP dispatch failure does NOT fail the registration: the account is already
// committed, so the result reports OTPSent=false and the client recovers via
// POST /auth/otp/request instead of retrying registration (409).
func (s *Service) RegisterConsumer(ctx context.Context, in RegisterConsumerInput) (RegistrationResult, error) {
	var v validation.Violations

	fullName := validation.SanitizeText(in.FullName)
	v.Required("full_name", fullName)
	v.TextLength("full_name", fullName, 2, 120)

	phone := v.Phone("phone_number", in.PhoneNumber)
	v.Password("password", in.Password)

	if err := v.Err(); err != nil {
		return RegistrationResult{}, err
	}

	// MVP: duplicates are detected by phone number only (no ID number exists
	// in the contract anymore). Legacy rows that carry an id_number remain
	// unique via their phone_number, which has always been UNIQUE NOT NULL.
	var dup uint64
	if err := s.DB.WithContext(ctx).Raw(
		`SELECT count(*) FROM users WHERE phone_number = ?`,
		phone).Scan(&dup).Error; err != nil {
		return RegistrationResult{}, apierr.Internal("Registration is temporarily unavailable.", "التسجيل غير متاح مؤقتاً.")
	}
	if dup > 0 {
		return RegistrationResult{}, apierr.ErrDuplicateAccount()
	}

	hash, err := s.Hasher.Hash(in.Password)
	if err != nil {
		return RegistrationResult{}, apierr.Internal("Registration is temporarily unavailable.", "التسجيل غير متاح مؤقتاً.")
	}

	var role models.Role
	if err := s.DB.WithContext(ctx).Where("name = ?", models.RoleConsumer).First(&role).Error; err != nil {
		return RegistrationResult{}, apierr.Internal("Registration is temporarily unavailable.", "التسجيل غير متاح مؤقتاً.")
	}

	// MVP: citizen self-registration creates an ACTIVE account — no admin
	// review step (providers keep pending_approval below). IDNumber stays
	// NULL: users.id_number is nullable in the schema, so no migration is
	// needed for the three-field contract.
	user := models.User{
		RoleID: role.ID, FullName: fullName, PhoneNumber: phone,
		PasswordHash: hash, IsActive: true,
		Status: models.AccountStatusActive,
	}
	// ERD default profile: every citizen is a home customer (customers.type
	// CHECK allows home|business; subtype is the generic home profile). The
	// Home subtype row is created with family_count NULL — the column is
	// nullable, and the optional identity/verification flow fills data later.
	customer := models.Customer{Type: models.CustomerTypeHome, Subtype: models.CustomerTypeHome}
	home := models.Home{}

	txErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		customer.UserID = user.ID
		if err := tx.Create(&customer).Error; err != nil {
			return err
		}
		home.UserID = user.ID
		return tx.Create(&home).Error
	})
	if txErr != nil {
		// Unique-violation race on phone.
		if strings.Contains(txErr.Error(), "duplicate key") || strings.Contains(txErr.Error(), "SQLSTATE 23505") {
			return RegistrationResult{}, apierr.ErrDuplicateAccount()
		}
		s.Log.ErrorContext(ctx, "consumer registration transaction failed", "err", txErr)
		return RegistrationResult{}, apierr.Internal("Registration is temporarily unavailable.", "التسجيل غير متاح مؤقتاً.")
	}

	s.audit(ctx, nil, &user.ID, "account.register.consumer", "", nil)
	s.Log.InfoContext(ctx, "consumer registered", "user_id", user.ID, "status", user.Status)

	// URS 1.1 scenario 1: dispatch OTP right after account creation. The
	// account is already committed — a dispatch failure must not fail the
	// registration (the client recovers via POST /auth/otp/request).
	otpSent := true
	demoCode := ""
	otpCode, err := s.RequestOTP(ctx, phone, models.OTPPurposeRegister)
	if err != nil {
		s.Log.ErrorContext(ctx, "registration otp dispatch failed",
			"user_id", user.ID, "purpose", models.OTPPurposeRegister, "err", err)
		otpSent = false
	} else if s.Cfg.OTP.DemoExposeCode {
		// Gated DEMO_PHONE_VERIFICATION echo (never in production — config
		// validation refuses to boot with the flag enabled there).
		demoCode = otpCode
	}

	user.Role = role
	return RegistrationResult{User: toUserResponse(&user), OTPSent: otpSent, DemoOTPCode: demoCode}, nil
}

// RegisterProvider creates a provider account (status pending_approval) with
// its first location, and dispatches the registration OTP.
// SRS 1.2 scenario 1 / FR-PRV-01..03.
//
// Like RegisterConsumer, an OTP dispatch failure does NOT fail the
// registration: the account exists and the client recovers through
// POST /auth/otp/request (OTPSent=false in the result).
func (s *Service) RegisterProvider(ctx context.Context, in RegisterProviderInput) (RegistrationResult, error) {
	var v validation.Violations

	fullName := validation.SanitizeText(in.FullName)
	v.Required("full_name", fullName)
	v.TextLength("full_name", fullName, 2, 120)

	phone := v.Phone("phone_number", in.PhoneNumber)
	v.Password("password", in.Password)

	pName := validation.SanitizeText(in.ProviderName)
	v.Required("provider_name", pName) // FR-PRV-01 required
	v.TextLength("provider_name", pName, 2, 120)

	pDesc := validation.SanitizeText(in.ProviderDescription)
	v.Required("provider_description", pDesc)
	v.TextLength("provider_description", pDesc, 2, 500) // FR-PRV-01 max 500

	v.Required("start_time", in.StartTime)
	v.TimeHHMM("start_time", in.StartTime)
	v.Required("end_time", in.EndTime)
	v.TimeHHMM("end_time", in.EndTime)
	if validTimeRe.MatchString(in.StartTime) && validTimeRe.MatchString(in.EndTime) && in.EndTime <= in.StartTime {
		// SRS 1.2 scenario 3 / FR-PRV-02.
		v.Add("end_time",
			"End time must be after start time.",
			"يجب أن يكون وقت انتهاء العمل بعد وقت بدء العمل.")
	}

	city := validation.SanitizeText(in.City)
	v.Required("city", city) // FR-PRV-03

	if err := v.Err(); err != nil {
		return RegistrationResult{}, err
	}

	// Duplicate check (URS 1.2 scenario 4).
	var dup uint64
	if err := s.DB.WithContext(ctx).Raw(`SELECT count(*) FROM users WHERE phone_number = ?`, phone).Scan(&dup).Error; err != nil {
		return RegistrationResult{}, apierr.Internal("Registration is temporarily unavailable.", "التسجيل غير متاح مؤقتاً.")
	}
	if dup > 0 {
		return RegistrationResult{}, apierr.ErrDuplicateAccount()
	}

	hash, err := s.Hasher.Hash(in.Password)
	if err != nil {
		return RegistrationResult{}, apierr.Internal("Registration is temporarily unavailable.", "التسجيل غير متاح مؤقتاً.")
	}
	var role models.Role
	if err := s.DB.WithContext(ctx).Where("name = ?", models.RoleProvider).First(&role).Error; err != nil {
		return RegistrationResult{}, apierr.Internal("Registration is temporarily unavailable.", "التسجيل غير متاح مؤقتاً.")
	}

	user := models.User{
		RoleID: role.ID, FullName: fullName, PhoneNumber: phone,
		PasswordHash: hash, IsActive: true,
		Status: models.AccountStatusPendingApproval,
	}
	start, end := in.StartTime, in.EndTime
	provider := models.Provider{
		ProviderName: pName, Description: ptr(pDesc),
		StartTime: &start, EndTime: &end,
	}
	district := validation.SanitizeText(in.District)
	street := validation.SanitizeText(in.Street)
	location := models.ProviderLocation{
		Name: pName, City: city, IsActive: true,
	}
	if district != "" {
		location.District = ptr(district)
	}
	if street != "" {
		location.Street = ptr(street)
	}

	txErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		provider.UserID = user.ID
		if err := tx.Create(&provider).Error; err != nil {
			return err
		}
		location.ProviderID = user.ID
		return tx.Create(&location).Error
	})
	if txErr != nil {
		if strings.Contains(txErr.Error(), "duplicate key") || strings.Contains(txErr.Error(), "SQLSTATE 23505") {
			return RegistrationResult{}, apierr.ErrDuplicateAccount()
		}
		s.Log.ErrorContext(ctx, "provider registration transaction failed", "err", txErr)
		return RegistrationResult{}, apierr.Internal("Registration is temporarily unavailable.", "التسجيل غير متاح مؤقتاً.")
	}

	s.audit(ctx, nil, &user.ID, "account.register.provider", "", nil)
	s.Log.InfoContext(ctx, "provider registered", "user_id", user.ID, "status", user.Status)

	otpSent := true
	demoCode := ""
	otpCode, err := s.RequestOTP(ctx, phone, models.OTPPurposeRegister)
	if err != nil {
		s.Log.ErrorContext(ctx, "registration otp dispatch failed",
			"user_id", user.ID, "purpose", models.OTPPurposeRegister, "err", err)
		otpSent = false
	} else if s.Cfg.OTP.DemoExposeCode {
		demoCode = otpCode
	}

	user.Role = role
	return RegistrationResult{User: toUserResponse(&user), OTPSent: otpSent, DemoOTPCode: demoCode}, nil
}

// ---------------------------------------------------------------------------
// Login / sessions (SRS URS 1.3, FR-AUTH-02/03, NFR-SEC-03)
// ---------------------------------------------------------------------------

const (
	loginAttemptsKey = "login:attempts:"
	loginLockKey     = "login:lock:"
)

// Login authenticates phone+password, enforces the lockout policy, and issues
// the access/refresh token pair plus a revocable server-side session.
func (s *Service) Login(ctx context.Context, in LoginInput, meta SessionMeta) (*TokenResponse, error) {
	var v validation.Violations
	phone := v.Phone("phone_number", in.PhoneNumber)
	if strings.TrimSpace(in.Password) == "" {
		v.Add("password", "This field is required.", "هذا الحقل مطلوب.")
	}
	if err := v.Err(); err != nil {
		return nil, err
	}

	maxFailed := s.Cfg.Security.LoginMaxFailed
	lockWin := s.Cfg.Security.LoginLockoutWin

	// 1) Existing lock? (SRS URS 1.3 scenario 5 / NFR-SEC-03)
	if _, retry, err := s.Limiter.Peek(ctx, loginLockKey+phone); err == nil && retry > 0 {
		return nil, apierr.ErrAccountLocked(int(retry.Minutes()) + 1)
	}
	// Also treat "already at threshold" as locked (defensive).
	if count, retry, err := s.Limiter.Peek(ctx, loginAttemptsKey+phone); err == nil && count >= maxFailed && retry > 0 {
		_, _, _ = s.Limiter.Hit(ctx, loginLockKey+phone, 1, retry)
		return nil, apierr.ErrAccountLocked(int(retry.Minutes()) + 1)
	}

	var user models.User
	err := s.DB.WithContext(ctx).Preload("Role").Where("phone_number = ?", phone).First(&user).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierr.Internal("Login is temporarily unavailable.", "تسجيل الدخول غير متاح مؤقتاً.")
		}
		// Count unknown-phone attempts too (prevents user enumeration timing).
		if _, _, e := s.Limiter.Hit(ctx, loginAttemptsKey+phone, maxFailed, lockWin); e == nil {
			if count, _, _ := s.Limiter.Peek(ctx, loginAttemptsKey+phone); count >= maxFailed {
				_, _, _ = s.Limiter.Hit(ctx, loginLockKey+phone, 1, lockWin)
				s.recordLoginAttempt(ctx, phone, meta, false, "unknown_user")
				return nil, apierr.ErrAccountLocked(int(lockWin.Minutes()))
			}
		}
		s.recordLoginAttempt(ctx, phone, meta, false, "unknown_user")
		return nil, apierr.ErrInvalidCredentials()
	}

	// 2) Verify password (constant-time argon2id compare).
	if !s.Hasher.Verify(in.Password, user.PasswordHash) {
		_, _, _ = s.Limiter.Hit(ctx, loginAttemptsKey+phone, maxFailed, lockWin)
		count, _, _ := s.Limiter.Peek(ctx, loginAttemptsKey+phone)
		s.recordLoginAttempt(ctx, phone, meta, false, "bad_password")
		if count >= maxFailed {
			_, _, _ = s.Limiter.Hit(ctx, loginLockKey+phone, 1, lockWin)
			return nil, apierr.ErrAccountLocked(int(lockWin.Minutes()))
		}
		return nil, apierr.ErrInvalidCredentials()
	}

	// 3) Account state gate (SRS FR-AUTH-02: Active / Pending / Rejected / Suspended).
	if user.PhoneVerifiedAt == nil {
		return nil, apierr.New(403, "PHONE_NOT_VERIFIED",
			"Please verify your phone number first.",
			"يرجى تأكيد رقم الهاتف أولاً.")
	}
	switch user.Status {
	case models.AccountStatusPendingApproval:
		return nil, apierr.ErrAccountPending()
	case models.AccountStatusRejected:
		return nil, apierr.ErrAccountRejected()
	case models.AccountStatusSuspended:
		return nil, apierr.ErrAccountSuspended()
	case models.AccountStatusActive:
	default:
		return nil, apierr.ErrAccountPending()
	}
	if !user.IsActive {
		return nil, apierr.ErrAccountSuspended()
	}

	// 4) Success — reset counters and open a session.
	_ = s.Limiter.Reset(ctx, loginAttemptsKey+phone)
	_ = s.Limiter.Reset(ctx, loginLockKey+phone)
	s.recordLoginAttempt(ctx, phone, meta, true, "")

	resp, err := s.issueSession(ctx, &user, meta)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, &user.ID, &user.ID, "auth.login", "", meta.IPPtr())
	return resp, nil
}

// SessionMeta is the caller context stored with sessions.
type SessionMeta struct {
	UserAgent string
	IP        string
}

// IPPtr returns a pointer for audit storage.
func (m SessionMeta) IPPtr() *string {
	if m.IP == "" {
		return nil
	}
	return &m.IP
}

// issueSession mints the access token and a rotating opaque refresh token.
func (s *Service) issueSession(ctx context.Context, user *models.User, meta SessionMeta) (*TokenResponse, error) {
	access, exp, err := s.JWT.IssueAccessToken(user.ID, user.Role.Name)
	if err != nil {
		return nil, apierr.Internal("Could not create a session.", "تعذر إنشاء الجلسة.")
	}
	refresh, err := randomToken()
	if err != nil {
		return nil, apierr.Internal("Could not create a session.", "تعذر إنشاء الجلسة.")
	}
	sess := models.AuthSession{
		ID:               refresh[:16],
		UserID:           user.ID,
		RefreshTokenHash: sha256Hex(refresh),
		ExpiresAt:        s.Now().Add(s.Cfg.Security.JWTRefreshTTL),
	}
	if meta.UserAgent != "" {
		sess.UserAgent = ptr(meta.UserAgent)
	}
	if meta.IP != "" {
		sess.IP = ptr(meta.IP)
	}
	if err := s.DB.WithContext(ctx).Create(&sess).Error; err != nil {
		return nil, apierr.Internal("Could not create a session.", "تعذر إنشاء الجلسة.")
	}

	return &TokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(time.Until(exp).Seconds()),
		User:         toUserResponse(user),
	}, nil
}

// Refresh rotates a session: the presented refresh token is exchanged for a
// new access+refresh pair; the old refresh token is invalidated.
func (s *Service) Refresh(ctx context.Context, refreshToken string, meta SessionMeta) (*TokenResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, apierr.BadRequest("REFRESH_TOKEN_REQUIRED", "A refresh token is required.", "مطلوب رمز تحديث.")
	}
	var sess models.AuthSession
	err := s.DB.WithContext(ctx).Preload("User").Preload("User.Role").
		Where("refresh_token_hash = ?", sha256Hex(refreshToken)).First(&sess).Error
	if err != nil {
		return nil, apierr.Unauthorized("SESSION_INVALID", "Session is invalid or expired.", "الجلسة غير صالحة أو منتهية.")
	}
	now := s.Now()
	if sess.RevokedAt != nil || now.After(sess.ExpiresAt) {
		return nil, apierr.Unauthorized("SESSION_INVALID", "Session is invalid or expired.", "الجلسة غير صالحة أو منتهية.")
	}
	if sess.User == nil || sess.User.Status != models.AccountStatusActive || !sess.User.IsActive {
		return nil, apierr.ErrAccountPending()
	}

	// Rotate: revoke old, create new.
	if err := s.DB.WithContext(ctx).Model(&sess).UpdateColumn("revoked_at", now).Error; err != nil {
		return nil, apierr.Internal("Could not refresh the session.", "تعذر تحديث الجلسة.")
	}
	return s.issueSession(ctx, sess.User, meta)
}

// Logout revokes the session behind refreshToken; when refreshToken is empty
// every session of the authenticated user is revoked (SRS FR-AUTH-03).
func (s *Service) Logout(ctx context.Context, userID uint, refreshToken string) error {
	q := s.DB.WithContext(ctx).Model(&models.AuthSession{}).Where("user_id = ? AND revoked_at IS NULL", userID)
	if refreshToken != "" {
		q = q.Where("refresh_token_hash = ?", sha256Hex(refreshToken))
	}
	if err := q.Update("revoked_at", s.Now()).Error; err != nil {
		return apierr.Internal("Could not close the session.", "تعذر إغلاق الجلسة.")
	}
	s.audit(ctx, &userID, &userID, "auth.logout", "", nil)
	return nil
}

// ---------------------------------------------------------------------------
// Password recovery (SRS URS 1.3 scenario 6)
// ---------------------------------------------------------------------------

// ForgotPassword starts recovery: always answers success for known or unknown
// phones (no account enumeration) and dispatches an OTP when the phone exists.
func (s *Service) ForgotPassword(ctx context.Context, rawPhone string) error {
	phone := validation.NormalizePhone(rawPhone)
	if phone == "" {
		return apierr.ErrValidation([]apierr.FieldViolation{{Field: "phone_number", Message: "Enter a valid Yemeni phone number. / يرجى إدخال رقم هاتف يمني صحيح."}})
	}
	var user models.User
	if err := s.DB.WithContext(ctx).Where("phone_number = ?", phone).First(&user).Error; err != nil {
		s.Log.InfoContext(ctx, "password recovery requested for unknown phone", "phone_masked", maskPhone(phone))
		return nil
	}
	// The generated code is intentionally discarded here: the gated demo echo
	// (config.OTP.DemoExposeCode) applies to the registration/OTP-request
	// endpoints only, never to the password-recovery response.
	_, err := s.RequestOTP(ctx, phone, models.OTPPurposePasswordReset)
	return err
}

// ResetPassword exchanges a verified-OTP reset token for a new password and
// revokes every existing session of that user.
func (s *Service) ResetPassword(ctx context.Context, resetToken, newPassword string) error {
	userID, err := s.JWT.VerifyPasswordResetToken(resetToken)
	if err != nil {
		return apierr.Unauthorized("RESET_TOKEN_INVALID",
			"The reset link is invalid or expired. Request a new code.",
			"رمز الاستعادة غير صالح أو منتهي الصلاحية. يرجى طلب رمز جديد.")
	}
	var v validation.Violations
	v.Password("new_password", newPassword)
	if err := v.Err(); err != nil {
		return err
	}
	hash, err := s.Hasher.Hash(newPassword)
	if err != nil {
		return apierr.Internal("Could not update the password.", "تعذر تحديث كلمة المرور.")
	}
	res := s.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).
		Updates(map[string]any{"password_hash": hash, "updated_at": s.Now()})
	if res.Error != nil || res.RowsAffected == 0 {
		return apierr.Internal("Could not update the password.", "تعذر تحديث كلمة المرور.")
	}
	// Kill every session (password change implies global sign-out).
	_ = s.DB.WithContext(ctx).Model(&models.AuthSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", s.Now()).Error
	s.audit(ctx, &userID, &userID, "auth.password_reset", "", nil)
	return nil
}

// Me returns the authenticated user with its role and profile flags.
func (s *Service) Me(ctx context.Context, userID uint) (UserResponse, error) {
	var user models.User
	if err := s.DB.WithContext(ctx).Preload("Role").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UserResponse{}, apierr.ErrNotFoundResource("User")
		}
		return UserResponse{}, apierr.Internal("Could not load the account.", "تعذر تحميل الحساب.")
	}
	return toUserResponse(&user), nil
}

// GetUserRole loads the role name for middleware decisions.
func (s *Service) GetUserRole(ctx context.Context, userID uint) (string, error) {
	var user models.User
	if err := s.DB.WithContext(ctx).Preload("Role").First(&user, userID).Error; err != nil {
		return "", err
	}
	return user.Role.Name, nil
}

// --- helpers ----------------------------------------------------------------

func (s *Service) recordLoginAttempt(ctx context.Context, phone string, meta SessionMeta, success bool, reason string) {
	attempt := models.LoginAttempt{Phone: phone, Success: success}
	if meta.IP != "" {
		attempt.IP = &meta.IP
	}
	if reason != "" {
		attempt.FailureReason = &reason
	}
	if err := s.DB.WithContext(ctx).Create(&attempt).Error; err != nil {
		s.Log.ErrorContext(ctx, "login attempt write failed", "err", err)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
