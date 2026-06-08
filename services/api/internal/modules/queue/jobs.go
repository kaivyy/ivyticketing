package queue

import (
	"context"
	"time"
)

// ReleaseJob returns a worker Job that releases up to release_rate waiting users
// per running event each tick.
func (s *Service) ReleaseJob(window time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		events, err := s.repo.ListRunningEvents(ctx)
		if err != nil {
			return err
		}
		for _, eventID := range events {
			ctrl, err := s.repo.GetControl(ctx, eventID)
			if err != nil {
				continue
			}
			if ctrl.State != StateRunning || ctrl.ReleaseRate <= 0 {
				continue
			}
			_, _ = s.Release(ctx, eventID, int(ctrl.ReleaseRate), window)
		}
		return nil
	}
}

// AdmissionExpiryJob returns a worker Job that expires due admissions and requeues tokens.
func (s *Service) AdmissionExpiryJob(limit int) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		_, err := s.ExpireDue(ctx, limit)
		return err
	}
}
