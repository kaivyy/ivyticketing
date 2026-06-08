package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
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
