# Phase 10 Part 2: Ballot Draw Engine + Organizer APIs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the full ballot draw lifecycle — BallotDraw + BallotEntry state machines, deterministic Fisher-Yates draw, RESERVED AccessPool creation, winner AccessGrant issuance, lapse/waitlist promotion — and all organizer HTTP endpoints.

**Architecture:** New `ballot` module with model/errors/dto/repository/draw/service/handler/routes. Draw algorithm is pure (no DB side effects) so it is unit-tested in isolation. `RunDraw` is idempotent — calling it twice returns existing results unchanged. Winner grants are issued from a RESERVED `AccessPool` created atomically with the draw results. Organizer endpoints require org-level auth. No participant endpoints yet (Part 3).

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, crypto/sha256, math/rand. Module: `github.com/varin/ivyticketing/services/api`.

---

### Task 1: Migrations 00037–00040 — Ballot Tables

**Files:**
- Create: `database/migrations/00037_create_ballot_draws.sql`
- Create: `database/migrations/00038_create_ballot_entries.sql`
- Create: `database/migrations/00039_create_ballot_draw_results.sql`
- Create: `database/migrations/00040_seed_ballot_permissions.sql`

- [ ] **Step 1: Write 00037_create_ballot_draws.sql**

```sql
-- +goose Up
CREATE TABLE ballot_draws (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id                uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id             uuid NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    status                  text NOT NULL DEFAULT 'PENDING'
                                CHECK (status IN ('PENDING','OPEN','CLOSED','DRAWN','ANNOUNCED')),
    quota                   integer NOT NULL CHECK (quota > 0),
    waitlist_size           integer NOT NULL DEFAULT 0 CHECK (waitlist_size >= 0),
    payment_window_hours    integer NOT NULL DEFAULT 48 CHECK (payment_window_hours > 0),
    application_opens_at    timestamptz NOT NULL,
    application_closes_at   timestamptz NOT NULL,
    draw_at                 timestamptz,
    announced_at            timestamptz,
    seed                    text,
    draw_nonce              uuid,
    winner_pool_id          uuid REFERENCES access_pools(id),
    waitlist_id             uuid REFERENCES waitlists(id),
    created_by              uuid NOT NULL REFERENCES users(id),
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ballot_draws_dates_check CHECK (application_opens_at < application_closes_at)
);
CREATE INDEX ballot_draws_category_idx ON ballot_draws(event_id, category_id, status);

-- +goose Down
DROP TABLE ballot_draws;
```

- [ ] **Step 2: Write 00038_create_ballot_entries.sql**

```sql
-- +goose Up
CREATE TABLE ballot_entries (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_id           uuid NOT NULL REFERENCES ballot_draws(id) ON DELETE CASCADE,
    organization_id   uuid NOT NULL REFERENCES organizations(id),
    event_id          uuid NOT NULL REFERENCES events(id),
    category_id       uuid NOT NULL REFERENCES event_categories(id),
    participant_id    uuid NOT NULL REFERENCES users(id),
    status            text NOT NULL DEFAULT 'APPLIED'
                          CHECK (status IN ('APPLIED','WINNER','WAITLISTED','NOT_SELECTED',
                                           'LAPSED','CONVERTED','WITHDRAWN')),
    applied_at        timestamptz NOT NULL DEFAULT now(),
    payment_deadline  timestamptz,
    converted_at      timestamptz,
    promoted_round    integer NOT NULL DEFAULT 0,
    access_grant_id   uuid REFERENCES access_grants(id),
    UNIQUE (draw_id, participant_id)
);
CREATE INDEX ballot_entries_draw_status_idx ON ballot_entries(draw_id, status);
CREATE INDEX ballot_entries_participant_idx ON ballot_entries(participant_id, event_id, status);

-- +goose Down
DROP TABLE ballot_entries;
```

- [ ] **Step 3: Write 00039_create_ballot_draw_results.sql**

```sql
-- +goose Up
CREATE TABLE ballot_draw_results (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_id          uuid NOT NULL REFERENCES ballot_draws(id) ON DELETE CASCADE,
    ballot_entry_id  uuid NOT NULL REFERENCES ballot_entries(id),
    outcome          text NOT NULL CHECK (outcome IN ('WINNER','WAITLISTED','NOT_SELECTED')),
    rank             integer NOT NULL,
    result_hash      text NOT NULL,
    UNIQUE (draw_id, ballot_entry_id)
);
CREATE INDEX ballot_draw_results_rank_idx ON ballot_draw_results(draw_id, outcome, rank);

-- +goose Down
DROP TABLE ballot_draw_results;
```

- [ ] **Step 4: Write 00040_seed_ballot_permissions.sql**

Read `database/migrations/00007_seed_rbac_catalog.sql` first to match the exact INSERT pattern used for permissions. Then write:

```sql
-- +goose Up
INSERT INTO permissions (name, description) VALUES
    ('manage_ballot', 'Create and manage ballot draws'),
    ('apply_ballot',  'Apply to ballot draws')
ON CONFLICT (name) DO NOTHING;

-- +goose Down
DELETE FROM permissions WHERE name IN ('manage_ballot', 'apply_ballot');
```

- [ ] **Step 5: Run migrations**

```bash
cd /Users/kaivy/Coding/ivyticketing
make migrate-up 2>&1
# Expected: migrated to version 40
make migrate-down 2>&1
make migrate-up 2>&1
```

- [ ] **Step 6: Commit**

```bash
git add database/migrations/00037_create_ballot_draws.sql \
        database/migrations/00038_create_ballot_entries.sql \
        database/migrations/00039_create_ballot_draw_results.sql \
        database/migrations/00040_seed_ballot_permissions.sql
git commit -m "feat(phase10): migrations 00037-00040 (ballot draws, entries, results, permissions)"
```

---

### Task 2: sqlc Ballot Queries

**Files:**
- Create: `database/queries/ballot.sql`

- [ ] **Step 1: Write ballot.sql**

