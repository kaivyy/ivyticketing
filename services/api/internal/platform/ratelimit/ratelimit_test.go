package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testClient(t *testing.T) *redis.Client {
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL not set")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	c := redis.NewClient(opt)
	t.Cleanup(func() { c.Close() })
	return c
}

func TestAllow_WithinAndOverLimit(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	l := New(c)
	key := "rl-test-" + t.Name()
	c.Del(ctx, "ratelimit:"+key)

	for i := 0; i < 3; i++ {
		ok, err := l.Allow(ctx, key, 3, time.Minute)
		if err != nil || !ok {
			t.Fatalf("call %d should be allowed: ok=%v err=%v", i, ok, err)
		}
	}
	ok, err := l.Allow(ctx, key, 3, time.Minute)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatal("4th call over limit should be denied")
	}
	c.Del(ctx, "ratelimit:"+key)
}
