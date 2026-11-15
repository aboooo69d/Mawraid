// Package auth implements the security primitives of the Mawarid backend:
// Argon2id password hashing, JWT access tokens, and refresh sessions.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Hasher produces and verifies argon2id password hashes in the standard
// PHC string format:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
type Hasher struct {
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
	saltLen uint32
}

// NewHasher builds a hasher with the given Argon2id parameters.
func NewHasher(time, memoryKiB uint32, threads uint8, keyLen, saltLen uint32) *Hasher {
	if time == 0 {
		time = 3
	}
	if memoryKiB == 0 {
		memoryKiB = 64 * 1024
	}
	if threads == 0 {
		threads = 2
	}
	if keyLen == 0 {
		keyLen = 32
	}
	if saltLen == 0 {
		saltLen = 16
	}
	return &Hasher{time: time, memory: memoryKiB, threads: threads, keyLen: keyLen, saltLen: saltLen}
}

// Hash returns the PHC-encoded argon2id hash of the password.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	sum := argon2.IDKey([]byte(password), salt, h.time, h.memory, h.threads, h.keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.memory, h.time, h.threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

// Verify reports whether password matches the stored PHC hash. The comparison
// is constant-time. Malformed hashes always fail verification.
func (h *Hasher) Verify(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	if version != argon2.Version {
		return false
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// --- token / secret helpers -------------------------------------------------

// RandomToken returns a URL-safe random token with n bytes of entropy.
func RandomToken(n int) (string, error) {
	if n <= 0 {
		n = 32
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// RandomDigits returns a numeric one-time code of exactly length digits.
func RandomDigits(length int) (string, error) {
	if length <= 0 || length > 10 {
		return "", errors.New("otp length must be between 1 and 10")
	}
	// Uniform mapping over [0, 10^length) via crypto/rand.
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	s := fmt.Sprintf("%0*d", length, n.Int64())
	return s, nil
}
