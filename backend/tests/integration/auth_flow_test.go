// Package integration_test contains the Sprint 1 end-to-end suite covering the
// whole auth + account + admin + provider flow over real HTTP against a real
// PostgreSQL database.
//
// RUN conditions (documented honestly in the report):
//
//	RUN_INTEGRATION=1 DATABASE_URL=postgres://... go test ./tests/integration/ -v
//
// Without RUN_INTEGRATION the suite skips (unit/HTTP tests still run anywhere).
// With RUN_INTEGRATION=1 and a DATABASE_URL that cannot be reached the suite
// FAILS (not silently skips) so real breakage is never hidden.
//
// SMS note: no real SMS is claimed or sent anywhere. Most tests read the OTP
// code from the captured dev-adapter log; TestDemoPhoneVerificationWithoutSMS
// additionally exercises the gated DEMO_PHONE_VERIFICATION response echo.
package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"mawarid/backend/internal/auth"
	"mawarid/backend/internal/cache"
	"mawarid/backend/internal/config"
	"mawarid/backend/internal/database"
	apihttp "mawarid/backend/internal/http"
	"mawarid/backend/internal/models"
	"mawarid/backend/internal/service"
	"mawarid/backend/internal/sms"
)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type env struct {
	t   *testing.T
	ts  *httptest.Server
	db  *gorm.DB
	cfg *config.Config
	sms *bytes.Buffer // captured dev-SMS log (holds OTP codes)
}

func newEnv(t *testing.T) *env {
	t.Helper()
	return newEnvWithSMSSender(t, nil)
}

// newEnvWithSMSSender is newEnv with a caller-supplied SMS sender factory
// (nil = the default dev adapter). Used by the OTP-dispatch-failure test to
// simulate an SMS gateway outage; the factory receives the captured-log
// logger so wrapped senders can delegate to the dev adapter.
func newEnvWithSMSSender(t *testing.T, senderFactory func(log *slog.Logger) sms.Sender) *env {
	t.Helper()

	if os.Getenv("RUN_INTEGRATION") == "" {
		t.Skip("integration suite skipped: set RUN_INTEGRATION=1 (and DATABASE_URL) to run")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("integration suite skipped: DATABASE_URL is not set")
	}

	cfg := &config.Config{}
	cfg.Env = "test"
	cfg.HTTP.APIBasePath = "/api/v1"
	cfg.HTTP.CORSOrigins = []string{"http://localhost:3000"}
	cfg.Security.JWTAccessTTL = 15 * time.Minute
	cfg.Security.JWTRefreshTTL = 14 * 24 * time.Hour
	cfg.Security.LoginMaxFailed = 5
	cfg.Security.LoginLockoutWin = 15 * time.Minute
	cfg.OTP.Length = 6
	cfg.OTP.TTL = 5 * time.Minute
	cfg.OTP.ResendCooldown = 60 * time.Second
	cfg.OTP.ResendMaxPerWin = 5
	cfg.OTP.MaxAttempts = 3
	cfg.Upload.MaxSizeBytes = 5 * 1024 * 1024
	cfg.Upload.Dir = t.TempDir()
	cfg.Redis.Disabled = true

	smsBuf := &bytes.Buffer{}
	log := slog.New(slog.NewTextHandler(smsBuf, nil))

	db, err := database.Connect(dsn, 5, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("cannot reach DATABASE_URL: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hasher := auth.NewHasher(1, 8*1024, 1, 32, 16)
	jwtm := auth.NewJWTManager("integration-test-secret-0123456789abcdef0123", cfg.Security.JWTAccessTTL)
	var sender sms.Sender = &sms.DevSender{Logger: log, From: "test"}
	if senderFactory != nil {
		sender = senderFactory(log)
	}
	limiter := cache.NewMemoryLimiter()
	t.Cleanup(func() { _ = limiter.Close() })

	svc := service.New(db, cfg, hasher, jwtm, sender, limiter, log)
	admin := service.NewAdminService(svc)
	prov := service.NewProviderService(svc)
	ts := httptest.NewServer(apihttp.New(cfg, svc, admin, prov, log).Handler())
	t.Cleanup(ts.Close)

	return &env{t: t, ts: ts, db: db, cfg: cfg, sms: smsBuf}
}

// uniquePhone returns a plausible fresh Yemeni number (7 + 8 digits).
func uniquePhone(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	n := binary.BigEndian.Uint32(b[:]) % 100000000
	return fmt.Sprintf("7%08d", n)
}

// lastOTP extracts the most recently logged OTP code from the dev-SMS buffer.
func (e *env) lastOTP() string {
	e.t.Helper()
	re := regexp.MustCompile(`رمز التحقق الخاص بك هو (\d+)`)
	matches := re.FindAllStringSubmatch(e.sms.String(), -1)
	if len(matches) == 0 {
		e.t.Fatalf("no OTP code found in the captured dev-SMS log")
	}
	return matches[len(matches)-1][1]
}

// call performs a JSON request and returns the response + raw body.
func (e *env) call(method, path, token string, body any) (*http.Response, []byte) {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.ts.URL+path, reader)
	if err != nil {
		e.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, raw
}

// wantStatus fails the test when the HTTP status does not match.
func (e *env) wantStatus(ctx string, resp *http.Response, raw []byte, want int) {
	e.t.Helper()
	if resp.StatusCode != want {
		e.t.Fatalf("%s: status = %d, want %d — body: %s", ctx, resp.StatusCode, want, raw)
	}
}

// wantCode fails the test when the envelope error code does not match.
func (e *env) wantCode(ctx string, raw []byte, wantCode string) {
	e.t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		e.t.Fatalf("%s: not an error envelope: %s", ctx, raw)
	}
	if env.Error.Code != wantCode {
		e.t.Fatalf("%s: error code = %s, want %s — body: %s", ctx, env.Error.Code, wantCode, raw)
	}
}

type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         struct {
		ID     uint   `json:"id"`
		Role   string `json:"role"`
		Status string `json:"status"`
	} `json:"user"`
}

