# IvyTicketing Phase 9–11 Implementation Roadmap

> **This is a planning document only. No code. No migrations. No API implementation.**
> **Wait for approval before executing any phase.**

---

## CURRENT SYSTEM AUDIT

### What exists (Phase 1–9 baseline)

**Registration modes** — all defined in `registration/model.go`, all valid/parseable:
- `NORMAL`, `WAR_QUEUE`, `RANDOMIZED_QUEUE`, `HYBRID_QUEUE` — fully implemented (Phases 5–8)
- `BALLOT`, `INVITATION_ONLY`, `PRIORITY_ACCESS`, `WAITLIST_ONLY`, `CLOSED` — mode constants exist, gate returns `ErrModeNotAvailable`

**Registration gate** (`registration/gate.go`) — `Admit()` switch already has a `default` case for unimplemented modes returning `ErrModeNotAvailable`. Phase 10–11 extends this switch.

**Queue token states**: `WAITING → ALLOWED → COMPLETED / EXPIRED / BLOCKED`

**Order states**: `DRAFT → PENDING_PAYMENT → PAID / EXPIRED / CANCELLED / REFUNDED`

**Inventory**: `event_categories.capacity` + `inventory_reservations` table. Capacity is per-category.

**Abuse (Phase 9)**: fully implemented — blocklist, rate limit, reputation, guard chain on queue-join/auth/checkout, `RequirePlatformAdmin`, super-admin endpoints, settings runtime toggle.

**Audit**: `audit.Entry{OrganizationID, ActorUserID, Action, TargetType, TargetID, Metadata}` — available everywhere.

**DB**: 30 migrations complete. Next available: `00031`.

---

## PHASE 8 DEPENDENCY REVIEW

Phase 10 and 11 build on top of these Phase 8 primitives:

| Primitive | Used by Phase 10 | Used by Phase 11 |
|---|---|---|
| `registration/gate.go Admit()` | Ballot admission replaces `ErrModeNotAvailable` | Access-code admission replaces `ErrModeNotAvailable` |
| `event_registration_settings.ballot_enabled` | Gates ballot creation | — |
| `event_registration_settings.priority_enabled` | — | Gates priority window |
| `event_categories.capacity` | Ballot quota is a subset | Access quota is a subset |
| `queue_tokens` | Ballot winners may join queue for payment | Priority users skip/bypass queue |
| `inventory_reservations` | Ballot holds inventory for winners | Access codes hold reserved inventory |
| `abuse guard` | Applied to ballot submit, invitation redemption | Applied to access code redemption |

---

## PHASE 9 PLAN

Phase 9 is **complete**. See committed plans:
- `docs/superpowers/plans/2026-06-08-phase9-antibot-abuse.md`
- `docs/superpowers/plans/2026-06-08-phase9-part1-foundation.md`
- `docs/superpowers/plans/2026-06-08-phase9-part2-abuse-module.md`
- `docs/superpowers/plans/2026-06-08-phase9-part3-wiring-frontend-docs.md`

DoD: 14/14 ✅

---

## PHASE 10 PLAN — BALLOT REGISTRATION SYSTEM

### Goal

Support lottery-based registration for high-demand events (marathons, major races). Participants apply during a ballot window; a deterministic, auditable draw selects winners; winners get a payment window; unpaid winners release back to a waitlist; waitlist gets secondary rounds.

### Architecture

The ballot is a **separate lifecycle** from the queue: it has its own state machine, its own tables, and its own draw engine. It integrates with the existing order/payment flow at the payment window stage — a ballot winner gets an `admission_token` that unlocks checkout, identical in shape to the queue admission token. The draw is seeded deterministically (seed stored before draw, results reproducible) and logged in full for audit. No queue token is created for ballot entries — ballot has its own waiting/winner/waitlist states.

### Tech Stack

Go 1.25, Chi v5, pgx v5, sqlc, goose, existing audit/apperr/authctx patterns. No new external dependencies.

---

### Domain Model

#### Ballot Entry state machine

```
OPEN ──apply──► APPLIED
APPLIED ──draw──► WINNER | WAITLISTED | NOT_SELECTED
WINNER ──pay within window──► CONVERTED (order created)
WINNER ──window expires──► LAPSED
LAPSED ──waitlist promotion──► WINNER (new window)
WAITLISTED ──promotion──► WINNER
NOT_SELECTED ──(terminal)
CONVERTED ──(terminal, order lifecycle takes over)
```

#### Ballot Draw state machine

```
PENDING ──open_applications──► OPEN
OPEN ──close_applications──► CLOSED
CLOSED ──run_draw──► DRAWN
DRAWN ──announce──► ANNOUNCED
ANNOUNCED ──payment_window_expires──► PAYMENT_COMPLETE (or triggers waitlist round)
```

---

### File Structure

