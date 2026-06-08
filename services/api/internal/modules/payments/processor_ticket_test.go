package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/varin/ivyticketing/services/api/internal/db"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

// countingIssuer records calls and can be forced to fail.
type countingIssuer struct {
	calls   int
	failNow bool
}

func (c *countingIssuer) IssueForOrder(_ context.Context, _ *db.Queries, _ db.Order) error {
	c.calls++
	if c.failNow {
		return errors.New("forced issuer failure")
	}
	return nil
}

// snapshotRepo wraps fakeRepo and restores in-memory state if the transaction
// function returns an error, simulating DB transaction rollback semantics.
type snapshotRepo struct {
	*fakeRepo
}

func (s *snapshotRepo) ExecTx(ctx context.Context, fn func(Repository) error) error {
	// Snapshot mutable state before running fn.
	paySnap := make(map[string]db.Payment, len(s.payments))
	for k, v := range s.payments {
		paySnap[k] = v
	}
	ordSnap := make(map[uuid.UUID]db.Order, len(s.orders))
	for k, v := range s.orders {
		ordSnap[k] = v
	}
	whSnap := make(map[uuid.UUID]db.PaymentWebhook, len(s.webhooks))
	for k, v := range s.webhooks {
		whSnap[k] = v
	}
	savedOrderPaid := s.orderPaidCount
	savedResvCompleted := s.reservationCompletedCount
	savedPaymentPaid := s.paymentPaidCount

	if err := fn(s); err != nil {
		// Rollback: restore snapshots.
		s.payments = paySnap
		s.orders = ordSnap
		s.webhooks = whSnap
		s.orderPaidCount = savedOrderPaid
		s.reservationCompletedCount = savedResvCompleted
		s.paymentPaidCount = savedPaymentPaid
		return err
	}
	return nil
}

// Querier delegates to the embedded fakeRepo.
func (s *snapshotRepo) Querier() *db.Queries { return nil }

func newSnapshotRepo() *snapshotRepo {
	return &snapshotRepo{fakeRepo: newFakeRepo()}
}

// TestProcessor_ApplyPaid_IssuerCalledOnce verifies that on a PAID callback the
// ticket issuer is called exactly once, and that a second identical Apply call
// (idempotent re-apply) does NOT call the issuer again.
func TestProcessor_ApplyPaid_IssuerCalledOnce(t *testing.T) {
	repo := newSnapshotRepo()
	orderID := uuid.New()
	repo.addPayment(orderID, "TICKET-REF-001", 75000, StatusPending)
	repo.addOrder(orderID, OrderPendingPayment)

	issuer := &countingIssuer{}
	proc := NewProcessor(repo, nil, issuer)
	ctx := context.Background()

	paidAt := time.Now()
	res := gw.CallbackResult{
		MerchantReference: "TICKET-REF-001",
		GatewayReference:  "GW-TICKET-001",
		Status:            gw.StatusPaid,
		Amount:            75000,
		PaidAt:            &paidAt,
	}

	// First apply → issuer called once, order becomes PAID.
	err := proc.Apply(ctx, "xendit", res)
	require.NoError(t, err)
	assert.Equal(t, 1, issuer.calls, "issuer should be called exactly once on first apply")
	assert.Equal(t, 1, repo.orderPaidCount, "order should be marked PAID once")
	order, ok := repo.orders[orderID]
	require.True(t, ok)
	assert.Equal(t, OrderPaid, order.Status, "order status should be PAID after first apply")

	// Second apply → payment already PAID, so the processor short-circuits before
	// calling the issuer again (idempotent).
	err = proc.Apply(ctx, "xendit", res)
	require.NoError(t, err)
	assert.Equal(t, 1, issuer.calls, "issuer should NOT be called again on idempotent re-apply")
	assert.Equal(t, 1, repo.orderPaidCount, "order paid count should remain 1")
}

// TestProcessor_ApplyPaid_IssuerError_RollsBack verifies that when the ticket
// issuer returns an error the entire transaction is rolled back: the order stays
// in PENDING_PAYMENT and the payment stays PENDING.
func TestProcessor_ApplyPaid_IssuerError_RollsBack(t *testing.T) {
	repo := newSnapshotRepo()
	orderID := uuid.New()
	repo.addPayment(orderID, "TICKET-REF-002", 75000, StatusPending)
	repo.addOrder(orderID, OrderPendingPayment)

	issuer := &countingIssuer{failNow: true}
	proc := NewProcessor(repo, nil, issuer)
	ctx := context.Background()

	paidAt := time.Now()
	res := gw.CallbackResult{
		MerchantReference: "TICKET-REF-002",
		GatewayReference:  "GW-TICKET-002",
		Status:            gw.StatusPaid,
		Amount:            75000,
		PaidAt:            &paidAt,
	}

	err := proc.Apply(ctx, "xendit", res)
	require.Error(t, err, "issuer failure should propagate as an error")
	assert.Equal(t, 1, issuer.calls, "issuer should have been called once before failing")

	// State must be rolled back — order still PENDING_PAYMENT, payment still PENDING.
	assert.Equal(t, 0, repo.orderPaidCount, "order paid count must be 0 after rollback")
	assert.Equal(t, 0, repo.paymentPaidCount, "payment paid count must be 0 after rollback")
	assert.Equal(t, 0, repo.reservationCompletedCount, "reservation count must be 0 after rollback")

	order, ok := repo.orders[orderID]
	require.True(t, ok)
	assert.Equal(t, OrderPendingPayment, order.Status, "order must remain PENDING_PAYMENT after rollback")

	var pay db.Payment
	for _, p := range repo.payments {
		if p.OrderID == orderID {
			pay = p
		}
	}
	assert.Equal(t, StatusPending, pay.Status, "payment must remain PENDING after rollback")
}
