# Phase 8 Plan — Part 2: Queue Core + Waiting Room

> Part of the Phase 8 implementation plan. Index: [2026-06-08-phase8-queue-war-system.md](2026-06-08-phase8-queue-war-system.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** New files + additive changes. Assumes Part 1 exists (registration foundation, gate seam).

This part builds queue persistence (tokens/admissions/control), the Redis sorted-set adapter, scoring, idempotent token issuance, and WAR_QUEUE join/status endpoints. No release engine yet (Part 3). WAR_QUEUE only (RANDOMIZED/HYBRID scoring in Part 4).

---

## Task 8: Queue migrations (tokens, admissions, control)

**Files:**
- Create: `database/migrations/00022_create_queue_tokens.sql`
- Create: `database/migrations/00023_create_queue_admissions.sql`
- Create: `database/migrations/00024_create_queue_control.sql`

> Number after Part 1's migrations. Part 1 used 00020 (settings) + 00021 (seed registration.manage). Queue migrations = 00022/00023/00024. The `queue.manage` seed is Task 14 (00025).

- [ ] **Step 1: Write queue_tokens migration**

`database/migrations/00022_create_queue_tokens.sql`:
```sql
-- +goose Up
CREATE TABLE queue_tokens (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    participant_id  uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status          text NOT NULL DEFAULT 'WAITING',
    pool            text NOT NULL DEFAULT 'FIFO',
    score           bigint NOT NULL,
    joined_at       timestamptz NOT NULL DEFAULT now(),
    allowed_at      timestamptz,
    expired_at      timestamptz,
    completed_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT qt_status_check CHECK (status IN ('WAITING','ALLOWED','EXPIRED','COMPLETED','BLOCKED')),
    CONSTRAINT qt_pool_check CHECK (pool IN ('PRESALE','FIFO')),
    CONSTRAINT qt_event_participant_unique UNIQUE (event_id, participant_id)
);
CREATE INDEX idx_queue_tokens_event_status ON queue_tokens(event_id, status);
CREATE INDEX idx_queue_tokens_event_pool_score ON queue_tokens(event_id, pool, score);

-- +goose Down
DROP TABLE queue_tokens;
```

- [ ] **Step 2: Write queue_admissions migration**

`database/migrations/00023_create_queue_admissions.sql`:
```sql
-- +goose Up
CREATE TABLE queue_admissions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id           uuid NOT NULL REFERENCES queue_tokens(id) ON DELETE CASCADE,
    event_id           uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    participant_id     uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    checkout_expires_at timestamptz NOT NULL,
    status             text NOT NULL DEFAULT 'ACTIVE',
    created_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT qa_status_check CHECK (status IN ('ACTIVE','CONSUMED','EXPIRED'))
);
CREATE INDEX idx_queue_admissions_expiry ON queue_admissions(event_id, status, checkout_expires_at);
CREATE UNIQUE INDEX uq_admission_active ON queue_admissions(token_id) WHERE status = 'ACTIVE';

-- +goose Down
DROP TABLE queue_admissions;
```

- [ ] **Step 3: Write queue_control migration**

`database/migrations/00024_create_queue_control.sql`:
```sql
-- +goose Up
CREATE TABLE queue_control (
    event_id             uuid PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
    state                text NOT NULL DEFAULT 'RUNNING',
    release_rate         integer NOT NULL DEFAULT 100,
    randomization_seed   text,
    sale_start_at        timestamptz,
    presale_pool_open_at timestamptz,
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT qc_state_check CHECK (state IN ('RUNNING','PAUSED')),
    CONSTRAINT qc_rate_check CHECK (release_rate >= 0)
);

-- +goose Down
DROP TABLE queue_control;
```

- [ ] **Step 4: Roundtrip**

```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: clean up/down/up for all three.

- [ ] **Step 5: Commit**

```bash
git add database/migrations/00022_create_queue_tokens.sql database/migrations/00023_create_queue_admissions.sql database/migrations/00024_create_queue_control.sql
git commit -m "feat(phase8): queue migrations (tokens, admissions, control)"
```

---

## Task 9: sqlc queries for queue

**Files:**
- Create: `database/queries/queue.sql`
- Regenerate: `services/api/internal/db/*`

- [ ] **Step 1: Write queries**

`database/queries/queue.sql`:
```sql
-- name: CreateQueueToken :one
INSERT INTO queue_tokens (organization_id, event_id, participant_id, status, pool, score)
VALUES ($1,$2,$3,'WAITING',$4,$5)
ON CONFLICT (event_id, participant_id) DO NOTHING
RETURNING *;

-- name: GetQueueTokenByEventParticipant :one
SELECT * FROM queue_tokens WHERE event_id = $1 AND participant_id = $2;

-- name: GetQueueTokenByID :one
SELECT * FROM queue_tokens WHERE id = $1;

-- name: ListWaitingTokens :many
SELECT * FROM queue_tokens
WHERE event_id = $1 AND status = 'WAITING'
ORDER BY pool DESC, score ASC
LIMIT $2;

-- name: MarkTokenAllowed :one
UPDATE queue_tokens SET status='ALLOWED', allowed_at=now(), updated_at=now()
WHERE id = $1 AND status = 'WAITING'
RETURNING *;

-- name: MarkTokenCompleted :exec
UPDATE queue_tokens SET status='COMPLETED', completed_at=now(), updated_at=now()
WHERE id = $1 AND status = 'ALLOWED';

-- name: RequeueToken :exec
UPDATE queue_tokens SET status='WAITING', score=$2, allowed_at=NULL, updated_at=now()
WHERE id = $1 AND status = 'ALLOWED';

-- name: CountTokensByStatus :one
SELECT count(*) FROM queue_tokens WHERE event_id = $1 AND status = $2;

-- name: ListWaitingTokensAll :many
SELECT * FROM queue_tokens WHERE event_id = $1 AND status = 'WAITING' ORDER BY pool DESC, score ASC;

-- name: CreateAdmission :one
INSERT INTO queue_admissions (token_id, event_id, participant_id, checkout_expires_at, status)
VALUES ($1,$2,$3,$4,'ACTIVE')
RETURNING *;

-- name: GetActiveAdmissionByParticipant :one
SELECT * FROM queue_admissions
WHERE event_id = $1 AND participant_id = $2 AND status = 'ACTIVE';

-- name: ConsumeAdmission :exec
UPDATE queue_admissions SET status='CONSUMED' WHERE id = $1 AND status = 'ACTIVE';

-- name: ListExpiredActiveAdmissions :many
SELECT * FROM queue_admissions
WHERE status = 'ACTIVE' AND checkout_expires_at < now()
ORDER BY checkout_expires_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: ExpireAdmission :exec
UPDATE queue_admissions SET status='EXPIRED' WHERE id = $1 AND status = 'ACTIVE';

-- name: GetQueueControl :one
SELECT * FROM queue_control WHERE event_id = $1;

-- name: UpsertQueueControl :one
INSERT INTO queue_control (event_id, state, release_rate, randomization_seed, sale_start_at, presale_pool_open_at)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (event_id) DO UPDATE SET
    state = EXCLUDED.state,
    release_rate = EXCLUDED.release_rate,
    randomization_seed = EXCLUDED.randomization_seed,
    sale_start_at = EXCLUDED.sale_start_at,
    presale_pool_open_at = EXCLUDED.presale_pool_open_at,
    updated_at = now()
RETURNING *;

-- name: SetQueueState :exec
UPDATE queue_control SET state=$2, updated_at=now() WHERE event_id=$1;

-- name: SetReleaseRate :exec
UPDATE queue_control SET release_rate=$2, updated_at=now() WHERE event_id=$1;

-- name: ListEventsWithRunningQueue :many
SELECT event_id FROM queue_control WHERE state = 'RUNNING';
```

- [ ] **Step 2: Regenerate + build**

```bash
make sqlc && cd services/api && go build ./internal/db/...; cd ../..
```
Expected: `QueueToken`, `QueueAdmission`, `QueueControl` structs + methods; builds clean.

- [ ] **Step 3: Commit**

```bash
git add database/queries/queue.sql services/api/internal/db
git commit -m "feat(phase8): queue sqlc queries"
```

---

## Task 10: Redis sorted-set adapter (platform/queue)

**Files:**
- Create: `services/api/internal/platform/queue/queue.go`
- Create: `services/api/internal/platform/queue/queue_test.go`

- [ ] **Step 1: Write the failing test** (uses miniredis if available; else skip without REDIS_TEST_URL)

Create `services/api/internal/platform/queue/queue_test.go`:
```go
package queue

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

func testClient(t *testing.T) *redis.Client {
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL not set")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	c := redis.NewClient(opt)
	t.Cleanup(func() { c.Close() })
	return c
}

func TestWaitingAddRankRange(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	a := New(c)
	ev := "evt-test-" + t.Name()
	t.Cleanup(func() { c.Del(ctx, waitingKey(ev), allowedKey(ev)) })

	a.AddWaiting(ctx, ev, "u1", 100)
	a.AddWaiting(ctx, ev, "u2", 200)

	rank, err := a.WaitingRank(ctx, ev, "u2")
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if rank != 1 {
		t.Fatalf("u2 rank = %d, want 1", rank)
	}
	members, err := a.WaitingRangeN(ctx, ev, 10)
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if len(members) != 2 || members[0] != "u1" {
		t.Fatalf("range = %v, want [u1 u2]", members)
	}
}

func TestMoveToAllowed(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	a := New(c)
	ev := "evt-test-" + t.Name()
	t.Cleanup(func() { c.Del(ctx, waitingKey(ev), allowedKey(ev)) })

	a.AddWaiting(ctx, ev, "u1", 100)
	if err := a.MoveToAllowed(ctx, ev, "u1", 9999); err != nil {
		t.Fatalf("move: %v", err)
	}
	cnt, _ := a.WaitingCount(ctx, ev)
	if cnt != 0 {
		t.Fatalf("waiting count = %d, want 0", cnt)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd services/api && go test ./internal/platform/queue/ -v; cd ../..
```
Expected: FAIL — `New`/`AddWaiting` undefined (or SKIP if no REDIS_TEST_URL — then it must at least compile-fail until implemented).

- [ ] **Step 3: Implement queue.go**

```go
// Package queue provides Redis sorted-set primitives for the waiting room.
// Postgres is the durable source of truth; these structures are rebuildable.
package queue

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Adapter struct {
	c *redis.Client
}

func New(c *redis.Client) *Adapter { return &Adapter{c: c} }

func waitingKey(eventID string) string { return fmt.Sprintf("queue:%s:waiting", eventID) }
func allowedKey(eventID string) string { return fmt.Sprintf("queue:%s:allowed", eventID) }

// AddWaiting adds/updates a member in the waiting sorted set with the given score.
func (a *Adapter) AddWaiting(ctx context.Context, eventID, member string, score int64) error {
	return a.c.ZAdd(ctx, waitingKey(eventID), redis.Z{Score: float64(score), Member: member}).Err()
}

// WaitingRank returns the 0-based position of member in the waiting set.
func (a *Adapter) WaitingRank(ctx context.Context, eventID, member string) (int64, error) {
	return a.c.ZRank(ctx, waitingKey(eventID), member).Result()
}

// WaitingRangeN returns up to n members ordered by score ascending.
func (a *Adapter) WaitingRangeN(ctx context.Context, eventID string, n int64) ([]string, error) {
	return a.c.ZRange(ctx, waitingKey(eventID), 0, n-1).Result()
}

// WaitingCount returns the number of waiting members.
func (a *Adapter) WaitingCount(ctx context.Context, eventID string) (int64, error) {
	return a.c.ZCard(ctx, waitingKey(eventID)).Result()
}

// AllowedCount returns the number of allowed members.
func (a *Adapter) AllowedCount(ctx context.Context, eventID string) (int64, error) {
	return a.c.ZCard(ctx, allowedKey(eventID)).Result()
}

// MoveToAllowed atomically removes from waiting and adds to allowed (score = checkout expiry unix).
func (a *Adapter) MoveToAllowed(ctx context.Context, eventID, member string, expiresAtUnix int64) error {
	pipe := a.c.TxPipeline()
	pipe.ZRem(ctx, waitingKey(eventID), member)
	pipe.ZAdd(ctx, allowedKey(eventID), redis.Z{Score: float64(expiresAtUnix), Member: member})
	_, err := pipe.Exec(ctx)
	return err
}

// MoveToWaiting moves a member back from allowed to waiting (requeue, new score).
func (a *Adapter) MoveToWaiting(ctx context.Context, eventID, member string, score int64) error {
	pipe := a.c.TxPipeline()
	pipe.ZRem(ctx, allowedKey(eventID), member)
	pipe.ZAdd(ctx, waitingKey(eventID), redis.Z{Score: float64(score), Member: member})
	_, err := pipe.Exec(ctx)
	return err
}

// RemoveAllowed removes a member from the allowed set (on checkout complete).
func (a *Adapter) RemoveAllowed(ctx context.Context, eventID, member string) error {
	return a.c.ZRem(ctx, allowedKey(eventID), member).Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
REDIS_TEST_URL=redis://localhost:6379 go test ./internal/platform/queue/ -v   # from services/api
```
Expected: PASS if Redis available; SKIP otherwise (acceptable — note it).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/platform/queue
git commit -m "feat(phase8): redis sorted-set adapter for queue"
```

---

## Task 11: Queue scoring (FIFO; randomized stub for Part 4)

**Files:**
- Create: `services/api/internal/modules/queue/score.go`
- Create: `services/api/internal/modules/queue/score_test.go`

- [ ] **Step 1: Write the failing test**

```go
package queue

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFifoScore_Monotonic(t *testing.T) {
	t0 := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	s1 := FifoScore(t0)
	s2 := FifoScore(t0.Add(time.Nanosecond))
	if s2 <= s1 {
		t.Fatalf("expected monotonic increasing scores, got %d then %d", s1, s2)
	}
}

func TestPresaleScore_Deterministic(t *testing.T) {
	seed := "seed-123"
	u := uuid.New()
	a := PresaleScore(seed, u)
	b := PresaleScore(seed, u)
	if a != b {
		t.Fatalf("presale score not deterministic: %d != %d", a, b)
	}
}

func TestPresaleScore_SeedSensitive(t *testing.T) {
	u := uuid.New()
	if PresaleScore("seed-a", u) == PresaleScore("seed-b", u) {
		t.Fatal("different seeds should (almost always) produce different scores")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd services/api && go test ./internal/modules/queue/ -run 'TestFifoScore|TestPresaleScore' -v; cd ../..
```
Expected: FAIL — `FifoScore`/`PresaleScore` undefined.

- [ ] **Step 3: Implement score.go**

```go
package queue

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

// FifoScore returns a monotonic score from wall-clock time (FIFO ordering).
func FifoScore(now time.Time) int64 {
	return now.UnixNano()
}

// PresaleScore returns a deterministic, seed-based pseudo-random score for a
// participant. Same (seed, participant) → same score (reproducible/auditable).
// The score range is positive int64 so it sorts before/after FIFO scores
// depending on pool ordering (PRESALE pool is ranked before FIFO pool).
func PresaleScore(seed string, participantID uuid.UUID) int64 {
	h := sha256.New()
	h.Write([]byte(seed))
	h.Write(participantID[:])
	sum := h.Sum(nil)
	// take first 8 bytes as unsigned, mask to positive int64
	v := binary.BigEndian.Uint64(sum[:8])
	return int64(v >> 1) // ensure non-negative
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd services/api && go test ./internal/modules/queue/ -run 'TestFifoScore|TestPresaleScore' -v; cd ../..
```
Expected: PASS (all 3).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/queue/score.go services/api/internal/modules/queue/score_test.go
git commit -m "feat(phase8): queue scoring (FIFO + deterministic presale)"
```

---

## Task 12: Queue model + repository + store

**Files:**
- Create: `services/api/internal/modules/queue/model.go`
- Create: `services/api/internal/modules/queue/repository.go`
- Create: `services/api/internal/modules/queue/store.go`
- Create: `services/api/internal/modules/queue/errors.go`

- [ ] **Step 1: model.go**

```go
package queue

const (
	StatusWaiting   = "WAITING"
	StatusAllowed   = "ALLOWED"
	StatusExpired   = "EXPIRED"
	StatusCompleted = "COMPLETED"
	StatusBlocked   = "BLOCKED"

	PoolPresale = "PRESALE"
	PoolFifo    = "FIFO"

	AdmissionActive   = "ACTIVE"
	AdmissionConsumed = "CONSUMED"
	AdmissionExpired  = "EXPIRED"

	StateRunning = "RUNNING"
	StatePaused  = "PAUSED"
)
```

- [ ] **Step 2: errors.go**

```go
package queue

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrNotEnabled    = apperr.New(http.StatusConflict, "QUEUE_NOT_ENABLED", "queue is not enabled for this event")
	ErrTokenNotFound = apperr.New(http.StatusNotFound, "QUEUE_TOKEN_NOT_FOUND", "queue token not found")
	ErrNotAllowed    = apperr.New(http.StatusForbidden, "QUEUE_NOT_ALLOWED", "not yet released to checkout")
	ErrAdmissionRequired = apperr.New(http.StatusForbidden, "ADMISSION_REQUIRED", "queue admission required")
	ErrAdmissionExpired  = apperr.New(http.StatusForbidden, "ADMISSION_EXPIRED", "checkout window expired")
)
```

- [ ] **Step 3: repository.go** (sqlc wrapper; ExecTx pattern from payments)

```go
package queue

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error

	CreateToken(ctx context.Context, arg db.CreateQueueTokenParams) (db.QueueToken, error)
	GetTokenByEventParticipant(ctx context.Context, eventID, participantID uuid.UUID) (db.QueueToken, error)
	GetTokenByID(ctx context.Context, id uuid.UUID) (db.QueueToken, error)
	ListWaiting(ctx context.Context, arg db.ListWaitingTokensParams) ([]db.QueueToken, error)
	MarkAllowed(ctx context.Context, id uuid.UUID) (db.QueueToken, error)
	MarkCompleted(ctx context.Context, id uuid.UUID) error
	Requeue(ctx context.Context, arg db.RequeueTokenParams) error
	CountByStatus(ctx context.Context, arg db.CountTokensByStatusParams) (int64, error)

	CreateAdmission(ctx context.Context, arg db.CreateAdmissionParams) (db.QueueAdmission, error)
	GetActiveAdmission(ctx context.Context, arg db.GetActiveAdmissionByParticipantParams) (db.QueueAdmission, error)
	ConsumeAdmission(ctx context.Context, id uuid.UUID) error
	ListExpiredAdmissions(ctx context.Context, limit int32) ([]db.QueueAdmission, error)
	ExpireAdmission(ctx context.Context, id uuid.UUID) error

	GetControl(ctx context.Context, eventID uuid.UUID) (db.QueueControl, error)
	UpsertControl(ctx context.Context, arg db.UpsertQueueControlParams) (db.QueueControl, error)
	SetState(ctx context.Context, arg db.SetQueueStateParams) error
	SetRate(ctx context.Context, arg db.SetReleaseRateParams) error
	ListRunningEvents(ctx context.Context) ([]uuid.UUID, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{pool: pool, q: db.New(pool)} }

func (r *sqlcRepo) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(&sqlcRepo{pool: r.pool, q: db.New(tx)}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// ... thin pass-through methods for each interface method, calling r.q.<Generated>.
```

> Write every pass-through method calling the generated `r.q.*`. Confirm each generated method name + param struct in `services/api/internal/db/queue.sql.go` (e.g., `ListWaitingTokensParams{EventID, Limit}`, `CountTokensByStatusParams{EventID, Status}`, `RequeueTokenParams{ID, Score}`, `GetActiveAdmissionByParticipantParams{EventID, ParticipantID}`, `SetQueueStateParams{EventID, State}`, `SetReleaseRateParams{EventID, ReleaseRate}`). `ListRunningEvents` wraps `ListEventsWithRunningQueue`. `ListExpiredAdmissions` wraps `ListExpiredActiveAdmissions` (param: limit int32).

- [ ] **Step 4: store.go** (write-through Redis helper bound to the adapter)

```go
package queue

import (
	"context"

	pq "github.com/varin/ivyticketing/services/api/internal/platform/queue"
)

// Store wraps the Redis adapter with event-id string conversion conveniences.
type Store struct {
	a *pq.Adapter
}

func NewStore(a *pq.Adapter) *Store { return &Store{a: a} }

func (s *Store) AddWaiting(ctx context.Context, eventID, participantID string, score int64) error {
	return s.a.AddWaiting(ctx, eventID, participantID, score)
}
func (s *Store) Rank(ctx context.Context, eventID, participantID string) (int64, error) {
	return s.a.WaitingRank(ctx, eventID, participantID)
}
func (s *Store) RangeN(ctx context.Context, eventID string, n int64) ([]string, error) {
	return s.a.WaitingRangeN(ctx, eventID, n)
}
func (s *Store) MoveToAllowed(ctx context.Context, eventID, participantID string, expiresUnix int64) error {
	return s.a.MoveToAllowed(ctx, eventID, participantID, expiresUnix)
}
func (s *Store) MoveToWaiting(ctx context.Context, eventID, participantID string, score int64) error {
	return s.a.MoveToWaiting(ctx, eventID, participantID, score)
}
func (s *Store) RemoveAllowed(ctx context.Context, eventID, participantID string) error {
	return s.a.RemoveAllowed(ctx, eventID, participantID)
}
func (s *Store) WaitingCount(ctx context.Context, eventID string) (int64, error) {
	return s.a.WaitingCount(ctx, eventID)
}
func (s *Store) AllowedCount(ctx context.Context, eventID string) (int64, error) {
	return s.a.AllowedCount(ctx, eventID)
}
```

- [ ] **Step 5: Build**

```bash
cd services/api && go build ./internal/modules/queue/...; cd ../..
```
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/queue/model.go services/api/internal/modules/queue/errors.go services/api/internal/modules/queue/repository.go services/api/internal/modules/queue/store.go
git commit -m "feat(phase8): queue model, repository, redis store"
```

---

## Task 13: Queue service — join + status (WAR_QUEUE)

**Files:**
- Create: `services/api/internal/modules/queue/service.go`
- Create: `services/api/internal/modules/queue/dto.go`
- Create: `services/api/internal/modules/queue/tests/service_test.go`

- [ ] **Step 1: dto.go**

```go
package queue

type JoinResponse struct {
	TokenID  string `json:"tokenId"`
	Status   string `json:"status"`
	Position int64  `json:"position"`
}

type StatusResponse struct {
	TokenID         string `json:"tokenId"`
	Status          string `json:"status"`
	Position        int64  `json:"position"`
	EstimatedWaitSeconds int64 `json:"estimatedWaitSeconds"`
	SystemState     string `json:"systemState"`
	AdmissionToken  string `json:"admissionToken,omitempty"`
	CheckoutExpiresAt string `json:"checkoutExpiresAt,omitempty"`
}
```

- [ ] **Step 2: Write the failing test** (fake repo + fake store)

Create `services/api/internal/modules/queue/tests/service_test.go`. Test join idempotency: calling Join twice for the same (event, participant) returns the same token. Use a fake repo where `CreateToken` returns `pgx.ErrNoRows` on the second call (simulating ON CONFLICT DO NOTHING), and `GetTokenByEventParticipant` returns the existing token.

```go
package queue_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/queue"
)

// minimal fake repo — implement queue.Repository; only methods used by Join/Status.
// (Write full fake satisfying the interface; unused methods can return zero values.)

func TestJoin_Idempotent(t *testing.T) {
	// First join inserts; second join hits ON CONFLICT (CreateToken → ErrNoRows) and
	// returns the existing token via GetTokenByEventParticipant. Assert same token id.
	t.Skip("implement fake repo satisfying queue.Repository, then assert idempotent token id")
	_ = pgx.ErrNoRows
	_ = db.QueueToken{}
	_ = uuid.New
	_ = context.Background
	_ = queue.StatusWaiting
}
```

> Replace the `t.Skip` with a real fake. The fake must implement ALL `queue.Repository` methods (return zero values for unused). `Join` logic: `CreateToken` → if `pgx.ErrNoRows`, call `GetTokenByEventParticipant` and return it (idempotent). The store `AddWaiting` is also called; fake store records calls. Keep the test focused on idempotency (same token id on 2nd join) and that a WAITING token reports a position.

- [ ] **Step 3: Implement service.go**

```go
package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type Service struct {
	repo  Repository
	store *Store
	audit AuditRecorder
}

func NewService(repo Repository, store *Store, recorder AuditRecorder) *Service {
	return &Service{repo: repo, store: store, audit: recorder}
}

// Join issues (or returns existing) a queue token for the participant. Idempotent.
func (s *Service) Join(ctx context.Context, orgID, eventID, participantID uuid.UUID) (JoinResponse, error) {
	score := FifoScore(time.Now()) // WAR_QUEUE; RANDOMIZED/HYBRID scoring in Part 4
	tok, err := s.repo.CreateToken(ctx, db.CreateQueueTokenParams{
		OrganizationID: orgID, EventID: eventID, ParticipantID: participantID,
		Pool: PoolFifo, Score: score,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// already queued → return existing (idempotent; refresh/reconnect safe)
		tok, err = s.repo.GetTokenByEventParticipant(ctx, eventID, participantID)
		if err != nil {
			return JoinResponse{}, err
		}
	} else if err != nil {
		return JoinResponse{}, err
	} else {
		_ = s.store.AddWaiting(ctx, eventID.String(), participantID.String(), tok.Score)
		if s.audit != nil {
			oid := orgID
			aid := participantID
			s.audit.Record(ctx, audit.Entry{OrganizationID: &oid, ActorUserID: &aid,
				Action: "QUEUE_TOKEN_ISSUED", TargetType: "queue_token", TargetID: tok.ID.String(),
				Metadata: map[string]any{"eventId": eventID.String()}})
		}
	}
	pos, _ := s.store.Rank(ctx, eventID.String(), participantID.String())
	return JoinResponse{TokenID: tok.ID.String(), Status: tok.Status, Position: pos}, nil
}

// Status returns the participant's queue position and state.
func (s *Service) Status(ctx context.Context, eventID, participantID uuid.UUID) (StatusResponse, error) {
	tok, err := s.repo.GetTokenByEventParticipant(ctx, eventID, participantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return StatusResponse{}, ErrTokenNotFound
	}
	if err != nil {
		return StatusResponse{}, err
	}

	ctrl, err := s.repo.GetControl(ctx, eventID)
	state := StateRunning
	rate := int32(100)
	if err == nil {
		state = ctrl.State
		rate = ctrl.ReleaseRate
	}

	resp := StatusResponse{TokenID: tok.ID.String(), Status: tok.Status, SystemState: state}
	if tok.Status == StatusWaiting {
		pos, _ := s.store.Rank(ctx, eventID.String(), participantID.String())
		resp.Position = pos
		if rate > 0 {
			resp.EstimatedWaitSeconds = pos / int64(rate) // best-effort, in intervals; refine in handler with interval
		}
	}
	if tok.Status == StatusAllowed {
		adm, err := s.repo.GetActiveAdmission(ctx, db.GetActiveAdmissionByParticipantParams{EventID: eventID, ParticipantID: participantID})
		if err == nil {
			resp.AdmissionToken = adm.ID.String()
			resp.CheckoutExpiresAt = adm.CheckoutExpiresAt.Time.Format(time.RFC3339)
		}
	}
	return resp, nil
}
```

> `ReleaseRate` generated type is `int32` (integer column). Confirm in generated struct. The estimate is best-effort; the handler can multiply by the release interval for a seconds value, or leave as documented approximation. Keep simple.

- [ ] **Step 4: Run tests**

```bash
cd services/api && go test ./internal/modules/queue/... -race; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/queue/service.go services/api/internal/modules/queue/dto.go services/api/internal/modules/queue/tests
git commit -m "feat(phase8): queue join + status service (idempotent token)"
```

---

## Task 14: Queue admission check + anti-bot guard stub + handler/routes (participant)

**Files:**
- Create: `services/api/internal/modules/queue/admission.go` (CheckAdmission only; expiry worker in Part 3)
- Create: `services/api/internal/modules/queue/guard.go`
- Create: `services/api/internal/modules/queue/handler.go`
- Create: `services/api/internal/modules/queue/routes.go`

- [ ] **Step 1: admission.go — CheckAdmission (implements registration.QueueAdmitter)**

```go
package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// CheckAdmission verifies the participant holds an ACTIVE, unexpired admission for
// the event. Implements registration.QueueAdmitter. The admissionToken (header
// X-Queue-Token) carries the admission id; we validate ownership + expiry.
func (s *Service) CheckAdmission(ctx context.Context, participantID, eventID uuid.UUID, admissionToken string) error {
	adm, err := s.repo.GetActiveAdmission(ctx, db.GetActiveAdmissionByParticipantParams{
		EventID: eventID, ParticipantID: participantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAdmissionRequired
	}
	if err != nil {
		return err
	}
	if admissionToken == "" || adm.ID.String() != admissionToken {
		return ErrAdmissionRequired
	}
	if time.Now().After(adm.CheckoutExpiresAt.Time) {
		return ErrAdmissionExpired
	}
	return nil
}

// ConsumeOnCheckout marks the admission consumed and the token completed.
// Called by the orders flow after a successful checkout (Part 3 wires this).
func (s *Service) ConsumeOnCheckout(ctx context.Context, participantID, eventID uuid.UUID) error {
	adm, err := s.repo.GetActiveAdmission(ctx, db.GetActiveAdmissionByParticipantParams{
		EventID: eventID, ParticipantID: participantID,
	})
	if err != nil {
		return err
	}
	if err := s.repo.ConsumeAdmission(ctx, adm.ID); err != nil {
		return err
	}
	if err := s.repo.MarkCompleted(ctx, adm.TokenID); err != nil {
		return err
	}
	_ = s.store.RemoveAllowed(ctx, eventID.String(), participantID.String())
	return nil
}
```

> NOTE on consume timing: the spec consumes admission after checkout success. Part 3 decides whether to call `ConsumeOnCheckout` from the gate (after Admit) or post-commit. For Part 2, just define the method. Gate's `CheckAdmission` is read-only (validation), which is what runs inside the orders tx.

- [ ] **Step 2: guard.go — anti-bot stub (Phase 9 fills)**

```go
package queue

import "net/http"

// EntryGuard is a no-op middleware placeholder for Phase 9 (Turnstile, rate limit,
// duplicate detection). It currently passes every request through unchanged.
func EntryGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Phase 9: verify Turnstile token, apply rate limit, duplicate detection.
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 3: handler.go (participant join/status)**

```go
package queue

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func caller(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	resp, err := h.svc.JoinByEvent(r.Context(), eventID, uid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	resp, err := h.svc.Status(r.Context(), eventID, uid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, resp)
}
```

> `Join` handler calls `JoinByEvent(ctx, eventID, uid)` — add this convenience on Service that looks up the event's organization_id (via a repo/event lookup) then calls `Join(ctx, orgID, eventID, uid)`. The queue must also check the event's resolved mode is a queue mode before issuing a token (else `ErrNotEnabled`). Add a dependency on registration's resolver OR pass mode check via a small interface. SIMPLEST: queue.Service holds a `ModeResolver` interface `ResolveForCheckout(ctx, eventID, categoryID)` — but join is per-event not per-category. Add a per-event resolver `ResolveEventMode(ctx, eventID) (string, error)` to registration.Service and a matching interface in queue. Wire in Part 3 server. For Part 2, define the interface and guard; if resolver is nil, allow (tested via fake).

- [ ] **Step 4: routes.go (participant; EntryGuard stub applied)**

```go
package queue

import "github.com/go-chi/chi/v5"

// RegisterRoutes mounts participant queue endpoints at the authn level.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(EntryGuard).Post("/events/{eventId}/queue/join", h.Join)
	r.Get("/events/{eventId}/queue/status", h.Status)
}
```

- [ ] **Step 5: Build + test**

```bash
cd services/api && go build ./internal/modules/queue/... && go test ./internal/modules/queue/... -race; cd ../..
```
Expected: clean + green.

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/queue/admission.go services/api/internal/modules/queue/guard.go services/api/internal/modules/queue/handler.go services/api/internal/modules/queue/routes.go
git commit -m "feat(phase8): queue admission check, anti-bot guard stub, participant handler/routes"
```

---

## Task 15: Wire queue module into server (join/status live; release in Part 3)

**Files:**
- Modify: `services/api/internal/app/server.go`
- Modify: `services/api/cmd/api/main.go`

- [ ] **Step 1: Pass concrete redis to NewRouter**

In `server.go`, change `NewRouter` signature to accept the concrete redis client alongside the `system.Checker`:
```go
func NewRouter(cfg Config, log *slog.Logger, pool *pgxpool.Pool, pg, rdb system.Checker, redisClient *redis.Client) (http.Handler, error) {
```
Add import `"github.com/redis/go-redis/v9"`. In `cmd/api/main.go`, pass `rdb.Client`:
```go
	handler, err := app.NewRouter(cfg, log, pg.Pool, pg, rdb, rdb.Client)
```

> Check `NewRouter`'s other callers (integration `newTestServer` builds the router via `app.NewRouter`). Update `helpers_test.go` to pass a redis client. For integration tests without Redis, pass a client pointed at `REDIS_URL`/`REDIS_TEST_URL`, or make the queue store tolerate a nil-ish client by skipping. SIMPLEST: integration harness sets `REDIS_TEST_URL`; if not set, queue-mode tests skip but NORMAL tests still run. Update `newTestServer` to build `redis.NewClient` from an env url and pass it.

- [ ] **Step 2: Build queue module wiring in server.go**

After the registration wiring (Part 1), add:
```go
	queueAdapter := platformqueue.New(redisClient)
	queueStore := queuemod.NewStore(queueAdapter)
	queueRepo := queuemod.NewRepository(pool)
	queueSvc := queuemod.NewService(queueRepo, queueStore, auditLog)
	queueHandler := queuemod.NewHandler(queueSvc)
```
Add imports `queuemod ".../modules/queue"` and `platformqueue ".../platform/queue"`. Mount participant routes in the authn group:
```go
			queueHandler.RegisterRoutes(r)
```

> The registration gate's queue admitter is still `nil` here (set in Part 3 where checkout consumption + release land). Join/status work now; checkout via queue still returns REGISTRATION_MODE_NOT_AVAILABLE until Part 3 wires `registrationmod.NewGate(registrationSvc, queueSvc)`.

- [ ] **Step 3: Build + smoke test**

```bash
cd services/api && go build ./...; cd ../..
```
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/app/server.go services/api/cmd/api/main.go services/api/tests/integration/helpers_test.go
git commit -m "feat(phase8): wire queue module (join/status); pass redis client to router"
```

---

Part 2 complete. Waiting room join/status work; release engine + checkout gate + admin in Part 3. Next: [Part 3 — Release Engine + Admission Gate + Admin](2026-06-08-phase8-part3-release-admin.md).
