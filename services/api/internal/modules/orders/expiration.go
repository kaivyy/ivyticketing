package orders

import (
	"context"

	"github.com/varin/ivyticketing/services/api/internal/db"
	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
)

// ExpireOrders finds PENDING_PAYMENT orders past their expiry and transitions them
// to EXPIRED, releasing their reservations. Idempotent and safe to run concurrently
// (the UpdateOrderStatus guard ensures only PENDING_PAYMENT rows transition).
// Returns the number of orders expired.
func (s *Service) ExpireOrders(ctx context.Context, batch int32) (int, error) {
	var expired int
	err := s.repo.ExecTx(ctx, func(tx Repository) error {
		ids, err := tx.ListExpiredPendingOrders(ctx, batch)
		if err != nil {
			return err
		}
		for _, id := range ids {
			updated, err := tx.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
				ID: id, Status: StatusExpired, Status_2: StatusPendingPayment,
			})
			if err != nil {
				// row no longer PENDING_PAYMENT (raced) → skip, stay idempotent
				continue
			}
			if err := inv.Release(ctx, tx.Inventory(), id, ReservationExpired); err != nil {
				return err
			}
			expired++
			s.record(ctx, updated, "ORDER_EXPIRED")
			s.recordReservation(ctx, updated, "RESERVATION_EXPIRED")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return expired, nil
}

// ExpireJob adapts ExpireOrders to a worker.Job. batch caps rows per tick.
func (s *Service) ExpireJob(batch int32) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := s.ExpireOrders(ctx, batch)
		return err
	}
}
