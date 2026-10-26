package config

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

var Env = NewConfig()

type CloudflareConfig struct {
	BucketName      string
	AccountId       string
	AccessKeyId     string
	AccessKeySecret string
}

type Config struct {
	Cloudflare CloudflareConfig
	// EncryptionKey is a hex-encoded 32-byte key (64 hex chars) used to
	// encrypt uploaded files with AES-256-GCM. Generate: openssl rand -hex 32
	EncryptionKey string
}

func NewConfig() *Config {
	godotenv.Load()
	return &Config{
		Cloudflare: CloudflareConfig{
			BucketName:      getEnv("BUCKET_NAME", ""),
			AccountId:       getEnv("ACCOUNT_ID", ""),
			AccessKeyId:     getEnv("ACCESS_KEY_ID", ""),
			AccessKeySecret: getEnv("ACCESS_KEY_SECRET", ""),
		},
		EncryptionKey: getEnv("ENCRYPTION_KEY", ""),
	}
}

// EncryptionKeyBytes decodes and validates the encryption key.
// Call it once at startup and stop the app if it returns an error.
func (c *Config) EncryptionKeyBytes() ([]byte, error) {
	key, err := hex.DecodeString(c.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be hex encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}
	return key, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
