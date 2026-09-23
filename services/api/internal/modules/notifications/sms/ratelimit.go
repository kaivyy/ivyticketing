package sms

import (
	"sync"
	"time"
)

// RateLimiter checks if an action for a given key exceeds limits.
type RateLimiter interface {
	Allow(key string) bool
}

// MemoryRateLimiter implements a sliding window rate limiter per key.
type MemoryRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string][]time.Time
}

// NewMemoryRateLimiter creates a RateLimiter that allows up to `limit` requests per `window`.
func NewMemoryRateLimiter(limit int, window time.Duration) *MemoryRateLimiter {
	return &MemoryRateLimiter{
		limit:    limit,
		window:   window,
		requests: make(map[string][]time.Time),
	}
}

// Allow returns true if the request under key is permitted, false if rate limited.
func (rl *MemoryRateLimiter) Allow(key string) bool {
	if rl.limit <= 0 {
		return true
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	timestamps := rl.requests[key]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[key] = valid
		return false
	}

	valid = append(valid, now)
	rl.requests[key] = valid
	return true
}
