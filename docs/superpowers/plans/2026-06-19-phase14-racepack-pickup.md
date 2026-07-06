# Phase 14 — Racepack Pickup System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete racepack pickup system — slot scheduling, multi-counter operation, proxy pickup, problem desk, full audit — that integrates with the existing Ticket and BIB systems and is API-compatible with the future Phase 15 Scanner PWA.

**Architecture:** New self-contained `racepack` Go module owning its own DB tables and lifecycle. Backend exposes JSON APIs only (no HTML state) so the future Scanner PWA can consume the same endpoints. Frontend uses Astro pages as thin shells + Svelte 5 islands for stateful UI (slot manager, counter manager, pickup dashboard, problem desk board, proxy approval modal). Ticket `used_at` column is **not** reused — racepack owns its own pickup record table.

**Tech Stack:** Go 1.22, sqlc-generated pgx/v5 queries, PostgreSQL 15, Astro 4 (SSR + `@astrojs/node` standalone), Svelte 5 (runes), Tailwind 3, `@astrojs/svelte` (added in Part 5).

---

# PART 1 — Foundation & Architecture

> **Scope of this part:** Domain model, database schema, migration 00050, sqlc queries, RBAC extension seed, file structure. No application code yet — only DDL, DML, and the SQL contract that downstream code consumes.

---

## 1.1. Domain Model

Phase 14 introduces **5 new entities**, all owned by the `racepack` module. None of them overload `tickets` or any existing table.

| Entity | Purpose | Lifecycle |
|---|---|---|
| `RacepackCounter` | Physical pickup station at the venue | CRUD by organizer; `active=true/false` toggles whether staff can use it |
| `RacepackPickupSlot` | Time window during which a participant may come to pick up | CRUD by organizer; participants reserve a slot via API |
| `RacepackPickupRecord` | Immutable record of one pickup event | Insert-only on successful pickup; never updated, never deleted |
| `RacepackProxyAuthorization` | Stores proxy identity when someone other than the participant picks up | Insert-only; references `pickup_record_id` on use |
| `RacepackProblemCase` | Escalation record for issues during pickup (missing BIB, identity mismatch, duplicate QR, etc.) | Open → Under Review → Resolved/Escalated |

### 1.1.1. Pickup record invariant (anti-duplicate)

**Rule:** At most one `RacepackPickupRecord` per `ticket_id` is allowed.

**Enforcement:** Unique partial index on `racepack_pickup_records(ticket_id) WHERE status = 'PICKED_UP'`. A `CANCELLED` row does not block a re-pickup, but a `PICKED_UP` row is permanent.

### 1.1.2. Eligibility rule (centralized in `CanPickup()`)

A participant may pick up **only when ALL** of the following hold:

1. `order.status = 'PAID'`
2. `ticket.bib_number IS NOT NULL`
3. `racepack_pickup_records.status != 'PICKED_UP'` for this ticket
4. `racepack_pickup_slot.active = true` (if a slot was assigned)
5. Current time is within `[slot.start_time, slot.end_time]` (if a slot was assigned)
6. `ticket.status = 'VALID'` (not CANCELLED)

All eligibility checks live in **one function** in the racepack service layer. Handlers never re-implement this. The function returns either `Eligible` or a typed error (`ErrOrderNotPaid`, `ErrBibMissing`, `ErrAlreadyPickedUp`, `ErrSlotInactive`, `ErrOutsideWindow`, `ErrTicketCancelled`).

---

## 1.2. Database Schema — Migration 00050

**File:** `database/migrations/00050_create_racepack_pickup.{up,down}.sql`

### 1.2.1. `racepack_counters`

```sql
CREATE TABLE racepack_counters (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name            text NOT NULL,
    location        text,
    active          boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT racepack_counters_name_per_event UNIQUE (event_id, name)
);
CREATE INDEX idx_racepack_counters_event ON racepack_counters(event_id);
CREATE INDEX idx_racepack_counters_event_active ON racepack_counters(event_id) WHERE active = true;
```

### 1.2.2. `racepack_pickup_slots`

