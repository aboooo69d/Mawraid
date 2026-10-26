package config

import (
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
}

func NewConfig() *Config {
	godotenv.Load()
	return &Config{
		Cloudflare: CloudflareConfig{
			BucketName:      getEnv("BACKET_NAME", ""),
			AccountId:       getEnv("ACCOUNT_ID", ""),
			AccessKeyId:     getEnv("ACCESS_KEY_ID", ""),
			AccessKeySecret: getEnv("ACCESS_KEY_SECRET", ""),
		},
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
