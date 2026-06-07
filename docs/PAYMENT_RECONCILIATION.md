# Payment Reconciliation

## When to Use Reconcile

Reconciliation is a manual trigger for the organizer (or finance team) when a payment
is stuck in `PENDING` but the participant reports having paid. Common scenarios:

- The gateway callback was never delivered (network failure, misconfigured callback URL,
  gateway outage).
- The callback was delivered but rejected due to a transient error (e.g., DB timeout).
- The payment gateway's retry window has closed without a successful callback.
- The participant shows a payment receipt but the order is still `PENDING_PAYMENT`.

Reconcile is a read-then-apply operation using the same idempotent processor as the
webhook path. It is safe to call multiple times.

---

## How Reconcile Works

### Endpoint

```
POST /api/v1/organizations/:orgId/payments/:paymentId/reconcile
Authorization: Bearer <accessToken>   (requires payment.manage)
```

Returns `204 No Content` on success.

### Flow

```
Caller → API: POST /organizations/:orgId/payments/:paymentId/reconcile

API: RequirePermission("payment.manage")

Reconciler.Reconcile(ctx, paymentID):
  │
  ├─ GetPaymentByID(paymentID)
  │    not found → 404
  │
  ├─ registry.Get(payment.gateway)
  │    gateway not registered → 422 GATEWAY_NOT_AVAILABLE
  │
  ├─ g.QueryStatus(ctx, payment.gatewayReference)
  │    error → 422 RECONCILE_FAILED
  │    [stub in V1 — always returns RECONCILE_FAILED until adapter is complete]
  │
  ├─ res.MerchantReference = payment.merchantReference  (fill from DB)
  │   res.Amount = payment.amount (if gateway returned 0)
  │
  └─ processor.Apply(ctx, gateway, res)
       → same idempotent path as webhook processing
       → no webhook row created (webhookID = uuid.Nil)
       → DB status guards prevent double-PAID
```

The reconcile path skips the dedupe_key layer (no webhook row). Idempotency is
guaranteed entirely by the DB status guards (`MarkPaymentPaid WHERE status='PENDING'`,
`UpdateOrderStatus WHERE status='PENDING_PAYMENT'`).

---

## Required Permission

`payment.manage` is required. This permission is assigned to:

- **Owner** role template
- **Finance** role template

Roles with only `payment.view` (e.g., Customer Service) cannot trigger reconciliation.
Participants cannot call this endpoint regardless of role.

---

## Edge Case: Order Expired Before Reconcile Runs

```
Timeline:
  T=0   Order created (PENDING_PAYMENT)
  T=15  Expiration worker runs → order → EXPIRED, reservation → EXPIRED
  T=20  Finance team notices participant's payment receipt
  T=21  Finance calls POST .../payments/:paymentId/reconcile

Reconcile outcome:
  QueryStatus → gateway returns PAID
  processor.Apply → MarkPaymentPaid succeeds (payment still PENDING)
  GetOrderByIDForUpdate → order.status = EXPIRED
  UpdateOrderStatus condition fails → 0 rows (order stays EXPIRED)
  note = "ORDER_ALREADY_EXPIRED"

Final state:
  payment  → PAID   (money confirmed received)
  order    → EXPIRED (slot already released)
```

This is a **money-in, slot-released** scenario. The participant has paid but their
slot was already given back to the pool. Manual resolution is required:

1. Contact the participant to inform them of the situation.
2. Issue a refund via the gateway dashboard (no automated refund in V1).
3. Optionally, re-checkout the participant into a new order if slots remain.

The `REFUNDED` status exists in the order/payment enums for forward compatibility
(Phase 23 will add automated refund flows). Until then, refunds are manual.

This scenario is surfaced via:
- `payment_webhooks.error_detail = "ORDER_ALREADY_EXPIRED"` (for webhook path)
- Audit log with `note: "ORDER_ALREADY_EXPIRED"` (for reconcile path)
- The payment status itself being `PAID` while the order is `EXPIRED`

Organizers can identify these cases by querying payments where
`status='PAID'` but the associated order is `EXPIRED`.
