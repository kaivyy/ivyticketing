# Orders, Inventory, and Payment Processing

This document explains how IvyTicketing handles the commercial transaction pipeline: cart checkout, PostgreSQL inventory locking, gateway payment dispatch, webhook verification, and fee ledgering.

---

## 1. Inventory Reservation Mechanics

IvyTicketing guarantees zero overselling under concurrent load through strict database row locking.

```mermaid
flowchart TD
    Checkout[POST /api/v1/orders/checkout] --> TxBegin[BEGIN PostgreSQL Transaction]
    TxBegin --> LockRow["SELECT available_quota, reserved_quota FROM categories WHERE id = :id FOR UPDATE"]
    LockRow --> CheckQuota{"available_quota >= quantity?"}

    CheckQuota -->|No| RollbackQuota[ROLLBACK & Return 409 POOL_EXHAUSTED]
    CheckQuota -->|Yes| DecrementQuota["available_quota = available_quota - quantity<br/>reserved_quota = reserved_quota + quantity"]

    DecrementQuota --> CreateHold[INSERT INTO inventory_reservations expires_at = now + 15m]
    CreateHold --> CreateOrder[INSERT INTO orders status = PENDING_PAYMENT]
    CreateOrder --> ConsumeGrant[UPDATE access_grants SET status = CONSUMED]
    ConsumeGrant --> TxCommit[COMMIT Transaction]
    TxCommit --> DispatchGateway[Call Duitku / Xendit to Create Invoice]
```

### The 15-Minute Reservation Hold

- When a checkout order is created, the inventory is immediately decremented from `available_quota` and incremented in `reserved_quota`.
- An `inventory_reservations` record is inserted with an expiration timestamp calculated as `NOW() + ORDER_EXPIRATION` (default: 15 minutes).
- If the participant fails to pay within 15 minutes, the background daemon [`expire_orders`](file:///root/ivyticketing/services/api/cmd/worker/main.go) executes:
  ```sql
  UPDATE categories
  SET available_quota = available_quota + r.quantity,
      reserved_quota = reserved_quota - r.quantity
  FROM inventory_reservations r
  WHERE r.order_id = orders.id
    AND orders.status = 'PENDING_PAYMENT'
    AND r.expires_at < NOW();

  UPDATE orders
  SET status = 'EXPIRED'
  WHERE status = 'PENDING_PAYMENT'
    AND expires_at < NOW();
  ```

---

## 2. Payment Gateway Integrations

IvyTicketing integrates with Southeast Asia's leading payment gateways via a unified driver interface:

### Supported Gateways

| Gateway Provider | Environments | Payment Channels | Verification Method |
| :--- | :--- | :--- | :--- |
| **Duitku** | Sandbox, Production | QRIS, Virtual Accounts (BCA, Mandiri, BNI, BRI), Credit Cards, E-Wallets (GoPay, OVO, ShopeePay) | SHA256 / MD5 signature hash matching `merchantCode + amount + merchantOrderId + apiKey` |
| **Xendit** | Sandbox, Production | XenPlatform, QRIS, Virtual Accounts, Credit Cards, PayLater | Static header verification matching `x-callback-token` |

---

## 3. Webhook Processing and Idempotency

Payment gateways communicate transaction settlements via asynchronous HTTP POST webhooks. IvyTicketing treats webhooks with strict cryptographic verification and idempotency defense.

```mermaid
sequenceDiagram
    autonumber
    actor Gateway as Payment Gateway (Duitku/Xendit)
    participant Receiver as API Webhook Receiver
    participant PG as PostgreSQL (payment_webhooks)
    participant Orders as Orders Service
    participant Tickets as Ticket Issuer
    participant Billing as Platform Billing

    Gateway->>Receiver: POST /api/v1/payments/{gateway}/callback
    Receiver->>Receiver: Verify HMAC / Callback Token
    alt Invalid Signature
        Receiver-->>Gateway: 401 Unauthorized
    end

    Receiver->>PG: INSERT INTO payment_webhooks (payload, hash, status=PENDING)
    alt Duplicate Webhook Hash (Already Processed)
        Receiver-->>Gateway: 200 OK (Idempotent acknowledge)
    end

    Receiver->>Orders: MarkOrderPaid(orderId)
    Orders->>PG: BEGIN Tx: UPDATE orders SET status = 'PAID'
    Orders->>Tickets: IssueTicketsForOrder(orderId)
    Tickets->>PG: INSERT tickets (status: VALID, signed_qr)
    Orders->>Billing: RecordPlatformFee(orderTotal)
    Billing->>PG: INSERT INTO platform_fee_ledger
    Orders->>PG: COMMIT Tx

    Receiver->>PG: UPDATE payment_webhooks SET status = 'COMPLETED'
    Receiver-->>Gateway: 200 OK
```

### Idempotency Invariants

- **Deduplication Hash**: Every inbound webhook payload is hashed (SHA-256) and recorded in `payment_webhooks`.
- **Zero Double-Issuance**: If network retries cause a gateway to deliver the same payment notification 5 times, subsequent requests encounter an existing processed record and immediately return `200 OK` without triggering duplicate ticket creation or fee calculations.
