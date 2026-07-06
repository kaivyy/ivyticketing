package tickets

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository defines data access for tickets.
type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error

	CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error)
	GetTicketByID(ctx context.Context, id uuid.UUID) (db.Ticket, error)
	GetTicketByOrderID(ctx context.Context, orderID uuid.UUID) (db.Ticket, error)
	ListTicketsByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Ticket, error)
	ListTicketsByEvent(ctx context.Context, arg db.ListTicketsByEventParams) ([]db.Ticket, error)

	// Lookups for snapshotting + invoice (reuse existing queries).
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error)

	// BIB management (migration 00049+).
	AssignBib(ctx context.Context, ticketID uuid.UUID, bib string, assignedBy uuid.UUID, method string) (db.Ticket, error)
	ClearBib(ctx context.Context, ticketID uuid.UUID) (db.Ticket, error)
	GetNextBibNumeric(ctx context.Context, eventID uuid.UUID) (int64, error)
	ListUnassignedTicketsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.Ticket, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

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

func (r *sqlcRepo) CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error) {
	return r.q.CreateTicket(ctx, arg)
}

func (r *sqlcRepo) GetTicketByID(ctx context.Context, id uuid.UUID) (db.Ticket, error) {
	return r.q.GetTicketByID(ctx, id)
}

func (r *sqlcRepo) GetTicketByOrderID(ctx context.Context, orderID uuid.UUID) (db.Ticket, error) {
	return r.q.GetTicketByOrderID(ctx, orderID)
}

func (r *sqlcRepo) ListTicketsByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Ticket, error) {
	return r.q.ListTicketsByParticipant(ctx, participantID)
}

func (r *sqlcRepo) ListTicketsByEvent(ctx context.Context, arg db.ListTicketsByEventParams) ([]db.Ticket, error) {
	return r.q.ListTicketsByEvent(ctx, arg)
}

func (r *sqlcRepo) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return r.q.GetUserByID(ctx, id)
}

func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}

func (r *sqlcRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return r.q.GetCategoryByID(ctx, id)
}

func (r *sqlcRepo) GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error) {
	return r.q.GetOrderByID(ctx, id)
}
