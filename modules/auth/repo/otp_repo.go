package repo

import (
	"context"
	"time"

	redisKeys "github.com/najm-x/nova/pkg/constants/redis_keys"
	valueHashing "github.com/najm-x/nova/pkg/value_hashing"
	"github.com/redis/go-redis/v9"
)

type OtpRepo interface {
	Save(c context.Context, phone string, code string) error
	Verify(c context.Context, phone string, code string) bool
	GetIsCooldown(c context.Context, phone string) bool
	GetDailyLimit(c context.Context, phone string) int
	GetVerifyLimit(c context.Context, phone string) int
}

type otpRepo struct {
	redis *redis.Client
}

func NewOtpRepo(redis *redis.Client) OtpRepo {
	return &otpRepo{
		redis: redis,
	}
}

func (r *otpRepo) GetIsCooldown(c context.Context, phone string) bool {
	key := redisKeys.BuildOtpKey(redisKeys.OtpCooldown, phone)

	count, err := r.redis.Exists(c, key).Result()
	if err != nil {
		return false
	}
	return count > 0
}

func (r *otpRepo) GetDailyLimit(c context.Context, phone string) int {
	key := redisKeys.BuildOtpKey(redisKeys.OtpDailyLimit, phone)
	result, err := r.redis.Get(c, key).Int()
	if err != nil {
		return 0
	}
	return result
}

func (r *otpRepo) GetVerifyLimit(c context.Context, phone string) int {
	verifyLimitKey := redisKeys.BuildOtpKey(redisKeys.OtpVerifyLimit, phone)
	count, err := r.redis.Get(c, verifyLimitKey).Int()
	if err != nil {
		return 0
	}
	return count
}

func (r *otpRepo) Save(c context.Context, phone string, code string) error {
	coolDownKey := redisKeys.BuildOtpKey(redisKeys.OtpCooldown, phone)
	otpCodeKey := redisKeys.BuildOtpKey(redisKeys.OtpCode, phone)
	dailyLimitKey := redisKeys.BuildOtpKey(redisKeys.OtpDailyLimit, phone)
	verifyLimitKey := redisKeys.BuildOtpKey(redisKeys.OtpVerifyLimit, phone)

	hashedCode, err := valueHashing.HashValue(code)
	if err != nil {
		return err
	}

	pipe := r.redis.Pipeline()
	pipe.Set(c, otpCodeKey, hashedCode, 5*time.Minute)
	pipe.Set(c, coolDownKey, "1", 1*time.Minute)

	pipe.Del(c, verifyLimitKey)
	incrCmd := pipe.Incr(c, dailyLimitKey)

	if _, err := pipe.Exec(c); err != nil {
		return err
	}

	if incrCmd.Val() == 1 {
		r.redis.Expire(c, dailyLimitKey, 6*time.Hour)
	}

	return nil
}

func (r *otpRepo) Verify(c context.Context, phone string, code string) bool {
	otpCodeKey := redisKeys.BuildOtpKey(redisKeys.OtpCode, phone)
	otpVerifyLimitKey := redisKeys.BuildOtpKey(redisKeys.OtpVerifyLimit, phone)

	hashedCode, err := r.redis.Get(c, otpCodeKey).Result()
	if err != nil {
		return false
	}

	count, err := r.redis.Incr(c, otpVerifyLimitKey).Result()
	if err == nil && count == 1 {
		r.redis.Expire(c, otpVerifyLimitKey, 5*time.Minute)
	}

	isValid := valueHashing.VerifyValue(code, hashedCode)
	if isValid {

		r.redis.Del(c, otpCodeKey, otpVerifyLimitKey)
	}

	return isValid
}
