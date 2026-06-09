package email

import (
	"context"
	"log/slog"
)

// LogSender logs emails instead of sending them. Used as the default sender
// until a real email provider is configured.
type LogSender struct{ Log *slog.Logger }

func (s *LogSender) Send(_ context.Context, to, subject, _, _ string) error {
	s.Log.Info("notification email", "to", to, "subject", subject)
	return nil
}
