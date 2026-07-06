package scanner

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the data-access surface for the scanner module. It covers the
// ticket status transition (VALID -> USED), the row lock that closes the
// TOCTOU window, the shared idempotency table, and event ownership checks.
// It owns no tables of its own; the check-in write reuses the tickets table.
type Repository interface {
	// ExecTx runs fn inside a transaction with a tx-bound Repository so the
	// row lock + transition happen atomically.
	ExecTx(ctx context.Context, fn func(Repository) error) error

	// GetTicketByID reads a ticket row (used for display info + duplicate flags).
	GetTicketByID(ctx context.Context, id uuid.UUID) (db.Ticket, error)

	// LockTicketForUpdate issues SELECT ... FOR UPDATE on the ticket row. Must
	// be called inside ExecTx or the lock is meaningless.
	LockTicketForUpdate(ctx context.Context, ticketID uuid.UUID) (db.Ticket, error)

	// MarkTicketUsed performs the guarded VALID -> USED transition, returning
	// the updated row. No row is updated (pgx.ErrNoRows) when the ticket was not
	// VALID.
	MarkTicketUsed(ctx context.Context, arg db.MarkTicketUsedParams) (db.Ticket, error)

	// GetEventOrganizationID returns the org that owns an event; used to verify
	// the event in the URL belongs to the org in the URL.
	GetEventOrganizationID(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error)

	// ListScannableEventsForUser returns the events (across all the user's orgs)
	// for which the user holds racepack.execute or checkin.execute — the
	// Permitted_Events for the scanner.
	ListScannableEventsForUser(ctx context.Context, userID uuid.UUID) ([]db.ListScannableEventsForUserRow, error)

	// UserCanScanEvent reports whether the user holds racepack.execute or
	// checkin.execute in the org that owns the given event. Used as the
	// per-operation authorization guard.
	UserCanScanEvent(ctx context.Context, eventID, userID uuid.UUID) (bool, error)

	// Idempotency.
	GetIdempotencyKey(ctx context.Context, arg db.GetIdempotencyKeyParams) (db.GetIdempotencyKeyRow, error)
	InsertIdempotencyKey(ctx context.Context, arg db.InsertIdempotencyKeyParams) (db.IdempotencyKey, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewRepository constructs a Repository backed by the sqlc-generated Queries.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(&sqlcRepo{pool: r.pool, q: db.New(tx)}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *sqlcRepo) GetTicketByID(ctx context.Context, id uuid.UUID) (db.Ticket, error) {
	return r.q.GetTicketByID(ctx, id)
}

func (r *sqlcRepo) LockTicketForUpdate(ctx context.Context, ticketID uuid.UUID) (db.Ticket, error) {
	return r.q.LockTicketForUpdate(ctx, ticketID)
}

func (r *sqlcRepo) MarkTicketUsed(ctx context.Context, arg db.MarkTicketUsedParams) (db.Ticket, error) {
	return r.q.MarkTicketUsed(ctx, arg)
}

func (r *sqlcRepo) GetEventOrganizationID(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error) {
	return r.q.GetEventOrganizationID(ctx, eventID)
}

func (r *sqlcRepo) ListScannableEventsForUser(ctx context.Context, userID uuid.UUID) ([]db.ListScannableEventsForUserRow, error) {
	return r.q.ListScannableEventsForUser(ctx, userID)
}

func (r *sqlcRepo) UserCanScanEvent(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	return r.q.UserCanScanEvent(ctx, db.UserCanScanEventParams{ID: eventID, UserID: userID})
}

func (r *sqlcRepo) GetIdempotencyKey(ctx context.Context, arg db.GetIdempotencyKeyParams) (db.GetIdempotencyKeyRow, error) {
	return r.q.GetIdempotencyKey(ctx, arg)
}

func (r *sqlcRepo) InsertIdempotencyKey(ctx context.Context, arg db.InsertIdempotencyKeyParams) (db.IdempotencyKey, error) {
	return r.q.InsertIdempotencyKey(ctx, arg)
}
