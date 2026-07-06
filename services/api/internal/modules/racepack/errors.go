package racepack

import "errors"

var (
	ErrOrderNotPaid         = errors.New("order not paid")
	ErrBibMissing           = errors.New("bib not assigned")
	ErrAlreadyPickedUp      = errors.New("ticket already picked up")
	ErrSlotInactive         = errors.New("slot inactive")
	ErrOutsideWindow        = errors.New("outside pickup window")
	ErrTicketCancelled      = errors.New("ticket cancelled")
	ErrCounterInactive      = errors.New("counter inactive")
	ErrCounterNotFound      = errors.New("counter not found")
	ErrSlotNotFound         = errors.New("slot not found")
	ErrSlotFull             = errors.New("slot full")
	ErrTicketNotFound       = errors.New("ticket not found")
	ErrInvalidStateChange   = errors.New("invalid status transition")
	ErrInvalidMethod        = errors.New("invalid pickup method")
	ErrTicketEventMismatch  = errors.New("ticket does not belong to this event")
	ErrCounterEventMismatch = errors.New("counter does not belong to this event")
	ErrNoProblemTarget      = errors.New("at least one of ticket_id or participant_id required")
	ErrIdempotencyConflict  = errors.New("idempotency key reused with different payload")
)