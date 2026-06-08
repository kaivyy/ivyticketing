# Phase 11: Access Engine — Design Spec

> Extends the Phase 9–11 roadmap and Phase 10 foundations. See also:
> - `docs/superpowers/specs/2026-06-09-registration-access-architecture.md`
> - `docs/superpowers/specs/2026-06-09-phase10-ballot-design.md`
> EXTEND, NEVER REWRITE. Assumes Phase 10 delivered: LifecycleEngine, WaitlistEngine, AccessPool (RESERVED type), AccessGrant.

---

## Overview

Phase 11 delivers the full Access Engine: all pool types (COMMUNITY/CORPORATE/SPONSOR/VIP/PARTNER/PRIORITY/ELITE), access codes with redemption, corporate bulk registration, priority eligibility, and complete RAE gate integration for all remaining modes (INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY).

**What Phase 10 already delivered that Phase 11 builds on:**
- `access_pools` table (RESERVED type) + atomic slot operations (ReserveSlot/ConsumeSlot/ReleaseSlot)
- `access_grants` table + grant state machine
- `waitlist_entries` + WaitlistEngine (used by quota release)
- LifecycleEngine (used by priority window control)

**Phase 11 adds:**
- All remaining pool types
- `access_pool_members` table
- `access_codes` table + redemption flow
- Corporate accounts + bulk upload + invoice
- EligibilityChecker (priority/community auto-rules)
- Full RAE gate (INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY)
- Admin quota management endpoints

---

## 1. Access Pool Architecture — Full Model

### Pool Types

| Pool Type | Member Source | Requires Approval | Visible to Participant |
|---|---|---|---|
| RESERVED | No members — direct grant issuance (ballot) | No | No |
| COMMUNITY | Organizer bulk + self-apply w/ eligibility | No | Yes |
| CORPORATE | Corporate account bulk CSV | Optional | No |
| SPONSOR | Organizer manual | No | No |
| VIP | Organizer manual | No | No |
| PARTNER | Partner org upload | Optional | No |
| PRIORITY | Auto-eligibility rule | No | Yes (if eligible) |
| ELITE | Organizer manual | No | No |

### access_pools table (Phase 10 added RESERVED; Phase 11 extends with all types)

No schema change needed — `pool_type` column already accepts any text value. Phase 11 adds application-level support for the remaining types and adds:
- `owner_account_id uuid nullable FK → corporate_accounts` (for CORPORATE/PARTNER pools)
- `is_visible_to_participants boolean NOT NULL DEFAULT false`
- `eligibility_rule jsonb nullable` (for PRIORITY/COMMUNITY auto-check)

Migration **00039_alter_access_pools_phase11** adds these columns.

### Atomic Quota Operations (already in Phase 10, Phase 11 reuses unchanged)

```sql
-- ReserveSlot (returns 0 rows = ErrPoolExhausted)
UPDATE access_pools
SET reserved_slots = reserved_slots + 1
WHERE id = $1
  AND reserved_slots + used_slots < total_slots
RETURNING *;

-- ConsumeSlot
UPDATE access_pools
SET reserved_slots = reserved_slots - 1,
    used_slots = used_slots + 1
WHERE id = $1;

-- ReleaseSlot (grant expired)
UPDATE access_pools
SET reserved_slots = reserved_slots - 1,
    released_slots = released_slots + 1
WHERE id = $1;
```

### Pool Transfer

```go
type PoolTransfer struct {
    FromPoolID uuid.UUID
    ToPoolID   uuid.UUID
    N          int
}
// Atomic: deduct from source total_slots, add to dest total_slots
// Guard: from.available_slots >= N (i.e. total - reserved - used >= N)
```

### access_pool_members table

```
access_pool_members
├── id                  uuid PK
├── pool_id             uuid FK → access_pools ON DELETE CASCADE
├── user_id             uuid nullable FK → users  (nullable: pre-loaded before account creation)
├── email               text NOT NULL             (dedup key within pool)
├── member_status       text  PENDING | ACTIVE | REGISTERED | EXPIRED | REVOKED
├── eligibility_meta    jsonb nullable            (membership_id, bib_number, employer...)
├── access_grant_id     uuid nullable FK → access_grants
├── invited_at          timestamptz NOT NULL DEFAULT now()
├── registered_at       timestamptz nullable
└── revoked_at          timestamptz nullable
```

Indexes: `UNIQUE (pool_id, email)`; `(pool_id, member_status)`; `(user_id)` for lookup.

Migration: **00040_create_access_pool_members**

---

## 2. Access Codes + Grants

### access_codes table

