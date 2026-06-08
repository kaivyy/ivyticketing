# Phase 10: Ballot System — Design Spec

> Extends the Phase 9–11 roadmap. See also: `docs/superpowers/specs/2026-06-09-registration-access-architecture.md`.
> EXTEND, NEVER REWRITE. Phases 1–9 baseline untouched.

---

## Overview

Phase 10 delivers three things:

1. **Registration Lifecycle Engine** — models time-windowed registration phases per category (PRIORITY→QUEUE→WAITLIST→CLOSED). Foundation for all future mode transitions.
2. **Waitlist Engine** — generic ordered-promotion service. Used by ballot waitlist now; reused by access engine (Phase 11) and queue overflow (future).
3. **Ballot Module** — full lottery registration: application window, deterministic auditable draw, winner payment window, waitlist promotion rounds.

**Out of scope (Phase 11):** Access pools (COMMUNITY/CORPORATE/VIP/SPONSOR/PARTNER), invitation codes, priority eligibility checker, corporate module.

**Key architectural constraint:** Ballot winners receive an `AccessGrant` from a `RESERVED` `AccessPool` created at draw time. This means the `access_pools` and `access_grants` tables are introduced in Phase 10 (RESERVED type only). Phase 11 adds the full pool type set on top.

---

## 1. Registration Lifecycle Engine

### Purpose

Single model for "what registration mode is active right now for this category." The RAE calls `LifecycleEngine.IsWindowOpen` before dispatching to any admitter. Without a lifecycle row, the RAE falls back to `ResolveForCheckout` (existing behavior — Phase 1–9 events unaffected).

### Domain Model

```
RegistrationLifecycle
├── id                    uuid PK
├── organization_id       uuid FK → organizations
├── event_id              uuid FK → events
├── category_id           uuid FK → event_categories
├── status                text  DRAFT | ACTIVE | PAUSED | COMPLETED | CANCELLED
├── current_phase_index   integer (0-based)
├── created_by            uuid FK → users
├── created_at            timestamptz
└── updated_at            timestamptz

LifecyclePhase
├── id                    uuid PK
├── lifecycle_id          uuid FK → registration_lifecycles ON DELETE CASCADE
├── phase_index           integer (ordered, 0-based)
├── registration_mode     text  (maps to registration.Mode constants)
├── label                 text  (e.g. "Priority Window", "General Sale")
├── opens_at              timestamptz nullable (null = immediately after previous phase completes)
├── closes_at             timestamptz nullable (null = manual close only)
├── capacity_override     integer nullable (override category.capacity for this phase)
├── auto_advance          boolean NOT NULL DEFAULT true
├── status                text  PENDING | ACTIVE | COMPLETED | SKIPPED
├── activated_at          timestamptz nullable
└── completed_at          timestamptz nullable
```

### State Machines

```
RegistrationLifecycle:

DRAFT ──activate──► ACTIVE ──all_phases_complete──► COMPLETED
                       │
                       ├──pause (admin)──► PAUSED ──resume──► ACTIVE
                       └──cancel──► CANCELLED (terminal)

LifecyclePhase:

PENDING ──(opens_at reached OR previous COMPLETED)──► ACTIVE
ACTIVE ──(closes_at reached AND auto_advance=true)──► COMPLETED ──► next PENDING activates
ACTIVE ──manual_complete──► COMPLETED
ACTIVE ──skip (organizer)──► SKIPPED ──► next PENDING activates
```

### LifecycleEngine Service Interface

```go
type LifecycleChecker interface {
    IsWindowOpen(ctx context.Context, categoryID uuid.UUID, mode registration.Mode) (bool, WindowClosedReason, error)
}

type WindowClosedReason string
const (
    ReasonWindowNotYetOpen  WindowClosedReason = "WINDOW_NOT_YET_OPEN"
    ReasonWindowExpired     WindowClosedReason = "WINDOW_EXPIRED"
    ReasonModeNotInLifecycle WindowClosedReason = "MODE_NOT_IN_LIFECYCLE"
    ReasonLifecyclePaused   WindowClosedReason = "LIFECYCLE_PAUSED"
)
```

`IsWindowOpen` returns true when there exists an ACTIVE LifecyclePhase for the category with matching `registration_mode`. If no lifecycle row exists for the category, returns true (fail-open → existing behavior).

### LifecycleAdvancer Background Job

