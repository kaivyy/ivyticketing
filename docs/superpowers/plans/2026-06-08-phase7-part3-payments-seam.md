# Phase 7 Plan — Part 3: Payments seam (atomic issuance)

> Part of the Phase 7 implementation plan. Index: [2026-06-08-phase7-participant-dashboard-ticket.md](2026-06-08-phase7-participant-dashboard-ticket.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** Additive changes to payments only: one interface, one struct field, one ctor arg, one call site, one repo method. Do NOT change existing payment behavior or signatures beyond adding the optional issuer.

This is the part that closes the atomicity gap: the ticket INSERT runs on the **same tx querier** as the order→PAID transition, so PAID ⟺ ticket exists, and an issuer error rolls back the entire transaction.

---

## Task 11: Expose tx querier on payments Repository

**Files:**
- Modify: `services/api/internal/modules/payments/repository.go`

The payments `sqlcRepo` already holds a private `q *db.Queries` that is tx-bound inside `ExecTx`. Expose it so `applyPaid` can hand it to the issuer.

- [ ] **Step 1: Add `Querier()` to the Repository interface**

In `services/api/internal/modules/payments/repository.go`, add to the `Repository` interface (after `ExecTx`):
```go
	// Querier returns the underlying sqlc querier (tx-bound inside ExecTx).
	// Used to run the ticket issuer in the same transaction as the PAID transition.
	Querier() *db.Queries
```

- [ ] **Step 2: Implement it on sqlcRepo**

Add (next to the other `sqlcRepo` methods):
```go
func (r *sqlcRepo) Querier() *db.Queries { return r.q }
```

- [ ] **Step 3: Build**

Run:
```bash
cd services/api && go build ./internal/modules/payments/...; cd ../..
```
Expected: builds clean (no callers yet).

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/payments/repository.go
git commit -m "feat(phase7): expose tx querier on payments repository"
```

---

## Task 12: Add TicketIssuer interface + wire into applyPaid

**Files:**
- Modify: `services/api/internal/modules/payments/processor.go`

- [ ] **Step 1: Declare the interface + add field + ctor arg**

In `services/api/internal/modules/payments/processor.go`:

Add the interface near `AuditRecorder` (top of file):
```go
// TicketIssuer issues a ticket for a just-PAID order, using the SAME tx querier.
// Implemented by *tickets.Issuer. Must be idempotent (no-op if a ticket for the
// order already exists). Declared here so payments does not import tickets.
type TicketIssuer interface {
	IssueForOrder(ctx context.Context, q *db.Queries, order db.Order) error
}
```

Add the field to `Processor`:
```go
type Processor struct {
	repo   Repository
	audit  AuditRecorder
	issuer TicketIssuer
}
```

Change the constructor to accept the issuer:
```go
func NewProcessor(repo Repository, recorder AuditRecorder, issuer TicketIssuer) *Processor {
	return &Processor{repo: repo, audit: recorder, issuer: issuer}
}
```

- [ ] **Step 2: Call the issuer inside applyPaid (same tx)**

In `applyPaid`, locate the success block that transitions the order (currently):
```go
	if order.Status == OrderPendingPayment {
		if _, err := tx.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:       order.ID,
			Status:   OrderPaid,
			Status_2: OrderPendingPayment,
		}); err != nil {
			return err
		}
		if err := tx.CompleteReservationsForOrder(ctx, order.ID); err != nil {
			return err
		}
	} else {
```
Insert the issuer call **after** `CompleteReservationsForOrder` succeeds and **before** the closing `}` of the `if` (so it only runs when the order actually moved to PAID, and on the same `tx` querier):
```go
		if err := tx.CompleteReservationsForOrder(ctx, order.ID); err != nil {
			return err
		}
		if p.issuer != nil {
			refreshed, err := tx.GetOrderByIDForUpdate(ctx, order.ID)
			if err != nil {
				return err
			}
			if err := p.issuer.IssueForOrder(ctx, tx.Querier(), refreshed); err != nil {
				return err
			}
		}
```

> Why re-fetch `refreshed`: the local `order` was read before the status update; the issuer snapshots from the order's FKs (event/category/participant) which are unchanged, but re-fetching keeps the row consistent and uses the tx lock already held. If you prefer, pass `order` directly — the FK fields used by the issuer are not mutated by `UpdateOrderStatus`. Either is correct; the plan uses `refreshed` for clarity.

- [ ] **Step 3: Build (expected to fail at call sites)**

Run:
```bash
cd services/api && go build ./... 2>&1 | head; cd ../..
```
Expected: FAIL — `NewProcessor` now needs 3 args; callers in `server.go` (and any test/`cmd/webhook`) pass 2. Fixed in Task 13 + Part 4.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/payments/processor.go
git commit -m "feat(phase7): issue ticket atomically in payments applyPaid"
```

---

## Task 13: Processor tests — idempotency + rollback (gap closure)

**Files:**
- Create: `services/api/internal/modules/payments/processor_ticket_test.go`

These tests prove (a) one ticket per order even on duplicate PAID, and (b) if the issuer errors, the order does NOT become PAID (full rollback). They use the real `ExecTx`/repo against the test DB, mirroring existing payments integration-style tests.

