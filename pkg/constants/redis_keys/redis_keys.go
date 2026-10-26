package redisKeys

import "fmt"

const (
	OtpCode        = "otp:code:%s"
	OtpDailyLimit  = "otp:daily_limit:%s"
	OtpCooldown    = "otp:cooldown:%s"
	OtpVerifyLimit = "otp:verify:%s"
)

func BuildOtpKey(key string, phone string) string {
	return fmt.Sprintf(key, phone)
}