- Runs every minute via existing cron pattern
- Query: `LifecyclePhase WHERE status=ACTIVE AND auto_advance=true AND closes_at < now()`
- For each match: `SELECT FOR UPDATE` on lifecycle row → complete phase → activate next PENDING phase (if any) → if none, mark lifecycle COMPLETED
- Idempotent: second worker sees phase already COMPLETED, no-ops

### Migration Candidates

**00031_create_registration_lifecycle**
- Tables: `registration_lifecycles`, `lifecycle_phases`
- Indexes: `(event_id, category_id)` on lifecycles; `(lifecycle_id, phase_index)` UNIQUE on phases; partial index `(status, closes_at)` on phases for advancer query
- Constraint: `UNIQUE (lifecycle_id, phase_index)`

**00032_create_lifecycle_phases** *(or combined with 00031)*

---

## 2. Waitlist Engine

### Purpose

Single promotion service used by ballot (source=BALLOT), future queue overflow, and Phase 11 quota release. Always issues an `AccessGrant` on promotion — participant uses the grant to unlock checkout.

### Domain Model

```
Waitlist
├── id                    uuid PK
├── organization_id       uuid FK
├── event_id              uuid FK
├── category_id           uuid FK
├── pool_id               uuid FK → access_pools (null until Phase 11 for non-ballot)
├── mode                  text  FIFO | RANDOMIZED | HYBRID
├── status                text  ACTIVE | PAUSED | CLOSED
├── max_promotion_batch   integer NOT NULL DEFAULT 10
├── promotion_window_hours integer NOT NULL DEFAULT 48
├── auto_promote          boolean NOT NULL DEFAULT true
├── seed                  text nullable (for RANDOMIZED — committed before first promotion)
└── created_at            timestamptz

WaitlistEntry
├── id                    uuid PK
├── waitlist_id           uuid FK → waitlists
├── participant_id        uuid FK → users
├── event_id              uuid FK
├── category_id           uuid FK
├── source                text  BALLOT | QUOTA_RELEASE | MANUAL
├── source_ref_id         uuid nullable (ballot_entry_id etc.)
├── status                text  WAITING | PROMOTED | EXPIRED | WITHDRAWN
├── rank                  bigint (FIFO=joined_at epoch; RANDOMIZED=seeded score; HYBRID=composite)
├── notified_at           timestamptz nullable
├── promoted_at           timestamptz nullable
├── access_grant_id       uuid nullable FK → access_grants
├── promotion_window_hours integer nullable (overrides waitlist default)
└── created_at            timestamptz
```

### State Machine

```
WaitlistEntry:

WAITING ──PromoteBatch──► PROMOTED ──grant CONSUMED (checkout)──► (closed)
   │                          └──grant EXPIRED (window)──► EXPIRED → slot released
   └──Withdraw──► WITHDRAWN (terminal)
   └──waitlist CLOSED──► EXPIRED (terminal)
```

### Promotion Algorithms

**FIFO:** `rank = extract(epoch from created_at) * 1000000` — lower rank promoted first.

**RANDOMIZED:** Seed committed to `waitlists.seed` before first promotion fires. `rank = hash(seed || participant_id)` deterministic per-entry score. Same as queue PRESALE pool scoring — reproducible and auditable.

**HYBRID:** FIFO rank for entries joined within a priority window (e.g. first 24h), RANDOMIZED rank for the rest. Priority window configurable per waitlist.

### WaitlistEngine Service Interface

```go
type WaitlistEngine interface {
    Join(ctx context.Context, waitlistID uuid.UUID, participantID uuid.UUID, source string, sourceRefID *uuid.UUID) (WaitlistEntry, error)
    PromoteBatch(ctx context.Context, waitlistID uuid.UUID) ([]WaitlistEntry, error)
    Expire(ctx context.Context, entryID uuid.UUID) error
    Withdraw(ctx context.Context, entryID uuid.UUID, participantID uuid.UUID) error
}
```

`PromoteBatch`:
1. `SELECT FOR UPDATE` on waitlist row
2. Fetch top `max_promotion_batch` WAITING entries ordered by rank ASC
3. For each: call `AccessPool.ReserveSlot(poolID)` — if ErrPoolExhausted, stop batch
4. On success: create `AccessGrant`, set entry status=PROMOTED, set `access_grant_id`, set `notified_at`
5. All writes in single transaction per entry (not batch-wide — partial success is fine)
6. Async notification triggered after transaction commits

### Migration Candidates

**00033_create_waitlists**
- Index: `(event_id, category_id, status)`

