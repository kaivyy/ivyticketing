# Queue Operations

## Admin Controls

All admin endpoints are mounted under `/organizations/{orgId}/events/{eventId}` and
require `queue.manage` permission (assigned to Owner + Manager role templates in
migration 00025).

### Pause / Resume

```
POST /organizations/{orgId}/events/{eventId}/queue/pause
POST /organizations/{orgId}/events/{eventId}/queue/resume
```

Pausing sets the queue control state to `PAUSED`. The release engine's job checks
`ctrl.State != StateRunning` on each tick and skips paused events -- no new
admissions are created. Participants already in `ALLOWED` state can still complete
checkout within their existing admission window. New joins are still accepted
(tokens are created as WAITING) but they remain in the waiting line until the
queue is resumed.

Resuming sets the state back to `RUNNING`. The release engine picks up on the next
tick and begins promoting WAITING tokens at the configured rate.

### Set Release Rate

```
PUT /organizations/{orgId}/events/{eventId}/queue/release-rate
Body: { "rate": 50 }
```

Changes the per-tick release rate without pausing the queue. Zero or negative
values are accepted but the release engine will skip events with `rate <= 0`.
This allows live tuning during high-traffic events -- start conservative, observe
inventory velocity, increase if there is headroom.

### Set Schedule

```
PUT /organizations/{orgId}/events/{eventId}/queue/schedule
Body: {
  "seed": "abc123def456",
  "saleStartAt": "2026-07-01T10:00:00Z",
  "presalePoolOpenAt": "2026-06-30T10:00:00Z"
}
```

Configures the sale window and randomization seed for `RANDOMIZED_QUEUE` and
`HYBRID_QUEUE` modes. If `seed` is empty or omitted, a random 32-hex-char seed
is auto-generated and persisted. Setting a known seed makes the presale ordering
reproducible and auditable.

`saleStartAt`: After this time, new joins use FIFO scoring regardless of pool.
`presalePoolOpenAt`: Time at which the presale pool opens for joins.

### Queue Stats

```
GET /organizations/{orgId}/events/{eventId}/queue/stats
Response: {
  "waiting": 5423,
  "allowed": 47,
  "releaseRate": 100,
  "state": "RUNNING"
}
```

Reads waiting/allowed counts from Redis (degraded if Redis is down), rate and
state from Postgres.

## Redis and Postgres: Dual-Store Architecture

### Postgres -- Durable Source of Truth

All queue state is persisted in Postgres via three tables:

- `queue_tokens` (migration 00022): participant tokens with status, pool, and score.
  `UNIQUE (event_id, participant_id)` enforces one token per participant per event.
- `queue_admissions` (migration 00023): admission windows created when tokens
  are promoted to ALLOWED. Foreign key to `queue_tokens.id`.
- `queue_control` (migration 00024): per-event control row: state, release rate,
  randomization seed, sale timestamps.

Postgres is the authoritative source. All mutations (join, promote, complete, expire,
requeue) are written to Postgres first, within transactions.

### Redis -- Real-Time Position and Counts

Redis sorted sets provide O(log N) rank lookup and fast counts for the
participant-facing status endpoint:

- `queue:{eventID}:waiting` -- sorted set of participant UUIDs scored by join score.
- `queue:{eventID}:allowed` -- sorted set of participant UUIDs scored by
  checkout expiration Unix timestamp.

Redis is populated as a **best-effort side effect** after Postgres writes:

1. `AddWaiting` -- called after token creation.
2. `MoveToAllowed` -- called after `MarkAllowed` + `CreateAdmission`.
3. `MoveToWaiting` -- called after admission expiry + requeue.
4. `RemoveAllowed` -- called after checkout complete.

If any Redis operation fails, the error is silently swallowed (`_ =`). The
participant's position display degrades gracefully (shows position 0) but the
queue state in Postgres is correct and the checkout admission check still works.

### Redis-Down Recovery

When Redis is unavailable:
- `GET /queue/status` returns `position: 0` (degraded). The participant sees
  "Position: --" or a generic waiting indicator. The status and admission token
  fields remain correct (sourced from Postgres).
- Checkout admission verification (`CheckAdmission`) reads from Postgres only --
  no Redis dependency. The checkout path is unaffected.
- Admin stats (`GET /queue/stats`) return 0 for waiting/allowed counts.

Redis sorted sets are fully rebuildable from Postgres `WAITING` tokens. A
rebuild script or endpoint can be added in a future phase; the current
architecture makes it straightforward (iterate `SELECT * FROM queue_tokens WHERE
status = 'WAITING' ORDER BY pool DESC, score ASC`, `ZADD` to Redis).

