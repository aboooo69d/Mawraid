package http

import (
	"net/http"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/models"
	"mawarid/backend/internal/service"
)

// ---------------------------------------------------------------------------
// Registration (SRS FR-CON-01..03 / FR-PRV-01..03, URS 1.1 & 1.2)
//
// DEVIATION (documented): the SRS describes one multipart registration form;
// this API accepts JSON here and a separate authenticated-or-anonymous
// document upload at POST /account/documents (see api/openapi.yaml §notes).
// ---------------------------------------------------------------------------

func (s *Server) handleRegisterConsumer(w http.ResponseWriter, r *http.Request) {
	var in service.RegisterConsumerInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	res, err := s.svc.RegisterConsumer(r.Context(), in)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, registrationPayload(res,
		"Account created. Verify the code sent to your phone.",
		"تم إنشاء الحساب. يرجى تأكيد الرمز المرسل إلى هاتفك."))
}

func (s *Server) handleRegisterProvider(w http.ResponseWriter, r *http.Request) {
	var in service.RegisterProviderInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	res, err := s.svc.RegisterProvider(r.Context(), in)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, registrationPayload(res,
		"Account created. Verify the code sent to your phone, then wait for admin approval.",
		"تم إنشاء الحساب. يرجى تأكيد الرمز المرسل إلى هاتفك ثم انتظار موافقة المسؤول."))
}

// registrationPayload renders the 201 registration body. The account is
// committed before the OTP is dispatched: when the dispatch failed
// (OTPSent=false) the response still succeeds and steers the client to
// POST /auth/otp/request — re-registering would only hit
// 409 ACCOUNT_ALREADY_EXISTS.
func registrationPayload(res service.RegistrationResult, okMsg, okMsgAr string) map[string]any {
	payload := map[string]any{"user": res.User}
	// Gated DEMO_PHONE_VERIFICATION echo: the service only populates
	// DemoOTPCode when config.OTP.DemoExposeCode is enabled (production
	// refuses that configuration at startup), so an empty string simply
	// omits the field.
	if res.DemoOTPCode != "" {
		payload["dev_otp_code"] = res.DemoOTPCode
	}
	if res.OTPSent {
		payload["next_step"] = "verify_otp"
		payload["message"] = okMsg
		payload["message_ar"] = okMsgAr
		return payload
	}
	payload["next_step"] = "request_otp"
	payload["message"] = "Account created, but the verification SMS could not be sent. Request a new verification code to continue."
	payload["message_ar"] = "تم إنشاء الحساب، ولكن تعذر إرسال رسالة التحقق. يرجى طلب رمز تحقق جديد للمتابعة."
	return payload
}

// ---------------------------------------------------------------------------
// OTP request / verify (SRS FR-AUTH-01, NFR-SEC-03/04)
// ---------------------------------------------------------------------------

type otpRequestInput struct {
	PhoneNumber string `json:"phone_number"`
	Purpose     string `json:"purpose"` // register | password_reset
}

func (i *otpRequestInput) normalize() {
	if i.Purpose == "" {
		i.Purpose = models.OTPPurposeRegister
	}
}

func (s *Server) handleOTPRequest(w http.ResponseWriter, r *http.Request) {
	var in otpRequestInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	in.normalize()
	code, err := s.svc.RequestOTP(r.Context(), in.PhoneNumber, in.Purpose)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	resp := map[string]any{
		"status":     "otp_sent",
		"purpose":    in.Purpose,
		"message":    "Verification code sent.",
		"message_ar": "تم إرسال رمز التحقق.",
	}
	// Gated DEMO_PHONE_VERIFICATION echo (development/demo only): the freshly
	// generated code is returned so a local demo can complete verification
	// without any SMS provider. config.Validate() refuses to start in
	// production with this enabled; the code stays random, single-use and
	// hashed at rest with all TTL/attempt limits unchanged.
	if s.svc.Cfg.OTP.DemoExposeCode && code != "" {
		resp["dev_otp_code"] = code
	}
	s.writeJSON(w, r, http.StatusOK, resp)
}

type otpVerifyInput struct {
	PhoneNumber string `json:"phone_number"`
	Purpose     string `json:"purpose"`
	Code        string `json:"code"`
}

func (s *Server) handleOTPVerify(w http.ResponseWriter, r *http.Request) {
	var in otpVerifyInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	if in.Purpose == "" {
		in.Purpose = models.OTPPurposeRegister
	}
	resetToken, err := s.svc.VerifyOTP(r.Context(), in.PhoneNumber, in.Purpose, in.Code)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out := map[string]any{
		"verified":       true,
		"purpose":        in.Purpose,
		"phone_verified": in.Purpose == models.OTPPurposeRegister,
		"message":        "Verification code accepted.",
		"message_ar":     "تم قبول رمز التحقق.",
	}
	if resetToken != "" {
		out["reset_token"] = resetToken
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// Login / refresh / logout (SRS URS 1.3, FR-AUTH-02/03, NFR-SEC-03)
// ---------------------------------------------------------------------------

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in service.LoginInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	tokens, err := s.svc.Login(r.Context(), in, sessionMeta(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, tokens)
}

type refreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var in refreshInput
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	tokens, err := s.svc.Refresh(r.Context(), in.RefreshToken, sessionMeta(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, tokens)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var in refreshInput
	// The body is optional: an empty body (or {}) revokes every session of
	// the caller. Malformed JSON must NOT silently degrade to logout-all —
	// it is rejected with 400 INVALID_JSON.
	if err := decodeJSONOptional(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	userID := CurrentUserID(r.Context())
	if err := s.svc.Logout(r.Context(), userID, in.RefreshToken); err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"status":     "logged_out",
		"message":    "Signed out.",
		"message_ar": "تم تسجيل الخروج.",
	})
}

// ---------------------------------------------------------------------------
// Password recovery (SRS URS 1.3 scenario 6)
// ---------------------------------------------------------------------------

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PhoneNumber string `json:"phone_number"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	if err := s.svc.ForgotPassword(r.Context(), in.PhoneNumber); err != nil {
		s.writeError(w, r, err)
		return
	}
	// Always the same answer (unknown phones included) — no enumeration.
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"status":     "otp_sent",
		"message":    "If the phone number exists, a recovery code has been sent.",
		"message_ar": "إذا كان رقم الهاتف مسجلاً، فقد تم إرسال رمز الاستعادة.",
	})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ResetToken  string `json:"reset_token"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		s.writeError(w, r, err)
		return
	}
	if in.ResetToken == "" {
		s.writeError(w, r, apierr.BadRequest("RESET_TOKEN_REQUIRED",
			"A reset token is required.", "مطلوب رمز استعادة."))
		return
	}
	if err := s.svc.ResetPassword(r.Context(), in.ResetToken, in.NewPassword); err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{
		"status":     "password_updated",
		"message":    "Password updated. Please sign in with the new password.",
		"message_ar": "تم تحديث كلمة المرور. يرجى تسجيل الدخول بكلمة المرور الجديدة.",
	})
}

// ---------------------------------------------------------------------------
// Current user (SRS "currentUser")
// ---------------------------------------------------------------------------

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.svc.Me(r.Context(), CurrentUserID(r.Context()))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, user)
}
