# Phase 10 Part 1: Foundation — Lifecycle Engine, Waitlist Engine, AccessPool

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lay the database and service foundations for Phase 10: Registration Lifecycle Engine, Waitlist Engine, and AccessPool (RESERVED type only) — all behind feature flags, no UX changes, existing tests unchanged.

**Architecture:** Three new modules (`lifecycle`, `waitlist`, `access`) with their own DB tables, sqlc queries, model constants, repository interfaces, and skeleton services. The Registration Access Engine in `registration/gate.go` gains a `LifecycleChecker` interface — existing queue behavior unchanged. `AccessPool` introduces atomic slot operations (`ReserveSlot`/`ConsumeSlot`/`ReleaseSlot`) that ballot (Part 2) and Phase 11 build on.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, existing audit/apperr/authctx patterns. Module path: `github.com/varin/ivyticketing/services/api`.

---

### Task 1: Migrations 00031–00036

**Files:**
- Create: `database/migrations/00031_create_registration_lifecycle.sql`
- Create: `database/migrations/00032_create_lifecycle_phases.sql`
- Create: `database/migrations/00033_create_waitlists.sql`
- Create: `database/migrations/00034_create_waitlist_entries.sql`
- Create: `database/migrations/00035_create_access_pools.sql`
- Create: `database/migrations/00036_create_access_grants.sql`

- [ ] **Step 1: Write 00031_create_registration_lifecycle.sql**

```sql
-- +goose Up
CREATE TABLE registration_lifecycles (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id             uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id          uuid NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    status               text NOT NULL DEFAULT 'DRAFT'
                             CHECK (status IN ('DRAFT','ACTIVE','PAUSED','COMPLETED','CANCELLED')),
    current_phase_index  integer NOT NULL DEFAULT 0,
    created_by           uuid NOT NULL REFERENCES users(id),
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX registration_lifecycles_active_idx
    ON registration_lifecycles(event_id, category_id)
    WHERE status NOT IN ('COMPLETED','CANCELLED');

-- +goose Down
DROP TABLE registration_lifecycles;
```

- [ ] **Step 2: Write 00032_create_lifecycle_phases.sql**

```sql
-- +goose Up
CREATE TABLE lifecycle_phases (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    lifecycle_id      uuid NOT NULL REFERENCES registration_lifecycles(id) ON DELETE CASCADE,
    phase_index       integer NOT NULL,
    registration_mode text NOT NULL,
    label             text NOT NULL,
    opens_at          timestamptz,
    closes_at         timestamptz,
    capacity_override integer CHECK (capacity_override > 0),
    auto_advance      boolean NOT NULL DEFAULT true,
    status            text NOT NULL DEFAULT 'PENDING'
                          CHECK (status IN ('PENDING','ACTIVE','COMPLETED','SKIPPED')),
    activated_at      timestamptz,
    completed_at      timestamptz,
    UNIQUE (lifecycle_id, phase_index)
);
CREATE INDEX lifecycle_phases_auto_advance_idx
    ON lifecycle_phases(status, closes_at)
    WHERE status = 'ACTIVE' AND auto_advance = true;

-- +goose Down
DROP TABLE lifecycle_phases;
```

- [ ] **Step 3: Write 00033_create_waitlists.sql**

```sql
-- +goose Up
CREATE TABLE waitlists (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id                uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id             uuid NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    pool_id                 uuid,
    mode                    text NOT NULL DEFAULT 'FIFO'
                                CHECK (mode IN ('FIFO','RANDOMIZED','HYBRID')),
    status                  text NOT NULL DEFAULT 'ACTIVE'
                                CHECK (status IN ('ACTIVE','PAUSED','CLOSED')),
    max_promotion_batch     integer NOT NULL DEFAULT 10,
    promotion_window_hours  integer NOT NULL DEFAULT 48,
    auto_promote            boolean NOT NULL DEFAULT true,
    seed                    text,
    created_at              timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX waitlists_category_idx ON waitlists(event_id, category_id, status);

-- +goose Down
DROP TABLE waitlists;
```

- [ ] **Step 4: Write 00034_create_waitlist_entries.sql**

```sql
-- +goose Up
CREATE TABLE waitlist_entries (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    waitlist_id             uuid NOT NULL REFERENCES waitlists(id) ON DELETE CASCADE,
    participant_id          uuid NOT NULL REFERENCES users(id),
    event_id                uuid NOT NULL REFERENCES events(id),
    category_id             uuid NOT NULL REFERENCES event_categories(id),
    source                  text NOT NULL CHECK (source IN ('BALLOT','QUOTA_RELEASE','MANUAL')),
    source_ref_id           uuid,
    status                  text NOT NULL DEFAULT 'WAITING'
                                CHECK (status IN ('WAITING','PROMOTED','EXPIRED','WITHDRAWN')),
    rank                    bigint NOT NULL,
    notified_at             timestamptz,
    promoted_at             timestamptz,
    access_grant_id         uuid,
    promotion_window_hours  integer,
    created_at              timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX waitlist_entries_active_idx
    ON waitlist_entries(waitlist_id, participant_id)
    WHERE status NOT IN ('WITHDRAWN','EXPIRED');
CREATE INDEX waitlist_entries_rank_idx ON waitlist_entries(waitlist_id, status, rank)
    WHERE status = 'WAITING';

-- +goose Down
DROP TABLE waitlist_entries;
```

- [ ] **Step 5: Write 00035_create_access_pools.sql**

```sql
-- +goose Up
CREATE TABLE access_pools (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id             uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id                    uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id                 uuid NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    pool_type                   text NOT NULL
                                    CHECK (pool_type IN ('RESERVED','COMMUNITY','CORPORATE',
                                           'SPONSOR','VIP','PARTNER','PRIORITY','ELITE')),
    name                        text NOT NULL,
    total_slots                 integer NOT NULL CHECK (total_slots > 0),
    reserved_slots              integer NOT NULL DEFAULT 0,
    used_slots                  integer NOT NULL DEFAULT 0,
    released_slots              integer NOT NULL DEFAULT 0,
    owner_account_id            uuid,
    is_visible_to_participants  boolean NOT NULL DEFAULT false,
    eligibility_rule            jsonb,
    valid_from                  timestamptz,
    valid_until                 timestamptz,
    created_by                  uuid NOT NULL REFERENCES users(id),
    created_at                  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT access_pools_slots_check
        CHECK (reserved_slots + used_slots <= total_slots),
    CONSTRAINT access_pools_non_negative
        CHECK (reserved_slots >= 0 AND used_slots >= 0 AND released_slots >= 0)
);
CREATE INDEX access_pools_category_idx ON access_pools(event_id, category_id, pool_type);

-- +goose Down
DROP TABLE access_pools;
```