```sql
CREATE TABLE racepack_pickup_slots (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name            text NOT NULL,
    pickup_date     date NOT NULL,
    start_time      timestamptz NOT NULL,
    end_time        timestamptz NOT NULL,
    capacity        integer NOT NULL CHECK (capacity > 0),
    reserved_count  integer NOT NULL DEFAULT 0 CHECK (reserved_count >= 0),
    active          boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT racepack_pickup_slots_window CHECK (end_time > start_time),
    CONSTRAINT racepack_pickup_slots_capacity CHECK (reserved_count <= capacity)
);
CREATE INDEX idx_racepack_pickup_slots_event ON racepack_pickup_slots(event_id);
CREATE INDEX idx_racepack_pickup_slots_event_date ON racepack_pickup_slots(event_id, pickup_date);
CREATE INDEX idx_racepack_pickup_slots_event_active ON racepack_pickup_slots(event_id, pickup_date) WHERE active = true;
```

### 1.2.3. `racepack_pickup_records` (insert-only, immutable)

```sql
CREATE TABLE racepack_pickup_records (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id          uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    ticket_id         uuid NOT NULL REFERENCES tickets(id) ON DELETE RESTRICT,
    participant_id    uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    bib_number        text NOT NULL,
    counter_id        uuid NOT NULL REFERENCES racepack_counters(id) ON DELETE RESTRICT,
    staff_id          uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    pickup_method     text NOT NULL CHECK (pickup_method IN ('SELF','PROXY','MANUAL_OVERRIDE')),
    pickup_timestamp  timestamptz NOT NULL DEFAULT now(),
    notes             text,
    status            text NOT NULL DEFAULT 'PICKED_UP'
                      CHECK (status IN ('PICKED_UP','CANCELLED'))
);

-- The anti-duplicate guard. One PICKED_UP record per ticket, ever.
CREATE UNIQUE INDEX uniq_racepack_pickup_records_ticket_active
    ON racepack_pickup_records(ticket_id)
    WHERE status = 'PICKED_UP';

CREATE INDEX idx_racepack_pickup_records_event ON racepack_pickup_records(event_id);
CREATE INDEX idx_racepack_pickup_records_counter ON racepack_pickup_records(counter_id);
CREATE INDEX idx_racepack_pickup_records_staff ON racepack_pickup_records(staff_id);
CREATE INDEX idx_racepack_pickup_records_participant ON racepack_pickup_records(participant_id);
CREATE INDEX idx_racepack_pickup_records_event_timestamp ON racepack_pickup_records(event_id, pickup_timestamp);
```

**Append-only enforcement** is application-level (no UPDATE/DELETE statements in repository; verified by code review and integration tests). A database trigger is **not** added in Part 1 to avoid coupling — can be added later if governance requires.

### 1.2.4. `racepack_proxy_authorizations`

```sql
CREATE TABLE racepack_proxy_authorizations (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id              uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    ticket_id             uuid NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    pickup_record_id      uuid REFERENCES racepack_pickup_records(id) ON DELETE SET NULL,
    proxy_name            text NOT NULL,
    proxy_phone           text,
    proxy_identity        text NOT NULL,
    authorization_document text,
    created_by            uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_racepack_proxy_authorizations_event ON racepack_proxy_authorizations(event_id);
CREATE INDEX idx_racepack_proxy_authorizations_ticket ON racepack_proxy_authorizations(ticket_id);
CREATE INDEX idx_racepack_proxy_authorizations_pickup ON racepack_proxy_authorizations(pickup_record_id);
```

### 1.2.5. `racepack_problem_cases`

```sql
CREATE TABLE racepack_problem_cases (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    ticket_id       uuid REFERENCES tickets(id) ON DELETE SET NULL,
    participant_id  uuid REFERENCES users(id) ON DELETE SET NULL,
    status          text NOT NULL DEFAULT 'OPEN'
                    CHECK (status IN ('OPEN','UNDER_REVIEW','RESOLVED','ESCALATED')),
    reason          text NOT NULL,
    resolution      text,
    created_by      uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    resolved_by     uuid REFERENCES users(id) ON DELETE SET NULL,
    resolved_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_racepack_problem_cases_event ON racepack_problem_cases(event_id);
CREATE INDEX idx_racepack_problem_cases_event_status ON racepack_problem_cases(event_id, status);
CREATE INDEX idx_racepack_problem_cases_ticket ON racepack_problem_cases(ticket_id);
```

### 1.2.6. Down migration

The `.down.sql` mirrors the up in reverse order. Drops indexes first, then tables. Uses `IF EXISTS` for safety.

---

## 1.3. sqlc Queries — Initial Surface

**File:** `database/queries/racepack.sql`

### 1.3.1. Counter queries

