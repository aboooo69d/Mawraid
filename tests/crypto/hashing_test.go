package crypto_test

import (
	"testing"

	valueHashing "github.com/najm-x/nova/pkg/value_hashing"
)

// TestPasswordHashing tests password hashing generation and verification.
func TestPasswordHashing(t *testing.T) {
	password := "SecureP@ssw0rd!2026"

	t.Run("Generate Valid Hash and Verify", func(t *testing.T) {
		hashed, err := valueHashing.HashValue(password)
		if err != nil {
			t.Fatalf("unexpected hashing error: %v", err)
		}

		if len(hashed) == 0 {
			t.Fatal("hashed password must not be empty")
		}

		if !valueHashing.VerifyValue(password, hashed) {
			t.Fatal("password verification failed for matching password")
		}
	})

	t.Run("Fail with Incorrect Password", func(t *testing.T) {
		hashed, err := valueHashing.HashValue(password)
		if err != nil {
			t.Fatalf("unexpected hashing error: %v", err)
		}

		wrongPassword := "WrongPassword123"
		if valueHashing.VerifyValue(wrongPassword, hashed) {
			t.Fatal("password verification succeeded for wrong password")
		}
	})

	t.Run("Fail with Empty Password Against Valid Hash", func(t *testing.T) {
		hashed, err := valueHashing.HashValue(password)
		if err != nil {
			t.Fatalf("unexpected hashing error: %v", err)
		}

		if valueHashing.VerifyValue("", hashed) {
			t.Fatal("empty password should not match a valid password hash")
		}
	})

	t.Run("Distinct Hashes for Identical Passwords (Salted)", func(t *testing.T) {
		hash1, err := valueHashing.HashValue(password)
		if err != nil {
			t.Fatal(err)
		}

		hash2, err := valueHashing.HashValue(password)
		if err != nil {
			t.Fatal(err)
		}

		if hash1 == hash2 {
			t.Fatal("bcrypt hashes should differ due to distinct random salts")
		}
	})
}

