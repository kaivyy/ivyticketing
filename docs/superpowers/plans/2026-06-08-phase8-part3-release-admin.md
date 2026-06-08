# Phase 8 Plan — Part 3: Release Engine + Admission Gate + Admin

> Part of the Phase 8 implementation plan. Index: [2026-06-08-phase8-queue-war-system.md](2026-06-08-phase8-queue-war-system.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** Assumes Parts 1-2 exist. This part makes WAR_QUEUE end-to-end: release engine, admission-gated checkout, admin pause/resume, and the integration/concurrency tests that prove no-oversold / no-queue-reset / no-duplicate.

---

## Task 16: Release engine core

**Files:**
- Create: `services/api/internal/modules/queue/release.go`
- Create: `services/api/internal/modules/queue/tests/release_test.go`

- [ ] **Step 1: Write the failing test** (fake repo + fake store; assert N tokens move WAITING→ALLOWED, admissions created)

Create `services/api/internal/modules/queue/tests/release_test.go`:
```go
package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/queue"
)

func TestRelease_PromotesUpToRate(t *testing.T) {
	// Seed 5 WAITING tokens; release rate 3 → exactly 3 ALLOWED + 3 admissions,
	// 2 remain WAITING. Use a fake repo capturing MarkAllowed + CreateAdmission calls.
	t.Skip("implement fake repo+store; assert 3 promoted, 3 admissions, window=checkoutWindow")
	_ = context.Background
	_ = time.Minute
	_ = uuid.New
	_ = queue.StatusAllowed
}
```

> Replace `t.Skip` with a real fake. The release function signature (Step 3) is `(*Service).Release(ctx, eventID uuid.UUID, n int, window time.Duration) (int, error)` returning count promoted. Fake repo: `ListWaiting` returns 5 tokens; `MarkAllowed` records ids; `CreateAdmission` records calls. Assert promoted==3 when n==3.

- [ ] **Step 2: Run to verify fail**

```bash
cd services/api && go test ./internal/modules/queue/tests/ -run TestRelease -v; cd ../..
```
Expected: FAIL — `Release` undefined.

- [ ] **Step 3: Implement release.go**

```go
package queue

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Release promotes up to n WAITING tokens to ALLOWED, creating an admission with
// checkout window TTL for each. Returns the number promoted. Idempotent per token
// via MarkAllowed's WHERE status='WAITING' guard. Pure rate (decision Q9):
// inventory lock (Phase 5) is the oversold backstop, not the release count.
func (s *Service) Release(ctx context.Context, eventID uuid.UUID, n int, window time.Duration) (int, error) {
	if n <= 0 {
		return 0, nil
	}
	waiting, err := s.repo.ListWaiting(ctx, db.ListWaitingTokensParams{EventID: eventID, Limit: int32(n)})
	if err != nil {
		return 0, err
	}
	promoted := 0
	for _, tok := range waiting {
		err := s.repo.ExecTx(ctx, func(tx Repository) error {
			allowed, err := tx.MarkAllowed(ctx, tok.ID)
			if err != nil {
				return err // pgx.ErrNoRows → already promoted by concurrent tick; skip
			}
			_, err = tx.CreateAdmission(ctx, db.CreateAdmissionParams{
				TokenID:           allowed.ID,
				EventID:           eventID,
				ParticipantID:     allowed.ParticipantID,
				CheckoutExpiresAt: pgTimestamptz(time.Now().Add(window)),
			})
			return err
		})
		if err != nil {
			// token already ALLOWED concurrently (ErrNoRows) → skip silently
			continue
		}
		_ = s.store.MoveToAllowed(ctx, eventID.String(), tok.ParticipantID.String(), time.Now().Add(window).Unix())
		promoted++
	}
	if promoted > 0 && s.audit != nil {
		eid := eventID
		s.audit.Record(ctx, audit.Entry{OrganizationID: nil, ActorUserID: nil,
			Action: "QUEUE_RELEASED", TargetType: "event", TargetID: eid.String(),
			Metadata: map[string]any{"promoted": promoted}})
	}
	return promoted, nil
}
```

> Add helper `pgTimestamptz(t time.Time) pgtype.Timestamptz` in the package (or reuse an existing one). Import `audit` (already imported in service.go — keep release.go imports consistent; if audit import causes a cycle, move the audit call into service.go). Confirm `MarkAllowed` returns `(db.QueueToken, error)` and yields `pgx.ErrNoRows` when the row is no longer WAITING — that is what makes concurrent ticks idempotent.

- [ ] **Step 4: Run to verify pass**

```bash
cd services/api && go test ./internal/modules/queue/tests/ -run TestRelease -race -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/queue/release.go services/api/internal/modules/queue/tests/release_test.go
git commit -m "feat(phase8): queue release engine (pure rate, idempotent promote)"
```

---

## Task 17: Admission expiry handling

**Files:**
- Modify: `services/api/internal/modules/queue/admission.go` (add ExpireDue)
- Create: `services/api/internal/modules/queue/tests/admission_test.go`

- [ ] **Step 1: Write the failing test** (expired ACTIVE admission → token requeued WAITING with new score)

Create `services/api/internal/modules/queue/tests/admission_test.go`:
```go
package queue_test

import "testing"

func TestExpireDue_RequeuesToken(t *testing.T) {
	// Fake repo: ListExpiredAdmissions returns 1 expired admission. ExpireDue must:
	// ExpireAdmission(id), Requeue(tokenID, newScore), store.MoveToWaiting.
	t.Skip("implement fake; assert admission EXPIRED + token requeued WAITING")
}
```

> Implement the fake fully. `ExpireDue(ctx, limit int) (int, error)` returns count expired.

- [ ] **Step 2: Run to verify fail**

```bash
cd services/api && go test ./internal/modules/queue/tests/ -run TestExpireDue -v; cd ../..
```
Expected: FAIL.

- [ ] **Step 3: Implement ExpireDue in admission.go**

```go
// ExpireDue expires ACTIVE admissions past their checkout window and requeues
// their tokens to the back of the WAITING line (decision Q10). Returns count expired.
func (s *Service) ExpireDue(ctx context.Context, limit int) (int, error) {
	due, err := s.repo.ListExpiredAdmissions(ctx, int32(limit))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, adm := range due {
		newScore := FifoScore(time.Now())
		err := s.repo.ExecTx(ctx, func(tx Repository) error {
			if err := tx.ExpireAdmission(ctx, adm.ID); err != nil {
				return err
			}
			return tx.Requeue(ctx, db.RequeueTokenParams{ID: adm.TokenID, Score: newScore})
		})
		if err != nil {
			continue
		}
		_ = s.store.MoveToWaiting(ctx, adm.EventID.String(), adm.ParticipantID.String(), newScore)
		if s.audit != nil {
			s.audit.Record(ctx, audit.Entry{Action: "QUEUE_ADMISSION_EXPIRED",
				TargetType: "queue_admission", TargetID: adm.ID.String(),
				Metadata: map[string]any{"tokenId": adm.TokenID.String()}})
		}
		count++
	}
	return count, nil
}
```

> Confirm `RequeueTokenParams{ID, Score}` and `ListExpiredActiveAdmissions` (wrapped as `ListExpiredAdmissions`) generated names. `ExpireAdmission` has `WHERE status='ACTIVE'` guard so concurrent runs are safe.

- [ ] **Step 4: Run to verify pass**

```bash
cd services/api && go test ./internal/modules/queue/tests/ -run TestExpireDue -race -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/queue/admission.go services/api/internal/modules/queue/tests/admission_test.go
git commit -m "feat(phase8): admission expiry requeues token to back of line"
```

---

## Task 18: Admin control service + handler + routes

**Files:**
- Create: `services/api/internal/modules/queue/control.go`
- Modify: `services/api/internal/modules/queue/handler.go` (add admin handlers)
- Modify: `services/api/internal/modules/queue/routes.go` (add RegisterOrgRoutes)

- [ ] **Step 1: control.go**

```go
package queue

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

func (s *Service) Pause(ctx context.Context, eventID uuid.UUID) error {
	if err := s.ensureControl(ctx, eventID); err != nil {
		return err
	}
	err := s.repo.SetState(ctx, db.SetQueueStateParams{EventID: eventID, State: StatePaused})
	if err == nil && s.audit != nil {
		s.audit.Record(ctx, audit.Entry{Action: "QUEUE_PAUSED", TargetType: "event", TargetID: eventID.String()})
	}
	return err
}

func (s *Service) Resume(ctx context.Context, eventID uuid.UUID) error {
	if err := s.ensureControl(ctx, eventID); err != nil {
		return err
	}
	err := s.repo.SetState(ctx, db.SetQueueStateParams{EventID: eventID, State: StateRunning})
	if err == nil && s.audit != nil {
		s.audit.Record(ctx, audit.Entry{Action: "QUEUE_RESUMED", TargetType: "event", TargetID: eventID.String()})
	}
	return err
}

func (s *Service) SetRate(ctx context.Context, eventID uuid.UUID, rate int32) error {
	if err := s.ensureControl(ctx, eventID); err != nil {
		return err
	}
	err := s.repo.SetRate(ctx, db.SetReleaseRateParams{EventID: eventID, ReleaseRate: rate})
	if err == nil && s.audit != nil {
		s.audit.Record(ctx, audit.Entry{Action: "QUEUE_RATE_CHANGED", TargetType: "event", TargetID: eventID.String(),
			Metadata: map[string]any{"rate": rate}})
	}
	return err
}

type StatsResponse struct {
	Waiting int64  `json:"waiting"`
	Allowed int64  `json:"allowed"`
	Rate    int32  `json:"releaseRate"`
	State   string `json:"state"`
}

func (s *Service) Stats(ctx context.Context, eventID uuid.UUID) (StatsResponse, error) {
	ctrl, err := s.repo.GetControl(ctx, eventID)
	if err != nil {
		return StatsResponse{}, err
	}
	waiting, _ := s.store.WaitingCount(ctx, eventID.String())
	allowed, _ := s.store.AllowedCount(ctx, eventID.String())
	return StatsResponse{Waiting: waiting, Allowed: allowed, Rate: ctrl.ReleaseRate, State: ctrl.State}, nil
}

// ensureControl creates a default control row (RUNNING, default rate) if absent.
func (s *Service) ensureControl(ctx context.Context, eventID uuid.UUID) error {
	_, err := s.repo.GetControl(ctx, eventID)
	if err == nil {
		return nil
	}
	_, err = s.repo.UpsertControl(ctx, db.UpsertQueueControlParams{
		EventID: eventID, State: StateRunning, ReleaseRate: s.defaultRate,
	})
	return err
}
```

> Add `defaultRate int32` field to `Service` and constructor param (from `cfg.QueueDefaultReleaseRate`). Update `NewService(repo, store, recorder, defaultRate)` and its Part 2 caller in server.go. Confirm `UpsertQueueControlParams` nullable fields (`RandomizationSeed pgtype.Text`, `SaleStartAt`/`PresalePoolOpenAt pgtype.Timestamptz`) — leave zero-value (invalid) for now.

- [ ] **Step 2: Admin handlers in handler.go**

Add `Pause`, `Resume`, `SetRate`, `Stats` handlers reading `{orgId}`/`{eventId}` from chi params, decoding `{ "rate": N }` for SetRate, returning 204 (controls) / 200 JSON (stats). Mirror the participant handler style.

- [ ] **Step 3: RegisterOrgRoutes in routes.go**

```go
// RegisterOrgRoutes mounts organizer queue controls under /organizations/{orgId}/events/{eventId}.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	g := r.With(middleware.RequirePermission(loader, "queue.manage"))
	g.Post("/queue/pause", h.Pause)
	g.Post("/queue/resume", h.Resume)
	g.Put("/queue/release-rate", h.SetRate)
	g.Get("/queue/stats", h.Stats)
}
```
Add `middleware` import.

- [ ] **Step 4: Seed queue.manage migration**

Create `database/migrations/00025_seed_queue_manage.sql` (mirrors registration seed). Key `queue.manage` → owner/manager. Roundtrip.

> If Part 1 already seeded `queue.manage` (it was noted there to split — Part 1 seeds only `registration.manage`), then this 00025 seeds `queue.manage`. Verify Part 1's actual seed before writing to avoid duplicate-key (ON CONFLICT DO NOTHING makes it safe regardless).

- [ ] **Step 5: Build + test**

```bash
cd services/api && go build ./... && go test ./internal/modules/queue/... -race; cd ../..
```
Expected: clean + green.

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/queue database/migrations/00025_seed_queue_manage.sql
git commit -m "feat(phase8): queue admin controls (pause/resume/rate/stats) + queue.manage"
```

---

## Task 19: Worker jobs — release + admission expiry

**Files:**
- Modify: `services/api/cmd/worker/main.go`
- Create: `services/api/internal/modules/queue/jobs.go`

- [ ] **Step 1: jobs.go — job closures over the service**

```go
package queue

import (
	"context"
	"time"
)

// ReleaseJob returns a worker Job that releases up to release_rate waiting users
// per running event each tick.
func (s *Service) ReleaseJob(window time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		events, err := s.repo.ListRunningEvents(ctx)
		if err != nil {
			return err
		}
		for _, eventID := range events {
			ctrl, err := s.repo.GetControl(ctx, eventID)
			if err != nil {
				continue
			}
			if ctrl.State != StateRunning || ctrl.ReleaseRate <= 0 {
				continue
			}
			_, _ = s.Release(ctx, eventID, int(ctrl.ReleaseRate), window)
		}
		return nil
	}
}