```sql
-- name: CreateRacepackCounter :one
INSERT INTO racepack_counters (organization_id, event_id, name, location, active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListRacepackCountersByEvent :many
SELECT * FROM racepack_counters WHERE event_id = $1 ORDER BY name ASC;

-- name: GetRacepackCounterByID :one
SELECT * FROM racepack_counters WHERE id = $1;

-- name: UpdateRacepackCounter :one
UPDATE racepack_counters
SET name = $2, location = $3, active = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetRacepackCounterActive :one
UPDATE racepack_counters
SET active = $2, updated_at = now()
WHERE id = $1
RETURNING *;
```

### 1.3.2. Slot queries

```sql
-- name: CreateRacepackPickupSlot :one
INSERT INTO racepack_pickup_slots (organization_id, event_id, name, pickup_date, start_time, end_time, capacity)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListRacepackPickupSlotsByEvent :many
SELECT * FROM racepack_pickup_slots
WHERE event_id = $1
ORDER BY pickup_date ASC, start_time ASC;

-- name: GetRacepackPickupSlotByID :one
SELECT * FROM racepack_pickup_slots WHERE id = $1;

-- name: IncrementRacepackPickupSlotReserved :one
-- Atomic capacity check. Returns NULL if capacity would be exceeded.
UPDATE racepack_pickup_slots
SET reserved_count = reserved_count + 1, updated_at = now()
WHERE id = $1 AND reserved_count < capacity
RETURNING *;

-- name: DecrementRacepackPickupSlotReserved :exec
UPDATE racepack_pickup_slots
SET reserved_count = reserved_count - 1, updated_at = now()
WHERE id = $1 AND reserved_count > 0;

-- name: UpdateRacepackPickupSlot :one
UPDATE racepack_pickup_slots
SET name = $2, pickup_date = $3, start_time = $4, end_time = $5,
    capacity = $6, active = $7, updated_at = now()
WHERE id = $1
RETURNING *;
```

### 1.3.3. Pickup record queries

```sql
-- name: CreateRacepackPickupRecord :one
INSERT INTO racepack_pickup_records (
    organization_id, event_id, ticket_id, participant_id, bib_number,
    counter_id, staff_id, pickup_method, notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetRacepackPickupRecordByTicket :one
SELECT * FROM racepack_pickup_records
WHERE ticket_id = $1 AND status = 'PICKED_UP';

-- name: ListRacepackPickupRecordsByEvent :many
SELECT * FROM racepack_pickup_records
WHERE event_id = $1
ORDER BY pickup_timestamp DESC
LIMIT $2 OFFSET $3;

-- name: CountRacepackPickupRecordsByCounter :many
SELECT counter_id, COUNT(*) AS pickup_count
FROM racepack_pickup_records
WHERE event_id = $1 AND pickup_timestamp >= $2 AND pickup_timestamp < $3
GROUP BY counter_id;

-- name: CountRacepackPickupRecordsByEvent :one
SELECT COUNT(*) FROM racepack_pickup_records
WHERE event_id = $1 AND status = 'PICKED_UP';
```

### 1.3.4. Proxy authorization queries

```sql
-- name: CreateRacepackProxyAuthorization :one
INSERT INTO racepack_proxy_authorizations (
    organization_id, event_id, ticket_id, pickup_record_id,
    proxy_name, proxy_phone, proxy_identity, authorization_document, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListRacepackProxyAuthorizationsByTicket :many
SELECT * FROM racepack_proxy_authorizations
WHERE ticket_id = $1
ORDER BY created_at DESC;

-- name: GetRacepackProxyAuthorizationByID :one
SELECT * FROM racepack_proxy_authorizations WHERE id = $1;
```

### 1.3.5. Problem case queries

```sql
-- name: CreateRacepackProblemCase :one
INSERT INTO racepack_problem_cases (
    organization_id, event_id, ticket_id, participant_id, status, reason, created_by
) VALUES ($1, $2, $3, $4, 'OPEN', $5, $6)
RETURNING *;

-- name: UpdateRacepackProblemCaseStatus :one
UPDATE racepack_problem_cases
SET status = $2,
    resolution = CASE WHEN $2 IN ('RESOLVED','ESCALATED') THEN $3 ELSE resolution END,
    resolved_by = CASE WHEN $2 IN ('RESOLVED','ESCALATED') THEN $4 ELSE resolved_by END,
    resolved_at = CASE WHEN $2 IN ('RESOLVED','ESCALATED') THEN now() ELSE resolved_at END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListRacepackProblemCasesByEvent :many
SELECT * FROM racepack_problem_cases
WHERE event_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetRacepackProblemCaseByID :one
SELECT * FROM racepack_problem_cases WHERE id = $1;
```