- [ ] **Step 6: Write 00036_create_access_grants.sql**

```sql
-- +goose Up
CREATE TABLE access_grants (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id         uuid REFERENCES access_pools(id),
    participant_id  uuid NOT NULL REFERENCES users(id),
    event_id        uuid NOT NULL REFERENCES events(id),
    category_id     uuid NOT NULL REFERENCES event_categories(id),
    code_id         uuid,
    status          text NOT NULL DEFAULT 'ACTIVE'
                        CHECK (status IN ('ACTIVE','CONSUMED','EXPIRED')),
    granted_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    consumed_at     timestamptz,
    order_id        uuid,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX access_grants_participant_idx ON access_grants(participant_id, event_id, status);
CREATE INDEX access_grants_expiry_idx ON access_grants(expires_at)
    WHERE status = 'ACTIVE';

-- +goose Down
DROP TABLE access_grants;
```

- [ ] **Step 7: Run migrations**

```bash
cd /Users/kaivy/Coding/ivyticketing
make migrate-up 2>&1
# Expected: goose: successfully migrated database to version 36
make migrate-down 2>&1
# Expected: down to version 30 (or to 0 if full rollback)
make migrate-up 2>&1
```

- [ ] **Step 8: Commit**

```bash
git add database/migrations/00031_create_registration_lifecycle.sql \
        database/migrations/00032_create_lifecycle_phases.sql \
        database/migrations/00033_create_waitlists.sql \
        database/migrations/00034_create_waitlist_entries.sql \
        database/migrations/00035_create_access_pools.sql \
        database/migrations/00036_create_access_grants.sql
git commit -m "feat(phase10): migrations 00031-00036 (lifecycle, waitlist, access_pools, access_grants)"
```

---

### Task 2: sqlc Queries

**Files:**
- Create: `database/queries/lifecycle.sql`
- Create: `database/queries/waitlist.sql`
- Create: `database/queries/access.sql`

- [ ] **Step 1: Write lifecycle.sql**

```sql
-- name: CreateLifecycle :one
INSERT INTO registration_lifecycles
    (organization_id, event_id, category_id, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLifecycleByCategory :one
SELECT * FROM registration_lifecycles
WHERE event_id = $1 AND category_id = $2
  AND status NOT IN ('COMPLETED','CANCELLED')
LIMIT 1;

-- name: ActivateLifecycle :one
UPDATE registration_lifecycles SET status = 'ACTIVE', updated_at = now()
WHERE id = $1 AND status = 'DRAFT'
RETURNING *;

-- name: UpdateLifecycleStatus :one
UPDATE registration_lifecycles SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateLifecyclePhase :one
INSERT INTO lifecycle_phases
    (lifecycle_id, phase_index, registration_mode, label, opens_at, closes_at,
     capacity_override, auto_advance)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetActivePhaseForMode :one
SELECT lp.* FROM lifecycle_phases lp
JOIN registration_lifecycles rl ON rl.id = lp.lifecycle_id
WHERE lp.lifecycle_id = $1
  AND lp.registration_mode = $2
  AND lp.status = 'ACTIVE'
  AND rl.status = 'ACTIVE'
LIMIT 1;

-- name: ListPhasesForLifecycle :many
SELECT * FROM lifecycle_phases WHERE lifecycle_id = $1 ORDER BY phase_index ASC;

-- name: UpdateLifecyclePhaseStatus :one
UPDATE lifecycle_phases
SET status = $2,
    activated_at = CASE WHEN $2 = 'ACTIVE' THEN now() ELSE activated_at END,
    completed_at = CASE WHEN $2 IN ('COMPLETED','SKIPPED') THEN now() ELSE completed_at END
WHERE id = $1
RETURNING *;

-- name: ListPhasesForAutoAdvance :many
SELECT lp.* FROM lifecycle_phases lp
JOIN registration_lifecycles rl ON rl.id = lp.lifecycle_id
WHERE lp.status = 'ACTIVE'
  AND lp.auto_advance = true
  AND lp.closes_at < now()
  AND rl.status = 'ACTIVE';

-- name: GetNextPendingPhase :one
SELECT * FROM lifecycle_phases
WHERE lifecycle_id = $1 AND status = 'PENDING'
ORDER BY phase_index ASC
LIMIT 1;
```

- [ ] **Step 2: Write waitlist.sql**

```sql
-- name: CreateWaitlist :one
INSERT INTO waitlists
    (organization_id, event_id, category_id, mode, max_promotion_batch,
     promotion_window_hours, auto_promote)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetWaitlistByCategory :one
SELECT * FROM waitlists
WHERE event_id = $1 AND category_id = $2 AND status = 'ACTIVE'
LIMIT 1;

-- name: SetWaitlistPool :exec
UPDATE waitlists SET pool_id = $2 WHERE id = $1;

-- name: SetWaitlistSeed :exec
UPDATE waitlists SET seed = $2 WHERE id = $1 AND seed IS NULL;

-- name: JoinWaitlist :one
INSERT INTO waitlist_entries
    (waitlist_id, participant_id, event_id, category_id, source, source_ref_id,
     rank, promotion_window_hours)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetWaitlistEntry :one
SELECT * FROM waitlist_entries
WHERE waitlist_id = $1 AND participant_id = $2 AND status NOT IN ('WITHDRAWN','EXPIRED')
LIMIT 1;

-- name: ListWaitingEntries :many
SELECT * FROM waitlist_entries
WHERE waitlist_id = $1 AND status = 'WAITING'
ORDER BY rank ASC
LIMIT $2;

-- name: UpdateWaitlistEntryStatus :one
UPDATE waitlist_entries
SET status = $2,
    promoted_at = CASE WHEN $2 = 'PROMOTED' THEN now() ELSE promoted_at END,
    access_grant_id = COALESCE($3, access_grant_id)
WHERE id = $1
RETURNING *;

-- name: CountWaitlistPosition :one
SELECT count(*) FROM waitlist_entries
WHERE waitlist_id = $1 AND status = 'WAITING' AND rank < $2;
```

