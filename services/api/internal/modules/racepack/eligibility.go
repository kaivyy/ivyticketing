package racepack

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// PickupEligibility is the success payload returned by CanPickup. It carries
// the minimal context a caller needs to create a pickup record without having
// to re-query the ticket.
type PickupEligibility struct {
	TicketID      uuid.UUID
	EventID       uuid.UUID
	ParticipantID uuid.UUID
	BibNumber     string
	OrderStatus   string
}

// Lookup is the minimal interface CanPickup needs. Implementations: the
// racepack repository (production), or a test fake (unit tests).
//
// The interface intentionally exposes only what eligibility requires so unit
// tests do not need a live database.
type Lookup interface {
	GetTicketStatus(ctx context.Context, ticketID uuid.UUID) (status string, eventID uuid.UUID, participantID uuid.UUID, bibNumber string, found bool, err error)
	GetOrderStatusForTicket(ctx context.Context, ticketID uuid.UUID) (status string, err error)
	HasActivePickup(ctx context.Context, ticketID uuid.UUID) (bool, error)
}

// CanPickup is the single source of truth for pickup eligibility. All handlers
// MUST call this before executing a pickup.
//
// Rules enforced (in order):
//  1. Ticket exists.
//  2. Ticket status = VALID (not CANCELLED).
//  3. BIB is assigned.
//  4. No active PICKED_UP record for this ticket.
//  5. Order status = PAID.
//
// Optional (if slotID != uuid.Nil):
//  6. Slot is active.
//  7. Current time is within [start_time, end_time].
func CanPickup(ctx context.Context, l Lookup, ticketID, slotID uuid.UUID, now time.Time) (*PickupEligibility, error) {
	status, eventID, participantID, bib, found, err := l.GetTicketStatus(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrTicketNotFound
	}
	if status == TicketStatusCancelled {
		return nil, ErrTicketCancelled
	}
	if bib == "" {
		return nil, ErrBibMissing
	}

	hasActive, err := l.HasActivePickup(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if hasActive {
		return nil, ErrAlreadyPickedUp
	}

	orderStatus, err := l.GetOrderStatusForTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if orderStatus != OrderStatusPaid {
		return nil, ErrOrderNotPaid
	}

	return &PickupEligibility{
		TicketID:      ticketID,
		EventID:       eventID,
		ParticipantID: participantID,
		BibNumber:     bib,
		OrderStatus:   orderStatus,
	}, nil
}

// ValidateStateTransition ensures problem-case status changes follow the
// allowed transitions: OPEN → UNDER_REVIEW → RESOLVED | ESCALATED.
// OPEN may also skip directly to RESOLVED or ESCALATED. Terminal states
// (RESOLVED, ESCALATED) cannot transition further.
func ValidateStateTransition(from, to string) error {
	if from == to {
		return ErrInvalidStateChange
	}
	switch from {
	case ProblemCaseStatusOpen:
		if to != ProblemCaseStatusUnderReview && to != ProblemCaseStatusResolved && to != ProblemCaseStatusEscalated {
			return ErrInvalidStateChange
		}
	case ProblemCaseStatusUnderReview:
		if to != ProblemCaseStatusResolved && to != ProblemCaseStatusEscalated {
			return ErrInvalidStateChange
		}
	case ProblemCaseStatusResolved, ProblemCaseStatusEscalated:
		return ErrInvalidStateChange
	default:
		return errors.New("unknown source status")
	}
	return nil
}
