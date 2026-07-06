# Phase 15 — Scanner PWA Readiness

This document records the public API contract that the Phase 15 Scanner PWA can rely on. It is generated from the post–Phase 14.1 racepack implementation and is the authoritative reference for scanner integration.

**Status:** Phase 14.1 hardening complete. Scanner PWA can be built against this API without schema changes.

---

## 1. Authentication

All racepack endpoints require a **Bearer token** issued by `POST /api/v1/auth/login`.

```
Authorization: Bearer <jwt-access-token>
```

The token identifies the staff user. The org in the URL must match a `organization_members` row for that user, OR the user must have `IsPlatformAdmin = true`.

---

## 2. Endpoints

### 2.1. Execute pickup (single ticket)

```
POST /api/v1/organizations/{orgId}/events/{eventId}/racepack/pickups
Permission: racepack.execute
Idempotency-Key: <opaque-string, max 128 chars>   ← optional but recommended
```

Request body (both formats accepted):

```jsonc
// CamelCase (current contract)
{
  "ticketId": "uuid",
  "counterId": "uuid",
  "slotId": "uuid",              // optional; enforces slot window + capacity
  "method": "SELF" | "PROXY" | "MANUAL_OVERRIDE",
  "notes": "string"
}

// Snake_case (Phase 14.0 compat)
{
  "ticket_id": "uuid",
  "counter_id": "uuid",
  "slot_id": "uuid",
  "method": "SELF",
  "notes": "string"
}
```

Success: `201 Created` with `PickupResponse` JSON.

Failure codes:
| Code | HTTP | Meaning |
|---|---|---|
| `TICKET_NOT_FOUND` | 404 | Ticket ID does not exist |
| `ALREADY_PICKED_UP` | 409 | A PICKED_UP record already exists for this ticket |
| `ORDER_NOT_PAID` | 409 | Linked order is not in PAID status |
| `BIB_MISSING` | 409 | Ticket has no BIB assigned |
| `TICKET_CANCELLED` | 409 | Ticket status = CANCELLED |
| `COUNTER_NOT_FOUND` | 404 | Counter ID does not exist |
| `COUNTER_INACTIVE` | 409 | Counter is disabled |
| `SLOT_FULL` | 409 | Slot capacity exhausted |
| `SLOT_INACTIVE` | 409 | Slot is disabled |
| `OUTSIDE_WINDOW` | 409 | Current time outside [start_time, end_time] |
| `INVALID_METHOD` | 400 | method is empty or not in the allowlist |
| `IDEMPOTENCY_CONFLICT` | 409 | Same key, different payload |
| `FORBIDDEN` | 403 | User not a member of the org |

**Idempotency**: if the request includes `Idempotency-Key`, the response body and status are cached. Replays with the same key + same payload return the cached response. Replays with the same key + different payload return 409 `IDEMPOTENCY_CONFLICT`.

### 2.2. Lookup by ticket (status check)

```
GET /api/v1/organizations/{orgId}/events/{eventId}/racepack/pickups/status?ticket_id={uuid}
GET /api/v1/organizations/{orgId}/events/{eventId}/racepack/pickups/status?ticketId={uuid}
Permission: racepack.execute
```

Success: `200 OK` with `PickupResponse` JSON, OR 404 `TICKET_NOT_FOUND` if no pickup exists for the ticket.

### 2.3. Open problem case

```
POST /api/v1/organizations/{orgId}/events/{eventId}/racepack/problem-cases
Permission: racepack.problemdesk
```

Request body:

```jsonc
// Both formats accepted
{
  "ticketId" / "ticket_id": "uuid",          // optional
  "participantId" / "participant_id": "uuid", // optional
  "reason": "string"                          // required
}
```

Validation: at least one of `ticket_id` / `participant_id` must be provided.

### 2.4. List / dashboard (read-only)

```
GET /api/v1/organizations/{orgId}/events/{eventId}/racepack/dashboard
GET /api/v1/organizations/{orgId}/events/{eventId}/racepack/pickups?limit=50&offset=0
GET /api/v1/organizations/{orgId}/events/{eventId}/racepack/problem-cases?limit=50&offset=0
GET /api/v1/organizations/{orgId}/events/{eventId}/racepack/counters
GET /api/v1/organizations/{orgId}/events/{eventId}/racepack/slots
```

Dashboard JSON shape (frontend contract):

```json
{
  "totalPickups": 123,
  "byCounter": [
    { "counter_id": "uuid", "counter_name": "Counter A", "count": 50, "active": true }
  ],
  "openCases": 4,
  "totalCounters": 3,
  "activeCounters": 2
}
```

---

## 3. Offline / Sync

### 3.1. Online scan (single pickup)

Single ticket → one POST → server returns 201 or 409 ALREADY_PICKED_UP.

### 3.2. Offline mode (deferred)

A future endpoint (`POST /racepack/pickups/batch`) for batch submission from offline queues is **not yet implemented**. Phase 15 should implement this if offline mode is in scope. Until then:

