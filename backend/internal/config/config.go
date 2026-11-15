// Package config loads and validates the backend configuration from the
// environment (a local .env file may be used in development; production must
// inject real secrets through the deployment environment).
//
// SECURITY: this package never logs secret values.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every runtime setting of the Mawarid backend.
type Config struct {
	Env  string // development | test | production
	HTTP struct {
		Addr            string
		APIBasePath     string
		CORSOrigins     []string
		ReadTimeoutSec  int
		WriteTimeoutSec int
	}
	Database struct {
		URL          string
		AutoMigrate  bool
		MaxOpenConns int
	}
	Redis struct {
		Addr     string
		Password string
		DB       int
		Disabled bool
	}
	Security struct {
		JWTSecret         string
		JWTAccessTTL      time.Duration
		JWTRefreshTTL     time.Duration
		Argon2Time        uint32
		Argon2MemoryKiB   uint32
		Argon2Threads     uint8
		Argon2KeyLen      uint32
		Argon2SaltLen     uint32
		LoginMaxFailed    int
		LoginLockoutWin   time.Duration
		AllowedHostsProbe string // reserved
	}
	OTP struct {
		Length          int
		TTL             time.Duration
		ResendCooldown  time.Duration
		ResendMaxPerWin int
		MaxAttempts     int
		// DemoExposeCode gates DEMO_PHONE_VERIFICATION: when true, the freshly
		// generated OTP is echoed back in the registration / OTP-request
		// responses (field `dev_otp_code`) so a local demo can complete phone
		// verification without any SMS provider. The code stays random,
		// single-use and hashed at rest; TTL and attempt limits are unchanged.
		// Validate() REFUSES to start in production while this is enabled.
		DemoExposeCode bool
	}
	Upload struct {
		MaxSizeBytes int64
		Dir          string
	}
	SMS struct {
		Provider   string // dev | http
		FromLabel  string
		HTTPURL    string
		HTTPAPIKey string
		HTTPSender string
	}
	Log struct {
		Level string
	}
	// AdminBootstrap is used to create the first admin account on an empty DB.
	AdminBootstrap struct {
		Phone    string
		Name     string
		Password string
	}
}