---

## 1.4. RBAC Extension — Migration 00051

**File:** `database/migrations/00051_extend_racepack_rbac.{up,down}.sql`

Two new permissions are added for granular control. Existing `racepack.scan` and `racepack.manage` (from migration 00007) remain.

```sql
-- +goose Up
INSERT INTO permissions (key, description) VALUES
    ('racepack.execute',     'Execute a pickup (confirm or proxy) at a counter'),
    ('racepack.problemdesk', 'Open and resolve problem desk cases')
ON CONFLICT (key) DO NOTHING;

-- Grant racepack.execute + racepack.problemdesk to the existing Racepack Staff role.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug = 'racepack-staff'
  AND p.key IN ('racepack.execute', 'racepack.problemdesk')
ON CONFLICT DO NOTHING;

-- Grant the same to Manager (organizers managing their event).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug = 'manager'
  AND p.key IN ('racepack.manage', 'racepack.execute', 'racepack.problemdesk')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key IN ('racepack.execute','racepack.problemdesk'));
DELETE FROM permissions WHERE key IN ('racepack.execute','racepack.problemdesk');
```

### 1.4.1. RBAC Matrix (final)

| Permission | Owner | Manager | Racepack Staff | Customer Service | Finance |
|---|---|---|---|---|---|
| `racepack.scan` (legacy, unused) | ✓ | — | ✓ | — | — |
| `racepack.manage` (slot+counter CRUD) | ✓ | ✓ | ✓ | — | — |
| `racepack.execute` (pickup confirmation) | ✓ | ✓ | ✓ | — | — |
| `racepack.problemdesk` (case lifecycle) | ✓ | ✓ | ✓ | — | — |

Note: `racepack.scan` is preserved for backward compatibility with the existing seed but the actual scanner role will use `racepack.execute`. Future cleanup may remove `racepack.scan` if confirmed unused.

---

## 1.5. File Structure (full plan)

```
services/api/internal/modules/racepack/
├── model.go                       # Status / method / problem-case constants + typed errors
├── bib_lookup.go                  # Read-only helper to fetch bib_number for a ticket
├── order_lookup.go                # Read-only helper to fetch order status for a ticket
├── eligibility.go                 # CanPickup() — single source of truth (M1)
├── slot_service.go                # Slot CRUD + capacity logic (M3)
├── counter_service.go             # Counter CRUD (M3)
├── pickup_service.go              # Pickup execution + audit emission (M3)
├── proxy_service.go               # Proxy authorization (M3)
├── problem_service.go             # Problem desk case lifecycle (M3)
├── dashboard_service.go           # Aggregate summary for organizer view (M3)
├── repository.go                  # Repository interface + sqlcRepo (M2)
├── slot_repo.go                   # Slot-specific repository extensions
├── pickup_repo.go                 # Pickup-record repo with FOR UPDATE locking helper
├── handler.go                     # HTTP handlers (M4)
├── routes.go                      # chi route registration (M4)
├── dto.go                         # JSON shapes (M4)
├── errors.go                      # sentinel errors
└── tests/
    ├── eligibility_test.go        # CanPickup() table tests (M1)
    ├── slot_service_test.go       # Capacity logic (M3)
    ├── pickup_service_test.go     # Eligibility + duplicate prevention + audit (M3)
    ├── proxy_service_test.go      # (M3)
    ├── problem_service_test.go    # (M3)
    └── concurrency_test.go        # Parallel pickup attempts (M7)

apps/web/src/
├── lib/
│   └── racepack.ts                # TS client for racepack APIs (M5)
├── components/svelte/racepack/
│   ├── SlotManager.svelte         # Organizer: CRUD slots + capacity display (M5)
│   ├── CounterManager.svelte      # Organizer: CRUD counters + active toggle (M5)
│   ├── PickupDashboard.svelte     # Organizer: live stats, polling (M5)
│   ├── ProblemDeskBoard.svelte    # Staff: open/resolve cases (M5)
│   └── ProxyApprovalModal.svelte  # Staff: confirm proxy identity (M5)
└── pages/org/[orgId]/events/[eventId]/
    ├── racepack-slots.astro       # <SlotManager client:load /> (M5)
    ├── racepack-counters.astro    # <CounterManager client:load /> (M5)
    ├── racepack-dashboard.astro   # <PickupDashboard client:load /> (M5)
    └── racepack-problem-desk.astro # <ProblemDeskBoard client:load /> (M5)
```