```
access_codes
├── id                  uuid PK
├── organization_id     uuid FK → organizations
├── event_id            uuid FK → events
├── category_id         uuid nullable FK → event_categories  (null = event-wide code)
├── code_type           text  INVITATION | PRIORITY | COMMUNITY | CORPORATE | COUPON | PARTNER | SPONSOR | VIP | ELITE
├── code_value_hash     text NOT NULL  (sha256 hex of plaintext code — never store plaintext)
├── is_single_use       boolean NOT NULL DEFAULT true
├── max_uses            integer NOT NULL DEFAULT 1
├── use_count           integer NOT NULL DEFAULT 0
├── valid_from          timestamptz NOT NULL
├── valid_until         timestamptz NOT NULL
├── pool_id             uuid nullable FK → access_pools  (quota pool this code draws from)
├── eligibility_rule    jsonb nullable  (auto-check before granting)
├── created_by          uuid FK → users
├── created_at          timestamptz NOT NULL DEFAULT now()
└── metadata            jsonb nullable  (discount_pct, category_restriction, etc.)
```

Indexes: `UNIQUE (event_id, code_value_hash)`; `(event_id, code_type, valid_until)`; partial index on active codes `WHERE valid_until > now() AND use_count < max_uses`.

Migration: **00041_create_access_codes**

### access_grants table (Phase 10 added; Phase 11 adds columns)

Phase 10 schema already covers the grant lifecycle. Phase 11 adds:
- `code_id uuid nullable FK → access_codes` (which code was redeemed; null for ballot grants)

Migration: **00042_alter_access_grants_add_code_id**

### Redemption Flow

```
POST /api/v1/events/{eventId}/access/redeem
Body: { "code": "PLAINTEXT-CODE", "categoryId": "uuid" }

1. code_value_hash = sha256(body.code)
2. SELECT * FROM access_codes WHERE event_id=$1 AND code_value_hash=$2
   → NOT FOUND → ErrCodeNotFound
3. Check valid_from <= now() <= valid_until → ErrCodeExpired
4. Check use_count < max_uses → ErrCodeExhausted
5. EligibilityChecker.Check(ctx, userID, code.eligibility_rule) → ErrNotEligible
6. AccessPool.ReserveSlot(code.pool_id) → ErrPoolExhausted (if pool exhausted)
7. INSERT INTO access_grants (code_id, participant_id, ..., status=ACTIVE, expires_at=valid_until)
8. UPDATE access_codes SET use_count = use_count + 1 WHERE id=$code_id AND use_count < max_uses
   → 0 rows updated (race condition) → rollback, return ErrCodeExhausted
9. Return AccessGrantDTO {id, token: grant.id.String(), expires_at, category_id}
```

Steps 6-8 run in a single DB transaction. The `use_count` update uses an optimistic race guard.

### Grant Expiry → WaitlistEngine

`ExpireAccessGrants` job (every minute):
1. Find ACTIVE grants where `expires_at < now()` and `order_id IS NULL`
2. Set grant.status=EXPIRED
3. Call `ReleaseSlot(pool_id, grant_id)`
4. If grant's category has an ACTIVE Waitlist: call `WaitlistEngine.PromoteBatch(waitlist_id)`

### AccessGrantChecker Interface (for RAE)

```go
type AccessGrantChecker interface {
    CheckGrant(ctx context.Context, participantID, categoryID uuid.UUID, grantToken string) error
}
// Verifies: grant exists, participant_id matches, category_id matches,
//           status=ACTIVE, expires_at > now()
```

---

## 3. Corporate Module

### corporate_accounts table

```
corporate_accounts
├── id                  uuid PK
├── organization_id     uuid FK → organizations
├── name                text NOT NULL
├── billing_email       text NOT NULL
├── invoice_required    boolean NOT NULL DEFAULT false
├── status              text  PENDING | ACTIVE | SUSPENDED
├── approved_at         timestamptz nullable
├── approved_by         uuid nullable FK → users
├── created_by          uuid FK → users
└── created_at          timestamptz NOT NULL DEFAULT now()
```

Migration: **00043_create_corporate_accounts**

### Corporate Pool

A corporate account owns an `access_pool` with `pool_type=CORPORATE` and `owner_account_id` set. The pool is created when the corporate account is approved. Members are `access_pool_members` rows. Each active member gets an access code issued to their email.

### Bulk Upload Flow

```
POST /api/v1/org/{orgId}/access/corporate/{accountId}/members
Content-Type: multipart/form-data; file=members.csv

CSV columns: email, name (optional), eligibility_meta (optional JSON)

1. Parse CSV → validate (dedup emails, check pool capacity)
2. If row_count > pool.available_slots → reject entire upload (not partial)
3. Transactional insert: all AccessPoolMember rows + use_count pre-check
4. Issue access code per member (bulk INSERT into access_codes)
5. Return: {imported: N, skipped: N (duplicates), errors: []}
```

