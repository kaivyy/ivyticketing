//go:build integration

package integration

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
)

func newTestRedis(t *testing.T) *goredis.Client {
	t.Helper()
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL not set")
	}
	opt, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse redis url: %v", err)
	}
	c := goredis.NewClient(opt)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestPhase9_RateLimiter_Concurrent(t *testing.T) {
	rc := newTestRedis(t)
	lim := ratelimit.New(rc)
	ctx := context.Background()
	key := "conc-test-" + t.Name()
	rc.Del(ctx, "ratelimit:"+key)

	const N = 50
	const limit = 10
	var allowed int64
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			ok, _ := lim.Allow(ctx, key, limit, time.Minute)
			if ok {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}
	wg.Wait()
	if allowed != limit {
		t.Fatalf("allowed = %d, want exactly %d", allowed, limit)
	}
	rc.Del(ctx, "ratelimit:"+key)
}
