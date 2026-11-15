package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter implements Limiter on top of Redis (INCR + EXPIRE fixed window).
type RedisLimiter struct {
	rdb *redis.Client
}

// NewRedisLimiter connects to Redis and verifies connectivity.
func NewRedisLimiter(addr, password string, db int) (*RedisLimiter, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return &RedisLimiter{rdb: rdb}, nil
}

// Hit implements Limiter. The window TTL is set on first increment; subsequent
// increments within the window share it.
func (r *RedisLimiter) Hit(ctx context.Context, key string, maxHits int, window time.Duration) (bool, time.Duration, error) {
	fullKey := "mawarid:rl:" + key
	n, err := r.rdb.Incr(ctx, fullKey).Result()
	if err != nil {
		return false, 0, err
	}
	if n == 1 {
		if err := r.rdb.Expire(ctx, fullKey, window).Err(); err != nil {
			return false, 0, err
		}
	}
	if n > int64(maxHits) {
		ttl, err := r.rdb.TTL(ctx, fullKey).Result()
		if err != nil {
			return false, 0, errors.Join(err, errors.New("ttl lookup failed"))
		}
		if ttl <= 0 {
			ttl = window
		}
		return false, ttl, nil
	}
	return true, 0, nil
}

// Reset implements Limiter.
func (r *RedisLimiter) Reset(ctx context.Context, key string) error {
	return r.rdb.Del(ctx, "mawarid:rl:"+key).Err()
}

// Peek implements Limiter.
func (r *RedisLimiter) Peek(ctx context.Context, key string) (int, time.Duration, error) {
	fullKey := "mawarid:rl:" + key
	n, err := r.rdb.Get(ctx, fullKey).Int()
	if err != nil {
		if err == redis.Nil {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	ttl, err := r.rdb.TTL(ctx, fullKey).Result()
	if err != nil && err != redis.Nil {
		return n, 0, err
	}
	if ttl < 0 {
		ttl = 0
	}
	return n, ttl, nil
}

// Close implements Limiter.
func (r *RedisLimiter) Close() error { return r.rdb.Close() }
