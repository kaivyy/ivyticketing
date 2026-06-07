# Phase 6 Design Decisions

## Why a Separate Webhook Binary

The webhook receiver runs as a distinct process (`services/api/cmd/webhook`) on a
separate port (`WEBHOOK_PORT`, default 8090) from the main API (`API_PORT`, default 8080).

**Reasons:**

- **Process isolation.** A panic or resource exhaustion in the webhook receiver (e.g.,
  from a malformed payload flood) does not affect the main API serving participant
  requests.

- **Independent scaling.** During high-traffic events, callback volume and API traffic
  spike independently. The webhook receiver can be horizontally scaled or given its own
  resource limits without touching the API deployment.

- **Security posture.** The webhook port is intended for gateway → server traffic only.
  It can be firewalled to accept connections only from known gateway IP ranges, while
  the API port remains participant-facing.

- **Independent deployability.** Callback processing logic can be updated and deployed
  without a full API restart. This matters during incident response when the callback
  pipeline needs to be patched quickly.

The shared codebase (same Go module, same processor and repository types) means no
duplication — the split is purely at the binary boundary.

---

## Why Platform Credentials (Not Per-Org) in V1

In V1, all gateways are configured at the platform level via environment variables.
Every organization uses the same Duitku merchant code and Xendit account.

**Reasons for V1 simplicity:**

- **MVP focus.** Phase 6 proves the end-to-end payment loop (callback → PAID). Per-org
  credential routing adds significant complexity (credential storage, encryption,
  key management, per-org payout routing) that is out of scope for MVP.

- **Regulatory deferral.** Per-org payment settlement requires KYB/KYC for each
  organization and potentially separate merchant agreements. These are commercial
  concerns outside the technical scope of Phase 6.

- **Platform-level routing is a known Phase 23 feature.** The `BuildPaymentRegistry`
  function is already parameterized by config so future work can replace it with a
  per-org registry lookup without changing the gateway interface or processor.

**Tradeoff:** All organizations share settlement accounts. Payouts to individual
organizers require manual ledger reconciliation until per-org routing is implemented.

---

## Why Store-Then-Process

Raw callback payloads are persisted in `payment_webhooks` before any validation or
processing occurs.

**Reasons:**

- **Never lose a callback.** Gateway SLAs for callback retries vary. If processing
  fails transiently (DB connection spike, code bug), the raw payload is already stored
  and can be replayed without asking the gateway to resend.

- **Auditability.** Every inbound callback — including rejected, duplicate, and
  malformed ones — is permanently recorded with its full payload, headers (signature),
  and processing outcome. This is essential for dispute resolution and debugging.

- **Enables the reconcile path.** The reconcile endpoint queries the gateway directly
  (`QueryStatus`) and applies the same idempotent processor. The stored webhook rows
  provide a complementary audit trail showing what the gateway actually sent vs. what
  was fetched on demand.

- **Immutable evidence.** The raw bytes are stored before signature verification.
  This means even if an attacker sends an invalid-signature callback, we have a
  permanent record of the attempt.

**Alternative considered:** Process-then-store (store only on success). Rejected
because it loses the payload on any failure before successful processing.

---

## Why Two-Layered Idempotency

Two independent guards protect against duplicate processing:

1. `dedupe_key` unique index (Layer 1)
2. DB status guards inside the transaction (Layer 2)

**Why Layer 1 alone is insufficient:**

The dedupe key only applies to webhook-row-backed calls. The reconcile path uses
`processor.Apply` directly (no webhook row, no dedupe key). If a callback and a
reconcile call race to mark the same payment PAID, Layer 1 cannot stop it — both
have different execution paths. Layer 2 (`MarkPaymentPaid WHERE status='PENDING'`)
is the correctness guarantee in this case.

**Why Layer 2 alone is insufficient:**

SQL `WHERE status='PENDING'` is enforced inside a transaction, but acquiring the
row lock (`FOR UPDATE`) takes time. Two concurrent webhooks for the same payment
could both pass the `status='PENDING'` check before either commits. The dedupe key
prevents both from even entering the transaction, making the common retry case
faster and cheaper.

**Together:** Layer 1 provides fast-path rejection for known duplicate callbacks.
Layer 2 provides correctness under all concurrent access patterns (webhook vs.
webhook, webhook vs. reconcile, reconcile vs. reconcile).

---

## Why Refund is Deferred

The `REFUNDED` status exists in the payment and order status enums but no gateway
refund API call is made.

**Reasons for deferral:**

- **Refund APIs are complex.** Each gateway has different partial/full refund semantics,
  async notification flows, idempotency keys, and settlement timing.

- **Refund authorization.** Who can authorize a refund? What approval workflow is
  needed? These are business process questions outside Phase 6 scope.

- **Forward compatibility preserved.** The `REFUNDED` enum value ensures that when
  refunds are implemented in Phase 23, existing rows can be transitioned without a
  schema migration. Code that switches on payment status will hit the default/fallthrough
  case rather than failing to compile.

Until automated refunds are available, organizers issue refunds via the gateway
dashboard directly and update the payment status manually via a support ticket.

---

## Why CreateCharge and QueryStatus Are Stubs

Both `CreateCharge` and `QueryStatus` return `fmt.Errorf("...: not yet implemented")`
in V1.

**Reasons:**

- **No sandbox credentials at implementation time.** Rather than hard-coding test
  credentials or adding conditional mock logic, the adapters are structurally complete
  but functionally stubbed. When sandbox credentials are ready, the two stub functions
  are replaced with real HTTP calls — no changes to the interface or processor.

- **Core logic is fully validated.** The signature verification, callback parsing,
  idempotency layers, and state machine are all covered by unit tests without requiring
  a live gateway connection. This is the highest-value behavior to validate in Phase 6.

- **Prevents accidental live charges.** Running tests or development against a live
  gateway with real credentials is risky. Stubs make it impossible to accidentally
  create charges or trigger real financial transactions.

- **Clear separation of concerns.** The processor tests (`processor_test.go`) validate
  the idempotency logic. Gateway adapter tests (`duitku_test.go`, `xendit_test.go`)
  validate parsing and signature verification. Full integration tests against a live
  sandbox are a Phase 23 concern when adapters are completed.