// AdmissionExpiryJob returns a worker Job that expires due admissions and requeues tokens.
func (s *Service) AdmissionExpiryJob(limit int) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		_, err := s.ExpireDue(ctx, limit)
		return err
	}
}
```

- [ ] **Step 2: Wire jobs in cmd/worker/main.go**

The worker currently builds only orders. Add redis connect + queue service + two runners. Since `worker.Runner.Run(ctx)` blocks, run the existing expire_orders and the two new jobs as separate goroutines (or use a small multi-runner). Add:
```go
	rdb, err := redis.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Error("redis connect failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	queueSvc := queuemod.NewService(
		queuemod.NewRepository(pg.Pool),
		queuemod.NewStore(platformqueue.New(rdb.Client)),
		auditLog,
		int32(cfg.QueueDefaultReleaseRate),
	)

	releaseRunner := worker.New("queue_release", cfg.QueueReleaseInterval, queueSvc.ReleaseJob(cfg.QueueCheckoutWindow), log)
	expiryRunner := worker.New("queue_admission_expiry", cfg.QueueReleaseInterval, queueSvc.AdmissionExpiryJob(500), log)

	go releaseRunner.Run(ctx)
	go expiryRunner.Run(ctx)
	// existing expire_orders runner blocks main goroutine:
	runner.Run(ctx)