func (e *env) login(phone, password string) (tokenPair, []byte) {
	e.t.Helper()
	resp, raw := e.call(http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"phone_number": phone, "password": password})
	e.wantStatus("login "+phone, resp, raw, 200)
	var tok tokenPair
	if err := json.Unmarshal(raw, &tok); err != nil {
		e.t.Fatalf("login body: %v (%s)", err, raw)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		e.t.Fatalf("missing tokens: %s", raw)
	}
	return tok, raw
}

// createAdmin inserts an active admin directly (bootstrap-equivalent setup).
func (e *env) createAdmin() (phone, password string) {
	e.t.Helper()
	phone = uniquePhone(e.t)
	password = "IntegrationAdmin1"

	var role models.Role
	if err := e.db.Where("name = ?", models.RoleAdmin).First(&role).Error; err != nil {
		role = models.Role{Name: models.RoleAdmin}
		if err := e.db.Create(&role).Error; err != nil {
			e.t.Fatalf("create admin role: %v", err)
		}
	}
	hash, err := auth.NewHasher(1, 8*1024, 1, 32, 16).Hash(password)
	if err != nil {
		e.t.Fatal(err)
	}
	now := time.Now().UTC()
	admin := models.User{
		RoleID: role.ID, FullName: "Integration Admin", PhoneNumber: phone,
		PasswordHash: hash, IsActive: true, Status: models.AccountStatusActive,
		PhoneVerifiedAt: &now, VerificationStatus: models.VerificationVerified,
	}
	if err := e.db.Create(&admin).Error; err != nil {
		e.t.Fatalf("create admin user: %v", err)
	}
	return phone, password
}

// pngBytes builds a minimal PNG-magic payload accepted by magic-byte sniffing.
func pngBytes() []byte {
	head := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	return append(head, bytes.Repeat([]byte{0x42}, 64)...)
}

// multipart builds a multipart/form-data body.
func buildMultipart(t *testing.T, fields map[string]string, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileName))
	h.Set("Content-Type", "image/png")
	fw, err := w.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

// ---------------------------------------------------------------------------
// The flow
// ---------------------------------------------------------------------------

