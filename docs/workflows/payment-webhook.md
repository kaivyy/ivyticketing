# Payment Webhook Processing and Ticket Issuance Workflow

This workflow traces the inbound payment notification pipeline from payment gateway callback to cryptographic ticket issuance and platform fee accounting.

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    actor Gateway as Payment Gateway (Duitku / Xendit)
    participant Receiver as Payment Webhook Endpoint
    participant PG as PostgreSQL 16
    participant Processor as Payment Processor
    participant Tickets as Ticket Issuer
    participant Billing as Platform Billing
    participant Notif as Notification Service

    Gateway->>Receiver: POST /api/v1/payments/{gateway}/callback
    Note over Receiver: Verify HMAC / Callback Token
    alt Signature Verification Fails
        Receiver-->>Gateway: 401 Unauthorized
    end

    Receiver->>PG: Calculate SHA-256 hash of payload
    Receiver->>PG: INSERT INTO payment_webhooks (hash, payload, status = 'PENDING')
    alt Webhook Hash Already Processed (Duplicate Retry)
        Receiver-->>Gateway: 200 OK (Idempotent acknowledge)
    end

    Receiver->>Processor: ProcessPayment(orderId, gatewayReference, paidAmount)
    Processor->>PG: BEGIN Transaction
    Processor->>PG: SELECT * FROM orders WHERE id = :id FOR UPDATE
    Processor->>PG: UPDATE orders SET status = 'PAID', paid_at = NOW()
    Processor->>PG: DELETE FROM inventory_reservations WHERE order_id = :id
    Processor->>PG: UPDATE categories SET reserved_quota = reserved_quota - :qty, sold_quota = sold_quota + :qty

    alt Order was generated from a Ballot Entry
        Processor->>PG: UPDATE ballot_entries SET status = 'CONVERTED' WHERE participant_id = :user_id AND category_id = :cat_id
    end

    Processor->>Tickets: IssueTicketsForOrder(orderId)
    loop For each item in order
        Tickets->>Tickets: Compute HMAC-SHA256 signature using TICKET_QR_SECRET
        Tickets->>PG: INSERT INTO tickets (order_id, participant_id, category_id, status = 'VALID', signed_qr)
    end

    Processor->>Billing: RecordPlatformFee(orderTotal, feeBps)
    Billing->>PG: INSERT INTO platform_fee_ledger (order_id, fee_amount)

    Processor->>PG: UPDATE payment_webhooks SET status = 'COMPLETED'
    Processor->>PG: COMMIT Transaction

    Processor->>Notif: Enqueue Order Confirmation Email & Ticket QR PDF
    Receiver-->>Gateway: 200 OK
```

---

## 2. Invariants and Safety Guarantees

1. **Strict Idempotency**: Payment gateways frequently retry callbacks when responses take longer than a few hundred milliseconds. IvyTicketing guarantees that duplicate callbacks never issue duplicate tickets or double-deduct fees.
2. **Atomic Inventory State Shift**: The transition of quota from `reserved_quota` to permanent `sold_quota` executes inside the same transaction as the ticket issuance.
3. **Ballot Conversion Consistency**: If the purchaser won their slot through a ballot draw, their ballot entry transitions from `WINNER` to `CONVERTED` simultaneously, permanently preventing the `ballot_winner_expirer` daemon from lapsing the entry.
