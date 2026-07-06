package scanner

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// displayInfoQuerier is the minimal read surface the TicketReader adapter needs.
// The sqlc-generated *db.Queries satisfies it. Keeping it narrow makes the
// adapter trivial to fake in unit tests and documents that the scanner only
// ever reads the whitelisted display projection — never the full ticket row.
type displayInfoQuerier interface {
	GetTicketDisplayInfo(ctx context.Context, id uuid.UUID) (db.GetTicketDisplayInfoRow, error)
}

// ticketReader is the concrete TicketReader. It maps the sqlc display-info
// projection to the non-sensitive DisplayInfo DTO and translates a missing row
// into ErrTicketNotFound. It deliberately reads through GetTicketDisplayInfo
// (which projects ONLY the whitelisted columns) rather than the full ticket
// row, so no sensitive column can ever reach the scanner client.
type ticketReader struct {
	q displayInfoQuerier
}

// NewTicketReader constructs the injected TicketReader over the shared pool.
// It honors "no cross-module DB access; use services as boundaries" by exposing
// only the display-info read the scanner is allowed to see.
func NewTicketReader(pool *pgxpool.Pool) TicketReader {
	return &ticketReader{q: db.New(pool)}
}

// GetDisplayInfo returns the non-sensitive participant display fields for a
// ticket: participant name, BIB number (empty when unassigned), category name,
// and ticket status. Returns ErrTicketNotFound when the ticket does not exist.
func (t *ticketReader) GetDisplayInfo(ctx context.Context, ticketID uuid.UUID) (DisplayInfo, error) {
	row, err := t.q.GetTicketDisplayInfo(ctx, ticketID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DisplayInfo{}, ErrTicketNotFound
		}
		return DisplayInfo{}, err
	}

	bib := ""
	if row.BibNumber.Valid {
		bib = row.BibNumber.String
	}

	return DisplayInfo{
		ParticipantName: row.ParticipantName,
		BibNumber:       bib,
		CategoryName:    row.CategoryName,
		TicketStatus:    row.TicketStatus,
	}, nil
}
