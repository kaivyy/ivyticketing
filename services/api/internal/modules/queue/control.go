package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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

// SetSchedule sets the sale window and randomization seed for randomized/hybrid modes.
// If seed is empty, a random hex seed is auto-generated and stored.
func (s *Service) SetSchedule(ctx context.Context, eventID uuid.UUID, seed string, saleStart, presaleOpen *time.Time) error {
	if seed == "" {
		seed = generateSeed()
	}
	// Preserve existing state and rate if a control row already exists.
	ctrl, err := s.repo.GetControl(ctx, eventID)
	state := StateRunning
	rate := s.defaultRate
	if err == nil {
		state = ctrl.State
		rate = ctrl.ReleaseRate
	}
	_, err = s.repo.UpsertControl(ctx, db.UpsertQueueControlParams{
		EventID:           eventID,
		State:             state,
		ReleaseRate:       rate,
		RandomizationSeed: pgtype.Text{String: seed, Valid: true},
		SaleStartAt:       toPGTimestamptz(saleStart),
		PresalePoolOpenAt: toPGTimestamptz(presaleOpen),
	})
	if err == nil && s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			Action:     "QUEUE_SCHEDULE_SET",
			TargetType: "event",
			TargetID:   eventID.String(),
			Metadata:   map[string]any{"seed": seed},
		})
	}
	return err
}

func generateSeed() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func toPGTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
