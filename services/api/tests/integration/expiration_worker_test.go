//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
)

func TestExpirationWorker_ReleasesAndIsIdempotent(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	_, eventID, categoryID := seedPublishedCategory(t, pool, 100, 5)
	users := seedUsers(t, pool, 1)

	// TTL = 0 so the order is immediately expired.
	svc := ordersmod.NewService(ordersmod.NewRepository(pool), nil, 0)
	order, err := svc.Checkout(context.Background(), users[0], eventID, categoryID)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// allow expired_at (= now + 0) to be strictly in the past
	time.Sleep(10 * time.Millisecond)

	n, err := svc.ExpireOrders(context.Background(), 100)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if n != 1 {
		t.Fatalf("expired = %d, want 1", n)
	}

	// order EXPIRED, reservation EXPIRED, slot back.
	var orderStatus, resStatus string
	pool.QueryRow(context.Background(), `SELECT status FROM orders WHERE id=$1`, order.ID).Scan(&orderStatus)
	pool.QueryRow(context.Background(), `SELECT status FROM inventory_reservations WHERE order_id=$1`, order.ID).Scan(&resStatus)
	if orderStatus != "EXPIRED" || resStatus != "EXPIRED" {
		t.Fatalf("order=%s reservation=%s, want both EXPIRED", orderStatus, resStatus)
	}

	var active int
	pool.QueryRow(context.Background(), `SELECT count(*) FROM inventory_reservations WHERE category_id=$1 AND status='ACTIVE'`, categoryID).Scan(&active)
	if active != 0 {
		t.Errorf("active reservations = %d, want 0 after expiry", active)
	}

	// Idempotent: second run expires nothing.
	n2, _ := svc.ExpireOrders(context.Background(), 100)
	if n2 != 0 {
		t.Errorf("second expire = %d, want 0", n2)
	}
}