**00034_create_waitlist_entries**
- Index: `(waitlist_id, status, rank)` for promotion query
- Index: `(participant_id, status)` for participant view
- UNIQUE `(waitlist_id, participant_id)` WHERE status NOT IN ('WITHDRAWN','EXPIRED')

---

## 3. Ballot Module

### Purpose

Lottery registration for high-demand events. Participant applies during window → organizer runs draw → winners get payment window → unpaid winners lapse → waitlist gets promoted.

### Domain Model

```
BallotDraw
├── id                      uuid PK
├── organization_id         uuid FK
├── event_id                uuid FK
├── category_id             uuid FK
├── status                  text  PENDING | OPEN | CLOSED | DRAWN | ANNOUNCED
├── quota                   integer NOT NULL (number of winners)
├── waitlist_size           integer NOT NULL DEFAULT 0
├── payment_window_hours    integer NOT NULL DEFAULT 48
├── application_opens_at    timestamptz NOT NULL
├── application_closes_at   timestamptz NOT NULL
├── draw_at                 timestamptz nullable
├── announced_at            timestamptz nullable
├── seed                    text nullable (committed just before draw runs)
├── draw_nonce              uuid nullable (generated at run time, part of seed input)
├── winner_pool_id          uuid nullable FK → access_pools (RESERVED, created at draw time)
├── waitlist_id             uuid nullable FK → waitlists (created at draw time)
└── created_by              uuid FK → users

BallotEntry
├── id                      uuid PK
├── draw_id                 uuid FK → ballot_draws
├── organization_id         uuid FK
├── event_id                uuid FK
├── category_id             uuid FK
├── participant_id          uuid FK → users
├── status                  text  APPLIED | WINNER | WAITLISTED | NOT_SELECTED | LAPSED | CONVERTED | WITHDRAWN
├── applied_at              timestamptz NOT NULL DEFAULT now()
├── payment_deadline        timestamptz nullable (set when status→WINNER)
├── converted_at            timestamptz nullable (set when status→CONVERTED)
├── promoted_round          integer NOT NULL DEFAULT 0 (incremented on each waitlist promotion)
└── access_grant_id         uuid nullable FK → access_grants

BallotDrawResult
├── id                      uuid PK
├── draw_id                 uuid FK → ballot_draws
├── ballot_entry_id         uuid FK → ballot_entries
├── outcome                 text  WINNER | WAITLISTED | NOT_SELECTED
├── rank                    integer (deterministic position in shuffled list)
└── result_hash             text (sha256 of seed || rank || entry_id — integrity check)
```

### State Machines

```
BallotDraw:

PENDING ──OpenDraw──► OPEN
OPEN ──CloseDraw──► CLOSED
CLOSED ──RunDraw (seed committed first)──► DRAWN
DRAWN ──Announce──► ANNOUNCED (terminal — payment window now active)

BallotEntry:

(draw OPEN) ──Apply──► APPLIED
APPLIED ──draw DRAWN, outcome=WINNER──► WINNER ──pay within window──► CONVERTED (terminal)
APPLIED ──draw DRAWN, outcome=WAITLISTED──► WAITLISTED
APPLIED ──draw DRAWN, outcome=NOT_SELECTED──► NOT_SELECTED (terminal)
WINNER ──payment_deadline passes without pay──► LAPSED
LAPSED ──WaitlistEngine.PromoteBatch──► WINNER (new window, promoted_round++)
WAITLISTED ──WaitlistEngine.PromoteBatch──► WINNER (new window)
APPLIED ──Withdraw (while OPEN)──► WITHDRAWN (terminal)
```

### Draw Algorithm — Deterministic Fisher-Yates