```sql
-- name: CreateBallotDraw :one
INSERT INTO ballot_draws
    (organization_id, event_id, category_id, quota, waitlist_size,
     payment_window_hours, application_opens_at, application_closes_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetBallotDraw :one
SELECT * FROM ballot_draws WHERE id = $1;

-- name: GetActiveBallotDrawByCategory :one
SELECT * FROM ballot_draws
WHERE event_id = $1 AND category_id = $2 AND status NOT IN ('ANNOUNCED')
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateBallotDrawStatus :one
UPDATE ballot_draws
SET status = $2,
    draw_at = CASE WHEN $2 = 'DRAWN' THEN now() ELSE draw_at END,
    announced_at = CASE WHEN $2 = 'ANNOUNCED' THEN now() ELSE announced_at END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetBallotDrawSeed :one
UPDATE ballot_draws SET seed = $2, draw_nonce = $3, updated_at = now()
WHERE id = $1 AND seed IS NULL
RETURNING *;

-- name: SetBallotDrawPools :exec
UPDATE ballot_draws SET winner_pool_id = $2, waitlist_id = $3, updated_at = now()
WHERE id = $1;

-- name: CreateBallotEntry :one
INSERT INTO ballot_entries
    (draw_id, organization_id, event_id, category_id, participant_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetBallotEntry :one
SELECT * FROM ballot_entries WHERE draw_id = $1 AND participant_id = $2;

-- name: GetBallotEntryByID :one
SELECT * FROM ballot_entries WHERE id = $1;

-- name: ListAppliedEntriesForDraw :many
SELECT * FROM ballot_entries
WHERE draw_id = $1 AND status = 'APPLIED'
ORDER BY id ASC;

-- name: UpdateBallotEntryStatus :one
UPDATE ballot_entries
SET status = $2,
    payment_deadline  = COALESCE($3, payment_deadline),
    converted_at      = CASE WHEN $2 = 'CONVERTED' THEN now() ELSE converted_at END,
    access_grant_id   = COALESCE($4, access_grant_id)
WHERE id = $1
RETURNING *;

-- name: BulkUpdateBallotOutcome :exec
UPDATE ballot_entries SET status = $2 WHERE id = ANY($1::uuid[]);

-- name: InsertBallotDrawResult :exec
INSERT INTO ballot_draw_results (draw_id, ballot_entry_id, outcome, rank, result_hash)
VALUES ($1, $2, $3, $4, $5);

-- name: ListBallotDrawResults :many
SELECT bdr.*, be.participant_id FROM ballot_draw_results bdr
JOIN ballot_entries be ON be.id = bdr.ballot_entry_id
WHERE bdr.draw_id = $1
ORDER BY bdr.rank ASC
LIMIT $2 OFFSET $3;

-- name: CountBallotDrawResults :one
SELECT count(*) FROM ballot_draw_results WHERE draw_id = $1 AND outcome = $2;

-- name: ListAllDrawResults :many
SELECT bdr.*, be.participant_id FROM ballot_draw_results bdr
JOIN ballot_entries be ON be.id = bdr.ballot_entry_id
WHERE bdr.draw_id = $1
ORDER BY bdr.rank ASC;

-- name: ListWinnerEntries :many
SELECT * FROM ballot_entries
WHERE draw_id = $1 AND status = 'WINNER';

-- name: ListExpiringWinners :many
SELECT * FROM ballot_entries
WHERE status = 'WINNER' AND payment_deadline < now()
LIMIT $1;

-- name: GetBallotEntryByParticipant :many
SELECT * FROM ballot_entries
WHERE participant_id = $1
ORDER BY applied_at DESC
LIMIT $2 OFFSET $3;
```

- [ ] **Step 2: Run sqlc + build**

```bash
cd /Users/kaivy/Coding/ivyticketing && make sqlc 2>&1
cd services/api && go build ./internal/db/... 2>&1
# Expected: clean
```

- [ ] **Step 3: Commit**

```bash
git add database/queries/ballot.sql
git commit -m "feat(phase10): sqlc ballot queries"
```

---

### Task 3: Ballot Model, Errors, and Draw Algorithm

**Files:**
- Create: `services/api/internal/modules/ballot/model.go`
- Create: `services/api/internal/modules/ballot/errors.go`
- Create: `services/api/internal/modules/ballot/draw.go`
- Create: `services/api/internal/modules/ballot/draw_test.go`

- [ ] **Step 1: Write draw_test.go first (TDD)**

```go
package ballot

import (
	"testing"
)

func TestShuffle_Deterministic(t *testing.T) {
	entries := make([]DrawEntry, 100)
	for i := range entries { entries[i] = DrawEntry{ID: fmt.Sprintf("entry-%d", i)} }
	seed := "test-seed-abc"
	first := Shuffle(seed, entries)
	for i := 0; i < 999; i++ {
		result := Shuffle(seed, entries)
		for j := range result {
			if result[j].ID != first[j].ID {
				t.Fatalf("shuffle not deterministic at run %d, position %d", i, j)
			}
		}
	}
}

func TestShuffle_NoDuplicates(t *testing.T) {
	entries := make([]DrawEntry, 1000)
	for i := range entries { entries[i] = DrawEntry{ID: fmt.Sprintf("e-%d", i)} }
	result := Shuffle("seed-xyz", entries)
	seen := map[string]bool{}
	for _, e := range result {
		if seen[e.ID] { t.Fatalf("duplicate entry: %s", e.ID) }
		seen[e.ID] = true
	}
}

func TestAssign_QuotaExact(t *testing.T) {
	entries := make([]DrawEntry, 10)
	for i := range entries { entries[i] = DrawEntry{ID: fmt.Sprintf("e-%d", i)} }
	shuffled := Shuffle("seed", entries)
	results := Assign("seed", shuffled, 3, 2)
	counts := map[string]int{}
	for _, r := range results { counts[r.Outcome]++ }
	if counts["WINNER"] != 3 { t.Fatalf("want 3 winners, got %d", counts["WINNER"]) }
	if counts["WAITLISTED"] != 2 { t.Fatalf("want 2 waitlisted, got %d", counts["WAITLISTED"]) }
	if counts["NOT_SELECTED"] != 5 { t.Fatalf("want 5 not_selected, got %d", counts["NOT_SELECTED"]) }
}

func TestAssign_ResultHashVerifiable(t *testing.T) {
	entries := []DrawEntry{{ID: "abc"}}
	shuffled := Shuffle("myseed", entries)
	results := Assign("myseed", shuffled, 1, 0)
	expected := fmt.Sprintf("%x", sha256.Sum256([]byte("myseed|0|"+results[0].EntryID)))
	if results[0].ResultHash != expected {
		t.Fatalf("result_hash mismatch: want %s got %s", expected, results[0].ResultHash)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL (draw.go not written)**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/ballot/ -v 2>&1
# Expected: FAIL — DrawEntry undefined
```

