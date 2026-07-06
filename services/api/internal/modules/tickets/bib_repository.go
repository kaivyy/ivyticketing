package tickets

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// pgtypeText is a tiny helper to make a non-NULL pgtype.Text from a Go string.
func pgtypeText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// AssignBib persists a BIB number on a ticket. method must be "AUTO", "MANUAL", or "OVERRIDE".
func (r *sqlcRepo) AssignBib(ctx context.Context, ticketID uuid.UUID, bib string, assignedBy uuid.UUID, method string) (db.Ticket, error) {
	return r.q.AssignBib(ctx, db.AssignBibParams{
		ID:                  ticketID,
		BibNumber:           pgtypeText(bib),
		BibAssignedBy:       &assignedBy,
		BibAssignmentMethod: pgtypeText(method),
	})
}

// ClearBib removes a BIB assignment from a ticket.
func (r *sqlcRepo) ClearBib(ctx context.Context, ticketID uuid.UUID) (db.Ticket, error) {
	return r.q.ClearBib(ctx, ticketID)
}

// GetNextBibNumeric returns the current MAX of numeric-only BIBs for an event.
// Prefixed BIBs (e.g., "A00042") are NOT counted; only pure-numeric values
// contribute to the running max.
func (r *sqlcRepo) GetNextBibNumeric(ctx context.Context, eventID uuid.UUID) (int64, error) {
	return r.q.GetNextBibNumeric(ctx, eventID)
}

// ListUnassignedTicketsByEvent returns VALID tickets with no BIB for an event.
func (r *sqlcRepo) ListUnassignedTicketsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.Ticket, error) {
	return r.q.ListUnassignedTicketsByEvent(ctx, eventID)
}

// IsUniqueViolation returns true if err is a Postgres unique-constraint
// violation (SQLSTATE 23505) — used by the BIB retry loop.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}