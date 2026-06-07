package orders

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error

	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error)
	GetOrderByNumber(ctx context.Context, number string) (db.Order, error)
	CreateOrder(ctx context.Context, arg db.CreateOrderParams) (db.Order, error)
	UpdateOrderStatus(ctx context.Context, arg db.UpdateOrderStatusParams) (db.Order, error)
	ListOrdersByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Order, error)
	ListOrdersByOrgEvent(ctx context.Context, arg db.ListOrdersByOrgEventParams) ([]db.Order, error)
	CountActiveOrdersForUserCategory(ctx context.Context, arg db.CountActiveOrdersForUserCategoryParams) (int64, error)
	ListExpiredPendingOrders(ctx context.Context, limit int32) ([]uuid.UUID, error)

	// Inventory returns an inventory.Repository bound to this repo's queries (pool or tx).
	// When called within ExecTx, both order and reservation writes use the same transaction.
	Inventory() inv.Repository
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

func (r *sqlcRepo) Inventory() inv.Repository { return inv.NewRepository(r.q) }

func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}

func (r *sqlcRepo) GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error) {
	return r.q.GetOrderByID(ctx, id)
}

func (r *sqlcRepo) GetOrderByNumber(ctx context.Context, number string) (db.Order, error) {
	return r.q.GetOrderByNumber(ctx, number)
}

func (r *sqlcRepo) CreateOrder(ctx context.Context, arg db.CreateOrderParams) (db.Order, error) {
	return r.q.CreateOrder(ctx, arg)
}

func (r *sqlcRepo) UpdateOrderStatus(ctx context.Context, arg db.UpdateOrderStatusParams) (db.Order, error) {
	return r.q.UpdateOrderStatus(ctx, arg)
}

func (r *sqlcRepo) ListOrdersByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Order, error) {
	return r.q.ListOrdersByParticipant(ctx, participantID)
}

func (r *sqlcRepo) ListOrdersByOrgEvent(ctx context.Context, arg db.ListOrdersByOrgEventParams) ([]db.Order, error) {
	return r.q.ListOrdersByOrgEvent(ctx, arg)
}

func (r *sqlcRepo) CountActiveOrdersForUserCategory(ctx context.Context, arg db.CountActiveOrdersForUserCategoryParams) (int64, error) {
	return r.q.CountActiveOrdersForUserCategory(ctx, arg)
}

func (r *sqlcRepo) ListExpiredPendingOrders(ctx context.Context, limit int32) ([]uuid.UUID, error) {
	return r.q.ListExpiredPendingOrders(ctx, limit)
}
