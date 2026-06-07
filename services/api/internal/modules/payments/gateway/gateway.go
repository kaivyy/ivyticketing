package gateway

import (
	"context"
	"net/http"
	"time"
)

type PaymentStatus string

const (
	StatusPending PaymentStatus = "PENDING"
	StatusPaid    PaymentStatus = "PAID"
	StatusExpired PaymentStatus = "EXPIRED"
	StatusFailed  PaymentStatus = "FAILED"
)

type CreateChargeInput struct {
	MerchantReference string
	Amount            int64
	Method            string
	Channel           string
	CustomerEmail     string
	CustomerName      string
	ExpiresAt         time.Time
}

type CreateChargeResult struct {
	GatewayReference string
	PayURL           string
	QRString         string
	VANumber         string
	Instructions     map[string]any
	ExpiresAt        time.Time
}

type CallbackResult struct {
	MerchantReference string
	GatewayReference  string
	Status            PaymentStatus
	Amount            int64
	PaidAt            *time.Time
	EventType         string
}

type Gateway interface {
	Name() string
	CreateCharge(ctx context.Context, in CreateChargeInput) (CreateChargeResult, error)
	VerifySignature(headers http.Header, rawBody []byte) bool
	ParseCallback(rawBody []byte) (CallbackResult, error)
	QueryStatus(ctx context.Context, gatewayReference string) (CallbackResult, error)
}
