# Phase 7 Plan — Part 4: HTTP + integration

> Part of the Phase 7 implementation plan. Index: [2026-06-08-phase7-participant-dashboard-ticket.md](2026-06-08-phase7-participant-dashboard-ticket.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** New handler/routes + additive wiring. Assumes Parts 1-3 exist.

---

## Task 14: Tickets HTTP handler + routes

**Files:**
- Create: `services/api/internal/modules/tickets/handler.go`
- Create: `services/api/internal/modules/tickets/routes.go`

- [ ] **Step 1: Implement handler.go** (mirrors payments handler conventions)

Create `services/api/internal/modules/tickets/handler.go`:
```go
package tickets

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func userID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.ListMyTickets(r.Context(), uid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) GetMine(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	tid, err := uuid.Parse(chi.URLParam(r, "ticketId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TICKET_ID", "invalid ticket id"))
		return
	}
	out, err := h.svc.GetTicketForUser(r.Context(), h.svc.repo, uid, tid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) GetQR(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	tid, err := uuid.Parse(chi.URLParam(r, "ticketId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TICKET_ID", "invalid ticket id"))
		return
	}
	token, err := h.svc.GetQRForUser(r.Context(), uid, tid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, QRResponse{QRToken: token})
}

func (h *Handler) GetByOrder(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	oid, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	out, err := h.svc.GetTicketByOrderForUser(r.Context(), uid, oid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	oid, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	out, err := h.svc.GetInvoiceForUser(r.Context(), uid, oid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) ListByOrgEvent(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid org id"))
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	out, err := h.svc.ListEventTickets(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}
```

> `GetMine` calls `h.svc.GetTicketForUser(ctx, h.svc.repo, ...)`. `repo` is an unexported field on `Service`; since the handler is in the same package, this is legal. (Alternatively add a `GetTicket(ctx, uid, tid)` wrapper on Service that passes `s.repo`; either works — keep the handler in-package.)

- [ ] **Step 2: Implement routes.go**

Create `services/api/internal/modules/tickets/routes.go`:
```go
package tickets

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts participant self-service ticket endpoints at the authn level.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/tickets", h.ListMine)
	r.Route("/tickets/{ticketId}", func(r chi.Router) {
		r.Get("/", h.GetMine)
		r.Get("/qr", h.GetQR)
	})
	r.Get("/orders/{orderId}/ticket", h.GetByOrder)
	r.Get("/orders/{orderId}/invoice", h.GetInvoice)
}

// RegisterEventRoutes mounts organizer ticket listing under /organizations/{orgId}/events/{eventId}.
// Call inside the events route group (which provides {orgId} and {eventId}).
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "ticket.view")).
		Get("/tickets", h.ListByOrgEvent)
}
```

- [ ] **Step 3: Build**

Run:
```bash
cd services/api && go build ./internal/modules/tickets/...; cd ../..
```
Expected: builds clean.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/tickets/handler.go services/api/internal/modules/tickets/routes.go
git commit -m "feat(phase7): tickets HTTP handler + routes"
```

---

## Task 15: Wire tickets into server.go + fix all NewProcessor callers

**Files:**
- Modify: `services/api/internal/app/server.go`
- Modify: `services/api/cmd/webhook/main.go`
- Modify: `services/api/internal/modules/payments/processor_test.go`
- Modify: `services/api/internal/modules/payments/reconcile_test.go`
- Modify: `services/api/internal/modules/payments/webhook/http/server_test.go`

`NewProcessor` gained a third arg (`TicketIssuer`) in Part 3. Update every caller.

- [ ] **Step 1: Wire issuer + tickets handler in server.go**

In `services/api/internal/app/server.go`:

Add import (with the other module imports):
```go
	ticketsmod "github.com/varin/ivyticketing/services/api/internal/modules/tickets"
```
Build the QR signer, issuer, tickets service/handler, and inject the issuer into the processor. Replace the existing payments construction block:
```go
	paymentsRegistry := BuildPaymentRegistry(cfg)
	paymentsRepo := paymentsmod.NewRepository(pool)
	paymentsProc := paymentsmod.NewProcessor(paymentsRepo, auditLog)
```
with:
```go
	qrSigner := ticketsmod.NewQRSigner(cfg.TicketQRSecret)
	ticketsRepo := ticketsmod.NewRepository(pool)
	ticketIssuer := ticketsmod.NewIssuer(auditLog)
	ticketsSvc := ticketsmod.NewService(ticketsRepo, qrSigner, auditLog)
	ticketsHandler := ticketsmod.NewHandler(ticketsSvc)

	paymentsRegistry := BuildPaymentRegistry(cfg)
	paymentsRepo := paymentsmod.NewRepository(pool)
	paymentsProc := paymentsmod.NewProcessor(paymentsRepo, auditLog, ticketIssuer)
```
Mount participant routes — after `paymentsHandler.RegisterRoutes(r)`:
```go
			ticketsHandler.RegisterRoutes(r)
```
Mount organizer routes — inside the events route group, after `ordersHandler.RegisterEventRoutes(r, loader)`:
```go
					ticketsHandler.RegisterEventRoutes(r, loader)
```

- [ ] **Step 2: Wire issuer in the webhook binary**

In `services/api/cmd/webhook/main.go`, the callback path issues tickets too, so it needs a real issuer. Replace:
```go
	proc := paymentsmod.NewProcessor(repo, auditLog)
```
with:
```go
	ticketIssuer := ticketsmod.NewIssuer(auditLog)
	proc := paymentsmod.NewProcessor(repo, auditLog, ticketIssuer)
```
Add the import:
```go
	ticketsmod "github.com/varin/ivyticketing/services/api/internal/modules/tickets"
```

> The webhook binary already constructs `repo := paymentsmod.NewRepository(pool)` and `auditLog`. The issuer needs no extra config (QR signing happens at read-time in the API, not at issue-time), so no `TICKET_QR_SECRET` is required in the webhook binary. Confirm `cmd/webhook/main.go` does not call `LoadConfig()` in a way that now requires `TICKET_QR_SECRET`; if it does call the shared `LoadConfig`, set `TICKET_QR_SECRET` in the webhook deploy env too (document in Part 6).

- [ ] **Step 3: Fix payments unit-test callers**

In `processor_test.go`, `reconcile_test.go`, and `webhook/http/server_test.go`, every `NewProcessor(repo, nil)` / `NewProcessor(nil, nil)` becomes a 3-arg call with a nil issuer:
```go
NewProcessor(repo, nil, nil)
```
(For `webhook/http/server_test.go`: `payments.NewProcessor(nil, nil, nil)`.) A `nil` issuer is tolerated by `applyPaid` (`if p.issuer != nil`), so these existing tests keep asserting pure payment behavior without ticket issuance.

- [ ] **Step 4: Build everything**

Run:
```bash
cd services/api && go build ./...; cd ../..
```
Expected: builds clean (all `NewProcessor` callers updated).

- [ ] **Step 5: Run payments + tickets tests**

Run:
```bash
cd services/api && go test ./internal/modules/payments/... ./internal/modules/tickets/... -race; cd ../..
```
Expected: PASS (existing payments tests unaffected; new ticket tests green).

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/app/server.go services/api/cmd/webhook/main.go services/api/internal/modules/payments
git commit -m "feat(phase7): wire tickets issuer/handler; update NewProcessor callers"
```

---

## Task 16: Integration test — full PAID → ticket flow

**Files:**
- Create: `services/api/tests/integration/phase7_ticket_test.go`

> Use the existing integration harness in `services/api/tests/integration/` (same DB `ivyticketing_test`, truncate per test, HTTP test server or direct service calls — match the Phase 5/6 integration style already present). The skeleton below expresses the assertions; adapt helper names to the actual harness.

- [ ] **Step 1: Write the integration test**

Create `services/api/tests/integration/phase7_ticket_test.go`:
```go
//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/varin/ivyticketing/services/api/internal/modules/tickets/qr"
)

// TestPhase7_PaidIssuesTicket: checkout (Phase 5) → create payment (Phase 6) →
// apply PAID callback → order PAID + reservation COMPLETED + ticket VALID + QR verifies.
func TestPhase7_PaidIssuesTicket(t *testing.T) {
	env := newEnv(t)            // harness: pool, services, qr secret
	defer env.truncate()

	a := env.seedActor(t)       // org, event(published), category(capacity>0), participant user
	orderID := env.checkout(t, a)               // Phase 5 checkout → PENDING_PAYMENT
	mref := env.createPayment(t, a, orderID)    // Phase 6 create payment → PENDING

	env.applyPaidCallback(t, "duitku", mref)    // PAID callback

	env.assertOrderStatus(t, orderID, "PAID")
	env.assertReservationStatus(t, orderID, "COMPLETED")
	ticket := env.getTicketByOrder(t, a, orderID)
	if ticket.Status != "VALID" {
		t.Fatalf("ticket status = %q, want VALID", ticket.Status)
	}

	// QR verifies with the configured secret.
	ref, err := qr.NewSigner(env.qrSecret).Verify(ticket.QRToken)
	if err != nil {
		t.Fatalf("verify QR: %v", err)
	}
	if ref.TicketID.String() != ticket.ID {
		t.Fatalf("QR ticket id mismatch")
	}
}

// TestPhase7_DuplicateCallback_OneTicket
func TestPhase7_DuplicateCallback_OneTicket(t *testing.T) {
	env := newEnv(t)
	defer env.truncate()
	a := env.seedActor(t)
	orderID := env.checkout(t, a)
	mref := env.createPayment(t, a, orderID)

	env.applyPaidCallback(t, "duitku", mref)
	env.applyPaidCallback(t, "duitku", mref)

	env.assertOrderStatus(t, orderID, "PAID")
	env.assertTicketCount(t, orderID, 1)
}

// TestPhase7_Ownership_404
func TestPhase7_Ownership_404(t *testing.T) {
	env := newEnv(t)
	defer env.truncate()
	a := env.seedActor(t)
	orderID := env.checkout(t, a)
	mref := env.createPayment(t, a, orderID)
	env.applyPaidCallback(t, "duitku", mref)
	ticket := env.getTicketByOrder(t, a, orderID)

	other := env.seedActor(t)
	status := env.getTicketStatusCode(t, other, ticket.ID) // GET /tickets/{id} as other user
	if status != 404 {
		t.Fatalf("other user GET ticket = %d, want 404", status)
	}
}

// TestPhase7_Invoice_GatedByPaid
func TestPhase7_Invoice_GatedByPaid(t *testing.T) {
	env := newEnv(t)
	defer env.truncate()
	a := env.seedActor(t)
	orderID := env.checkout(t, a)

	if code := env.getInvoiceStatusCode(t, a, orderID); code == 200 {
		t.Fatal("invoice should not be available before PAID")
	}
	mref := env.createPayment(t, a, orderID)
	env.applyPaidCallback(t, "duitku", mref)
	if code := env.getInvoiceStatusCode(t, a, orderID); code != 200 {
		t.Fatalf("invoice after PAID = %d, want 200", code)
	}
}

// TestPhase7_OrganizerList_RequiresPermission
func TestPhase7_OrganizerList_RequiresPermission(t *testing.T) {
	env := newEnv(t)
	defer env.truncate()
	a := env.seedActor(t)
	orderID := env.checkout(t, a)
	mref := env.createPayment(t, a, orderID)
	env.applyPaidCallback(t, "duitku", mref)

	noPerm := env.seedOrgMemberWithout(t, a.orgID, "ticket.view")
	if code := env.getEventTicketsStatusCode(t, noPerm, a.orgID, a.eventID); code != 403 {
		t.Fatalf("list without ticket.view = %d, want 403", code)
	}
	withPerm := env.seedOrgMemberWith(t, a.orgID, "ticket.view")
	if code := env.getEventTicketsStatusCode(t, withPerm, a.orgID, a.eventID); code != 200 {
		t.Fatalf("list with ticket.view = %d, want 200", code)
	}
}
```

- [ ] **Step 2: Implement missing harness helpers**

Add any missing methods to the integration env (reuse Phase 5/6 helpers for `checkout`, `createPayment`, `applyPaidCallback`, `assertOrderStatus`, `assertReservationStatus`). New ones: `getTicketByOrder`, `assertTicketCount`, `getTicketStatusCode`, `getInvoiceStatusCode`, `getEventTicketsStatusCode`, `seedOrgMemberWith/Without`. Set `TICKET_QR_SECRET` in the env config used by the test server.

- [ ] **Step 3: Run integration tests**

Run:
```bash
cd services/api && go test -tags=integration ./tests/integration/ -run TestPhase7 -v; cd ../..
```
Expected: PASS (all five). Requires `ivyticketing_test` DB migrated.

- [ ] **Step 4: Commit**

```bash
git add services/api/tests/integration/phase7_ticket_test.go
git commit -m "test(phase7): integration — PAID issues ticket, dedupe, ownership, invoice, perms"
```

---

## Task 17: Concurrency test — one ticket under concurrent PAID

**Files:**
- Create: `services/api/tests/integration/phase7_concurrency_test.go`

- [ ] **Step 1: Write the concurrency test**

Create `services/api/tests/integration/phase7_concurrency_test.go`:
```go
//go:build integration

package integration

import (
	"sync"
	"testing"
)

// TestPhase7_ConcurrentPaid_OneTicket fires N concurrent PAID callbacks for the
// same order; exactly one ticket must exist and the order must be PAID once.
func TestPhase7_ConcurrentPaid_OneTicket(t *testing.T) {
	env := newEnv(t)
	defer env.truncate()
	a := env.seedActor(t)
	orderID := env.checkout(t, a)
	mref := env.createPayment(t, a, orderID)

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			env.applyPaidCallbackTolerant(t, "duitku", mref) // ignores benign dup errors
		}()
	}
	wg.Wait()

	env.assertOrderStatus(t, orderID, "PAID")
	env.assertTicketCount(t, orderID, 1)
}
```

- [ ] **Step 2: Add `applyPaidCallbackTolerant` helper**

A variant of `applyPaidCallback` that calls the processor's `Apply` (or webhook `ProcessRaw`) and does not fail the test on idempotent/no-op outcomes (dedupe/duplicate). The key assertion is the final state, not per-call results.

- [ ] **Step 3: Run with -race**

Run:
```bash
cd services/api && go test -tags=integration -race ./tests/integration/ -run TestPhase7_ConcurrentPaid -v; cd ../..
```
Expected: PASS — exactly one ticket, order PAID. No data races.

- [ ] **Step 4: Commit**

```bash
git add services/api/tests/integration/phase7_concurrency_test.go
git commit -m "test(phase7): concurrent PAID yields exactly one ticket (-race)"
```

---

Part 4 complete. Backend done end-to-end. Next: [Part 5 — Frontend participant dashboard](2026-06-08-phase7-part5-frontend.md).
