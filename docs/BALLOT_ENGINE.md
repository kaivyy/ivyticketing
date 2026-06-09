# Ballot Engine

## Overview

The ballot system allocates tickets fairly for high-demand events where supply is less than demand. Organizers create a draw, participants apply during the open window, and the engine runs a deterministic shuffle to select winners.

## Draw Lifecycle

```
PENDING → OPEN → CLOSED → DRAWN → ANNOUNCED
                     ↓
                 CANCELLED (any stage)
```

| Status     | Meaning                                                      |
|------------|--------------------------------------------------------------|
| PENDING    | Draw created; application window not yet open                |
| OPEN       | Participants may apply or withdraw                           |
| CLOSED     | Application window closed; draw has not run yet              |
| DRAWN      | Engine has run; results recorded; not yet visible to public  |
| ANNOUNCED  | Results published; winners can convert to orders             |
| CANCELLED  | Draw aborted; all entries voided                             |

## How the Deterministic Draw Works

1. Organizer triggers `POST /ballot/{drawId}/run`.
2. Service generates a seed: `sha256(eventID + "|" + categoryID + "|" + nonce)` where nonce is a fresh UUID.
3. Seed is committed to the `ballot_draws.seed` column before any shuffling occurs (commitment scheme).
4. All APPLIED entries are loaded ordered by `id ASC` (stable, reproducible input).
5. Fisher-Yates shuffle is driven by HMAC-SHA256 keyed on the seed — same seed always produces the same order.
6. Top N entries (N = quota) → WINNER; next M (M = waitlist_size) → WAITLISTED; remainder → NOT_SELECTED.
7. Each result row stores: `result_hash = hex(sha256(seed + "|" + rank + "|" + entry_id))`.

Anyone with the seed and entry IDs can independently recompute every result hash.

## Organizer Workflow

### 1. Create a draw
```
POST /org/{orgId}/events/{eventId}/categories/{categoryId}/ballot
{
  "quota": 100,
  "waitlist_size": 50,
  "payment_window_hours": 48,
  "application_opens_at": "2026-07-01T09:00:00Z",
  "application_closes_at": "2026-07-07T23:59:59Z"
}
```

### 2. Open the draw
```
POST /org/{orgId}/ballot/{drawId}/open
```

### 3. Close the draw
```
POST /org/{orgId}/ballot/{drawId}/close
```

### 4. Run the draw
```
POST /org/{orgId}/ballot/{drawId}/run
```
Idempotent — safe to retry if interrupted.

### 5. Announce results
```
POST /org/{orgId}/ballot/{drawId}/announce
```
Issues grants to winners and adds waitlisted entries to the waitlist engine.

### 6. Export results (CSV)
```
GET /org/{orgId}/ballot/{drawId}/export
```
Returns `text/csv` with columns: `rank, outcome, ballot_entry_id, participant_id, result_hash`.

## Participant Workflow

1. Participant sees "Enter Ballot" CTA when draw status is OPEN.
2. `POST /events/{eventId}/categories/{categoryId}/ballot/apply` — enters the draw.
3. After announce, participant checks their entry status via `GET .../ballot/my-entry`.
4. WINNER: convert to order via checkout using the grant token.
5. WAITLISTED: poll for promotion; re-check `my-entry`.
6. NOT_SELECTED: no further action available.

## Result Hash Verification

Verify a specific result without trusting the server:

```
GET /org/{orgId}/ballot/{drawId}/verify?entry_id={entryID}&rank={rank}&hash={claimedHash}
```

Response:
```json
{ "valid": true }
```

To verify manually:
1. Fetch the seed from the draw record (visible after ANNOUNCED).
2. Compute: `sha256(seed + "|" + rank + "|" + entry_id)` hex-encoded.
3. Compare against the `result_hash` in the CSV export or verification endpoint.

## Error Codes

| Code                          | HTTP | Meaning                                           |
|-------------------------------|------|---------------------------------------------------|
| `BALLOT_CLOSED`               | 409  | Draw is not in OPEN status                        |
| `BALLOT_ALREADY_APPLIED`      | 409  | Participant already has an entry in this draw     |
| `BALLOT_NOT_WINNER`           | 403  | Participant is not a winner (admission check)     |
| `BALLOT_DRAW_NOT_ANNOUNCED`   | 409  | Draw seed not set — results not yet announced     |
| `BALLOT_DRAW_ALREADY_RUN`     | 409  | Draw has already been executed                    |
| `BALLOT_WITHDRAW_NOT_ALLOWED` | 409  | Entry is not APPLIED or draw is not OPEN          |