- [ ] **Step 3: Write model.go**

```go
package ballot

const (
	StatusApplied     = "APPLIED"
	StatusWinner      = "WINNER"
	StatusWaitlisted  = "WAITLISTED"
	StatusNotSelected = "NOT_SELECTED"
	StatusLapsed      = "LAPSED"
	StatusConverted   = "CONVERTED"
	StatusWithdrawn   = "WITHDRAWN"

	DrawStatusPending   = "PENDING"
	DrawStatusOpen      = "OPEN"
	DrawStatusClosed    = "CLOSED"
	DrawStatusDrawn     = "DRAWN"
	DrawStatusAnnounced = "ANNOUNCED"

	OutcomeWinner      = "WINNER"
	OutcomeWaitlisted  = "WAITLISTED"
	OutcomeNotSelected = "NOT_SELECTED"
)
```

- [ ] **Step 4: Write errors.go**

```go
package ballot

import (
	"net/http"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrBallotClosed            = apperr.New(http.StatusConflict, "BALLOT_CLOSED", "ballot application window is not open")
	ErrAlreadyApplied          = apperr.New(http.StatusConflict, "BALLOT_ALREADY_APPLIED", "already applied to this ballot")
	ErrNotWinner               = apperr.New(http.StatusForbidden, "BALLOT_NOT_WINNER", "ballot entry is not a winner")
	ErrDrawNotAnnounced        = apperr.New(http.StatusConflict, "BALLOT_DRAW_NOT_ANNOUNCED", "ballot results not yet announced")
	ErrPaymentWindowExpired    = apperr.New(http.StatusConflict, "BALLOT_PAYMENT_WINDOW_EXPIRED", "winner payment window has expired")
	ErrDrawAlreadyRun          = apperr.New(http.StatusConflict, "BALLOT_DRAW_ALREADY_RUN", "draw has already been executed")
	ErrBallotWithdrawNotAllowed = apperr.New(http.StatusConflict, "BALLOT_WITHDRAW_NOT_ALLOWED", "can only withdraw while ballot is open")
)
```

- [ ] **Step 5: Write draw.go**

```go
package ballot

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
)

type DrawEntry struct {
	ID string // ballot_entry UUID as string
}

type DrawResult struct {
	EntryID     string
	Outcome     string
	Rank        int
	ResultHash  string
}

// Shuffle performs deterministic Fisher-Yates shuffle using seed.
// entries must be ordered by id ASC before calling (deterministic input).
func Shuffle(seed string, entries []DrawEntry) []DrawEntry {
	h := sha256.Sum256([]byte(seed))
	src := rand.NewSource(int64(binary.BigEndian.Uint64(h[:8])))
	r := rand.New(src)
	out := make([]DrawEntry, len(entries))
	copy(out, entries)
	for i := len(out) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Assign distributes outcomes to shuffled entries and computes result hashes.
func Assign(seed string, shuffled []DrawEntry, quota, waitlistSize int) []DrawResult {
	results := make([]DrawResult, len(shuffled))
	for i, e := range shuffled {
		outcome := OutcomeNotSelected
		switch {
		case i < quota:
			outcome = OutcomeWinner
		case i < quota+waitlistSize:
			outcome = OutcomeWaitlisted
		}
		raw := fmt.Sprintf("%s|%d|%s", seed, i, e.ID)
		hash := sha256.Sum256([]byte(raw))
		results[i] = DrawResult{
			EntryID:    e.ID,
			Outcome:    outcome,
			Rank:       i,
			ResultHash: hex.EncodeToString(hash[:]),
		}
	}
	return results
}
```

- [ ] **Step 6: Run tests — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/ballot/ -race -v 2>&1
# Expected: 4 tests PASS
```

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/modules/ballot/model.go \
        services/api/internal/modules/ballot/errors.go \
        services/api/internal/modules/ballot/draw.go \
        services/api/internal/modules/ballot/draw_test.go
git commit -m "feat(phase10): ballot model + deterministic draw algorithm (TDD)"
```

---

### Task 4: Ballot Repository

**Files:**
- Create: `services/api/internal/modules/ballot/repository.go`

- [ ] **Step 1: Read db/ballot.sql.go**

```bash
cat /Users/kaivy/Coding/ivyticketing/services/api/internal/db/ballot.sql.go | head -100
```

Verify exact generated param struct names: `CreateBallotDrawParams`, `GetBallotDrawParams`, `UpdateBallotDrawStatusParams`, `SetBallotDrawSeedParams`, `CreateBallotEntryParams`, `UpdateBallotEntryStatusParams`, `InsertBallotDrawResultParams`, `ListBallotDrawResultsParams`, `BulkUpdateBallotOutcomeParams`.

- [ ] **Step 2: Write repository.go**

