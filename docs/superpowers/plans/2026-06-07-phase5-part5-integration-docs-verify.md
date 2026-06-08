# Phase 5 Plan — Part 5: Integration, Docs & Verification (Tasks 12-14)

> Part of the Phase 5 implementation plan. Index: [2026-06-07-phase5-orders-inventory-checkout.md](2026-06-07-phase5-orders-inventory-checkout.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **Depends on:** Parts 1-3 (orders/inventory/worker). Part 4 (UI) independent. Phase 2/3/4 integration harness exists at `services/api/tests/integration/`.

---

## Task 12: HTTP-level integration tests (checkout flow over the API)

Parts 3's concurrency tests call the service directly. This task tests the full HTTP path:
auth → checkout endpoint → order → cancel → org-list, reusing the harness
(`testPool`, `truncate`, `newTestServer`, `postJSON`, `loginCreateOrg`, `createEvent`).

**Files:**
- Modify: `services/api/tests/integration/helpers_test.go`
- Create: `services/api/tests/integration/checkout_flow_test.go`

- [ ] **Step 1: Extend truncate**

Read the current `truncate` in `helpers_test.go`. Add `DELETE FROM inventory_reservations;`
and `DELETE FROM orders;` BEFORE the `event_categories`/`events` deletes (they FK to those).
Correct child-first order:
```
inventory_reservations → orders → form_fields → form_schemas →
event_categories → events → member_roles → organization_members →
audit_logs → refresh_tokens → role_permissions(org) → roles(org) → organizations → users
```
Keep all existing deletes; only add the two new lines in the right place.

- [ ] **Step 2: Helper to publish an event + add a category via API**

`createEvent` (from Phase 3/4 helpers) makes a draft event. Checkout needs a PUBLISHED event
with a category. Add to `helpers_test.go` (check it doesn't already exist):
```go
// publishEventWithCategory creates an event, adds a category (capacity), and publishes it.
// Returns (eventID, categoryID). Requires event.create/edit/publish + category.manage.
func publishEventWithCategory(t *testing.T, client *http.Client, baseURL, token, orgID string, capacity, maxOrder int) (string, string) {
	t.Helper()
	eventID := createEvent(t, client, baseURL, token, orgID, "Marathon "+orgID[:8])

	resp := postJSON(t, client, baseURL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories",
		map[string]any{
			"name": "42K", "price": 100000, "capacity": capacity,
			"registrationOpensAt":  time.Now().Add(-time.Hour).Format(time.RFC3339),
			"registrationClosesAt": time.Now().Add(time.Hour).Format(time.RFC3339),
			"maxOrderPerUser":      maxOrder,
		}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create category = %d, want 201", resp.StatusCode)
	}
	var cat struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&cat)
	resp.Body.Close()

	resp = postJSON(t, client, baseURL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/publish", map[string]any{}, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("publish = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	return eventID, cat.ID
}
```
Note: ensure `time` is imported in `helpers_test.go`.

- [ ] **Step 3: Checkout flow test**

Create `services/api/tests/integration/checkout_flow_test.go`:
```go
//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestCheckoutFlow_CreateGetListCancel(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Checkout Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 100, 2)

	// Participant (a different logged-in user).
	partToken := registerAndLogin(t, client, srv.URL, "participant@x.com")

	// Checkout.
	resp := postJSON(t, client, srv.URL+"/api/v1/events/"+eventID+"/categories/"+categoryID+"/checkout", map[string]any{}, partToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("checkout = %d, want 201", resp.StatusCode)
	}
	var order struct {
		ID          string `json:"id"`
		OrderNumber string `json:"orderNumber"`
		Status      string `json:"status"`
		Total       int64  `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()
	if order.Status != "PENDING_PAYMENT" || order.Total != 100000 {
		t.Fatalf("unexpected order: %+v", order)
	}

	// Get own order.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer "+partToken)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get order = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// List own orders.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+partToken)
	resp, _ = client.Do(req)
	var list []map[string]any
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 {
		t.Fatalf("list = %d orders, want 1", len(list))
	}

	// Organizer sees it via order.view.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/orders", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("org list = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Cancel.
	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer "+partToken)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	// Reservation released → slot back.
	var active int
	pool.QueryRow(t.Context(), `SELECT count(*) FROM inventory_reservations WHERE category_id=$1 AND status='ACTIVE'`, categoryID).Scan(&active)
	if active != 0 {
		t.Errorf("active reservations after cancel = %d, want 0", active)
	}
}

func TestCheckout_OwnershipIsolation(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Iso Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 100, 5)

	aToken := registerAndLogin(t, client, srv.URL, "a@x.com")
	bToken := registerAndLogin(t, client, srv.URL, "b@x.com")

	resp := postJSON(t, client, srv.URL+"/api/v1/events/"+eventID+"/categories/"+categoryID+"/checkout", map[string]any{}, aToken)
	var order struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()

	// B tries to read A's order → 404.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer "+bToken)
	resp, _ = client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-participant get = %d, want 404", resp.StatusCode)
	}
}
```
Note: `registerAndLogin(t, client, baseURL, email) string` returns an access token for a
fresh user. Check if a helper like this exists in `helpers_test.go` (Phase 2 had
`loginCreateOrg` which registers+logs in+creates org). If no register-only-login helper
exists, add one to `helpers_test.go`:
```go
func registerAndLogin(t *testing.T, client *http.Client, baseURL, email string) string {
	t.Helper()
	postJSON(t, client, baseURL+"/api/v1/auth/register",
		map[string]string{"email": email, "password": "pw123456", "fullName": email}, "").Body.Close()
	resp := postJSON(t, client, baseURL+"/api/v1/auth/login",
		map[string]string{"email": email, "password": "pw123456"}, "")
	var login struct{ AccessToken string `json:"accessToken"` }
	json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()
	return login.AccessToken
}
```
Also: `t.Context()` requires Go 1.24+ (we're on 1.25, fine). If unavailable, use `context.Background()` + import `context`.

- [ ] **Step 4: Run integration tests**

Run:
```bash
make test-db-setup && make test-integration
```
Expected: all Phase 2/3/4/5 integration tests PASS (including the new checkout flow).

- [ ] **Step 5: Commit**

```bash
git add services/api/tests/integration
git commit -m "test(orders): add HTTP-level checkout flow and ownership integration tests"
```

---

## Task 13: Documentation

**Files:**
- Create: `docs/ORDER_FLOW.md`
- Create: `docs/INVENTORY.md`
- Create: `docs/RESERVATION_SYSTEM.md`
- Create: `docs/CHECKOUT_FLOW.md`
- Create: `docs/PHASE5_DECISIONS.md`

- [ ] **Step 1: ORDER_FLOW.md**

Create `docs/ORDER_FLOW.md` covering the order lifecycle. Include:
- The state machine (text/ASCII):
  ```
  DRAFT → PENDING_PAYMENT → PAID (Phase 6)
                  ├→ EXPIRED (worker)
                  └→ CANCELLED (participant)
  PAID → REFUNDED (Phase 6)
  ```
- Each status meaning, who triggers each transition, and which are valid in Phase 5
  (only PENDING_PAYMENT→EXPIRED and →CANCELLED).
- Order fields + the `ORD-YYYYMMDD-XXXXXX` number format.
- Failure scenarios: sold out (409 SOLD_OUT), max order (409 MAX_ORDER_EXCEEDED),
  event not published (409), registration closed (409).

- [ ] **Step 2: INVENTORY.md**

Create `docs/INVENTORY.md` covering:
- Source of truth = PostgreSQL (`event_categories.capacity` + counts from orders/reservations). Never frontend.
- Formula: `remaining = capacity - active_reservations - paid_orders`.
- Why no separate inventory table (capacity lives on category; counts are derived).
- The counting rule: reserved = reservations with status ACTIVE; paid = orders with status PAID.
- Recovery: expired/cancelled reservations leave ACTIVE → slot returns automatically.

- [ ] **Step 3: RESERVATION_SYSTEM.md**

Create `docs/RESERVATION_SYSTEM.md` covering:
- Reservation lifecycle: ACTIVE → (COMPLETED on pay | EXPIRED on timeout | RELEASED on cancel).
- One reservation per order (UNIQUE order_id).
- `expires_at` = order created + ORDER_EXPIRATION (default 15m).
- Worker releases expired reservations; idempotency via `AND status='ACTIVE'` guards.

- [ ] **Step 4: CHECKOUT_FLOW.md**

Create `docs/CHECKOUT_FLOW.md` with a sequence diagram (text) of the atomic checkout:
```
Participant → API: POST /events/{eventId}/categories/{categoryId}/checkout
API: BEGIN TX
API → DB: SELECT event_categories WHERE id=cat FOR UPDATE   (lock)
API → DB: count ACTIVE reservations + PAID orders
API: remaining <= 0?  → ROLLBACK, 409 SOLD_OUT
API: active orders for user >= max?  → ROLLBACK, 409 MAX_ORDER_EXCEEDED
API → DB: INSERT order (PENDING_PAYMENT, expired_at)
API → DB: INSERT reservation (ACTIVE, expires_at)
API: COMMIT
API → Participant: 201 order
```
Plus: failure/recovery scenarios (concurrent checkout serialized by FOR UPDATE; crash
mid-tx → rollback leaves no partial state; expiry handled by worker).

- [ ] **Step 5: PHASE5_DECISIONS.md**

Create `docs/PHASE5_DECISIONS.md` documenting key decisions & tradeoffs:
- Why `SELECT ... FOR UPDATE` on the category row (serialization point) vs optimistic/Redis counters.
- Why no Redis for inventory in Phase 5 (Postgres is the source of truth; Redis reserved for queue Phase 8).
- Why participant = logged-in user (guest checkout deferred).
- Why one slot per checkout (multi-item cart deferred).
- Why worker is a separate binary (clean separation, independent scaling) + idempotency design.
- Order-number format + collision retry.
- Extend-don't-rewrite: what Phase 5 added without touching Phase 1-4.

- [ ] **Step 6: Commit**

```bash
git add docs/ORDER_FLOW.md docs/INVENTORY.md docs/RESERVATION_SYSTEM.md docs/CHECKOUT_FLOW.md docs/PHASE5_DECISIONS.md
git commit -m "docs: add phase 5 order/inventory/reservation/checkout documentation"
```

---

## Task 14: Full DoD verification + README + CHANGELOG

**Files:**
- Modify: `README.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Run the complete DoD gate**

