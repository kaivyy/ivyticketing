package payments

import gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"

const (
	StatusPending = "PENDING"
	StatusPaid    = "PAID"
	StatusExpired = "EXPIRED"
	StatusFailed  = "FAILED"
)

const (
	WebhookReceived  = "RECEIVED"
	WebhookProcessed = "PROCESSED"
	WebhookRejected  = "REJECTED"
	WebhookDuplicate = "DUPLICATE"
	WebhookFailed    = "FAILED"
)

const ReservationCompleted = "COMPLETED"

const (
	OrderPendingPayment = "PENDING_PAYMENT"
	OrderPaid           = "PAID"
)

func dbStatusFromGateway(s gw.PaymentStatus) string {
	switch s {
	case gw.StatusPaid:
		return StatusPaid
	case gw.StatusExpired:
		return StatusExpired
	case gw.StatusFailed:
		return StatusFailed
	default:
		return StatusPending
	}
}