---

## 1.6. Tasks (Part 1 only)

### Task 1.1 — Write migration 00050 up

**Files:**
- Create: `database/migrations/00050_create_racepack_pickup.up.sql`

- [ ] **Step 1: Create the file with all 5 CREATE TABLE statements and their indexes**

Content matches sections 1.2.1 through 1.2.5 above (the `CREATE TABLE racepack_counters`, `racepack_pickup_slots`, `racepack_pickup_records`, `racepack_proxy_authorizations`, `racepack_problem_cases` blocks plus their indexes). Wrap everything in `-- +goose Up` at the top.

- [ ] **Step 2: Verify SQL is syntactically valid by reading**

Run: `cat database/migrations/00050_create_racepack_pickup.up.sql | head -20`
Expected: First line is `-- +goose Up`, followed by the first CREATE TABLE.

- [ ] **Step 3: Commit**

```bash
git add database/migrations/00050_create_racepack_pickup.up.sql
git commit -m "feat(phase14): migration 00050 - racepack_pickup tables (up)"
```

### Task 1.2 — Write migration 00050 down

**Files:**
- Create: `database/migrations/00050_create_racepack_pickup.down.sql`

- [ ] **Step 1: Create the file with DROP statements in reverse order**

Wrap in `-- +goose Down` at the top. Drop order:
1. Drop indexes on `racepack_problem_cases`
2. `DROP TABLE IF EXISTS racepack_problem_cases;`
3. Drop indexes on `racepack_proxy_authorizations`
4. `DROP TABLE IF EXISTS racepack_proxy_authorizations;`
5. Drop indexes on `racepack_pickup_records` (including the unique partial index)
6. `DROP TABLE IF EXISTS racepack_pickup_records;`
7. Drop indexes on `racepack_pickup_slots`
8. `DROP TABLE IF EXISTS racepack_pickup_slots;`
9. Drop indexes on `racepack_counters`
10. `DROP TABLE IF EXISTS racepack_counters;`

- [ ] **Step 2: Commit**

```bash
git add database/migrations/00050_create_racepack_pickup.down.sql
git commit -m "feat(phase14): migration 00050 - racepack_pickup tables (down)"
```

### Task 1.3 — Write sqlc queries for racepack

**Files:**
- Create: `database/queries/racepack.sql`

- [ ] **Step 1: Create the file with all 19 query directives from section 1.3**

Order: counter queries → slot queries → pickup record queries → proxy queries → problem case queries. Each query must follow sqlc v2 conventions (`-- name: VerbObject :returnclass`, positional `$1, $2` parameters).

