package auth

import (
	"strings"
	"testing"
)

// fastHasher keeps the suite quick while exercising the real Argon2id path.
func fastHasher() *Hasher {
	return NewHasher(1, 8*1024, 1, 32, 16)
}

func TestHashAndVerify(t *testing.T) {
	h := fastHasher()
	hash, err := h.Hash("Sup3rSecret!")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=") {
		t.Errorf("unexpected PHC format: %s", hash)
	}
	if !h.Verify("Sup3rSecret!", hash) {
		t.Error("correct password rejected")
	}
	if h.Verify("WrongPassword1", hash) {
		t.Error("wrong password accepted")
	}
}

func TestHashesAreSalted(t *testing.T) {
	h := fastHasher()
	a, _ := h.Hash("same-password-1")
	b, _ := h.Hash("same-password-1")
	if a == b {
		t.Error("two hashes of the same password are identical (missing salt)")
	}
	if !h.Verify("same-password-1", a) || !h.Verify("same-password-1", b) {
		t.Error("both salted hashes must verify")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	h := fastHasher()
	for _, bad := range []string{
		"", "not-a-hash", "$2y$10$abcdefghijklmnopqrstuv",
		"$argon2id$v=19$m=65536,t=3,p=2$bad-base64!!$zz",
		"$bcrypt$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA",
	} {
		if h.Verify("anything", bad) {
			t.Errorf("malformed hash accepted: %q", bad)
		}
	}
}

func TestRandomDigitsLengthAndCharset(t *testing.T) {
	for _, n := range []int{4, 6, 8} {
		code, err := RandomDigits(n)
		if err != nil {
			t.Fatalf("RandomDigits(%d): %v", n, err)
		}
		if len(code) != n {
			t.Errorf("length = %d, want %d (%q)", len(code), n, code)
		}
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("non-digit %q in code %q", c, code)
			}
		}
	}
	if _, err := RandomDigits(0); err == nil {
		t.Error("length 0 accepted")
	}
	if _, err := RandomDigits(11); err == nil {
		t.Error("length 11 accepted")
	}
}

func TestRandomTokenEntropy(t *testing.T) {
	a, err := RandomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RandomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two tokens are identical")
	}
	if len(a) < 40 {
		t.Errorf("token too short: %d chars", len(a))
	}
}
