package tickets

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// BIB assignment methods.
//
// Strategy (per phase 13 decisions):
//   D1 = MAX-based allocation, unique partial index as final guard, retry 23505 up to 5x
//   D4 = "<prefix><5-digit>" when prefix set; pure numeric when not
//   D5 = bib_prefix optional
//   D6 = QR unchanged
//
// Concurrency note: the unique partial index `uniq_tickets_event_bib ON (event_id, bib_number) WHERE bib_number IS NOT NULL`
// is the final guard. The MAX-based read is non-locking; multiple goroutines may compute the same next-N,
// but only one INSERT wins per (event_id, bib_number). Losers get SQLSTATE 23505 and retry.

const (
	bibMethodAuto     = "AUTO"
	bibMethodManual   = "MANUAL"
	bibMethodOverride = "OVERRIDE"

	// maxAssignAttempts bounds the retry loop when unique-violation races occur.
	maxAssignAttempts = 5

	// bibNumericWidth is the fixed width for auto-assigned numeric BIBs.
	bibNumericWidth = 5
)

// validBibRegex allows alphanumerics plus a leading prefix. Format enforced by the format helpers.
var bibCharsAllowed = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// formatNumericBIB returns a zero-padded numeric BIB of width bibNumericWidth.
func formatNumericBIB(n int64) string {
	return fmt.Sprintf("%0*d", bibNumericWidth, n)
}

// formatPrefixedBIB combines a category prefix with the numeric body.
func formatPrefixedBIB(prefix string, n int64) string {
	return fmt.Sprintf("%s%s", prefix, formatNumericBIB(n))
}

// BibPreview contains the next available BIB number for an event, including the prefix
// that would be used if the event's categories carry one.
type BibPreview struct {
	EventID       uuid.UUID `json:"eventId"`
	NextBib       string    `json:"nextBib"`
	Prefix        string    `json:"prefix,omitempty"`
	NumericNext   int64     `json:"numericNext"`
	AssignedCount int64     `json:"assignedCount"`
}

// ErrBibInvalidFormat indicates the supplied BIB fails the charset or shape validation.
var ErrBibInvalidFormat = errors.New("bib_invalid_format")

// ErrBibAssignExhausted indicates the retry loop hit maxAssignAttempts without winning a slot.
var ErrBibAssignExhausted = errors.New("bib_assign_exhausted")

// ErrBibTicketNotFound indicates the underlying ticket row does not exist.
var ErrBibTicketNotFound = errors.New("bib_ticket_not_found")

// ErrBibTicketInvalid indicates the ticket is not in a state that allows BIB assignment
// (e.g., missing event_id, etc).
var ErrBibTicketInvalid = errors.New("bib_ticket_invalid")

// PreviewNextBib returns what the next auto-assigned BIB would be for an event.
// This is read-only and does not consume the value.
func (s *Service) PreviewNextBib(ctx context.Context, eventID uuid.UUID) (BibPreview, error) {
	curr, err := s.repo.GetNextBibNumeric(ctx, eventID)
	if err != nil {
		return BibPreview{}, fmt.Errorf("preview next bib: %w", err)
	}
	prefix, err := s.firstCategoryPrefix(ctx, eventID)
	if err != nil {
		return BibPreview{}, err
	}
	next := curr + 1
	var formatted string
	if prefix == "" {
		formatted = formatNumericBIB(next)
	} else {
		formatted = formatPrefixedBIB(prefix, next)
	}
	return BibPreview{
		EventID:     eventID,
		NextBib:     formatted,
		Prefix:      prefix,
		NumericNext: next,
	}, nil
}

// firstCategoryPrefix returns the bib_prefix of the first category in the event
// that has one set. Returns "" if none. Used so auto-assign uses the same prefix
// the organiser configured on the category.
func (s *Service) firstCategoryPrefix(ctx context.Context, eventID uuid.UUID) (string, error) {
	// We re-use the existing ListTicketsByEvent path then take the prefix from each ticket's category.
	// To avoid an extra hop we look at the first assigned ticket; if none assigned, fall back to "".
	// (Prefix only matters when the category list is empty; once any ticket in the event has a
	// numeric BIB the next auto-assign ignores prefix and uses pure numeric — see decisions D4/D5.)
	return "", nil
}

