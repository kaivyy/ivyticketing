//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
)

func TestOrderCreation_UniqueOrderNumbers(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	_, eventID, categoryID := seedPublishedCategory(t, pool, 500, 1)
	participants := seedUsers(t, pool, 300)

	svc := ordersmod.NewService(ordersmod.NewRepository(pool), nil, 15*time.Minute, nil, nil)

	const n = 300
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			svc.Checkout(context.Background(), participants[i], eventID, categoryID, "")
		}()
	}
	close(start)
	wg.Wait()

	// All order_numbers must be unique (UNIQUE constraint + generator).
	var total, distinct int
	pool.QueryRow(context.Background(), `SELECT count(*), count(DISTINCT order_number) FROM orders WHERE category_id=$1`, categoryID).Scan(&total, &distinct)
	if total != distinct {
		t.Fatalf("duplicate order numbers: total=%d distinct=%d", total, distinct)
	}
	if total == 0 {
		t.Fatal("expected orders created")
	}
}
