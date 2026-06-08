package orders

import (
	"context"

	"github.com/google/uuid"
)

// RegistrationGate decides whether a participant may proceed to checkout.
// Implemented by the registration module (dependency inversion; orders does not
// import registration/queue).
type RegistrationGate interface {
	Admit(ctx context.Context, participantID, eventID, categoryID uuid.UUID, admissionToken string) error
}

// noopGate permits everything — used when no gate is wired (preserves NORMAL
// behavior in tests and the expiry worker).
type noopGate struct{}

func (noopGate) Admit(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) error { return nil }
