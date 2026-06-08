package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// Release promotes up to n WAITING tokens to ALLOWED, creating admission windows.
// Pure rate (decision Q9): inventory lock is the oversold backstop.
// Idempotent: MarkAllowed WHERE status='WAITING' no-ops if already promoted.
func (s *Service) Release(ctx context.Context, eventID uuid.UUID, n int, window time.Duration) (int, error) {
	if n <= 0 {
		return 0, nil
	}
	waiting, err := s.repo.ListWaiting(ctx, db.ListWaitingTokensParams{
		EventID: eventID,
		Limit:   int32(n),
	})
	if err != nil {
		return 0, err
	}
	promoted := 0
	expiresAt := time.Now().Add(window)
	for _, tok := range waiting {
		err := s.repo.ExecTx(ctx, func(tx Repository) error {
			allowed, err := tx.MarkAllowed(ctx, tok.ID)
			if errors.Is(err, pgx.ErrNoRows) {
				// already promoted concurrently — skip
				return nil
			}
			if err != nil {
				return err
			}
			_, err = tx.CreateAdmission(ctx, db.CreateAdmissionParams{
				TokenID:           allowed.ID,
				EventID:           eventID,
				ParticipantID:     allowed.ParticipantID,
				CheckoutExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
			})
			return err
		})
		if err != nil {
			continue
		}
		if s.store != nil {
			_ = s.store.MoveToAllowed(ctx, eventID.String(), tok.ParticipantID.String(), expiresAt.Unix())
		}
		promoted++
	}
	if promoted > 0 && s.audit != nil {
		eid := eventID
		s.audit.Record(ctx, audit.Entry{
			Action:     "QUEUE_RELEASED",
			TargetType: "event",
			TargetID:   eid.String(),
			Metadata:   map[string]any{"promoted": promoted},
		})
	}
	return promoted, nil
}
