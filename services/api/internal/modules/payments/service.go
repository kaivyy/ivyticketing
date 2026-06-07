package payments

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

var validMethods = map[string]bool{"qris": true, "va": true, "ewallet": true}

// Service handles payment creation and retrieval.
type Service struct {
	repo     Repository
	registry *gw.Registry
	audit    AuditRecorder
	expiry   time.Duration
}

func NewService(repo Repository, registry *gw.Registry, recorder AuditRecorder, expiry time.Duration) *Service {
	return &Service{repo: repo, registry: registry, audit: recorder, expiry: expiry}
}

// CreatePayment validates eligibility, calls the gateway to create a charge,
// and persists the resulting payment row.
func (s *Service) CreatePayment(ctx context.Context, participantID, orderID uuid.UUID, req CreatePaymentRequest) (PaymentResponse, error) {
	if !validMethods[req.Method] {
		return PaymentResponse{}, ErrUnsupportedMethod
	}
	g, ok := s.registry.Get(req.Gateway)
	if !ok {
		return PaymentResponse{}, ErrGatewayNotAvail
	}

	order, err := s.repo.GetOrderByIDForUpdate(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentResponse{}, ErrPaymentNotFound
	} else if err != nil {
		return PaymentResponse{}, err
	}
	if order.ParticipantID != participantID {
		return PaymentResponse{}, ErrPaymentNotFound
	}
	if order.Status != OrderPendingPayment {
		return PaymentResponse{}, ErrOrderNotPayable
	}
	if _, err := s.repo.GetActivePaymentByOrder(ctx, orderID); err == nil {
		return PaymentResponse{}, ErrPaymentActive
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return PaymentResponse{}, err
	}

	now := time.Now()
	ref, err := generateMerchantReference(now)
	if err != nil {
		return PaymentResponse{}, ErrMerchantRefGen
	}
	expiresAt := now.Add(s.expiry)
	if order.ExpiredAt.Valid && order.ExpiredAt.Time.Before(expiresAt) {
		expiresAt = order.ExpiredAt.Time
	}

	charge, err := g.CreateCharge(ctx, gw.CreateChargeInput{
		MerchantReference: ref,
		Amount:            order.Total,
		Method:            req.Method,
		Channel:           req.Channel,
		ExpiresAt:         expiresAt,
	})
	if err != nil {
		return PaymentResponse{}, ErrGatewayError
	}

	payment, err := s.repo.CreatePayment(ctx, db.CreatePaymentParams{
		OrganizationID:    order.OrganizationID,
		EventID:           order.EventID,
		OrderID:           order.ID,
		ParticipantID:     participantID,
		Gateway:           req.Gateway,
		Method:            req.Method,
		Channel:           nullText(req.Channel),
		Status:            StatusPending,
		Amount:            order.Total,
		Currency:          "IDR",
		GatewayReference:  nullText(charge.GatewayReference),
		MerchantReference: ref,
		PayUrl:            nullText(charge.PayURL),
		QrString:          nullText(charge.QRString),
		VaNumber:          nullText(charge.VANumber),
		ExpiresAt:         pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return PaymentResponse{}, err
	}

	s.recordCreated(ctx, payment)
	return toResponse(payment), nil
}

// GetForParticipant returns a payment by ID, enforcing ownership.
func (s *Service) GetForParticipant(ctx context.Context, participantID, paymentID uuid.UUID) (PaymentResponse, error) {
	pay, err := s.repo.GetPaymentByID(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentResponse{}, ErrPaymentNotFound
	} else if err != nil {
		return PaymentResponse{}, err
	}
	if pay.ParticipantID != participantID {
		return PaymentResponse{}, ErrPaymentNotFound
	}
	return toResponse(pay), nil
}

// ListByOrder returns all payments for an order filtered by participant ownership.
func (s *Service) ListByOrder(ctx context.Context, participantID, orderID uuid.UUID) ([]PaymentResponse, error) {
	rows, err := s.repo.ListPaymentsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentResponse, 0, len(rows))
	for _, p := range rows {
		if p.ParticipantID == participantID {
			out = append(out, toResponse(p))
		}
	}
	return out, nil
}

// ListForOrgEvent returns all payments for an event (organizer view).
func (s *Service) ListForOrgEvent(ctx context.Context, orgID, eventID uuid.UUID) ([]PaymentResponse, error) {
	rows, err := s.repo.ListPaymentsByOrgEvent(ctx, db.ListPaymentsByOrgEventParams{
		OrganizationID: orgID,
		EventID:        eventID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]PaymentResponse, 0, len(rows))
	for _, p := range rows {
		out = append(out, toResponse(p))
	}
	return out, nil
}

func (s *Service) recordCreated(ctx context.Context, p db.Payment) {
	if s.audit == nil {
		return
	}
	oid := p.OrganizationID
	uid := p.ParticipantID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid,
		ActorUserID:    &uid,
		Action:         "PAYMENT_CREATED",
		TargetType:     "payment",
		TargetID:       p.ID.String(),
	})
}
