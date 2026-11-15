package config

import (
	"strings"
	"testing"
)

// validProdBase returns a configuration that would otherwise be valid in
// production (strong JWT secret, real SMS gateway) so the tests below isolate
// the DEMO_PHONE_VERIFICATION rule.
func validProdBase() *Config {
	c := &Config{}
	c.Env = "production"
	c.Database.URL = "postgres://example"
	c.Security.JWTSecret = "strong-secret-at-least-32-chars-000000"
	c.SMS.Provider = "http"
	c.SMS.HTTPURL = "https://sms.example.test/send"
	c.OTP.Length = 6
	return c
}

// Sprint 1 MVP guard: the gated demo phone-verification mechanism must be
// refused outright in production — no production authentication bypass.
func TestValidateRefusesDemoPhoneVerificationInProduction(t *testing.T) {
	c := validProdBase()
	c.OTP.DemoExposeCode = true
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error for DEMO_PHONE_VERIFICATION=true in production")
	}
	if !strings.Contains(err.Error(), "DEMO_PHONE_VERIFICATION") {
		t.Errorf("error %q does not mention DEMO_PHONE_VERIFICATION", err)
	}
}

// The same configuration must remain usable in development and test (that is
// what powers the local SMS-free demo flow).
func TestValidateAllowsDemoPhoneVerificationOutsideProduction(t *testing.T) {
	for _, env := range []string{"development", "test"} {
		c := validProdBase()
		c.Env = env
		c.SMS.Provider = "dev"
		c.OTP.DemoExposeCode = true
		if err := c.Validate(); err != nil {
			t.Errorf("Validate() in %s with demo flag = %v, want nil", env, err)
		}
	}
}

// The flag defaults to off — production verification stays mandatory-by-SMS.
func TestDemoPhoneVerificationDefaultsOff(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_SECRET", "strong-secret-at-least-32-chars-000000")
	t.Setenv("APP_ENV", "development")
	t.Setenv("DEMO_PHONE_VERIFICATION", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.OTP.DemoExposeCode {
		t.Error("DEMO_PHONE_VERIFICATION default = true, want false")
	}
}

// Load() must parse the flag (and still refuse a production boot with it).
func TestLoadParsesDemoPhoneVerification(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_SECRET", "strong-secret-at-least-32-chars-000000")
	t.Setenv("APP_ENV", "development")
	t.Setenv("DEMO_PHONE_VERIFICATION", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if !cfg.OTP.DemoExposeCode {
		t.Error("DEMO_PHONE_VERIFICATION=true was not applied")
	}

	// Production boot with the same flag must fail fast.
	t.Setenv("APP_ENV", "production")
	t.Setenv("SMS_PROVIDER", "http")
	t.Setenv("SMS_HTTP_URL", "https://sms.example.test/send")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DEMO_PHONE_VERIFICATION") {
		t.Errorf("Load() in production with demo flag = %v, want DEMO_PHONE_VERIFICATION refusal", err)
	}
}