func TestSprint1EndToEnd(t *testing.T) {
	e := newEnv(t)

	phone1 := uniquePhone(t)
	idNumber := fmt.Sprintf("ID%d", time.Now().UnixNano())
	adminPhone, adminPass := e.createAdmin()

	// --- 1. citizen registration — Sprint 1 MVP: EXACTLY three fields -------
	resp, raw := e.call(http.MethodPost, "/api/v1/auth/register/consumer", "",
		map[string]any{
			"full_name": "أحمد الصبري", "phone_number": phone1,
			"password": "Consumer123",
		})
	e.wantStatus("register consumer", resp, raw, 201)
	var regResp struct {
		User struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		} `json:"user"`
		NextStep   string `json:"next_step"`
		DevOTPCode string `json:"dev_otp_code"`
	}
	if err := json.Unmarshal(raw, &regResp); err != nil {
		t.Fatalf("register body: %v", err)
	}
	if regResp.User.Status != "active" {
		t.Errorf("new consumer status = %s, want active (no admin approval for citizens)",
			regResp.User.Status)
	}
	if regResp.NextStep != "verify_otp" {
		t.Errorf("next_step = %s, want verify_otp", regResp.NextStep)
	}
	if regResp.DevOTPCode != "" {
		t.Errorf("dev_otp_code must be absent without DEMO_PHONE_VERIFICATION, got %q",
			regResp.DevOTPCode)
	}
	consumerID := regResp.User.ID

	// Login before phone verification → PHONE_NOT_VERIFIED (FR-AUTH-02).
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"phone_number": phone1, "password": "Consumer123"})
	e.wantStatus("login pre-verification", resp, raw, 403)
	e.wantCode("login pre-verification", raw, "PHONE_NOT_VERIFIED")

	// OTP resend cooldown (FR-AUTH-01: 60s) right after registration dispatch.
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/request", "",
		map[string]any{"phone_number": phone1, "purpose": "register"})
	e.wantStatus("otp resend too soon", resp, raw, 429)
	e.wantCode("otp resend too soon", raw, "OTP_RESEND_TOO_SOON")

	// Wrong code first: 400, then correct code verifies the phone.
	code := e.lastOTP()
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/verify", "",
		map[string]any{"phone_number": phone1, "purpose": "register", "code": wrong})
	e.wantStatus("wrong otp", resp, raw, 400)
	e.wantCode("wrong otp", raw, "OTP_INVALID")

	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/verify", "",
		map[string]any{"phone_number": phone1, "purpose": "register", "code": code})
	e.wantStatus("verify otp", resp, raw, 200)

	// Sprint 1 MVP: citizens need NO admin approval — a verified, active
	// consumer signs in immediately. Admin approval remains mandatory for
	// providers (step 13) and the review/suspend/reject tools still work on
	// consumers (steps 8 and 12), they are just no longer a login gate.
	consumerTok, _ := e.login(phone1, "Consumer123")
	if consumerTok.User.Status != "active" {
		t.Errorf("consumer login status = %s, want active", consumerTok.User.Status)
	}

	// --- 2. admin session ----------------------------------------------------
	adminTok, _ := e.login(adminPhone, adminPass)

	// The verification queue endpoint answers admins.
	resp, raw = e.call(http.MethodGet, "/api/v1/admin/verifications", adminTok.AccessToken, nil)
	e.wantStatus("admin verifications", resp, raw, 200)

	resp, raw = e.call(http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d", consumerID),
		adminTok.AccessToken, nil)
	e.wantStatus("admin account detail", resp, raw, 200)

	// --- 3. consumer self-service --------------------------------------------
	resp, raw = e.call(http.MethodGet, "/api/v1/auth/me", consumerTok.AccessToken, nil)
	e.wantStatus("consumer me", resp, raw, 200)
	var me struct {
		Role          string `json:"role"`
		PhoneVerified bool   `json:"phone_verified"`
	}
	_ = json.Unmarshal(raw, &me)
	if me.Role != "consumer" || !me.PhoneVerified {
		t.Errorf("me = %+v", me)
	}

	resp, raw = e.call(http.MethodGet, "/api/v1/account/status", consumerTok.AccessToken, nil)
	e.wantStatus("account status", resp, raw, 200)
	var accStatus struct {
		CanLogin           bool   `json:"can_login"`
		VerificationStatus string `json:"verification_status"`
	}
	_ = json.Unmarshal(raw, &accStatus)
	if !accStatus.CanLogin {
		t.Errorf("can_login = false for a verified, active citizen: %s", raw)
	}

	// RBAC: consumer gets 403 on admin endpoints.
	resp, raw = e.call(http.MethodGet, "/api/v1/admin/accounts", consumerTok.AccessToken, nil)
	e.wantStatus("consumer hits admin route", resp, raw, 403)
	e.wantCode("consumer hits admin route", raw, "FORBIDDEN")

	// --- 5. identity verification + document upload (FR-CON-01/05) ----------
	resp, raw = e.call(http.MethodPost, "/api/v1/account/verification", consumerTok.AccessToken,
		map[string]any{"id_number": idNumber})
	e.wantStatus("submit verification", resp, raw, 200)

	body, ctype := buildMultipart(t, map[string]string{"document_type": "national_id"}, "id.png", pngBytes())
	req, _ := http.NewRequest(http.MethodPost, e.ts.URL+"/api/v1/account/documents", body)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("Authorization", "Bearer "+consumerTok.AccessToken)
	upResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	raw, _ = io.ReadAll(upResp.Body)
	upResp.Body.Close()
	e.wantStatus("upload document", upResp, raw, 201)
	var upRespBody struct {
		Document struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		} `json:"document"`
	}
	if err := json.Unmarshal(raw, &upRespBody); err != nil {
		t.Fatalf("upload body: %v (%s)", err, raw)
	}
	if upRespBody.Document.Status != "pending" {
		t.Errorf("document status = %s, want pending", upRespBody.Document.Status)
	}
	docID := upRespBody.Document.ID

	resp, raw = e.call(http.MethodGet, "/api/v1/account/documents", consumerTok.AccessToken, nil)
	e.wantStatus("list documents", resp, raw, 200)

	// --- 6. admin reviews the document → verification verified --------------
	resp, raw = e.call(http.MethodPost, fmt.Sprintf("/api/v1/admin/documents/%d/review", docID),
		adminTok.AccessToken, map[string]any{"decision": "approve"})
	e.wantStatus("approve document", resp, raw, 200)

	resp, raw = e.call(http.MethodGet, "/api/v1/account/status", consumerTok.AccessToken, nil)
	e.wantStatus("status after doc approval", resp, raw, 200)
	_ = json.Unmarshal(raw, &accStatus)
	if accStatus.VerificationStatus != "verified" {
		t.Errorf("verification_status = %s, want verified (%s)", accStatus.VerificationStatus, raw)
	}

	// --- 7. secure download (owner) ------------------------------------------
	resp, raw = e.call(http.MethodGet, fmt.Sprintf("/api/v1/account/documents/%d/download", docID),
		consumerTok.AccessToken, nil)
	e.wantStatus("download own document", resp, raw, 200)
	if !bytes.Equal(raw, pngBytes()) {
		t.Error("downloaded bytes differ from the uploaded file")
	}

	// --- 8. suspend / reinstate (NFR-AUTH-06) --------------------------------
	resp, raw = e.call(http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/suspend", consumerID),
		adminTok.AccessToken, map[string]any{"reason": "integration test"})
	e.wantStatus("suspend", resp, raw, 200)

	resp, raw = e.call(http.MethodGet, "/api/v1/auth/me", consumerTok.AccessToken, nil)
	e.wantStatus("me while suspended", resp, raw, 403)
	e.wantCode("me while suspended", raw, "ACCOUNT_SUSPENDED")

	resp, raw = e.call(http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/reinstate", consumerID),
		adminTok.AccessToken, map[string]any{"reason": "test done"})
	e.wantStatus("reinstate", resp, raw, 200)
	resp, raw = e.call(http.MethodGet, "/api/v1/auth/me", consumerTok.AccessToken, nil)
	e.wantStatus("me after reinstate", resp, raw, 200)

	// --- 9. logout + refresh revocation (FR-AUTH-03) -------------------------
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/logout", consumerTok.AccessToken,
		map[string]any{"refresh_token": consumerTok.RefreshToken})
	e.wantStatus("logout", resp, raw, 200)
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/refresh", "",
		map[string]any{"refresh_token": consumerTok.RefreshToken})
	e.wantStatus("refresh after logout", resp, raw, 401)
	e.wantCode("refresh after logout", raw, "SESSION_INVALID")

	// --- 10. password recovery (URS 1.3 scenario 6) --------------------------
	consumerTok, _ = e.login(phone1, "Consumer123") // fresh session

	resp, raw = e.call(http.MethodPost, "/api/v1/auth/forgot-password", "",
		map[string]any{"phone_number": phone1})
	e.wantStatus("forgot password", resp, raw, 200)

	// Cooldown applies to password_reset too.
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/forgot-password", "",
		map[string]any{"phone_number": phone1})
	e.wantStatus("forgot password resend", resp, raw, 429)
	e.wantCode("forgot password resend", raw, "OTP_RESEND_TOO_SOON")

	// Attempt limit (NFR-SEC-03): 3 wrong codes lock verification.
	resetCode := e.lastOTP()
	wrongReset := "000000"
	if wrongReset == resetCode {
		wrongReset = "111111"
	}
	verifyReset := func(code string) (*http.Response, []byte) {
		return e.call(http.MethodPost, "/api/v1/auth/otp/verify", "",
			map[string]any{"phone_number": phone1, "purpose": "password_reset", "code": code})
	}
	resp, raw = verifyReset(wrongReset)
	e.wantStatus("wrong reset otp 1", resp, raw, 400)
	resp, raw = verifyReset(wrongReset)
	e.wantStatus("wrong reset otp 2", resp, raw, 400)
	resp, raw = verifyReset(wrongReset)
	e.wantStatus("wrong reset otp 3", resp, raw, 429)
	e.wantCode("wrong reset otp 3", raw, "OTP_ATTEMPTS_EXCEEDED")

	// Bypass the 60s cooldown deterministically (test-controlled clock on the
	// OTP row) instead of sleeping, then request a fresh code.
	if err := e.db.Exec(`UPDATE otp_codes SET created_at = ? WHERE phone_number = ? AND purpose = ?`,
		time.Now().UTC().Add(-2*time.Minute), phone1, models.OTPPurposePasswordReset).Error; err != nil {
		t.Fatalf("cooldown bypass: %v", err)
	}
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/forgot-password", "",
		map[string]any{"phone_number": phone1})
	e.wantStatus("forgot password after cooldown bypass", resp, raw, 200)

	newResetCode := e.lastOTP()
	resp, raw = verifyReset(newResetCode)
	e.wantStatus("verify reset otp", resp, raw, 200)
	var verifyOut struct {
		ResetToken string `json:"reset_token"`
	}
	_ = json.Unmarshal(raw, &verifyOut)
	if verifyOut.ResetToken == "" {
		t.Fatalf("no reset_token in response: %s", raw)
	}

	resp, raw = e.call(http.MethodPost, "/api/v1/auth/reset-password", "",
		map[string]any{"reset_token": verifyOut.ResetToken, "new_password": "NewPass123"})
	e.wantStatus("reset password", resp, raw, 200)

	// Old password must fail, new one must work.
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"phone_number": phone1, "password": "Consumer123"})
	e.wantStatus("old password after reset", resp, raw, 401)
	e.wantCode("old password after reset", raw, "INVALID_CREDENTIALS")
	_, _ = e.login(phone1, "NewPass123")

	// --- 11. login attempt lockout (NFR-SEC-03) ------------------------------
	for i := 1; i <= 5; i++ {
		resp, raw = e.call(http.MethodPost, "/api/v1/auth/login", "",
			map[string]any{"phone_number": phone1, "password": "WrongPass99"})
		if i < 5 {
			e.wantStatus(fmt.Sprintf("failed login %d", i), resp, raw, 401)
			continue
		}
		e.wantStatus("failed login 5", resp, raw, 423)
		e.wantCode("failed login 5", raw, "ACCOUNT_LOCKED")
	}
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"phone_number": phone1, "password": "NewPass123"})
	e.wantStatus("correct password while locked", resp, raw, 423)
	e.wantCode("correct password while locked", raw, "ACCOUNT_LOCKED")

	// --- 12. rejected consumer (admin reject still applies to citizens) -----
	phone2 := uniquePhone(t)
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/register/consumer", "",
		map[string]any{
			"full_name": "Rejected User", "phone_number": phone2,
			"password": "Consumer456",
		})
	e.wantStatus("register consumer 2", resp, raw, 201)
	var reg2 struct {
		User struct {
			ID uint `json:"id"`
		} `json:"user"`
	}
	_ = json.Unmarshal(raw, &reg2)

	code2 := e.lastOTP()
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/verify", "",
		map[string]any{"phone_number": phone2, "purpose": "register", "code": code2})
	e.wantStatus("verify otp consumer 2", resp, raw, 200)

	resp, raw = e.call(http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/review", reg2.User.ID),
		adminTok.AccessToken, map[string]any{"decision": "reject", "comment": "data mismatch"})
	e.wantStatus("reject consumer 2", resp, raw, 200)

	resp, raw = e.call(http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"phone_number": phone2, "password": "Consumer456"})
	e.wantStatus("login rejected account", resp, raw, 403)
	e.wantCode("login rejected account", raw, "ACCOUNT_REJECTED")

	// --- 13. provider flow (URS 1.2 / FR-PRV-01..04) -------------------------
	phone3 := uniquePhone(t)
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/register/provider", "",
		map[string]any{
			"full_name": "مورد الغاز", "phone_number": phone3,
			"password":             "Provider123",
			"provider_name":        "Test Gas Co",
			"provider_description": "توصيل اسطوانات الغاز المنزلية",
			"start_time":           "08:00", "end_time": "17:00", "city": "صنعاء",
		})
	e.wantStatus("register provider", resp, raw, 201)
	var reg3 struct {
		User struct {
			ID uint `json:"id"`
		} `json:"user"`
	}
	_ = json.Unmarshal(raw, &reg3)
	providerID := reg3.User.ID

	// Pending provider cannot log in (approval required by the flows doc).
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"phone_number": phone3, "password": "Provider123"})
	e.wantStatus("provider login while unverified", resp, raw, 403)
	e.wantCode("provider login while unverified", raw, "PHONE_NOT_VERIFIED")

	code3 := e.lastOTP()
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/verify", "",
		map[string]any{"phone_number": phone3, "purpose": "register", "code": code3})
	e.wantStatus("verify provider otp", resp, raw, 200)

	resp, raw = e.call(http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"phone_number": phone3, "password": "Provider123"})
	e.wantStatus("provider login while pending approval", resp, raw, 403)
	e.wantCode("provider login while pending approval", raw, "ACCOUNT_PENDING_APPROVAL")

	resp, raw = e.call(http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/review", providerID),
		adminTok.AccessToken, map[string]any{"decision": "approve"})
	e.wantStatus("approve provider", resp, raw, 200)

	providerTok, _ := e.login(phone3, "Provider123")
	if providerTok.User.Role != "provider" || providerTok.User.Status != "active" {
		t.Errorf("provider login user = %+v", providerTok.User)
	}

	// Profile: end before start must be rejected (FR-PRV-02 / SRS scenario 3).
	resp, raw = e.call(http.MethodPut, "/api/v1/providers/me/profile", providerTok.AccessToken,
		map[string]any{
			"provider_name": "Test Gas Co", "provider_description": "updated",
			"start_time": "08:00", "end_time": "07:00", "city": "صنعاء",
		})
	e.wantStatus("profile with end<start", resp, raw, 400)
	e.wantCode("profile with end<start", raw, "VALIDATION_ERROR")

	// Valid profile update (with location fields upsert).
	resp, raw = e.call(http.MethodPut, "/api/v1/providers/me/profile", providerTok.AccessToken,
		map[string]any{
			"provider_name": "Test Gas Co", "provider_description": "توصيل سريع وموثوق",
			"start_time": "08:00", "end_time": "17:00",
			"city": "صنعاء", "district": "معين", "street": "شارع الزبيري",
			"latitude": 15.3694, "longitude": 44.1910,
		})
	e.wantStatus("profile update", resp, raw, 200)

	// Working hours: close before open → 400; valid set → stored.
	resp, raw = e.call(http.MethodPut, "/api/v1/providers/me/working-hours", providerTok.AccessToken,
		map[string]any{"working_hours": []map[string]any{
			{"day_of_week": 1, "start_time": "08:00", "end_time": "17:00"},
			{"day_of_week": 2, "start_time": "18:00", "end_time": "17:00"},
		}})
	e.wantStatus("working hours end<start", resp, raw, 400)

	resp, raw = e.call(http.MethodPut, "/api/v1/providers/me/working-hours", providerTok.AccessToken,
		map[string]any{"working_hours": []map[string]any{
			{"day_of_week": 1, "start_time": "08:00", "end_time": "17:00"},
			{"day_of_week": 6, "start_time": "09:00", "end_time": "13:00"},
		}})
	e.wantStatus("working hours update", resp, raw, 200)
	var hoursResp struct {
		WorkingHours []map[string]any `json:"working_hours"`
	}
	_ = json.Unmarshal(raw, &hoursResp)
	if len(hoursResp.WorkingHours) != 2 {
		t.Errorf("working hours count = %d, want 2 (%s)", len(hoursResp.WorkingHours), raw)
	}

	resp, raw = e.call(http.MethodPost, "/api/v1/providers/me/locations", providerTok.AccessToken,
		map[string]any{"name": "فرع عدن", "city": "عدن"})
	e.wantStatus("create location", resp, raw, 201)

	resp, raw = e.call(http.MethodGet, "/api/v1/providers/me/profile", providerTok.AccessToken, nil)
	e.wantStatus("get profile", resp, raw, 200)
	var profile struct {
		Provider     map[string]any `json:"provider"`
		Locations    []any          `json:"locations"`
		WorkingHours []any          `json:"working_hours"`
	}
	_ = json.Unmarshal(raw, &profile)
	if len(profile.Locations) < 2 || len(profile.WorkingHours) != 2 {
		t.Errorf("profile locations=%d hours=%d (%s)", len(profile.Locations), len(profile.WorkingHours), raw)
	}

	// Logo upload + public serving.
	lbody, lctype := buildMultipart(t, nil, "logo.png", pngBytes())
	lreq, _ := http.NewRequest(http.MethodPost, e.ts.URL+"/api/v1/providers/me/logo", lbody)
	lreq.Header.Set("Content-Type", lctype)
	lreq.Header.Set("Authorization", "Bearer "+providerTok.AccessToken)
	lresp, err := http.DefaultClient.Do(lreq)
	if err != nil {
		t.Fatalf("logo upload: %v", err)
	}
	raw, _ = io.ReadAll(lresp.Body)
	lresp.Body.Close()
	e.wantStatus("logo upload", lresp, raw, 200)
	var logoOut struct {
		LogoURL string `json:"logo_url"`
	}
	_ = json.Unmarshal(raw, &logoOut)
	if logoOut.LogoURL == "" {
		t.Fatalf("no logo_url: %s", raw)
	}

	resp, raw = e.call(http.MethodGet, logoOut.LogoURL, "", nil)
	e.wantStatus("serve logo", resp, raw, 200)
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "image/png") {
		t.Errorf("logo content-type = %q, want image/png", ct)
	}

	// Ownership: the provider may NOT download the consumer's identity document.
	resp, raw = e.call(http.MethodGet, fmt.Sprintf("/api/v1/account/documents/%d/download", docID),
		providerTok.AccessToken, nil)
	e.wantStatus("cross-user download", resp, raw, 403)
	e.wantCode("cross-user download", raw, "FORBIDDEN")

	// RBAC: provider cannot reach admin endpoints.
	resp, raw = e.call(http.MethodGet, "/api/v1/admin/accounts", providerTok.AccessToken, nil)
	e.wantStatus("provider hits admin route", resp, raw, 403)

	// --- 14. admin reads ------------------------------------------------------
	resp, raw = e.call(http.MethodGet, "/api/v1/admin/accounts?role=provider&status=active",
		adminTok.AccessToken, nil)
	e.wantStatus("admin list providers by role", resp, raw, 200)

	resp, raw = e.call(http.MethodGet, "/api/v1/admin/accounts?status=not_a_status",
		adminTok.AccessToken, nil)
	e.wantStatus("admin list invalid status", resp, raw, 400)

	resp, raw = e.call(http.MethodGet, "/api/v1/admin/accounts", adminTok.AccessToken, nil)
	e.wantStatus("admin list accounts", resp, raw, 200)

	resp, raw = e.call(http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d", consumerID),
		adminTok.AccessToken, nil)
	e.wantStatus("admin account detail", resp, raw, 200)

	resp, raw = e.call(http.MethodGet, "/api/v1/admin/providers", adminTok.AccessToken, nil)
	e.wantStatus("admin providers", resp, raw, 200)

	resp, raw = e.call(http.MethodGet, "/api/v1/admin/providers/stats", adminTok.AccessToken, nil)
	e.wantStatus("admin provider stats", resp, raw, 200)

	resp, raw = e.call(http.MethodGet, "/api/v1/admin/audit-logs", adminTok.AccessToken, nil)
	e.wantStatus("admin audit logs", resp, raw, 200)
	if !strings.Contains(string(raw), "account.review.approve") {
		t.Errorf("audit log missing review action: %s", raw)
	}

	resp, raw = e.call(http.MethodGet, "/api/v1/notifications", consumerTok.AccessToken, nil)
	e.wantStatus("notifications", resp, raw, 200)

	// Health endpoint with a live database.
	resp, raw = e.call(http.MethodGet, "/health", "", nil)
	e.wantStatus("health", resp, raw, 200)

	// No identity number may ever appear in the audit log details (NFR-SEC-02).
	resp, raw = e.call(http.MethodGet, "/api/v1/admin/audit-logs?limit=200", adminTok.AccessToken, nil)
	e.wantStatus("audit logs for PII scan", resp, raw, 200)
	if strings.Contains(string(raw), idNumber) {
		t.Error("id number leaked into the audit log response (NFR-SEC-02)")
	}
}