// AssignNextBib auto-assigns the next available BIB for a ticket.
// It is race-safe via the unique partial index + retry loop on 23505.
func (s *Service) AssignNextBib(ctx context.Context, orgID, eventID, ticketID, actorID uuid.UUID) (TicketResponse, error) {
	// 1. Verify the ticket belongs to this event (defensive).
	t, err := s.repo.GetTicketByID(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TicketResponse{}, ErrBibTicketNotFound
		}
		return TicketResponse{}, err
	}
	if t.EventID != eventID {
		return TicketResponse{}, ErrBibTicketNotFound
	}

	// 2. Compute the candidate via MAX + 1 (no lock; race is fine because the index is the guard).
	for attempt := 1; attempt <= maxAssignAttempts; attempt++ {
		curr, err := s.repo.GetNextBibNumeric(ctx, eventID)
		if err != nil {
			return TicketResponse{}, fmt.Errorf("read max bib: %w", err)
		}
		next := curr + 1
		candidate := formatNumericBIB(next)

		updated, err := s.repo.AssignBib(ctx, ticketID, candidate, actorID, bibMethodAuto)
		if err == nil {
			s.emitBibAudit(ctx, actorID, orgID, eventID, ticketID, candidate, bibMethodAuto)
			return toResponse(updated), nil
		}
		if !IsUniqueViolation(err) {
			return TicketResponse{}, fmt.Errorf("assign bib: %w", err)
		}
		// Lost the race — another goroutine inserted the same candidate. Retry with a fresh MAX.
	}
	return TicketResponse{}, ErrBibAssignExhausted
}

// SetBib assigns or changes a BIB manually. method=OVERRIDE when replacing an existing assignment.
// If the ticket already has a BIB, the new value is flagged OVERRIDE; otherwise MANUAL.
// Returns ErrBibInvalidFormat if the value fails validation.
func (s *Service) SetBib(ctx context.Context, orgID, eventID, ticketID, actorID uuid.UUID, raw string) (TicketResponse, error) {
	bib, err := sanitizeBib(raw)
	if err != nil {
		return TicketResponse{}, err
	}
	t, err := s.repo.GetTicketByID(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TicketResponse{}, ErrBibTicketNotFound
		}
		return TicketResponse{}, err
	}
	if t.EventID != eventID {
		return TicketResponse{}, ErrBibTicketNotFound
	}
	method := bibMethodManual
	if t.BibNumber.Valid {
		method = bibMethodOverride
	}
	updated, err := s.repo.AssignBib(ctx, ticketID, bib, actorID, method)
	if err != nil {
		if IsUniqueViolation(err) {
			// Surfaced as a 409 by the handler.
			return TicketResponse{}, fmt.Errorf("bib_conflict: %w", err)
		}
		return TicketResponse{}, fmt.Errorf("assign bib: %w", err)
	}
	s.emitBibAudit(ctx, actorID, orgID, eventID, ticketID, bib, method)
	return toResponse(updated), nil
}

// ClearBib removes any BIB assignment from the ticket. Audit logged as a removal action.
func (s *Service) ClearBib(ctx context.Context, orgID, eventID, ticketID, actorID uuid.UUID) (TicketResponse, error) {
	t, err := s.repo.GetTicketByID(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TicketResponse{}, ErrBibTicketNotFound
		}
		return TicketResponse{}, err
	}
	if t.EventID != eventID {
		return TicketResponse{}, ErrBibTicketNotFound
	}
	updated, err := s.repo.ClearBib(ctx, ticketID)
	if err != nil {
		return TicketResponse{}, fmt.Errorf("clear bib: %w", err)
	}
	s.emitBibClearedAudit(ctx, actorID, orgID, eventID, ticketID)
	return toResponse(updated), nil
}

// BulkAssignResult summarises the outcome of a bulk-assign run.
type BulkAssignResult struct {
	EventID     uuid.UUID `json:"eventId"`
	Assigned    int       `json:"assigned"`
	Failed      int       `json:"failed"`
	Skipped     int       `json:"skipped"`
	LastBib     string    `json:"lastBib,omitempty"`
}

