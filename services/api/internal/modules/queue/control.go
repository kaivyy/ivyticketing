package queue

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

func (s *Service) Pause(ctx context.Context, eventID uuid.UUID) error {
	if err := s.ensureControl(ctx, eventID); err != nil {
		return err
	}
	err := s.repo.SetState(ctx, db.SetQueueStateParams{EventID: eventID, State: StatePaused})
	if err == nil && s.audit != nil {
		s.audit.Record(ctx, audit.Entry{Action: "QUEUE_PAUSED", TargetType: "event", TargetID: eventID.String()})
	}
	return err
}

func (s *Service) Resume(ctx context.Context, eventID uuid.UUID) error {
	if err := s.ensureControl(ctx, eventID); err != nil {
		return err
	}
	err := s.repo.SetState(ctx, db.SetQueueStateParams{EventID: eventID, State: StateRunning})
	if err == nil && s.audit != nil {
		s.audit.Record(ctx, audit.Entry{Action: "QUEUE_RESUMED", TargetType: "event", TargetID: eventID.String()})
	}
	return err
}

func (s *Service) SetRate(ctx context.Context, eventID uuid.UUID, rate int32) error {
	if err := s.ensureControl(ctx, eventID); err != nil {
		return err
	}
	err := s.repo.SetRate(ctx, db.SetReleaseRateParams{EventID: eventID, ReleaseRate: rate})
	if err == nil && s.audit != nil {
		s.audit.Record(ctx, audit.Entry{Action: "QUEUE_RATE_CHANGED", TargetType: "event", TargetID: eventID.String(),
			Metadata: map[string]any{"rate": rate}})
	}
	return err
}

type StatsResponse struct {
	Waiting     int64  `json:"waiting"`
	Allowed     int64  `json:"allowed"`
	ReleaseRate int32  `json:"releaseRate"`
	State       string `json:"state"`
}

func (s *Service) Stats(ctx context.Context, eventID uuid.UUID) (StatsResponse, error) {
	ctrl, err := s.repo.GetControl(ctx, eventID)
	if err != nil {
		return StatsResponse{}, err
	}
	waiting, _ := s.store.WaitingCount(ctx, eventID.String())
	allowed, _ := s.store.AllowedCount(ctx, eventID.String())
	return StatsResponse{
		Waiting:     waiting,
		Allowed:     allowed,
		ReleaseRate: ctrl.ReleaseRate,
		State:       ctrl.State,
	}, nil
}

// ensureControl creates a default control row (RUNNING, default rate) if absent.
func (s *Service) ensureControl(ctx context.Context, eventID uuid.UUID) error {
	_, err := s.repo.GetControl(ctx, eventID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = s.repo.UpsertControl(ctx, db.UpsertQueueControlParams{
		EventID:     eventID,
		State:       StateRunning,
		ReleaseRate: s.defaultRate,
		// nullable fields left as zero values (pgtype.Text{}, pgtype.Timestamptz{})
	})
	return err
}
