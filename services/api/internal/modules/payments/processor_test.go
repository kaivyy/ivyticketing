package payments

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo is an in-memory Repository for testing.
type fakeRepo struct {
	payments map[string]db.Payment // keyed by merchantReference
	orders   map[uuid.UUID]db.Order
	webhooks map[uuid.UUID]db.PaymentWebhook
	dedupe   map[string]bool // set of claimed dedupe keys

	orderPaidCount           int
	reservationCompletedCount int
	paymentPaidCount         int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		payments: make(map[string]db.Payment),
		orders:   make(map[uuid.UUID]db.Order),
		webhooks: make(map[uuid.UUID]db.PaymentWebhook),
		dedupe:   make(map[string]bool),
	}
}

func (f *fakeRepo) ExecTx(_ context.Context, fn func(Repository) error) error {
	return fn(f)
}

func (f *fakeRepo) Querier() *db.Queries { return nil }

func (f *fakeRepo) CreatePayment(_ context.Context, arg db.CreatePaymentParams) (db.Payment, error) {
	p := db.Payment{
		ID:                uuid.New(),
		OrganizationID:    arg.OrganizationID,
		EventID:           arg.EventID,
		OrderID:           arg.OrderID,
		ParticipantID:     arg.ParticipantID,
		Gateway:           arg.Gateway,
		Method:            arg.Method,
		Channel:           arg.Channel,
		Status:            arg.Status,
		Amount:            arg.Amount,
		Currency:          arg.Currency,
		GatewayReference:  arg.GatewayReference,
		MerchantReference: arg.MerchantReference,
		PayUrl:            arg.PayUrl,
		QrString:          arg.QrString,
		VaNumber:          arg.VaNumber,
		ExpiresAt:         arg.ExpiresAt,
	}
	f.payments[arg.MerchantReference] = p
	return p, nil
}

func (f *fakeRepo) GetPaymentByID(_ context.Context, id uuid.UUID) (db.Payment, error) {
	for _, p := range f.payments {
		if p.ID == id {
			return p, nil
		}
	}
	return db.Payment{}, pgx.ErrNoRows
}

func (f *fakeRepo) GetPaymentByMerchantRefForUpdate(_ context.Context, ref string) (db.Payment, error) {
	p, ok := f.payments[ref]
	if !ok {
		return db.Payment{}, pgx.ErrNoRows
	}
	return p, nil
}

func (f *fakeRepo) ListPaymentsByOrder(_ context.Context, _ uuid.UUID) ([]db.Payment, error) {
	return nil, nil
}

func (f *fakeRepo) ListPaymentsByOrgEvent(_ context.Context, _ db.ListPaymentsByOrgEventParams) ([]db.Payment, error) {
	return nil, nil
}

func (f *fakeRepo) GetActivePaymentByOrder(_ context.Context, orderID uuid.UUID) (db.Payment, error) {
	for _, p := range f.payments {
		if p.OrderID == orderID && (p.Status == StatusPending || p.Status == StatusPaid) {
			return p, nil
		}
	}
	return db.Payment{}, pgx.ErrNoRows
}

func (f *fakeRepo) MarkPaymentPaid(_ context.Context, arg db.MarkPaymentPaidParams) (db.Payment, error) {
	for ref, p := range f.payments {
		if p.ID == arg.ID {
			if p.Status != StatusPending {
				return db.Payment{}, pgx.ErrNoRows // already final
			}
			p.Status = StatusPaid
			p.PaidAt = arg.PaidAt
			if arg.GatewayReference.Valid {
				p.GatewayReference = arg.GatewayReference
			}
			f.payments[ref] = p
			f.paymentPaidCount++
			return p, nil
		}
	}
	return db.Payment{}, pgx.ErrNoRows
}

func (f *fakeRepo) UpdatePaymentStatus(_ context.Context, arg db.UpdatePaymentStatusParams) (db.Payment, error) {
	for ref, p := range f.payments {
		if p.ID == arg.ID && p.Status == StatusPending {
			p.Status = arg.Status
			f.payments[ref] = p
			return p, nil
		}
	}
	return db.Payment{}, pgx.ErrNoRows
}

