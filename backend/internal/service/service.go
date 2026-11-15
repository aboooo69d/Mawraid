// Package service implements the business logic of the Mawarid backend.
// Handlers in internal/http stay thin; all state changes happen here.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"regexp"
	"time"

	"gorm.io/gorm"

	"mawarid/backend/internal/auth"
	"mawarid/backend/internal/cache"
	"mawarid/backend/internal/config"
	"mawarid/backend/internal/sms"
)

// Service bundles the dependencies of every domain service.
type Service struct {
	DB      *gorm.DB
	Cfg     *config.Config
	Hasher  *auth.Hasher
	JWT     *auth.JWTManager
	SMS     sms.Sender
	Limiter cache.Limiter
	Log     *slog.Logger
	// Now allows tests to control the clock.
	Now func() time.Time
}

// New builds a Service.
func New(db *gorm.DB, cfg *config.Config, hasher *auth.Hasher, jwtm *auth.JWTManager, sender sms.Sender, limiter cache.Limiter, log *slog.Logger) *Service {
	return &Service{
		DB: db, Cfg: cfg, Hasher: hasher, JWT: jwtm, SMS: sender, Limiter: limiter, Log: log,
		Now: time.Now,
	}
}

// sha256Hex returns the lowercase hex SHA-256 of s.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ptr returns a pointer to v (small helper).
func ptr[T any](v T) *T { return &v }

// audit writes an audit-log row. details must never contain sensitive
// identity data (SRS NFR-SEC-02: admin-only visibility, no PII leakage).
func (s *Service) audit(ctx context.Context, actorID, targetID *uint, action, details string, ip *string) {
	entry := map[string]any{"action": action}
	if details != "" {
		entry["details"] = details
	}
	if err := s.DB.WithContext(ctx).Exec(
		`INSERT INTO audit_logs (actor_user_id, target_user_id, action, details, ip, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		actorID, targetID, action, marshalJSON(details), ip, s.Now(),
	).Error; err != nil {
		s.Log.ErrorContext(ctx, "audit log write failed", "action", action, "err", err)
	}
}

func marshalJSON(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// randomToken returns a URL-safe random token (32 bytes of entropy).
func randomToken() (string, error) {
	return auth.RandomToken(32)
}

// validTimeRe reports whether v matches the "HH:MM" 24h format.
var validTimeRe = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// validTimeReFn keeps call sites short.
func validTimeReFn(v string) bool { return validTimeRe.MatchString(v) }
