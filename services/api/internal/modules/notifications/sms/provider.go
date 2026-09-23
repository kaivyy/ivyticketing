package sms

import (
	"context"
	"errors"
	"time"
)

var (
	ErrRateLimited        = errors.New("sms: rate limit exceeded for recipient")
	ErrUnsupportedChannel = errors.New("sms: channel not supported by provider")
	ErrProviderFailed     = errors.New("sms: provider delivery failed")
	ErrInvalidPhoneNumber = errors.New("sms: recipient phone number is invalid")
)

type Channel string

const (
	ChannelSMS      Channel = "sms"
	ChannelWhatsApp Channel = "whatsapp"
)

// Message is a vendor-agnostic outbound notification payload.
type Message struct {
	ID             string            `json:"id"`
	To             string            `json:"to"`
	Body           string            `json:"body"`
	Channel        Channel           `json:"channel"`
	TemplateName   string            `json:"templateName,omitempty"`
	TemplateParams map[string]string `json:"templateParams,omitempty"`
	IdempotencyKey string            `json:"idempotencyKey,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// DeliveryResult contains delivery telemetry returned by the provider.
type DeliveryResult struct {
	MessageID         string     `json:"messageId"`
	ProviderMessageID string     `json:"providerMessageId"`
	Status            string     `json:"status"` // "sent", "delivered", "queued", "failed"
	Channel           Channel    `json:"channel"`
	Provider          string     `json:"provider"`
	DeliveredAt       *time.Time `json:"deliveredAt,omitempty"`
	Cost              float64    `json:"cost,omitempty"`
}

// Provider is the pluggable vendor-agnostic adapter interface.
type Provider interface {
	Name() string
	Send(ctx context.Context, msg Message) (DeliveryResult, error)
	SupportsChannel(ch Channel) bool
}