func (f *fakeRepo) CreatePaymentWebhook(_ context.Context, arg db.CreatePaymentWebhookParams) (db.PaymentWebhook, error) {
	wh := db.PaymentWebhook{
		ID:               uuid.New(),
		Gateway:          arg.Gateway,
		ProcessingStatus: arg.ProcessingStatus,
		Payload:          arg.Payload,
	}
	f.webhooks[wh.ID] = wh
	return wh, nil
}

func (f *fakeRepo) ClaimWebhookDedupe(_ context.Context, id uuid.UUID, dedupeKey string) error {
	if f.dedupe[dedupeKey] {
		return ErrDuplicateDedupe
	}
	f.dedupe[dedupeKey] = true
	return nil
}

func (f *fakeRepo) MarkWebhookProcessed(_ context.Context, arg db.MarkWebhookProcessedParams) error {
	wh, ok := f.webhooks[arg.ID]
	if !ok {
		return nil
	}
	wh.ProcessingStatus = arg.ProcessingStatus
	f.webhooks[arg.ID] = wh
	return nil
}

func (f *fakeRepo) GetOrderByIDForUpdate(_ context.Context, id uuid.UUID) (db.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return db.Order{}, pgx.ErrNoRows
	}
	return o, nil
}

func (f *fakeRepo) UpdateOrderStatus(_ context.Context, arg db.UpdateOrderStatusParams) (db.Order, error) {
	o, ok := f.orders[arg.ID]
	if !ok {
		return db.Order{}, pgx.ErrNoRows
	}
	if o.Status != arg.Status_2 {
		// Guard condition not met — no rows updated
		return db.Order{}, pgx.ErrNoRows
	}
	o.Status = arg.Status
	f.orders[arg.ID] = o
	f.orderPaidCount++
	return o, nil
}

func (f *fakeRepo) CompleteReservationsForOrder(_ context.Context, _ uuid.UUID) error {
	f.reservationCompletedCount++
	return nil
}

// Helpers to add test data
func (f *fakeRepo) addPayment(orderID uuid.UUID, ref string, amount int64, status string) db.Payment {
	p := db.Payment{
		ID:                uuid.New(),
		OrganizationID:    uuid.New(),
		EventID:           uuid.New(),
		OrderID:           orderID,
		ParticipantID:     uuid.New(),
		Gateway:           "xendit",
		Method:            "qris",
		Status:            status,
		Amount:            amount,
		Currency:          "IDR",
		MerchantReference: ref,
	}
	f.payments[ref] = p
	return p
}

func (f *fakeRepo) addOrder(id uuid.UUID, status string) db.Order {
	o := db.Order{
		ID:     id,
		Status: status,
		Total:  50000,
	}
	f.orders[id] = o
	return o
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestProcessCallback_PaidTransitionsOrderOnce verifies that a PAID callback
// transitions the order to PAID and completes reservations exactly once.
// Calling Apply a second time is a no-op (idempotent).
func TestProcessCallback_PaidTransitionsOrderOnce(t *testing.T) {
	repo := newFakeRepo()
	orderID := uuid.New()
	pay := repo.addPayment(orderID, "PAY-REF-001", 50000, StatusPending)
	_ = pay
	repo.addOrder(orderID, OrderPendingPayment)

	proc := NewProcessor(repo, nil, nil)
	ctx := context.Background()

	paidAt := time.Now()
	res := gw.CallbackResult{
		MerchantReference: "PAY-REF-001",
		GatewayReference:  "GW-001",
		Status:            gw.StatusPaid,
		Amount:            50000,
		PaidAt:            &paidAt,
	}

	// First apply → should succeed
	err := proc.Apply(ctx, "xendit", res)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.paymentPaidCount, "payment should be marked paid once")
	assert.Equal(t, 1, repo.orderPaidCount, "order should be transitioned once")
	assert.Equal(t, 1, repo.reservationCompletedCount, "reservations should be completed once")

	// Second apply → idempotent, payment already PAID so no second transition
	err = proc.Apply(ctx, "xendit", res)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.paymentPaidCount, "payment paid count should remain 1")
	assert.Equal(t, 1, repo.orderPaidCount, "order paid count should remain 1")
	assert.Equal(t, 1, repo.reservationCompletedCount, "reservation count should remain 1")
}

