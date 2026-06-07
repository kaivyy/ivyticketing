# Inventory System

## Source of Truth: PostgreSQL

Inventory is never managed in memory, in a cache, or on the frontend. The authoritative
remaining-capacity count is always derived on demand from the database. This ensures
correctness across multiple API instances, worker processes, and concurrent requests.

The capacity ceiling lives on the `event_categories` table:

```sql
event_categories.capacity  -- max slots ever available for a category
```

## Remaining Capacity Formula

```
remaining = capacity - active_reservations - paid_orders
```

Expanded:

```sql
SELECT
  ec.capacity
  - (
      SELECT count(*) FROM inventory_reservations ir
       WHERE ir.category_id = ec.id
         AND ir.status = 'ACTIVE'
    )
  - (
      SELECT count(*) FROM orders o
       WHERE o.category_id = ec.id
         AND o.status = 'PAID'
    )
  AS remaining
FROM event_categories ec
WHERE ec.id = $1
FOR UPDATE;  -- held during checkout transaction
```

The `FOR UPDATE` row lock serializes concurrent checkouts on the same category, preventing
overselling. See [CHECKOUT_FLOW.md](CHECKOUT_FLOW.md) for the full transaction sequence.

## Why No Separate Inventory Table?

A dedicated inventory counter table (e.g. `inventory_counts`) was considered and rejected:

1. **Single source of truth** — `event_categories.capacity` is the ceiling; counts derived
   from `orders` and `inventory_reservations` are always consistent with the real data.
2. **No double-write** — a counter table requires atomic updates in sync with order/reservation
   inserts, adding complexity and a failure mode.
3. **Simplicity** — `SELECT count(*)` on indexed columns is fast enough at the scale targeted
   by Phase 5. A Redis counter (Phase 8+) can be added as a read-cache without changing the
   write path.

## Counting Rules

| What counts as "reserved" | `inventory_reservations.status = 'ACTIVE'` |
|---|---|
| What counts as "paid" (sold) | `orders.status = 'PAID'` |
| What does NOT count | `EXPIRED`, `RELEASED`, `COMPLETED` reservations; `CANCELLED`, `EXPIRED` orders |

In other words:
- A `PENDING_PAYMENT` order that has not expired: its reservation is `ACTIVE`, so it
  occupies one slot even though payment hasn't occurred yet.
- Once the order expires or is cancelled, the reservation transitions out of `ACTIVE`
  and the slot is freed.
- Once the order is paid (Phase 6), the reservation is marked `COMPLETED` and the paid
  count increases by one — the net effect on capacity is zero (reservation freed, paid added).

## Slot Recovery

Slots are automatically reclaimed in two ways:

1. **Cancellation** — `DELETE /api/v1/orders/:id` transitions the order to `CANCELLED` and
   the reservation to `RELEASED` atomically in a single transaction. The slot is available
   immediately for the next checkout.

2. **Expiration** — The expiration worker periodically (every `WORKER_INTERVAL`, default 1m)
   scans for `PENDING_PAYMENT` orders whose `expired_at` is in the past. It transitions them
   to `EXPIRED` and their reservations to `EXPIRED` as well. The next inventory count after
   the worker runs will reflect the freed slots.

No manual intervention or batch job is required.