```
Add imports for `queuemod`, `platformqueue`, `redis`.

- [ ] **Step 3: Build**

```bash
cd services/api && go build ./...; cd ../..
```
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/queue/jobs.go services/api/cmd/worker/main.go
git commit -m "feat(phase8): worker jobs for queue release + admission expiry"
```

---

## Task 20: Wire queue admitter into registration gate + consume on checkout

**Files:**
- Modify: `services/api/internal/app/server.go`
- Modify: `services/api/internal/modules/orders/service.go` (consume after successful checkout)

- [ ] **Step 1: Inject queue as the gate's admitter**

In `server.go`, the gate was built with `nil` admitter in Part 1. Now `queueSvc` exists (Part 2 wiring). Reorder so queue is built before the gate, then:
```go
	registrationGate := registrationmod.NewGate(registrationSvc, queueSvc)
```
`queueSvc` satisfies `registration.QueueAdmitter` via `CheckAdmission`.

- [ ] **Step 2: Consume admission after checkout success**

The gate's `CheckAdmission` only validates (read-only, safe inside orders tx). Consuming (token→COMPLETED, admission→CONSUMED) must happen after checkout commits. Add an optional post-commit hook to orders.Service:
```go
// in orders.Service struct:
	onCheckout CheckoutHook
// interface in gate.go:
type CheckoutHook interface {
	OnCheckoutComplete(ctx context.Context, participantID, eventID uuid.UUID) error
}
```
After the `ExecTx` returns nil in `Checkout` (right before `return toResponse(created)`), call:
```go
	if s.onCheckout != nil {
		_ = s.onCheckout.OnCheckoutComplete(ctx, participantID, created.EventID)
	}
```
`queueSvc.ConsumeOnCheckout` (Part 2 defined) satisfies `CheckoutHook` (rename method or add adapter). Wire it via `NewService(..., gate, hook)` — extend the constructor with a hook param, or set via a setter. Update server.go to pass `queueSvc` as hook.