```
database/
  migrations/
    00031_create_ballot_entries.sql
    00032_create_ballot_draws.sql
    00033_create_ballot_draw_results.sql
    00034_seed_ballot_permissions.sql
  queries/
    ballot.sql

services/api/internal/
  modules/ballot/
    model.go          — state constants, BallotEntry, BallotDraw domain types
    errors.go         — ErrBallotClosed, ErrAlreadyApplied, ErrNotWinner, etc.
    dto.go            — ApplyRequest, BallotEntryDTO, DrawResultDTO, WinnerListDTO
    repository.go     — Repository interface + sqlcRepo
    draw.go           — deterministic draw algorithm (Fisher-Yates, seeded)
    seed.go           — seed generation strategy (timestamp + event_id + nonce)
    service.go        — Apply, RunDraw, AnnounceResults, ConvertWinner, PromoteWaitlist
    handler.go        — HTTP handlers (participant + organizer + admin)
    routes.go         — RegisterParticipantRoutes, RegisterOrganizerRoutes
    tests/
      draw_test.go    — deterministic draw reproducibility, no duplicates
      service_test.go — state machine transitions
  modules/registration/
    gate.go           — MODIFY: add BALLOT case calling ballot.Admitter
```

---

### Migration Candidates

#### 00031_create_ballot_entries

**Purpose:** Stores each participant's application for a ballot round.
**Owner:** ballot module
**Key columns:** id, organization_id, event_id, category_id, participant_id, status, applied_at, draw_id (FK nullable until draw), payment_deadline, converted_at, promoted_round
**Relationships:** → users, events, event_categories, ballot_draws
**Index considerations:** (event_id, participant_id) UNIQUE; (event_id, status) for draw queries; (draw_id, status) for winner queries
**Expected growth:** up to 500k rows per major event; partitioning by event_id when >10M total
**Retention:** keep indefinitely (audit)

#### 00032_create_ballot_draws

**Purpose:** Represents one draw round for an event+category. Stores draw configuration, timing, seed.
**Owner:** ballot module
**Key columns:** id, organization_id, event_id, category_id, status, quota (integer), payment_window_hours, application_opens_at, application_closes_at, draw_at, announced_at, seed (text, stored before draw runs), draw_nonce (uuid, generated at run time), created_by
**Relationships:** → events, event_categories, users (created_by)
**Index considerations:** (event_id, category_id, status); UNIQUE (event_id, category_id) for active draw
**Expected growth:** low (1–10 draws per event); no partition needed
**Retention:** indefinitely

#### 00033_create_ballot_draw_results

**Purpose:** Immutable ordered log of every winner/waitlisted/not_selected entry from a draw. Enables full reproducibility.
**Owner:** ballot module
**Key columns:** id, draw_id, ballot_entry_id, outcome (WINNER/WAITLISTED/NOT_SELECTED), rank (integer, deterministic order), result_hash (sha256 of seed+rank+entry_id — integrity check)
**Relationships:** → ballot_draws, ballot_entries
**Index considerations:** (draw_id, outcome, rank); UNIQUE (draw_id, ballot_entry_id)
**Expected growth:** mirrors ballot_entries; partition with draws
**Retention:** indefinitely (legal/audit)

#### 00034_seed_ballot_permissions

**Purpose:** RBAC permissions for ballot management (organizer: manage_ballot; participant: apply_ballot).
**Owner:** auth/rbac
**Relationships:** → permissions catalog
**Expected growth:** static

---

### API Candidates

#### Participant
- `POST /api/v1/events/{eventId}/categories/{categoryId}/ballot/apply` — submit ballot application
- `GET /api/v1/events/{eventId}/categories/{categoryId}/ballot/my-entry` — check own entry status
- `DELETE /api/v1/events/{eventId}/categories/{categoryId}/ballot/my-entry` — withdraw application (only while OPEN)
- `POST /api/v1/events/{eventId}/categories/{categoryId}/ballot/convert` — winner converts to order (creates order + reservation, requires valid admission token from ballot)

#### Organizer
- `POST /api/v1/org/{orgId}/events/{eventId}/categories/{categoryId}/ballot` — create/configure draw
- `PUT /api/v1/org/{orgId}/ballot/{drawId}` — update draw config (only while PENDING/OPEN)
- `POST /api/v1/org/{orgId}/ballot/{drawId}/open` — open applications
- `POST /api/v1/org/{orgId}/ballot/{drawId}/close` — close applications
- `POST /api/v1/org/{orgId}/ballot/{drawId}/run` — execute draw (idempotent, seed locked before run)
- `POST /api/v1/org/{orgId}/ballot/{drawId}/announce` — announce results (triggers notifications)
- `GET /api/v1/org/{orgId}/ballot/{drawId}/results` — paginated winner/waitlist list
- `POST /api/v1/org/{orgId}/ballot/{drawId}/promote-waitlist` — manually promote next waitlisted batch
- `GET /api/v1/org/{orgId}/ballot/{drawId}/export` — CSV export of all results

