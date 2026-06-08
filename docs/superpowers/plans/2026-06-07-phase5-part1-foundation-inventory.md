# Phase 5 Plan — Part 1: Foundation & Inventory (Tasks 1-4)

> Part of the Phase 5 implementation plan. Index: [2026-06-07-phase5-orders-inventory-checkout.md](2026-06-07-phase5-orders-inventory-checkout.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** New files + additive changes only. Do not alter Phase 1-4 behavior.

---

## Task 1: Config (order expiration + worker interval)

**Files:**
- Modify: `services/api/internal/app/config.go`
- Modify: `services/api/internal/app/config_test.go`
- Modify: `services/api/.env.example`
- Modify: `.env.example` (root)

- [ ] **Step 1: Write the failing test**

Add to `services/api/internal/app/config_test.go` (keep existing tests):
```go
func TestLoadConfig_OrderDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("ORDER_EXPIRATION", "")
	t.Setenv("WORKER_INTERVAL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OrderExpiration != 15*time.Minute {
		t.Errorf("OrderExpiration = %v, want 15m", cfg.OrderExpiration)
	}
	if cfg.WorkerInterval != time.Minute {
		t.Errorf("WorkerInterval = %v, want 1m", cfg.WorkerInterval)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig_OrderDefaults -v; cd ../..
```
Expected: FAIL — `cfg.OrderExpiration` undefined.

- [ ] **Step 3: Extend config**

In `services/api/internal/app/config.go`, add fields to `Config` (after the Storage block):
```go
	OrderExpiration time.Duration
	WorkerInterval  time.Duration
```
In `LoadConfig`, before `return cfg, nil`, add:
```go
	orderExp, err := getDuration("ORDER_EXPIRATION", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	cfg.OrderExpiration = orderExp

	workerInterval, err := getDuration("WORKER_INTERVAL", time.Minute)
	if err != nil {
		return Config{}, err
	}
	cfg.WorkerInterval = workerInterval
```
(`getDuration` already exists from Phase 2.)

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/app/ -v; cd ../..
```
Expected: PASS (all config tests).

- [ ] **Step 5: Update env templates**

Append to BOTH `.env.example` (root) and `services/api/.env.example`:
```bash
ORDER_EXPIRATION=15m
WORKER_INTERVAL=1m
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/app/config.go services/api/internal/app/config_test.go services/api/.env.example .env.example
git commit -m "feat(api): add order expiration and worker interval config"
```

---

## Task 2: Database migrations

**Files:**
- Create: `database/migrations/00012_create_orders.sql`
- Create: `database/migrations/00013_create_inventory_reservations.sql`
- Create: `database/migrations/00014_seed_order_permissions.sql`

- [ ] **Step 1: Orders migration**

Create `database/migrations/00012_create_orders.sql`:
```sql
-- +goose Up
CREATE TABLE orders (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id     uuid NOT NULL REFERENCES event_categories(id) ON DELETE RESTRICT,
    participant_id  uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    order_number    text NOT NULL UNIQUE,
    status          text NOT NULL DEFAULT 'DRAFT',
    subtotal        bigint NOT NULL,
    fee             bigint NOT NULL DEFAULT 0,
    discount        bigint NOT NULL DEFAULT 0,
    total           bigint NOT NULL,
    expired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT orders_status_check CHECK (status IN ('DRAFT','PENDING_PAYMENT','PAID','EXPIRED','CANCELLED','REFUNDED')),
    CONSTRAINT orders_amounts_check CHECK (subtotal >= 0 AND fee >= 0 AND discount >= 0 AND total >= 0)
);
CREATE INDEX idx_orders_org_event ON orders(organization_id, event_id);
CREATE INDEX idx_orders_participant ON orders(participant_id);
CREATE INDEX idx_orders_status_expired ON orders(status, expired_at);
CREATE INDEX idx_orders_category ON orders(category_id);

-- +goose Down
DROP TABLE orders;
```

- [ ] **Step 2: Reservations migration**

Create `database/migrations/00013_create_inventory_reservations.sql`:
```sql
-- +goose Up
CREATE TABLE inventory_reservations (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id     uuid NOT NULL REFERENCES event_categories(id) ON DELETE RESTRICT,
    order_id        uuid NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    participant_id  uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status          text NOT NULL DEFAULT 'ACTIVE',
    expires_at      timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT reservations_status_check CHECK (status IN ('ACTIVE','EXPIRED','COMPLETED','RELEASED'))
);
CREATE INDEX idx_reservations_category ON inventory_reservations(category_id);
CREATE INDEX idx_reservations_status ON inventory_reservations(status);
CREATE INDEX idx_reservations_category_status ON inventory_reservations(category_id, status);

-- +goose Down
DROP TABLE inventory_reservations;
```

- [ ] **Step 3: Seed order permissions migration**

Create `database/migrations/00014_seed_order_permissions.sql`:
```sql
-- +goose Up
INSERT INTO permissions (key, description) VALUES
    ('order.create', 'Create orders on behalf of participants'),
    ('order.manage', 'Manage and cancel any order in the organization')
ON CONFLICT (key) DO NOTHING;

-- grant to template Owner & Manager roles (organization_id IS NULL)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN ('order.create','order.manage')
WHERE r.organization_id IS NULL AND r.slug IN ('owner','manager')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE key IN ('order.create','order.manage'));
DELETE FROM permissions WHERE key IN ('order.create','order.manage');
```

- [ ] **Step 4: Apply and verify roundtrip**

Run:
```bash
make migrate-up
make migrate-down && make migrate-up
```
Expected: all three apply, roll back cleanly, re-apply with no errors.

- [ ] **Step 5: Commit**

```bash
git add database/migrations
git commit -m "feat(db): add orders, inventory_reservations migrations and order permissions"
```

---

## Task 3: sqlc queries + regenerate

**Files:**
- Create: `database/queries/orders.sql`
- Create: `database/queries/inventory.sql`
- Regenerate: `services/api/internal/db/*`

- [ ] **Step 1: Orders queries**

Create `database/queries/orders.sql`:
```sql
-- name: CreateOrder :one
INSERT INTO orders (organization_id, event_id, category_id, participant_id,
    order_number, status, subtotal, fee, discount, total, expired_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetOrderByID :one
SELECT * FROM orders WHERE id = $1;

-- name: GetOrderByNumber :one
SELECT * FROM orders WHERE order_number = $1;

-- name: ListOrdersByParticipant :many
SELECT * FROM orders WHERE participant_id = $1 ORDER BY created_at DESC;

-- name: ListOrdersByOrgEvent :many
SELECT * FROM orders WHERE organization_id = $1 AND event_id = $2 ORDER BY created_at DESC;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = $2, updated_at = now()
WHERE id = $1 AND status = $3
RETURNING *;

-- name: CountActiveOrdersForUserCategory :one
SELECT count(*) FROM orders
WHERE category_id = $1 AND participant_id = $2
  AND status IN ('PENDING_PAYMENT','PAID');

-- name: CountPaidByCategory :one
SELECT count(*) FROM orders WHERE category_id = $1 AND status = 'PAID';

-- name: ListExpiredPendingOrders :many
SELECT id FROM orders
WHERE status = 'PENDING_PAYMENT' AND expired_at < now()
ORDER BY expired_at
FOR UPDATE SKIP LOCKED
LIMIT $1;
```

- [ ] **Step 2: Inventory queries**

Create `database/queries/inventory.sql`:
```sql
-- name: LockCategoryForUpdate :one
SELECT * FROM event_categories WHERE id = $1 FOR UPDATE;

-- name: CountActiveReservationsByCategory :one
SELECT count(*) FROM inventory_reservations
WHERE category_id = $1 AND status = 'ACTIVE';

-- name: CreateReservation :one
INSERT INTO inventory_reservations (organization_id, event_id, category_id,
    order_id, participant_id, status, expires_at)
VALUES ($1, $2, $3, $4, $5, 'ACTIVE', $6)
RETURNING *;

-- name: GetReservationByOrder :one
SELECT * FROM inventory_reservations WHERE order_id = $1;

-- name: UpdateReservationStatusByOrder :exec
UPDATE inventory_reservations SET status = $2
WHERE order_id = $1 AND status = 'ACTIVE';

-- name: ExpireReservationsForOrder :exec
UPDATE inventory_reservations SET status = 'EXPIRED'
WHERE order_id = $1 AND status = 'ACTIVE';
```

- [ ] **Step 3: Regenerate and verify build**

Run:
```bash
make sqlc
cd services/api && go build ./...; cd ../..
```
Expected: `sqlc generate` succeeds; new `orders.sql.go`, `inventory.sql.go`, and `Order`/`InventoryReservation` structs in `models.go`. Build passes.

- [ ] **Step 4: Inspect generated types**

Run:
```bash
sed -n '/type Order struct/,/^}/p;/type InventoryReservation struct/,/^}/p' services/api/internal/db/models.go
grep -A 14 "type CreateOrderParams" services/api/internal/db/orders.sql.go
```
Note types: `Subtotal/Fee/Discount/Total int64`, `ExpiredAt pgtype.Timestamptz`, `Status string`, counts return `int64`. Parts 2-3 reference these — adjust if sqlc differs.

- [ ] **Step 5: Commit**

```bash
git add database/queries services/api/internal/db
git commit -m "feat(db): add orders and inventory sqlc queries and regenerate"
```

---

## Task 4: Inventory domain (lock, stock, reservation)

**Files:**
- Create: `services/api/internal/modules/inventory/errors.go`
- Create: `services/api/internal/modules/inventory/stock.go`
- Test: `services/api/internal/modules/inventory/stock_test.go`
- Create: `services/api/internal/modules/inventory/repository.go`
- Create: `services/api/internal/modules/inventory/service.go`

The inventory package is domain-only (no HTTP). It exposes a `Repository` interface for the
DB ops orders needs, a pure `stock` calculation, and a `Service` with `Reserve`/`Release`
methods that orders calls **inside its own transaction** (orders passes a tx-bound repo).

- [ ] **Step 1: Typed errors**

Create `services/api/internal/modules/inventory/errors.go`:
```go
package inventory

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrSoldOut  = apperr.New(http.StatusConflict, "SOLD_OUT", "no remaining capacity for this category")
	ErrCategory = apperr.New(http.StatusNotFound, "CATEGORY_NOT_FOUND", "category not found")
)
```

- [ ] **Step 2: Write failing test for stock formula**

Create `services/api/internal/modules/inventory/stock_test.go`:
```go
package inventory

import "testing"

func TestRemaining(t *testing.T) {
	cases := []struct {
		capacity, reserved, paid int64
		want                     int64
	}{
		{100, 0, 0, 100},
		{100, 30, 20, 50},
		{100, 100, 0, 0},
		{100, 60, 40, 0},
	}
	for _, c := range cases {
		if got := Remaining(c.capacity, c.reserved, c.paid); got != c.want {
			t.Errorf("Remaining(%d,%d,%d) = %d, want %d", c.capacity, c.reserved, c.paid, got, c.want)
		}
	}
}

func TestHasCapacity(t *testing.T) {
	if !HasCapacity(100, 50, 49) {
		t.Error("expected capacity available (1 left)")
	}
	if HasCapacity(100, 60, 40) {
		t.Error("expected no capacity (full)")
	}
}
```

- [ ] **Step 3: Implement stock**

Create `services/api/internal/modules/inventory/stock.go`:
```go
package inventory

// Remaining computes available slots. Never returns negative.
func Remaining(capacity, reserved, paid int64) int64 {
	r := capacity - reserved - paid
	if r < 0 {
		return 0
	}
	return r
}

// HasCapacity reports whether at least one slot is free.
func HasCapacity(capacity, reserved, paid int64) bool {
	return capacity-reserved-paid > 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/inventory/ -run 'TestRemaining|TestHasCapacity' -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Repository interface + adapter**

Create `services/api/internal/modules/inventory/repository.go`:
```go
package inventory

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the subset of DB ops inventory needs. It is satisfied both by a
// pool-backed adapter and (within a transaction) by a tx-backed *db.Queries — the
// orders module passes its tx-bound queries so reservation + order creation are atomic.
type Repository interface {
	LockCategoryForUpdate(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	CountActiveReservationsByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error)
	CountPaidByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error)
	CreateReservation(ctx context.Context, arg db.CreateReservationParams) (db.InventoryReservation, error)
	ExpireReservationsForOrder(ctx context.Context, orderID uuid.UUID) error
	UpdateReservationStatusByOrder(ctx context.Context, arg db.UpdateReservationStatusByOrderParams) error
}

// QueriesRepo adapts *db.Queries (pool or tx) to Repository.
type QueriesRepo struct {
	Q *db.Queries
}

func NewRepository(q *db.Queries) Repository { return &QueriesRepo{Q: q} }

func (r *QueriesRepo) LockCategoryForUpdate(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return r.Q.LockCategoryForUpdate(ctx, id)
}
func (r *QueriesRepo) CountActiveReservationsByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error) {
	return r.Q.CountActiveReservationsByCategory(ctx, categoryID)
}
func (r *QueriesRepo) CountPaidByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error) {
	return r.Q.CountPaidByCategory(ctx, categoryID)
}
func (r *QueriesRepo) CreateReservation(ctx context.Context, arg db.CreateReservationParams) (db.InventoryReservation, error) {
	return r.Q.CreateReservation(ctx, arg)
}
func (r *QueriesRepo) ExpireReservationsForOrder(ctx context.Context, orderID uuid.UUID) error {
	return r.Q.ExpireReservationsForOrder(ctx, orderID)
}
func (r *QueriesRepo) UpdateReservationStatusByOrder(ctx context.Context, arg db.UpdateReservationStatusByOrderParams) error {
	return r.Q.UpdateReservationStatusByOrder(ctx, arg)
}
```
Note: confirm `CountActiveReservationsByCategory`/`CountPaidByCategory` return `int64` and `LockCategoryForUpdate` returns `db.EventCategory` against generated code.

- [ ] **Step 6: Service (reserve/release within caller's tx)**

Create `services/api/internal/modules/inventory/service.go`:
```go
package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// CheckResult carries the locked category and current counts after a capacity check.
type CheckResult struct {
	Category  db.EventCategory
	Reserved  int64
	Paid      int64
	Remaining int64
}

// CheckAndLock locks the category row (FOR UPDATE) and computes current capacity.
// Must be called inside a transaction (repo must be tx-bound). Returns ErrSoldOut
// if no slot remains.
func CheckAndLock(ctx context.Context, repo Repository, categoryID uuid.UUID) (CheckResult, error) {
	cat, err := repo.LockCategoryForUpdate(ctx, categoryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return CheckResult{}, ErrCategory
	} else if err != nil {
		return CheckResult{}, err
	}
	reserved, err := repo.CountActiveReservationsByCategory(ctx, categoryID)
	if err != nil {
		return CheckResult{}, err
	}
	paid, err := repo.CountPaidByCategory(ctx, categoryID)
	if err != nil {
		return CheckResult{}, err
	}
	rem := Remaining(int64(cat.Capacity), reserved, paid)
	if rem <= 0 {
		return CheckResult{}, ErrSoldOut
	}
	return CheckResult{Category: cat, Reserved: reserved, Paid: paid, Remaining: rem}, nil
}

// Reserve creates an ACTIVE reservation for an order. Must run in the same tx as
// the order insert (caller passes tx-bound repo).
func Reserve(ctx context.Context, repo Repository, arg db.CreateReservationParams) (db.InventoryReservation, error) {
	return repo.CreateReservation(ctx, arg)
}

// Release marks an order's ACTIVE reservation as the given terminal status
// (RELEASED on cancel, EXPIRED on timeout). Idempotent: only touches ACTIVE rows.
func Release(ctx context.Context, repo Repository, orderID uuid.UUID, status string) error {
	return repo.UpdateReservationStatusByOrder(ctx, db.UpdateReservationStatusByOrderParams{
		OrderID: orderID, Status: status,
	})
}

var _ = time.Now // referenced by callers building expires_at; remove if unused
```
Note: remove the trailing `var _ = time.Now` if the build flags it. Confirm `UpdateReservationStatusByOrderParams` field names (`OrderID`, `Status`) against generated code — sqlc names the `status = $2` param `Status` and `order_id = $1` param `OrderID`.

- [ ] **Step 7: Build and test**

Run:
```bash
cd services/api && go build ./... && go test ./internal/modules/inventory/ -v; cd ../..
```
Expected: build OK; stock tests PASS.

- [ ] **Step 8: Commit**

```bash
git add services/api/internal/modules/inventory
git commit -m "feat(inventory): add stock formula, lock-based capacity check, and reservation ops"
```

---
