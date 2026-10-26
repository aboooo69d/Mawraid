package database

import (
	"context"
	"fmt"
	"time"

	"github.com/najm-x/nova/config"
	"github.com/redis/go-redis/v9"
)

// NewRedis initializes and validates a Redis client connection.
func NewRedis(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis at %s: %w", cfg.Addr(), err)
	}

	return rdb, nil
}
