package inventory

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the subset of DB ops inventory needs. It is satisfied both by a
// pool-backed adapter and (within a transaction) by a tx-backed *db.Queries — the
// orders module passes its tx-bound queries so reservation + order creation are atomic.
type Repository interface {
	LockCategoryForUpdate(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	CountActiveReservationsByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error)
	CountPaidByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error)
	CreateReservation(ctx context.Context, arg db.CreateReservationParams) (db.InventoryReservation, error)
	ExpireReservationsForOrder(ctx context.Context, orderID uuid.UUID) error
	UpdateReservationStatusByOrder(ctx context.Context, arg db.UpdateReservationStatusByOrderParams) error
}

// QueriesRepo adapts *db.Queries (pool or tx) to Repository.
type QueriesRepo struct {
	Q *db.Queries
}

func NewRepository(q *db.Queries) Repository { return &QueriesRepo{Q: q} }

func (r *QueriesRepo) LockCategoryForUpdate(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return r.Q.LockCategoryForUpdate(ctx, id)
}
func (r *QueriesRepo) CountActiveReservationsByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error) {
	return r.Q.CountActiveReservationsByCategory(ctx, categoryID)
}
func (r *QueriesRepo) CountPaidByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error) {
	return r.Q.CountPaidByCategory(ctx, categoryID)
}
func (r *QueriesRepo) CreateReservation(ctx context.Context, arg db.CreateReservationParams) (db.InventoryReservation, error) {
	return r.Q.CreateReservation(ctx, arg)
}
func (r *QueriesRepo) ExpireReservationsForOrder(ctx context.Context, orderID uuid.UUID) error {
	return r.Q.ExpireReservationsForOrder(ctx, orderID)
}
func (r *QueriesRepo) UpdateReservationStatusByOrder(ctx context.Context, arg db.UpdateReservationStatusByOrderParams) error {
	return r.Q.UpdateReservationStatusByOrder(ctx, arg)
}
