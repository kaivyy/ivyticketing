package notifications

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/notifications/email"
	"github.com/varin/ivyticketing/services/api/internal/modules/notifications/templates"
)

const (
	statusPending = "pending"
	statusSent    = "sent"
	statusFailed  = "failed"
	channelEmail  = "email"
)

// Notifier is the interface other modules depend on. Each consuming package
// should declare a matching local interface to avoid import-cycle risk.
type Notifier interface {
	Enqueue(ctx context.Context, participantID uuid.UUID, typ string, data TemplateData) error
}

// Service persists a notification record then dispatches it asynchronously.
type Service struct {
	repo     Repository
	sender   email.Sender
	lookup   ParticipantLookup
	resolver templates.Resolver
	log      *slog.Logger
}

// NewService creates a Service. sender must not be nil (use LogSender as default).
// lookup and resolver may be nil — when nil, sendAsync skips lookup and uses inline templates.
func NewService(repo Repository, sender email.Sender, lookup ParticipantLookup, resolver templates.Resolver, log *slog.Logger) *Service {
	return &Service{repo: repo, sender: sender, lookup: lookup, resolver: resolver, log: log}
}

// Enqueue persists the notification as pending and fires sendAsync in a goroutine.
func (s *Service) Enqueue(ctx context.Context, participantID uuid.UUID, typ string, data TemplateData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	n, err := s.repo.Create(ctx, participantID, typ, channelEmail, statusPending, payload)
	if err != nil {
		return err
	}

	go s.sendAsync(n.ID, typ, data)
	return nil
}

// sendAsync renders the template, calls the sender, and updates the DB record.
// Runs in a detached goroutine — errors are logged, not propagated.
func (s *Service) sendAsync(id uuid.UUID, typ string, data TemplateData) {
	ctx := context.Background()

	// Resolve participant info via lookup if email is empty and lookup is available.
	if data.ParticipantEmail == "" && s.lookup != nil {
		n, err := s.repo.GetByID(ctx, id)
		if err != nil {
			// DB error — notification remains retryable
			s.log.Warn("notification lookup: get record failed", "id", id, "err", err)
			return
		}
		email, name, err := s.lookup.GetByID(ctx, n.ParticipantID)
		if err != nil {
			// DB error — notification remains retryable
			s.log.Warn("notification lookup: participant failed", "id", id, "participant_id", n.ParticipantID, "err", err)
			return
		}
		data.ParticipantEmail = email
		data.ParticipantName = name
	}

	// Terminal failure: no email address.
	if data.ParticipantEmail == "" {
		now := time.Now()
		lastErr := "no_email_address"
		if err := s.repo.UpdateRetry(ctx, id, "failed", 1, &lastErr, nil, &now); err != nil {
			s.log.Warn("notification UpdateRetry failed (no email)", "id", id, "err", err)
		}
		s.log.Warn("notification terminal: no email address", "id", id)
		return
	}

	// Resolve template: DB override if resolver available, else inline fallback.
	var result templates.RenderResult
	var renderErr error
	if s.resolver != nil {
		result, renderErr = s.resolver.Render(typ, data)
	} else {
		result, renderErr = templates.Render(typ, data)
	}

	if renderErr != nil {
		s.log.Warn("notification template render failed", "id", id, "type", typ, "err", renderErr)
		now := time.Now()
		lastErr := renderErr.Error()
		if err := s.repo.UpdateRetry(ctx, id, "failed", 1, &lastErr, nil, &now); err != nil {
			s.log.Warn("notification UpdateRetry failed (render)", "id", id, "err", err)
		}
		return
	}

	// Attempt send.
	senderErr := s.sender.Send(ctx, data.ParticipantEmail, result.Subject, result.HTMLBody, result.TextBody)
	now := time.Now()

	if senderErr != nil {
		// Send failed — check if we should retry or go terminal.
		// We need to read current attempts first.
		n, err := s.repo.GetByID(ctx, id)
		attempts := int32(1)
		if err == nil {
			attempts = n.Attempts + 1
		}

		lastErr := senderErr.Error()
		if attempts >= MaxRetryAttempts {
			// Terminal — no more retries.
			if err := s.repo.UpdateRetry(ctx, id, "failed", attempts, &lastErr, nil, &now); err != nil {
				s.log.Warn("notification UpdateRetry failed (terminal)", "id", id, "attempts", attempts, "err", err)
			}
			s.log.Warn("notification terminal: max attempts reached", "id", id, "attempts", attempts)
			return
		}

		// Retryable — calculate backoff.
		backoff := BackoffForAttempt(attempts)
		retryAt := time.Now().Add(backoff)
		if err := s.repo.UpdateRetry(ctx, id, "failed", attempts, &lastErr, &retryAt, &now); err != nil {
			s.log.Warn("notification UpdateRetry failed (retryable)", "id", id, "attempts", attempts, "err", err)
		}
		s.log.Warn("notification send failed — will retry", "id", id, "attempts", attempts, "backoff", backoff)
		return
	}

	// Success.
	attempts := int32(1)
	if n, err := s.repo.GetByID(ctx, id); err == nil {
		attempts = n.Attempts + 1
	}
	if err := s.repo.UpdateRetry(ctx, id, "sent", attempts, nil, nil, &now); err != nil {
		s.log.Warn("notification UpdateRetry failed (success)", "id", id, "attempts", attempts, "err", err)
	}
}

// BackoffForAttempt returns the exponential backoff duration for the given
// 1-based attempt number. The schedule is defined in RetryBackoffs; attempts
// beyond the slice length reuse the last value.
// Sequence: 30s, 60s, 120s, 240s, 480s.
func BackoffForAttempt(attempt int32) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	idx := int(attempt) - 1
	if idx >= len(RetryBackoffs) {
		idx = len(RetryBackoffs) - 1
	}
	return RetryBackoffs[idx]
}
