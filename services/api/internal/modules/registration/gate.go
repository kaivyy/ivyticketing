package registration

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// QueueAdmitter is the queue module's admission check (filled in Part 3).
// Declared here so registration depends on an interface, not the concrete queue package.
type QueueAdmitter interface {
	CheckAdmission(ctx context.Context, participantID, eventID uuid.UUID, admissionToken string) error
}

// Gate implements orders.RegistrationGate.
type Gate struct {
	svc   *Service
	queue QueueAdmitter // may be nil until Part 3
}

func NewGate(svc *Service, queue QueueAdmitter) *Gate {
	return &Gate{svc: svc, queue: queue}
}

var (
	ErrModeNotAvailable = apperr.New(http.StatusConflict, "REGISTRATION_MODE_NOT_AVAILABLE", "this registration mode is not available yet")
	ErrClosed           = apperr.New(http.StatusConflict, "REGISTRATION_CLOSED", "registration is closed")
)

func (g *Gate) Admit(ctx context.Context, participantID, eventID, categoryID uuid.UUID, admissionToken string) error {
	mode, err := g.svc.ResolveForCheckout(ctx, eventID, categoryID)
	if err != nil {
		return err
	}
	switch mode {
	case ModeNormal:
		return nil
	case ModeClosed:
		return ErrClosed
	case ModeWarQueue, ModeRandomizedQueue, ModeHybridQueue:
		if g.queue == nil {
			return ErrModeNotAvailable
		}
		return g.queue.CheckAdmission(ctx, participantID, eventID, admissionToken)
	default:
		// BALLOT / INVITATION_ONLY / PRIORITY_ACCESS / WAITLIST_ONLY — Phase 10-11
		return ErrModeNotAvailable
	}
}
