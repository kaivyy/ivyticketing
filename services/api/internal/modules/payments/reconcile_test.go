package payments

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReconcile_PendingToPaid verifies that Reconcile queries the gateway,
// gets StatusPaid back, and transitions the payment + order to PAID.
func TestReconcile_PendingToPaid(t *testing.T) {
	repo := newFakeRepo()
	orderID := uuid.New()
	participantID := uuid.New()

	// Set up a PENDING payment
	pay := repo.addPayment(orderID, "PAY-REC-001", 50000, StatusPending)
	pay.ParticipantID = participantID
	repo.payments["PAY-REC-001"] = pay

	// Set up the order in PENDING_PAYMENT
	repo.addOrder(orderID, OrderPendingPayment)

	// Gateway returns StatusPaid
	paidAt := time.Now()
	fg := &fakeGateway{
		name: "xendit",
		queryResult: gw.CallbackResult{
			GatewayReference: "GW-REC-001",
			Status:           gw.StatusPaid,
			Amount:           50000,
			PaidAt:           &paidAt,
		},
	}

	reg := gw.NewRegistry()
	reg.Register(fg)

	proc := NewProcessor(repo, nil, nil)
	rec := NewReconciler(repo, reg, proc)

	ctx := context.Background()
	err := rec.Reconcile(ctx, pay.ID)
	require.NoError(t, err)

	// Payment should now be PAID
	assert.Equal(t, 1, repo.paymentPaidCount)
	// Order should be transitioned
	assert.Equal(t, 1, repo.orderPaidCount)
	// Reservations should be completed
	assert.Equal(t, 1, repo.reservationCompletedCount)

	// Verify the stored payment status
	var updatedPay interface{}
	_ = updatedPay
	for _, p := range repo.payments {
		if p.OrderID == orderID {
			assert.Equal(t, StatusPaid, p.Status)
		}
	}
}

// TestReconcile_PaymentNotFound verifies that Reconcile returns ErrPaymentNotFound
// for an unknown payment ID.
func TestReconcile_PaymentNotFound(t *testing.T) {
	repo := newFakeRepo()
	reg := gw.NewRegistry()
	proc := NewProcessor(repo, nil, nil)
	rec := NewReconciler(repo, reg, proc)

	err := rec.Reconcile(context.Background(), uuid.New())
	assert.ErrorIs(t, err, ErrPaymentNotFound)
}

// TestReconcile_GatewayNotAvailable verifies that Reconcile returns
// ErrGatewayNotAvail when the payment's gateway is not registered.
func TestReconcile_GatewayNotAvailable(t *testing.T) {
	repo := newFakeRepo()
	orderID := uuid.New()
	repo.addPayment(orderID, "PAY-REC-002", 50000, StatusPending)

	// Empty registry
	reg := gw.NewRegistry()
	proc := NewProcessor(repo, nil, nil)
	rec := NewReconciler(repo, reg, proc)

	// Look up the payment by its ID
	var payID uuid.UUID
	for _, p := range repo.payments {
		payID = p.ID
	}

	err := rec.Reconcile(context.Background(), payID)
	assert.ErrorIs(t, err, ErrGatewayNotAvail)
}
