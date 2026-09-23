package sms

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// LogProvider logs all outbound messages without dispatching across real network gateways.
type LogProvider struct {
	NameStr string
	Log     *slog.Logger
}

// NewLogProvider initializes a LogProvider.
func NewLogProvider(log *slog.Logger) *LogProvider {
	if log == nil {
		log = slog.Default()
	}
	return &LogProvider{
		NameStr: "log_mock",
		Log:     log,
	}
}

func (p *LogProvider) Name() string {
	return p.NameStr
}

func (p *LogProvider) SupportsChannel(ch Channel) bool {
	return ch == ChannelSMS || ch == ChannelWhatsApp
}

func (p *LogProvider) Send(ctx context.Context, msg Message) (DeliveryResult, error) {
	masked := MaskPhoneNumber(msg.To)
	providerMsgID := fmt.Sprintf("mock-%s-%s", msg.Channel, uuid.NewString()[:8])
	now := time.Now()

	p.Log.Info("sms/whatsapp outbound dispatch",
		"provider", p.NameStr,
		"channel", string(msg.Channel),
		"to", masked,
		"provider_message_id", providerMsgID,
		"idempotency_key", msg.IdempotencyKey,
		"body_length", len(msg.Body),
	)

	return DeliveryResult{
		MessageID:         msg.ID,
		ProviderMessageID: providerMsgID,
		Status:            "delivered",
		Channel:           msg.Channel,
		Provider:          p.NameStr,
		DeliveredAt:       &now,
		Cost:              0.0,
	}, nil
}
