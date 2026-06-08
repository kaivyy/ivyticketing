# QR Ticket

## Token Format

```
<version>.<base64url(payload)>.<base64url(hmac_sha256)>
```

Example:

```
1.eyJ0aWQiOiI3ZjNhYjJjMS0uLi4iLCJlaWQiOiI0ZTFkYzg5MC0uLi4iLCJ2IjoxfQ.dGhpcyBpcyBhIGZha2Ugc2lnbmF0dXJl
```

Fields:

| Segment              | Value                                         |
|----------------------|-----------------------------------------------|
| `version`            | Literal `1` (token schema version)            |
| `base64url(payload)` | URL-safe base64, no padding, of JSON payload  |
| `base64url(hmac)`    | URL-safe base64, no padding, of HMAC-SHA256   |

---

## Payload

```json
{
  "tid": "<ticket_uuid>",
  "eid": "<event_uuid>",
  "v": 1
}
```

| Field | Type   | Description                              |
|-------|--------|------------------------------------------|
| `tid` | string | Ticket UUID (primary lookup key at scan) |
| `eid` | string | Event UUID (scope check at scan)         |
| `v`   | int    | Payload schema version                   |

The payload contains only UUIDs and a version integer. No PII (name, email, phone),
no order amount, no participant ID. A leaked QR token reveals only that a ticket for
a specific event exists.

---

## Signing

Tokens are signed with HMAC-SHA256 using `TICKET_QR_SECRET`.

The signed message is:

```
<version> + "." + base64url(payload)
```

This binds the version prefix to the signature — a version-stripped or
version-swapped token fails verification.

`TICKET_QR_SECRET` is a separate environment variable from `JWT_SECRET`. See
"Why Separate TICKET_QR_SECRET" below.

---

## Stateless Verification

`qr.Verify(token, secret)` checks the HMAC signature without any database call:

1. Split token on `.` — expect exactly 3 segments.
2. Verify `len(version) > 0` and version matches supported set.
3. Recompute HMAC over `version + "." + encodedPayload`.
4. Compare with `hmac.Equal` (constant-time).
5. Decode and unmarshal payload JSON.

If any step fails, `Verify` returns an error and the token is rejected.

Signature verification is stateless — no DB lookup is needed to confirm the token is
genuine. At scan time (Phase 15), after signature verification passes, the scanner
fetches the ticket row by `tid` to check its live `status` (VALID / USED / CANCELLED).

This split keeps the hot scan path fast: signature check is pure CPU; DB lookup only
happens for tokens that are cryptographically valid.

---

## Versioning

The `v` field in the payload and the `qr_version` column on the `tickets` row record
the token schema version at issuance time.

This enables:

- **Secret rotation**: generate a new `TICKET_QR_SECRET`, re-sign affected tickets on
  a rolling basis. Old tokens remain verifiable until rotation is complete by accepting
  multiple active secrets during the transition window.

- **Schema rotation**: a new payload shape (e.g., adding a seat field) gets `v: 2`.
  The scanner can dispatch on `v` to handle both formats during a migration window.

Tickets issued in Phase 7 always have `qr_version = 1`.

---

## Why No Expiry Claim

JWT-style tokens often include an `exp` claim to bound token lifetime. QR tickets do
not.

Ticket validity is event-bound, not time-bound by the token itself. A ticket is valid
until the holder scans it at the gate. Encoding an expiry in the token would either:

- Expire too early (e.g., before a delayed event starts), or
- Require re-issuance when event times change.

Instead, validity is enforced at scan time by the DB `status` field:

- `VALID` → admit
- `USED` → already scanned, reject
- `CANCELLED` → refunded, reject

The token is a bearer credential that names the ticket. The source of truth for
whether it grants entry is the ticket row, checked at the moment of scan.

---

## Why Separate TICKET_QR_SECRET

`TICKET_QR_SECRET` is distinct from `JWT_SECRET` for two reasons:

1. **Blast-radius isolation.** A compromised `JWT_SECRET` lets an attacker forge
   session tokens and impersonate any user. A compromised `TICKET_QR_SECRET` lets an
   attacker forge QR tokens. Keeping the keys separate means a breach of one secret
   does not automatically compromise the other.

2. **Independent rotation.** Auth JWTs and QR tokens have different rotation cadences
   and operational procedures. Separate keys let each be rotated without affecting the
   other.

---

## Phase 7 Scope

Phase 7 generates and exposes the QR token. It does not implement verification or
scanning.

| Capability              | Phase |
|-------------------------|-------|
| Generate QR token       | 7     |
| Display QR in browser   | 7     |
| Verify/scan endpoint    | 15    |
| Scanner PWA             | 15    |
| Token revocation (scan) | 15    |
