# Queue Modes

## Overview

Three queue modes are implemented in Phase 8. They control how participants are ordered
in the waiting line and when they are released to checkout.

## Mode Summary

| Mode | Pool | Ordering | Presale Behaviour |
|---|---|---|---|
| WAR_QUEUE | FIFO only | Join-time monotonic (Unix nano) | N/A -- always FIFO |
| RANDOMIZED_QUEUE | PRESALE + FIFO | Presale: seeded random. Post-sale: FIFO | Participants joining before `saleStartAt` get PRESALE pool scores |
| HYBRID_QUEUE | PRESALE + FIFO | Same as RANDOMIZED_QUEUE | Same presale/pool logic |

## Scoring

### FIFO Score (WAR_QUEUE, post-sale RANDOMIZED/HYBRID)

```
FifoScore(now) = now.UnixNano()
```

Wall-clock monotonic. Two calls at different nanoseconds produce different scores.
Lower score = joined earlier = higher priority. The practical resolution is
nanosecond-level, far finer than any observable race.

### Presale Score (RANDOMIZED_QUEUE, HYBRID_QUEUE presale window)

```
PresaleScore(seed, participantID) = (SHA256(seed || participantID.b[:]) as uint64) >> 1
```

- **Deterministic**: same `(seed, participantID)` always produces the same score.
  Auditable and reproducible.
- **Seed-sensitive**: different seeds produce unrelated scores. Partial knowledge
  of one seed does not help predict another event's ordering.
- **Non-negative**: `>> 1` ensures the high bit is 0, producing a positive `int64`.
- **Not time-based**: join order within the presale window does not affect rank.

### Pool Ordering

When the release engine promotes users from WAITING to ALLOWED, it uses:

```sql
-- Postgres ListWaiting ORDER BY
ORDER BY pool DESC, score ASC
```

`pool DESC` means `PRESALE` sorts before `FIFO`. `score ASC` means lower score sorts
first within each pool. This guarantees presale participants are released before any
FIFO participant, preserving the presale benefit regardless of absolute score values.

## Token Lifecycle

### Token State Machine

```
         join (POST /queue/join)
                │
                ▼
         ┌───────────┐
         │  WAITING   │
         └─────┬─────┘
               │ release engine promotes
               ▼
         ┌───────────┐
         │  ALLOWED   │──── admission expires ────► EXPIRED ────► WAITING (requeue)
         └─────┬─────┘
               │ checkout complete
               ▼
         ┌───────────┐
         │ COMPLETED  │
         └───────────┘
```

- **WAITING → ALLOWED**: Release engine calls `MarkAllowed WHERE status='WAITING'`.
  Concurrent release is idempotent -- `MarkAllowed` returns `ErrNoRows` if already
  promoted and the engine skips it.
- **ALLOWED → COMPLETED**: `OnCheckoutComplete` (best-effort hook called after
  successful checkout) marks the admission CONSUMED and the token COMPLETED.
- **ALLOWED → EXPIRED → WAITING**: If the checkout window expires before the
  participant completes checkout, the expiry worker marks the admission EXPIRED and
  requeues the token to the back of the WAITING line with a new FIFO score. The user
  gets another chance.

Additional statuses reserved: `BLOCKED` (not used in Phase 8, planned for anti-bot in
Phase 9).

### Admission Lifecycle

```
  release engine creates admission
                │
                ▼
         ┌───────────┐
         │   ACTIVE   │
         └─────┬─────┘
               │
      ┌────────┴────────┐
      │                  │
      ▼                  ▼
  CONSUMED           EXPIRED
  (checkout done)    (timeout, token requeued)
```

An admission row is created alongside `MarkAllowed`. It carries:
- `admission_id` (UUID, returned as `X-Queue-Token` to the participant)
- `token_id` (links back to the queue token)
- `checkout_expires_at` (timestamp after which the admission is invalid)
- `status`: `ACTIVE` → `CONSUMED` / `EXPIRED`

## End-to-End Sequence

```
Participant                    API                           Queue Engine
    │                           │                                 │
    │  POST /events/{id}/       │                                 │
    │  queue/join               │                                 │
    │ ──────────────────────►   │                                 │
    │                           │  CreateToken (WAITING, score)   │
    │                           │  Redis AddWaiting               │
    │    {tokenId, WAITING,     │                                 │
    │     position}             │                                 │
    │ ◄──────────────────────   │                                 │
    │                           │                                 │
    │  Poll GET /queue/status   │                                 │
    │  (every 4s)               │                                 │
    │ ──────────────────────►   │                                 │
    │    {position, estimate}   │                                 │
    │ ◄──────────────────────   │                                 │
    │                           │                                 │
    │                           │         release tick            │
    │                           │ ◄───────────────────────────    │
    │                           │  ListWaiting (ORDER BY pool     │
    │                           │  DESC, score ASC LIMIT n)       │
    │                           │  For each:  MarkAllowed →       │
    │                           │    CreateAdmission (ACTIVE)     │
    │                           │    Redis MoveToAllowed          │
    │                           │ ───────────────────────────►    │
    │                           │                                 │
    │  Poll GET /queue/status   │                                 │
    │ ──────────────────────►   │                                 │
    │    {ALLOWED, admission    │                                 │
    │     token, expiresAt}     │                                 │
    │ ◄──────────────────────   │                                 │
    │                           │                                 │
    │  POST /checkout           │                                 │
    │  Header: X-Queue-Token    │                                 │
    │ ──────────────────────►   │                                 │
    │                           │  gate.Admit → CheckAdmission    │
    │                           │  (verify ACTIVE, not expired)   │
    │                           │  → Checkout tx                  │
    │                           │  → OnCheckoutComplete hook      │
    │                           │    ConsumeAdmission             │
    │                           │    MarkCompleted                │
    │                           │    Redis RemoveAllowed          │
    │    {order}                │                                 │
    │ ◄──────────────────────   │                                 │
```

## Idempotent Join

`POST /queue/join` uses `UNIQUE (event_id, participant_id)` on the `queue_tokens` table
with `ON CONFLICT DO NOTHING`. Repeated calls from the same participant:
1. First call: inserts the token, returns `{tokenId, WAITING, position}`.
2. Subsequent calls: `ON CONFLICT DO NOTHING` returns 0 rows; the service falls through
   to `GetTokenByEventParticipant` and returns the existing token.

This handles browser refresh, tab reopen, mobile sleep, and accidental double-clicks
transparently.