// defaultJWTSecret is only acceptable for local development.
const defaultJWTSecret = "dev-only-insecure-change-me-000000000000000000000000000000"

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envDurationMin(key string, defMin int) time.Duration {
	return time.Duration(envInt(key, defMin)) * time.Minute
}

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	cfg := &Config{}
	cfg.Env = envStr("APP_ENV", "development")

	cfg.HTTP.Addr = envStr("HTTP_ADDR", ":8080")
	cfg.HTTP.APIBasePath = envStr("API_BASE_PATH", "/api/v1")
	if !strings.HasPrefix(cfg.HTTP.APIBasePath, "/") {
		cfg.HTTP.APIBasePath = "/" + cfg.HTTP.APIBasePath
	}
	origins := envStr("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.HTTP.CORSOrigins = append(cfg.HTTP.CORSOrigins, o)
		}
	}
	cfg.HTTP.ReadTimeoutSec = envInt("HTTP_READ_TIMEOUT_SEC", 15)
	cfg.HTTP.WriteTimeoutSec = envInt("HTTP_WRITE_TIMEOUT_SEC", 30)

	cfg.Database.URL = envStr("DATABASE_URL", "")
	cfg.Database.AutoMigrate = envBool("DATABASE_AUTO_MIGRATE", cfg.Env != "production")
	cfg.Database.MaxOpenConns = envInt("DATABASE_MAX_OPEN_CONNS", 10)

	cfg.Redis.Addr = envStr("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
	cfg.Redis.DB = envInt("REDIS_DB", 0)
	cfg.Redis.Disabled = envBool("REDIS_DISABLED", false)

	cfg.Security.JWTSecret = envStr("JWT_SECRET", defaultJWTSecret)
	cfg.Security.JWTAccessTTL = envDurationMin("JWT_ACCESS_TTL_MINUTES", 15)
	cfg.Security.JWTRefreshTTL = time.Duration(envInt("JWT_REFRESH_TTL_DAYS", 14)) * 24 * time.Hour
	cfg.Security.Argon2Time = uint32(envInt("ARGON2_TIME", 3))
	cfg.Security.Argon2MemoryKiB = uint32(envInt("ARGON2_MEMORY_KIB", 65536))
	cfg.Security.Argon2Threads = uint8(envInt("ARGON2_THREADS", 2))
	cfg.Security.Argon2KeyLen = uint32(envInt("ARGON2_KEY_LEN", 32))
	cfg.Security.Argon2SaltLen = uint32(envInt("ARGON2_SALT_LEN", 16))
	cfg.Security.LoginMaxFailed = envInt("LOGIN_MAX_FAILED_ATTEMPTS", 5)
	cfg.Security.LoginLockoutWin = envDurationMin("LOGIN_LOCKOUT_WINDOW_MINUTES", 15)

	cfg.OTP.Length = envInt("OTP_LENGTH", 6)
	// SRS NFR-SEC-04: OTP validity 2–5 minutes from request.
	cfg.OTP.TTL = envDurationMin("OTP_TTL_MINUTES", 5)
	cfg.OTP.ResendCooldown = time.Duration(envInt("OTP_RESEND_COOLDOWN_SECONDS", 60)) * time.Second
	cfg.OTP.ResendMaxPerWin = envInt("OTP_RESEND_MAX_PER_WINDOW", 5)
	// SRS NFR-SEC-03: max 3 OTP verification attempts per code/hour per number.
	cfg.OTP.MaxAttempts = envInt("OTP_MAX_ATTEMPTS", 3)
	// DEMO_PHONE_VERIFICATION — development/demo only (refused in production).
	cfg.OTP.DemoExposeCode = envBool("DEMO_PHONE_VERIFICATION", false)

	cfg.Upload.MaxSizeBytes = int64(envInt("MAX_UPLOAD_SIZE_BYTES", 5*1024*1024))
	cfg.Upload.Dir = envStr("UPLOAD_DIR", "./data/uploads")

	cfg.SMS.Provider = strings.ToLower(envStr("SMS_PROVIDER", "dev"))
	cfg.SMS.FromLabel = envStr("SMS_FROM_LABEL", "Mawarid")
	cfg.SMS.HTTPURL = os.Getenv("SMS_HTTP_URL")
	cfg.SMS.HTTPAPIKey = os.Getenv("SMS_HTTP_API_KEY")
	cfg.SMS.HTTPSender = os.Getenv("SMS_HTTP_SENDER_ID")

	cfg.Log.Level = strings.ToLower(envStr("LOG_LEVEL", "info"))

	cfg.AdminBootstrap.Phone = envStr("ADMIN_BOOTSTRAP_PHONE", "")
	cfg.AdminBootstrap.Name = envStr("ADMIN_BOOTSTRAP_NAME", "Mawarid Admin")
	cfg.AdminBootstrap.Password = os.Getenv("ADMIN_BOOTSTRAP_PASSWORD")

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate enforces environment-dependent invariants.
func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return errors.New("DATABASE_URL is required (copy .env.example to .env and configure it)")
	}
	if len(c.Security.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 characters")
	}
	if c.Env == "production" {
		if c.Security.JWTSecret == defaultJWTSecret {
			return errors.New("refusing to start in production with the default JWT_SECRET; set a strong secret")
		}
		if strings.Contains(c.Security.JWTSecret, "dev-only") {
			return errors.New("refusing to start in production with a development JWT_SECRET")
		}
		if c.SMS.Provider == "dev" {
			return errors.New("SMS_PROVIDER=dev is not allowed in production (no real SMS would be sent)")
		}
		if c.OTP.DemoExposeCode {
			// No production authentication bypass: the demo verification
			// mechanism must never be reachable from a production process.
			return errors.New("DEMO_PHONE_VERIFICATION=true is not allowed in production (development/demo phone verification only)")
		}
	}
	switch c.SMS.Provider {
	case "dev", "http":
	default:
		return fmt.Errorf("SMS_PROVIDER must be dev or http, got %q", c.SMS.Provider)
	}
	if c.SMS.Provider == "http" && c.SMS.HTTPURL == "" {
		return errors.New("SMS_PROVIDER=http requires SMS_HTTP_URL (provider credentials are supplied separately)")
	}
	if c.OTP.Length < 4 || c.OTP.Length > 10 {
		return errors.New("OTP_LENGTH must be between 4 and 10")
	}
	return nil
}

// IsProduction reports whether the process runs in production mode.
func (c *Config) IsProduction() bool { return c.Env == "production" }