Run:
```bash
# DoD #1 migrations roundtrip
make migrate-down && make migrate-up
# DoD #11/#12 unit + sqlc + build + vet
make sqlc && cd services/api && go build ./... && go vet ./... && go test ./... && cd ../..
# DoD #2-#8 integration + concurrency (race)
make test-db-setup
cd services/api && TEST_DATABASE_URL="postgres://localhost:5432/ivyticketing_test?sslmode=disable" go test -tags=integration -race ./tests/integration/... && cd ../..
# DoD #9 ui typecheck
cd packages/ui && pnpm typecheck && cd ../..
# DoD #13 extend-only sanity: confirm no Phase 1-4 module files changed except server.go/config.go (additive)
git diff --stat 31f11e7..HEAD -- services/api/internal/modules/auth services/api/internal/modules/events services/api/internal/modules/forms || echo "no changes to phase1-4 modules"
```
Expected: every command green. The git diff should show NO changes to auth/events/forms/etc modules (only new orders/inventory + additive server.go/config.go).

Mapping to spec DoD:
- #1 migrations → roundtrip
- #2 atomic checkout → checkout_flow + service tests
- #3 no oversell (200 vs 100) → inventory_concurrency
- #4 max_order → service test + checkout
- #5 cancel releases → checkout_flow
- #6 worker idempotent → expiration tests
- #7 ownership/org-list → checkout_flow ownership + org list
- #8 audit → service record() calls (ORDER_*/RESERVATION_*)
- #9 packages/ui → typecheck
- #10 docs → Task 13
- #11 go test green + race → gate
- #12 sqlc clean + vet → gate
- #13 extend-only → git diff sanity
- #14 CHANGELOG → this task