// TestProcessCallback_AmountMismatchRejected verifies that a callback with a
// different amount returns ErrAmountMismatch and does not transition the order.
func TestProcessCallback_AmountMismatchRejected(t *testing.T) {
	repo := newFakeRepo()
	orderID := uuid.New()
	repo.addPayment(orderID, "PAY-REF-002", 50000, StatusPending)
	repo.addOrder(orderID, OrderPendingPayment)

	proc := NewProcessor(repo, nil, nil)
	ctx := context.Background()

	res := gw.CallbackResult{
		MerchantReference: "PAY-REF-002",
		GatewayReference:  "GW-002",
		Status:            gw.StatusPaid,
		Amount:            99999, // wrong amount
	}

	err := proc.Apply(ctx, "xendit", res)
	assert.ErrorIs(t, err, ErrAmountMismatch)
	assert.Equal(t, 0, repo.orderPaidCount, "order should not be transitioned")
}

// TestProcessCallback_PaymentNotFound verifies that an unknown merchant ref
// returns ErrPaymentNotFound.
func TestProcessCallback_PaymentNotFound(t *testing.T) {
	repo := newFakeRepo()
	proc := NewProcessor(repo, nil, nil)
	ctx := context.Background()

	res := gw.CallbackResult{
		MerchantReference: "UNKNOWN-REF",
		Status:            gw.StatusPaid,
		Amount:            50000,
	}

	err := proc.Apply(ctx, "xendit", res)
	assert.ErrorIs(t, err, ErrPaymentNotFound)
}

// TestProcessCallback_OrderAlreadyExpired verifies that when a payment is PENDING
// but the order is EXPIRED, the payment is still marked PAID but the order stays
// EXPIRED (because the UpdateOrderStatus guard condition fails on status mismatch).
func TestProcessCallback_OrderAlreadyExpired(t *testing.T) {
	repo := newFakeRepo()
	orderID := uuid.New()
	repo.addPayment(orderID, "PAY-REF-003", 50000, StatusPending)
	repo.addOrder(orderID, "EXPIRED") // order already expired

	proc := NewProcessor(repo, nil, nil)
	ctx := context.Background()

	paidAt := time.Now()
	res := gw.CallbackResult{
		MerchantReference: "PAY-REF-003",
		GatewayReference:  "GW-003",
		Status:            gw.StatusPaid,
		Amount:            50000,
		PaidAt:            &paidAt,
	}

	// Payment is PENDING so it gets marked PAID; order stays EXPIRED.
	// UpdateOrderStatus guard (Status_2 == PENDING_PAYMENT) fails → no-op, note set.
	err := proc.Apply(ctx, "xendit", res)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.paymentPaidCount, "payment should be marked PAID")
	assert.Equal(t, 0, repo.orderPaidCount, "expired order should not be transitioned")

	// Order status should still be EXPIRED
	order, ok := repo.orders[orderID]
	require.True(t, ok)
	assert.Equal(t, "EXPIRED", order.Status)

	// Payment should now be PAID
	var paidPayment db.Payment
	for _, p := range repo.payments {
		if p.OrderID == orderID {
			paidPayment = p
		}
	}
	assert.Equal(t, StatusPaid, paidPayment.Status)

	// Reservations should NOT be completed since order wasn't transitioned
	assert.Equal(t, 0, repo.reservationCompletedCount)
}

// pgtype.Timestamptz helper used in tests
var _ = pgtype.Timestamptz{}
