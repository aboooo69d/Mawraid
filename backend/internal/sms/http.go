package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// HTTPSender is a generic JSON HTTP SMS-gateway adapter.
//
// ⚠️ NOT VERIFIED AGAINST A REAL PROVIDER. The real provider's API contract
// will be supplied separately; adapt the payload mapping in Send() to that
// contract before enabling SMS_PROVIDER=http. Until then, treat this as the
// integration seam, not as a working delivery path.
type HTTPSender struct {
	URL    string
	APIKey string
	Sender string
	From   string
	Client *http.Client
}

// Name implements Sender.
func (h *HTTPSender) Name() string { return "http" }

// RealDelivery implements Sender — the http adapter does transmit requests
// (subject to the real gateway being configured correctly).
func (h *HTTPSender) RealDelivery() bool { return true }

type httpPayload struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Message string `json:"message"`
}

// Send implements Sender with a generic {"from","to","message"} JSON body and
// an X-Api-Key authentication header — the most common gateway convention.
func (h *HTTPSender) Send(ctx context.Context, phone, message string) error {
	body, err := json.Marshal(httpPayload{From: h.Sender, To: phone, Message: message})
	if err != nil {
		return fmt.Errorf("sms http: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sms http: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if h.APIKey != "" {
		req.Header.Set("X-Api-Key", h.APIKey)
	}
	resp, err := h.Client.Do(req)
	if err != nil {
		return fmt.Errorf("sms http: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sms http: gateway returned status %d", resp.StatusCode)
	}
	return nil
}