#### System/Worker
- Background job: `ExpireBallotWinners` — cron runs every minute, moves WINNER→LAPSED past payment_deadline, triggers waitlist promotion
- Background job: `AutoPromoteWaitlist` — after each expiry batch, promotes N next waitlisted entries to WINNER with new payment window

---

### Draw Algorithm

**Deterministic Fisher-Yates shuffle:**
1. Before draw: generate `seed = sha256(event_id + category_id + draw_at.UnixNano() + nonce_uuid)`. Store seed in `ballot_draws.seed`. Seed is public — organizer can share it for independent verification.
2. At draw time: load all `APPLIED` entries ordered by `id` (stable, deterministic). Shuffle using seed as PRNG source (crypto/rand seeded with seed hash → deterministic).
3. First `quota` entries → WINNER. Next `waitlist_size` entries → WAITLISTED. Remainder → NOT_SELECTED.
4. Write `ballot_draw_results` rows with rank and `result_hash = sha256(seed + rank + entry_id)`.
5. Draw is idempotent: if results already exist, return existing (no re-draw).

**Reproducibility guarantee:** Given seed + original entry list, any third party can reproduce exact same outcome. Seed is committed to DB before draw executes.

---

### Registration Gate Integration

`registration/gate.go Admit()`:
```
case ModeBallot:
    return g.ballot.CheckBallotAdmission(ctx, participantID, categoryID, admissionToken)
```

`ballot.Admitter.CheckBallotAdmission` verifies: entry exists, status=WINNER, payment_deadline not passed, admission_token matches entry.

---

### Abuse Guard Integration

- `POST .../ballot/apply` → guarded with `CategoryBallotApply` (rate: 5/IP/min, 2/user/min; Turnstile optional)
- `POST .../ballot/run` → RequirePlatformAdmin or organizer role; no rate limit needed
- Reputation bump on repeated failed apply attempts (wrong window, etc.)

---

### Test Strategy

- **Unit:** draw algorithm reproducibility (same seed → same output 1000 runs), no duplicates, quota exactness, state machine transitions
- **Integration:** full apply→draw→convert flow, payment window expiry→lapse→promote, withdraw while open, attempt apply after close (rejected)
- **Concurrency:** 1000 simultaneous apply requests → no duplicate winner, no quota overflow
- **Chaos:** draw interrupted mid-write (idempotency), DB unavailable during announce (retry)
- **Load:** 50k applications, draw of 5000 winners — draw must complete <5s

---

## PHASE 11 PLAN — ACCESS ENGINE

### Goal

Build controlled registration access: invitation codes, priority windows, community quotas, corporate bulk registration, reserved inventory. All modes share a unified "access grant" concept that unlocks checkout for a specific participant+category combination.

### Architecture

A single `access` module owns all controlled-access mechanics. Each access type (invitation code, priority grant, community quota, corporate slot) produces an **AccessGrant** — a short-lived token that the registration gate consumes identically to a queue admission token or ballot admission token. This keeps `orders/checkout` untouched. Quotas are tracked in a dedicated `access_quotas` table that can represent community pools, corporate allocations, or reserved blocks — all the same shape.

### Tech Stack

Go 1.25, Chi v5, pgx v5, sqlc, goose. No new external dependencies.

---

### Domain Model

#### AccessCode types
- `INVITATION` — single-use or multi-use, category-scoped, generates an AccessGrant on redemption
- `PRIORITY` — time-windowed, group-scoped (e.g. "returning runners"), auto-granted to eligible users
- `COMMUNITY` — pool-backed, redeemed by community members up to quota
- `CORPORATE` — org-backed, bulk-issued to corporate accounts, may require approval
- `COUPON` — discount + access unlock, may have public or restricted distribution
- `PARTNER` — partner org scoped, like corporate but cross-organization
- `SPONSOR` — event sponsor access, high-priority window
- `VIP` / `ELITE` — special category access, manually assigned

#### AccessGrant state machine
```
PENDING_REDEMPTION ──redeem──► ACTIVE
ACTIVE ──checkout_complete──► CONSUMED
ACTIVE ──expires──► EXPIRED
```

#### AccessQuota
```
quota: total_slots
reserved: slots currently held by ACTIVE grants
used: CONSUMED grants
available = total_slots - reserved - used
```

---

### File Structure