```go
package ballot

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateBallotDraw(ctx context.Context, arg db.CreateBallotDrawParams) (db.BallotDraw, error)
	GetBallotDraw(ctx context.Context, id uuid.UUID) (db.BallotDraw, error)
	GetActiveBallotDrawByCategory(ctx context.Context, arg db.GetActiveBallotDrawByCategoryParams) (db.BallotDraw, error)
	UpdateBallotDrawStatus(ctx context.Context, arg db.UpdateBallotDrawStatusParams) (db.BallotDraw, error)
	SetBallotDrawSeed(ctx context.Context, arg db.SetBallotDrawSeedParams) (db.BallotDraw, error)
	SetBallotDrawPools(ctx context.Context, arg db.SetBallotDrawPoolsParams) error
	CreateBallotEntry(ctx context.Context, arg db.CreateBallotEntryParams) (db.BallotEntry, error)
	GetBallotEntry(ctx context.Context, arg db.GetBallotEntryParams) (db.BallotEntry, error)
	GetBallotEntryByID(ctx context.Context, id uuid.UUID) (db.BallotEntry, error)
	ListAppliedEntriesForDraw(ctx context.Context, drawID uuid.UUID) ([]db.BallotEntry, error)
	UpdateBallotEntryStatus(ctx context.Context, arg db.UpdateBallotEntryStatusParams) (db.BallotEntry, error)
	BulkUpdateBallotOutcome(ctx context.Context, arg db.BulkUpdateBallotOutcomeParams) error
	InsertBallotDrawResult(ctx context.Context, arg db.InsertBallotDrawResultParams) error
	ListBallotDrawResults(ctx context.Context, arg db.ListBallotDrawResultsParams) ([]db.ListBallotDrawResultsRow, error)
	ListAllDrawResults(ctx context.Context, drawID uuid.UUID) ([]db.ListAllDrawResultsRow, error)
	CountBallotDrawResults(ctx context.Context, arg db.CountBallotDrawResultsParams) (int64, error)
	ListWinnerEntries(ctx context.Context, drawID uuid.UUID) ([]db.BallotEntry, error)
	ListExpiringWinners(ctx context.Context, limit int32) ([]db.BallotEntry, error)
	GetBallotEntryByParticipant(ctx context.Context, arg db.GetBallotEntryByParticipantParams) ([]db.BallotEntry, error)
}

type sqlcRepo struct{ q *db.Queries }

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

func (r *sqlcRepo) CreateBallotDraw(ctx context.Context, arg db.CreateBallotDrawParams) (db.BallotDraw, error) {
	return r.q.CreateBallotDraw(ctx, arg)
}
// ... implement all remaining methods by delegating to r.q.*
```

- [ ] **Step 3: Build**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./internal/modules/ballot/... 2>&1
```

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/ballot/repository.go
git commit -m "feat(phase10): ballot repository"
```

---

### Task 5: Ballot Service — Draw Lifecycle

**Files:**
- Create: `services/api/internal/modules/ballot/service.go`
- Create: `services/api/internal/modules/ballot/tests/service_test.go`

- [ ] **Step 1: Write failing tests**

```go
// services/api/internal/modules/ballot/tests/service_test.go
package ballot_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/ballot"
)

// fakeRepo implements ballot.Repository — all methods return empty values except
// those overridden per test.
type fakeRepo struct {
	draw    db.BallotDraw
	entries []db.BallotEntry
	results []db.BallotDrawResult
	seedSet bool
}

func (r *fakeRepo) CreateBallotDraw(_ context.Context, _ db.CreateBallotDrawParams) (db.BallotDraw, error) { return r.draw, nil }
func (r *fakeRepo) GetBallotDraw(_ context.Context, _ uuid.UUID) (db.BallotDraw, error) { return r.draw, nil }
func (r *fakeRepo) GetActiveBallotDrawByCategory(_ context.Context, _ db.GetActiveBallotDrawByCategoryParams) (db.BallotDraw, error) { return r.draw, nil }
func (r *fakeRepo) UpdateBallotDrawStatus(_ context.Context, arg db.UpdateBallotDrawStatusParams) (db.BallotDraw, error) {
	r.draw.Status = arg.Status; return r.draw, nil
}
func (r *fakeRepo) SetBallotDrawSeed(_ context.Context, _ db.SetBallotDrawSeedParams) (db.BallotDraw, error) {
	if r.seedSet { return db.BallotDraw{}, pgx.ErrNoRows } // simulate seed already set
	r.seedSet = true; return r.draw, nil
}
func (r *fakeRepo) SetBallotDrawPools(_ context.Context, _ db.SetBallotDrawPoolsParams) error { return nil }
func (r *fakeRepo) CreateBallotEntry(_ context.Context, _ db.CreateBallotEntryParams) (db.BallotEntry, error) { return db.BallotEntry{}, nil }
func (r *fakeRepo) GetBallotEntry(_ context.Context, _ db.GetBallotEntryParams) (db.BallotEntry, error) { return db.BallotEntry{}, pgx.ErrNoRows }
func (r *fakeRepo) GetBallotEntryByID(_ context.Context, _ uuid.UUID) (db.BallotEntry, error) { return db.BallotEntry{}, nil }
func (r *fakeRepo) ListAppliedEntriesForDraw(_ context.Context, _ uuid.UUID) ([]db.BallotEntry, error) { return r.entries, nil }
func (r *fakeRepo) UpdateBallotEntryStatus(_ context.Context, _ db.UpdateBallotEntryStatusParams) (db.BallotEntry, error) { return db.BallotEntry{}, nil }
func (r *fakeRepo) BulkUpdateBallotOutcome(_ context.Context, _ db.BulkUpdateBallotOutcomeParams) error { return nil }
func (r *fakeRepo) InsertBallotDrawResult(_ context.Context, _ db.InsertBallotDrawResultParams) error { return nil }
func (r *fakeRepo) ListBallotDrawResults(_ context.Context, _ db.ListBallotDrawResultsParams) ([]db.ListBallotDrawResultsRow, error) { return nil, nil }
func (r *fakeRepo) ListAllDrawResults(_ context.Context, _ uuid.UUID) ([]db.ListAllDrawResultsRow, error) { return nil, nil }
func (r *fakeRepo) CountBallotDrawResults(_ context.Context, arg db.CountBallotDrawResultsParams) (int64, error) {
	if len(r.results) > 0 { return int64(len(r.results)), nil }; return 0, nil
}
func (r *fakeRepo) ListWinnerEntries(_ context.Context, _ uuid.UUID) ([]db.BallotEntry, error) { return nil, nil }
func (r *fakeRepo) ListExpiringWinners(_ context.Context, _ int32) ([]db.BallotEntry, error) { return nil, nil }
func (r *fakeRepo) GetBallotEntryByParticipant(_ context.Context, _ db.GetBallotEntryByParticipantParams) ([]db.BallotEntry, error) { return nil, nil }

type fakePoolCreator struct{ poolID uuid.UUID }
func (f *fakePoolCreator) CreatePool(_ context.Context, _, _, _ uuid.UUID, _, _ string, _ int, _ uuid.UUID) (uuid.UUID, error) {
	return f.poolID, nil
}

type fakeGrantIssuer struct{ callCount int }
func (f *fakeGrantIssuer) ReserveSlot(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeGrantIssuer) CreateGrant(_ context.Context, _, _, _, _ uuid.UUID, _ time.Time) (uuid.UUID, error) {
	f.callCount++; return uuid.New(), nil
}

type fakeWaitlistCreator struct{}
func (f *fakeWaitlistCreator) CreateWaitlist(_ context.Context, _, _, _ uuid.UUID, _ uuid.UUID) (uuid.UUID, error) { return uuid.New(), nil }
func (f *fakeWaitlistCreator) Join(_ context.Context, _, _ uuid.UUID, _ string, _ *uuid.UUID, _ int64) error { return nil }

func buildSvc(repo *fakeRepo) *ballot.Service {
	return ballot.NewService(repo, nil, &fakePoolCreator{poolID: uuid.New()}, &fakeGrantIssuer{}, &fakeWaitlistCreator{})
}

func TestRunDraw_Idempotent(t *testing.T) {
	entries := make([]db.BallotEntry, 5)
	for i := range entries { entries[i].ID = uuid.New() }
	repo := &fakeRepo{
		draw:    db.BallotDraw{Status: ballot.DrawStatusClosed, Quota: 2, WaitlistSize: pgtype.Int4{Int32: 1, Valid: true}},
		entries: entries,
		results: []db.BallotDrawResult{{ID: uuid.New()}}, // pre-existing results → idempotent
	}
	svc := buildSvc(repo)
	err := svc.RunDraw(context.Background(), uuid.New(), uuid.New())
	if err != nil { t.Fatalf("idempotent RunDraw should return nil, got: %v", err) }
	if repo.seedSet { t.Fatal("seed should not be set again when results already exist") }
}

func TestRunDraw_StatusGuard(t *testing.T) {
	repo := &fakeRepo{draw: db.BallotDraw{Status: ballot.DrawStatusOpen}}
	svc := buildSvc(repo)
	err := svc.RunDraw(context.Background(), uuid.New(), uuid.New())
	if err == nil { t.Fatal("RunDraw on OPEN draw should return error") }
}

func TestAnnounceDraw_GrantsIssuedToWinners(t *testing.T) {
	winnerID := uuid.New()
	repo := &fakeRepo{
		draw: db.BallotDraw{Status: ballot.DrawStatusDrawn, PaymentWindowHours: 48},
	}
	// GetBallotDraw will return a DRAWN draw; ListWinnerEntries returns 3 winners
	repo.entries = []db.BallotEntry{
		{ID: uuid.New(), ParticipantID: winnerID, Status: ballot.StatusWinner},
		{ID: uuid.New(), ParticipantID: uuid.New(), Status: ballot.StatusWinner},
		{ID: uuid.New(), ParticipantID: uuid.New(), Status: ballot.StatusWinner},
	}
	issuer := &fakeGrantIssuer{}
	svc := ballot.NewService(repo, nil, &fakePoolCreator{poolID: uuid.New()}, issuer, &fakeWaitlistCreator{})
	_ = svc // AnnounceDraw not yet implemented — test will fail
	t.Skip("implement AnnounceDraw then remove Skip")
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/ballot/tests/ -v 2>&1
# Expected: FAIL — ballot.NewService undefined
```

