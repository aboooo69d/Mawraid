// Package sms provides the SMS integration layer.
//
// SECURITY: SMS credentials live only in server-side environment variables and
// are NEVER exposed to the frontend or written to logs.
//
// HONESTY NOTE: the real SMS provider API will be supplied separately. Until
// it is configured and verified end-to-end, the backend uses the "dev"
// adapter, which renders OTP codes into the server log instead of sending
// real SMS. The "http" adapter implements a generic JSON gateway contract and
// is NOT verified against any real provider — do not claim real delivery.
package sms

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Sender delivers a single SMS message to a phone number.
type Sender interface {
	// Send transmits message to phone (E.164-ish local format).
	// Implementations must not log credentials.
	Send(ctx context.Context, phone, message string) error
	// Name identifies the adapter ("dev" | "http").
	Name() string
	// RealDelivery reports whether messages actually leave the machine.
	RealDelivery() bool
}

// ErrNotConfigured is returned by adapters that cannot really deliver SMS.
var ErrNotConfigured = errors.New("SMS provider not configured for real delivery")

// Config carries the adapter selection settings.
type Config struct {
	Provider   string // dev | http
	FromLabel  string
	HTTPURL    string
	HTTPAPIKey string
	HTTPSender string
}

// New builds the configured SMS sender.
func New(cfg Config, logger *slog.Logger) (Sender, error) {
	switch cfg.Provider {
	case "dev", "":
		return &DevSender{Logger: logger, From: cfg.FromLabel}, nil
	case "http":
		if cfg.HTTPURL == "" {
			return nil, errors.New("sms: http adapter requires SMS_HTTP_URL")
		}
		return &HTTPSender{
			URL:    cfg.HTTPURL,
			APIKey: cfg.HTTPAPIKey,
			Sender: cfg.HTTPSender,
			From:   cfg.FromLabel,
			Client: &http.Client{Timeout: 5 * time.Second},
		}, nil
	default:
		return nil, fmt.Errorf("sms: unknown provider %q", cfg.Provider)
	}
}
