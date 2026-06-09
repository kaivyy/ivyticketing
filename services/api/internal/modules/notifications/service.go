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
	repo   Repository
	sender email.Sender
	log    *slog.Logger
}

// NewService creates a Service. sender must not be nil (use LogSender as default).
func NewService(repo Repository, sender email.Sender, log *slog.Logger) *Service {
	return &Service{repo: repo, sender: sender, log: log}
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
	now := time.Now()

	result, err := templates.Render(typ, data)
	if err != nil {
		s.log.Warn("notification template render failed", "id", id, "type", typ, "err", err)
		_ = s.repo.UpdateStatus(ctx, id, statusFailed, 1, &now, nil)
		return
	}

	sendErr := s.sender.Send(ctx, data.ParticipantEmail, result.Subject, result.HTMLBody, result.TextBody)
	attempts := int32(1)
	if sendErr != nil {
		s.log.Warn("notification send failed", "id", id, "type", typ, "err", sendErr)
		_ = s.repo.UpdateStatus(ctx, id, statusFailed, attempts, &now, nil)
		return
	}

	_ = s.repo.UpdateStatus(ctx, id, statusSent, attempts, &now, &now)
}
