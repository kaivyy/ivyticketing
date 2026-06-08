package queue

import (
	"context"
	"os"
	"testing"

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

func TestWaitingAddRankRange(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	a := New(c)
	ev := "evt-test-" + t.Name()
	t.Cleanup(func() { c.Del(ctx, waitingKey(ev), allowedKey(ev)) })

	a.AddWaiting(ctx, ev, "u1", 100)
	a.AddWaiting(ctx, ev, "u2", 200)

	rank, err := a.WaitingRank(ctx, ev, "u2")
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if rank != 1 {
		t.Fatalf("u2 rank = %d, want 1", rank)
	}
	members, err := a.WaitingRangeN(ctx, ev, 10)
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if len(members) != 2 || members[0] != "u1" {
		t.Fatalf("range = %v, want [u1 u2]", members)
	}
}

func TestMoveToAllowed(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	a := New(c)
	ev := "evt-test-" + t.Name()
	t.Cleanup(func() { c.Del(ctx, waitingKey(ev), allowedKey(ev)) })

	a.AddWaiting(ctx, ev, "u1", 100)
	if err := a.MoveToAllowed(ctx, ev, "u1", 9999); err != nil {
		t.Fatalf("move: %v", err)
	}
	cnt, _ := a.WaitingCount(ctx, ev)
	if cnt != 0 {
		t.Fatalf("waiting count = %d, want 0", cnt)
	}
}