### Invoice

`GET /api/v1/org/{orgId}/access/corporate/{accountId}/invoice` returns JSON:
```json
{
  "account": { "name": "...", "billing_email": "..." },
  "event": { "id": "...", "name": "..." },
  "line_items": [
    { "description": "Corporate slots — Marathon Open", "quantity": 50, "unit_price": 150000, "total": 7500000 }
  ],
  "total": 7500000,
  "currency": "IDR",
  "generated_at": "2026-06-09T..."
}
```

PDF generation is deferred. JSON invoice is the Phase 11 deliverable.

---

## 4. Eligibility Checker

### Purpose

Auto-evaluates whether a participant qualifies for a PRIORITY or COMMUNITY access code without a manual invite.

### Supported Rules (jsonb schema)

```json
{ "returning_runner": true }           — has at least 1 PAID order on any past event in same org
{ "min_completions": 3 }               — has 3+ PAID orders in same org
{ "membership_id_prefix": "MEM" }      — user.membership_id starts with "MEM"
{ "event_completed": "uuid" }          — has PAID order on specific event_id
{ "org_member": true }                 — user belongs to organization (org member role)
```

Rules are AND-combined. Unknown keys are ignored (forward-compatible).

### Interface

```go
type EligibilityChecker interface {
    Check(ctx context.Context, userID uuid.UUID, rule json.RawMessage) (bool, string, error)
    // returns: eligible bool, reason string (for audit), error
}
```

---

## 5. Priority Access

### Flow

1. Organizer creates a LifecyclePhase with `registration_mode=PRIORITY_ACCESS` and a time window
2. Organizer creates an `access_pool` with `pool_type=PRIORITY` + `eligibility_rule`
3. During priority window: participant hits `GET /events/{id}/access/priority-window`
   - EligibilityChecker.Check → if eligible: auto-issue AccessGrant (no code required)
   - If not eligible: return 403 ErrNotEligible
4. Participant uses grant token to checkout
5. When lifecycle phase closes_at passes: LifecycleAdvancer completes PRIORITY_ACCESS phase, activates next phase (NORMAL or WAR_QUEUE)

### PriorityChecker Interface (for RAE)

```go
type PriorityChecker interface {
    CheckPriorityAdmission(ctx context.Context, participantID, categoryID uuid.UUID, grantToken string) error
    // Returns ErrModeNotAvailable if priority window is closed (lifecycle phase not ACTIVE)
    // Returns ErrNotEligible if no valid grant for this participant
}
```

---

## 6. WAITLIST_ONLY Mode

When `category.registration_mode=WAITLIST_ONLY`:
- Participant cannot register directly — no checkout without a grant
- Slots become available when: existing orders are cancelled/refunded → `ReleaseSlot` → `WaitlistEngine.PromoteBatch`
- Participant joins waitlist: `POST /api/v1/events/{id}/categories/{id}/waitlist/join`
- On promotion: AccessGrant issued → participant notified → uses grant token at checkout

WaitlistEngine for WAITLIST_ONLY uses `source=QUOTA_RELEASE`.

---

## 7. Registration Access Engine — Final Form

### gate.go final NewGate signature

```go
func NewGate(
    svc            *Service,
    lifecycle       LifecycleChecker,
    queue           QueueAdmitter,
    ballot          BallotAdmitter,
    accessGrant     AccessGrantChecker,
    priority        PriorityChecker,
) *Gate
```

### Admit() final switch

```go
switch mode {
case ModeNormal:
    return nil
case ModeClosed:
    return ErrClosed
case ModeWarQueue, ModeRandomizedQueue, ModeHybridQueue:
    return g.queue.CheckAdmission(ctx, participantID, eventID, admissionToken)
case ModeBallot:
    return g.ballot.CheckBallotAdmission(ctx, participantID, categoryID, admissionToken)
case ModeInvitationOnly:
    return g.accessGrant.CheckGrant(ctx, participantID, categoryID, admissionToken)
case ModePriorityAccess:
    return g.priority.CheckPriorityAdmission(ctx, participantID, categoryID, admissionToken)
case ModeWaitlistOnly:
    return g.accessGrant.CheckGrant(ctx, participantID, categoryID, admissionToken)
default:
    return ErrModeNotAvailable
}
```

No more `ErrModeNotAvailable` for any defined mode after Phase 11.

---

## 8. API Design