// ---------------------------------------------------------------------------
// Focused flows: registration OTP dispatch failure (reliability fix 1) and
// logout body validation (reliability fix 2). DB-gated like the rest.
// ---------------------------------------------------------------------------

// failFirstSender fails the first `remaining` Send calls (simulated SMS
// gateway outage) and then delegates to a working sender.
type failFirstSender struct {
	inner     sms.Sender
	mu        sync.Mutex
	remaining int
}

func (f *failFirstSender) Send(ctx context.Context, phone, message string) error {
	f.mu.Lock()
	if f.remaining > 0 {
		f.remaining--
		f.mu.Unlock()
		return errors.New("simulated SMS gateway outage")
	}
	f.mu.Unlock()
	return f.inner.Send(ctx, phone, message)
}

func (f *failFirstSender) Name() string       { return "fail-first" }
func (f *failFirstSender) RealDelivery() bool { return false }

// TestRegistrationSurvivesOTPDispatchFailure pins the fix: an SMS dispatch
// failure AFTER the account is committed no longer fails the registration
// (no misleading 500 → 409 loop); the client recovers via /auth/otp/request.
func TestRegistrationSurvivesOTPDispatchFailure(t *testing.T) {
	e := newEnvWithSMSSender(t, func(log *slog.Logger) sms.Sender {
		return &failFirstSender{
			inner:     &sms.DevSender{Logger: log, From: "test"},
			remaining: 2, // provider + consumer registrations both fail to dispatch
		}
	})

	// --- provider registration during the outage ---------------------------
	phoneP := uniquePhone(t)
	resp, raw := e.call(http.MethodPost, "/api/v1/auth/register/provider", "",
		map[string]any{
			"full_name": "مورد تجريبي", "phone_number": phoneP,
			"password":             "Provider789",
			"provider_name":        "Outage Gas Co",
			"provider_description": "اختبار تعذر إرسال الرسائل",
			"start_time":           "08:00", "end_time": "16:00", "city": "صنعاء",
		})
	e.wantStatus("provider register during sms outage", resp, raw, 201)
	var regP struct {
		User struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		} `json:"user"`
		NextStep string `json:"next_step"`
	}
	if err := json.Unmarshal(raw, &regP); err != nil {
		t.Fatalf("provider register body: %v (%s)", err, raw)
	}
	if regP.NextStep != "request_otp" {
		t.Errorf("provider next_step = %s, want request_otp (%s)", regP.NextStep, raw)
	}
	if regP.User.Status != "pending_approval" {
		t.Errorf("provider status = %s, want pending_approval — the account must exist (%s)",
			regP.User.Status, raw)
	}

	// --- consumer registration during the outage ---------------------------
	phone := uniquePhone(t)
	payload := map[string]any{
		"full_name": "مستخدم انقطاع", "phone_number": phone,
		"password": "Outage123",
	}
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/register/consumer", "", payload)
	e.wantStatus("consumer register during sms outage", resp, raw, 201)
	var reg struct {
		User struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		} `json:"user"`
		NextStep  string `json:"next_step"`
		MessageAr string `json:"message_ar"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("consumer register body: %v (%s)", err, raw)
	}
	if reg.NextStep != "request_otp" {
		t.Errorf("consumer next_step = %s, want request_otp (%s)", reg.NextStep, raw)
	}
	if reg.User.Status != "active" {
		t.Errorf("consumer status = %s, want active (%s)", reg.User.Status, raw)
	}
	if reg.MessageAr == "" {
		t.Error("consumer message_ar missing (NFR-USA-02)")
	}

	// The account exists: re-registering is NOT the recovery path.
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/register/consumer", "", payload)
	e.wantStatus("re-register same phone", resp, raw, 409)
	e.wantCode("re-register same phone", raw, "ACCOUNT_ALREADY_EXISTS")

	// Recovery: the SMS gateway is back — the next OTP request succeeds
	// immediately (the undelivered code was discarded, so it cannot block
	// its own retry) and the delivered code verifies the phone.
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/request", "",
		map[string]any{"phone_number": phone, "purpose": "register"})
	e.wantStatus("otp request after sms recovery", resp, raw, 200)

	code := e.lastOTP()
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/verify", "",
		map[string]any{"phone_number": phone, "purpose": "register", "code": code})
	e.wantStatus("verify otp after recovery", resp, raw, 200)
}

// TestDemoPhoneVerificationWithoutSMS pins the gated DEMO_PHONE_VERIFICATION
// mechanism (Sprint 1 MVP): with the flag on — development/demo only; the
// server refuses to boot in production with it — the freshly generated OTP is
// echoed as `dev_otp_code`, so the complete register → verify → login flow
// runs with NO SMS provider. The echo is not a verification bypass and not a
// universal code: every code is random, single-use, hashed at rest, and a
// wrong code still fails with OTP_INVALID under the same attempt limits.
func TestDemoPhoneVerificationWithoutSMS(t *testing.T) {
	e := newEnv(t)
	e.cfg.OTP.DemoExposeCode = true // gated flag; env=test, never production

	phone := uniquePhone(t)
	resp, raw := e.call(http.MethodPost, "/api/v1/auth/register/consumer", "",
		map[string]any{
			"full_name": "تجربة تحقق", "phone_number": phone,
			"password": "DemoPass123",
		})
	e.wantStatus("demo register", resp, raw, 201)
	var reg struct {
		User struct {
			Status string `json:"status"`
		} `json:"user"`
		NextStep   string `json:"next_step"`
		DevOTPCode string `json:"dev_otp_code"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("register body: %v (%s)", err, raw)
	}
	if reg.User.Status != "active" || reg.NextStep != "verify_otp" {
		t.Errorf("register = %+v, want active + verify_otp (%s)", reg, raw)
	}
	code := reg.DevOTPCode
	if len(code) != 6 {
		t.Fatalf("dev_otp_code = %q, want a 6-digit code (%s)", code, raw)
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			t.Fatalf("dev_otp_code = %q is not numeric", code)
		}
	}

	// A wrong code still fails — the echo does not weaken verification.
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/verify", "",
		map[string]any{"phone_number": phone, "purpose": "register", "code": wrong})
	e.wantStatus("demo wrong otp", resp, raw, 400)
	e.wantCode("demo wrong otp", raw, "OTP_INVALID")

	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/verify", "",
		map[string]any{"phone_number": phone, "purpose": "register", "code": code})
	e.wantStatus("demo verify otp", resp, raw, 200)

	tok, _ := e.login(phone, "DemoPass123")
	if tok.User.Role != "consumer" || tok.User.Status != "active" {
		t.Errorf("demo login user = %+v, want active consumer", tok.User)
	}

	// The resend path also echoes a FRESH random code once the deterministic
	// cooldown bypass (same technique as the password-recovery test) elapses.
	if err := e.db.Exec(`UPDATE otp_codes SET created_at = ? WHERE phone_number = ? AND purpose = ?`,
		time.Now().UTC().Add(-2*time.Minute), phone, models.OTPPurposeRegister).Error; err != nil {
		t.Fatalf("cooldown bypass: %v", err)
	}
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/request", "",
		map[string]any{"phone_number": phone, "purpose": "register"})
	e.wantStatus("demo otp resend", resp, raw, 200)
	var resend struct {
		DevOTPCode string `json:"dev_otp_code"`
	}
	_ = json.Unmarshal(raw, &resend)
	if len(resend.DevOTPCode) != 6 {
		t.Fatalf("resend dev_otp_code = %q (%s)", resend.DevOTPCode, raw)
	}

	// Each code is single-use and independent: the fresh code verifies.
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/otp/verify", "",
		map[string]any{"phone_number": phone, "purpose": "register", "code": resend.DevOTPCode})
	e.wantStatus("verify fresh demo code", resp, raw, 200)
}