```
database/
  migrations/
    00035_create_access_codes.sql
    00036_create_access_grants.sql
    00037_create_access_quotas.sql
    00038_create_corporate_accounts.sql
    00039_create_corporate_members.sql
    00040_seed_access_permissions.sql
  queries/
    access.sql

services/api/internal/
  modules/access/
    model.go          — AccessCode, AccessGrant, AccessQuota, CorporateAccount constants/types
    errors.go         — ErrCodeNotFound, ErrCodeExhausted, ErrCodeExpired, ErrNotEligible, etc.
    dto.go            — RedeemRequest, AccessGrantDTO, QuotaDTO, CorporateSlotDTO
    repository.go     — Repository interface + sqlcRepo
    quota.go          — QuotaManager: reserve, release, consume (atomic, DB-level locking)
    eligibility.go    — EligibilityChecker: checks user eligibility for priority/community codes
    service.go        — RedeemCode, CheckGrant, ConsumeGrant, CreateCode, BulkIssue, ReleaseExpired
    handler.go        — HTTP handlers
    routes.go         — RegisterParticipantRoutes, RegisterOrganizerRoutes, RegisterAdminRoutes
    corporate.go      — CorporateService: account management, bulk upload, invoice billing
    tests/
      quota_test.go   — concurrent reserve/release atomicity
      eligibility_test.go
      service_test.go
  modules/registration/
    gate.go           — MODIFY: add INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY cases
```

---

### Migration Candidates

#### 00035_create_access_codes

**Purpose:** Stores access codes (invitation, priority, community, corporate, coupon, partner, VIP, elite).
**Owner:** access module
**Key columns:** id, organization_id, event_id, category_id (nullable — some codes are event-wide), code_type (enum), code_value (hashed, not plain text), is_single_use, max_uses, use_count, valid_from, valid_until, quota_id (FK nullable), eligibility_rule (jsonb — e.g. `{"returning_runner": true, "min_completions": 1}`), created_by, created_at, metadata (jsonb)
**Relationships:** → organizations, events, event_categories (nullable), access_quotas (nullable), users
**Index considerations:** UNIQUE (event_id, code_value_hash); (event_id, code_type, valid_until); partial index on active codes
**Expected growth:** thousands per event; no partition needed until >1M total
**Retention:** indefinitely

#### 00036_create_access_grants

**Purpose:** Short-lived grant issued to a participant when they redeem a code. Consumed at checkout.
**Owner:** access module
**Key columns:** id, access_code_id, participant_id, event_id, category_id, status (PENDING_REDEMPTION/ACTIVE/CONSUMED/EXPIRED), granted_at, expires_at, consumed_at, order_id (FK nullable, set on consume)
**Relationships:** → access_codes, users, events, event_categories, orders
**Index considerations:** UNIQUE (access_code_id, participant_id) for single-use enforcement; (participant_id, event_id, status); (expires_at) for expiry worker
**Expected growth:** mirrors access_codes × redemptions; partition by created_at monthly at scale
**Retention:** indefinitely

#### 00037_create_access_quotas

**Purpose:** Tracks a named pool of slots — community quota, corporate allocation, reserved block, sponsor block.
**Owner:** access module
**Key columns:** id, organization_id, event_id, category_id, quota_type (COMMUNITY/CORPORATE/RESERVED/SPONSOR/VIP), name, total_slots, reserved_slots, used_slots, released_slots, owner_id (nullable — corporate account), expires_at, created_at
**Relationships:** → organizations, events, event_categories, corporate_accounts
**Index considerations:** (event_id, category_id, quota_type); (owner_id) for corporate queries
**Expected growth:** low (10–100 per event); no partition needed
**Retention:** indefinitely

#### 00038_create_corporate_accounts

**Purpose:** Corporate entity that owns one or more access quotas and can bulk-upload members.
**Owner:** access module
**Key columns:** id, organization_id, name, billing_email, invoice_required, approved_at, created_by, created_at
**Relationships:** → organizations, users
**Expected growth:** low
**Retention:** indefinitely

#### 00039_create_corporate_members

**Purpose:** Maps users to a corporate account, enabling bulk registration and invoice billing.
**Owner:** access module
**Key columns:** id, corporate_account_id, user_id (nullable — may be pre-loaded before user creates account), email, status (PENDING/ACTIVE/REGISTERED), registered_at, access_grant_id (FK nullable)
**Relationships:** → corporate_accounts, users (nullable), access_grants
**Index considerations:** UNIQUE (corporate_account_id, email); (corporate_account_id, status)
**Expected growth:** up to 10k per corporate account
**Retention:** indefinitely

#### 00040_seed_access_permissions

**Purpose:** RBAC permissions: manage_access_codes, redeem_access_code, manage_corporate.
**Owner:** auth/rbac
**Retention:** static

---

### API Candidates

#### Participant
- `POST /api/v1/events/{eventId}/access/redeem` — redeem a code (body: `{code, categoryId}`), returns AccessGrantDTO with token
- `GET /api/v1/events/{eventId}/access/my-grants` — list own active grants for this event
- `GET /api/v1/events/{eventId}/access/priority-window` — check if participant qualifies for priority window (no code needed for auto-priority)

