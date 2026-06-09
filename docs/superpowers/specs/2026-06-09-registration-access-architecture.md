# Registration Access Architecture — Pre-Phase 10/11 Design Refinement

> **Architecture document only. No implementation. No migrations. No code.**
> Extends the existing Phase 10–11 roadmap. Does not rewrite or discard it.

---

## REGISTRATION ACCESS ENGINE

### Purpose

The Registration Access Engine (RAE) is a single orchestrator that answers one question at checkout time:

> **"May this participant register for this category right now?"**

Currently `registration/gate.go` `Admit()` is a switch on registration mode — it will grow fragile as Phase 10 (Ballot) and Phase 11 (Access Engine) add their own admission paths. The RAE formalizes that switch into a composable, auditable decision pipeline.

**The key insight:** The existing `orders.RegistrationGate` interface is already the right abstraction — `Admit(ctx, participantID, eventID, categoryID, admissionToken) error`. The RAE is the concrete implementation of that interface. Nothing in orders/payments changes.

---

### Responsibilities

1. **Mode resolution** — resolve the effective registration mode for the category (category override → event default), delegating to `registration.Service.ResolveForCheckout` (already exists)
2. **Lifecycle gate** — check the Registration Lifecycle Engine (Part 4): is the current window open? Has the mode transitioned?
3. **Access grant evaluation** — for access-controlled modes (BALLOT, INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY), verify the participant holds a valid access grant or admission token
4. **Queue admission** — for queue modes (WAR_QUEUE, RANDOMIZED_QUEUE, HYBRID_QUEUE), delegate to `queue.QueueAdmitter` (already exists)
5. **Quota check** — verify the category has remaining capacity (already enforced in inventory; RAE does not duplicate this, it trusts the inventory reservation layer)
6. **Audit** — record every admission decision (allowed or denied) with reason code
7. **Error enrichment** — return structured errors that the frontend can act on (e.g. `BALLOT_NOT_WINNER` vs `QUEUE_NOT_ADMITTED` vs `PRIORITY_WINDOW_CLOSED`)

---

### Decision Tree

```
Admit(ctx, participantID, eventID, categoryID, admissionToken)
│
├─ 1. ResolveMode(eventID, categoryID)
│     → effective Mode
│
├─ 2. LifecycleGate.IsWindowOpen(ctx, categoryID, Mode)
│     → false → ErrRegistrationClosed (with reason: WINDOW_NOT_OPEN / WINDOW_EXPIRED / MODE_NOT_ACTIVE)
│
├─ 3. switch Mode:
│
│   NORMAL
│     → allow (inventory checked at reservation time)
│
│   CLOSED
│     → ErrRegistrationClosed
│
│   WAR_QUEUE / RANDOMIZED_QUEUE / HYBRID_QUEUE
│     → QueueAdmitter.CheckAdmission(ctx, participantID, eventID, admissionToken)
│
│   BALLOT
│     → BallotAdmitter.CheckBallotAdmission(ctx, participantID, categoryID, admissionToken)
│
│   INVITATION_ONLY
│     → AccessGrantChecker.CheckGrant(ctx, participantID, categoryID, admissionToken)
│
│   PRIORITY_ACCESS
│     → PriorityChecker.CheckPriorityAdmission(ctx, participantID, categoryID, admissionToken)
│        → if priority window closed → fall through to lifecycle engine for next mode
│
│   WAITLIST_ONLY
│     → AccessGrantChecker.CheckGrant(ctx, participantID, categoryID, admissionToken)
│        (waitlist grants are issued by the Waitlist Engine when slots open)
│
└─ 4. On allow → audit.Record(ADMISSION_ALLOWED, mode, reason)
   On deny  → audit.Record(ADMISSION_DENIED, mode, reason)
             → return structured error
```

---

### State Diagram

```
[Request] ──► [Mode Resolved] ──► [Window Check]
                                       │
                              closed ◄─┤─► open
                                       │
                              [Mode Dispatch]
                              ┌────────┼──────────┐
                           NORMAL   QUEUE      ACCESS-GATED
                              │        │       (BALLOT/INV/PRI/WAIT)
                           allow    queue        grant check
                                   admit         │
                                      │       valid ──► allow
                                      │       invalid ─► deny + bump reputation
                              allow/deny
```

---

### Integration Points

| Collaborator | Interface | Direction |
|---|---|---|
| `registration.Service` | `ModeResolver` | RAE calls → resolves mode |
| `LifecycleEngine` | `WindowChecker` | RAE calls → is window open |
| `queue.Service` | `QueueAdmitter` (existing) | RAE calls → queue admission |
| `ballot.Service` | `BallotAdmitter` (new, Phase 10) | RAE calls → ballot admission |
| `access.Service` | `AccessGrantChecker` (new, Phase 11) | RAE calls → grant check |
| `access.Service` | `PriorityChecker` (new, Phase 11) | RAE calls → priority check |
| `audit.Logger` | `AuditRecorder` (existing) | RAE calls → admission audit |
| `orders.Service` | `RegistrationGate` (existing) | orders calls RAE |
| `abuse.Guard` | middleware (existing) | applied upstream, before RAE |

