package orders

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
	notifmod "github.com/varin/ivyticketing/services/api/internal/modules/notifications"
)

// ExpireOrders finds PENDING_PAYMENT orders past their expiry and transitions them
// to EXPIRED, releasing their reservations. Idempotent and safe to run concurrently
// (the UpdateOrderStatus guard ensures only PENDING_PAYMENT rows transition).
// Returns the number of orders expired.
func (s *Service) ExpireOrders(ctx context.Context, batch int32) (int, error) {
	var expired int
	var expiredOrderIDs []uuid.UUID
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
			expiredOrderIDs = append(expiredOrderIDs, id)
			s.record(ctx, updated, "ORDER_EXPIRED")
			s.recordReservation(ctx, updated, "RESERVATION_EXPIRED")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Fire payment.expired notifications for successfully expired orders.
	for _, orderID := range expiredOrderIDs {
		s.notifyPaymentExpired(ctx, orderID)
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

// notifyPaymentExpired fires a payment.expired notification for an expired order.
// Only called after successful PENDING_PAYMENT→EXPIRED transition.
func (s *Service) notifyPaymentExpired(ctx context.Context, orderID uuid.UUID) {
	if s.notifier == nil {
		return
	}
	order, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if s.log != nil {
			s.log.Warn("notifyPaymentExpired: get order failed", "order_id", orderID, "err", err)
		}
		return
	}
	total := fmt.Sprintf("Rp %d", order.Total)
	var deadline string
	if order.ExpiredAt.Valid {
		deadline = order.ExpiredAt.Time.Format("02 Jan 2006 15:04")
	}
	pid := order.ParticipantID
	go func() {
		if err := s.notifier.Enqueue(context.Background(), pid, "payment.expired", notifmod.TemplateData{
			OrderID:         order.ID.String(),
			OrderNumber:     order.OrderNumber,
			TotalAmount:     total,
			PaymentDeadline: deadline,
		}); err != nil {
			if s.log != nil {
				s.log.Warn("notifyPaymentExpired: enqueue failed", "order_id", order.ID, "err", err)
			}
		}
	}()
}