#### Organizer
- `POST /api/v1/org/{orgId}/events/{eventId}/access/codes` — create access code
- `GET /api/v1/org/{orgId}/events/{eventId}/access/codes` — list codes with use_count
- `DELETE /api/v1/org/{orgId}/access/codes/{codeId}` — revoke code
- `POST /api/v1/org/{orgId}/access/codes/bulk` — bulk-create invitation codes (returns CSV)
- `GET /api/v1/org/{orgId}/events/{eventId}/access/quotas` — list quotas with utilization
- `PUT /api/v1/org/{orgId}/access/quotas/{quotaId}` — adjust quota size
- `POST /api/v1/org/{orgId}/access/corporate` — create corporate account
- `POST /api/v1/org/{orgId}/access/corporate/{accountId}/members` — bulk upload members (CSV)
- `GET /api/v1/org/{orgId}/access/corporate/{accountId}/members` — list members + status
- `POST /api/v1/org/{orgId}/access/corporate/{accountId}/approve` — approve corporate account
- `GET /api/v1/org/{orgId}/access/corporate/{accountId}/invoice` — generate invoice PDF/data

#### Admin
- `GET /api/v1/admin/access/codes` — list codes across all events (paginated)
- `POST /api/v1/admin/access/quotas/{quotaId}/adjust` — emergency quota adjustment

#### System/Worker
- Background job: `ExpireAccessGrants` — cron, ACTIVE grants past expires_at → EXPIRED, releases quota reservation
- Background job: `ReleaseCorporateSlots` — if member didn't register by deadline, release slot back to quota

---

### Quota Atomicity

All `reserve`, `release`, `consume` operations on `access_quotas` use a single SQL statement with `WHERE reserved_slots + used_slots < total_slots` row-level locking (or `SELECT FOR UPDATE` + check). No application-level optimistic retry needed — DB-level constraint. This prevents overselling under concurrent redemption.

---

### Registration Gate Integration

`registration/gate.go Admit()`:
```
case ModeInvitationOnly:
    return g.access.CheckAccessGrant(ctx, participantID, categoryID, admissionToken)
case ModePriorityAccess:
    // check priority window open + grant OR fall through to normal if window closed
    return g.access.CheckPriorityAdmission(ctx, participantID, categoryID, admissionToken)
case ModeWaitlistOnly:
    // waitlist is just INVITATION_ONLY with auto-issued codes when slots open
    return g.access.CheckAccessGrant(ctx, participantID, categoryID, admissionToken)
```

---

### Abuse Guard Integration

- `POST .../access/redeem` → guarded with `CategoryAccessRedeem` (rate: 10/IP/min, 5/user/min; Turnstile on high-demand events)
- Code value is hashed (sha256) at rest — brute-force enumeration is expensive
- Abuse bump on failed redemptions (wrong code, wrong category)
- `RequirePlatformAdmin` on quota adjustment endpoints

---

### Test Strategy

- **Unit:** quota atomicity (reserve+consume never exceeds total_slots), eligibility rules, code hash/verify, grant expiry
- **Integration:** full redeem→checkout flow, corporate bulk upload→member registration, priority window open/close, waitlist slot release→re-issue
- **Concurrency:** 500 simultaneous redemptions of a quota=100 code → exactly 100 grants issued, rest rejected
- **Security:** code brute-force attempt → rate limited + reputation bump; single-use code reuse → rejected; expired grant → rejected at checkout
- **Load:** 10k simultaneous redemptions at event launch

---

## CROSS-PHASE DEPENDENCIES

### How Phase 9 protects Phase 10 (Ballot)

| Threat | Phase 9 mechanism |
|---|---|
| Ballot spam (mass applications) | Rate limit on `/ballot/apply` (category `ballot_apply`), Turnstile |
| Multiple accounts per person | IP reputation bump on repeated creates, fingerprint |
| Bot-driven applications | Turnstile + reputation score |
| Draw result manipulation | Draw runs server-side, seed committed before run — guard doesn't touch draw logic |

### How Phase 9 protects Phase 11 (Access Engine)

| Threat | Phase 9 mechanism |
|---|---|
| Code brute-force enumeration | Rate limit on `/access/redeem` (5/user/min, 10/IP/min) + reputation |
| Bulk redemption via bots | Turnstile on high-demand redemption endpoints |
| Corporate member spam | Rate limit on corporate bulk upload |
| Quota exhaustion attack | Rate limit prevents rapid parallel redemptions from single IP |

### Ballot ↔ Queue

- Ballot and Queue are **mutually exclusive** per category — a category cannot have both `BALLOT` and `WAR_QUEUE` mode simultaneously (enforced in registration settings validation)
- A ballot winner's `ConvertToOrder` flow uses the same `inventory_reservations` path as queue-admitted orders — no special inventory path
- Waitlist after ballot lapse does NOT re-enter the queue — waitlist is ballot-internal state

