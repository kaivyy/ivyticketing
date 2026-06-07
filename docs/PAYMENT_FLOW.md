# Payment Flow

## Overview

Phase 6 adds payment creation and processing on top of the order system from Phase 5.
A participant already holds a `PENDING_PAYMENT` order (from checkout). They then call
`POST /orders/:orderId/payments` to get a payment link / QR code / VA number from the
gateway, complete the payment in their banking app, and receive confirmation via a
gateway-initiated callback to the webhook receiver.

---

## End-to-End Sequence

```
Participant → API: POST /api/v1/orders/:orderId/payments
                   { "gateway": "duitku", "method": "qris" }

API: authenticate (JWT)

API: BEGIN TX
  │
  ├─ API → DB: SELECT * FROM orders WHERE id = :orderId FOR UPDATE
  │              order not found / not owned?       → 404 PAYMENT_NOT_FOUND
  │              order.status ≠ PENDING_PAYMENT?    → 409 ORDER_NOT_PAYABLE
  │
  ├─ API → DB: SELECT * FROM payments WHERE order_id = :orderId AND status = 'PENDING'
  │              already has active payment?         → 409 PAYMENT_ACTIVE
  │
  ├─ API: generate merchant reference PAY-YYYYMMDD-XXXXXX
  │
  ├─ API: compute expiresAt = min(now + PAYMENT_DEFAULT_EXPIRY, order.expired_at)
  │              (payment expiry is always ≤ order expiry)
  │
  └─ API → Gateway: CreateCharge(merchantRef, amount, method, expiresAt)
                       [stub in V1 — returns empty result, no network call]

API → DB: INSERT INTO payments (status='PENDING', merchant_reference, …)
API → DB: INSERT audit_log (PAYMENT_CREATED)

API → Participant: 201 Created
  {
    "id": "…",
    "status": "PENDING",
    "merchantReference": "PAY-20260607-A3F9Z2",
    "payUrl": "…",
    "qrString": "…",
    "vaNumber": "…",
    "expiresAt": "2026-06-07T14:15:00Z"
  }

— Participant completes payment in their app —

Gateway → Webhook receiver: POST /webhooks/duitku  (or /webhooks/xendit)
  body: form-encoded (Duitku) or JSON (Xendit) callback

Webhook: store raw payload → verify signature → parse → dedupe → apply tx

Webhook → DB: BEGIN TX
  │
  ├─ Webhook → DB: MarkPaymentPaid WHERE id = :paymentId AND status = 'PENDING'
  │              (no-op if already PAID — concurrent guard)
  │
  └─ Webhook → DB: UpdateOrderStatus WHERE id = :orderId AND status = 'PENDING_PAYMENT'
                   → PAID
                   CompleteReservations for order

Webhook → DB: INSERT audit_log (PAYMENT_PAID)

Webhook → Gateway: 200 OK
```

---

## Payment State Machine

```
               CreatePayment
                    │
                    ▼
              ┌─────────┐
              │ PENDING │ ←────────────────────────┐
              └────┬────┘                           │
                   │                                │
       ┌───────────┼────────────┐                   │
       │           │            │                   │
       ▼           ▼            ▼              (duplicate
    PAID       EXPIRED       FAILED            callback →
                                               idempotent
                                               no-op)
```

| Status    | Description                                                                 |
|-----------|-----------------------------------------------------------------------------|
| `PENDING` | Payment created, awaiting user action. Only one PENDING payment per order.  |
| `PAID`    | Gateway confirmed payment. Triggers order → PAID transition.                |
| `EXPIRED` | Gateway or callback explicitly expired the payment (resultCode 02, etc.).   |
| `FAILED`  | Gateway explicitly reported failure.                                         |

Transitions are one-way. A PAID, EXPIRED, or FAILED payment is terminal — further
callbacks for the same reference are idempotent no-ops.

---

## How Payment Links to Order

```
orders                          payments
──────────────────────          ──────────────────────────────
id           UUID               id            UUID
status       PENDING_PAYMENT ──→ order_id      UUID  (FK)
total        int64              amount        int64  (= order.total)
expired_at   timestamptz        expires_at    timestamptz  (≤ order.expired_at)
                                merchant_ref  PAY-YYYYMMDD-XXXXXX
                                status        PENDING
```

When the payment transitions to PAID, the order transitions to PAID in the same DB
transaction. The inventory reservation is simultaneously completed (`COMPLETED`),
making the slot permanent.

If the order expires before the callback arrives (expiration worker ran), the payment
can still be marked PAID (the callback is valid), but the order remains EXPIRED and
its slot has already been released. This is documented as a reconcile edge case — see
`PAYMENT_RECONCILIATION.md`.

---

## Payment Expiry Clamping

`expiresAt = min(now + PAYMENT_DEFAULT_EXPIRY, order.expired_at)`

The payment's expiry never exceeds the order's expiry. This ensures:

- The gateway's payment link cannot expire after the order has already lapsed.
- Users cannot hold a slot by having an active payment window beyond the order window.
- Both `PAYMENT_DEFAULT_EXPIRY` (default 15m) and `ORDER_EXPIRATION` (default 15m)
  are configurable independently.