- [ ] **Step 2: Commit (do NOT regen yet — that's Task 1.5)**

```bash
git add database/queries/racepack.sql
git commit -m "feat(phase14): sqlc queries for racepack module"
```

### Task 1.4 — Run sqlc regen

**Files:**
- Modify: `services/api/internal/db/racepack.sql.go` (generated)
- Modify: `services/api/internal/db/models.go` (generated — adds 5 new structs)

- [ ] **Step 1: Run sqlc**

Run: `cd services/api && $HOME/go/bin/sqlc generate`
Expected: clean output, no errors. Files `services/api/internal/db/racepack.sql.go` and updated `services/api/internal/db/models.go` should appear.

- [ ] **Step 2: Verify generated methods exist**

Run: `grep -E "CreateRacepackCounter|IncrementRacepackPickupSlotReserved|GetRacepackPickupRecordByTicket" services/api/internal/db/racepack.sql.go | head -10`
Expected: at least 3 matches, one per query.

- [ ] **Step 3: Verify build still passes**

Run: `cd services/api && go build ./...`
Expected: clean exit, no errors.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/db/
git commit -m "feat(phase14): regen sqlc - racepack queries"
```

### Task 1.5 — Write RBAC extension migration 00051

**Files:**
- Create: `database/migrations/00051_extend_racepack_rbac.up.sql`
- Create: `database/migrations/00051_extend_racepack_rbac.down.sql`

- [ ] **Step 1: Create the .up.sql file with content from section 1.4**

Wrap in `-- +goose Up` at the top. Use `ON CONFLICT DO NOTHING` for idempotency.

- [ ] **Step 2: Create the .down.sql file**

Wrap in `-- +goose Down`. Reverse order:
1. `DELETE FROM role_permissions WHERE permission_id IN (...)` — both new perms
2. `DELETE FROM permissions WHERE key IN ('racepack.execute','racepack.problemdesk');`

- [ ] **Step 3: Commit**

```bash
git add database/migrations/00051_extend_racepack_rbac.up.sql database/migrations/00051_extend_racepack_rbac.down.sql
git commit -m "feat(phase14): RBAC extension - racepack.execute + racepack.problemdesk"
```

### Task 1.6 — Verify migrations apply cleanly against the test database

**Files:** none (verification only)

- [ ] **Step 1: Apply migrations**

Run: `make test-db-setup`
Expected: goose output shows both 00050 and 00051 applied. No errors.

- [ ] **Step 2: Verify tables exist**

Run: `psql "$TEST_DATABASE_URL" -c "\dt racepack_*"`
Expected: 5 tables listed: `racepack_counters`, `racepack_pickup_records`, `racepack_pickup_slots`, `racepack_problem_cases`, `racepack_proxy_authorizations`.

- [ ] **Step 3: Verify unique partial index exists**

Run: `psql "$TEST_DATABASE_URL" -c "\d racepack_pickup_records"`
Expected: includes index `uniq_racepack_pickup_records_ticket_active` with `WHERE` clause.

- [ ] **Step 4: Verify permissions seeded**

Run: `psql "$TEST_DATABASE_URL" -c "SELECT key FROM permissions WHERE key LIKE 'racepack.%' ORDER BY key;"`
Expected: 4 rows: `racepack.execute`, `racepack.manage`, `racepack.problemdesk`, `racepack.scan`.

- [ ] **Step 5: Verify Racepack Staff has the 4 permissions**

Run: `psql "$TEST_DATABASE_URL" -c "SELECT p.key FROM role_permissions rp JOIN roles r ON r.id=rp.role_id JOIN permissions p ON p.id=rp.permission_id WHERE r.slug='racepack-staff' ORDER BY p.key;"`
Expected: 4 rows: `racepack.execute`, `racepack.manage`, `racepack.problemdesk`, `racepack.scan`.

- [ ] **Step 6: Run down + up cycle to confirm reversibility**

Run:
```bash
goose -dir database/migrations postgres "$TEST_DATABASE_URL" down
make test-db-setup
```
Expected: goose reports 00051 and 00050 down successfully, then up successfully. Tables and indexes come back.

---

## 1.7. Exit Criteria for Part 1

Part 1 is complete when:

- [ ] Migration 00050 up + down exist and apply cleanly
- [ ] Migration 00051 up + down exist and apply cleanly
- [ ] sqlc regen produces 19 query methods in `services/api/internal/db/racepack.sql.go`
- [ ] 5 new structs appear in `services/api/internal/db/models.go`
- [ ] `go build ./...` passes after regen
- [ ] 5 tables exist in the test database
- [ ] `uniq_racepack_pickup_records_ticket_active` partial unique index exists
- [ ] All 4 `racepack.*` permissions exist
- [ ] Racepack Staff role holds all 4 permissions
- [ ] Down + up cycle is reversible

---

## 1.8. What's NOT in Part 1

The following are explicitly deferred:

- Domain logic (`CanPickup`, state machine) — **Part 2**
- Repository Go interface — **Part 2**
- Service layer — **Part 3**
- HTTP handlers — **Part 4**
- Frontend — **Part 5**
- Tests beyond migration smoke — **Part 6**
- Phase 15 Scanner — separate phase

---

## 1.9. Open Questions for Approval

These decisions were locked in by the directive. Listed here so reviewer can flag any disagreement before Part 2.

| # | Decision | Source |
|---|---|---|
| Q1 | Reuse existing ticket `used_at` column for pickup timestamp? | **No** — racepack owns its own pickup record. `used_at` remains reserved for future Check-In (Phase 15+) |
| Q2 | Racepack Staff gets both `execute` and `problemdesk`? | **Yes** — single staff role covers both counter and problem desk workflows |
| Q3 | Append-only enforcement at DB or application level? | **Application level** (no UPDATE/DELETE in repo). DB trigger deferred to governance phase |
| Q4 | Slot `reserved_count` capacity enforcement? | **Atomic `UPDATE ... WHERE reserved_count < capacity`** — single SQL statement is the guard |
| Q5 | Use existing `tickets.bib_number` column or duplicate into pickup_records.bib_number? | **Duplicate into pickup_records.bib_number** — pickup record is immutable, must capture BIB at the moment of pickup even if ticket's BIB later changes (it shouldn't, but defense in depth) |

---

**End of Part 1. Awaiting approval before proceeding to Part 2 (Domain Logic + Repository).**