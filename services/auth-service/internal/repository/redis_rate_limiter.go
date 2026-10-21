package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRateLimiter struct {
	rdb *redis.Client
}

func NewRedisRateLimiter(rdb *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{rdb: rdb}
}

// CheckAndRecordOTPRequest enforces max 5 OTP requests per hour per phone.
func (l *RedisRateLimiter) CheckAndRecordOTPRequest(ctx context.Context, phone string) (bool, int, error) {
	key := fmt.Sprintf("ratelimit:otp:%s", phone)

	// Check existing count first
	count, err := l.rdb.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		return false, 0, err
	}

	if count >= 5 {
		ttl, _ := l.rdb.TTL(ctx, key).Result()
		retryAfter := int(ttl.Seconds())
		if retryAfter <= 0 {
			retryAfter = 3600
		}
		return false, retryAfter, nil
	}

	// Increment count
	newCount, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, err
	}

	// If this was the first request in the window, set 1-hour expiration
	if newCount == 1 {
		_ = l.rdb.Expire(ctx, key, 1*time.Hour).Err()
	}

	return true, 0, nil
}