// TestLogoutBodyValidation pins the fix: malformed JSON is rejected with
// 400 INVALID_JSON and never revokes sessions; an empty body or {} still
// logs out of every session; a valid refresh token logs out exactly one.
func TestLogoutBodyValidation(t *testing.T) {
	e := newEnv(t)
	adminPhone, adminPass := e.createAdmin()
	tok, _ := e.login(adminPhone, adminPass)

	rawLogout := func(body string) (*http.Response, []byte) {
		req, err := http.NewRequest(http.MethodPost, e.ts.URL+"/api/v1/auth/logout", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, b
	}

	resp, raw := rawLogout("{not json")
	e.wantStatus("logout malformed json", resp, raw, 400)
	e.wantCode("logout malformed json", raw, "INVALID_JSON")

	// The rejected request must NOT have revoked anything.
	resp, raw = e.call(http.MethodGet, "/api/v1/auth/me", tok.AccessToken, nil)
	e.wantStatus("session survives rejected logout", resp, raw, 200)

	// Trailing content after a valid object is also rejected.
	resp, raw = rawLogout(`{"refresh_token":"x"} {"a":1}`)
	e.wantStatus("logout trailing content", resp, raw, 400)
	e.wantCode("logout trailing content", raw, "INVALID_JSON")

	// {} → intentional logout-all: the refresh token dies.
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/logout", tok.AccessToken, map[string]any{})
	e.wantStatus("logout empty object", resp, raw, 200)
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/refresh", "",
		map[string]any{"refresh_token": tok.RefreshToken})
	e.wantStatus("refresh after logout-all", resp, raw, 401)
	e.wantCode("refresh after logout-all", raw, "SESSION_INVALID")

	// No body at all → also logout-all.
	tok2, _ := e.login(adminPhone, adminPass)
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/logout", tok2.AccessToken, nil)
	e.wantStatus("logout empty body", resp, raw, 200)
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/refresh", "",
		map[string]any{"refresh_token": tok2.RefreshToken})
	e.wantStatus("refresh after empty-body logout", resp, raw, 401)
	e.wantCode("refresh after empty-body logout", raw, "SESSION_INVALID")

	// A valid refresh token revokes exactly that session, leaving others.
	tok3, _ := e.login(adminPhone, adminPass)
	tok4, _ := e.login(adminPhone, adminPass)
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/logout", tok3.AccessToken,
		map[string]any{"refresh_token": tok3.RefreshToken})
	e.wantStatus("logout single session", resp, raw, 200)
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/refresh", "",
		map[string]any{"refresh_token": tok3.RefreshToken})
	e.wantStatus("refresh revoked session", resp, raw, 401)
	e.wantCode("refresh revoked session", raw, "SESSION_INVALID")
	resp, raw = e.call(http.MethodPost, "/api/v1/auth/refresh", "",
		map[string]any{"refresh_token": tok4.RefreshToken})
	e.wantStatus("refresh surviving session", resp, raw, 200)
}
