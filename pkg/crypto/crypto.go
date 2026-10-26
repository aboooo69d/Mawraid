package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// keyVersion is written as the first byte of every encrypted blob so the key
// can be rotated later without breaking old files.
const keyVersion byte = 1

var ErrInvalidCiphertext = errors.New("invalid ciphertext")

// Encrypt encrypts plain with AES-256-GCM (key must be 32 bytes).
// Output layout: [version(1)] [nonce(12)] [ciphertext+tag].
func Encrypt(key, plain []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	out := make([]byte, 0, 1+len(nonce)+len(plain)+gcm.Overhead())
	out = append(out, keyVersion)
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plain, nil), nil
}

// Decrypt reverses Encrypt. It fails if the data was modified or the key is wrong.
func Decrypt(key, data []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}

	ns := gcm.NonceSize()
	if len(data) < 1+ns+gcm.Overhead() || data[0] != keyVersion {
		return nil, ErrInvalidCiphertext
	}

	nonce := data[1 : 1+ns]
	return gcm.Open(nil, nonce, data[1+ns:], nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