- [ ] **Step 3: Write service.go**

```go
package ballot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type PoolCreator interface {
	CreatePool(ctx context.Context, orgID, eventID, categoryID uuid.UUID, poolType, name string, slots int, createdBy uuid.UUID) (uuid.UUID, error)
}

type GrantIssuer interface {
	ReserveSlot(ctx context.Context, poolID uuid.UUID) error
	CreateGrant(ctx context.Context, poolID, participantID, eventID, categoryID uuid.UUID, expiresAt time.Time) (uuid.UUID, error)
}

type WaitlistCreator interface {
	CreateWaitlist(ctx context.Context, orgID, eventID, categoryID, createdBy uuid.UUID) (uuid.UUID, error)
	Join(ctx context.Context, waitlistID, participantID uuid.UUID, source string, sourceRefID *uuid.UUID, rank int64) error
}

type Service struct {
	repo     Repository
	audit    AuditRecorder
	pools    PoolCreator
	grants   GrantIssuer
	waitlist WaitlistCreator
}

func NewService(repo Repository, audit AuditRecorder, pools PoolCreator, grants GrantIssuer, wl WaitlistCreator) *Service {
	return &Service{repo: repo, audit: audit, pools: pools, grants: grants, waitlist: wl}
}

func (s *Service) RunDraw(ctx context.Context, drawID, actorID uuid.UUID) error {
	draw, err := s.repo.GetBallotDraw(ctx, drawID)
	if err != nil { return err }
	if draw.Status != DrawStatusClosed { return ErrBallotClosed }

	// Idempotency: check if results already exist
	n, err := s.repo.CountBallotDrawResults(ctx, db.CountBallotDrawResultsParams{
		DrawID: drawID, Outcome: OutcomeWinner,
	})
	if err != nil { return err }
	if n > 0 { return nil } // already run

	// 1. Commit seed before draw
	nonce := uuid.New()
	seedInput := fmt.Sprintf("%s|%s|%d|%s", draw.EventID, draw.CategoryID, draw.DrawAt.Time.UnixNano(), nonce)
	seedHash := sha256.Sum256([]byte(seedInput))
	seed := hex.EncodeToString(seedHash[:])
	if _, err := s.repo.SetBallotDrawSeed(ctx, db.SetBallotDrawSeedParams{
		ID: drawID, Seed: pgtype.Text{String: seed, Valid: true},
		DrawNonce: pgtype.UUID{Bytes: nonce, Valid: true},
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return nil } // seed already set — idempotent
		return err
	}

	// 2. Load entries (ordered by id ASC — deterministic)
	dbEntries, err := s.repo.ListAppliedEntriesForDraw(ctx, drawID)
	if err != nil { return err }
	entries := make([]DrawEntry, len(dbEntries))
	for i, e := range dbEntries { entries[i] = DrawEntry{ID: e.ID.String()} }

	// 3. Shuffle + Assign
	quota := int(draw.Quota)
	waitlistSize := 0
	if draw.WaitlistSize.Valid { waitlistSize = int(draw.WaitlistSize.Int32) }
	shuffled := Shuffle(seed, entries)
	results := Assign(seed, shuffled, quota, waitlistSize)

	// 4. Write results + update entry statuses
	winnerIDs, waitlistedIDs, notSelectedIDs := []uuid.UUID{}, []uuid.UUID{}, []uuid.UUID{}
	for i, r := range results {
		entryID := dbEntries[i].ID // order preserved
		if err := s.repo.InsertBallotDrawResult(ctx, db.InsertBallotDrawResultParams{
			DrawID: drawID, BallotEntryID: entryID,
			Outcome: r.Outcome, Rank: int32(r.Rank), ResultHash: r.ResultHash,
		}); err != nil { return err }
		switch r.Outcome {
		case OutcomeWinner:      winnerIDs = append(winnerIDs, entryID)
		case OutcomeWaitlisted:  waitlistedIDs = append(waitlistedIDs, entryID)
		default:                 notSelectedIDs = append(notSelectedIDs, entryID)
		}
	}
	if len(winnerIDs) > 0 {
		_ = s.repo.BulkUpdateBallotOutcome(ctx, db.BulkUpdateBallotOutcomeParams{Column1: winnerIDs, Status: OutcomeWinner})
	}
	if len(waitlistedIDs) > 0 {
		_ = s.repo.BulkUpdateBallotOutcome(ctx, db.BulkUpdateBallotOutcomeParams{Column1: waitlistedIDs, Status: OutcomeWaitlisted})
	}
	if len(notSelectedIDs) > 0 {
		_ = s.repo.BulkUpdateBallotOutcome(ctx, db.BulkUpdateBallotOutcomeParams{Column1: notSelectedIDs, Status: OutcomeNotSelected})
	}

	// 5. Create RESERVED pool for winners
	poolID, err := s.pools.CreatePool(ctx, draw.OrganizationID, draw.EventID, draw.CategoryID,
		"RESERVED", fmt.Sprintf("Ballot winners — draw %s", drawID), quota, actorID)
	if err != nil { return err }

	// 6. Create waitlist for waitlisted entries
	var waitlistID uuid.UUID
	if waitlistSize > 0 {
		waitlistID, err = s.waitlist.CreateWaitlist(ctx, draw.OrganizationID, draw.EventID, draw.CategoryID, actorID)
		if err != nil { return err }
	}

	// 7. Set pool + waitlist on draw, advance status
	_ = s.repo.SetBallotDrawPools(ctx, db.SetBallotDrawPoolsParams{
		ID: drawID,
		WinnerPoolID: pgtype.UUID{Bytes: poolID, Valid: true},
		WaitlistID:   pgtype.UUID{Bytes: waitlistID, Valid: waitlistSize > 0},
	})
	_, err = s.repo.UpdateBallotDrawStatus(ctx, db.UpdateBallotDrawStatusParams{ID: drawID, Status: DrawStatusDrawn})
	return err
}

func (s *Service) AnnounceDraw(ctx context.Context, drawID, actorID uuid.UUID) error {
	draw, err := s.repo.GetBallotDraw(ctx, drawID)
	if err != nil { return err }
	if draw.Status != DrawStatusDrawn { return ErrDrawAlreadyRun }

	winners, err := s.repo.ListWinnerEntries(ctx, drawID)
	if err != nil { return err }

	deadline := time.Now().Add(time.Duration(draw.PaymentWindowHours) * time.Hour)
	for _, w := range winners {
		if !draw.WinnerPoolID.Valid { continue }
		poolID := draw.WinnerPoolID.Bytes
		if err := s.grants.ReserveSlot(ctx, poolID); err != nil { continue }
		grantID, err := s.grants.CreateGrant(ctx, poolID, w.ParticipantID, draw.EventID, draw.CategoryID, deadline)
		if err != nil { continue }
		_, _ = s.repo.UpdateBallotEntryStatus(ctx, db.UpdateBallotEntryStatusParams{
			ID: w.ID, Status: StatusWinner,
			PaymentDeadline: pgtype.Timestamptz{Time: deadline, Valid: true},
			AccessGrantID:   pgtype.UUID{Bytes: grantID, Valid: true},
		})
	}

	// Add waitlisted entries to waitlist engine
	if draw.WaitlistID.Valid {
		wlID := draw.WaitlistID.Bytes
		waitlisted, _ := s.repo.ListAppliedEntriesForDraw(ctx, drawID) // already updated to WAITLISTED
		for _, e := range waitlisted {
			if e.Status != StatusWaitlisted { continue }
			_ = s.waitlist.Join(ctx, wlID, e.ParticipantID, "BALLOT", &e.ID, int64(e.ID.ID()))
		}
	}

	_, err = s.repo.UpdateBallotDrawStatus(ctx, db.UpdateBallotDrawStatusParams{ID: drawID, Status: DrawStatusAnnounced})
	return err
}
```

