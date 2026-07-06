package racepack_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/racepack"
)

type fakeLookup struct {
	ticketStatus  string
	ticketEventID uuid.UUID
	ticketPartID  uuid.UUID
	bib           string
	ticketFound   bool
	ticketErr     error
	orderStatus   string
	orderErr      error
	hasActive     bool
	activeErr     error
}

func (f *fakeLookup) GetTicketStatus(ctx context.Context, ticketID uuid.UUID) (string, uuid.UUID, uuid.UUID, string, bool, error) {
	return f.ticketStatus, f.ticketEventID, f.ticketPartID, f.bib, f.ticketFound, f.ticketErr
}
func (f *fakeLookup) GetOrderStatusForTicket(ctx context.Context, ticketID uuid.UUID) (string, error) {
	return f.orderStatus, f.orderErr
}
func (f *fakeLookup) HasActivePickup(ctx context.Context, ticketID uuid.UUID) (bool, error) {
	return f.hasActive, f.activeErr
}

func TestCanPickup_HappyPath(t *testing.T) {
	l := &fakeLookup{
		ticketStatus:  racepack.TicketStatusValid,
		ticketEventID: uuid.New(),
		ticketPartID:  uuid.New(),
		bib:           "A00001",
		ticketFound:   true,
		orderStatus:   racepack.OrderStatusPaid,
		hasActive:     false,
	}
	elig, err := racepack.CanPickup(context.Background(), l, uuid.New(), uuid.Nil, time.Now())
	if err != nil {
		t.Fatalf("expected eligible, got %v", err)
	}
	if elig.BibNumber != "A00001" {
		t.Errorf("expected A00001, got %s", elig.BibNumber)
	}
}

func TestCanPickup_TicketNotFound(t *testing.T) {
	l := &fakeLookup{ticketFound: false}
	_, err := racepack.CanPickup(context.Background(), l, uuid.New(), uuid.Nil, time.Now())
	if !errors.Is(err, racepack.ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
}

func TestCanPickup_TicketCancelled(t *testing.T) {
	l := &fakeLookup{
		ticketStatus: racepack.TicketStatusCancelled,
		ticketFound:  true,
	}
	_, err := racepack.CanPickup(context.Background(), l, uuid.New(), uuid.Nil, time.Now())
	if !errors.Is(err, racepack.ErrTicketCancelled) {
		t.Fatalf("expected ErrTicketCancelled, got %v", err)
	}
}

func TestCanPickup_BibMissing(t *testing.T) {
	l := &fakeLookup{
		ticketStatus: racepack.TicketStatusValid,
		ticketFound:  true,
		bib:          "",
	}
	_, err := racepack.CanPickup(context.Background(), l, uuid.New(), uuid.Nil, time.Now())
	if !errors.Is(err, racepack.ErrBibMissing) {
		t.Fatalf("expected ErrBibMissing, got %v", err)
	}
}

func TestCanPickup_AlreadyPickedUp(t *testing.T) {
	l := &fakeLookup{
		ticketStatus: racepack.TicketStatusValid,
		ticketFound:  true,
		bib:          "A00001",
		hasActive:    true,
	}
	_, err := racepack.CanPickup(context.Background(), l, uuid.New(), uuid.Nil, time.Now())
	if !errors.Is(err, racepack.ErrAlreadyPickedUp) {
		t.Fatalf("expected ErrAlreadyPickedUp, got %v", err)
	}
}

func TestCanPickup_OrderNotPaid(t *testing.T) {
	l := &fakeLookup{
		ticketStatus: racepack.TicketStatusValid,
		ticketFound:  true,
		bib:          "A00001",
		hasActive:    false,
		orderStatus:  "PENDING_PAYMENT",
	}
	_, err := racepack.CanPickup(context.Background(), l, uuid.New(), uuid.Nil, time.Now())
	if !errors.Is(err, racepack.ErrOrderNotPaid) {
		t.Fatalf("expected ErrOrderNotPaid, got %v", err)
	}
}

func TestValidateStateTransition_HappyPath(t *testing.T) {
	if err := racepack.ValidateStateTransition(racepack.ProblemCaseStatusOpen, racepack.ProblemCaseStatusUnderReview); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if err := racepack.ValidateStateTransition(racepack.ProblemCaseStatusUnderReview, racepack.ProblemCaseStatusResolved); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateStateTransition_TerminalBlocked(t *testing.T) {
	if err := racepack.ValidateStateTransition(racepack.ProblemCaseStatusResolved, racepack.ProblemCaseStatusOpen); err == nil {
		t.Error("expected error for transition from terminal state")
	}
}

func TestValidateStateTransition_Skip(t *testing.T) {
	if err := racepack.ValidateStateTransition(racepack.ProblemCaseStatusOpen, racepack.ProblemCaseStatusResolved); err != nil {
		t.Errorf("OPEN -> RESOLVED should be allowed, got %v", err)
	}
}