- [ ] **Step 3: Write access.sql**

```sql
-- name: CreateAccessPool :one
INSERT INTO access_pools
    (organization_id, event_id, category_id, pool_type, name, total_slots,
     is_visible_to_participants, eligibility_rule, valid_from, valid_until, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetAccessPool :one
SELECT * FROM access_pools WHERE id = $1;

-- name: ReservePoolSlot :one
UPDATE access_pools
SET reserved_slots = reserved_slots + 1
WHERE id = $1 AND reserved_slots + used_slots < total_slots
RETURNING *;

-- name: ConsumePoolSlot :exec
UPDATE access_pools
SET reserved_slots = reserved_slots - 1, used_slots = used_slots + 1
WHERE id = $1;

-- name: ReleasePoolSlot :exec
UPDATE access_pools
SET reserved_slots = reserved_slots - 1, released_slots = released_slots + 1
WHERE id = $1;

-- name: CreateAccessGrant :one
INSERT INTO access_grants
    (pool_id, participant_id, event_id, category_id, code_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAccessGrant :one
SELECT * FROM access_grants WHERE id = $1;

-- name: GetActiveGrantForParticipant :one
SELECT * FROM access_grants
WHERE participant_id = $1 AND category_id = $2 AND status = 'ACTIVE'
ORDER BY granted_at DESC
LIMIT 1;

-- name: ExpireGrant :exec
UPDATE access_grants SET status = 'EXPIRED' WHERE id = $1;

-- name: ConsumeGrant :exec
UPDATE access_grants
SET status = 'CONSUMED', consumed_at = now(), order_id = $2
WHERE id = $1;

-- name: ListExpiredActiveGrants :many
SELECT * FROM access_grants
WHERE status = 'ACTIVE' AND expires_at < now()
LIMIT $1;
```

- [ ] **Step 4: Run sqlc**

```bash
cd /Users/kaivy/Coding/ivyticketing && make sqlc 2>&1
# Expected: Generated files in services/api/internal/db/
cd services/api && go build ./internal/db/... 2>&1
# Expected: no errors
```

- [ ] **Step 5: Commit**

```bash
git add database/queries/lifecycle.sql database/queries/waitlist.sql database/queries/access.sql
git commit -m "feat(phase10): sqlc queries for lifecycle, waitlist, access pools+grants"
```

---

### Task 3: Lifecycle Module — Model, Errors, Repository

**Files:**
- Create: `services/api/internal/modules/lifecycle/model.go`
- Create: `services/api/internal/modules/lifecycle/errors.go`
- Create: `services/api/internal/modules/lifecycle/repository.go`

- [ ] **Step 1: Write model.go**

```go
package lifecycle

const (
	StatusDraft     = "DRAFT"
	StatusActive    = "ACTIVE"
	StatusPaused    = "PAUSED"
	StatusCompleted = "COMPLETED"
	StatusCancelled = "CANCELLED"

	PhaseStatusPending   = "PENDING"
	PhaseStatusActive    = "ACTIVE"
	PhaseStatusCompleted = "COMPLETED"
	PhaseStatusSkipped   = "SKIPPED"
)

type WindowClosedReason string

const (
	ReasonWindowNotYetOpen   WindowClosedReason = "WINDOW_NOT_YET_OPEN"
	ReasonWindowExpired      WindowClosedReason = "WINDOW_EXPIRED"
	ReasonModeNotInLifecycle WindowClosedReason = "MODE_NOT_IN_LIFECYCLE"
	ReasonLifecyclePaused    WindowClosedReason = "LIFECYCLE_PAUSED"
)
```

- [ ] **Step 2: Write errors.go**

```go
package lifecycle

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrLifecycleNotFound = apperr.New(http.StatusNotFound, "LIFECYCLE_NOT_FOUND", "lifecycle not found")
	ErrLifecyclePaused   = apperr.New(http.StatusConflict, "LIFECYCLE_PAUSED", "registration is paused")
	ErrPhaseNotActive    = apperr.New(http.StatusConflict, "LIFECYCLE_PHASE_NOT_ACTIVE", "registration phase not active")
	ErrInvalidTransition = apperr.New(http.StatusConflict, "LIFECYCLE_INVALID_TRANSITION", "invalid lifecycle status transition")
	ErrAlreadyActive     = apperr.New(http.StatusConflict, "LIFECYCLE_ALREADY_ACTIVE", "only one active lifecycle per category")
)
```

- [ ] **Step 3: Write repository.go**

Read `services/api/internal/modules/abuse/repository.go` first for exact pattern. Then write:

```go
package lifecycle

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateLifecycle(ctx context.Context, arg db.CreateLifecycleParams) (db.RegistrationLifecycle, error)
	GetLifecycleByCategory(ctx context.Context, arg db.GetLifecycleByCategoryParams) (db.RegistrationLifecycle, error)
	ActivateLifecycle(ctx context.Context, id uuid.UUID) (db.RegistrationLifecycle, error)
	UpdateLifecycleStatus(ctx context.Context, arg db.UpdateLifecycleStatusParams) (db.RegistrationLifecycle, error)
	CreateLifecyclePhase(ctx context.Context, arg db.CreateLifecyclePhaseParams) (db.LifecyclePhase, error)
	GetActivePhaseForMode(ctx context.Context, arg db.GetActivePhaseForModeParams) (db.LifecyclePhase, error)
	ListPhasesForLifecycle(ctx context.Context, lifecycleID uuid.UUID) ([]db.LifecyclePhase, error)
	UpdateLifecyclePhaseStatus(ctx context.Context, arg db.UpdateLifecyclePhaseStatusParams) (db.LifecyclePhase, error)
	ListPhasesForAutoAdvance(ctx context.Context) ([]db.LifecyclePhase, error)
	GetNextPendingPhase(ctx context.Context, lifecycleID uuid.UUID) (db.LifecyclePhase, error)
}

type sqlcRepo struct{ q *db.Queries }

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

// implement each method delegating to r.q.*
```

