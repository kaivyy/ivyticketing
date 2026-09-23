# Direct Registration Workflow (`NORMAL` Mode)

This workflow describes the direct registration flow used for standard community races and events operating under the `NORMAL` registration mode without virtual queue waiting rooms.

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    actor User as Participant
    participant Web as Web Frontend (Astro)
    participant API as Chi Router (/api/v1)
    participant Gate as Registration Gate
    participant Orders as Orders Service
    participant PG as PostgreSQL 16
    participant Gateway as Payment Gateway (Duitku/Xendit)

    User->>Web: Select Category & Fill Form
    Web->>API: POST /api/v1/orders/checkout
    Note over API: Bearer JWT Authn

    API->>Gate: ValidateRegistration(eventId, categoryId)
    Gate->>PG: Check event.registration_mode == 'NORMAL'
    Gate->>PG: Check NOW() between opens_at and closes_at
    Gate-->>API: Registration Approved (No queue token required)

    API->>Orders: CreateCheckout(cartPayload)
    Orders->>PG: BEGIN Transaction
    Orders->>PG: SELECT quota FROM categories WHERE id = :id FOR UPDATE
    Orders->>PG: Decrement available_quota, increment reserved_quota
    Orders->>PG: INSERT INTO inventory_reservations (expires_at = now + 15m)
    Orders->>PG: INSERT INTO orders (status = 'PENDING_PAYMENT')
    Orders->>PG: COMMIT Transaction

    Orders->>Gateway: Create Payment Invoice (Amount, OrderID)
    Gateway-->>Orders: Return Payment URL / QRIS string / VA Number
    Orders-->>API: Order Created Envelope
    API-->>Web: 201 Created (orderId, paymentUrl, expiresAt)
    Web-->>User: Display Payment Gateway & 15m Countdown
```

---

## 2. Invariants and Safety Guarantees

1. **Immediate Quota Hold**: As soon as the checkout request succeeds, the ticket is reserved for 15 minutes. No other participant can claim this inventory during this window.
2. **Atomic Failure Handling**: If the category is sold out, the transaction rolls back cleanly and returns `409 POOL_EXHAUSTED` with zero impact on database state.
3. **Automatic Abandonment Expiration**: If the participant closes their browser and never pays, the `expire_orders` background worker restores the quota exactly 15 minutes later.