- [ ] **Step 2: README Phase 5 section**

Prepend a "Phase 5 — Orders, Inventory & Checkout" section to `README.md` (newest-first),
covering: checkout endpoint, order lifecycle, the worker (`make worker`), new env
(`ORDER_EXPIRATION`, `WORKER_INTERVAL`), and `packages/ui`. Include a curl smoke test:
```markdown
## Phase 5 — Orders, Inventory & Checkout

Participant checkout with atomic oversold-prevention, reservation system, and an
expiration worker. Plus `packages/ui` design-system foundation. No payment yet (Phase 6).

### New env
```bash
ORDER_EXPIRATION=15m
WORKER_INTERVAL=1m
```

### Run the expiration worker
```bash
make worker   # ticks every WORKER_INTERVAL, expires stale PENDING_PAYMENT orders
```

### Smoke test
```bash
# as a logged-in participant (access token from login)
curl -s -X POST localhost:8080/api/v1/events/<eventId>/categories/<categoryId>/checkout \
  -H "authorization: Bearer <accessToken>"
# → 201 { orderNumber: "ORD-...", status: "PENDING_PAYMENT", total: 100000 }

curl -s localhost:8080/api/v1/orders -H "authorization: Bearer <accessToken>"
curl -s -X DELETE localhost:8080/api/v1/orders/<orderId> -H "authorization: Bearer <accessToken>"
```
```

