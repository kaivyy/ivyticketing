# Ticket Flow

## Overview

Phase 7 wires ticket issuance directly into the payment processor. When a payment
callback marks an order PAID, a ticket is created in the same database transaction.
There is no async worker, no queue, no eventual consistency — PAID and ticket existence
are atomic.

---

## End-to-End Sequence

```
Gateway → webhook(8090): POST /webhooks/{gateway}

webhook → payments.Processor.ProcessRaw
  1. Store raw callback (store-first, see WEBHOOK_PROCESSING.md)
  2. VerifySignature
  3. ParseCallback
  4. ClaimWebhookDedupe (Layer 1 idempotency)

Processor → ExecTx BEGIN
  │
  ├─ MarkPaymentPaid WHERE status='PENDING'
  ├─ UpdateOrderStatus(PENDING_PAYMENT → PAID)
  ├─ CompleteReservations(ACTIVE → COMPLETED)
  │
  └─ TicketIssuer.IssueForOrder(txQuerier, order)
       INSERT INTO tickets (...) ON CONFLICT (order_id) DO NOTHING
       generate HMAC-signed QR token (see QR_TICKET.md)

ExecTx COMMIT  (or ROLLBACK on any error — nothing persisted)
```

The `TicketIssuer` receives the transaction-scoped querier (`txQuerier`) so its
`INSERT` participates in the same transaction as the payment and order updates.

---

## Atomicity Guarantee

> **PAID ⟺ ticket exists.**

Either both the payment-PAID transition and the ticket row are committed, or neither
is. There is no observable intermediate state where an order is PAID but has no ticket.

If `IssueForOrder` returns an error (e.g., constraint violation other than the expected
`ON CONFLICT`, or any unexpected DB error), the entire transaction rolls back:

- Payment stays `PENDING`
- Order stays `PENDING_PAYMENT`
- Reservation stays `ACTIVE`
- No ticket row is created

This behaviour is verified by `TestProcessor_ApplyPaid_IssuerError_RollsBack`.

---

## Idempotency

Duplicate payment callbacks (gateway retry after receiving 200) are handled at two
levels:

1. **Layer 1 — dedupe_key** (inherited from Phase 6): the second callback is marked
   `DUPLICATE` before any transaction starts. No ticket insert is attempted.

2. **Layer 2 — ON CONFLICT DO NOTHING**: if two callbacks somehow bypass Layer 1 and
   both reach `IssueForOrder` concurrently, the `UNIQUE` constraint on `order_id` in
   the `tickets` table ensures only one row is inserted. The second transaction commits
   cleanly with 0 rows inserted (idempotent no-op).

The result: exactly one ticket per order, regardless of how many callbacks arrive.

See `WEBHOOK_PROCESSING.md` for the full idempotency layer description inherited from
Phase 6.

---

## Ticket State Machine

```
                 IssueForOrder (Phase 7)
                       │
                       ▼
                 ┌───────────┐
                 │   VALID   │
                 └─────┬─────┘
                       │
          ┌────────────┴────────────┐
          │                         │
          ▼                         ▼
       USED                    CANCELLED
  (Phase 15 scan)          (refund, future)
```

Phase 7 only produces `VALID` tickets. The `USED` transition (scanner marks ticket
consumed at event entry) is deferred to Phase 15. The `CANCELLED` status is reserved
for the future refund flow.

---

## TicketIssuer Interface

`payments.Processor` depends on the `TicketIssuer` interface, not on the `tickets`
package directly:

```go
type TicketIssuer interface {
    IssueForOrder(ctx context.Context, q db.Querier, order db.Order) error
}
```

This keeps the `payments` package free of an import cycle with `tickets`. The concrete
implementation (`tickets.Issuer`) is wired in `main.go`. The same dependency-inversion
pattern is used by `AuditRecorder`.

The `db.Querier` parameter is the transaction-scoped querier from `ExecTx`, which
ensures the ticket `INSERT` is part of the enclosing transaction.
