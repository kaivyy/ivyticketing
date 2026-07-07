# Enterprise API & Webhooks

The enterprise surface (Phase 23) gives integrators two things: a **versioned,
API-key-authenticated read API** for pulling event/order/payment data, and
**outbound webhooks** that push business events to a receiver as they happen.

Both are organizer-managed from the **API & Webhook** page
(`/org/{orgId}/enterprise`). Management routes live under `/api/v1` and are gated
on the `apikey.manage` permission; the public read API is a separate auth domain
mounted at the router root.

---

## API Keys

Keys are minted per organization and scoped to a read-only subset of the public
API. The raw key is shown **exactly once** on creation — only its SHA-256 hash
and an 8-char prefix are stored, so a lost key cannot be recovered (mint a new
one and revoke the old).

Key format: `ivyk_<48 hex chars>` (24 bytes of entropy).

### Scopes

| Scope | Grants |
|---|---|
| `events:read` | List/read events |
| `orders:read` | List/read orders |
| `payments:read` | Read payments |

Scopes are sanitized on create (trimmed, lowercased, de-duped). Unknown strings
are stored as-is but match no endpoint.

### Rate limit

Each key carries a per-minute ceiling (`rateLimitPerMin`, default 120, clamped
1–10000). The public API enforces it with a Redis fixed-window limiter keyed
`ratelimit:apikey:{keyID}`. Rate limiting is **fail-open**: a Redis outage never
blocks a legitimate integrator. Over-limit requests get `429`.

### Lifecycle

- **Create** — `POST /api/v1/organizations/{orgId}/enterprise/api-keys`
  → `201` with `rawKey` set (once).
- **List** — `GET .../api-keys` → keys with prefix, scopes, rate limit,
  `lastUsedAt`, `revokedAt`. Never the hash or raw key.
- **Revoke** — `DELETE .../api-keys/{keyId}` → `204`. Soft-revoke (sets
  `revoked_at`); a revoked key fails `Authenticate` immediately.

---

## Public Read API

Base: `/api/public/v1` (root-mounted, versioned, distinct from `/api/v1`).

### Authentication

Send the key in **either** header:

```
X-API-Key: ivyk_...
Authorization: Bearer ivyk_...
```

Auth flow: extract key → validate `ivyk_` prefix → SHA-256 → `GetAPIKeyByHash`
→ constant-time hash compare → per-key rate limit → inject org + scopes into the
request context. A best-effort goroutine stamps `last_used_at`.

Missing/invalid/revoked key → `401`. Missing required scope → `403`.

### Endpoints

| Method & Path | Scope |
|---|---|
| `GET /events` | `events:read` |
| `GET /events/{eventId}` | `events:read` |
| `GET /events/{eventId}/orders` | `orders:read` |
| `GET /orders/{orderId}` | `orders:read` |
| `GET /payments/{paymentId}` | `payments:read` |

### Cross-org isolation

`GetByID` queries are **not** org-filtered at the SQL layer, so every
single-resource handler re-checks ownership against the authenticated key's org:

```go
e, err := api.repo.GetEventByID(ctx, id)
if err != nil || e.OrganizationID != ac.OrgID {
    apperr.WriteError(w, r, ErrResourceNotFound)  // 404, not 403 — don't leak existence
}
```

A `404` (not `403`) is returned for another org's resource so existence is not
disclosed.

### Public-safe views

Responses use `publicEvent` / `publicOrder` / `publicPayment` DTOs that omit
internal columns (storage object keys, gateway/merchant refs, raw payment
instructions). Never assume a public response mirrors the internal row.

---

## Outbound Webhooks

Register an HTTPS endpoint and subscribe it to business events. On each event
the platform enqueues a delivery row per subscribed active endpoint and a
background worker (`webhook_dispatch` in `cmd/worker`) POSTs the payload.

### Events

| Event | Fires when |
|---|---|
| `order.paid` | An order transitions to PAID |
| `order.expired` | An order expires unpaid |
| `ticket.issued` | A ticket is issued |
| `ticket.checked_in` | A ticket is scanned in |

### Registration

- **Create** — `POST .../enterprise/webhooks` `{ "url", "events" }` → `201` with
  `secret` set (once). URL must be absolute **https** with a host (payloads may
  carry PII, so plaintext http is rejected). At least one event required.
- **List** — `GET .../webhooks` → endpoints (never the secret).
- **Delete** — `DELETE .../webhooks/{webhookId}` → `204` (cascades deliveries).
- **Deliveries** — `GET .../webhooks/deliveries?limit&offset` → the delivery
  ledger for observability.

### Delivery request

```
POST <your url>
Content-Type: application/json
X-IvyTicketing-Event: order.paid
X-IvyTicketing-Delivery: <delivery uuid>
X-IvyTicketing-Timestamp: <unix seconds>
X-IvyTicketing-Signature: sha256=<hex hmac>
```

### Signature verification

The signature is `HMAC-SHA256(secret, "<timestamp>.<rawBody>")`, hex-encoded,
prefixed `sha256=`. Binding the timestamp guards against replay. Verify on your
end:

```
expected = "sha256=" + hex(hmac_sha256(secret, timestamp + "." + rawBody))
constant_time_compare(expected, header["X-IvyTicketing-Signature"])
```

Reject if the timestamp is too old for your tolerance.

### Idempotency

Each business event has a stable `event_key` (e.g. `order.paid:<orderID>`). A
`UNIQUE(endpoint_id, event_key)` constraint plus `ON CONFLICT DO NOTHING` makes a
duplicate emit a no-op per endpoint. Receivers should **also** dedupe on
`X-IvyTicketing-Delivery` since retries reuse the same delivery id.

### Retries & backoff

A non-2xx response, transport error, or a since-deleted endpoint schedules a
retry with exponential backoff: **30s → 1m → 2m → 4m → …**, capped at 30m. After
**6 attempts** the delivery is parked `DEAD` and no longer retried. Delivery
statuses: `PENDING` → `DELIVERED` | `FAILED` | `DEAD`.

The dispatcher isolates failures: one bad endpoint never aborts the batch, and
`Emit` never returns an error into a core flow — outbound integration must not
break checkout or ticketing.

---

## Sandbox / testing notes

- **Signature check** — the HMAC is over `"<timestamp>.<rawBody>"`, not the body
  alone. Sign the exact bytes received before any JSON re-encoding.
- **Local receiver** — because URLs must be https, use a tunnel (e.g. an https
  forwarding service) to expose a local endpoint during development.
- **Force a retry** — return a `500` from your receiver to watch a delivery walk
  the backoff schedule and land in the ledger; return `200` to mark it
  `DELIVERED`.
- **Key hygiene** — a leaked key can only be rotated, not recovered. Revoke and
  re-mint. Keys are org-scoped, so a key can never read another org's data even
  if the resource id is guessed.
