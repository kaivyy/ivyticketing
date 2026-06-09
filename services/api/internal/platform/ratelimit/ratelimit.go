// Package ratelimit provides a Redis-backed fixed-window token bucket.
// Fail-open: on Redis error, Allow returns true (never block normal users on infra failure).
package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	c *redis.Client
}

func New(c *redis.Client) *Limiter { return &Limiter{c: c} }

// Allow increments the counter for key and returns false once it exceeds limit
// within the window. Fail-open on Redis errors.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if l == nil || l.c == nil {
		return true, nil
	}
	rk := "ratelimit:" + key
	n, err := l.c.Incr(ctx, rk).Result()
	if err != nil {
		return true, nil // fail-open
	}
	if n == 1 {
		_ = l.c.Expire(ctx, rk, window).Err()
	}
	return n <= int64(limit), nil
}

// IncrExpire increments key and sets expiry on first increment. Returns the
// new counter value. Fail-open: returns (0, err) on Redis error.
func (l *Limiter) IncrExpire(ctx context.Context, key string, window time.Duration) (int64, error) {
	if l == nil || l.c == nil {
		return 0, nil
	}
	n, err := l.c.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_ = l.c.Expire(ctx, key, window).Err()
	}
	return n, nil
}
