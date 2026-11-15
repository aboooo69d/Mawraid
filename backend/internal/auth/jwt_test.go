package auth

import (
	"testing"
	"time"
)

func TestAccessTokenIssueAndVerify(t *testing.T) {
	m := NewJWTManager("unit-test-secret-0123456789abcdef0123456789", 15*time.Minute)
	tok, exp, err := m.IssueAccessToken(42, "consumer")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if time.Until(exp) <= 14*time.Minute {
		t.Errorf("expiry ~15m expected, got %s", time.Until(exp))
	}
	claims, err := m.VerifyAccessToken(tok)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if claims.UserID != 42 || claims.Role != "consumer" || claims.TokenUse != "access" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestAccessTokenRejectsWrongSecret(t *testing.T) {
	a := NewJWTManager("secret-a-0123456789abcdefghij0123456789", time.Minute)
	b := NewJWTManager("secret-b-0123456789abcdefghij0123456789", time.Minute)
	tok, _, _ := a.IssueAccessToken(1, "admin")
	if _, err := b.VerifyAccessToken(tok); err == nil {
		t.Error("token verified with a different secret")
	}
}

func TestAccessTokenRejectsExpired(t *testing.T) {
	m := NewJWTManager("unit-test-secret-0123456789abcdef0123456789", -time.Minute)
	tok, _, err := m.IssueAccessToken(7, "provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.VerifyAccessToken(tok); err == nil {
		t.Error("expired token accepted")
	}
}

func TestAccessTokenRejectsGarbage(t *testing.T) {
	m := NewJWTManager("unit-test-secret-0123456789abcdef0123456789", time.Minute)
	for _, bad := range []string{"", "abc", "eyJhbGciOiJub25lIn0.e30."} {
		if _, err := m.VerifyAccessToken(bad); err == nil {
			t.Errorf("garbage token accepted: %q", bad)
		}
	}
}

func TestPasswordResetTokenLifecycle(t *testing.T) {
	m := NewJWTManager("unit-test-secret-0123456789abcdef0123456789", 15*time.Minute)

	// An access token must NOT be usable as a reset token.
	access, _, _ := m.IssueAccessToken(5, "consumer")
	if _, err := m.VerifyPasswordResetToken(access); err == nil {
		t.Error("access token accepted as a reset token")
	}

	tok, _, err := m.IssuePasswordResetToken(5, time.Now().Add(15*time.Minute))
	if err != nil {
		t.Fatalf("IssuePasswordResetToken: %v", err)
	}
	uid, err := m.VerifyPasswordResetToken(tok)
	if err != nil {
		t.Fatalf("VerifyPasswordResetToken: %v", err)
	}
	if uid != 5 {
		t.Errorf("uid = %d, want 5", uid)
	}

	// Expired reset token must be rejected.
	old, _, _ := m.IssuePasswordResetToken(5, time.Now().Add(-time.Minute))
	if _, err := m.VerifyPasswordResetToken(old); err == nil {
		t.Error("expired reset token accepted")
	}
}
