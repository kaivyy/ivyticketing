package payments

import (
	"time"

	"github.com/google/uuid"
	"github.com/varin/ivyticketing/services/api/internal/db"
)

type CreatePaymentRequest struct {
	Gateway string `json:"gateway"`
	Method  string `json:"method"`
	Channel string `json:"channel,omitempty"`
}

type PaymentResponse struct {
	ID                uuid.UUID  `json:"id"`
	OrderID           uuid.UUID  `json:"orderId"`
	Gateway           string     `json:"gateway"`
	Method            string     `json:"method"`
	Channel           string     `json:"channel,omitempty"`
	Status            string     `json:"status"`
	Amount            int64      `json:"amount"`
	Currency          string     `json:"currency"`
	MerchantReference string     `json:"merchantReference"`
	GatewayReference  string     `json:"gatewayReference,omitempty"`
	PayURL            string     `json:"payUrl,omitempty"`
	QRString          string     `json:"qrString,omitempty"`
	VANumber          string     `json:"vaNumber,omitempty"`
	ExpiresAt         *time.Time `json:"expiresAt,omitempty"`
	PaidAt            *time.Time `json:"paidAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
}

func toResponse(p db.Payment) PaymentResponse {
	r := PaymentResponse{
		ID:                p.ID,
		OrderID:           p.OrderID,
		Gateway:           p.Gateway,
		Method:            p.Method,
		Status:            p.Status,
		Amount:            p.Amount,
		Currency:          p.Currency,
		MerchantReference: p.MerchantReference,
		CreatedAt:         p.CreatedAt.Time,
	}
	if p.Channel.Valid {
		r.Channel = p.Channel.String
	}
	if p.GatewayReference.Valid {
		r.GatewayReference = p.GatewayReference.String
	}
	if p.PayUrl.Valid {
		r.PayURL = p.PayUrl.String
	}
	if p.QrString.Valid {
		r.QRString = p.QrString.String
	}
	if p.VaNumber.Valid {
		r.VANumber = p.VaNumber.String
	}
	if p.ExpiresAt.Valid {
		v := p.ExpiresAt.Time
		r.ExpiresAt = &v
	}
	if p.PaidAt.Valid {
		v := p.PaidAt.Time
		r.PaidAt = &v
	}
	return r
}
