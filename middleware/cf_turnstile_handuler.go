package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/config"
	"github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

const (
	turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	maxTokenLength     = 2048
)

type TurnstileResponse struct {
	Success    bool     `json:"success"`
	Hostname   string   `json:"hostname"`
	Action     string   `json:"action"`
	ErrorCodes []string `json:"error-codes"`
}

func CfTurnstileHanduler(cfg *config.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqErr := &respons.AppError{
			Status:  http.StatusForbidden,
			Code:    errs_code.ErrTurnstileToken,
			Message: "invalid or missing turnstile token",
		}

		// 1. Get token
		token := ctx.GetHeader("cf-turnstile-response")

		if token == "" || len(token) > maxTokenLength {
			respons.Abort(ctx, reqErr)
			return
		}

		// 2. Validate configuration
		if cfg.Turnstile.SecretKey == "" || len(cfg.Turnstile.Hostnames) == 0 {
			respons.Abort(ctx, reqErr)
			return
		}

		// 3. Prepare verification request
		form := url.Values{}
		form.Set("secret", cfg.Turnstile.SecretKey)
		form.Set("response", token)

		if ip := net.ParseIP(ctx.ClientIP()); ip != nil {
			form.Set("remoteip", ip.String())
		}

		reqCtx, cancel := context.WithTimeout(
			ctx.Request.Context(),
			10*time.Second,
		)
		defer cancel()

		req, err := http.NewRequestWithContext(
			reqCtx,
			http.MethodPost,
			turnstileVerifyURL,
			strings.NewReader(form.Encode()),
		)

		if err != nil {
			respons.Abort(ctx, reqErr)
			return
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		// 4. Send request to Cloudflare
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		resp, err := client.Do(req)
		if err != nil {
			respons.Abort(ctx, reqErr)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respons.Abort(ctx, reqErr)
			return
		}

		// 5. Decode response
		var result TurnstileResponse

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			respons.Abort(ctx, reqErr)
			return
		}

		// 6. Validate response
		if !result.Success {
			respons.Abort(ctx, reqErr)
			return
		}

		// 7. To DO Validate action
		_ = "subscribe"
		//

		// 8. Validate hostname
		validHostname := false

		for _, hostname := range cfg.Turnstile.Hostnames {
			if strings.EqualFold(
				strings.TrimSpace(hostname),
				result.Hostname,
			) {
				validHostname = true
				break
			}
		}

		if !validHostname {
			respons.Abort(ctx, reqErr)
			return
		}

		// Verification successful
		ctx.Next()
	}
}
