package apierr

import (
	"strings"
	"testing"
)

func TestEnvelopeShape(t *testing.T) {
	e := ErrInvalidCredentials()
	if e.Status != 401 || e.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("unexpected: %d %s", e.Status, e.Code)
	}
	env := e.Envelope("req-123")
	if env.Code != "INVALID_CREDENTIALS" {
		t.Errorf("code = %q", env.Code)
	}
	if env.RequestID != "req-123" {
		t.Errorf("request_id = %q", env.RequestID)
	}
	if !strings.Contains(env.MessageAr, "رقم الهاتف") {
		t.Errorf("Arabic message missing: %q", env.MessageAr)
	}
	if env.Message == "" {
		t.Error("English message empty")
	}
}

func TestAllPrebuiltErrorsCarryArabic(t *testing.T) {
	errs := map[string]*Error{
		"invalid_credentials": ErrInvalidCredentials(),
		"account_locked":      ErrAccountLocked(15),
		"account_pending":     ErrAccountPending(),
		"account_rejected":    ErrAccountRejected(),
		"account_suspended":   ErrAccountSuspended(),
		"duplicate":           ErrDuplicateAccount(),
		"field_required":      ErrFieldRequired("phone_number"),
		"invalid_otp":         ErrInvalidOTP(),
		"otp_limit":           ErrOTPLimit(),
		"otp_resend":          ErrOTPResend(30),
		"unauthorized":        ErrUnauthorizedNoToken(),
		"forbidden":           ErrForbiddenRole(),
		"not_found":           ErrNotFoundResource("User"),
	}
	for name, e := range errs {
		if e.MessageAr == "" {
			t.Errorf("%s: message_ar empty", name)
		}
		if e.Message == "" {
			t.Errorf("%s: message empty", name)
		}
		if e.Code == "" || e.Status == 0 {
			t.Errorf("%s: code/status missing", name)
		}
	}
}

func TestValidationErrorsCarryFieldDetails(t *testing.T) {
	e := ErrValidation([]FieldViolation{
		{Field: "phone_number", Message: "invalid / غير صالح"},
	})
	if e.Status != 400 || e.Code != "VALIDATION_ERROR" {
		t.Fatalf("unexpected: %d %s", e.Status, e.Code)
	}
	if e.Details == nil {
		t.Fatal("details missing")
	}
	vs, ok := e.Details.([]FieldViolation)
	if !ok || len(vs) != 1 || vs[0].Field != "phone_number" {
		t.Errorf("details = %#v", e.Details)
	}
}

func TestLockedStatusIs423(t *testing.T) {
	if got := ErrAccountLocked(15).Status; got != 423 {
		t.Errorf("status = %d, want 423", got)
	}
	if !strings.Contains(ErrAccountLocked(15).MessageAr, "15") {
		t.Errorf("minutes missing from Arabic message: %q", ErrAccountLocked(15).MessageAr)
	}
}

func TestWithDetailsDoesNotMutateOriginal(t *testing.T) {
	base := BadRequest("SOME_CODE", "English", "عربي")
	with := base.WithDetails([]FieldViolation{{Field: "y", Message: "z"}})
	if base.Details != nil {
		t.Error("original error mutated")
	}
	if with.Details == nil {
		t.Error("copy missing details")
	}
	if with.Code != base.Code || with.Status != base.Status {
		t.Error("copy diverged from the original")
	}
}
