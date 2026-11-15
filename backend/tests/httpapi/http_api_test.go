// Package httpapi_test exercises the REST layer end-to-end through httptest
// WITHOUT a database: every case below is expected to fail (or pass) before
// any DB access, which keeps the suite runnable on machines without Docker.
// DB-dependent flows are covered by tests/integration (RUN_INTEGRATION=1).
package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apihttp "mawarid/backend/internal/http"

	"mawarid/backend/internal/auth"
	"mawarid/backend/internal/cache"
	"mawarid/backend/internal/config"
	"mawarid/backend/internal/service"
	"mawarid/backend/internal/sms"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	cfg := &config.Config{}
	cfg.Env = "test"
	cfg.HTTP.APIBasePath = "/api/v1"
	cfg.HTTP.CORSOrigins = []string{"http://localhost:3000"}
	cfg.Upload.MaxSizeBytes = 5 * 1024 * 1024
	cfg.OTP.Length = 6

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hasher := auth.NewHasher(1, 8*1024, 1, 32, 16)
	jwtm := auth.NewJWTManager("test-secret-0123456789abcdefghij0123456789", 15*60*1000000000)
	sender := &sms.DevSender{Logger: log, From: "test"}
	limiter := cache.NewMemoryLimiter()
	t.Cleanup(func() { _ = limiter.Close() })

	// db is intentionally nil: these tests must never reach the database.
	svc := service.New(nil, cfg, hasher, jwtm, sender, limiter, log)
	admin := service.NewAdminService(svc)
	prov := service.NewProviderService(svc)

	ts := httptest.NewServer(apihttp.New(cfg, svc, admin, prov, log).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func decodeError(t *testing.T, resp *http.Response) (status int, code, msgAr string) {
	t.Helper()
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			MessageAr string `json:"message_ar"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("error body is not the envelope: %s", raw)
	}
	return resp.StatusCode, body.Error.Code, body.Error.MessageAr
}

func TestUnknownEndpointReturnsJSONEnvelope(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	status, code, msgAr := decodeError(t, resp)
	if status != 404 || code != "ENDPOINT_NOT_FOUND" {
		t.Errorf("got %d/%s, want 404/ENDPOINT_NOT_FOUND", status, code)
	}
	if msgAr == "" {
		t.Error("message_ar missing (NFR-USA-02)")
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("x-request-id header missing")
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("content-type = %q", ct)
	}
}

func TestRequestIDPassthroughAndReplacement(t *testing.T) {
	ts := newTestServer(t)

	// Safe client-provided id is echoed.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/does-not-exist", nil)
	req.Header.Set("X-Request-Id", "client-abc-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("X-Request-Id"); got != "client-abc-123" {
		t.Errorf("echoed id = %q", got)
	}

	// Unsafe/oversized id is replaced with a generated one.
	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/does-not-exist", nil)
	req2.Header.Set("X-Request-Id", "bad id with spaces and <script>")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	got := resp2.Header.Get("X-Request-Id")
	resp2.Body.Close()
	if got == "" || got == "bad id with spaces and <script>" || strings.Contains(got, "<") {
		t.Errorf("unsafe id not replaced: %q", got)
	}
}

func TestMissingOrMalformedBearerToken(t *testing.T) {
	ts := newTestServer(t)
	cases := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"basic scheme", "Basic dXNlcjpwYXNz"},
		{"empty bearer", "Bearer "},
		{"garbage token", "Bearer not.a.jwt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/auth/me", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			status, code, msgAr := decodeError(t, resp)
			if status != 401 || code != "UNAUTHORIZED" {
				t.Errorf("got %d/%s, want 401/UNAUTHORIZED", status, code)
			}
			if msgAr == "" {
				t.Error("message_ar missing")
			}
		})
	}
}

func TestMalformedJSONBody(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	status, code, _ := decodeError(t, resp)
	if status != 400 || code != "INVALID_JSON" {
		t.Errorf("got %d/%s, want 400/INVALID_JSON", status, code)
	}
}

// TestLogoutAuthenticatedParsing is documented in tests/integration (real
// sessions); here we pin the precedence rule that needs no database: the
// bearer token is checked BEFORE the optional body is parsed, so an
// unauthenticated malformed-body probe can never trigger a session revocation.
func TestLogoutUnauthenticatedPrecedesBodyParsing(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/v1/auth/logout", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	status, code, msgAr := decodeError(t, resp)
	if status != 401 || code != "UNAUTHORIZED" {
		t.Errorf("got %d/%s, want 401/UNAUTHORIZED", status, code)
	}
	if msgAr == "" {
		t.Error("message_ar missing (NFR-USA-02)")
	}
}

func TestOversizedBodyRejected(t *testing.T) {
	ts := newTestServer(t)
	big := `{"phone_number":"` + strings.Repeat("7", 2<<20) + `"}`
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", strings.NewReader(big))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	status, code, _ := decodeError(t, resp)
	if status != 413 || code != "PAYLOAD_TOO_LARGE" {
		t.Errorf("got %d/%s, want 413/PAYLOAD_TOO_LARGE", status, code)
	}
}

// --- validation must happen BEFORE any database access (DB is nil here) -----

func TestLoginValidationFailsBeforeDatabase(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json",
		strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	status, code, msgAr := decodeError(t, resp)
	if status != 400 || code != "VALIDATION_ERROR" {
		t.Errorf("got %d/%s, want 400/VALIDATION_ERROR", status, code)
	}
	if msgAr == "" {
		t.Error("Arabic validation message missing (NFR-USA-02)")
	}
}

func TestRegisterValidationFailsBeforeDatabase(t *testing.T) {
	ts := newTestServer(t)
	// Sprint 1 MVP: exactly three fields are validated (full_name, phone,
	// password) — there is no id_number / usage_types contract anymore.
	payload := map[string]any{
		"full_name":    "A",
		"phone_number": "12345", // not a Yemeni number
		"password":     "abc",
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(ts.URL+"/api/v1/auth/register/consumer", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	status, code, _ := decodeError(t, resp)
	if status != 400 || code != "VALIDATION_ERROR" {
		t.Fatalf("got %d/%s, want 400/VALIDATION_ERROR", status, code)
	}
	_ = resp.Body.Close()

	// decodeError consumed the body — the details assertions need a fresh read,
	// so re-issue the request and parse the full envelope once.
	resp2, err := http.Post(ts.URL+"/api/v1/auth/register/consumer", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	raw, _ := io.ReadAll(resp2.Body)
	var env struct {
		Error struct {
			Code    string              `json:"code"`
			Details []map[string]string `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("envelope parse failed: %s", raw)
	}
	if env.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %s", env.Error.Code)
	}
	if len(env.Error.Details) == 0 {
		t.Errorf("no field violations in details: %s", raw)
	}
}

