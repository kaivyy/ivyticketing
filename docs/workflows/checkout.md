# Checkout and Inventory Reservation Workflow

The checkout workflow handles cart conversion, form validation, row-level PostgreSQL inventory locking, coupon calculation, and payment intent initialization.

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    actor User as Participant
    participant API as Chi HTTP Router
    participant Orders as Orders Service
    participant Forms as Forms Validator
    participant Coupons as Coupon Engine
    participant PG as PostgreSQL 16
    participant Gateway as Payment Gateway

    User->>API: POST /api/v1/orders/checkout (Payload + Idempotency-Key)
    API->>Orders: Checkout(ctx, payload)

    Orders->>PG: Check Idempotency-Key in idempotency_records
    alt Key already exists with identical payload
        Orders-->>API: Return existing cached order response (200 OK)
    end

    Orders->>Forms: ValidateFormData(categoryId, payload.FormData)
    Forms-->>Orders: Form schema valid

    alt Coupon Code Supplied
        Orders->>Coupons: ValidateCoupon(couponCode, subtotal)
        Coupons-->>Orders: Apply Discount Amount
    end

    Orders->>PG: BEGIN Transaction
    Orders->>PG: SELECT available_quota, reserved_quota FROM categories WHERE id = :id FOR UPDATE
    alt available_quota < requested_quantity
        Orders->>PG: ROLLBACK
        Orders-->>API: 409 POOL_EXHAUSTED
    end

    Orders->>PG: UPDATE categories SET available_quota = available_quota - :qty, reserved_quota = reserved_quota + :qty
    Orders->>PG: INSERT INTO inventory_reservations (order_id, expires_at = NOW() + 15m)

    alt Access Grant Supplied
        Orders->>PG: UPDATE access_grants SET status = 'CONSUMED' WHERE id = :grant_id AND status = 'ACTIVE'
    end

    Orders->>PG: INSERT INTO orders (order_number, status = 'PENDING_PAYMENT', expires_at = NOW() + 15m, total)
    Orders->>PG: INSERT INTO order_items (order_id, category_id, price, form_data)
    Orders->>PG: COMMIT Transaction

    Orders->>Gateway: CreatePaymentIntent(orderId, total, paymentMethod)
    Gateway-->>Orders: Return paymentUrl / QRIS string / VA number
    Orders-->>API: 201 Created (orderId, orderNumber, paymentUrl, expiresAt)
    API-->>User: Order Confirmation & Payment Instructions
```

---

## 2. Invariants and Safety Guarantees

1. **Row-Level Locking Invariant**: Inventory decrement is protected by `SELECT ... FOR UPDATE` inside the database transaction. Parallel checkouts queue at the row lock, making overselling mathematically impossible.
2. **Idempotent Retries**: If network issues cause the client to retry the checkout request with the same `Idempotency-Key`, the system returns the existing order rather than reserving duplicate inventory.
3. **Strict 15-Minute Deadline**: The order record and inventory hold share identical expiration timestamps (`NOW() + 15m`).
