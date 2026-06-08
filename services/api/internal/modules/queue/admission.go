package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// CheckAdmission verifies the participant holds an ACTIVE, unexpired admission.
// Implements registration.QueueAdmitter (read-only; safe to call outside a tx).
func (s *Service) CheckAdmission(ctx context.Context, participantID, eventID uuid.UUID, admissionToken string) error {
	adm, err := s.repo.GetActiveAdmission(ctx, db.GetActiveAdmissionByParticipantParams{
		EventID:       eventID,
		ParticipantID: participantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAdmissionRequired
	}
	if err != nil {
		return err
	}
	if admissionToken == "" || adm.ID.String() != admissionToken {
		return ErrAdmissionRequired
	}
	if time.Now().After(adm.CheckoutExpiresAt.Time) {
		return ErrAdmissionExpired
	}
	return nil
}

// OnCheckoutComplete implements orders.CheckoutHook. Called best-effort after checkout.
func (s *Service) OnCheckoutComplete(ctx context.Context, participantID, eventID uuid.UUID) error {
	return s.ConsumeOnCheckout(ctx, participantID, eventID)
}

// ConsumeOnCheckout marks the admission consumed and the token completed.
// Called best-effort after a successful checkout.
func (s *Service) ConsumeOnCheckout(ctx context.Context, participantID, eventID uuid.UUID) error {
	adm, err := s.repo.GetActiveAdmission(ctx, db.GetActiveAdmissionByParticipantParams{
		EventID:       eventID,
		ParticipantID: participantID,
	})
	if err != nil {
		return err
	}
	if err := s.repo.ConsumeAdmission(ctx, adm.ID); err != nil {
		return err
	}
	if err := s.repo.MarkCompleted(ctx, adm.TokenID); err != nil {
		return err
	}
	_ = s.store.RemoveAllowed(ctx, eventID.String(), participantID.String())
	return nil
}

// ExpireDue expires ACTIVE admissions past their checkout window and requeues
// their tokens to the back of the WAITING line (decision Q10).
// Returns count expired.
func (s *Service) ExpireDue(ctx context.Context, limit int) (int, error) {
	due, err := s.repo.ListExpiredAdmissions(ctx, int32(limit))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, adm := range due {
		newScore := FifoScore(time.Now())
		err := s.repo.ExecTx(ctx, func(tx Repository) error {
			if err := tx.ExpireAdmission(ctx, adm.ID); err != nil {
				return err
			}
			return tx.Requeue(ctx, db.RequeueTokenParams{ID: adm.TokenID, Score: newScore})
		})
		if err != nil {
			continue
		}
		if s.store != nil {
			_ = s.store.MoveToWaiting(ctx, adm.EventID.String(), adm.ParticipantID.String(), newScore)
		}
		if s.audit != nil {
			s.audit.Record(ctx, audit.Entry{
				Action:     "QUEUE_ADMISSION_EXPIRED",
				TargetType: "queue_admission",
				TargetID:   adm.ID.String(),
				Metadata:   map[string]any{"tokenId": adm.TokenID.String()},
			})
		}
		count++
	}
	return count, nil
}
