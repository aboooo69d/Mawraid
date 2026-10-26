package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var Env = NewConfig()

type ServerConfig struct {
	Port string
	Mode string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		d.Host, d.User, d.Password, d.DBName, d.Port, d.SSLMode)
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// Addr returns the redis address host:port.
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

type CloudflareConfig struct {
	BucketName      string
	AccountId       string
	AccessKeyId     string
	AccessKeySecret string
}

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	Cloudflare CloudflareConfig

	// EncryptionKey is a hex-encoded 32-byte key (64 hex chars) used to
	// encrypt uploaded files with AES-256-GCM. Generate: openssl rand -hex 32
	EncryptionKey string
}

func NewConfig() *Config {
	_ = godotenv.Load()

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "nova"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
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
// Returns an error if the key is missing or not 32 bytes (64 hex characters).
func (c *Config) EncryptionKeyBytes() ([]byte, error) {
	if c.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required in configuration/env")
	}
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