### Priority ↔ Queue

- `PRIORITY_ACCESS` mode: priority window opens before general registration; once priority window closes, mode may switch to `NORMAL` or `WAR_QUEUE` for remaining slots
- Priority users bypass queue entirely — their access grant is accepted at checkout directly
- Queue `PRESALE` pool is a related concept but separate mechanism — Priority Access is pre-queue

### Invitation ↔ Queue

- `INVITATION_ONLY` categories never use the queue — redemption goes directly to checkout
- An event can mix modes per category: category A = `WAR_QUEUE`, category B = `INVITATION_ONLY`

### Community/Corporate ↔ Inventory

- `access_quotas` are a **sub-allocation** of `event_categories.capacity` — organizer must set `total_slots ≤ remaining_capacity`
- When a grant is consumed, a normal `inventory_reservation` is created — inventory module is unaware of access grants
- Quota release (grant expired) → reservation released → slot available again for same-quota new grants

### Waitlist ↔ Ballot

- Ballot waitlist is ballot-internal (WAITLISTED status in `ballot_entries`)
- General-purpose waitlist mode (`WAITLIST_ONLY`) uses access engine: when a slot opens (cancellation/refund), a job issues access codes to the next N waitlisted users

### Waitlist ↔ Queue

- Queue tokens with `EXPIRED` status represent users who had a slot but didn't complete checkout — their inventory reservation is released
- `WAITLIST_ONLY` mode does not use queue tokens — it uses access grants issued by the access engine when slots open

### Reserved Quota ↔ Inventory

- `access_quotas.total_slots` should be ≤ `event_categories.capacity - already_sold`
- No automatic enforcement — organizer responsibility; system warns if quota creation would exceed available capacity
- Inventory module remains unaware of quota structure; only `inventory_reservations` is the shared contract

---

## RISK MATRIX

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Draw algorithm bug produces duplicates | Low | Critical | Fisher-Yates with extensive unit tests (1000 runs), integration test, independent audit log |
| Quota oversell under concurrency | Medium | High | DB-level atomic reserve (`SELECT FOR UPDATE` + constraint) |
| Payment window expires before user can pay | Medium | Medium | Configurable window (minimum 24h recommended), advance email notification |
| Code brute-force during high-demand launch | High | High | Phase 9 rate limit + hashed codes + Turnstile |
| Corporate bulk upload corrupts quota | Low | High | Transactional bulk insert, quota check before commit |
| Ballot seed manipulation by insider | Low | Critical | Seed generated from deterministic inputs + random nonce stored before draw; public + auditable |
| Priority window misconfig (opens too early/late) | Medium | Medium | Admin preview of window schedule; organizer confirmation step |
| Waitlist promotion spam (user creates multiple accounts) | Medium | Medium | Dedup on email at waitlist entry; Phase 9 reputation on re-registration |
| Access grant consumed but order fails | Low | High | Grant status stays ACTIVE until order reaches PAID; rollback on order failure |

---

## MIGRATION CANDIDATES SUMMARY

| Migration | Purpose | Phase |
|---|---|---|
| 00031_create_ballot_entries | Ballot application records | 10 |
| 00032_create_ballot_draws | Draw configuration + seed + status | 10 |
| 00033_create_ballot_draw_results | Immutable ordered draw outcome log | 10 |
| 00034_seed_ballot_permissions | RBAC for ballot management | 10 |
| 00035_create_access_codes | Invitation/priority/community/corporate codes | 11 |
| 00036_create_access_grants | Short-lived redemption tokens | 11 |
| 00037_create_access_quotas | Named slot pools (community/corporate/reserved) | 11 |
| 00038_create_corporate_accounts | Corporate entity records | 11 |
| 00039_create_corporate_members | Corporate member → user mapping | 11 |
| 00040_seed_access_permissions | RBAC for access management | 11 |

---

## API CANDIDATES SUMMARY

### Participant APIs
| Endpoint | Phase |
|---|---|
| POST /events/{id}/categories/{id}/ballot/apply | 10 |
| GET /events/{id}/categories/{id}/ballot/my-entry | 10 |
| DELETE /events/{id}/categories/{id}/ballot/my-entry | 10 |
| POST /events/{id}/categories/{id}/ballot/convert | 10 |
| POST /events/{id}/access/redeem | 11 |
| GET /events/{id}/access/my-grants | 11 |
| GET /events/{id}/access/priority-window | 11 |

