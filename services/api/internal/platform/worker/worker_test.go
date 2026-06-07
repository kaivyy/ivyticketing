package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunner_TicksUntilContextCancelled(t *testing.T) {
	var calls int64
	job := func(ctx context.Context) error {
		atomic.AddInt64(&calls, 1)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := New("test", 10*time.Millisecond, job, nil)

	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()

	time.Sleep(55 * time.Millisecond)
	cancel()
	<-done

	if got := atomic.LoadInt64(&calls); got < 3 {
		t.Errorf("expected at least 3 ticks, got %d", got)
	}
}

func TestRunner_RunsImmediatelyThenTicks(t *testing.T) {
	var calls int64
	job := func(ctx context.Context) error {
		atomic.AddInt64(&calls, 1)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := New("test", time.Hour, job, nil) // long interval; only the immediate run should fire

	go r.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()

	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Errorf("expected exactly 1 immediate run, got %d", got)
	}
}