- Each individual scan must be online.
- Retry the same scan with the same `Idempotency-Key` when network returns.

### 3.3. Sync conflict resolution

If the scanner sees `ALREADY_PICKED_UP` on replay:
- The ticket has already been picked up (likely by a different staff at a different counter).
- Scanner should display "already picked up" and not retry blindly.

If the scanner sees `IDEMPOTENCY_CONFLICT`:
- Same key reused with different payload. This is a programming error. The scanner should generate a fresh key per request.

---

## 4. QR Strategy

The scanner reads the existing ticket QR token (`v=1` payload `{tid, eid, v}`), resolves it server-side via `GET /pickups/status?ticket_id={tid}`, and decides:

- 200 + PICKED_UP → "already picked up"
- 404 → call `POST /pickups` to record the pickup

The QR is generic (no mode field). The scanner's job is to interpret the server's response and the pickup record's `pickup_method`.

**Phase 15 does not need a separate pickup QR.**

---

## 5. Counters and Slots (organizer-side, not for scanner)

These endpoints exist for organizers to manage the pickup infrastructure. The Scanner PWA does not call them directly but may read counter lists for display purposes.

- `GET /racepack/counters` — list counters
- `GET /racepack/slots` — list slots (organizer view)
- `GET /api/v1/events/{eventId}/racepack/slots` — list ACTIVE slots only (participant view, may be useful for showing slot info to staff)

---

## 6. Rate Limits

Phase 14.1 added abuse categories `racepack_pickup` and `racepack_problem` but the middleware wiring was deferred (the abuse `RateChecker` does not yet expose a `Middleware` method).

**Phase 15 should:**
- Either add a `Middleware(category string) func(http.Handler) http.Handler` method to `abuse.RateChecker`
- Or apply an alternative rate-limit middleware (e.g., `tollbooth`) at the route level

Without this, a stuck scanner button can flood the API. Phase 15 should address.

---

## 7. Phase 14.1 Compatibility Notes

The Phase 14.1 hardening added:

1. **`Idempotency-Key` header** — required for safe offline retry. Scanner MUST generate a unique key per pickup attempt (e.g., UUIDv4 generated client-side).

2. **`slot_id` optional field** — if present, enforces slot window + capacity atomically. If absent, pickup proceeds without slot validation. Phase 15 should always pass slot_id when the participant pre-selected a slot.

3. **Strict method validation** — only `SELF`, `PROXY`, `MANUAL_OVERRIDE` (case-insensitive, whitespace-trimmed). Empty string rejected with 400.

4. **Service-layer ownership checks** — `ticket.event_id` and `counter.event_id` are verified against the URL `eventId`. Cross-event writes are rejected with 404 NOT_FOUND (defense-in-depth on top of route middleware).

5. **TOCTOU closure** — `SELECT ... FOR UPDATE` on the ticket row inside the pickup transaction. A scan-then-cancel race is impossible.

6. **Dashboard `openCases`** — now populated from a dedicated count query, not zeroed.

---

## 8. Sample Scanner → API flow

```
1. Staff opens Scanner PWA, scans QR.
2. QR decodes to {tid, eid, v=1}.
3. Scanner calls:
     GET /api/v1/organizations/{org}/events/{eid}/racepack/pickups/status?ticketId={tid}
4a. 200 → show "already picked up" (display bib from response).
4b. 404 → proceed.
5. Scanner calls:
     POST /api/v1/organizations/{org}/events/{eid}/racepack/pickups
     Headers: Idempotency-Key: <random-uuid>
     Body: {ticketId, counterId, method, slotId?, notes?}
6. Server returns 201 + PickupResponse, OR 409 ALREADY_PICKED_UP.
7. Scanner updates UI: "Pickup confirmed. BIB: A00042."
8. If 409 → show "already picked up" with retry-suppressed message.
```

---

## 9. Known limitations for Phase 15

- **No batch endpoint** — offline replay requires per-ticket POST. Implement `POST /racepack/pickups/batch` if offline mode is required.
- **No QR mode discriminator** — scanner must hardcode pickup mode or query `/pickups/status` for context.
- **No rate limit middleware wired** — recommend Phase 15 add this BEFORE the scanner ships to production.
- **Audit log is mutable** (pre-existing) — DB-level immutability trigger recommended for compliance hardening.

---

## 10. Test fixtures for scanner dev

When running the scanner against the test DB:

- `make test-db-setup` to provision schema + seed RBAC.
- `make dev` to start API + worker.
- Login as a Racepack Staff user from a fixture org.
- Use `POST /api/v1/auth/login` with email + password to obtain a Bearer token.
- Test endpoint at: `http://localhost:8080/api/v1/organizations/{orgId}/events/{eventId}/racepack/...`

See `services/api/tests/integration/racepack_integration_test.go` for end-to-end HTTP smoke tests that exercise these endpoints against a real PostgreSQL.

---

**End of Phase 15 readiness document.**