### Organizer APIs
| Endpoint | Phase |
|---|---|
| POST /org/{id}/events/{id}/categories/{id}/ballot | 10 |
| PUT/POST /org/{id}/ballot/{drawId}/open|close|run|announce | 10 |
| GET /org/{id}/ballot/{drawId}/results | 10 |
| POST /org/{id}/ballot/{drawId}/promote-waitlist | 10 |
| GET /org/{id}/ballot/{drawId}/export | 10 |
| POST/GET /org/{id}/events/{id}/access/codes | 11 |
| POST /org/{id}/access/codes/bulk | 11 |
| GET/PUT /org/{id}/events/{id}/access/quotas | 11 |
| POST/GET /org/{id}/access/corporate | 11 |
| POST /org/{id}/access/corporate/{id}/members | 11 |
| POST /org/{id}/access/corporate/{id}/approve | 11 |
| GET /org/{id}/access/corporate/{id}/invoice | 11 |

### Admin APIs
| Endpoint | Phase |
|---|---|
| GET /admin/access/codes | 11 |
| POST /admin/access/quotas/{id}/adjust | 11 |

---

## DASHBOARD CHANGES

### Phase 10 — Ballot
- **Organizer:** Ballot management tab on event/category page — create draw, configure quota + payment window, monitor application count, trigger draw, view/export results, trigger waitlist promotion
- **Participant:** "My Applications" section showing ballot status (APPLIED → waiting → WINNER/WAITLISTED/NOT_SELECTED), payment countdown for winners, convert-to-order CTA
- **Admin:** Draw audit log viewer — seed, entry list snapshot, result hash verification

### Phase 11 — Access Engine
- **Organizer:** Access Codes tab — create/revoke codes, view use_count vs quota, bulk-generate invitation CSV, quota utilization chart
- **Organizer:** Corporate Accounts tab — create account, upload members CSV, approve account, generate invoice
- **Participant:** "My Access" — list of active grants, redemption history, priority window countdown
- **Admin:** Global quota health dashboard — utilization across events, oversubscription warnings

---

## PARTICIPANT EXPERIENCE

### Ballot flow
1. Participant opens event page during ballot window → sees "Enter Ballot" CTA
2. Applies (Turnstile challenge on high-demand events) → confirmation screen + "Applications close on [date]"
3. Draw date arrives → email notification of result
4. Winner: email with payment link, countdown timer. Clicks → pre-filled checkout with payment deadline
5. Lapsed winner / waitlisted: email "Unfortunately you were not selected this round. You are on the waitlist." If promoted: new email with payment link

### Access code flow
1. Participant receives email/physical letter with code
2. Opens event page → "I have an access code" → enters code
3. If valid: redirected to checkout for the unlocked category
4. If invalid/exhausted: clear error message + suggestion to contact organizer

### Priority window flow
1. Eligible participants (e.g. returning runners) get email "Your priority registration window opens [date/time]"
2. At window open: they log in → category shows "Priority Access Active" → checkout directly without queue
3. After window closes: remaining slots go to general registration mode

---

## TEST STRATEGY

### Phase 10 — Ballot

| Test type | What to cover |
|---|---|
| Unit | Draw algorithm: same seed → same output (1000 iterations), no duplicates, quota exactness, waitlist rank preservation |
| Unit | State machine: invalid transitions rejected (e.g. DRAWN→OPEN), lapse+promote happy path |
| Integration | Full apply→draw→announce→convert cycle |
| Integration | Payment window expiry → LAPSED → waitlist promotion → new WINNER → convert |
| Integration | Withdraw while OPEN accepted; withdraw after CLOSED rejected |
| Concurrency | 10k simultaneous apply requests → no duplicate entries, correct use_count |
| Concurrency | Simultaneous convert + expiry worker race (only one wins) |
| Chaos | Draw interrupted at 50% completion → re-run is idempotent |
| Security | Attempt apply after ballot closed → 409; attempt convert with wrong token → 403 |
| Load | 50k applications, draw of 5k winners — draw completes <5s |

### Phase 11 — Access Engine

| Test type | What to cover |
|---|---|
| Unit | Quota atomicity: reserve+consume never exceeds total_slots |
| Unit | Code hash/verify round-trip, expiry logic, eligibility rule evaluation |
| Unit | Corporate bulk upload parsing (CSV edge cases: duplicates, missing fields) |
| Integration | Full redeem→grant→checkout flow for each code type |
| Integration | Grant expires before checkout → rejected at gate |
| Integration | Corporate bulk upload → member registration → invoice generation |
| Concurrency | 500 parallel redemptions of quota=100 → exactly 100 grants, 400 ErrCodeExhausted |
| Security | Brute-force code attempt (1000 guesses) → rate limited after 5/min |
| Security | Single-use code used twice → rejected on second use |
| Security | Expired code redemption → rejected |
| Load | 10k simultaneous redemptions at event launch |

---

## LOAD TEST STRATEGY

All load tests via k6 (existing scripts under `tests/k6/`).

