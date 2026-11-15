package sms

import (
	"context"
	"log/slog"
)

// DevSender is the DEVELOPMENT adapter. It does NOT send real SMS. Messages
// (including OTP codes) are written to the server log only, so local
// development and integration tests can complete the OTP flow.
//
// It is refused in production by config validation.
type DevSender struct {
	Logger *slog.Logger
	From   string
}

// Name implements Sender.
func (d *DevSender) Name() string { return "dev" }

// RealDelivery implements Sender — always false.
func (d *DevSender) RealDelivery() bool { return false }

// Send implements Sender. The OTP is logged server-side ONLY.
func (d *DevSender) Send(_ context.Context, phone, message string) error {
	// Phone numbers are masked in logs to avoid leaking personal data.
	d.Logger.Warn("SMS dev adapter: message NOT sent to any real network",
		"adapter", "dev",
		"phone_masked", maskPhone(phone),
		"from", d.From,
		"message", message,
	)
	return nil
}

// maskPhone keeps the last 3 digits only.
func maskPhone(phone string) string {
	n := len(phone)
	if n <= 3 {
		return "***"
	}
	return phone[:n-3] + "***"
}
