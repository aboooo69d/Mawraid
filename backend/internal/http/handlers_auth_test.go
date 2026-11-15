package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mawarid/backend/internal/apierr"
	"mawarid/backend/internal/config"
	"mawarid/backend/internal/service"
)

// ---------------------------------------------------------------------------
// Task 2 — logout body validation (decodeJSONOptional)
// ---------------------------------------------------------------------------

func TestDecodeJSONOptional(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantErrCode string // "" = accepted
		wantToken   string
	}{
		{"empty body → logout-all", "", "", ""},
		{"whitespace body → logout-all", "   \n\t", "", ""},
		{"empty object → logout-all", "{}", "", ""},
		{"valid refresh token → single session", `{"refresh_token":"tok-abc"}`, "", "tok-abc"},
		{"malformed JSON → rejected", "{not json", "INVALID_JSON", ""},
		{"trailing content → rejected", `{"refresh_token":"x"} {"a":1}`, "INVALID_JSON", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(tc.body))
			var in refreshInput
			err := decodeJSONOptional(rec, req, &in)
			if tc.wantErrCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if in.RefreshToken != tc.wantToken {
					t.Errorf("refresh_token = %q, want %q", in.RefreshToken, tc.wantToken)
				}
				return
			}
			if err == nil {
				t.Fatalf("want %s, got nil", tc.wantErrCode)
			}
			var ae *apierr.Error
			if !errors.As(err, &ae) || ae.Code != tc.wantErrCode {
				t.Fatalf("error = %v, want code %s", err, tc.wantErrCode)
			}
		})
	}
}

func TestDecodeJSONOptionalRejectsOversizedBody(t *testing.T) {
	rec := httptest.NewRecorder()
	big := `{"refresh_token":"` + strings.Repeat("a", 2<<20) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader([]byte(big)))
	var in refreshInput
	err := decodeJSONOptional(rec, req, &in)
	if err == nil || !strings.Contains(err.Error(), "PAYLOAD_TOO_LARGE") {
		t.Fatalf("oversized body: err = %v, want PAYLOAD_TOO_LARGE", err)
	}
}

// TestLogoutRejectsMalformedJSONBeforeRevocation proves that malformed JSON
// answers 400 INVALID_JSON and never reaches session revocation: the Server
// below has a nil service, so any call into svc.Logout would panic instead of
// writing the 400 envelope.
func TestLogoutRejectsMalformedJSONBeforeRevocation(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := &Server{cfg: &config.Config{}, log: log} // svc intentionally nil

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader("{not json"))
	req = req.WithContext(context.WithValue(req.Context(), ctxUserID, uint(1)))
	srv.handleLogout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	var env struct {
		Error struct {
			Code      string `json:"code"`
			MessageAr string `json:"message_ar"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not the error envelope: %s", rec.Body.String())
	}
	if env.Error.Code != "INVALID_JSON" {
		t.Errorf("code = %s, want INVALID_JSON", env.Error.Code)
	}
	if env.Error.MessageAr == "" {
		t.Error("message_ar missing (NFR-USA-02)")
	}
}

// ---------------------------------------------------------------------------
// Task 1 — registration payload reflects the OTP dispatch outcome
// ---------------------------------------------------------------------------

func TestRegistrationPayloadOTPDispatchOutcome(t *testing.T) {
	user := service.UserResponse{ID: 7, FullName: "Test User", PhoneNumber: "771234567"}

	// OTP dispatched → next_step verify_otp + the caller-provided messages.
	p := registrationPayload(service.RegistrationResult{User: user, OTPSent: true},
		"ok message", "ok message ar")
	if p["next_step"] != "verify_otp" {
		t.Errorf("next_step = %v, want verify_otp", p["next_step"])
	}
	if p["message"] != "ok message" || p["message_ar"] != "ok message ar" {
		t.Errorf("success messages overwritten: %v / %v", p["message"], p["message_ar"])
	}
	if got, ok := p["user"].(service.UserResponse); !ok || got.ID != 7 {
		t.Errorf("user not embedded: %v", p["user"])
	}

	// OTP dispatch FAILED → the account exists: next_step request_otp with a
	// bilingual explanation, NOT a re-registration instruction.
	p2 := registrationPayload(service.RegistrationResult{User: user, OTPSent: false},
		"ok message", "ok message ar")
	if p2["next_step"] != "request_otp" {
		t.Errorf("next_step = %v, want request_otp", p2["next_step"])
	}
	if p2["message_ar"] == "" || p2["message_ar"] == "ok message ar" {
		t.Errorf("failure message_ar not set: %v", p2["message_ar"])
	}
	if p2["message"] == "" || p2["message"] == "ok message" {
		t.Errorf("failure message not set: %v", p2["message"])
	}
	if got, ok := p2["user"].(service.UserResponse); !ok || got.ID != 7 {
		t.Errorf("user missing after OTP failure: %v", p2["user"])
	}

	// Gated DEMO_PHONE_VERIFICATION: the dev_otp_code field appears ONLY when
	// the service populated it (config.OTP.DemoExposeCode enabled) and is
	// omitted otherwise — production never carries it.
	p3 := registrationPayload(service.RegistrationResult{
		User: user, OTPSent: true, DemoOTPCode: "483920",
	}, "ok message", "ok message ar")
	if p3["dev_otp_code"] != "483920" {
		t.Errorf("dev_otp_code = %v, want 483920", p3["dev_otp_code"])
	}
	if p3["next_step"] != "verify_otp" {
		t.Errorf("next_step with demo code = %v, want verify_otp", p3["next_step"])
	}
	if _, exists := p["dev_otp_code"]; exists {
		t.Error("dev_otp_code must be absent when DemoOTPCode is empty")
	}
	if _, exists := p2["dev_otp_code"]; exists {
		t.Error("dev_otp_code must be absent when the OTP dispatch failed")
	}
}