```
1. Pre-draw seed generation (committed to DB BEFORE draw runs):
   seed_input = event_id || category_id || draw_at.UnixNano() || nonce_uuid
   seed = hex(sha256(seed_input))
   UPDATE ballot_draws SET seed=$seed, draw_nonce=$nonce WHERE id=$draw_id AND seed IS NULL

2. Draw execution (idempotent — checks results already exist):
   entries = SELECT * FROM ballot_entries WHERE draw_id=$id AND status='APPLIED' ORDER BY id ASC
   shuffled = FisherYates(entries, PRNGFromSeed(seed))
   winners    = shuffled[0 : quota]
   waitlisted = shuffled[quota : quota+waitlist_size]
   rest       = shuffled[quota+waitlist_size :]

3. Write results (one transaction):
   For each entry in shuffled:
     INSERT INTO ballot_draw_results (draw_id, ballot_entry_id, outcome, rank, result_hash)
     VALUES ($draw_id, $entry.id, $outcome, $rank, sha256(seed||rank||entry.id))

4. Update entry statuses (same transaction):
   UPDATE ballot_entries SET status=outcome WHERE draw_id=$id

5. Create RESERVED access pool for winners (same transaction):
   INSERT INTO access_pools (pool_type='RESERVED', total_slots=quota, event_id, category_id)
   UPDATE ballot_draws SET winner_pool_id=$pool_id

6. Create WaitlistEntry for each WAITLISTED entry:
   INSERT INTO waitlist_entries (source='BALLOT', source_ref_id=ballot_entry_id, ...)

7. UPDATE ballot_draws SET status='DRAWN'
```

**Idempotency:** Steps 3-7 are skipped if `ballot_draw_results` rows already exist for this draw_id.

**Reproducibility:** Given `seed` + original `ballot_entries` ordered by `id`, any third party can reproduce exact same shuffled order and verify every `result_hash`.

### Winner → Payment Flow

When draw is ANNOUNCED:
1. For each WINNER entry: issue `AccessGrant` from `winner_pool_id` pool with `expires_at = now() + payment_window_hours`
2. Set `ballot_entries.access_grant_id = grant.id`, `payment_deadline = grant.expires_at`
3. Trigger notification (async)

Participant clicks payment link → `POST /ballot/convert`:
- Verify entry.status=WINNER, grant.status=ACTIVE, grant.expires_at > now()
- Create order + inventory reservation (existing checkout flow)
- On order PAID: `ConsumeSlot(winner_pool_id, grant_id)`, entry.status=CONVERTED

### Lapse + Waitlist Promotion

`ExpireBallotWinners` job (every minute):
1. Find WINNER entries where `payment_deadline < now()`
2. Set entry.status=LAPSED
3. Set grant.status=EXPIRED, call `ReleaseSlot(winner_pool_id, grant_id)`
4. Call `WaitlistEngine.PromoteBatch(waitlist_id)` — promotes next WAITLISTED entries to WINNER

### BallotAdmitter Interface (for RAE)

```go
type BallotAdmitter interface {
    CheckBallotAdmission(ctx context.Context, participantID, categoryID uuid.UUID, admissionToken string) error
}
```

Verifies: entry exists, status=WINNER, access_grant active, grant not expired. `admissionToken` = grant.id string.

### Migration Candidates

**00035_create_ballot_draws**
- Index: `(event_id, category_id, status)`
- UNIQUE `(event_id, category_id)` WHERE status NOT IN ('ANNOUNCED') — one active draw per category

**00036_create_ballot_entries**
- UNIQUE `(draw_id, participant_id)`
- Index: `(draw_id, status)`, `(participant_id, status)`

**00037_create_ballot_draw_results**
- UNIQUE `(draw_id, ballot_entry_id)`
- Index: `(draw_id, outcome, rank)`

**00038_create_access_pools** *(RESERVED type only — Phase 11 adds remaining types)*
- Tables: `access_pools`
- Constraint: `reserved_slots + used_slots <= total_slots` (check constraint)
- Index: `(event_id, category_id, pool_type)`

**00039_seed_ballot_permissions**
- Permissions: `manage_ballot` (organizer), `apply_ballot` (participant)

---

## 4. Registration Access Engine — Updates

### Refactored gate.go

`NewGate` gains two new injected interfaces:

```go
func NewGate(svc *Service, queue QueueAdmitter, lifecycle LifecycleChecker, ballot BallotAdmitter) *Gate
```

`Admit()` updated:

```go
func (g *Gate) Admit(ctx, participantID, eventID, categoryID, admissionToken) error {
    mode, err := g.svc.ResolveForCheckout(ctx, eventID, categoryID)
    // ...

    // Lifecycle window check (if lifecycle exists for category)
    if g.lifecycle != nil {
        open, reason, err := g.lifecycle.IsWindowOpen(ctx, categoryID, mode)
        if err == nil && !open {
            return apperr.New(409, "REGISTRATION_WINDOW_CLOSED", string(reason))
        }
    }

    switch mode {
    case ModeNormal:      return nil
    case ModeClosed:      return ErrClosed
    case ModeWarQueue, ModeRandomizedQueue, ModeHybridQueue:
        return g.queue.CheckAdmission(ctx, participantID, eventID, admissionToken)
    case ModeBallot:
        return g.ballot.CheckBallotAdmission(ctx, participantID, categoryID, admissionToken)
    default:
        return ErrModeNotAvailable
    }
}
```

