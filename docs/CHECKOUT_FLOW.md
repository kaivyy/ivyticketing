# Checkout Flow

## Endpoint

```
POST /api/v1/organizations/{orgId}/events/{eventId}/categories/{categoryId}/checkout
Authorization: Bearer <accessToken>
```

No request body required. The participant identity is taken from the JWT.

## Sequence Diagram

```
Participant → API: POST /organizations/:orgId/events/:eventId/categories/:categoryId/checkout

API: authenticate (JWT middleware)

API: BEGIN TX
  │
  ├─ API → DB: SELECT * FROM event_categories WHERE id = :categoryId FOR UPDATE
  │              (acquires row-level lock on the category; serializes concurrent checkouts)
  │
  ├─ API → DB: SELECT event status, registration window
  │              event.status ≠ 'published'?       → ROLLBACK, 409 EVENT_NOT_PUBLISHED
  │              now() < registration_opens_at?     → ROLLBACK, 409 REGISTRATION_CLOSED
  │              now() > registration_closes_at?    → ROLLBACK, 409 REGISTRATION_CLOSED
  │
  ├─ API: compute remaining = capacity
  │          - count(ACTIVE reservations for category)
  │          - count(PAID orders for category)
  │              remaining <= 0?                    → ROLLBACK, 409 SOLD_OUT
  │
  ├─ API → DB: count PENDING_PAYMENT + PAID orders for (user, category)
  │              count >= max_order_per_user?       → ROLLBACK, 409 MAX_ORDER_EXCEEDED
  │
  ├─ API: generate unique order_number (ORD-YYYYMMDD-XXXXXX, retry on collision)
  │
  ├─ API → DB: INSERT INTO orders (status='PENDING_PAYMENT', expired_at=now()+ORDER_EXPIRATION)
  │
  └─ API → DB: INSERT INTO inventory_reservations (status='ACTIVE', expires_at=same)

API: COMMIT

API → DB: INSERT audit_log (ORDER_CREATED, RESERVATION_CREATED)

API → Participant: 201 Created
  {
    "id": "...",
    "orderNumber": "ORD-20260607-A3F9Z2",
    "status": "PENDING_PAYMENT",
    "total": 100000,
    "expiredAt": "2026-06-07T14:15:00Z"
  }
```

## Why `FOR UPDATE` on the Category Row?

The `SELECT ... FOR UPDATE` on `event_categories` is the serialization point for all
concurrent checkouts of the same category. It ensures that:

- Only one transaction at a time can read and mutate the effective remaining count.
- No two transactions can both see `remaining = 1` and both proceed to insert an order.
- The lock is released at `COMMIT` or `ROLLBACK`, minimizing contention window.

This is an intentional choice over optimistic concurrency or application-level counters.
See [PHASE5_DECISIONS.md](PHASE5_DECISIONS.md) for the full reasoning.

## Failure and Recovery Scenarios

### Concurrent checkouts (race)

Two participants hit checkout at the same time for the last slot:
- Transaction A acquires `FOR UPDATE` first. Transaction B blocks on the lock.
- A commits (order + reservation inserted). B acquires lock, recomputes `remaining = 0`,
  gets `409 SOLD_OUT`. No oversell.

### Crash mid-transaction

If the API process crashes after `BEGIN TX` but before `COMMIT`:
- PostgreSQL automatically rolls back the transaction on connection close.
- No order, no reservation, no partial state. The participant sees a network error and
  can retry.

### Order expiration (participant doesn't pay)

The expiration worker runs every `WORKER_INTERVAL` (default 1m):
- Scans `orders WHERE status='PENDING_PAYMENT' AND expired_at < now()` using
  `FOR UPDATE SKIP LOCKED` to avoid double-processing.
- Transitions order → `EXPIRED`, reservation → `EXPIRED`.
- Slot is immediately available for the next checkout.

### Participant cancels

`DELETE /api/v1/orders/:id` runs an atomic transaction:
- Transitions order → `CANCELLED`, reservation → `RELEASED`.
- Returns `204 No Content`. Slot available immediately.

### Order-number collision

Extremely rare (36^6 ≈ 2.1 billion combos per day):
- Generator retries up to 5 times on `UNIQUE` constraint violation.
- If all 5 retries fail (essentially impossible in practice), returns `500 Internal Server Error`.