**Ownership boundary:** The RAE lives in `services/api/internal/modules/registration/`. It does not own ballot, access, or queue logic — it only dispatches to them via narrow interfaces defined in the registration package (dependency inversion, same pattern as existing `QueueAdmitter`).

---

### Extension Strategy

Adding a new registration mode (e.g. Phase 12 "CORPORATE_ONLY"):
1. Add constant to `registration/model.go`
2. Add interface (e.g. `CorporateAdmitter`) in `registration/gate.go`
3. Add case to the RAE switch
4. Inject implementation in `server.go`

No existing cases change. No existing modules change.

---

### Failure Handling

| Failure | Behavior |
|---|---|
| Mode resolver DB error | Fail closed (deny + log) — prevents unknown mode bypass |
| Lifecycle engine DB error | Fail closed (deny + log) |
| Queue admitter error | Propagate as-is (queue already has its own error types) |
| Ballot admitter error | Propagate as-is |
| Access grant checker error | Propagate as-is |
| Audit write failure | Non-fatal — log only, do not block admission |

---

### Audit Strategy

Every call to `Admit()` records an `audit.Entry`:
- Action: `ADMISSION_ALLOWED` or `ADMISSION_DENIED`
- TargetType: `"category"`
- TargetID: categoryID
- Metadata: `{mode, reason_code, admission_token_present: bool}`
- ActorUserID: participantID

This gives a complete admission audit log without requiring each sub-admitter to audit independently.

---

### Testing Strategy

- **Unit:** RAE switch — each mode dispatches to the correct admitter, admitter errors propagate correctly, lifecycle gate closed → deny before reaching admitter
- **Unit:** Failure handling — DB error on mode resolve → deny (not 500)
- **Integration:** Full admit chain per mode (one integration test per mode)
- **Race:** Concurrent admits on the same category — no shared mutable state in RAE itself (all state is in DB/sub-services)

---

## ACCESS POOL ARCHITECTURE

### Problem Statement

Community, Corporate, VIP, Sponsor, Partner, Priority, Elite are currently modeled as separate `code_type` values with shared `access_quotas`. The risk is that quota logic diverges per type. The solution is to formalize a unified **Access Pool** domain model where all types share the same quota mechanics, differing only in:
- How members are added (manual, bulk CSV, auto-eligibility)
- Who can consume (specific user list vs any eligible user)
- Whether approval is required
- Whether the pool is visible to participants

---

### Domain Model

```
AccessPool
├── id
├── organization_id
├── event_id
├── category_id (nullable — pool may cover whole event)
├── pool_type: COMMUNITY | CORPORATE | SPONSOR | VIP | PARTNER | PRIORITY | ELITE | RESERVED
├── name
├── total_slots         ← set by organizer
├── reserved_slots      ← atomically incremented on grant issuance
├── used_slots          ← atomically incremented on grant consumption
├── released_slots      ← incremented on grant expiry
├── owner_account_id (nullable → CorporateAccount / PartnerAccount)
├── requires_approval: bool
├── is_visible_to_participants: bool
├── valid_from / valid_until
├── created_by / created_at / updated_at
│
└── available_slots = total_slots - reserved_slots - used_slots
                      (derived, not stored — computed at query time)
```

**Key invariant:** `reserved_slots + used_slots ≤ total_slots` — enforced at DB level via check constraint and atomic `UPDATE ... WHERE reserved_slots + used_slots + 1 <= total_slots RETURNING *`.

---

### Pool Member Model

```
AccessPoolMember
├── id
├── pool_id → AccessPool
├── user_id (nullable — pre-loaded before account creation)
├── email             ← dedup key within pool
├── member_status: PENDING | ACTIVE | REGISTERED | EXPIRED | REVOKED
├── eligibility_metadata (jsonb — e.g. membership_id, bib_number, employer)
├── access_grant_id (nullable → AccessGrant, set when granted)
├── invited_at / registered_at / revoked_at
```

**Pool types and member sourcing:**

| Pool Type | Member Source | Approval | Visible to participant |
|---|---|---|---|
| COMMUNITY | Organizer bulk upload / self-apply with eligibility rule | No | Yes (shows "community spots available") |
| CORPORATE | Corporate account bulk CSV | Optional | No (codes distributed by corp) |
| SPONSOR | Organizer manual | No | No |
| VIP | Organizer manual | No | No |
| PARTNER | Partner org upload | Optional | No |
| PRIORITY | Auto-eligibility check (rule-based) | No | Yes (if eligible, CTA shown) |
| ELITE | Organizer manual | No | No |
| RESERVED | No members — direct grant issuance | No | No |

---

### Quota Operations (all atomic)