Phase 11 adds `accessGrantChecker` and `priorityChecker` to handle remaining modes.

---

## 5. API Design

### Participant Endpoints
| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/events/{eventId}/categories/{categoryId}/ballot/apply` | Submit application |
| GET | `/api/v1/events/{eventId}/categories/{categoryId}/ballot/my-entry` | Own entry status |
| DELETE | `/api/v1/events/{eventId}/categories/{categoryId}/ballot/my-entry` | Withdraw (OPEN only) |
| POST | `/api/v1/events/{eventId}/categories/{categoryId}/ballot/convert` | Winner converts to order |

### Organizer Endpoints
| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/org/{orgId}/events/{eventId}/categories/{categoryId}/ballot` | Create draw |
| PUT | `/api/v1/org/{orgId}/ballot/{drawId}` | Update config (PENDING/OPEN only) |
| POST | `/api/v1/org/{orgId}/ballot/{drawId}/open` | Open applications |
| POST | `/api/v1/org/{orgId}/ballot/{drawId}/close` | Close applications |
| POST | `/api/v1/org/{orgId}/ballot/{drawId}/run` | Execute draw |
| POST | `/api/v1/org/{orgId}/ballot/{drawId}/announce` | Announce results |
| GET | `/api/v1/org/{orgId}/ballot/{drawId}/results` | Paginated results |
| POST | `/api/v1/org/{orgId}/ballot/{drawId}/promote-waitlist` | Manual promotion trigger |
| GET | `/api/v1/org/{orgId}/ballot/{drawId}/export` | CSV export |

