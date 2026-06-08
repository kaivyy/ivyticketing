//go:build integration

package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
)

func TestInventoryConcurrency_NoOversell(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	const capacity = 100
	const requests = 200
	orgID, eventID, categoryID := seedPublishedCategory(t, pool, capacity, 1)
	_ = orgID
	participants := seedUsers(t, pool, requests)

	svc := ordersmod.NewService(ordersmod.NewRepository(pool), nil, 15*time.Minute, nil)

	var success, soldOut, other int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < requests; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.Checkout(context.Background(), participants[i], eventID, categoryID, "")
			switch {
			case err == nil:
				atomic.AddInt64(&success, 1)
			case err == inv.ErrSoldOut:
				atomic.AddInt64(&soldOut, 1)
			default:
				atomic.AddInt64(&other, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if other != 0 {
		t.Fatalf("unexpected errors: %d", other)
	}
	if success != capacity {
		t.Errorf("successful checkouts = %d, want %d", success, capacity)
	}
	if soldOut != requests-capacity {
		t.Errorf("sold-out = %d, want %d", soldOut, requests-capacity)
	}

	// Verify DB: active reservations never exceed capacity.
	var activeReservations int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM inventory_reservations WHERE category_id=$1 AND status='ACTIVE'`,
		categoryID).Scan(&activeReservations)
	if activeReservations > capacity {
		t.Fatalf("OVERSELL: %d active reservations > capacity %d", activeReservations, capacity)
	}
	if activeReservations != capacity {
		t.Errorf("active reservations = %d, want %d", activeReservations, capacity)
	}
}