- [ ] **Step 4: Build**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./internal/modules/lifecycle/... 2>&1
# Expected: clean
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/lifecycle/
git commit -m "feat(phase10): lifecycle module (model, errors, repository)"
```

---

### Task 4: Lifecycle Service — IsWindowOpen + Phase Management

**Files:**
- Create: `services/api/internal/modules/lifecycle/service.go`
- Create: `services/api/internal/modules/lifecycle/tests/service_test.go`

- [ ] **Step 1: Write failing tests first**

```go
// services/api/internal/modules/lifecycle/tests/service_test.go
package lifecycle_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/lifecycle"
	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

type fakeRepo struct {
	lifecycle *db.RegistrationLifecycle
	phase     *db.LifecyclePhase
}

func (f *fakeRepo) CreateLifecycle(_ context.Context, _ db.CreateLifecycleParams) (db.RegistrationLifecycle, error) {
	return db.RegistrationLifecycle{}, nil
}
func (f *fakeRepo) GetLifecycleByCategory(_ context.Context, _ db.GetLifecycleByCategoryParams) (db.RegistrationLifecycle, error) {
	if f.lifecycle == nil {
		return db.RegistrationLifecycle{}, pgx.ErrNoRows
	}
	return *f.lifecycle, nil
}
func (f *fakeRepo) ActivateLifecycle(_ context.Context, _ uuid.UUID) (db.RegistrationLifecycle, error) {
	return db.RegistrationLifecycle{}, nil
}
func (f *fakeRepo) UpdateLifecycleStatus(_ context.Context, _ db.UpdateLifecycleStatusParams) (db.RegistrationLifecycle, error) {
	return db.RegistrationLifecycle{}, nil
}
func (f *fakeRepo) CreateLifecyclePhase(_ context.Context, _ db.CreateLifecyclePhaseParams) (db.LifecyclePhase, error) {
	return db.LifecyclePhase{}, nil
}
func (f *fakeRepo) GetActivePhaseForMode(_ context.Context, _ db.GetActivePhaseForModeParams) (db.LifecyclePhase, error) {
	if f.phase == nil {
		return db.LifecyclePhase{}, pgx.ErrNoRows
	}
	return *f.phase, nil
}
func (f *fakeRepo) ListPhasesForLifecycle(_ context.Context, _ uuid.UUID) ([]db.LifecyclePhase, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateLifecyclePhaseStatus(_ context.Context, _ db.UpdateLifecyclePhaseStatusParams) (db.LifecyclePhase, error) {
	return db.LifecyclePhase{}, nil
}
func (f *fakeRepo) ListPhasesForAutoAdvance(_ context.Context) ([]db.LifecyclePhase, error) {
	return nil, nil
}
func (f *fakeRepo) GetNextPendingPhase(_ context.Context, _ uuid.UUID) (db.LifecyclePhase, error) {
	return db.LifecyclePhase{}, pgx.ErrNoRows
}

func TestIsWindowOpen_NoLifecycle_FailOpen(t *testing.T) {
	svc := lifecycle.NewService(&fakeRepo{lifecycle: nil})
	open, _, err := svc.IsWindowOpen(context.Background(), uuid.New(), registration.ModeNormal)
	if err != nil { t.Fatal(err) }
	if !open { t.Fatal("no lifecycle row should fail-open (return true)") }
}

func TestIsWindowOpen_LifecyclePaused(t *testing.T) {
	lc := &db.RegistrationLifecycle{Status: lifecycle.StatusPaused}
	svc := lifecycle.NewService(&fakeRepo{lifecycle: lc})
	open, reason, _ := svc.IsWindowOpen(context.Background(), uuid.New(), registration.ModeNormal)
	if open { t.Fatal("paused lifecycle should return false") }
	if reason != lifecycle.ReasonLifecyclePaused { t.Fatalf("want %q got %q", lifecycle.ReasonLifecyclePaused, reason) }
}

func TestIsWindowOpen_ActivePhaseForMode(t *testing.T) {
	lc := &db.RegistrationLifecycle{Status: lifecycle.StatusActive}
	ph := &db.LifecyclePhase{Status: lifecycle.PhaseStatusActive, RegistrationMode: string(registration.ModeNormal)}
	svc := lifecycle.NewService(&fakeRepo{lifecycle: lc, phase: ph})
	open, _, err := svc.IsWindowOpen(context.Background(), uuid.New(), registration.ModeNormal)
	if err != nil { t.Fatal(err) }
	if !open { t.Fatal("active phase for matching mode should return true") }
}

func TestIsWindowOpen_NoPhaseForMode(t *testing.T) {
	lc := &db.RegistrationLifecycle{Status: lifecycle.StatusActive}
	svc := lifecycle.NewService(&fakeRepo{lifecycle: lc, phase: nil})
	open, reason, _ := svc.IsWindowOpen(context.Background(), uuid.New(), registration.ModeNormal)
	if open { t.Fatal("no active phase for mode should return false") }
	if reason != lifecycle.ReasonModeNotInLifecycle { t.Fatalf("want %q got %q", lifecycle.ReasonModeNotInLifecycle, reason) }
}
```

- [ ] **Step 2: Run tests — expect FAIL (service not yet written)**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/lifecycle/tests/ -v 2>&1
# Expected: FAIL — lifecycle.NewService undefined
```

- [ ] **Step 3: Write service.go**

```go
package lifecycle

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) IsWindowOpen(ctx context.Context, categoryID uuid.UUID, mode registration.Mode) (bool, WindowClosedReason, error) {
	lc, err := s.repo.GetLifecycleByCategory(ctx, db.GetLifecycleByCategoryParams{
		EventID: ..., CategoryID: categoryID, // NOTE: need event_id — caller must pass it
	})
	// Alternative: GetLifecycleByCategoryID — add a query that takes only category_id
	if errors.Is(err, pgx.ErrNoRows) {
		return true, "", nil // fail-open
	}
	if err != nil { return false, "", err }
	if lc.Status == StatusPaused { return false, ReasonLifecyclePaused, nil }
	if lc.Status != StatusActive { return false, ReasonModeNotInLifecycle, nil }

	_, err = s.repo.GetActivePhaseForMode(ctx, db.GetActivePhaseForModeParams{
		LifecycleID:      lc.ID,
		RegistrationMode: string(mode),
	})
	if errors.Is(err, pgx.ErrNoRows) { return false, ReasonModeNotInLifecycle, nil }
	if err != nil { return false, "", err }
	return true, "", nil
}

func (s *Service) PauseLifecycle(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.UpdateLifecycleStatus(ctx, db.UpdateLifecycleStatusParams{ID: id, Status: StatusPaused})
	return err
}

func (s *Service) ResumeLifecycle(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.UpdateLifecycleStatus(ctx, db.UpdateLifecycleStatusParams{ID: id, Status: StatusActive})
	return err
}

func (s *Service) CompletePhase(ctx context.Context, phaseID uuid.UUID) error {
	_, err := s.repo.UpdateLifecyclePhaseStatus(ctx, db.UpdateLifecyclePhaseStatusParams{
		ID: phaseID, Status: PhaseStatusCompleted,
	})
	return err
}

func (s *Service) AdvanceToNextPhase(ctx context.Context, lifecycleID uuid.UUID) error {
	next, err := s.repo.GetNextPendingPhase(ctx, lifecycleID)
	if errors.Is(err, pgx.ErrNoRows) {
		// No more phases — mark lifecycle completed
		_, err = s.repo.UpdateLifecycleStatus(ctx, db.UpdateLifecycleStatusParams{
			ID: lifecycleID, Status: StatusCompleted,
		})
		return err
	}
	if err != nil { return err }
	_, err = s.repo.UpdateLifecyclePhaseStatus(ctx, db.UpdateLifecyclePhaseStatusParams{
		ID: next.ID, Status: PhaseStatusActive,
	})
	return err
}
```

**Note:** `GetLifecycleByCategory` needs category_id. Adjust the sqlc query in lifecycle.sql to accept only `category_id` if `event_id` isn't available at the RAE call site. Read `registration/gate.go:Admit()` — it receives `categoryID` — so add a query `GetLifecycleByCategoryID :one SELECT ... WHERE category_id = $1 AND status NOT IN ('COMPLETED','CANCELLED') LIMIT 1` and regenerate sqlc.

- [ ] **Step 4: Run tests — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/lifecycle/tests/ -v -race 2>&1
# Expected: PASS — 4 tests
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/lifecycle/service.go services/api/internal/modules/lifecycle/tests/
git commit -m "feat(phase10): lifecycle service (IsWindowOpen, pause/resume, phase advance)"
```

---

### Task 5: LifecycleAdvancer Background Job

**Files:**
- Create: `services/api/internal/modules/lifecycle/advancer.go`
- Create: `services/api/internal/modules/lifecycle/tests/advancer_test.go`

- [ ] **Step 1: Write failing test**

```go
// services/api/internal/modules/lifecycle/tests/advancer_test.go
package lifecycle_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/lifecycle"
)