## Release Engine

The release engine runs as a worker job that fires every `QUEUE_RELEASE_INTERVAL`
(default 10s). On each tick:

```
ReleaseJob(window):
  For each event with RUNNING queue:
    1. Read control row → state, releaseRate
    2. Skip if state != RUNNING or releaseRate <= 0
    3. Call Release(eventID, releaseRate, window)
    4. Release:
       a. ListWaiting(eventID, limit=rate)  -- ORDER BY pool DESC, score ASC
       b. For each WAITING token:
          - ExecTx: MarkAllowed + CreateAdmission(expiresAt = now + window)
          - If MarkAllowed returns ErrNoRows (already promoted) → skip
          - Redis: MoveToAllowed
```

The release is **pure rate** -- it does not check inventory before promoting.
Decision Q9: inventory lock (`SELECT ... FOR UPDATE` on the category row during
checkout) is the oversold backstop. This avoids TOCTOU races and complexity in
the release path.

## Admission Expiry Worker

A separate worker job runs every `QUEUE_RELEASE_INTERVAL`, scanning for ACTIVE
admissions whose `checkout_expires_at` is in the past.

```
AdmissionExpiryJob(limit):
  ListExpiredAdmissions(limit)  -- ACTIVE, checkout_expires_at < now()
  For each expired admission:
    ExecTx:
      ExpireAdmission (ACTIVE → EXPIRED)
      Requeue token (WAITING → WAITING, new FIFO score)
    Redis: MoveToWaiting(participantID, newScore)
```

Decision Q10: expired admissions are requeued to the **back of the waiting line**
with a fresh FIFO score (`time.Now().UnixNano()`). The user is not permanently
ejected -- they automatically get another chance when the release engine reaches
their position again. This avoids a support burden (user ejected, files complaint,
manual intervention) at the cost of potentially cycling the same user through the
queue multiple times.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `QUEUE_RELEASE_INTERVAL` | 10s | How often the release engine ticks |
| `QUEUE_DEFAULT_RELEASE_RATE` | 100 | Tokens promoted per tick per running event |
| `QUEUE_CHECKOUT_WINDOW` | 5m | How long an ALLOWED admission remains valid |

## War-Day Runbook

### Pre-Sale Setup

1. Configure the event's registration mode:
   ```
   PUT /registration  { "defaultMode": "WAR_QUEUE" }
   ```
2. Set the schedule (for RANDOMIZED/HYBRID modes):
   ```
   PUT /queue/schedule  { "saleStartAt": "...", "seed": "..." }
   ```
3. Verify the release rate is appropriate. Start conservative (e.g. 50/tick) and
   increase if inventory velocity is healthy.
4. Confirm Redis is up and responding to `PING`.
5. Open the organizer queue controls page (`/organizations/{orgId}/events/{eventId}/queue-controls`) for live monitoring.

### During Sale

1. Monitor `GET /queue/stats` periodically:
   - `waiting` count: how many participants are in line.
   - `allowed` count: how many are in the checkout window.
   - Inventory velocity: compare `allowed` and actual order completions.
2. Adjust `releaseRate` live if needed:
   - Too fast (checkout concurrency overwhelming, inventory depleting too quickly)
     → reduce rate.
   - Too slow (inventory not moving, `allowed` stays near 0) → increase rate.
3. **Pause** if oversold risk is detected:
   ```
   POST /queue/pause
   ```
   Pausing stops new admissions. Existing `ALLOWED` users can still complete
   checkout but no more will be admitted until resume.
4. Check admission expiry rate. If many users are cycling through (expire → requeue
   → release again), consider increasing `QUEUE_CHECKOUT_WINDOW` env var
   (requires restart) or reducing release rate so users have more time.

### If Redis Goes Down

1. Do not panic. Checkout still works (Postgres-only path).
2. Participant position display will degrade to "Position: --".
3. Admin stats will show 0 for waiting/allowed.
4. Restore Redis. Sorted sets can be rebuilt from `queue_tokens WHERE status = 'WAITING'` after the event if needed.
5. If Redis is down before the sale starts, consider delaying until it is restored
   -- position display is important for user trust.

### Post-Sale

1. Pause the queue (`POST /queue/pause`) or set release rate to 0.
2. Audit: review the audit log for `QUEUE_TOKEN_ISSUED`, `QUEUE_RELEASED`,
   `QUEUE_PAUSED`, `QUEUE_RATE_CHANGED`, and `QUEUE_ADMISSION_EXPIRED` events.
3. Compare actual ticket sales to admission count to identify expired or abandoned
   checkout sessions.
