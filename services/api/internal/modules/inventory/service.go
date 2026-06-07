package inventory

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// CheckResult carries the locked category and current counts after a capacity check.
type CheckResult struct {
	Category  db.EventCategory
	Reserved  int64
	Paid      int64
	Remaining int64
}

// CheckAndLock locks the category row FOR UPDATE, counts active reservations and
// paid orders, then returns ErrSoldOut if no slots remain.
func CheckAndLock(ctx context.Context, repo Repository, categoryID uuid.UUID) (CheckResult, error) {
	cat, err := repo.LockCategoryForUpdate(ctx, categoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CheckResult{}, ErrCategory
		}
		return CheckResult{}, err
	}

	reserved, err := repo.CountActiveReservationsByCategory(ctx, categoryID)
	if err != nil {
		return CheckResult{}, err
	}

	paid, err := repo.CountPaidByCategory(ctx, categoryID)
	if err != nil {
		return CheckResult{}, err
	}

	rem := Remaining(int64(cat.Capacity), reserved, paid)
	if rem <= 0 {
		return CheckResult{}, ErrSoldOut
	}

	return CheckResult{Category: cat, Reserved: reserved, Paid: paid, Remaining: rem}, nil
}

// Reserve creates a new ACTIVE reservation row.
func Reserve(ctx context.Context, repo Repository, arg db.CreateReservationParams) (db.InventoryReservation, error) {
	return repo.CreateReservation(ctx, arg)
}

// Release transitions all ACTIVE reservations for the given order to the
// supplied status (e.g. "PAID", "CANCELLED", "EXPIRED").
func Release(ctx context.Context, repo Repository, orderID uuid.UUID, status string) error {
	return repo.UpdateReservationStatusByOrder(ctx, db.UpdateReservationStatusByOrderParams{
		OrderID: orderID,
		Status:  status,
	})
}
