package orders

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type Service struct {
	repo  Repository
	audit AuditRecorder
	ttl   time.Duration
	gate  RegistrationGate
	hook  CheckoutHook
}

func NewService(repo Repository, recorder AuditRecorder, ttl time.Duration, gate RegistrationGate, hook CheckoutHook) *Service {
	if gate == nil {
		gate = noopGate{}
	}
	return &Service{repo: repo, audit: recorder, ttl: ttl, gate: gate, hook: hook}
}

func (s *Service) Checkout(ctx context.Context, participantID, eventID, categoryID uuid.UUID, admissionToken string) (OrderResponse, error) {
	if err := s.gate.Admit(ctx, participantID, eventID, categoryID, admissionToken); err != nil {
		return OrderResponse{}, err
	}

	var created db.Order
	err := s.repo.ExecTx(ctx, func(tx Repository) error {
		event, err := tx.GetEventByID(ctx, eventID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCategoryNotFound
		} else if err != nil {
			return err
		}

		check, err := inv.CheckAndLock(ctx, tx.Inventory(), categoryID)
		if errors.Is(err, inv.ErrCategory) {
			return ErrCategoryNotFound
		} else if err != nil {
			return err
		}
		cat := check.Category
		if cat.EventID != eventID {
			return ErrCategoryNotFound
		}

		now := time.Now()
		if err := checkoutEligible(event, cat, now); err != nil {
			return err
		}

		activeCount, err := tx.CountActiveOrdersForUserCategory(ctx, db.CountActiveOrdersForUserCategoryParams{
			CategoryID: categoryID, ParticipantID: participantID,
		})
		if err != nil {
			return err
		}
		if activeCount >= int64(cat.MaxOrderPerUser) {
			return ErrMaxOrderExceeded
		}

		number, err := s.uniqueOrderNumber(ctx, tx, now)
		if err != nil {
			return err
		}

		expiresAt := now.Add(s.ttl)
		order, err := tx.CreateOrder(ctx, db.CreateOrderParams{
			OrganizationID: cat.OrganizationID, EventID: eventID, CategoryID: categoryID,
			ParticipantID: participantID, OrderNumber: number, Status: StatusPendingPayment,
			Subtotal: cat.Price, Fee: 0, Discount: 0, Total: cat.Price,
			ExpiredAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		})
		if err != nil {
			return err
		}
		created = order

		if _, err := inv.Reserve(ctx, tx.Inventory(), db.CreateReservationParams{
			OrganizationID: cat.OrganizationID, EventID: eventID, CategoryID: categoryID,
			OrderID: order.ID, ParticipantID: participantID,
			ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return OrderResponse{}, err
	}

	if s.hook != nil {
		_ = s.hook.OnCheckoutComplete(ctx, participantID, created.EventID)
	}
	s.record(ctx, created, "ORDER_CREATED")
	s.recordReservation(ctx, created, "RESERVATION_CREATED")
	return toResponse(created), nil
}

func (s *Service) Cancel(ctx context.Context, participantID, orderID uuid.UUID) error {
	var cancelled db.Order
	err := s.repo.ExecTx(ctx, func(tx Repository) error {
		order, err := tx.GetOrderByID(ctx, orderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		} else if err != nil {
			return err
		}
		if order.ParticipantID != participantID {
			return ErrOrderNotFound
		}
		if order.Status != StatusPendingPayment {
			return ErrInvalidState
		}
		updated, err := tx.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID: orderID, Status: StatusCancelled, Status_2: StatusPendingPayment,
		})
		if err != nil {
			return err
		}
		cancelled = updated
		return inv.Release(ctx, tx.Inventory(), orderID, ReservationReleased)
	})
	if err != nil {
		return err
	}
	s.record(ctx, cancelled, "ORDER_CANCELLED")
	return nil
}

func (s *Service) GetForParticipant(ctx context.Context, participantID, orderID uuid.UUID) (OrderResponse, error) {
	order, err := s.repo.GetOrderByID(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderResponse{}, ErrOrderNotFound
	} else if err != nil {
		return OrderResponse{}, err
	}
	if order.ParticipantID != participantID {
		return OrderResponse{}, ErrOrderNotFound
	}
	return toResponse(order), nil
}

func (s *Service) ListForParticipant(ctx context.Context, participantID uuid.UUID) ([]OrderResponse, error) {
	rows, err := s.repo.ListOrdersByParticipant(ctx, participantID)
	if err != nil {
		return nil, err
	}
	return toResponses(rows), nil
}

func (s *Service) ListForOrgEvent(ctx context.Context, orgID, eventID uuid.UUID) ([]OrderResponse, error) {
	rows, err := s.repo.ListOrdersByOrgEvent(ctx, db.ListOrdersByOrgEventParams{
		OrganizationID: orgID, EventID: eventID,
	})
	if err != nil {
		return nil, err
	}
	return toResponses(rows), nil
}

func (s *Service) uniqueOrderNumber(ctx context.Context, tx Repository, now time.Time) (string, error) {
	for i := 0; i < 5; i++ {
		num, err := generateOrderNumber(now)
		if err != nil {
			return "", ErrOrderNumberGen
		}
		_, err = tx.GetOrderByNumber(ctx, num)
		if errors.Is(err, pgx.ErrNoRows) {
			return num, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", ErrOrderNumberGen
}

func (s *Service) record(ctx context.Context, order db.Order, action string) {
	if s.audit == nil {
		return
	}
	oid := order.OrganizationID
	uid := order.ParticipantID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid, ActorUserID: &uid, Action: action,
		TargetType: "order", TargetID: order.ID.String(),
	})
}

func (s *Service) recordReservation(ctx context.Context, order db.Order, action string) {
	if s.audit == nil {
		return
	}
	oid := order.OrganizationID
	uid := order.ParticipantID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid, ActorUserID: &uid, Action: action,
		TargetType: "reservation", TargetID: order.ID.String(),
	})
}

func toResponse(o db.Order) OrderResponse {
	r := OrderResponse{
		ID: o.ID, OrderNumber: o.OrderNumber, EventID: o.EventID, CategoryID: o.CategoryID,
		Status: o.Status, Subtotal: o.Subtotal, Fee: o.Fee, Discount: o.Discount, Total: o.Total,
		CreatedAt: o.CreatedAt.Time,
	}
	if o.ExpiredAt.Valid {
		v := o.ExpiredAt.Time
		r.ExpiredAt = &v
	}
	return r
}

func toResponses(rows []db.Order) []OrderResponse {
	out := make([]OrderResponse, 0, len(rows))
	for _, o := range rows {
		out = append(out, toResponse(o))
	}
	return out
}