> Check existing payments tests for the test-DB harness (pool setup + truncate helper). Reuse the same helper names. If payments has only unit tests with a fake repo, place these in `services/api/tests/integration/` instead and follow that harness. The code below assumes a `newTestPool(t)` + `truncateAll(t, pool)` helper exists in the payments test package; adapt names to the actual harness.

- [ ] **Step 1: Write the failing tests**

Create `services/api/internal/modules/payments/processor_ticket_test.go`:
```go
package payments_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/payments"
)

// countingIssuer records calls and can be forced to fail.
type countingIssuer struct {
	calls   int
	failNow bool
}

func (c *countingIssuer) IssueForOrder(ctx context.Context, q *db.Queries, order db.Order) error {
	c.calls++
	if c.failNow {
		return errors.New("forced issuer failure")
	}
	// real insert via tx querier (idempotent ON CONFLICT)
	_, err := q.CreateTicket(ctx, db.CreateTicketParams{
		OrganizationID: order.OrganizationID,
		EventID:        order.EventID,
		CategoryID:     order.CategoryID,
		OrderID:        order.ID,
		ParticipantID:  order.ParticipantID,
		TicketNumber:   "TIX-TEST-" + uuid.NewString()[:6],
		HolderName:     "Test",
		HolderEmail:    "t@example.com",
		EventTitle:     "E",
		CategoryName:   "C",
		QrVersion:      1,
	})
	if err != nil {
		// ON CONFLICT DO NOTHING returns no rows on duplicate → treat as no-op
		return nil
	}
	return nil
}

// TestApplyPaid_IssuesTicketOnce sets up org/event/category/user/order(PENDING_PAYMENT)/payment(PENDING),
// then applies a PAID callback twice. Order ends PAID, exactly one ticket exists.
func TestApplyPaid_IssuesTicketOnce(t *testing.T) {
	pool := newTestPool(t)
	defer truncateAll(t, pool)

	seed := seedPendingPayment(t, pool) // helper: returns ids incl. orderID, merchantRef, amount
	iss := &countingIssuer{}
	proc := payments.NewProcessor(payments.NewRepository(pool), nil, iss)

	res := paidCallback(seed) // helper builds gw.CallbackResult{Status: PAID, MerchantReference, Amount}
	if err := proc.Apply(context.Background(), "duitku", res); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := proc.Apply(context.Background(), "duitku", res); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	assertOrderStatus(t, pool, seed.orderID, "PAID")
	assertTicketCount(t, pool, seed.orderID, 1)
}

// TestApplyPaid_IssuerError_RollsBack proves the gap is closed: issuer error → order stays PENDING_PAYMENT, no ticket.
func TestApplyPaid_IssuerError_RollsBack(t *testing.T) {
	pool := newTestPool(t)
	defer truncateAll(t, pool)

	seed := seedPendingPayment(t, pool)
	iss := &countingIssuer{failNow: true}
	proc := payments.NewProcessor(payments.NewRepository(pool), nil, iss)

	err := proc.Apply(context.Background(), "duitku", paidCallback(seed))
	if err == nil {
		t.Fatal("expected error from issuer failure, got nil")
	}
	assertOrderStatus(t, pool, seed.orderID, "PENDING_PAYMENT")
	assertTicketCount(t, pool, seed.orderID, 0)
	assertPaymentStatus(t, pool, seed.orderID, "PENDING")
}
```

> Helpers (`newTestPool`, `truncateAll`, `seedPendingPayment`, `paidCallback`, `assertOrderStatus`, `assertTicketCount`, `assertPaymentStatus`): implement in a `payments_test` helpers file if not already present, reusing the existing payments integration harness. `seedPendingPayment` must insert organization, event, event_category, user, an order with status `PENDING_PAYMENT`, and a payment row with status `PENDING`, matching the FK columns. `paidCallback` returns a `gw.CallbackResult` (import the gateway package) with `Status: gw.StatusPaid`, the seeded `merchant_reference`, and matching `Amount`.

- [ ] **Step 2: Run tests to verify they fail/compile-fail**

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run 'TestApplyPaid_Issue' -v; cd ../..
```
Expected: FAIL — either helpers missing (write them) or, once compiling, the rollback test drives the new code path.

- [ ] **Step 3: Make them pass**

The processor change from Task 12 already implements the behavior. Implement any missing test helpers. Ensure `proc.Apply` → `apply` → `ExecTx` → `applyPaid` calls the issuer with `tx.Querier()`.

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run 'TestApplyPaid_Issue' -race -v; cd ../..
```
Expected: PASS — `TestApplyPaid_IssuesTicketOnce` (order PAID, 1 ticket) and `TestApplyPaid_IssuerError_RollsBack` (order PENDING_PAYMENT, 0 tickets, payment PENDING).

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/payments/processor_ticket_test.go
git commit -m "test(phase7): atomic ticket issuance — idempotent + rollback on issuer error"
```

---

Part 3 complete. The atomicity gap is closed and tested. Next: [Part 4 — HTTP + integration](2026-06-08-phase7-part4-http-integration.md).
