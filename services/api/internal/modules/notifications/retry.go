package notifications

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/notifications/email"
	"github.com/varin/ivyticketing/services/api/internal/modules/notifications/templates"
)

const maxRetryAttempts = 5

// RetryService handles retrying failed/pending notifications.
type RetryService struct {
	repo     Repository
	sender   email.Sender
	lookup   ParticipantLookup
	resolver templates.Resolver
	log      *slog.Logger
}

// NewRetryService creates a RetryService.
func NewRetryService(repo Repository, sender email.Sender, lookup ParticipantLookup, resolver templates.Resolver, log *slog.Logger) *RetryService {
	return &RetryService{repo: repo, sender: sender, lookup: lookup, resolver: resolver, log: log}
}

// RetryPending picks up to `batch` retryable notifications and attempts to send them.
func (s *RetryService) RetryPending(ctx context.Context, batch int32) (int, error) {
	notifs, err := s.repo.ListRetryable(ctx, maxRetryAttempts, batch)
	if err != nil {
		return 0, err
	}
	if len(notifs) == 0 {
		return 0, nil
	}

	var retried int
	for _, n := range notifs {
		if err := s.retryOne(ctx, n); err != nil {
			s.log.Warn("notification retry job failed", "id", n.ID, "err", err)
			continue
		}
		retried++
	}
	return retried, nil
}

// retryOne attempts to send a single notification and updates its state.
func (s *RetryService) retryOne(ctx context.Context, n db.Notification) error {
	var data templates.TemplateData
	if len(n.Payload) > 0 {
		if err := json.Unmarshal(n.Payload, &data); err != nil {
			lastErr := "invalid_payload_json"
			now := time.Now()
			_ = s.repo.UpdateRetry(ctx, n.ID, "failed", n.Attempts+1, &lastErr, nil, &now)
			return err
		}
	}

	// Resolve participant info if email is empty.
	if data.ParticipantEmail == "" && s.lookup != nil {
		emailAddr, name, err := s.lookup.GetByID(ctx, n.ParticipantID)
		if err != nil {
			// DB error — leave as retryable
			return err
		}
		data.ParticipantEmail = emailAddr
		data.ParticipantName = name
	}

	if data.ParticipantEmail == "" {
		lastErr := "no_email_address"
		now := time.Now()
		_ = s.repo.UpdateRetry(ctx, n.ID, "failed", n.Attempts+1, &lastErr, nil, &now)
		return nil
	}

	// Resolve template.
	var result templates.RenderResult
	var renderErr error
	if s.resolver != nil {
		result, renderErr = s.resolver.Render(n.Type, data)
	} else {
		result, renderErr = templates.Render(n.Type, data)
	}
	if renderErr != nil {
		lastErr := renderErr.Error()
		now := time.Now()
		_ = s.repo.UpdateRetry(ctx, n.ID, "failed", n.Attempts+1, &lastErr, nil, &now)
		return renderErr
	}

	sendErr := s.sender.Send(ctx, data.ParticipantEmail, result.Subject, result.HTMLBody, result.TextBody)
	now := time.Now()

	if sendErr != nil {
		attempts := n.Attempts + 1
		lastErr := sendErr.Error()
		if attempts >= maxRetryAttempts {
			_ = s.repo.UpdateRetry(ctx, n.ID, "failed", attempts, &lastErr, nil, &now)
			s.log.Warn("notification retry terminal", "id", n.ID, "attempts", attempts)
			return nil
		}
		backoff := backoffForAttempt(attempts)
		retryAt := time.Now().Add(backoff)
		_ = s.repo.UpdateRetry(ctx, n.ID, "failed", attempts, &lastErr, &retryAt, &now)
		s.log.Warn("notification retry failed", "id", n.ID, "attempts", attempts, "backoff", backoff)
		return sendErr
	}

	attempts := n.Attempts + 1
	_ = s.repo.UpdateRetry(ctx, n.ID, "sent", attempts, nil, nil, &now)
	return nil
}

// RetryWorkerJob adapts RetryPending to a worker.Job.
func (s *RetryService) RetryWorkerJob(batch int32) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := s.RetryPending(ctx, batch)
		return err
	}
}
