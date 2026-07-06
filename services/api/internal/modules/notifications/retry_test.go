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

// fakeSenderForRetry implements email.Sender
type fakeSenderForRetry struct {
	mu          sync.Mutex
	calls       []string
	shouldFail  bool
	failUntil   int // fail until this many calls have been made
}

func (f *fakeSenderForRetry) Send(_ context.Context, to, subject, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, to+"|"+subject)
	if f.shouldFail && len(f.calls) <= f.failUntil {
		return context.DeadlineExceeded
	}
	return nil
}

func (f *fakeSenderForRetry) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// fakeRepoForRetry implements notifications.Repository with retry support
type fakeRepoForRetry struct {
	mu            sync.Mutex
	notifications []db.Notification
	updates       []updateRecord
}

type updateRecord struct {
	id        uuid.UUID
	status    string
	attempts  int32
	lastError *string
	retryAt   *time.Time
}

func (r *fakeRepoForRetry) Create(_ context.Context, participantID uuid.UUID, typ, channel, status string, payload []byte) (db.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := db.Notification{
		ID:            uuid.New(),
		ParticipantID: participantID,
		Type:          typ,
		Channel:       channel,
		Status:        status,
		Payload:       payload,
		Attempts:      0,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	r.notifications = append(r.notifications, n)
	return n, nil
}

func (r *fakeRepoForRetry) UpdateStatus(_ context.Context, id uuid.UUID, status string, attempts int32, _, _ *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, updateRecord{id: id, status: status, attempts: attempts})
	return nil
}

func (r *fakeRepoForRetry) UpdateRetry(_ context.Context, id uuid.UUID, status string, attempts int32, lastError *string, nextRetryAt, _ *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, updateRecord{
		id:        id,
		status:    status,
		attempts:  attempts,
		lastError: lastError,
		retryAt:   nextRetryAt,
	})
	return nil
}

func (r *fakeRepoForRetry) ListPending(_ context.Context, _ int32) ([]db.Notification, error) {
	return nil, nil
}

func (r *fakeRepoForRetry) ListRetryable(_ context.Context, maxAttempts, limit int32) ([]db.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []db.Notification
	for _, n := range r.notifications {
		if (n.Status == "pending" || n.Status == "failed") && n.Attempts < maxAttempts && len(result) < int(limit) {
			result = append(result, n)
		}
	}
	return result, nil
}

func (r *fakeRepoForRetry) GetByID(_ context.Context, id uuid.UUID) (db.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range r.notifications {
		if n.ID == id {
			return n, nil
		}
	}
	return db.Notification{}, nil
}

func (r *fakeRepoForRetry) GetDefaultTemplate(_ context.Context, typ, channel string) (templates.DBTemplate, error) {
	return templates.DBTemplate{}, nil
}

func newTestLoggerRetry() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func TestSendAsync_NoEmail_TerminalFailed(t *testing.T) {
	repo := &fakeRepoForRetry{}
	sender := &fakeSenderForRetry{}
	svc := notifmod.NewService(repo, sender, nil, nil, newTestLoggerRetry())

	ctx := context.Background()
	participantID := uuid.New()
	data := notifmod.TemplateData{} // No email

	if err := svc.Enqueue(ctx, participantID, notifmod.NotifOrderCreated, data); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Check that it was marked as failed with terminal state
	repo.mu.Lock()
	defer repo.mu.Unlock()
	found := false
	for _, u := range repo.updates {
		if u.status == "failed" && u.lastError != nil && *u.lastError == "no_email_address" {
			found = true
			if u.retryAt != nil {
				t.Error("expected retryAt to be nil for terminal failure")
			}
		}
	}
	if !found {
		t.Fatal("expected notification to be marked as failed with no_email_address")
	}
}

func TestSendAsync_SendOk(t *testing.T) {
	repo := &fakeRepoForRetry{}
	sender := &fakeSenderForRetry{}
	svc := notifmod.NewService(repo, sender, nil, nil, newTestLogger())

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

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sender.count() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if sender.count() == 0 {
		t.Fatal("expected sender.Send to be called")
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	found := false
	for _, u := range repo.updates {
		if u.status == "sent" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected notification to be marked as sent")
	}
}

func TestRetryPending_PicksRetryable(t *testing.T) {
	repo := &fakeRepoForRetry{}
	sender := &fakeSenderForRetry{}
	
	// Create a failed notification that should be retried
	repo.notifications = append(repo.notifications, db.Notification{
		ID:            uuid.New(),
		ParticipantID: uuid.New(),
		Type:          notifmod.NotifOrderCreated,
		Status:        "failed",
		Attempts:      1,
		Payload:       []byte(`{"participant_email":"test@example.com","order_number":"ORD-001","total_amount":"Rp 100000"}`),
	})

	retrySvc := notifmod.NewRetryService(repo, sender, nil, nil, newTestLoggerRetry())
	ctx := context.Background()

	count, err := retrySvc.RetryPending(ctx, 10)
	if err != nil {
		t.Fatalf("RetryPending returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 retry, got %d", count)
	}
}

func TestRetryPending_MaxAttempts(t *testing.T) {
	repo := &fakeRepoForRetry{}
	sender := &fakeSenderForRetry{}

	// Create a notification that has reached max attempts
	repo.notifications = append(repo.notifications, db.Notification{
		ID:            uuid.New(),
		ParticipantID: uuid.New(),
		Type:          notifmod.NotifOrderCreated,
		Status:        "failed",
		Attempts:      5, // max attempts
		Payload:       []byte(`{"participant_email":"test@example.com","order_number":"ORD-001"}`),
	})

	retrySvc := notifmod.NewRetryService(repo, sender, nil, nil, newTestLoggerRetry())
	ctx := context.Background()

	count, err := retrySvc.RetryPending(ctx, 10)
	if err != nil {
		t.Fatalf("RetryPending returned error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 retries for max attempts notification, got %d", count)
	}
}

func TestBackoffForAttempt_Durations(t *testing.T) {
	// Pin the exact sequence for attempts 1..6. Attempts 1..5 follow the
	// exponential schedule; attempt 6 caps at the last value.
	expected := map[int32]time.Duration{
		1: 30 * time.Second,
		2: 60 * time.Second,
		3: 120 * time.Second,
		4: 240 * time.Second,
		5: 480 * time.Second,
		6: 480 * time.Second, // cap
	}
	for attempt, want := range expected {
		got := notifmod.BackoffForAttempt(attempt)
		if got != want {
			t.Errorf("BackoffForAttempt(%d) = %s, want %s", attempt, got, want)
		}
	}

	// Defensive: count must equal MaxRetryAttempts so the cap lines up.
	if got := len(notifmod.RetryBackoffs); got != notifmod.MaxRetryAttempts {
		t.Errorf("RetryBackoffs length = %d, want MaxRetryAttempts=%d", got, notifmod.MaxRetryAttempts)
	}
}
