package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	plain := []byte("secret image bytes")

	enc, err := Encrypt(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(enc, plain) {
		t.Fatal("ciphertext must not contain plaintext")
	}

	dec, err := Decrypt(key, enc)
	if err != nil || !bytes.Equal(dec, plain) {
		t.Fatalf("round trip failed: %v", err)
	}

	// wrong key and tampered data must fail
	if _, err := Decrypt(bytes.Repeat([]byte{8}, 32), enc); err == nil {
		t.Fatal("wrong key must fail")
	}
	enc[len(enc)-1] ^= 1
	if _, err := Decrypt(key, enc); err == nil {
		t.Fatal("tampered data must fail")
	}
}