### Phase 10 targets
| Scenario | Users | Target |
|---|---|---|
| Ballot application burst | 10k concurrent applies in 60s | Zero duplicate entries, p99 < 2s |
| Draw execution | — | 100k entries drawn in < 10s |
| Winner conversion burst | 5k winners simultaneously clicking pay link | Zero inventory oversell, p99 < 3s |

### Phase 11 targets
| Scenario | Users | Target |
|---|---|---|
| Code redemption launch | 10k concurrent redeems (quota=1000) | Exactly 1000 grants, rest 409, p99 < 1s |
| Corporate bulk upload | 10k member CSV | Upload + grant issuance < 30s |
| Priority window open | 50k eligible users notified, 20k attempt access in first 60s | No oversell, p99 < 2s |
| Quota exhaustion abuse | 100k redemption attempts by 100 bot IPs | Phase 9 blocks ≥99% of bot traffic |

---

## RELEASE STRATEGY

### Phase 10

**Part 1 — Foundation** (independent, no UX change)
- Migrations 00031-00033, sqlc, ballot model/errors/dto/repository, draw algorithm, seed strategy
- All behind `ballot_enabled = false` in registration settings (already in schema)

**Part 2 — Draw engine + organizer APIs**
- ballot service (Apply, RunDraw, AnnounceResults), organizer handler/routes
- Organizer can create draws, run them in staging; no participant-facing features yet

**Part 3 — Participant flow + gate integration**
- Participant apply/withdraw/convert endpoints, registration gate BALLOT case
- Payment window worker, waitlist promotion worker
- Frontend ballot UI

**Part 4 — Reporting + docs**
- Export, audit log viewer, result hash verification, CHANGELOG

### Phase 11

**Part 1 — Foundation**
- Migrations 00035-00037, sqlc, access model/errors/quota atomicity

**Part 2 — Invitation + basic redemption**
- Access codes (INVITATION type), grants, redemption endpoint, gate integration for INVITATION_ONLY
- Frontend "I have a code" flow

**Part 3 — Priority + Community**
- Priority window logic, community quota, eligibility checker
- Gate integration for PRIORITY_ACCESS, WAITLIST_ONLY

**Part 4 — Corporate**
- Corporate accounts, bulk upload, invoice, approval flow
- Corporate-specific frontend

**Part 5 — Docs + hardening**
- Abuse integration (rate limits on redeem), code hash hardening, CHANGELOG

---

## GO / NO-GO CRITERIA

### Phase 10 Go criteria
- [ ] Migrations 00031-00033 roundtrip clean
- [ ] Draw: 1000-run unit test, zero duplicates, deterministic
- [ ] Integration: apply→draw→convert full cycle passes
- [ ] Concurrency: 10k apply race test passes
- [ ] Phase 8 queue tests still green (no regression)
- [ ] Registration gate BALLOT case returns correct error for non-winners
- [ ] Payment window expiry worker tested
- [ ] `go test ./... -race` green

### Phase 11 Go criteria
- [ ] Migrations 00035-00040 roundtrip clean
- [ ] Quota atomicity: 500 concurrent redemptions test passes (exactly N grants)
- [ ] Single-use code cannot be redeemed twice (under concurrency)
- [ ] Rate limiting on redeem (Phase 9 guard) verified
- [ ] Corporate bulk upload: handles 10k CSV, no partial writes
- [ ] Gate integration: INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY all pass
- [ ] Phase 9 + Phase 10 tests still green
- [ ] `go test ./... -race` green

---

## OPEN QUESTIONS

### Phase 10
1. **Waitlist round limit** — how many waitlist promotion rounds before remaining slots go to general sale? Configurable per draw or platform-wide?
2. **Notification channel** — email only, or also in-app? (Affects whether a notifications module is needed before Phase 10)
3. **International time zones for draw** — draw_at should be displayed in organizer's time zone; confirm frontend i18n plan
4. **Multiple ballot draws per category** — e.g. early-bird ballot + general ballot on same category. Supported (remove UNIQUE constraint) or not?
5. **Re-application after NOT_SELECTED** — can a participant re-apply for a second draw on the same category? Default no.

### Phase 11
1. **Code distribution** — are invitation codes distributed entirely outside the platform (email/letter), or does the platform need a bulk-email sending feature?
2. **Eligibility rules schema** — what criteria are needed? (previous event completion, membership ID, employer, geography). Needs spec before implementation.
3. **Invoice format** — PDF generation requires a template. Is PDF needed in Phase 11 or can we ship JSON data first?
4. **Priority window automation** — is the priority window switch (close priority → open general) manual (organizer triggers) or automatic (cron at window_end)? Recommend automatic with manual override.
5. **WAITLIST_ONLY slot-open trigger** — what triggers slots opening? Only cancellations/refunds? Or can organizer manually add slots? Both options should be supported.
6. **Cross-organization partner codes** — `PARTNER` type crosses org boundaries. Is this needed in Phase 11 or deferred?
