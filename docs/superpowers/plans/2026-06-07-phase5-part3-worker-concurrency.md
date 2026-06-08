# Phase 5 Plan — Part 3: Expiration Worker & Concurrency Tests (Tasks 8-9)

> Part of the Phase 5 implementation plan. Index: [2026-06-07-phase5-orders-inventory-checkout.md](2026-06-07-phase5-orders-inventory-checkout.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **Depends on:** Parts 1-2 (inventory + orders modules, sqlc queries incl. `ListExpiredPendingOrders`). **EXTEND, DON'T REWRITE.**

---

## Task 8: Expiration worker

The worker has three pieces:
1. `platform/worker` — a generic ticker runner (reusable, testable).
2. `orders` expiration job — the idempotent logic that expires stale orders + releases reservations.
3. `cmd/worker/main.go` — the binary that wires config + DB + ticker + job.

**Files:**
- Create: `services/api/internal/platform/worker/worker.go`
- Test: `services/api/internal/platform/worker/worker_test.go`
- Create: `services/api/internal/modules/orders/expiration.go`
- Test: `services/api/internal/modules/orders/expiration_test.go`
- Create: `services/api/cmd/worker/main.go`

- [ ] **Step 1: Write failing test for the ticker runner**

Create `services/api/internal/platform/worker/worker_test.go`:
```go
package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunner_TicksUntilContextCancelled(t *testing.T) {
	var calls int64
	job := func(ctx context.Context) error {
		atomic.AddInt64(&calls, 1)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := New("test", 10*time.Millisecond, job, nil)

	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()

	time.Sleep(55 * time.Millisecond)
	cancel()
	<-done

	if got := atomic.LoadInt64(&calls); got < 3 {
		t.Errorf("expected at least 3 ticks, got %d", got)
	}
}

func TestRunner_RunsImmediatelyThenTicks(t *testing.T) {
	var calls int64
	job := func(ctx context.Context) error {
		atomic.AddInt64(&calls, 1)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := New("test", time.Hour, job, nil) // long interval; only the immediate run should fire

	go r.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()

	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Errorf("expected exactly 1 immediate run, got %d", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/worker/ -v; cd ../..
```
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Implement the ticker runner**

Create `services/api/internal/platform/worker/worker.go`:
```go
package worker

import (
	"context"
	"log/slog"
	"time"
)

// Job is a unit of work run on each tick.
type Job func(ctx context.Context) error

// Runner invokes a Job immediately, then on every interval, until the context
// is cancelled. Job errors are logged, never fatal.
type Runner struct {
	name     string
	interval time.Duration
	job      Job
	log      *slog.Logger
}

func New(name string, interval time.Duration, job Job, log *slog.Logger) *Runner {
	return &Runner{name: name, interval: interval, job: job, log: log}
}

func (r *Runner) Run(ctx context.Context) {
	r.runOnce(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if r.log != nil {
				r.log.Info("worker stopped", "job", r.name)
			}
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Runner) runOnce(ctx context.Context) {
	if err := r.job(ctx); err != nil && r.log != nil {
		r.log.Error("worker job failed", "job", r.name, "error", err)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/platform/worker/ -v; cd ../..
```
Expected: PASS (both ticker tests).

- [ ] **Step 5: Write failing test for the expiration job**

Create `services/api/internal/modules/orders/expiration_test.go`:
```go
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
	ev, cat := repo.seed(orgID, 100, 5)
	userID := uuid.New()

	// Create a PENDING_PAYMENT order with a past expiry + ACTIVE reservation.
	order := db.Order{
		ID: uuid.New(), OrganizationID: orgID, EventID: ev.ID, CategoryID: cat.ID,
		ParticipantID: userID, OrderNumber: "ORD-20260607-AAAAAA", Status: StatusPendingPayment,
		ExpiredAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
	}
	repo.orders[order.ID] = order
	repo.reservations[order.ID] = db.InventoryReservation{
		ID: uuid.New(), OrderID: order.ID, CategoryID: cat.ID, Status: ReservationActive,
	}

	svc := NewService(repo, nil, 15*time.Minute)

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
	if repo.reservations[order.ID].Status != ReservationExpired {
		t.Errorf("reservation = %q, want EXPIRED", repo.reservations[order.ID].Status)
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
```
Note: this requires `fakeRepo` (from `service_test.go`) to implement `ListExpiredPendingOrders`. Add that method to `fakeRepo` in `service_test.go`:
```go
func (f *fakeRepo) ListExpiredPendingOrders(_ context.Context, limit int32) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	now := time.Now()
	for _, o := range f.orders {
		if o.Status == StatusPendingPayment && o.ExpiredAt.Valid && o.ExpiredAt.Time.Before(now) {
			ids = append(ids, o.ID)
		}
	}
	return ids, nil
}
```
And add `ListExpiredPendingOrders(ctx, limit int32) ([]uuid.UUID, error)` to the `Repository` interface + `sqlcRepo` (Task 5's repository.go) — see Step 7.

- [ ] **Step 6: Implement the expiration job**

Create `services/api/internal/modules/orders/expiration.go`:
```go
package orders

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
)

// ExpireOrders finds PENDING_PAYMENT orders past their expiry and transitions them
// to EXPIRED, releasing their reservations. Idempotent and safe to run concurrently
// (the UpdateOrderStatus guard ensures only PENDING_PAYMENT rows transition).
// Returns the number of orders expired.
func (s *Service) ExpireOrders(ctx context.Context, batch int32) (int, error) {
	var expired int
	err := s.repo.ExecTx(ctx, func(tx Repository) error {
		ids, err := tx.ListExpiredPendingOrders(ctx, batch)
		if err != nil {
			return err
		}
		for _, id := range ids {
			updated, err := tx.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
				ID: id, Status: StatusExpired, Status_2: StatusPendingPayment,
			})
			if err != nil {
				// row no longer PENDING_PAYMENT (raced) → skip, stay idempotent
				continue
			}
			if err := inv.Release(ctx, tx.Inventory(), id, ReservationExpired); err != nil {
				return err
			}
			expired++
			s.record(ctx, updated, "ORDER_EXPIRED")
			s.recordReservation(ctx, updated, "RESERVATION_EXPIRED")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return expired, nil
}

// ExpireJob adapts ExpireOrders to a worker.Job. batch caps rows per tick.
func (s *Service) ExpireJob(batch int32) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := s.ExpireOrders(ctx, batch)
		return err
	}
}

var _ = uuid.Nil
var _ = audit.Entry{}
```
Note: remove the trailing `var _ =` lines if the build flags them. `UpdateOrderStatus` with the `Status_2` guard returning `pgx.ErrNoRows` when the row isn't PENDING_PAYMENT is what makes this idempotent — the `continue` handles it. Confirm the guard param name (`Status_2`) matches generated code (same as Task 6).

- [ ] **Step 7: Add ListExpiredPendingOrders to the orders repository**

In `services/api/internal/modules/orders/repository.go`, add to the `Repository` interface:
```go
	ListExpiredPendingOrders(ctx context.Context, limit int32) ([]uuid.UUID, error)
```
And the `sqlcRepo` method:
```go
func (r *sqlcRepo) ListExpiredPendingOrders(ctx context.Context, limit int32) ([]uuid.UUID, error) {
	return r.q.ListExpiredPendingOrders(ctx, limit)
}
```
Confirm the generated `ListExpiredPendingOrders` takes `int32` (the `LIMIT $1`) and returns `[]uuid.UUID`.

- [ ] **Step 8: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/modules/orders/ ./internal/platform/worker/ -v; cd ../..
```
Expected: PASS (expiration idempotent test + worker tests + earlier orders tests).

- [ ] **Step 9: Implement the worker binary**

Create `services/api/cmd/worker/main.go`:
```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/varin/ivyticketing/services/api/internal/app"
	"github.com/varin/ivyticketing/services/api/internal/db"
	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	"github.com/varin/ivyticketing/services/api/internal/platform/database"
	"github.com/varin/ivyticketing/services/api/internal/platform/logger"
	"github.com/varin/ivyticketing/services/api/internal/platform/worker"
)

func main() {
	cfg, err := app.LoadConfig()
	log := logger.New(cfg.AppEnv)
	if err != nil {
		log.Error("config load failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pg, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	auditLog := audit.NewLogger(db.New(pg.Pool), log)
	svc := ordersmod.NewService(ordersmod.NewRepository(pg.Pool), auditLog, cfg.OrderExpiration)

	runner := worker.New("expire_orders", cfg.WorkerInterval, svc.ExpireJob(100), log)
	log.Info("worker starting", "job", "expire_orders", "interval", cfg.WorkerInterval.String())
	runner.Run(ctx)
	log.Info("worker exited")
}
```
Note: confirm `database.Connect` returns a struct with `.Pool *pgxpool.Pool` and `.Close()` (Phase 1 pattern — same as `cmd/api/main.go`). Read `services/api/internal/platform/database/postgres.go` and `cmd/api/main.go` to match exactly. `logger.New(appEnv)` is the Phase 1 logger constructor — confirm signature.

- [ ] **Step 10: Build the worker + add Makefile target**

Run:
```bash
cd services/api && go build ./cmd/worker && cd ../..
```
Expected: builds with no errors.

Add to `Makefile` (additive):
```make
worker:
	cd services/api && go run ./cmd/worker
```

- [ ] **Step 11: Commit**

```bash
git add services/api/internal/platform/worker services/api/internal/modules/orders/expiration.go \
  services/api/internal/modules/orders/expiration_test.go services/api/internal/modules/orders/repository.go \
  services/api/internal/modules/orders/service_test.go services/api/cmd/worker/main.go Makefile
git commit -m "feat(worker): add ticker runner, idempotent expire_orders job, and worker binary"
```

---

## Task 9: Concurrency tests (real Postgres)

These run against `ivyticketing_test` and bombard the real checkout path with goroutines
to prove no oversell. They reuse the Phase 2/3/4 integration harness.

**Files:**
- Create: `services/api/tests/integration/inventory_concurrency_test.go`
- Create: `services/api/tests/integration/order_creation_test.go`
- Create: `services/api/tests/integration/expiration_worker_test.go`

- [ ] **Step 1: Concurrency helper — direct service against the pool**

These tests call the orders `Service` directly (not via HTTP) to bombard checkout with
many goroutines sharing one pool. Create a helper at the top of
`inventory_concurrency_test.go`. First read `helpers_test.go` to reuse `testPool`,
`truncate`, and any org/event/category seeding helpers. If no seeding helper exists for
"published event + category with capacity N", add one to `helpers_test.go`:
```go
// seedPublishedCategory inserts an org, a published event, and a category with the
// given capacity directly via SQL, returning their IDs. For concurrency tests.
func seedPublishedCategory(t *testing.T, pool *pgxpool.Pool, capacity, maxOrder int32) (orgID, eventID, categoryID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	orgID = uuid.New()
	eventID = uuid.New()
	categoryID = uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1,$2,$3)`,
		orgID, "Conc Org "+orgID.String()[:8], "conc-"+orgID.String()[:8])
	if err != nil { t.Fatalf("seed org: %v", err) }
	_, err = pool.Exec(ctx, `INSERT INTO events (id, organization_id, name, slug, event_type, status)
		VALUES ($1,$2,'E','e-`+eventID.String()[:8]+`','marathon','published')`, eventID, orgID)
	if err != nil { t.Fatalf("seed event: %v", err) }
	_, err = pool.Exec(ctx, `INSERT INTO event_categories
		(id, organization_id, event_id, name, price, capacity, registration_opens_at, registration_closes_at, max_order_per_user)
		VALUES ($1,$2,$3,'42K',100000,$4, now()-interval '1 hour', now()+interval '1 hour', $5)`,
		categoryID, orgID, eventID, capacity, maxOrder)
	if err != nil { t.Fatalf("seed category: %v", err) }
	return orgID, eventID, categoryID
}
```
Note: `helpers_test.go` should already import `context`, `uuid`, `pgxpool`, `testing`. Add any missing imports.

- [ ] **Step 2: Inventory concurrency test (no oversell)**

Create `services/api/tests/integration/inventory_concurrency_test.go`:
```go
//go:build integration

package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

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

	svc := ordersmod.NewService(ordersmod.NewRepository(pool), nil, 15*time.Minute)

	var success, soldOut, other int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// each request a distinct participant (max_order_per_user=1)
			_, err := svc.Checkout(context.Background(), uuid.New(), eventID, categoryID)
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
```

- [ ] **Step 3: Order-number uniqueness under concurrency**

Create `services/api/tests/integration/order_creation_test.go`:
```go
//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
)

func TestOrderCreation_UniqueOrderNumbers(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	_, eventID, categoryID := seedPublishedCategory(t, pool, 500, 1)

	svc := ordersmod.NewService(ordersmod.NewRepository(pool), nil, 15*time.Minute)

	const n = 300
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			svc.Checkout(context.Background(), uuid.New(), eventID, categoryID)
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
```

- [ ] **Step 4: Expiration worker integration test**

Create `services/api/tests/integration/expiration_worker_test.go`:
```go
//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
)

func TestExpirationWorker_ReleasesAndIsIdempotent(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	_, eventID, categoryID := seedPublishedCategory(t, pool, 100, 5)

	// TTL = 0 so the order is immediately expired.
	svc := ordersmod.NewService(ordersmod.NewRepository(pool), nil, 0)
	order, err := svc.Checkout(context.Background(), uuid.New(), eventID, categoryID)
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
```

- [ ] **Step 5: Run concurrency tests (with race detector)**

Run:
```bash
make test-db-setup
cd services/api && TEST_DATABASE_URL="postgres://localhost:5432/ivyticketing_test?sslmode=disable" go test -tags=integration -race ./tests/integration/... -run 'Concurrency|OrderCreation|ExpirationWorker' -v; cd ../..
```
Expected: all PASS, no race warnings, no oversell. Note: `-race` slows execution; 200 goroutines is fine.

- [ ] **Step 6: Commit**

```bash
git add services/api/tests/integration
git commit -m "test(orders): add concurrency tests proving no oversell and unique order numbers"
```

---
