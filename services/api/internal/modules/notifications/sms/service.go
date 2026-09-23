package sms

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ServiceConfig provides dependency configuration for MessagingService.
type ServiceConfig struct {
	PrimaryProvider   Provider
	FallbackProvider  Provider
	IdempotencyStore  IdempotencyStore
	RateLimiter       RateLimiter
	IdempotencyTTL    time.Duration
	Log               *slog.Logger
}

// MessagingService coordinates SMS and WhatsApp outbound messaging with fallback, rate limiting, and idempotency.
type MessagingService struct {
	primary   Provider
	fallback  Provider
	idemStore IdempotencyStore
	limiter   RateLimiter
	idemTTL   time.Duration
	log       *slog.Logger
}

// NewMessagingService constructs a new MessagingService.
func NewMessagingService(cfg ServiceConfig) *MessagingService {
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	idemStore := cfg.IdempotencyStore
	if idemStore == nil {
		idemStore = NewMemoryIdempotencyStore()
	}
	limiter := cfg.RateLimiter
	if limiter == nil {
		limiter = NewMemoryRateLimiter(10, time.Minute) // 10 msgs/min per recipient
	}
	ttl := cfg.IdempotencyTTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	primary := cfg.PrimaryProvider
	if primary == nil {
		primary = NewLogProvider(log)
	}

	return &MessagingService{
		primary:   primary,
		fallback:  cfg.FallbackProvider,
		idemStore: idemStore,
		limiter:   limiter,
		idemTTL:   ttl,
		log:       log,
	}
}

// Send dispatches a message with rate limiting, idempotency check, and fallback logic.
func (s *MessagingService) Send(ctx context.Context, msg Message) (DeliveryResult, error) {
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	if msg.To == "" {
		return DeliveryResult{}, ErrInvalidPhoneNumber
	}

	// 1. Rate-limit guard per recipient
	if !s.limiter.Allow(msg.To) {
		s.log.Warn("sms dispatch rate limited", "to", MaskPhoneNumber(msg.To))
		return DeliveryResult{}, ErrRateLimited
	}

	// 2. Idempotency key deduplication
	if msg.IdempotencyKey != "" {
		if cached, ok := s.idemStore.Get(msg.IdempotencyKey); ok {
			s.log.Info("sms dispatch idempotency cache hit",
				"idempotency_key", msg.IdempotencyKey,
				"to", MaskPhoneNumber(msg.To),
				"provider_msg_id", cached.ProviderMessageID,
			)
			return cached, nil
		}
	}

	masked := MaskPhoneNumber(msg.To)
	start := time.Now()

	// 3. Attempt Primary Provider
	var res DeliveryResult
	var sendErr error

	if s.primary.SupportsChannel(msg.Channel) {
		res, sendErr = s.primary.Send(ctx, msg)
	} else {
		sendErr = fmt.Errorf("%w: primary provider %s does not support %s", ErrUnsupportedChannel, s.primary.Name(), msg.Channel)
	}

	// 4. Fallback mechanism if primary failed and fallback provider is configured
	if sendErr != nil && s.fallback != nil {
		s.log.Warn("sms primary provider failed, attempting fallback",
			"primary", s.primary.Name(),
			"fallback", s.fallback.Name(),
			"to", masked,
			"channel", string(msg.Channel),
			"err", sendErr,
		)

		fallbackMsg := msg
		// If original was WhatsApp and fallback only supports SMS, degrade channel to SMS
		if fallbackMsg.Channel == ChannelWhatsApp && !s.fallback.SupportsChannel(ChannelWhatsApp) && s.fallback.SupportsChannel(ChannelSMS) {
			s.log.Info("sms adapting channel from whatsapp to sms for fallback", "to", masked)
			fallbackMsg.Channel = ChannelSMS
		}

		if s.fallback.SupportsChannel(fallbackMsg.Channel) {
			res, sendErr = s.fallback.Send(ctx, fallbackMsg)
		} else {
			sendErr = fmt.Errorf("%w: fallback provider %s does not support %s", ErrUnsupportedChannel, s.fallback.Name(), fallbackMsg.Channel)
		}
	}

	duration := time.Since(start)

	if sendErr != nil {
		s.log.Error("sms dispatch failed permanently",
			"to", masked,
			"channel", string(msg.Channel),
			"duration_ms", duration.Milliseconds(),
			"err", sendErr,
		)
		return DeliveryResult{}, sendErr
	}

	// 5. Success audit logging
	s.log.Info("sms dispatch succeeded",
		"provider", res.Provider,
		"channel", string(res.Channel),
		"to", masked,
		"provider_msg_id", res.ProviderMessageID,
		"duration_ms", duration.Milliseconds(),
	)

	// 6. Record idempotency
	if msg.IdempotencyKey != "" {
		s.idemStore.Set(msg.IdempotencyKey, res, s.idemTTL)
	}

	return res, nil
}
