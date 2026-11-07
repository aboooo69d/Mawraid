package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/pkg/constants/errs_code"
	redisKeys "github.com/najm-x/nova/pkg/constants/redis_keys"
	"github.com/najm-x/nova/pkg/respons"
	"github.com/redis/go-redis/v9"
)

func IPRateLimitHandler(redisClient *redis.Client, limit int, expiration time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqCtx := ctx.Request.Context()
		userIP := ctx.ClientIP()
		rateLimitKey := redisKeys.BuildKey(redisKeys.IPRateLimit, userIP)

		rateLimit, err := redisClient.Incr(reqCtx, rateLimitKey).Result()
		if err != nil {
			ctx.Next()
			return
		}

		if rateLimit == 1 {
			redisClient.Expire(reqCtx, rateLimitKey, expiration)
		}

		if int(rateLimit) > limit {
			rateLimitErr := &respons.AppError{
				Status: http.StatusTooManyRequests,
				Code:   errs_code.ErrToManyRequest,
			}
			respons.Abort(ctx, rateLimitErr)
			return
		}

		ctx.Next()
	}
}
