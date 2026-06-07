# Webhook Processing

## Store-First Principle

The webhook receiver's first action on every inbound callback is to persist the raw
payload in `payment_webhooks`. This happens before signature verification, before
parsing, and before any state transition. The invariant:

> **A callback is never lost.** Even if processing fails at any subsequent step, the
> raw bytes are in the database and can be replayed or audited.

This means a gateway can safely retry a callback and the system will handle it
idempotently (see Idempotency below).

---

## ProcessRaw Flow

```
Webhook receiver receives POST /webhooks/duitku (or /webhooks/xendit)

1. Read raw body
2. g.VerifySignature(headers, rawBody)     → bool (non-blocking)
3. g.ParseCallback(rawBody)                → CallbackResult, error (non-blocking)

4. INSERT INTO payment_webhooks            ← store-first, always
     { gateway, event_type, merchant_ref, gateway_ref,
       signature, signature_valid, payload, status='RECEIVED' }

   On DB error → 500 (rare; raw bytes unrecoverable)

5. signature_valid == false?
     → UPDATE payment_webhooks SET status='REJECTED', error_detail='INVALID_SIGNATURE'
     → return ErrInvalidSignature → HTTP 401

6. parseErr != nil?
     → UPDATE payment_webhooks SET status='FAILED', error_detail=parseErr
     → return error → HTTP 200 (do not trigger gateway retry)

7. applyWithWebhook(webhookID, gatewayName, callbackResult)
     → see "Apply Transaction" below
```

---

## Apply Transaction

```
applyWithWebhook(webhookID, gatewayName, res)

1. Compute dedupeKey = gateway + ":" + gatewayRef + ":" + status

2. ClaimWebhookDedupe(webhookID, dedupeKey)
     → INSERT INTO webhook_dedupe_keys (dedupe_key, webhook_id)
     → ON CONFLICT DO NOTHING + check rows affected
     → 0 rows affected → ErrDuplicateDedupe
          → UPDATE payment_webhooks SET status='DUPLICATE'
          → return nil  ← idempotent no-op

3. BEGIN TX
   │
   ├─ SELECT * FROM payments WHERE merchant_reference = :ref FOR UPDATE
   │    not found?          → REJECTED, ErrPaymentNotFound
   │    amount mismatch?    → REJECTED, ErrAmountMismatch
   │    status ≠ PENDING?   → PROCESSED (no-op), return nil
   │
   ├─ switch res.Status:
   │
   │  case PAID:
   │    MarkPaymentPaid WHERE id=:id AND status='PENDING'
   │      → no rows (concurrent)  → PROCESSED, return nil
   │    GetOrderByIDForUpdate(payment.order_id)
   │    if order.status == PENDING_PAYMENT:
   │      UpdateOrderStatus PENDING_PAYMENT → PAID
   │      CompleteReservationsForOrder
   │    else:
   │      note = "ORDER_ALREADY_" + order.status
   │    UPDATE payment_webhooks SET status='PROCESSED', error_detail=note
   │    INSERT audit_log (PAYMENT_PAID)
   │
   │  case EXPIRED / FAILED:
   │    UpdatePaymentStatus → EXPIRED or FAILED
   │    UPDATE payment_webhooks SET status='PROCESSED'
   │
   │  default (PENDING):
   │    UPDATE payment_webhooks SET status='PROCESSED'
   │
   └─ COMMIT
```

---

## Idempotency Layers

Two independent layers guard against double-processing:

### Layer 1 — dedupe_key (fast path)

The dedupe key `gateway:gatewayRef:status` is inserted into a unique index before the
main transaction begins. If two callbacks arrive simultaneously with identical content,
the second one hits a unique-constraint violation and is immediately marked `DUPLICATE`
without touching any payment or order row.

This covers the common case: gateways retrying unacknowledged callbacks.

### Layer 2 — DB status guards (correctness under concurrency)

Inside the transaction, all state changes use conditional `WHERE status = 'PENDING'`
guards:

- `MarkPaymentPaid` only applies if `payment.status = 'PENDING'` (SQL-level `WHERE`)
- `UpdateOrderStatus` only applies if `order.status = 'PENDING_PAYMENT'`

If a concurrent process (e.g., a second webhook or a reconcile call) beats us to
marking the payment PAID, both guards return 0 rows and the transaction commits as
a harmless no-op.

Layer 1 alone is not sufficient because a race between a callback and a reconcile
call would not share a dedupe key (reconcile has no webhook row). Layer 2 catches that.

---

## Race: Order Already EXPIRED When Callback Arrives

```
Timeline:
  T=0  Participant creates payment (PENDING)
  T=14 Expiration worker runs → order → EXPIRED, reservation → EXPIRED
  T=15 Gateway callback arrives → payment.merchant_reference found, status PENDING

ProcessRaw proceeds normally (signature valid, parse OK, dedupe passes).

Inside the transaction:
  MarkPaymentPaid  → succeeds (payment still PENDING)
  GetOrderByIDForUpdate  → order.status = EXPIRED
  UpdateOrderStatus condition fails  → 0 rows, no-op
  note = "ORDER_ALREADY_EXPIRED"

Result:
  payment → PAID     (gateway confirmed real money)
  order   → EXPIRED  (slot already released, unchanged)
  payment_webhooks.error_detail = "ORDER_ALREADY_EXPIRED"
```

The payment is marked PAID because money was received. The order remains EXPIRED
because the slot was already released to the next buyer. This scenario is surfaced in
`error_detail` and requires manual resolution (contact participant, issue refund) until
a refund flow is implemented. See `PAYMENT_RECONCILIATION.md` for details.

---

## Duplicate Callback Handling

When the same callback arrives a second time (gateway retry after receiving 200):

1. `VerifySignature` passes (same payload)
2. `ParseCallback` succeeds (same payload)
3. Raw row inserted (each callback gets its own row for auditability)
4. `ClaimWebhookDedupe` fails → ErrDuplicateDedupe
5. Webhook row updated to `status='DUPLICATE'`
6. Return nil → HTTP 200 (gateway sees success, stops retrying)

The payment and order states are untouched. No audit log entry is added.

---

## Webhook Processing Statuses

| Status      | Meaning                                                               |
|-------------|-----------------------------------------------------------------------|
| `RECEIVED`  | Raw payload stored, processing not yet complete                       |
| `PROCESSED` | Successfully applied (or idempotent no-op on already-final payment)   |
| `REJECTED`  | Invalid signature, payment not found, or amount mismatch              |
| `DUPLICATE` | Same dedupe key as a prior webhook; no-op                             |
| `FAILED`    | Parse error or unexpected internal error                              |