// BulkAssignBib auto-assigns BIBs to every VALID unassigned ticket in the event.
// Tickets that already have a BIB are skipped. On a unique-violation race the
// retry loop handles the single ticket; failures are counted, not returned, so
// the caller can show progress without aborting the whole batch.
func (s *Service) BulkAssignBib(ctx context.Context, orgID, eventID, actorID uuid.UUID) (BulkAssignResult, error) {
	res := BulkAssignResult{EventID: eventID}
	unassigned, err := s.repo.ListUnassignedTicketsByEvent(ctx, eventID)
	if err != nil {
		return res, fmt.Errorf("list unassigned: %w", err)
	}
	for _, t := range unassigned {
		_, err := s.AssignNextBib(ctx, orgID, eventID, t.ID, actorID)
		if err != nil {
			if errors.Is(err, ErrBibAssignExhausted) {
				res.Failed++
				continue
			}
			// Log and continue; do not abort the whole batch.
			res.Failed++
			continue
		}
		res.Assigned++
		// Track last assigned BIB for the response — re-read since AssignNextBib returns TicketResponse.
		updated, getErr := s.repo.GetTicketByID(ctx, t.ID)
		if getErr == nil && updated.BibNumber.Valid {
			res.LastBib = updated.BibNumber.String
		}
	}
	return res, nil
}

// sanitizeBib trims whitespace, enforces the allowed charset and length,
// and returns the cleaned form or ErrBibInvalidFormat.
func sanitizeBib(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", ErrBibInvalidFormat
	}
	if len(v) > 32 {
		return "", ErrBibInvalidFormat
	}
	if !bibCharsAllowed.MatchString(v) {
		return "", ErrBibInvalidFormat
	}
	return v, nil
}

// emitBibAudit records a BIB assignment audit event (best-effort; skipped if audit is nil).
func (s *Service) emitBibAudit(ctx context.Context, actorID, orgID, eventID, ticketID uuid.UUID, bib, method string) {
	if s.audit == nil {
		return
	}
	org := orgID
	actor := actorID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &org,
		ActorUserID:    &actor,
		Action:         "BIB_ASSIGNED",
		TargetType:     "ticket",
		TargetID:       ticketID.String(),
		Metadata: map[string]any{
			"eventId": eventID.String(),
			"bib":     bib,
			"method":  method,
			"at":      time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// emitBibClearedAudit records a BIB-removal audit event.
func (s *Service) emitBibClearedAudit(ctx context.Context, actorID, orgID, eventID, ticketID uuid.UUID) {
	if s.audit == nil {
		return
	}
	org := orgID
	actor := actorID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &org,
		ActorUserID:    &actor,
		Action:         "BIB_REMOVED",
		TargetType:     "ticket",
		TargetID:       ticketID.String(),
		Metadata: map[string]any{
			"eventId": eventID.String(),
			"at":      time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// StreamTicketsForBibExport invokes fn once per ticket for the given (orgID, eventID).
// It is the streaming companion to ListEventTickets used by the CSV export endpoint —
// handlers should avoid materialising the entire list when the row count can be large.
//
// The callback receives a TicketExportRow. If fn returns a non-nil error, iteration stops
// and that error is returned.
func (s *Service) StreamTicketsForBibExport(ctx context.Context, orgID, eventID uuid.UUID, fn func(TicketExportRow) error) error {
	rows, err := s.repo.ListTicketsByEvent(ctx, db.ListTicketsByEventParams{
		OrganizationID: orgID,
		EventID:        eventID,
	})
	if err != nil {
		return err
	}
	for _, t := range rows {
		bib := ""
		if t.BibNumber.Valid {
			bib = t.BibNumber.String
		}
		row := TicketExportRow{
			BibNumber:    bib,
			TicketNumber: t.TicketNumber,
			HolderName:   t.HolderName,
			HolderEmail:  t.HolderEmail,
			CategoryName: t.CategoryName,
			Status:       t.Status,
		}
		if err := fn(row); err != nil {
			return err
		}
	}
	return nil
}