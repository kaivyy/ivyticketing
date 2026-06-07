package worker

import (
	"context"
	"log/slog"
	"time"
)

// Job is a unit of work run on each tick.
type Job func(ctx context.Context) error

// Runner invokes a Job immediately, then on every interval, until the context
// is cancelled. Job errors are logged, never fatal.
type Runner struct {
	name     string
	interval time.Duration
	job      Job
	log      *slog.Logger
}

func New(name string, interval time.Duration, job Job, log *slog.Logger) *Runner {
	return &Runner{name: name, interval: interval, job: job, log: log}
}

func (r *Runner) Run(ctx context.Context) {
	r.runOnce(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if r.log != nil {
				r.log.Info("worker stopped", "job", r.name)
			}
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Runner) runOnce(ctx context.Context) {
	if err := r.job(ctx); err != nil && r.log != nil {
		r.log.Error("worker job failed", "job", r.name, "error", err)
	}
}
