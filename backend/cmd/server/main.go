// Command server runs the Mawarid Sprint 1 backend API.
//
// Startup order: config → logger → PostgreSQL (+ versioned migrations) →
// Redis limiter (in-memory fallback) → SMS adapter → services → first-admin
// bootstrap → HTTP server with graceful shutdown.
//
// SECURITY: every secret comes from the environment (see .env.example);
// nothing sensitive is printed to the logs.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	apihttp "mawarid/backend/internal/http"

	"gorm.io/gorm"

	"mawarid/backend/internal/auth"
	"mawarid/backend/internal/cache"
	"mawarid/backend/internal/config"
	"mawarid/backend/internal/database"
	"mawarid/backend/internal/models"
	"mawarid/backend/internal/service"
	"mawarid/backend/internal/sms"
	"mawarid/backend/internal/validation"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	// --- configuration ------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// --- logger -------------------------------------------------------------
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Log.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	log.Info("starting mawarid backend",
		"env", cfg.Env, "addr", cfg.HTTP.Addr, "base_path", cfg.HTTP.APIBasePath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- database -----------------------------------------------------------
	db, err := database.Connect(cfg.Database.URL, cfg.Database.MaxOpenConns, log)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}()

	if cfg.Database.AutoMigrate {
		if err := database.Migrate(db, log); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	} else {
		log.Warn("DATABASE_AUTO_MIGRATE is off — apply internal/database/migrations manually")
	}

	// --- rate limiter (Redis, with documented in-memory fallback) -----------
	var limiter cache.Limiter
	if cfg.Redis.Disabled {
		limiter = cache.NewMemoryLimiter()
		log.Warn("redis disabled — using the in-memory rate limiter (single instance only)")
	} else if rl, rerr := cache.NewRedisLimiter(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB); rerr != nil {
		limiter = cache.NewMemoryLimiter()
		log.Warn("redis unavailable — falling back to the in-memory rate limiter",
			"addr", cfg.Redis.Addr, "err", rerr)
	} else {
		limiter = rl
		log.Info("redis rate limiter connected", "addr", cfg.Redis.Addr)
	}
	defer limiter.Close() //nolint:errcheck

	// --- SMS adapter (dev log adapter unless a gateway is configured) -------
	sender, err := sms.New(sms.Config{
		Provider:   cfg.SMS.Provider,
		FromLabel:  cfg.SMS.FromLabel,
		HTTPURL:    cfg.SMS.HTTPURL,
		HTTPAPIKey: cfg.SMS.HTTPAPIKey,
		HTTPSender: cfg.SMS.HTTPSender,
	}, log)
	if err != nil {
		return fmt.Errorf("sms: %w", err)
	}
	log.Info("sms adapter ready",
		"adapter", sender.Name(), "real_delivery", sender.RealDelivery())
	if !sender.RealDelivery() {
		log.Warn("SMS_PROVIDER cannot deliver real messages — OTPs are rendered in the server log (development adapter)")
	}

	// --- services -----------------------------------------------------------
	hasher := auth.NewHasher(cfg.Security.Argon2Time, cfg.Security.Argon2MemoryKiB,
		cfg.Security.Argon2Threads, cfg.Security.Argon2KeyLen, cfg.Security.Argon2SaltLen)
	jwtm := auth.NewJWTManager(cfg.Security.JWTSecret, cfg.Security.JWTAccessTTL)
	svc := service.New(db, cfg, hasher, jwtm, sender, limiter, log)
	adminSvc := service.NewAdminService(svc)
	provSvc := service.NewProviderService(svc)

	// --- upload dir ---------------------------------------------------------
	if err := os.MkdirAll(cfg.Upload.Dir, 0o700); err != nil {
		return fmt.Errorf("upload dir: %w", err)
	}

	// --- first admin (only when explicitly configured) ----------------------
	if err := bootstrapAdmin(ctx, db, cfg, hasher, log); err != nil {
		return fmt.Errorf("admin bootstrap: %w", err)
	}

	// --- HTTP server --------------------------------------------------------
	api := apihttp.New(cfg, svc, adminSvc, provSvc, log)
	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       time.Duration(cfg.HTTP.ReadTimeoutSec) * time.Second,
		WriteTimeout:      time.Duration(cfg.HTTP.WriteTimeoutSec) * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTP.Addr)
		if e := server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			errCh <- e
		}
	}()

	select {
	case e := <-errCh:
		return fmt.Errorf("http: %w", e)
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("server stopped cleanly")
	return nil
}

// bootstrapAdmin creates the first admin account on an empty database when
// ADMIN_BOOTSTRAP_PHONE + ADMIN_BOOTSTRAP_PASSWORD are provided. It never runs
// when an admin already exists, never weakens validation, and never uses a
// hardcoded credential.
func bootstrapAdmin(ctx context.Context, db *gorm.DB, cfg *config.Config, hasher *auth.Hasher, log *slog.Logger) error {
	phone := strings.TrimSpace(cfg.AdminBootstrap.Phone)
	if phone == "" {
		log.Info("admin bootstrap skipped: ADMIN_BOOTSTRAP_PHONE not set")
		return nil
	}
	if strings.TrimSpace(cfg.AdminBootstrap.Password) == "" {
		return errors.New("ADMIN_BOOTSTRAP_PHONE is set but ADMIN_BOOTSTRAP_PASSWORD is empty (no default credentials are allowed)")
	}
	normalized := validation.NormalizePhone(phone)
	if normalized == "" {
		return fmt.Errorf("ADMIN_BOOTSTRAP_PHONE is not a valid Yemeni phone number")
	}

	var existing int64
	if err := db.WithContext(ctx).Model(&models.User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("roles.name = ?", models.RoleAdmin).Count(&existing).Error; err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if existing > 0 {
		return nil // already bootstrapped
	}

	var role models.Role
	if err := db.WithContext(ctx).Where("name = ?", models.RoleAdmin).First(&role).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load admin role: %w", err)
		}
		role = models.Role{Name: models.RoleAdmin}
		if err := db.WithContext(ctx).Create(&role).Error; err != nil {
			return fmt.Errorf("create admin role: %w", err)
		}
	}

	hash, err := hasher.Hash(cfg.AdminBootstrap.Password)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	now := time.Now().UTC()
	name := strings.TrimSpace(cfg.AdminBootstrap.Name)
	if name == "" {
		name = "Mawarid Admin"
	}
	user := models.User{
		RoleID:             role.ID,
		FullName:           name,
		PhoneNumber:        normalized,
		PasswordHash:       hash,
		IsActive:           true,
		Status:             models.AccountStatusActive,
		PhoneVerifiedAt:    &now,
		VerificationStatus: models.VerificationVerified,
	}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil // a concurrent bootstrap won the race
		}
		return fmt.Errorf("create admin user: %w", err)
	}
	// Never log the phone number or password — only the resulting id.
	log.Info("admin account bootstrapped", "user_id", user.ID)
	return nil
}