type advancerRepo struct {
	fakeRepo
	advanceCalled int64
	phases        []db.LifecyclePhase
}

func (r *advancerRepo) ListPhasesForAutoAdvance(_ context.Context) ([]db.LifecyclePhase, error) {
	return r.phases, nil
}
func (r *advancerRepo) UpdateLifecyclePhaseStatus(_ context.Context, p db.UpdateLifecyclePhaseStatusParams) (db.LifecyclePhase, error) {
	atomic.AddInt64(&r.advanceCalled, 1)
	return db.LifecyclePhase{}, nil
}

func TestAdvancer_ConcurrentRunIdempotent(t *testing.T) {
	ph := db.LifecyclePhase{Status: "ACTIVE"}
	repo := &advancerRepo{phases: []db.LifecyclePhase{ph}}
	svc := lifecycle.NewService(repo)
	adv := lifecycle.NewAdvancer(svc)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = adv.Run(context.Background()) }()
	}
	wg.Wait()
	// SELECT FOR UPDATE makes this safe — each phase processed once
	if repo.advanceCalled > 1 {
		t.Fatalf("phase should be advanced exactly once, got %d", repo.advanceCalled)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/lifecycle/tests/ -run TestAdvancer -v 2>&1
# Expected: FAIL — lifecycle.NewAdvancer undefined
```

- [ ] **Step 3: Write advancer.go**

```go
package lifecycle

import "context"

type Advancer struct{ svc *Service }

func NewAdvancer(svc *Service) *Advancer { return &Advancer{svc: svc} }

func (a *Advancer) Run(ctx context.Context) error {
	phases, err := a.svc.repo.ListPhasesForAutoAdvance(ctx)
	if err != nil { return err }
	for _, phase := range phases {
		if err := a.svc.CompletePhase(ctx, phase.ID); err != nil { continue }
		if err := a.svc.AdvanceToNextPhase(ctx, phase.LifecycleID); err != nil { continue }
	}
	return nil
}
```

- [ ] **Step 4: Run test — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/lifecycle/tests/ -race -v 2>&1
# Expected: all PASS
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/lifecycle/advancer.go services/api/internal/modules/lifecycle/tests/advancer_test.go
git commit -m "feat(phase10): lifecycle advancer background job"
```

---

### Task 6: Waitlist Module — Model, Errors, Repository

**Files:**
- Create: `services/api/internal/modules/waitlist/model.go`
- Create: `services/api/internal/modules/waitlist/errors.go`
- Create: `services/api/internal/modules/waitlist/repository.go`

- [ ] **Step 1: Write model.go**

```go
package waitlist

const (
	StatusWaiting  = "WAITING"
	StatusPromoted = "PROMOTED"
	StatusExpired  = "EXPIRED"
	StatusWithdrawn = "WITHDRAWN"

	ModeFIFO       = "FIFO"
	ModeRandomized = "RANDOMIZED"
	ModeHybrid     = "HYBRID"

	SourceBallot       = "BALLOT"
	SourceQuotaRelease = "QUOTA_RELEASE"
	SourceManual       = "MANUAL"
)
```

- [ ] **Step 2: Write errors.go**

```go
package waitlist

import (
	"net/http"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrWaitlistNotFound  = apperr.New(http.StatusNotFound, "WAITLIST_NOT_FOUND", "waitlist not found")
	ErrAlreadyOnWaitlist = apperr.New(http.StatusConflict, "ALREADY_ON_WAITLIST", "already on this waitlist")
	ErrNotOnWaitlist     = apperr.New(http.StatusNotFound, "NOT_ON_WAITLIST", "not on this waitlist")
	ErrWaitlistClosed    = apperr.New(http.StatusConflict, "WAITLIST_CLOSED", "waitlist is closed")
)
```

- [ ] **Step 3: Write repository.go** (follow abuse/repository.go pattern exactly)

- [ ] **Step 4: Build**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./internal/modules/waitlist/... 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/waitlist/
git commit -m "feat(phase10): waitlist module (model, errors, repository)"
```

---

### Task 7: Waitlist Service — Join, PromoteBatch, Expire, Withdraw

**Files:**
- Create: `services/api/internal/modules/waitlist/rank.go`
- Create: `services/api/internal/modules/waitlist/service.go`
- Create: `services/api/internal/modules/waitlist/tests/service_test.go`

- [ ] **Step 1: Write rank.go**

```go
package waitlist

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

func FIFORank(joinedAt time.Time) int64 { return joinedAt.UnixMicro() }

func RandomizedRank(seed string, participantID uuid.UUID) int64 {
	h := sha256.Sum256([]byte(seed + "|" + participantID.String()))
	return int64(binary.BigEndian.Uint64(h[:8]))
}
```

- [ ] **Step 2: Write failing tests**

```go
// services/api/internal/modules/waitlist/tests/service_test.go
package waitlist_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/waitlist"
)