### Participant Endpoints
| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/events/{eventId}/access/redeem` | Redeem access code |
| GET | `/api/v1/events/{eventId}/access/my-grants` | List own active grants |
| GET | `/api/v1/events/{eventId}/access/priority-window` | Check priority eligibility + auto-grant |
| POST | `/api/v1/events/{eventId}/categories/{categoryId}/waitlist/join` | Join WAITLIST_ONLY waitlist |
| GET | `/api/v1/events/{eventId}/categories/{categoryId}/waitlist/my-position` | Waitlist position |

### Organizer Endpoints
| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/org/{orgId}/events/{eventId}/access/codes` | Create access code |
| GET | `/api/v1/org/{orgId}/events/{eventId}/access/codes` | List codes with use_count |
| DELETE | `/api/v1/org/{orgId}/access/codes/{codeId}` | Revoke code (sets valid_until=now) |
| POST | `/api/v1/org/{orgId}/access/codes/bulk` | Bulk-generate invitation codes (returns CSV) |
| GET | `/api/v1/org/{orgId}/events/{eventId}/access/pools` | List pools with utilization |
| PUT | `/api/v1/org/{orgId}/access/pools/{poolId}` | Adjust total_slots |
| POST | `/api/v1/org/{orgId}/access/pools/transfer` | Transfer slots between pools |
| POST | `/api/v1/org/{orgId}/access/corporate` | Create corporate account |
| GET | `/api/v1/org/{orgId}/access/corporate` | List corporate accounts |
| GET | `/api/v1/org/{orgId}/access/corporate/{accountId}` | Get account + members |
| POST | `/api/v1/org/{orgId}/access/corporate/{accountId}/approve` | Approve account |
| POST | `/api/v1/org/{orgId}/access/corporate/{accountId}/members` | Bulk upload members CSV |
| GET | `/api/v1/org/{orgId}/access/corporate/{accountId}/members` | List members + status |
| GET | `/api/v1/org/{orgId}/access/corporate/{accountId}/invoice` | Invoice JSON |

