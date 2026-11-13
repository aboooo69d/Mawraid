package crypto_test

import (
	"bytes"
	"testing"

	"github.com/najm-x/nova/pkg/crypto"
)

// TestAES256GCMEncryption tests encryption, decryption, and security properties.
func TestAES256GCMEncryption(t *testing.T) {
	key32 := bytes.Repeat([]byte("k"), 32)
	plaintext := []byte("confidential citizen identity document")

	t.Run("Successful Encryption and Decryption Roundtrip", func(t *testing.T) {
		ciphertext, err := crypto.Encrypt(key32, plaintext)
		if err != nil {
			t.Fatalf("unexpected encryption error: %v", err)
		}

		if bytes.Contains(ciphertext, plaintext) {
			t.Fatal("ciphertext must not contain plaintext data")
		}

		decrypted, err := crypto.Decrypt(key32, ciphertext)
		if err != nil {
			t.Fatalf("unexpected decryption error: %v", err)
		}

		if !bytes.Equal(decrypted, plaintext) {
			t.Fatalf("expected decrypted text %q, got %q", plaintext, decrypted)
		}
	})

	t.Run("Fail with Invalid Key Size", func(t *testing.T) {
		shortKey := []byte("short-key-16b!!")
		_, err := crypto.Encrypt(shortKey, plaintext)
		if err == nil {
			t.Fatal("expected error when encrypting with non-32 byte key, got nil")
		}
	})

	t.Run("Fail with Wrong Decryption Key", func(t *testing.T) {
		ciphertext, err := crypto.Encrypt(key32, plaintext)
		if err != nil {
			t.Fatalf("unexpected encryption error: %v", err)
		}

		wrongKey := bytes.Repeat([]byte("w"), 32)
		_, err = crypto.Decrypt(wrongKey, ciphertext)
		if err == nil {
			t.Fatal("decryption must fail with wrong key")
		}
	})

	t.Run("Fail with Tampered Ciphertext", func(t *testing.T) {
		ciphertext, err := crypto.Encrypt(key32, plaintext)
		if err != nil {
			t.Fatalf("unexpected encryption error: %v", err)
		}

		// Flip a bit in the ciphertext payload
		ciphertext[len(ciphertext)-1] ^= 0xFF

		_, err = crypto.Decrypt(key32, ciphertext)
		if err == nil {
			t.Fatal("decryption must fail if ciphertext is tampered")
		}
	})

	t.Run("Fail with Truncated Ciphertext", func(t *testing.T) {
		truncated := []byte{1, 2, 3} // Too short to contain version + nonce + tag
		_, err := crypto.Decrypt(key32, truncated)
		if err == nil {
			t.Fatal("decryption must fail with truncated ciphertext")
		}
	})
}

