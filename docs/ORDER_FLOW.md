# Order Flow

## State Machine

```
DRAFT → PENDING_PAYMENT → PAID (Phase 6)
              ├→ EXPIRED  (expiration worker)
              └→ CANCELLED (participant)
PAID → REFUNDED (Phase 6)
```

## Status Definitions

| Status            | Meaning                                                              | Who Triggers                        | Phase 5?       |
|-------------------|----------------------------------------------------------------------|-------------------------------------|----------------|
| `DRAFT`           | Order created but not yet submitted (reserved for future multi-step flows) | System (not currently used)   | No             |
| `PENDING_PAYMENT` | Order created, inventory reserved, awaiting payment                  | Checkout handler (atomic TX)        | ✅ Yes          |
| `PAID`            | Payment confirmed; reservation upgraded to COMPLETED                 | Payment gateway webhook (Phase 6)   | No             |
| `EXPIRED`         | Order was not paid before `expired_at`; reservation released         | Expiration worker                   | ✅ Yes          |
| `CANCELLED`       | Participant explicitly cancelled the order; reservation released     | `DELETE /api/v1/orders/:id`         | ✅ Yes          |
| `REFUNDED`        | Paid order refunded after payment (Phase 6)                          | Refund handler (Phase 6)            | No             |

In Phase 5 the only reachable terminal states from `PENDING_PAYMENT` are `EXPIRED` and `CANCELLED`. `PAID` and `REFUNDED` are defined in the schema for forward compatibility.

## Order Fields

| Field          | Type      | Description                                                              |
|----------------|-----------|--------------------------------------------------------------------------|
| `id`           | UUID      | Primary key                                                              |
| `order_number` | string    | Human-readable unique identifier in `ORD-YYYYMMDD-XXXXXX` format        |
| `user_id`      | UUID      | Participant who placed the order                                         |
| `event_id`     | UUID      | The event this order is for                                              |
| `category_id`  | UUID      | The category (ticket type) purchased                                     |
| `organization_id` | UUID   | Denormalized for efficient org-scoped queries                            |
| `status`       | enum      | Current order status (see above)                                         |
| `total`        | int64     | Amount in smallest currency unit (e.g. IDR cents = Rupiah)              |
| `expired_at`   | timestamp | When the order expires if unpaid (`created_at + ORDER_EXPIRATION`)      |
| `created_at`   | timestamp | When the order was created                                               |
| `updated_at`   | timestamp | Last status change                                                       |

## Order Number Format

Orders are assigned a human-readable number in the format `ORD-YYYYMMDD-XXXXXX`:

- `ORD` — fixed prefix
- `YYYYMMDD` — UTC date of order creation (e.g. `20260607`)
- `XXXXXX` — 6 uppercase alphanumeric characters (base-36, crypto-random)

Example: `ORD-20260607-A3F9Z2`

The full number is stored with a `UNIQUE` constraint. On the rare chance of a collision (extremely unlikely given 36^6 ≈ 2.1 billion combinations per day), the generator retries up to 5 times before returning an error.

## Failure Scenarios

When a checkout request cannot be completed, the API returns a `409 Conflict` with a machine-readable error code:

| Error Code               | Meaning                                                           |
|--------------------------|-------------------------------------------------------------------|
| `SOLD_OUT`               | No remaining capacity for the category (`remaining <= 0`)        |
| `MAX_ORDER_EXCEEDED`     | The participant already has the maximum number of active orders for this category (`max_order_per_user`) |
| `EVENT_NOT_PUBLISHED`    | The event is not in `published` status                           |
| `REGISTRATION_CLOSED`    | The current time is outside the category's `registration_opens_at` / `registration_closes_at` window |

All failures cause the database transaction to roll back, leaving no partial state.

## Valid Transitions in Phase 5

```
PENDING_PAYMENT  ──[worker: expired_at passed]──►  EXPIRED
PENDING_PAYMENT  ──[DELETE /orders/:id]───────────►  CANCELLED
```

All other transitions (→ PAID, → REFUNDED) are reserved for Phase 6 (payment gateway integration).
