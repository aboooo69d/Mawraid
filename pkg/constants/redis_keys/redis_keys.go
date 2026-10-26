package redisKeys

import "fmt"

const (
	OtpCode        = "otp:code:%d"
	OtpDailyLimit  = "otp:daily_limit:%d"
	OtpCooldown    = "otp:cooldown:%d"
	OtpVerifyLimit = "otp:verify:%d"
)

func BuildOtpKey(key string, phone string) string {
	return fmt.Sprintf(key, phone)
}