- [ ] **Step 3: CHANGELOG Phase 5 entry**

Prepend a Phase 5 section to `CHANGELOG.md` (newest-first, above Phase 4):
```markdown
## [Phase 5] — 2026-06-07

Orders, inventory, reservation, and checkout foundation + UI design system. Backend + UI; no payment yet.

### Added

**Orders**
- Checkout: `POST /api/v1/events/:eventId/categories/:categoryId/checkout` → PENDING_PAYMENT order + reservation (atomic)
- `GET /api/v1/orders`, `GET /api/v1/orders/:id`, `DELETE /api/v1/orders/:id` (participant-owned)
- `GET /api/v1/organizations/:orgId/events/:eventId/orders` (organizer, order.view)
- Status machine: DRAFT/PENDING_PAYMENT/PAID/EXPIRED/CANCELLED/REFUNDED
- Order number `ORD-YYYYMMDD-XXXXXX` (unique, crypto-random)

**Inventory & Reservation**
- Source of truth = PostgreSQL: `remaining = capacity - active_reservations - paid_orders`
- Oversold prevention via `SELECT ... FOR UPDATE` on the category row inside a transaction
- max_order_per_user enforced
- Reservation lifecycle ACTIVE → EXPIRED/RELEASED/COMPLETED, one per order

**Expiration worker** (`services/api/cmd/worker`)
- Ticker (`WORKER_INTERVAL`, default 1m) expires PENDING_PAYMENT orders past `expired_at`, releases reservations; idempotent (`FOR UPDATE SKIP LOCKED` + status guards)
- `make worker`

**Audit**
- ORDER_CREATED, ORDER_EXPIRED, ORDER_CANCELLED, RESERVATION_CREATED, RESERVATION_EXPIRED

**UI foundation** (`packages/ui`)
- Tailwind + Radix design system: Button, Input, Select, Textarea, Checkbox, Radio, Badge, Alert, Card, Modal, Dialog, Table, EmptyState, LoadingState, ErrorState, QueueCard, PaymentCard, TicketCard
- Theme tokens + README

**Database** (goose migrations 00012–00014)
- Tables: `orders`, `inventory_reservations`; permissions `order.create`, `order.manage`

**Config**: `ORDER_EXPIRATION`, `WORKER_INTERVAL`

**Docs**: ORDER_FLOW, INVENTORY, RESERVATION_SYSTEM, CHECKOUT_FLOW, PHASE5_DECISIONS

**Tests**
- Unit: stock formula, order-number generator, orders service (checkout/cancel/max-order/ownership), expiration idempotency, worker ticker
- Integration: HTTP checkout flow, ownership isolation
- Concurrency (`-race`): 200 vs capacity 100 → no oversell, unique order numbers, worker idempotent
```

- [ ] **Step 4: Commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: document phase 5 and update changelog"
```

---

## Done

Phase 5 complete. All 14 tasks across 5 parts: config+migrations+inventory (Part 1),
orders module (Part 2), worker+concurrency (Part 3), packages/ui (Part 4),
integration+docs+verification (Part 5). Extend-only — Phase 1-4 untouched.