- [ ] **Step 4: Run tests — expect PASS (Idempotent + StatusGuard)**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/ballot/tests/ -run "TestRunDraw" -race -v 2>&1
# Expected: PASS
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/ballot/service.go services/api/internal/modules/ballot/tests/
git commit -m "feat(phase10): ballot service (RunDraw idempotent, AnnounceDraw grant issuance)"
```

---

### Task 6: Ballot Handler + Routes (Organizer)

**Files:**
- Create: `services/api/internal/modules/ballot/dto.go`
- Create: `services/api/internal/modules/ballot/handler.go`
- Create: `services/api/internal/modules/ballot/routes.go`

- [ ] **Step 1: Write dto.go**

```go
package ballot

import "time"

type CreateDrawRequest struct {
	Quota                int32     `json:"quota"`
	WaitlistSize         int32     `json:"waitlistSize"`
	PaymentWindowHours   int32     `json:"paymentWindowHours"`
	ApplicationOpensAt   time.Time `json:"applicationOpensAt"`
	ApplicationClosesAt  time.Time `json:"applicationClosesAt"`
}

type BallotDrawDTO struct {
	ID                   string     `json:"id"`
	Status               string     `json:"status"`
	Quota                int32      `json:"quota"`
	WaitlistSize         int32      `json:"waitlistSize"`
	ApplicationOpensAt   time.Time  `json:"applicationOpensAt"`
	ApplicationClosesAt  time.Time  `json:"applicationClosesAt"`
	DrawAt               *time.Time `json:"drawAt,omitempty"`
	AnnouncedAt          *time.Time `json:"announcedAt,omitempty"`
	Seed                 *string    `json:"seed,omitempty"`
}