var errExhausted = errors.New("pool exhausted")

type fakeWaitlistRepo struct {
	wl      db.Waitlist
	entries []db.WaitlistEntry
	joined  []db.WaitlistEntry
}
func (r *fakeWaitlistRepo) GetWaitlistByCategory(_ context.Context, _ db.GetWaitlistByCategoryParams) (db.Waitlist, error) {
	return r.wl, nil
}
func (r *fakeWaitlistRepo) JoinWaitlist(_ context.Context, arg db.JoinWaitlistParams) (db.WaitlistEntry, error) {
	e := db.WaitlistEntry{ID: uuid.New(), Rank: arg.Rank}
	r.joined = append(r.joined, e)
	return e, nil
}
func (r *fakeWaitlistRepo) ListWaitingEntries(_ context.Context, arg db.ListWaitingEntriesParams) ([]db.WaitlistEntry, error) {
	return r.entries, nil
}
func (r *fakeWaitlistRepo) UpdateWaitlistEntryStatus(_ context.Context, _ db.UpdateWaitlistEntryStatusParams) (db.WaitlistEntry, error) {
	return db.WaitlistEntry{}, nil
}
func (r *fakeWaitlistRepo) GetWaitlistEntry(_ context.Context, _ db.GetWaitlistEntryParams) (db.WaitlistEntry, error) {
	return db.WaitlistEntry{}, pgx.ErrNoRows
}
func (r *fakeWaitlistRepo) SetWaitlistSeed(_ context.Context, _ db.SetWaitlistSeedParams) error { return nil }
func (r *fakeWaitlistRepo) CountWaitlistPosition(_ context.Context, _ db.CountWaitlistPositionParams) (int64, error) { return 0, nil }
func (r *fakeWaitlistRepo) CreateWaitlist(_ context.Context, _ db.CreateWaitlistParams) (db.Waitlist, error) { return db.Waitlist{}, nil }
func (r *fakeWaitlistRepo) SetWaitlistPool(_ context.Context, _ db.SetWaitlistPoolParams) error { return nil }

type fakePoolReserver struct{ n int; exhausted bool }
func (f *fakePoolReserver) ReserveSlot(_ context.Context, _ uuid.UUID) error {
	if f.exhausted || f.n <= 0 { return errExhausted }
	f.n--
	return nil
}
func (f *fakePoolReserver) CreateGrant(_ context.Context, _, _, _, _ uuid.UUID, _ time.Time) (uuid.UUID, error) {
	return uuid.New(), nil
}

func TestPromoteBatch_StopsAtExhaustion(t *testing.T) {
	entries := []db.WaitlistEntry{
		{ID: uuid.New(), Rank: 1},
		{ID: uuid.New(), Rank: 2},
		{ID: uuid.New(), Rank: 3},
	}
	reserver := &fakePoolReserver{n: 1} // only 1 slot
	repo := &fakeWaitlistRepo{entries: entries}
	svc := waitlist.NewService(repo, reserver)
	promoted, err := svc.PromoteBatch(context.Background(), uuid.New())
	if err != nil { t.Fatal(err) }
	if len(promoted) != 1 { t.Fatalf("want 1 promoted, got %d", len(promoted)) }
}

func TestJoin_FIFORankIsMonotonicallyIncreasing(t *testing.T) {
	repo := &fakeWaitlistRepo{wl: db.Waitlist{Mode: waitlist.ModeFIFO}}
	reserver := &fakePoolReserver{n: 10}
	svc := waitlist.NewService(repo, reserver)
	wlID := uuid.New()
	pid1, pid2 := uuid.New(), uuid.New()
	e1, _ := svc.Join(context.Background(), wlID, pid1, waitlist.SourceManual, nil)
	time.Sleep(time.Microsecond)
	e2, _ := svc.Join(context.Background(), wlID, pid2, waitlist.SourceManual, nil)
	if e1.Rank >= e2.Rank { t.Fatal("FIFO rank should be monotonically increasing") }
}
```

- [ ] **Step 3: Run tests — expect FAIL**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/waitlist/tests/ -v 2>&1
```