> Failure to consume is non-fatal (best-effort) — the admission expiry worker is the backstop; a COMPLETED checkout with a still-ACTIVE admission will simply expire and requeue a token whose order already exists (harmless; the token is COMPLETED so requeue's `WHERE status='ALLOWED'` guard no-ops). Confirm `RequeueToken` has `WHERE status='ALLOWED'` so it won't resurrect a COMPLETED token. (It does — see Part 2 query.) This makes the best-effort consume safe.

- [ ] **Step 3: Build + orders regression**

```bash
cd services/api && go build ./... && go test ./internal/modules/orders/... -race; cd ../..
```
Expected: clean + NORMAL checkout still green.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/app/server.go services/api/internal/modules/orders
git commit -m "feat(phase8): wire queue admitter into gate; consume admission on checkout"
```

---

## Task 21: Integration tests — WAR_QUEUE end-to-end

**Files:**
- Create: `services/api/tests/integration/phase8_queue_test.go`

> Requires `TEST_DATABASE_URL` + Redis (`REDIS_TEST_URL` or the harness redis). Skips if unset. Use existing helpers. The release engine is worker-driven; in tests call `queueSvc.Release(...)` directly (or expose a test helper that promotes N) rather than waiting for the ticker.

- [ ] **Step 1: Write the integration test**

Cover (build tag `//go:build integration`, package `integration`):
- **NORMAL regression:** event with no registration settings → checkout works exactly as Phase 5 (reuse existing checkout helper). Asserts gate is transparent.
- **WAR full flow:** set event mode WAR_QUEUE (PUT registration) + queue control RUNNING; participant joins (`POST /events/{id}/queue/join`) → status WAITING + position; trigger release (direct service call to promote) → status ALLOWED + admission token; checkout with `X-Queue-Token: <admissionId>` → 201 PAID-able order; without header → `ADMISSION_REQUIRED`.
- **Idempotent join:** join twice → same tokenId, same position.
- **Duplicate prevention:** UNIQUE(event,participant) — second join never creates a second row.
- **Pause/resume:** pause → release promotes nothing; resume → promotes.
- **Admission expiry:** set tiny window, let expire (or call ExpireDue) → token back to WAITING.

Write helpers as needed (`setRegistrationMode`, `joinQueue`, `queueStatus`, `releaseN`). Build a queue service against the test pool+redis for direct release calls.

- [ ] **Step 2: Run**

```bash
cd services/api && go test -tags=integration ./tests/integration/ -run TestPhase8 -v; cd ../..
```
Expected: PASS (or SKIP without DB/Redis — note it).

- [ ] **Step 3: Commit**

```bash
git add services/api/tests/integration/phase8_queue_test.go
git commit -m "test(phase8): WAR queue integration (join, release, admission checkout, pause)"
```

---

## Task 22: Concurrency tests — no oversold, no duplicate, no reset

**Files:**
- Create: `services/api/tests/integration/phase8_concurrency_test.go`

- [ ] **Step 1: Write concurrency tests** (`//go:build integration`, `-race`)

- **No duplicate token:** 50 goroutines call Join for the same (event, participant) → exactly 1 token row (UNIQUE), no error surfaced to the participant.
- **No oversold via queue:** capacity=5; release 20 users ALLOWED; 20 concurrent checkouts with valid admission → at most 5 orders created (inventory lock backstop), rest fail with inventory error. No oversold.
- **No queue reset on concurrent status/join:** N goroutines interleave join+status → positions stable, no token duplicated.
- **Release idempotent:** two concurrent `Release(eventID, n)` calls → no token promoted twice (MarkAllowed WHERE status='WAITING' guard); admission count == distinct promoted tokens.

- [ ] **Step 2: Run with -race**

```bash
cd services/api && go test -tags=integration -race ./tests/integration/ -run TestPhase8_Concurrent -v; cd ../..
```
Expected: PASS (or SKIP without DB/Redis).

- [ ] **Step 3: Commit**

```bash
git add services/api/tests/integration/phase8_concurrency_test.go
git commit -m "test(phase8): concurrency — no oversold, no duplicate token, idempotent release"
```

---

Part 3 complete. WAR_QUEUE end-to-end with admin controls, proven race-safe. Next: [Part 4 — Randomized + Hybrid + Frontend + Docs](2026-06-08-phase8-part4-randomized-frontend-docs.md).
