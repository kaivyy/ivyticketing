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

// BallotAdmitter checks whether a participant holds a valid active grant
// for a given category. Declared here so registration depends on an
// interface, not the concrete ballot package.
type BallotAdmitter interface {
	CheckBallotAdmission(ctx context.Context, participantID, categoryID uuid.UUID, admissionToken string) error
}

// LifecycleChecker verifies whether a registration window is open for a given mode.
// Fail-open: if no lifecycle is configured for the category, returns true.
type LifecycleChecker interface {
	IsWindowOpen(ctx context.Context, categoryID uuid.UUID, mode Mode) (bool, WindowClosedReason, error)
}

// WindowClosedReason is an opaque string reason why a window is closed.
type WindowClosedReason = string

var ErrRegistrationWindowClosed = apperr.New(http.StatusConflict, "REGISTRATION_WINDOW_CLOSED", "registration window is not open")

// Gate implements orders.RegistrationGate.
type Gate struct {
	svc       *Service
	queue     QueueAdmitter    // may be nil until Part 3
	lifecycle LifecycleChecker // may be nil — fail-open
	ballot    BallotAdmitter   // may be nil until Phase 10 Part 3
}

func NewGate(svc *Service, queue QueueAdmitter, lifecycle LifecycleChecker, ballot BallotAdmitter) *Gate {
	return &Gate{svc: svc, queue: queue, lifecycle: lifecycle, ballot: ballot}
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

	// Lifecycle window check — fail-open if no lifecycle configured
	if g.lifecycle != nil && mode != ModeNormal && mode != ModeClosed {
		open, _, err := g.lifecycle.IsWindowOpen(ctx, categoryID, mode)
		if err != nil {
			return err
		}
		if !open {
			return ErrRegistrationWindowClosed
		}
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
	case ModeBallot:
		if g.ballot == nil {
			return ErrModeNotAvailable
		}
		return g.ballot.CheckBallotAdmission(ctx, participantID, categoryID, admissionToken)
	default:
		// INVITATION_ONLY / PRIORITY_ACCESS / WAITLIST_ONLY — Phase 11
		return ErrModeNotAvailable
	}
}
