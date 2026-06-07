# Reservation System

## Purpose

A reservation temporarily holds one inventory slot for a participant from the moment they
check out until they pay (or the order expires). This prevents two participants from
simultaneously completing checkout for the last available slot, while not permanently
removing it from available inventory until payment is confirmed.

## Reservation Lifecycle

```
checkout TX ──► ACTIVE ──[payment confirmed (Phase 6)]──► COMPLETED
                   │
                   ├──[DELETE /orders/:id by participant]──► RELEASED
                   │
                   └──[expired_at passed, worker runs]──► EXPIRED
```

| Status      | Meaning                                                                  |
|-------------|--------------------------------------------------------------------------|
| `ACTIVE`    | Slot is held; counts against remaining capacity                          |
| `COMPLETED` | Payment confirmed (Phase 6); reservation closed out, order is PAID       |
| `EXPIRED`   | `expired_at` passed and the worker released the hold                     |
| `RELEASED`  | Participant cancelled the order; slot immediately returned               |

`ACTIVE` is the only status that reduces remaining capacity. All other statuses leave the
slot free for the next buyer.

## One Reservation Per Order

The `inventory_reservations` table has a `UNIQUE` constraint on `order_id`:

```sql
UNIQUE (order_id)
```

Each checkout creates exactly one order and exactly one reservation. There is no concept
of a multi-item cart in Phase 5 — each checkout buys exactly one slot in one category.

## Expiry Time

```
reservation.expires_at = order.created_at + ORDER_EXPIRATION
```

`ORDER_EXPIRATION` is a server-side environment variable (default `15m`). The value is set
once at process start and applied to every new order. Both the order and its reservation
share the same `expires_at` value.

Example with default: a checkout at 14:00:00 expires at 14:15:00.

## Worker Idempotency

The expiration worker uses two guards to make expiration safe to run multiple times:

1. **`FOR UPDATE SKIP LOCKED`** on the orders row — concurrent worker instances skip rows
   already being processed by another instance, preventing double-processing.
2. **`AND status = 'ACTIVE'`** guard on the reservation update — if the reservation has
   already been released or completed, the update is a no-op.

This means the worker can be restarted, run on multiple nodes, or triggered manually
without risk of double-expiry or corrupted state.

## Database Schema

```sql
CREATE TABLE inventory_reservations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL UNIQUE REFERENCES orders(id),
    category_id UUID NOT NULL REFERENCES event_categories(id),
    status      TEXT NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE | COMPLETED | EXPIRED | RELEASED
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON inventory_reservations (category_id, status);
CREATE INDEX ON inventory_reservations (expires_at) WHERE status = 'ACTIVE';
```

The index on `(category_id, status)` makes the capacity count query (`count(*) WHERE category_id=X AND status='ACTIVE'`) fast. The partial index on `(expires_at) WHERE status='ACTIVE'` makes the worker's scan for expired reservations fast without scanning already-terminal rows.
