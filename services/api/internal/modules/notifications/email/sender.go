package email

import "context"

// Sender is the interface all email backends must implement.
type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}