// Sprint 1 MVP: legacy keys (id_number, usage_types, family_count, ...) are
// simply ignored — they are neither validated nor stored. Only the three MVP
// fields can produce violations.
func TestRegisterIgnoresLegacyFields(t *testing.T) {
	ts := newTestServer(t)
	payload := map[string]any{
		"full_name":    "Ahmed Saleh",
		"phone_number": "12345", // the only invalid field here
		"password":     "Pass1234",
		"id_number":    "",                   // legacy — would previously violate
		"usage_types":  []string{"teleport"}, // legacy — would previously violate
		"family_count": 0,                    // legacy — would previously violate
		"plate_number": "",                   // legacy — would previously violate
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(ts.URL+"/api/v1/auth/register/consumer", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env struct {
		Error struct {
			Code    string              `json:"code"`
			Details []map[string]string `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("envelope parse failed: %s", raw)
	}
	if resp.StatusCode != 400 || env.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("got %d/%s, want 400/VALIDATION_ERROR", resp.StatusCode, env.Error.Code)
	}
	legacy := map[string]bool{
		"id_number": true, "usage_types": true, "family_count": true,
		"commercial_registry_info": true, "plate_number": true, "vehicle_type": true,
	}
	foundPhone := false
	for _, d := range env.Error.Details {
		f := d["field"]
		if legacy[f] {
			t.Errorf("legacy field %q must be ignored, but produced a violation: %v", f, d)
		}
		if f == "phone_number" {
			foundPhone = true
		}
	}
	if !foundPhone {
		t.Errorf("expected a phone_number violation, got %v", env.Error.Details)
	}
}

func TestOTPRequestValidation(t *testing.T) {
	ts := newTestServer(t)

	// Invalid phone → 400 before any limiter/DB access.
	resp, err := http.Post(ts.URL+"/api/v1/auth/otp/request", "application/json",
		strings.NewReader(`{"phone_number":"123"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if status, code, _ := decodeError(t, resp); status != 400 || code != "VALIDATION_ERROR" {
		t.Errorf("invalid phone: got %d/%s", status, code)
	}

	// Unsupported purpose → 400 before any limiter/DB access.
	resp2, err := http.Post(ts.URL+"/api/v1/auth/otp/request", "application/json",
		strings.NewReader(`{"phone_number":"771234567","purpose":"hacking"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if status, code, _ := decodeError(t, resp2); status != 400 || code != "VALIDATION_ERROR" {
		t.Errorf("invalid purpose: got %d/%s", status, code)
	}
}

func TestPasswordRecoveryValidation(t *testing.T) {
	ts := newTestServer(t)

	// Unknown-format phone → 400 VALIDATION_ERROR (no enumeration possible).
	resp, err := http.Post(ts.URL+"/api/v1/auth/forgot-password", "application/json",
		strings.NewReader(`{"phone_number":"not-a-phone"}`))
	if err != nil {
		t.Fatal(err)
	}
	status, code, _ := decodeError(t, resp)
	resp.Body.Close()
	if status != 400 || code != "VALIDATION_ERROR" {
		t.Errorf("forgot-password: got %d/%s", status, code)
	}

	// Missing reset token → 400 RESET_TOKEN_REQUIRED (checked before DB).
	resp2, err := http.Post(ts.URL+"/api/v1/auth/reset-password", "application/json",
		strings.NewReader(`{"new_password":"abcdef12"}`))
	if err != nil {
		t.Fatal(err)
	}
	status2, code2, _ := decodeError(t, resp2)
	resp2.Body.Close()
	if status2 != 400 || code2 != "RESET_TOKEN_REQUIRED" {
		t.Errorf("reset-password: got %d/%s", status2, code2)
	}

	// Empty refresh token → 400 REFRESH_TOKEN_REQUIRED (before DB lookup).
	resp3, err := http.Post(ts.URL+"/api/v1/auth/refresh", "application/json",
		strings.NewReader(`{"refresh_token":""}`))
	if err != nil {
		t.Fatal(err)
	}
	status3, code3, _ := decodeError(t, resp3)
	resp3.Body.Close()
	if status3 != 400 || code3 != "REFRESH_TOKEN_REQUIRED" {
		t.Errorf("refresh: got %d/%s", status3, code3)
	}
}

// --- CORS ---------------------------------------------------------------------

func TestCORSPreflight(t *testing.T) {
	ts := newTestServer(t)

	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("allow-origin = %q", got)
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Error("Authorization header not allowed")
	}

	// Unknown origin must be refused.
	req2, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/auth/login", nil)
	req2.Header.Set("Origin", "http://evil.example")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("evil origin status = %d, want 403", resp2.StatusCode)
	}
	if resp2.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("allow-origin set for a disallowed origin")
	}
}

func TestHealthEndpointShapeWithoutDB(t *testing.T) {
	// /health touches the database; with a nil DB the panic-recovery
	// middleware must answer with the standard envelope (never a stack trace).
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(raw), "goroutine") || strings.Contains(string(raw), ".go:") {
		t.Errorf("stack trace leaked to the client: %s", raw)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Errorf("non-JSON response: %s", raw)
	}
	if _, hasErr := body["error"]; !hasErr {
		t.Errorf("expected error envelope, got: %s", raw)
	}
}
