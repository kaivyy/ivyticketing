package notifications_test

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	notifmod "github.com/varin/ivyticketing/services/api/internal/modules/notifications"
	"github.com/varin/ivyticketing/services/api/internal/modules/notifications/templates"
)

// --- fakes ---

type fakeSender struct {
	mu    sync.Mutex
	calls []sendCall
}

type sendCall struct {
	to      string
	subject string
}

func (f *fakeSender) Send(_ context.Context, to, subject, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, sendCall{to: to, subject: subject})
	return nil
}

func (f *fakeSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type fakeRepo struct {
	mu      sync.Mutex
	updated []string
}

func (r *fakeRepo) Create(_ context.Context, participantID uuid.UUID, typ, channel, status string, _ []byte) (db.Notification, error) {
	return db.Notification{
		ID:            uuid.New(),
		ParticipantID: participantID,
		Type:          typ,
		Channel:       channel,
		Status:        status,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil
}

func (r *fakeRepo) UpdateStatus(_ context.Context, _ uuid.UUID, status string, _ int32, _, _ *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updated = append(r.updated, status)
	return nil
}

func (r *fakeRepo) ListPending(_ context.Context, _ int32) ([]db.Notification, error) {
	return nil, nil
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (db.Notification, error) {
	return db.Notification{ID: id}, nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

// --- tests ---

func TestEnqueue_LogSender(t *testing.T) {
	sender := &fakeSender{}
	repo := &fakeRepo{}
	svc := notifmod.NewService(repo, sender, newTestLogger())

	ctx := context.Background()
	participantID := uuid.New()
	data := notifmod.TemplateData{
		ParticipantEmail: "test@example.com",
		OrderNumber:      "ORD-001",
		TotalAmount:      "Rp 100000",
	}

	if err := svc.Enqueue(ctx, participantID, notifmod.NotifOrderCreated, data); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	// sendAsync runs in a goroutine — wait up to 2s for it to complete
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sender.count() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if sender.count() == 0 {
		t.Fatal("expected sender.Send to be called at least once")
	}
}

func TestTemplates_AllTypes(t *testing.T) {
	types := []string{
		templates.NotifOrderCreated,
		templates.NotifPaymentPaid,
		templates.NotifPaymentExpired,
		templates.NotifQueueAllowed,
		templates.NotifBallotWinner,
		templates.NotifBallotNotSelected,
		templates.NotifWaitlistPromoted,
	}
	data := templates.TemplateData{
		ParticipantName:  "Budi",
		ParticipantEmail: "budi@example.com",
		EventName:        "Jakarta Marathon 2026",
		OrderNumber:      "ORD-999",
		TotalAmount:      "Rp 250000",
		CheckoutURL:      "https://ivyticketing.com/checkout/abc",
		BallotDrawID:     uuid.New().String(),
	}

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			result, err := templates.Render(typ, data)
			if err != nil {
				t.Fatalf("Render(%q) returned error: %v", typ, err)
			}
			if result.Subject == "" {
				t.Errorf("Render(%q): subject is empty", typ)
			}
			if result.HTMLBody == "" {
				t.Errorf("Render(%q): HTMLBody is empty", typ)
			}
		})
	}
}

func TestTemplates_UnknownType(t *testing.T) {
	_, err := templates.Render("unknown.type", templates.TemplateData{})
	if err == nil {
		t.Fatal("expected error for unknown notification type, got nil")
	}
}