### Admin Endpoints
| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/admin/access/codes` | List codes across all events |
| POST | `/api/v1/admin/access/pools/{poolId}/adjust` | Emergency quota adjustment |

### System Jobs
- `ExpireAccessGrants` — every minute, grants past expires_at → EXPIRED → ReleaseSlot → PromoteBatch
- `ReleaseCorporateSlots` — daily, ACTIVE member grants past corporate account deadline → EXPIRED
- `WaitlistPromoter` — every minute, promotes WAITLIST_ONLY waitlists when slots available (Phase 10 job, reused)

---

## 9. Error Codes

| Code | HTTP | Meaning |
|---|---|---|
| `CODE_NOT_FOUND` | 404 | Code hash not found or event mismatch |
| `CODE_EXHAUSTED` | 409 | use_count >= max_uses or pool exhausted |
| `CODE_EXPIRED` | 409 | now() outside valid_from..valid_until |
| `NOT_ELIGIBLE` | 403 | Eligibility rule check failed |
| `GRANT_NOT_FOUND` | 404 | No matching active grant |
| `GRANT_EXPIRED` | 403 | Grant exists but past expires_at |
| `GRANT_ALREADY_CONSUMED` | 409 | Grant already used for checkout |
| `CORPORATE_NOT_APPROVED` | 403 | Account in PENDING status |
| `POOL_EXHAUSTED` | 409 | No available slots in pool |
| `POOL_TRANSFER_INSUFFICIENT` | 409 | Source pool doesn't have enough available slots |
| `INVALID_CODE_TYPE` | 400 | Unrecognized code_type value |
| `PRIORITY_WINDOW_CLOSED` | 409 | No active PRIORITY_ACCESS lifecycle phase |

---

## 10. Abuse Guard Integration

New guard category constants in `abuse/model.go`:
```go
CategoryAccessRedeem  = "access_redeem"
CategoryWaitlistJoin  = "waitlist_join"
```

Rate limits:
- `access_redeem`: 10/IP/min, 5/user/min
- `waitlist_join`: 20/IP/min, 10/user/min
- Corporate bulk upload: 3/org/hour (custom guard, not per-IP)

Hardening:
- `code_value` is never logged or returned in API responses — only `grant.id` is returned
- Failed redemptions (wrong code) bump IP reputation (+2)
- 3 consecutive ErrCodeNotFound from same IP within 60s → auto-block IP for 10min (new abuse setting: `code_brute_force_block`)
- `code_value_hash` is indexed but the plaintext never stored — brute-force of sha256 is computationally infeasible given rate limits

---

## 11. Frontend Changes

### Part 1: "I have an access code" (INVITATION_ONLY events)
- Event page: "I have an access code" link → modal with code input
- On success: redirect to checkout for unlocked category
- On error: clear message (exhausted / expired / not eligible)

### Part 2: Priority window countdown
- Eligible participants: "Priority access opens in [countdown]" banner
- During window: "Priority Access Active — register now" CTA
- After window: banner disappears, normal flow shown

### Part 3: Community pool availability
- Category page: "Community spots available: N" if `pool.is_visible_to_participants=true`
- Self-apply button if eligibility_rule defined and participant may qualify
- Clear error if not eligible

### Part 4: Corporate registration flow
- Organizer: Corporate accounts tab in event management
- Member receives email with code → redeems via normal "I have a code" flow
- Corporate dashboard: member registration status, invoice download

### Part 5: "My Access" participant page
- List of active grants with category + expiry
- Redemption history (consumed/expired)
- Waitlist positions with rank indicator

---

## 12. Test Strategy

### Access Pool
- Unit: ReserveSlot atomic — 500 goroutines, quota=100 → exactly 100 succeed
- Unit: available_slots never negative (check constraint test)
- Unit: Pool transfer — both pools updated correctly, insufficient slots rejected
- Integration: Full create → reserve → consume → release cycle

### Access Codes + Grants
- Unit: Code hash round-trip (sha256, never reverses)
- Unit: Single-use code redeemed twice → second rejected (use_count race guard)
- Unit: Expired code rejected, exhausted code rejected
- Unit: Eligibility rules — each rule type (5 rules × pass/fail)
- Integration: Full redeem → grant → checkout cycle for each code type
- Integration: Grant expires → ReleaseSlot → PromoteBatch fires
- Concurrency: 500 parallel redemptions of quota=100 code → exactly 100 grants
- Security: 1000 bad code attempts from same IP → rate limited after 10/min, reputation bumped

### Corporate
- Unit: CSV parser — duplicates, missing fields, oversized upload
- Integration: Bulk upload → member rows → code issuance → member registration
- Integration: Unapproved account → redemption rejected

### Eligibility Checker
- Unit: Each rule type passes and fails correctly
- Unit: Unknown rule keys ignored (forward-compat)
- Unit: AND combination of multiple rules

### RAE — Full Coverage
- Unit: Each of 9 modes dispatches to correct admitter
- Unit: Lifecycle window closed → deny before reaching any admitter
- Integration: INVITATION_ONLY full chain
- Integration: PRIORITY_ACCESS full chain (window open → grant → checkout; window closed → next mode)
- Integration: WAITLIST_ONLY full chain (join → slot opens → promote → grant → checkout)

### Load
- 10k concurrent redemptions at event launch, quota=1000 → exactly 1000 grants, p99 < 1s
- Corporate bulk upload 10k CSV → completes < 30s
- Priority window opens: 50k eligible notified, 20k attempt in first 60s → no oversell

---

## 13. Release Strategy

### Part 1 — Foundation
Migrations 00039–00043. AccessPool full type support (add columns). AccessPoolMember. CorporateAccount. EligibilityChecker skeleton. AccessCode model. All behind `access_engine_enabled=false` platform setting.

### Part 2 — Invitation + basic redemption
AccessCode (INVITATION type), AccessGrant, redemption endpoint, `AccessGrantChecker`. Gate integration for INVITATION_ONLY. Frontend "I have a code" flow. Tests.

### Part 3 — Priority + Community
PRIORITY pool, EligibilityChecker (all 5 rules), `PriorityChecker`. COMMUNITY pool + self-apply. Gate integration for PRIORITY_ACCESS, WAITLIST_ONLY. Priority window countdown frontend. Tests.

### Part 4 — Corporate
CorporateAccount service. Bulk upload (CSV parse + transactional insert). Invoice JSON. Approval flow. Corporate frontend tab. Tests.

### Part 5 — Hardening + docs
Code brute-force block setting. Reputation bumps on failed redemption. Full RAE integration test suite. CHANGELOG. k6 load test scripts.

---

## 14. Go / No-Go Criteria

- [ ] Migrations 00039–00043 roundtrip clean
- [ ] AccessPool: 500-goroutine atomicity test passes (exactly N grants)
- [ ] Single-use code reuse rejected under concurrency
- [ ] EligibilityChecker: all 5 rule types tested
- [ ] Full RAE integration: all 9 modes tested
- [ ] Corporate bulk upload: 10k CSV, no partial writes
- [ ] Gate: INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY all pass
- [ ] Phase 9 + Phase 10 tests still green
- [ ] `go test ./... -race` green
