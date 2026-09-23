package orders

import (
	"time"

	"github.com/google/uuid"
)

type OrderResponse struct {
	ID             uuid.UUID      `json:"id"`
	OrderNumber    string         `json:"orderNumber"`
	EventID        uuid.UUID      `json:"eventId"`
	CategoryID     uuid.UUID      `json:"categoryId"`
	ParticipantID  *uuid.UUID     `json:"participantId,omitempty"`
	Status         string         `json:"status"`
	Subtotal       int64          `json:"subtotal"`
	Fee            int64          `json:"fee"`
	Discount       int64          `json:"discount"`
	Total          int64          `json:"total"`
	GuestEmail     *string        `json:"guestEmail,omitempty"`
	GuestName      *string        `json:"guestName,omitempty"`
	ExpiredAt      *time.Time     `json:"expiredAt"`
	CreatedAt      time.Time      `json:"createdAt"`
	FormAnswers    map[string]any `json:"formAnswers,omitempty"`
	RefundedAmount int64          `json:"refundedAmount"`
	RefundReason   *string        `json:"refundReason,omitempty"`
	RefundedAt     *time.Time     `json:"refundedAt,omitempty"`
}

type GuestCheckoutRequest struct {
	GuestEmail     string         `json:"guestEmail"`
	GuestName      string         `json:"guestName"`
	GuestPhone     string         `json:"guestPhone"`
	TermsAccepted  bool           `json:"termsAccepted"`
	TermsVersion   string         `json:"termsVersion"`
	WaiverAccepted bool           `json:"waiverAccepted"`
	WaiverVersion  string         `json:"waiverVersion"`
	AdmissionToken string         `json:"admissionToken,omitempty"`
	FormAnswers    map[string]any `json:"formAnswers,omitempty"`
	Answers        map[string]any `json:"answers,omitempty"`
}

type ClaimOrderRequest struct {
	OrderID uuid.UUID `json:"orderId"`
}

type RefundOrderRequest struct {
	Amount int64  `json:"amount"`
	Reason string `json:"reason"`
}