```
ReserveSlot(poolID, memberID) → AccessGrant | ErrPoolExhausted | ErrMemberNotInPool
  → UPDATE access_pools SET reserved_slots = reserved_slots + 1
     WHERE id = $1 AND reserved_slots + used_slots < total_slots
     RETURNING *
  → if 0 rows updated → ErrPoolExhausted

ConsumeSlot(poolID, grantID)
  → UPDATE access_pools SET reserved_slots = reserved_slots - 1, used_slots = used_slots + 1

ReleaseSlot(poolID, grantID)  ← called on grant expiry
  → UPDATE access_pools SET reserved_slots = reserved_slots - 1, released_slots = released_slots + 1
```

No application-level retry needed — the DB-level atomic UPDATE handles concurrency.

---

### State Machine (AccessGrant)

```
PENDING ──redeem──► ACTIVE ──checkout_complete──► CONSUMED (terminal)
                       └──────expires──────────► EXPIRED  (terminal)
```

On EXPIRED: `ReleaseSlot` called → slot available for new grant from same pool.
On CONSUMED: `ConsumeSlot` called → slot permanently used.

---

### Quota Transfer

Organizer may transfer slots between pools of the same event+category:
- `TransferSlots(fromPoolID, toPoolID, n)` — only if `from.available_slots ≥ n`
- Atomic: decrement `total_slots` on source, increment `total_slots` on destination
- Audit: `POOL_TRANSFER` action

**Constraint:** No transfer if it would leave `from.total_slots < from.used_slots + from.reserved_slots` (would make available go negative).

---

### Reporting Model

Per pool:
- `total_slots` / `reserved` / `used` / `available` / `released`
- `conversion_rate = used / (used + expired)` — how many granted slots converted to registrations
- `utilization_rate = (reserved + used) / total_slots`

Per event:
- Sum of all pool slots vs `event_categories.capacity`
- `unallocated_capacity = category.capacity - sum(pool.total_slots) - direct_registrations`

---

### Risk Analysis

