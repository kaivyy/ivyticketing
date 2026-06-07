package payments

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGateway is a test stub that implements gw.Gateway.
type fakeGateway struct {
	name          string
	chargeResult  gw.CreateChargeResult
	chargeErr     error
	queryResult   gw.CallbackResult
	queryErr      error
}

func (f *fakeGateway) Name() string { return f.name }

func (f *fakeGateway) CreateCharge(_ context.Context, _ gw.CreateChargeInput) (gw.CreateChargeResult, error) {
	return f.chargeResult, f.chargeErr
}

func (f *fakeGateway) VerifySignature(_ http.Header, _ []byte) bool { return true }

func (f *fakeGateway) ParseCallback(_ []byte) (gw.CallbackResult, error) {
	return gw.CallbackResult{}, nil
}

func (f *fakeGateway) QueryStatus(_ context.Context, _ string) (gw.CallbackResult, error) {
	return f.queryResult, f.queryErr
}

// TestCreatePayment_OrderNotPayable verifies that CreatePayment returns
// ErrOrderNotPayable when the order is not in PENDING_PAYMENT state.
func TestCreatePayment_OrderNotPayable(t *testing.T) {
	repo := newFakeRepo()
	orderID := uuid.New()
	participantID := uuid.New()

	// Order is already PAID
	o := repo.addOrder(orderID, "PAID")
	o.ParticipantID = participantID
	repo.orders[orderID] = o

	reg := gw.NewRegistry()
	fg := &fakeGateway{name: "xendit"}
	reg.Register(fg)

	svc := NewService(repo, reg, nil, 15*time.Minute)
	ctx := context.Background()

	_, err := svc.CreatePayment(ctx, participantID, orderID, CreatePaymentRequest{
		Gateway: "xendit",
		Method:  "qris",
	})
	assert.ErrorIs(t, err, ErrOrderNotPayable)
}

// TestCreatePayment_GatewayNotAvailable verifies that CreatePayment returns
// ErrGatewayNotAvail when the requested gateway is not registered.
func TestCreatePayment_GatewayNotAvailable(t *testing.T) {
	repo := newFakeRepo()
	orderID := uuid.New()
	participantID := uuid.New()

	o := repo.addOrder(orderID, OrderPendingPayment)
	o.ParticipantID = participantID
	repo.orders[orderID] = o

	// Empty registry — no gateways registered
	reg := gw.NewRegistry()

	svc := NewService(repo, reg, nil, 15*time.Minute)
	ctx := context.Background()

	_, err := svc.CreatePayment(ctx, participantID, orderID, CreatePaymentRequest{
		Gateway: "xendit",
		Method:  "qris",
	})
	assert.ErrorIs(t, err, ErrGatewayNotAvail)
}

// TestCreatePayment_Success verifies that a valid CreatePayment call stores the
// payment and returns a PaymentResponse with the expected fields.
func TestCreatePayment_Success(t *testing.T) {
	repo := newFakeRepo()
	orderID := uuid.New()
	participantID := uuid.New()

	o := repo.addOrder(orderID, OrderPendingPayment)
	o.ParticipantID = participantID
	o.Total = 75000
	repo.orders[orderID] = o

	reg := gw.NewRegistry()
	fg := &fakeGateway{
		name: "xendit",
		chargeResult: gw.CreateChargeResult{
			GatewayReference: "GW-XEN-001",
			QRString:         "qr-data-here",
		},
	}
	reg.Register(fg)

	svc := NewService(repo, reg, nil, 15*time.Minute)
	ctx := context.Background()

	resp, err := svc.CreatePayment(ctx, participantID, orderID, CreatePaymentRequest{
		Gateway: "xendit",
		Method:  "qris",
	})
	require.NoError(t, err)
	assert.Equal(t, "xendit", resp.Gateway)
	assert.Equal(t, "qris", resp.Method)
	assert.Equal(t, int64(75000), resp.Amount)
	assert.Equal(t, StatusPending, resp.Status)
	assert.Equal(t, "GW-XEN-001", resp.GatewayReference)
	assert.Equal(t, "qr-data-here", resp.QRString)
}
