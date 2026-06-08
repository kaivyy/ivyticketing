package orders

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// expireFakeRepo extends the checkout fakeRepo behaviors needed by the expiration job.
// We reuse fakeRepo from service_test.go (same package) and add expired-order seeding.

func TestExpireOrders_Idempotent(t *testing.T) {
	repo := newFakeRepo()
	orgID := uuid.New()
	eventID, catID := repo.seed(orgID, 100, 5)
	userID := uuid.New()

	// Create a PENDING_PAYMENT order with a past expiry + ACTIVE reservation.
	order := db.Order{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EventID:        eventID,
		CategoryID:     catID,
		ParticipantID:  userID,
		OrderNumber:    "ORD-20260607-AAAAAA",
		Status:         StatusPendingPayment,
		ExpiredAt:      pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
	}
	repo.orders[order.ID] = order
	resID := uuid.New()
	repo.reserves[resID] = db.InventoryReservation{
		ID:         resID,
		OrderID:    order.ID,
		CategoryID: catID,
		Status:     ReservationActive,
	}

	svc := NewService(repo, nil, 15*time.Minute, nil)

	// First run expires it.
	n, err := svc.ExpireOrders(context.Background(), 100)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if n != 1 {
		t.Fatalf("expired count = %d, want 1", n)
	}
	if repo.orders[order.ID].Status != StatusExpired {
		t.Errorf("order status = %q, want EXPIRED", repo.orders[order.ID].Status)
	}
	if repo.reserves[resID].Status != ReservationExpired {
		t.Errorf("reservation = %q, want EXPIRED", repo.reserves[resID].Status)
	}

	// Second run is a no-op (idempotent).
	n2, err := svc.ExpireOrders(context.Background(), 100)
	if err != nil {
		t.Fatalf("second expire: %v", err)
	}
	if n2 != 0 {
		t.Errorf("second run expired %d, want 0", n2)
	}
}