| Risk | Mitigation |
|---|---|
| Pool total_slots > category.capacity | Warn at creation; hard cap enforced via application-level check (not DB FK — category capacity changes don't automatically cascade) |
| Two organizers simultaneously create pools that together exceed capacity | Serialized pool creation via DB transaction + capacity check |
| Corporate bulk upload exceeds pool quota | Validate CSV row count against available slots before any insert |
| Pool created but no members added before valid_until | System alert at (valid_until - 48h) if pool has 0 members |

---

### Testing Strategy

- **Unit:** Quota atomicity — `ReserveSlot` under 500 concurrent goroutines, exactly N slots reserved, remainder ErrPoolExhausted
- **Unit:** `available_slots` never goes negative
- **Integration:** Full pool lifecycle (create → add members → reserve → consume / expire → release)
- **Integration:** Transfer between pools
- **Race:** Concurrent reserve + release on same pool

---

## WAITLIST ENGINE

### Problem Statement

The roadmap currently has three distinct waitlist concepts:
1. Ballot waitlist (WAITLISTED status on `ballot_entries`)
2. `WAITLIST_ONLY` registration mode
3. Future: queue overflow waitlist, quota-release waitlist

If each is implemented independently they will diverge. The Waitlist Engine is a **shared promotion service** — the mechanism of "queue someone up and promote them when a slot opens" is the same regardless of source.

---

### Engine Design

The Waitlist Engine is a module (`waitlist`) that manages ordered participant queues and slot-promotion. It is **not** a replacement for the ballot entry list or the queue token — it is an additional coordination layer that issues access grants when promotions fire.

**Core abstraction:**

```
WaitlistEntry
├── id
├── waitlist_id → Waitlist
├── participant_id → users
├── event_id / category_id
├── source: BALLOT | QUEUE | INVITATION | PRIORITY | QUOTA_RELEASE | MANUAL
├── source_ref_id (nullable — ballot_entry_id or queue_token_id)
├── status: WAITING | PROMOTED | EXPIRED | WITHDRAWN
├── rank (integer — promotion order)
├── score (bigint — determines rank: FIFO=timestamp, RANDOM=seeded, HYBRID=composite)
├── notified_at (nullable)
├── promoted_at (nullable)
├── access_grant_id (nullable — set on promotion)
├── promotion_window_hours
├── created_at

Waitlist
├── id
├── event_id / category_id
├── mode: FIFO | RANDOMIZED | HYBRID
├── status: ACTIVE | PAUSED | CLOSED
├── max_promotion_batch (integer — how many to promote at once)
├── promotion_window_hours
├── auto_promote: bool (cron-driven) vs manual
├── created_at
```

---

### State Machine (WaitlistEntry)

```
WAITING ──promote──► PROMOTED ──grant_consumed──► (entry closed, grant=CONSUMED)
   │                    └──window_expires──► EXPIRED → slot released → may re-promote next
   └──withdraw──► WITHDRAWN (terminal)
   └──waitlist_closed──► EXPIRED (terminal)
```

---

### Promotion Algorithm

**FIFO:** rank = `joined_at` timestamp (lowest = first promoted)

**RANDOMIZED:** rank = `presale_score(seed, participant_id)` — same algorithm as queue PRESALE pool. Seed committed before any promotions fire. Reproducible.

**HYBRID:** rank = FIFO during a priority window, then RANDOMIZED for remainder (matches queue HYBRID_QUEUE pattern).

**Promotion batch:**
1. Lock waitlist row (`SELECT FOR UPDATE`)
2. Fetch top N `WAITING` entries by rank
3. For each: call `AccessPool.ReserveSlot` → if success: create `AccessGrant`, set entry status=PROMOTED, set `access_grant_id`, record `notified_at`
4. If `ReserveSlot` returns `ErrPoolExhausted` → stop batch (no more slots)
5. Release waitlist lock
6. Trigger notification (async — separate job, not in the promotion transaction)

**Idempotency:** promotion job checks `status=WAITING` before acting — safe to re-run.

---

### Sources and Integration

| Source | What adds to waitlist | What triggers promotion |
|---|---|---|
| BALLOT lapse | `AutoPromoteWaitlist` job after each WINNER→LAPSED batch | Immediate (same job run) |
| QUEUE overflow | Future — queue_tokens.EXPIRED can feed waitlist | Cron + `ExpireQueueTokens` |
| INVITATION unused | If organizer enables waitlist for INVITATION_ONLY category | Organizer manually triggers or cron |
| QUOTA_RELEASE | Access grant expires → `ReleaseSlot` → triggers promotion | `ExpireAccessGrants` job |
| MANUAL | Organizer adds participant directly | Organizer triggers promote |

**Ballot-specific integration:** `ballot_entries.status=WAITLISTED` entries are mirrored as `WaitlistEntry` records with `source=BALLOT` and `source_ref_id=ballot_entry_id`. When the Waitlist Engine promotes, it updates both the `WaitlistEntry` and the corresponding `ballot_entries.status=WINNER` in a single transaction.

---

### Expiration

When a promoted participant's access grant expires without checkout:
1. `ExpireAccessGrants` job sets `AccessGrant.status=EXPIRED`
2. Calls `ReleaseSlot(poolID, grantID)` → slot available
3. Sets `WaitlistEntry.status=EXPIRED`
4. Triggers next promotion batch (same job run)

**Re-entry policy:** A participant whose promotion expired is NOT automatically re-added to the waitlist. Re-entry requires explicit re-application (for ballot) or organizer action. This prevents infinite loops and reduces gaming.

---

### Failure Handling

| Failure | Behavior |
|---|---|
| `ReserveSlot` fails during promotion | Entry stays WAITING; retry on next job run |
| Notification send fails | Entry still promoted; notification retried independently |
| Transaction aborted mid-batch | All-or-nothing per entry (one transaction per entry promotion); entries not promoted stay WAITING |
| Waitlist Engine unavailable | Slots not promoted; cron retries on next tick; no slot loss |

---

### Audit Strategy

- `WAITLIST_JOINED` — participant added
- `WAITLIST_PROMOTED` — participant promoted, grant issued
- `WAITLIST_EXPIRED` — promotion window expired, slot released
- `WAITLIST_WITHDRAWN` — participant withdrew
- `WAITLIST_PROMOTION_BATCH` — batch run metadata (N promoted, N slots remaining, seed if RANDOMIZED)

---

### Testing Strategy

- **Unit:** FIFO rank ordering, RANDOMIZED reproducibility, HYBRID window boundary
- **Unit:** Promotion batch — exactly N promoted when N slots available, stops at pool exhaustion
- **Unit:** Re-entry policy — expired entry not re-added automatically
- **Integration:** Ballot lapse → waitlist promotion → new grant → checkout
- **Integration:** Quota release → waitlist promotion → grant
- **Concurrency:** Two promotion jobs run simultaneously — no double-promotion (SELECT FOR UPDATE)
- **Chaos:** Promotion job crashes mid-batch — surviving entries stay WAITING, no orphaned grants

---

## REGISTRATION LIFECYCLE ENGINE

### Problem Statement

There is no model for how a category's registration mode evolves over time. Example flows:

```
PRIORITY window (48h) → WAR_QUEUE → WAITLIST_ONLY → CLOSED
BALLOT application window → DRAWN → winner payment window → WAITLIST rounds → general NORMAL sale → CLOSED
INVITATION_ONLY → NORMAL (leftover slots) → CLOSED
```

Without a lifecycle model, these transitions are ad-hoc per mode and per organizer action, making automation, scheduling, and emergency controls impossible.

---

### Lifecycle Model

A `RegistrationLifecycle` is a **sequence of phases** attached to a category. Each phase has a mode, a window (open/close times), and transition triggers.

```
RegistrationLifecycle
├── id
├── event_id / category_id
├── status: DRAFT | ACTIVE | PAUSED | COMPLETED | CANCELLED
├── current_phase_index (integer)
├── created_by / created_at / updated_at

LifecyclePhase
├── id
├── lifecycle_id → RegistrationLifecycle
├── phase_index (integer, ordered)
├── registration_mode (maps to existing Mode constants)
├── label (e.g. "Priority Window", "General Sale", "Waitlist")
├── opens_at (timestamptz, nullable — null = immediately after previous phase)
├── closes_at (timestamptz, nullable — null = manual close only)
├── capacity_override (nullable — override category.capacity for this phase)
├── auto_advance: bool (advance to next phase when closes_at passes)
├── status: PENDING | ACTIVE | COMPLETED | SKIPPED
├── activated_at / completed_at
```

---

### State Machine

**Lifecycle:**
```
DRAFT ──activate──► ACTIVE ──all_phases_complete──► COMPLETED
                       └──pause──► PAUSED ──resume──► ACTIVE
                       └──cancel──► CANCELLED (terminal)
                       └──emergency_stop──► PAUSED (admin action, immediate)
```

**Phase:**
```
PENDING ──lifecycle_activates + opens_at reached──► ACTIVE
ACTIVE ──closes_at reached (if auto_advance)──► COMPLETED → next PENDING phase activates
ACTIVE ──manual_close──► COMPLETED → next PENDING phase activates
ACTIVE ──skip──► SKIPPED → next PENDING phase activates
COMPLETED / SKIPPED ──(terminal for this phase)
```

---

### Transition Matrix

| From Mode | To Mode | Trigger | Automatic? |
|---|---|---|---|
| PRIORITY_ACCESS | WAR_QUEUE | Priority window closes_at | Yes (auto_advance) |
| PRIORITY_ACCESS | NORMAL | Priority window closes_at | Yes |
| BALLOT (announced) | WAITLIST_ONLY | Payment window expires | Yes |
| WAITLIST_ONLY | NORMAL | All waitlist rounds exhausted | Yes or Manual |
| NORMAL | CLOSED | closes_at reached | Yes |
| INVITATION_ONLY | NORMAL | Manual override | No |
| Any active | PAUSED | Admin emergency stop | Manual |
| PAUSED | previous active | Admin resume | Manual |

---

### Admin Controls

| Control | Effect | Requires |
|---|---|---|
| `PauseLifecycle` | Current phase stays but no new admits | Platform admin or event organizer |
| `ResumeLifecycle` | Current phase resumes | Platform admin or event organizer |
| `AdvancePhase` | Skip current phase, activate next | Organizer |
| `SkipPhase` | Mark a future phase SKIPPED without activating it | Organizer |
| `EmergencyStop` | Immediately pause + set lifecycle.status=PAUSED | Platform admin only |
| `ExtendPhase` | Update closes_at of active phase | Organizer |
| `OverrideCapacity` | Set capacity_override on active phase | Organizer |

---

### Scheduling

The lifecycle engine runs as a cron job (`LifecycleAdvancer`) every minute:
1. Query all `LifecyclePhase` where `status=ACTIVE AND auto_advance=true AND closes_at < now()`
2. For each: complete current phase, activate next PENDING phase (if any)
3. If no next phase: mark lifecycle COMPLETED

**Concurrency safety:** Lifecycle advancement is serialized per lifecycle via `SELECT FOR UPDATE ON lifecycle`. Two cron workers advancing the same lifecycle simultaneously is safe — the second will see the phase already COMPLETED and no-op.

---

### Integration with RAE

The RAE calls `LifecycleEngine.IsWindowOpen(ctx, categoryID, effectiveMode)` before dispatching to any admitter:
- Returns true if there is an ACTIVE phase with matching `registration_mode` for the category
- Returns false (with reason code) otherwise

This means the RAE does not need to know about phase scheduling — it only asks "is this mode currently open?"

---

### Failure Handling

| Failure | Behavior |
|---|---|
| Lifecycle cron fails | Phases don't auto-advance; organizer can manually advance; no data loss |
| Phase activation DB error | Phase stays PENDING; retry on next cron tick |
| Emergency stop during draw (ballot) | Draw job checks lifecycle status before writing results; if PAUSED, defers |
| Phase advances while participant is mid-checkout | Order already created → checkout completes normally (RAE already admitted them) |

---

### Audit Strategy

- `LIFECYCLE_PHASE_ACTIVATED` — phase became ACTIVE (with opens_at, mode)
- `LIFECYCLE_PHASE_COMPLETED` — phase ended (with completed_at, reason: auto_advance/manual/emergency)
- `LIFECYCLE_PAUSED` / `LIFECYCLE_RESUMED` — with actor
- `LIFECYCLE_EMERGENCY_STOP` — always with platform admin actor

---

### Testing Strategy

- **Unit:** Phase transition state machine — invalid transitions rejected
- **Unit:** `IsWindowOpen` returns false when no ACTIVE phase matches mode
- **Unit:** Auto-advance cron — phase completes at closes_at, next phase activates
- **Integration:** Full priority→queue→waitlist→closed lifecycle cycle
- **Integration:** Emergency stop pauses admission, resume restores it
- **Concurrency:** Two cron workers advancing same lifecycle → idempotent (exactly one wins)
- **Chaos:** Cron down for 30 minutes → phases catch up on next run; no phases skipped

---

## CROSS-SYSTEM DEPENDENCIES

### Access Engine ↔ Queue

- Queue modes check `QueueAdmitter` — RAE dispatches there unchanged
- Priority Access users bypass queue entirely — they go through `AccessGrantChecker`, never touch queue tables
- `HYBRID_QUEUE` presale pool and `PRIORITY_ACCESS` are different things: presale = early-scored queue position; priority = no queue at all
- **Potential race:** A participant holds a queue ALLOWED token AND an access grant for the same category. The RAE should prefer the access grant path for INVITATION_ONLY/PRIORITY_ACCESS categories — queue tokens should not exist for those modes. Lifecycle Engine prevents this by ensuring modes are mutually exclusive per phase.

### Access Engine ↔ Ballot

- Ballot is not an access pool — it has its own entry/draw lifecycle
- At the point of conversion (WINNER → order), the ballot issues an `AccessGrant` scoped to the winner's slot. This grant is then checked by `BallotAdmitter`, not `AccessGrantChecker`. The grant is issued from a `RESERVED` pool created specifically for that draw.
- **No double-grant risk:** Ballot creates one grant per winner entry. Grant issuance and winner status update are atomic.

### Access Engine ↔ Waitlist Engine

- Waitlist Engine consumes `AccessPool` slots (calls `ReserveSlot`)
- When Waitlist Engine promotes, it creates an `AccessGrant` — the promoted participant then uses that grant through the normal RAE path
- **Pool must pre-exist:** The Waitlist Engine requires an `AccessPool` to exist for the category before it can promote. The lifecycle engine's phase activation should auto-create a RESERVED pool if none exists.

### Access Engine ↔ Inventory

- Inventory knows nothing about pools or grants — it only sees `inventory_reservations`
- An `AccessGrant` being ACTIVE does NOT hold an inventory reservation — the reservation is created only when checkout begins (existing behavior)
- **Risk:** A participant has an ACTIVE grant but capacity is exhausted by others at checkout time. Resolution: `AccessPool.reserved_slots` tracks "slots promised but not yet checked out." Pool `total_slots` must be ≤ category capacity minus already-checked-out orders. The lifecycle engine's phase capacity_override can reduce effective capacity per phase.

### Access Engine ↔ Payment

- Payment flow is unchanged — grant is consumed on `PAID` order status (same as queue admission consumption)
- `ConsumeSlot` is called in the `CheckoutHook.OnCheckoutComplete` callback (existing pattern)
- On payment failure / order expiry: grant stays ACTIVE (not consumed); on order expiry → grant not consumed → grant eventually expires → slot released

### Lifecycle Engine ↔ Queue

- Queue join is gated by `LifecycleEngine.IsWindowOpen` in RAE — if the queue phase is not active, join returns `ErrRegistrationClosed` with reason `WINDOW_NOT_OPEN`
- Queue control (`StateRunning/StatePaused`) is a separate operational control from the lifecycle — queue can be paused mid-phase for operational reasons without closing the lifecycle phase

### Lifecycle Engine ↔ Ballot

- Ballot `application_opens_at / application_closes_at` are ballot-internal — they are a subset of the lifecycle's BALLOT phase window
- Lifecycle phase `opens_at / closes_at` must encompass the ballot application window
- When lifecycle auto-advances out of BALLOT phase, it should also close the ballot draw if still OPEN (cascade close)

### Lifecycle Engine ↔ Priority

- Priority phase window and `AccessPool.valid_from/valid_until` must be consistent — the lifecycle phase is the authoritative window; pool valid times are informational
- On priority phase completion: RAE sees no active PRIORITY_ACCESS phase → denies priority-path admits; next NORMAL or WAR_QUEUE phase activates

### Lifecycle Engine ↔ Waitlist

- Waitlist Engine runs while lifecycle is in WAITLIST_ONLY phase
- If lifecycle advances to CLOSED: `Waitlist.status=CLOSED`, no further promotions
- If lifecycle advances to NORMAL (remaining slots): waitlist entries that haven't been promoted yet can be fast-tracked (organizer decision)

---

## STATE MACHINES SUMMARY

### RAE Decision Flow
```
Request → Mode → Window? → Admitter → Allow/Deny → Audit
```

### AccessPool Quota
```
[slot: available] ──reserve──► [slot: reserved] ──consume──► [slot: used]
                                    └──release──► [slot: available]
```

### AccessGrant
```
PENDING ──redeem──► ACTIVE ──paid──► CONSUMED
                       └──expires──► EXPIRED → slot released
```

### WaitlistEntry
```
WAITING ──promote──► PROMOTED ──checkout──► (grant CONSUMED)
   │                    └──expires──► EXPIRED → re-queue policy
   └──withdraw──► WITHDRAWN
```

### LifecyclePhase
```
PENDING ──time/manual──► ACTIVE ──time/manual──► COMPLETED ──► (next phase activates)
                             └──skip──► SKIPPED ──► (next phase activates)
```

### RegistrationLifecycle
```
DRAFT ──activate──► ACTIVE ──all_phases_complete──► COMPLETED
                       └──pause──► PAUSED ──resume──► ACTIVE
                       └──cancel──► CANCELLED
```

---

## AUDIT REQUIREMENTS

All audit entries use existing `audit.Entry` shape. New action codes:

| Action | Trigger | Key metadata |
|---|---|---|
| `ADMISSION_ALLOWED` | RAE allows checkout | mode, category_id |
| `ADMISSION_DENIED` | RAE denies checkout | mode, reason_code, category_id |
| `POOL_CREATED` | Organizer creates pool | pool_type, total_slots |
| `POOL_SLOT_RESERVED` | Grant issued | pool_id, grant_id, participant_id |
| `POOL_SLOT_CONSUMED` | Grant consumed (checkout paid) | pool_id, grant_id, order_id |
| `POOL_SLOT_RELEASED` | Grant expired | pool_id, grant_id |
| `POOL_TRANSFER` | Slots moved between pools | from_pool_id, to_pool_id, n |
| `WAITLIST_JOINED` | Entry added | source, rank |
| `WAITLIST_PROMOTED` | Entry promoted | grant_id, source |
| `WAITLIST_EXPIRED` | Promotion window expired | grant_id |
| `WAITLIST_PROMOTION_BATCH` | Batch run | n_promoted, n_remaining |
| `LIFECYCLE_PHASE_ACTIVATED` | Phase becomes ACTIVE | mode, phase_index |
| `LIFECYCLE_PHASE_COMPLETED` | Phase ends | reason (auto/manual/emergency) |
| `LIFECYCLE_EMERGENCY_STOP` | Admin stops registration | actor, reason |
| `BALLOT_DRAW_SEED_COMMITTED` | Seed stored pre-draw | seed, draw_id |
| `BALLOT_DRAW_COMPLETED` | Draw results written | n_winners, n_waitlisted, seed |

---

## REPORTING REQUIREMENTS

### Per-Category Registration Health
- Current lifecycle phase + mode
- Slots: capacity / sold / reserved / available / pool-allocated
- Pool utilization per pool type
- Waitlist depth per waitlist
- Conversion funnel: applied → granted → checked_out → paid

### Per-Event Aggregate
- Registration mode distribution across categories
- Total revenue by registration mode
- Waitlist depth + promotion rate
- Ballot: application count, draw results, conversion rate, lapse rate

### Organizer Operational Dashboard
- Active lifecycle phase with countdown to auto-advance
- Pool utilization (bar chart: used / reserved / available per pool)
- Waitlist queue depth with estimated wait
- Recent admission denials with reason breakdown

---

## TESTING REQUIREMENTS

### New test categories introduced by this architecture

| Test | What | Pass criteria |
|---|---|---|
| RAE dispatch unit | Each mode → correct admitter called | 9 modes, 9 tests |
| RAE window gate unit | Lifecycle closed → deny before admitter | Any mode |
| RAE audit unit | Every decision produces audit entry | allow and deny |
| Pool atomicity concurrency | 500 reserves of quota=100 | Exactly 100 succeed |
| Pool transfer unit | Transfer n slots, both pools correct | No oversell |
| Waitlist FIFO rank | 100 entries promoted in join order | Rank = join_time |
| Waitlist RANDOMIZED reproducibility | Same seed → same promotion order | 1000 runs |
| Waitlist double-promotion race | Two jobs simultaneously | Zero double-promotes |
| Lifecycle auto-advance | closes_at passes → next phase activates | Phase index increments |
| Lifecycle cron down | Missed ticks → catchup on next run | No phases skipped |
| Emergency stop | Admin pauses → admits denied | Within 1 tick |

---

## RISKS

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Pool total_slots exceeds category.capacity silently | Medium | High | Application-level capacity check at pool creation; warning in organizer UI |
| Two lifecycle phases with different modes active simultaneously (bug) | Low | Critical | DB constraint: UNIQUE ACTIVE phase per lifecycle; application guard |
| Waitlist promotes to exhausted pool (TOCTOU) | Medium | Medium | `ReserveSlot` is atomic — promotion fails gracefully, entry stays WAITING |
| Ballot draw fires while lifecycle is PAUSED | Low | High | Draw job checks lifecycle status before writing results |
| Admin emergency stop loses in-flight checkouts | Low | Low | In-flight orders already past RAE → complete normally |
| Lifecycle not configured for a category (no lifecycle row) | Medium | Low | RAE falls back to direct `ResolveForCheckout` (existing behavior) — lifecycle is additive |
| Multiple organizers editing lifecycle simultaneously | Medium | Medium | Optimistic lock (updated_at check) on lifecycle row |
| Notification flood on mass waitlist promotion | Medium | Medium | Notification batching in async job; rate-limited by notification service |

---

## RECOMMENDATIONS

1. **Add Lifecycle Engine in Phase 10 Part 1** (alongside ballot foundation). The ballot draw + payment window maps perfectly to a 2-phase lifecycle (BALLOT → WAITLIST_ONLY). Building it now prevents refactoring in Phase 11.

2. **Unify pool model before Phase 11 Part 1**. All of corporate/VIP/sponsor/community/partner share the same quota mechanics. Implement `AccessPool` as the single quota table — `access_quotas` in the roadmap becomes `access_pools`.

3. **Waitlist Engine in Phase 10 Part 2** (when ballot waitlist is needed). The ballot waitlist is the first consumer — build the generic engine there rather than ballot-specific logic.

4. **RAE refactor is additive**. The existing `registration/gate.go` becomes the RAE by adding two injected interfaces (`LifecycleChecker`, and later `BallotAdmitter`, `AccessGrantChecker`). No breaking changes. Existing queue tests pass unchanged.

5. **Keep `ballot_entries.status=WAITLISTED`**. The Waitlist Engine mirrors this as a `WaitlistEntry` — it does not replace the ballot entry status. Both exist. The waitlist entry is the promotion mechanism; the ballot entry is the participant's record.

6. **Lifecycle is optional per category**. Categories without a lifecycle row behave as today (existing `ResolveForCheckout` path). Lifecycle adoption is opt-in — this keeps Phase 1-8 events unaffected.

---

## CHANGES TO PHASE 10 ROADMAP

The existing roadmap is preserved. The following **additions** are recommended:

### Part 1 — Foundation (additions)
- Add `RegistrationLifecycle` + `LifecyclePhase` migrations (00031/00032 may need reordering — lifecycle is foundational)
- Add `Waitlist` + `WaitlistEntry` migrations
- Implement `LifecycleEngine` as a thin service (IsWindowOpen, AdvancePhase, EmergencyStop)
- Implement `WaitlistEngine` skeleton (Join, Promote, Expire) — used by ballot in Part 2

### Part 2 — Draw engine
- `AutoPromoteWaitlist` uses `WaitlistEngine.PromoteBatch` rather than direct ballot_entries update
- Ballot creates a `RESERVED` AccessPool for winners at draw time
- Issues AccessGrants from that pool to each winner

### Part 3 — Participant flow
- `BallotAdmitter.CheckBallotAdmission` verifies AccessGrant (from RESERVED pool), not a separate token mechanism
- RAE updated: BALLOT case uses `BallotAdmitter`; `LifecycleChecker` injected

### Revised migration numbering suggestion
```
00031_create_registration_lifecycle.sql
00032_create_lifecycle_phases.sql
00033_create_waitlist.sql
00034_create_waitlist_entries.sql
00035_create_ballot_draws.sql
00036_create_ballot_entries.sql
00037_create_ballot_draw_results.sql
00038_seed_ballot_permissions.sql
```

---

## CHANGES TO PHASE 11 ROADMAP

The existing roadmap is preserved. The following **additions/refinements** are recommended:

### Rename `access_quotas` → `access_pools`
The roadmap uses `access_quotas` as the table name. Rename to `access_pools` to match the unified domain model above. The shape is identical — this is a naming clarification only.

### Pool member model is `access_pool_members` (was `corporate_members`)
`corporate_members` in the roadmap is a special case of `access_pool_members` where `pool.pool_type=CORPORATE`. The generic model handles all member-backed pool types. Corporate accounts remain as-is — they simply own pools of type CORPORATE.

### Revised migration numbering suggestion
```
00039_create_access_pools.sql          (was 00037_create_access_quotas)
00040_create_access_pool_members.sql   (was 00039_create_corporate_members, now generic)
00041_create_access_codes.sql          (was 00035)
00042_create_access_grants.sql         (was 00036)
00043_create_corporate_accounts.sql    (was 00038)
00044_seed_access_permissions.sql      (was 00040)
```

### RAE integration (Phase 11 Part 2)
- `AccessGrantChecker` and `PriorityChecker` injected into RAE
- `LifecycleChecker` already present from Phase 10 — no new injection needed

---

## OPEN QUESTIONS

1. **Lifecycle vs existing category `registration_opens_at / registration_closes_at`** — `event_categories` already has open/close times. Should the Lifecycle Engine supersede these or run alongside? Recommendation: lifecycle supersedes when a lifecycle row exists; existing times used as fallback.

2. **Who creates the lifecycle?** Organizer manually builds phases, or does setting a registration mode auto-create a default 1-phase lifecycle? Recommendation: auto-create single-phase lifecycle on mode set; organizer can add phases.

3. **Waitlist re-entry policy edge cases** — if a participant's promoted grant expires, they can't auto-re-enter. Should they be offered a "rejoin waitlist" CTA? If yes, do they go to back of queue or retain their original rank?

4. **Pool visibility to participants** — for COMMUNITY pools, should participants see "N community spots available"? This requires a public API that exposes pool availability without revealing pool configuration. Design needed.

5. **Lifecycle emergency stop granularity** — stop the whole event or per-category? Recommendation: per-category (stop a specific lifecycle), with a "stop all categories on event" convenience action.

6. **Ballot reserved pool size** — the RESERVED pool for ballot winners equals the draw quota. But if organizer later increases the draw quota (before draw runs), the pool must be updated. Is this automatic or manual?

7. **Cross-phase capacity continuity** — if PRIORITY phase uses 50 slots and NORMAL phase opens for remaining 50, how is the NORMAL phase's effective capacity communicated? Via `capacity_override` on the phase, or derived from actual reservations? Recommendation: `capacity_override` on phase, set by organizer at lifecycle creation.