### Lifecycle Endpoints
| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/org/{orgId}/events/{eventId}/categories/{categoryId}/lifecycle` | Create lifecycle |
| GET | `/api/v1/org/{orgId}/events/{eventId}/categories/{categoryId}/lifecycle` | Get lifecycle + phases |
| PUT | `/api/v1/org/{orgId}/lifecycle/{lifecycleId}/phases/{phaseId}` | Update phase |
| POST | `/api/v1/org/{orgId}/lifecycle/{lifecycleId}/activate` | Activate lifecycle |
| POST | `/api/v1/org/{orgId}/lifecycle/{lifecycleId}/pause` | Pause (emergency) |
| POST | `/api/v1/org/{orgId}/lifecycle/{lifecycleId}/resume` | Resume |
| POST | `/api/v1/admin/lifecycle/{lifecycleId}/emergency-stop` | Platform admin emergency stop |

---

## 6. Error Codes

| Code | HTTP | Meaning |
|---|---|---|
| `BALLOT_CLOSED` | 409 | Application window not open |
| `BALLOT_ALREADY_APPLIED` | 409 | Participant already has entry for this draw |
| `BALLOT_NOT_WINNER` | 403 | Entry is not in WINNER status |
| `BALLOT_PAYMENT_WINDOW_EXPIRED` | 409 | Winner payment deadline passed |
| `BALLOT_DRAW_ALREADY_RUN` | 409 | Draw idempotency — already executed |
| `BALLOT_DRAW_NOT_ANNOUNCED` | 409 | Results not yet announced |
| `BALLOT_WITHDRAW_NOT_ALLOWED` | 409 | Can only withdraw while draw is OPEN |
| `REGISTRATION_WINDOW_CLOSED` | 409 | Lifecycle window not active for this mode |
| `LIFECYCLE_ALREADY_ACTIVE` | 409 | Only one ACTIVE lifecycle per category |

---

## 7. Abuse Guard Integration

- `POST .../ballot/apply` → guard category `CategoryBallotApply`
  - Rate: 5/IP/min, 2/user/min
  - Turnstile: optional (toggleable via `platform_settings.turnstile_enabled`)
  - Reputation bump (+2) on: apply after close, apply twice
- `POST .../ballot/run` → RequirePlatformAdmin or organizer role — no rate limit
- All abort paths audit-logged via existing `audit.Logger`

New category constant added to `services/api/internal/modules/abuse/model.go`:
```go
CategoryBallotApply = "ballot_apply"
```

---

## 8. Frontend Changes

- **Event page:** "Enter Ballot" CTA when draw is OPEN; application count indicator
- **My Applications page:** Status badge (APPLIED/WINNER/WAITLISTED/NOT_SELECTED/LAPSED/CONVERTED); winner payment countdown timer; "Pay Now" CTA linking to checkout
- **Waitlist position:** When WAITLISTED, show estimated position (rank / total_waitlisted)
- **Organizer draw page:** Draw status, quota, application count, run draw CTA with seed preview, results table with export, promote-waitlist CTA
- **Organizer lifecycle page:** Phase timeline visualization, auto-advance toggle, emergency stop button

---

## 9. Test Strategy

### Lifecycle Engine
- Unit: IsWindowOpen — ACTIVE phase → true; PENDING/COMPLETED → false; no lifecycle row → true (fail-open)
- Unit: Auto-advance — phase completes at closes_at, next PENDING activates
- Unit: Emergency stop — ACTIVE lifecycle → PAUSED → admits denied
- Concurrency: Two advancer workers simultaneously — exactly one advances phase (SELECT FOR UPDATE)
- Chaos: Cron down 30 min — phases catch up on next run, no phases double-advanced

### Waitlist Engine
- Unit: FIFO rank ordering — 100 entries promoted in join order
- Unit: RANDOMIZED — same seed → same promotion order (1000 runs)
- Unit: PromoteBatch stops at pool exhaustion (ErrPoolExhausted)
- Concurrency: Two PromoteBatch calls simultaneously — zero double-promotions
- Integration: Ballot lapse → PromoteBatch → new AccessGrant → checkout

### Ballot Draw Algorithm
- Unit: Deterministic — same seed → same shuffled output (1000 runs)
- Unit: No duplicates — 100k entries, quota 10k — exactly 10k winners, zero repeats
- Unit: Quota exactness — winners = quota, waitlisted = waitlist_size
- Unit: result_hash verification — each hash reproducible from (seed, rank, entry_id)
- Unit: Idempotency — RunDraw called twice → second call returns existing results, no changes

### Ballot Service
- Integration: Full apply → run → announce → convert cycle
- Integration: WINNER → payment deadline expires → LAPSED → waitlist promoted → new WINNER → convert
- Integration: Withdraw while OPEN accepted; withdraw after CLOSED rejected (409)
- Integration: Attempt convert with wrong/expired token → 403
- Concurrency: 10k simultaneous apply requests → no duplicate entries, use_count correct
- Concurrency: Simultaneous convert + expiry worker → only one wins (entry status idempotent)

### RAE Updates
- Unit: BALLOT case dispatches to BallotAdmitter
- Unit: Lifecycle window closed → deny before reaching BallotAdmitter
- Integration: Full RAE chain for BALLOT mode

### Load
- 50k applications submitted in 60s → zero duplicate entries, p99 < 2s
- Draw of 100k entries, quota 10k → draw completes < 5s
- 5k simultaneous winner conversions → zero inventory oversell, p99 < 3s

---

## 10. Release Strategy

### Part 1 — Foundation (no UX change)
Migrations 00031–00038, sqlc regeneration. Lifecycle Engine + Waitlist Engine skeleton (service + repo, no handlers). AccessPool model (RESERVED type). All gated behind ballot_enabled=false. Existing tests unchanged.

### Part 2 — Draw engine + organizer APIs
BallotDraw/BallotEntry service (Apply, RunDraw, AnnounceResults). Organizer handler + routes. Draw algorithm + seed strategy. `ExpireBallotWinners` + `WaitlistPromoter` background jobs. Organizer can create and run draws in staging — no participant-facing features yet.

### Part 3 — Participant flow + gate integration
Participant endpoints (apply/withdraw/convert). RAE refactored with LifecycleChecker + BallotAdmitter. `ballot_enabled=true` for pilot events. Frontend ballot UI (apply CTA, my-entry page, winner countdown).

### Part 4 — Reporting + docs
Export endpoint (CSV). Audit log viewer (seed + result_hash verification). CHANGELOG. k6 load test scripts.

---

## 11. Go / No-Go Criteria

- [ ] Migrations 00031–00038 roundtrip clean
- [ ] Lifecycle: IsWindowOpen unit tests pass
- [ ] Draw: 1000-run deterministic unit test passes, zero duplicates
- [ ] Draw: Idempotency test passes (second RunDraw no-ops)
- [ ] Integration: apply→draw→announce→convert full cycle
- [ ] Concurrency: 10k apply race test
- [ ] Payment window expiry worker tested
- [ ] Phase 8 queue tests still green
- [ ] `go test ./... -race` green