- [ ] **Step 4: Write service.go**

```go
package waitlist

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type PoolReserver interface {
	ReserveSlot(ctx context.Context, poolID uuid.UUID) error
	CreateGrant(ctx context.Context, poolID, participantID, eventID, categoryID uuid.UUID, expiresAt time.Time) (uuid.UUID, error)
}

type Service struct {
	repo     Repository
	reserver PoolReserver
}

func NewService(repo Repository, reserver PoolReserver) *Service {
	return &Service{repo: repo, reserver: reserver}
}

func (s *Service) Join(ctx context.Context, waitlistID, participantID uuid.UUID, source string, sourceRefID *uuid.UUID) (db.WaitlistEntry, error) {
	rank := FIFORank(time.Now())
	return s.repo.JoinWaitlist(ctx, db.JoinWaitlistParams{
		WaitlistID:    waitlistID,
		ParticipantID: participantID,
		Source:        source,
		Rank:          rank,
	})
}

func (s *Service) PromoteBatch(ctx context.Context, waitlistID uuid.UUID) ([]db.WaitlistEntry, error) {
	wl, err := s.repo.GetWaitlistByCategory(ctx, db.GetWaitlistByCategoryParams{})
	// Note: PromoteBatch takes waitlistID directly, not event+category
	// Adjust query: add GetWaitlistByID query to access.sql
	_ = wl
	entries, err := s.repo.ListWaitingEntries(ctx, db.ListWaitingEntriesParams{
		WaitlistID: waitlistID, Limit: 10,
	})
	if err != nil { return nil, err }

	var promoted []db.WaitlistEntry
	for _, entry := range entries {
		if err := s.reserver.ReserveSlot(ctx, uuid.Nil); err != nil {
			if errors.Is(err, errPoolExhausted) { break }
			continue
		}
		grantID, err := s.reserver.CreateGrant(ctx, uuid.Nil, entry.ParticipantID,
			entry.EventID, entry.CategoryID, time.Now().Add(48*time.Hour))
		if err != nil { continue }
		updated, err := s.repo.UpdateWaitlistEntryStatus(ctx, db.UpdateWaitlistEntryStatusParams{
			ID: entry.ID, Status: StatusPromoted, AccessGrantID: &grantID,
		})
		if err != nil { continue }
		promoted = append(promoted, updated)
	}
	return promoted, nil
}

var errPoolExhausted = errors.New("pool exhausted")

func (s *Service) Expire(ctx context.Context, entryID uuid.UUID) error {
	_, err := s.repo.UpdateWaitlistEntryStatus(ctx, db.UpdateWaitlistEntryStatusParams{
		ID: entryID, Status: StatusExpired,
	})
	return err
}

func (s *Service) Withdraw(ctx context.Context, entryID, participantID uuid.UUID) error {
	entry, err := s.repo.GetWaitlistEntry(ctx, db.GetWaitlistEntryParams{
		WaitlistID: uuid.Nil, ParticipantID: participantID,
	})
	if errors.Is(err, pgx.ErrNoRows) { return ErrNotOnWaitlist }
	if err != nil { return err }
	_, err = s.repo.UpdateWaitlistEntryStatus(ctx, db.UpdateWaitlistEntryStatusParams{
		ID: entry.ID, Status: StatusWithdrawn,
	})
	return err
}
```

**Note:** `PromoteBatch` needs `GetWaitlistByID` — add to waitlist.sql: `-- name: GetWaitlist :one SELECT * FROM waitlists WHERE id = $1;` and regenerate sqlc.

- [ ] **Step 5: Run tests — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/waitlist/tests/ -race -v 2>&1
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/waitlist/
git commit -m "feat(phase10): waitlist service (join, promote-batch, expire, withdraw)"
```

---

### Task 8: AccessPool Module — PoolManager

**Files:**
- Create: `services/api/internal/modules/access/model.go`
- Create: `services/api/internal/modules/access/errors.go`
- Create: `services/api/internal/modules/access/repository.go`
- Create: `services/api/internal/modules/access/pool.go`
- Create: `services/api/internal/modules/access/tests/pool_test.go`

- [ ] **Step 1: Write model.go**

```go
package access

const (
	PoolTypeReserved  = "RESERVED"
	PoolTypeCommunity = "COMMUNITY"
	PoolTypeCorporate = "CORPORATE"
	PoolTypeSponsor   = "SPONSOR"
	PoolTypeVIP       = "VIP"
	PoolTypePartner   = "PARTNER"
	PoolTypePriority  = "PRIORITY"
	PoolTypeElite     = "ELITE"

	GrantStatusActive   = "ACTIVE"
	GrantStatusConsumed = "CONSUMED"
	GrantStatusExpired  = "EXPIRED"
)
```

- [ ] **Step 2: Write errors.go**

```go
package access

import (
	"net/http"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrPoolExhausted        = apperr.New(http.StatusConflict, "POOL_EXHAUSTED", "no available slots in pool")
	ErrGrantNotFound        = apperr.New(http.StatusNotFound, "GRANT_NOT_FOUND", "access grant not found")
	ErrGrantExpired         = apperr.New(http.StatusForbidden, "GRANT_EXPIRED", "access grant has expired")
	ErrGrantAlreadyConsumed = apperr.New(http.StatusConflict, "GRANT_ALREADY_CONSUMED", "access grant already used")
)
```

- [ ] **Step 3: Write repository.go** (follow abuse/repository.go pattern)

- [ ] **Step 4: Write failing pool test**

```go
// services/api/internal/modules/access/tests/pool_test.go
package access_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

type fakeAccessRepo struct {
	reserveCount int
	maxSlots     int
}

func (r *fakeAccessRepo) ReservePoolSlot(_ context.Context, _ uuid.UUID) (db.AccessPool, error) {
	if r.reserveCount >= r.maxSlots {
		return db.AccessPool{}, access.ErrPoolExhausted
	}
	r.reserveCount++
	return db.AccessPool{}, nil
}
// implement remaining interface methods as no-ops