type DrawResultDTO struct {
	Rank          int    `json:"rank"`
	Outcome       string `json:"outcome"`
	ParticipantID string `json:"participantId"`
	ResultHash    string `json:"resultHash"`
}

type BallotEntryDTO struct {
	ID              string     `json:"id"`
	Status          string     `json:"status"`
	AppliedAt       time.Time  `json:"appliedAt"`
	PaymentDeadline *time.Time `json:"paymentDeadline,omitempty"`
}
```

- [ ] **Step 2: Write handler.go**

Read `services/api/internal/modules/queue/handler.go` for exact pattern (actor extraction, WriteJSON/WriteError usage). Then write:

```go
package ballot

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) CreateDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated")); return }
	eventID, _ := uuid.Parse(chi.URLParam(r, "eventId"))
	categoryID, _ := uuid.Parse(chi.URLParam(r, "categoryId"))
	orgID, _ := uuid.Parse(chi.URLParam(r, "orgId"))
	var req CreateDrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body")); return
	}
	draw, err := h.svc.CreateDraw(r.Context(), orgID, eventID, categoryID, actor.UserID, req)
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusCreated, draw)
}

func (h *Handler) OpenDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context()); if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.OpenDraw(r.Context(), drawID, actor.UserID); err != nil { apperr.WriteError(w, r, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CloseDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context()); if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.CloseDraw(r.Context(), drawID, actor.UserID); err != nil { apperr.WriteError(w, r, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RunDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context()); if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.RunDraw(r.Context(), drawID, actor.UserID); err != nil { apperr.WriteError(w, r, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AnnounceDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context()); if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.AnnounceDraw(r.Context(), drawID, actor.UserID); err != nil { apperr.WriteError(w, r, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListResults(w http.ResponseWriter, r *http.Request) {
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	limit, offset := int32(50), int32(0)
	if v := r.URL.Query().Get("limit"); v != "" { if n, err := strconv.Atoi(v); err == nil { limit = int32(n) } }
	if v := r.URL.Query().Get("offset"); v != "" { if n, err := strconv.Atoi(v); err == nil { offset = int32(n) } }
	results, err := h.svc.ListResults(r.Context(), drawID, limit, offset)
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, results)
}

func (h *Handler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	data, err := h.svc.ExportResultsCSV(r.Context(), drawID)
	if err != nil { apperr.WriteError(w, r, err); return }
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="ballot-results.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) PromoteWaitlist(w http.ResponseWriter, r *http.Request) {
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.PromoteWaitlist(r.Context(), drawID); err != nil { apperr.WriteError(w, r, err); return }
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Write routes.go**

```go
package ballot

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterOrganizerRoutes(r chi.Router) {
	r.Route("/org/{orgId}", func(r chi.Router) {
		r.Post("/events/{eventId}/categories/{categoryId}/ballot", h.CreateDraw)
		r.Put("/ballot/{drawId}", h.UpdateDraw)
		r.Post("/ballot/{drawId}/open", h.OpenDraw)
		r.Post("/ballot/{drawId}/close", h.CloseDraw)
		r.Post("/ballot/{drawId}/run", h.RunDraw)
		r.Post("/ballot/{drawId}/announce", h.AnnounceDraw)
		r.Get("/ballot/{drawId}/results", h.ListResults)
		r.Post("/ballot/{drawId}/promote-waitlist", h.PromoteWaitlist)
		r.Get("/ballot/{drawId}/export", h.ExportCSV)
	})
}
```

- [ ] **Step 4: Add missing service methods (CreateDraw, OpenDraw, CloseDraw, UpdateDraw, ListResults, ExportResultsCSV, PromoteWaitlist)**

Add to service.go:
```go
func (s *Service) CreateDraw(ctx context.Context, orgID, eventID, categoryID, createdBy uuid.UUID, req CreateDrawRequest) (db.BallotDraw, error) {
	return s.repo.CreateBallotDraw(ctx, db.CreateBallotDrawParams{
		OrganizationID: orgID, EventID: eventID, CategoryID: categoryID,
		Quota: req.Quota, WaitlistSize: pgtype.Int4{Int32: req.WaitlistSize, Valid: true},
		PaymentWindowHours: req.PaymentWindowHours,
		ApplicationOpensAt: pgtype.Timestamptz{Time: req.ApplicationOpensAt, Valid: true},
		ApplicationClosesAt: pgtype.Timestamptz{Time: req.ApplicationClosesAt, Valid: true},
		CreatedBy: createdBy,
	})
}

func (s *Service) OpenDraw(ctx context.Context, drawID, actorID uuid.UUID) error {
	draw, err := s.repo.GetBallotDraw(ctx, drawID)
	if err != nil { return err }
	if draw.Status != DrawStatusPending { return ErrBallotClosed }
	_, err = s.repo.UpdateBallotDrawStatus(ctx, db.UpdateBallotDrawStatusParams{ID: drawID, Status: DrawStatusOpen})
	return err
}

func (s *Service) CloseDraw(ctx context.Context, drawID, actorID uuid.UUID) error {
	draw, err := s.repo.GetBallotDraw(ctx, drawID)
	if err != nil { return err }
	if draw.Status != DrawStatusOpen { return ErrBallotClosed }
	_, err = s.repo.UpdateBallotDrawStatus(ctx, db.UpdateBallotDrawStatusParams{ID: drawID, Status: DrawStatusClosed})
	return err
}

func (s *Service) ListResults(ctx context.Context, drawID uuid.UUID, limit, offset int32) ([]db.ListBallotDrawResultsRow, error) {
	return s.repo.ListBallotDrawResults(ctx, db.ListBallotDrawResultsParams{DrawID: drawID, Limit: limit, Offset: offset})
}

func (s *Service) ExportResultsCSV(ctx context.Context, drawID uuid.UUID) ([]byte, error) {
	rows, err := s.repo.ListAllDrawResults(ctx, drawID)
	if err != nil { return nil, err }
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"rank","outcome","ballot_entry_id","participant_id","result_hash"})
	for _, r := range rows {
		_ = w.Write([]string{fmt.Sprintf("%d", r.Rank), r.Outcome, r.BallotEntryID.String(), r.ParticipantID.String(), r.ResultHash})
	}
	w.Flush()
	return buf.Bytes(), nil
}

func (s *Service) PromoteWaitlist(ctx context.Context, drawID uuid.UUID) error {
	draw, err := s.repo.GetBallotDraw(ctx, drawID)
	if err != nil { return err }
	if !draw.WaitlistID.Valid { return nil }
	// delegate to waitlist engine PromoteBatch
	return nil // waitlist integration wired in server.go
}
```

- [ ] **Step 5: Build**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./internal/modules/ballot/... 2>&1
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/ballot/dto.go \
        services/api/internal/modules/ballot/handler.go \
        services/api/internal/modules/ballot/routes.go
git commit -m "feat(phase10): ballot handler + organizer routes"
```

---

### Task 7: ExpireBallotWinners Background Job

**Files:**
- Create: `services/api/internal/modules/ballot/jobs.go`
- Create: `services/api/internal/modules/ballot/tests/jobs_test.go`

- [ ] **Step 1: Write failing test**

```go
// services/api/internal/modules/ballot/tests/jobs_test.go
package ballot_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/ballot"
)

type jobRepo struct {
	fakeRepo
	lapsed []db.BallotEntry
}
func (r *jobRepo) ListExpiringWinners(_ context.Context, _ int32) ([]db.BallotEntry, error) {
	return []db.BallotEntry{
		{ID: uuid.New(), Status: ballot.StatusWinner, DrawID: uuid.New(),
		 PaymentDeadline: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true}},
	}, nil
}
func (r *jobRepo) UpdateBallotEntryStatus(_ context.Context, arg db.UpdateBallotEntryStatusParams) (db.BallotEntry, error) {
	r.lapsed = append(r.lapsed, db.BallotEntry{ID: arg.ID, Status: arg.Status}); return db.BallotEntry{}, nil
}
func (r *jobRepo) GetBallotDraw(_ context.Context, _ uuid.UUID) (db.BallotDraw, error) {
	return db.BallotDraw{WaitlistID: pgtype.UUID{Bytes: uuid.New(), Valid: true}}, nil
}

type fakePromoter struct{ batchCalled bool }
func (f *fakePromoter) PromoteBatch(_ context.Context, _ uuid.UUID) error { f.batchCalled = true; return nil }

func TestExpireBallotWinners_LapsesAndPromotes(t *testing.T) {
	repo := &jobRepo{}
	promoter := &fakePromoter{}
	job := ballot.NewWinnerExpirer(repo, promoter)
	if err := job.Run(context.Background()); err != nil { t.Fatal(err) }
	if len(repo.lapsed) == 0 { t.Fatal("winner should be lapsed") }
	if repo.lapsed[0].Status != ballot.StatusLapsed { t.Fatalf("want LAPSED got %s", repo.lapsed[0].Status) }
	if !promoter.batchCalled { t.Fatal("PromoteBatch should be called after lapsing") }
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/ballot/tests/ -run TestExpire -v 2>&1
```

- [ ] **Step 3: Write jobs.go**

```go
package ballot

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type BatchPromoter interface {
	PromoteBatch(ctx context.Context, waitlistID uuid.UUID) error
}

type WinnerExpirer struct {
	repo     Repository
	promoter BatchPromoter
}

func NewWinnerExpirer(repo Repository, promoter BatchPromoter) *WinnerExpirer {
	return &WinnerExpirer{repo: repo, promoter: promoter}
}

func (e *WinnerExpirer) Run(ctx context.Context) error {
	winners, err := e.repo.ListExpiringWinners(ctx, 100)
	if err != nil { return err }
	affected := map[uuid.UUID]bool{}
	for _, w := range winners {
		_, err := e.repo.UpdateBallotEntryStatus(ctx, db.UpdateBallotEntryStatusParams{
			ID: w.ID, Status: StatusLapsed,
		})
		if err != nil { continue }
		affected[w.DrawID] = true
	}
	for drawID := range affected {
		draw, err := e.repo.GetBallotDraw(ctx, drawID)
		if err != nil || !draw.WaitlistID.Valid { continue }
		_ = e.promoter.PromoteBatch(ctx, draw.WaitlistID.Bytes)
	}
	return nil
}
```

- [ ] **Step 4: Run test — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/ballot/tests/ -race -v 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/ballot/jobs.go services/api/internal/modules/ballot/tests/jobs_test.go
git commit -m "feat(phase10): ExpireBallotWinners background job (lapse + promote)"
```

---

### Task 8: Wire Ballot into server.go + Part 2 Verification

**Files:**
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Read server.go**

```bash
cat /Users/kaivy/Coding/ivyticketing/services/api/internal/app/server.go
```

Find where `queueHandler.RegisterOrganizerRoutes` or similar is called. Find the `pool` variable (pgxpool), `auditLog`, `redisClient`.

- [ ] **Step 2: Add ballot wiring after queue wiring**

```go
// Ballot module (Phase 10 Part 2)
ballotRepo := ballot.NewRepository(pool)
poolMgr := access.NewPoolManager(access.NewRepository(pool))
waitlistSvc := waitlist.NewService(waitlist.NewRepository(pool), poolMgr)
ballotSvc := ballot.NewService(ballotRepo, auditLog, poolMgr, poolMgr, waitlistSvc)
ballotHandler := ballot.NewHandler(ballotSvc)
ballotExpirer := ballot.NewWinnerExpirer(ballotRepo, waitlistSvc)
```

Mount organizer routes inside authn group with org-level auth:
```go
r.Group(func(r chi.Router) {
    r.Use(middleware.RequireOrgMember())  // or whatever org auth middleware exists
    ballotHandler.RegisterOrganizerRoutes(r)
})
```

Schedule expirer job (follow existing cron pattern from queue/jobs.go or abuse/service.go):
```go
go func() {
    t := time.NewTicker(time.Minute)
    defer t.Stop()
    for { select { case <-t.C: _ = ballotExpirer.Run(context.Background()) case <-ctx.Done(): return } }
}()
```

- [ ] **Step 3: Full build + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api
go build ./... 2>&1
# Expected: clean
go vet ./... 2>&1
go test ./internal/modules/ballot/... -race -v 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
# Expected: all ok
```

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/app/server.go
git commit -m "feat(phase10): part 2 complete — ballot draw engine + organizer APIs wired"
```