func TestReserveSlot_ExhaustedReturnsError(t *testing.T) {
	repo := &fakeAccessRepo{maxSlots: 3}
	pm := access.NewPoolManager(repo)
	for i := 0; i < 3; i++ {
		if err := pm.ReserveSlot(context.Background(), uuid.New()); err != nil {
			t.Fatalf("slot %d should succeed: %v", i, err)
		}
	}
	if err := pm.ReserveSlot(context.Background(), uuid.New()); err == nil {
		t.Fatal("4th reserve should return ErrPoolExhausted")
	}
}
```

- [ ] **Step 5: Write pool.go**

```go
package access

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type PoolManager struct{ repo Repository }

func NewPoolManager(repo Repository) *PoolManager { return &PoolManager{repo: repo} }

func (p *PoolManager) ReserveSlot(ctx context.Context, poolID uuid.UUID) error {
	_, err := p.repo.ReservePoolSlot(ctx, poolID)
	if err != nil {
		// ReservePoolSlot returns 0 rows (pgx.ErrNoRows) when pool is full
		if errors.Is(err, pgx.ErrNoRows) { return ErrPoolExhausted }
		return err
	}
	return nil
}

func (p *PoolManager) ConsumeSlot(ctx context.Context, poolID uuid.UUID) error {
	return p.repo.ConsumePoolSlot(ctx, poolID)
}

func (p *PoolManager) ReleaseSlot(ctx context.Context, poolID uuid.UUID) error {
	return p.repo.ReleasePoolSlot(ctx, poolID)
}

func (p *PoolManager) CreateGrant(ctx context.Context, poolID, participantID, eventID, categoryID uuid.UUID, expiresAt time.Time) (uuid.UUID, error) {
	grant, err := p.repo.CreateAccessGrant(ctx, db.CreateAccessGrantParams{
		PoolID:        &poolID,
		ParticipantID: participantID,
		EventID:       eventID,
		CategoryID:    categoryID,
		ExpiresAt:     pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil { return uuid.Nil, err }
	return grant.ID, nil
}

func (p *PoolManager) CheckGrant(ctx context.Context, participantID, categoryID uuid.UUID, grantToken string) error {
	grantID, err := uuid.Parse(grantToken)
	if err != nil { return ErrGrantNotFound }
	grant, err := p.repo.GetAccessGrant(ctx, grantID)
	if err != nil { return ErrGrantNotFound }
	if grant.ParticipantID != participantID || grant.CategoryID != categoryID { return ErrGrantNotFound }
	if grant.Status == GrantStatusConsumed { return ErrGrantAlreadyConsumed }
	if grant.Status == GrantStatusExpired || grant.ExpiresAt.Time.Before(time.Now()) { return ErrGrantExpired }
	return nil
}
```

- [ ] **Step 6: Run tests — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -race -v 2>&1
```

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/modules/access/
git commit -m "feat(phase10): access pool manager (reserve/consume/release/grant atomic ops)"
```

---

### Task 9: Inject LifecycleChecker into RAE

**Files:**
- Modify: `services/api/internal/modules/registration/gate.go`
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Write failing test**

```go
// services/api/internal/modules/registration/tests/gate_lifecycle_test.go
package registration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/lifecycle"
	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

type fakeLifecycleChecker struct{ open bool; reason lifecycle.WindowClosedReason }

func (f *fakeLifecycleChecker) IsWindowOpen(_ context.Context, _ uuid.UUID, _ registration.Mode) (bool, lifecycle.WindowClosedReason, error) {
	return f.open, f.reason, nil
}

func TestAdmit_LifecycleClosed_Returns409(t *testing.T) {
	checker := &fakeLifecycleChecker{open: false, reason: lifecycle.ReasonWindowExpired}
	// build gate with nil queue, nil ballot, lifecycle=checker
	// call Admit for ModeNormal
	// expect error with code REGISTRATION_WINDOW_CLOSED
	_ = checker
	t.Skip("implement after gate.go updated")
}
```

- [ ] **Step 2: Update gate.go**

Add `LifecycleChecker` interface and `lifecycle` field. Add lifecycle check in `Admit()` after mode resolution:

```go
// In registration/gate.go — add interface
type LifecycleChecker interface {
	IsWindowOpen(ctx context.Context, categoryID uuid.UUID, mode Mode) (open bool, reason lifecycle.WindowClosedReason, err error)
}

// Update Gate struct
type Gate struct {
	svc       *Service
	queue     QueueAdmitter
	lifecycle LifecycleChecker
}

// Update NewGate
func NewGate(svc *Service, queue QueueAdmitter, lc LifecycleChecker) *Gate {
	return &Gate{svc: svc, queue: queue, lifecycle: lc}
}

// In Admit(), after mode resolution, before the switch:
if g.lifecycle != nil {
	open, reason, err := g.lifecycle.IsWindowOpen(ctx, categoryID, mode)
	if err == nil && !open {
		return apperr.New(http.StatusConflict, "REGISTRATION_WINDOW_CLOSED", string(reason))
	}
}
```

- [ ] **Step 3: Update server.go**

In server.go, find where `registration.NewGate` is called. Pass `lifecycleSvc` as third argument:
```go
lifecycleRepo := lifecycle.NewRepository(pool)
lifecycleSvc := lifecycle.NewService(lifecycleRepo)
registrationGate := registration.NewGate(registrationSvc, queueAdmitter, lifecycleSvc)
```

- [ ] **Step 4: Build + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./... 2>&1
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
# Expected: all ok
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/registration/gate.go services/api/internal/app/server.go services/api/internal/modules/registration/tests/
git commit -m "feat(phase10): inject LifecycleChecker into RAE gate"
```

---

### Task 10: Part 1 Full Verification

- [ ] **Step 1: Full build + vet + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api
go build ./... 2>&1
go vet ./... 2>&1
go test ./internal/modules/lifecycle/... ./internal/modules/waitlist/... ./internal/modules/access/... -race -v 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
# Expected: all ok, zero FAIL
```

- [ ] **Step 2: Commit verification**

```bash
git add -A
git commit -m "test(phase10): part 1 foundation green — lifecycle, waitlist, access pool"
```
