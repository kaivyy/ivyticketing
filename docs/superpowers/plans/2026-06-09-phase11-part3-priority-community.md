# Phase 11 Part 3: Priority + Community + PRIORITY_ACCESS + WAITLIST_ONLY Gates

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement PRIORITY pool auto-eligibility grants, COMMUNITY pool self-apply, `PriorityChecker` for the RAE gate, and WAITLIST_ONLY join flow. Wire PRIORITY_ACCESS and WAITLIST_ONLY modes in the RAE.

**Architecture:** `PriorityChecker` checks the LifecycleEngine for an active PRIORITY_ACCESS phase, then calls `EligibilityChecker` and issues a grant from the PRIORITY pool automatically (no code required). COMMUNITY self-apply redeems against a COMMUNITY pool using the eligibility rule. WAITLIST_ONLY uses `AccessGrantChecker` — grants issued by `WaitlistEngine.PromoteBatch` when slots open.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc. Module: `github.com/varin/ivyticketing/services/api`.

---

### Task 1: PriorityChecker Service

**Files:**
- Create: `services/api/internal/modules/access/priority.go`
- Create: `services/api/internal/modules/access/tests/priority_test.go`

- [ ] **Step 1: Write failing tests**

```go
// services/api/internal/modules/access/tests/priority_test.go
package access_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/access"
	"github.com/varin/ivyticketing/services/api/internal/modules/lifecycle"
	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

type fakePriorityLifecycle struct{ open bool }

func (f *fakePriorityLifecycle) IsWindowOpen(_ context.Context, _ uuid.UUID, _ registration.Mode) (bool, lifecycle.WindowClosedReason, error) {
	return f.open, lifecycle.ReasonWindowExpired, nil
}

type fakePriorityRepo struct {
	fakeAccessRepoFull
	pool  *db.AccessPool
	grant *db.AccessGrant
}

func (r *fakePriorityRepo) ListVisiblePoolsByCategory(_ context.Context, _ db.ListVisiblePoolsByCategoryParams) ([]db.AccessPool, error) {
	if r.pool == nil { return nil, nil }
	return []db.AccessPool{*r.pool}, nil
}
func (r *fakePriorityRepo) ReservePoolSlot(_ context.Context, _ uuid.UUID) (db.AccessPool, error) {
	if r.pool == nil { return db.AccessPool{}, pgx.ErrNoRows }
	return *r.pool, nil
}
func (r *fakePriorityRepo) CreateAccessGrant(_ context.Context, _ db.CreateAccessGrantParams) (db.AccessGrant, error) {
	return db.AccessGrant{ID: uuid.New(), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil
}

func TestPriorityChecker_WindowClosed_ReturnsError(t *testing.T) {
	lc := &fakePriorityLifecycle{open: false}
	checker := access.NewPriorityChecker(&fakePriorityRepo{}, lc, access.NewEligibilityChecker(&fakeEligRepo{}))
	err := checker.CheckPriorityAdmission(context.Background(), uuid.New(), uuid.New(), "")
	if err == nil { t.Fatal("closed priority window should return error") }
}

func TestPriorityChecker_NoPool_ReturnsError(t *testing.T) {
	lc := &fakePriorityLifecycle{open: true}
	checker := access.NewPriorityChecker(&fakePriorityRepo{pool: nil}, lc, access.NewEligibilityChecker(&fakeEligRepo{}))
	err := checker.CheckPriorityAdmission(context.Background(), uuid.New(), uuid.New(), "")
	if err == nil { t.Fatal("no priority pool should return error") }
}

func TestPriorityChecker_EligibleWithOpenWindow_ReturnsNil(t *testing.T) {
	lc := &fakePriorityLifecycle{open: true}
	pool := &db.AccessPool{ID: uuid.New(), TotalSlots: 10}
	checker := access.NewPriorityChecker(
		&fakePriorityRepo{pool: pool},
		lc,
		access.NewEligibilityChecker(&fakeEligRepo{orderCount: 1}),
	)
	err := checker.CheckPriorityAdmission(context.Background(), uuid.New(), uuid.New(), "")
	if err != nil { t.Fatalf("eligible user with open window should pass: %v", err) }
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -run TestPriority -v 2>&1
```

- [ ] **Step 3: Write priority.go**

```go
package access

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/lifecycle"
	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

type LifecycleWindowChecker interface {
	IsWindowOpen(ctx context.Context, categoryID uuid.UUID, mode registration.Mode) (bool, lifecycle.WindowClosedReason, error)
}

// PriorityChecker implements registration.PriorityChecker.
// On CheckPriorityAdmission: verifies priority window is open, checks eligibility,
// auto-issues an AccessGrant if no valid grant exists yet.
type PriorityChecker struct {
	repo      Repository
	lifecycle LifecycleWindowChecker
	elig      *EligibilityChecker
}

func NewPriorityChecker(repo Repository, lc LifecycleWindowChecker, elig *EligibilityChecker) *PriorityChecker {
	return &PriorityChecker{repo: repo, lifecycle: lc, elig: elig}
}

func (p *PriorityChecker) CheckPriorityAdmission(ctx context.Context, participantID, categoryID uuid.UUID, grantToken string) error {
	// 1. If grantToken provided, verify existing grant
	if grantToken != "" {
		return p.repo.(*poolManagerRepo).pm.CheckGrant(ctx, participantID, categoryID, grantToken)
	}

	// 2. Check priority window open via lifecycle
	open, reason, err := p.lifecycle.IsWindowOpen(ctx, categoryID, registration.ModePriorityAccess)
	if err != nil { return err }
	if !open {
		return apperr.New(409, "PRIORITY_WINDOW_CLOSED", string(reason))
	}

	// 3. Find PRIORITY pool for this category
	pools, err := p.repo.ListVisiblePoolsByCategory(ctx, db.ListVisiblePoolsByCategoryParams{
		EventID: uuid.Nil, CategoryID: categoryID,
	})
	if err != nil { return err }
	var priorityPool *db.AccessPool
	for _, pool := range pools {
		if pool.PoolType == PoolTypePriority { priorityPool = &pool; break }
	}
	if priorityPool == nil { return ErrPoolExhausted }

	// 4. Eligibility check
	if len(priorityPool.EligibilityRule) > 0 {
		ok, reason, err := p.elig.Check(ctx, participantID, priorityPool.OrganizationID, priorityPool.EligibilityRule)
		if err != nil { return err }
		if !ok { return fmt.Errorf("%w: %s", ErrNotEligible, reason) }
	}

	// 5. Check if participant already has an active grant (idempotent)
	existing, err := p.repo.GetActiveGrantForParticipant(ctx, db.GetActiveGrantForParticipantParams{
		ParticipantID: participantID, CategoryID: categoryID,
	})
	if err == nil && existing.Status == GrantStatusActive {
		return nil // already granted
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) { return err }

	// 6. Reserve slot + issue grant
	pool, err := p.repo.ReservePoolSlot(ctx, priorityPool.ID)
	if errors.Is(err, pgx.ErrNoRows) { return ErrPoolExhausted }
	if err != nil { return err }
	_ = pool

	expiresAt := priorityPool.ValidUntil
	_, err = p.repo.CreateAccessGrant(ctx, db.CreateAccessGrantParams{
		PoolID:        pgtype.UUID{Bytes: priorityPool.ID, Valid: true},
		ParticipantID: participantID,
		CategoryID:    categoryID,
		EventID:       priorityPool.EventID,
		ExpiresAt:     expiresAt,
	})
	return err
}
```

**Note:** The `apperr` and `fmt` imports need to be added. Also `poolManagerRepo` cast is a placeholder — simplify by accepting `*PoolManager` directly or using the `AccessGrantChecker` interface. Clean this up during build.

- [ ] **Step 4: Run tests — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -run TestPriority -race -v 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/access/priority.go \
        services/api/internal/modules/access/tests/priority_test.go
git commit -m "feat(phase11): PriorityChecker (window check, eligibility, auto-grant)"
```

---

### Task 2: Wire PriorityChecker into RAE Gate

**Files:**
- Modify: `services/api/internal/modules/registration/gate.go`
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Add PriorityChecker interface to gate.go**

```go
type PriorityChecker interface {
	CheckPriorityAdmission(ctx context.Context, participantID, categoryID uuid.UUID, grantToken string) error
}

type Gate struct {
	svc         *Service
	queue        QueueAdmitter
	lifecycle    LifecycleChecker
	ballot       BallotAdmitter
	accessGrant  AccessGrantChecker
	priority     PriorityChecker   // Phase 11 Part 3
}

func NewGate(svc *Service, queue QueueAdmitter, lc LifecycleChecker, ballot BallotAdmitter,
	accessGrant AccessGrantChecker, priority PriorityChecker) *Gate {
	return &Gate{svc: svc, queue: queue, lifecycle: lc, ballot: ballot,
		accessGrant: accessGrant, priority: priority}
}

// In Admit() switch, add:
case ModePriorityAccess:
    if g.priority == nil { return ErrModeNotAvailable }
    return g.priority.CheckPriorityAdmission(ctx, participantID, categoryID, admissionToken)
```

- [ ] **Step 2: Update server.go**

```go
priorityChecker := access.NewPriorityChecker(accessRepo, lifecycleSvc, eligibilityChecker)
registrationGate := registration.NewGate(
    registrationSvc, queueAdmitter, lifecycleSvc,
    ballotSvc, poolMgr, priorityChecker,
)
```

- [ ] **Step 3: Build + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./... 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
```

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/registration/gate.go services/api/internal/app/server.go
git commit -m "feat(phase11): RAE gate — PRIORITY_ACCESS via PriorityChecker"
```

---

### Task 3: Priority Window Endpoint + Community Self-Apply

**Files:**
- Modify: `services/api/internal/modules/access/handler.go`
- Modify: `services/api/internal/modules/access/routes.go`

- [ ] **Step 1: Add to handler.go**

```go
func (h *Handler) PriorityWindow(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	eventID, _ := uuid.Parse(chi.URLParam(r, "eventId"))
	categoryID, _ := uuid.Parse(r.URL.Query().Get("categoryId"))
	// Call priority checker — on success means grant was issued
	err := h.priority.CheckPriorityAdmission(r.Context(), actor.UserID, categoryID, "")
	if err != nil { apperr.WriteError(w, r, err); return }
	// Return the active grant
	grant, err := h.codes.repo.GetActiveGrantForParticipant(r.Context(), db.GetActiveGrantForParticipantParams{
		ParticipantID: actor.UserID, CategoryID: categoryID,
	})
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, AccessGrantDTO{
		ID: grant.ID.String(), Token: grant.ID.String(),
		CategoryID: grant.CategoryID.String(), ExpiresAt: grant.ExpiresAt.Time,
	})
}

func (h *Handler) WaitlistJoin(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	eventID, _ := uuid.Parse(chi.URLParam(r, "eventId"))
	categoryID, _ := uuid.Parse(chi.URLParam(r, "categoryId"))
	wl, err := h.waitlistRepo.GetWaitlistByCategory(r.Context(), db.GetWaitlistByCategoryParams{
		EventID: eventID, CategoryID: categoryID,
	})
	if err != nil { apperr.WriteError(w, r, apperr.New(404, "WAITLIST_NOT_FOUND", "no waitlist for this category")); return }
	entry, err := h.waitlistSvc.Join(r.Context(), wl.ID, actor.UserID, "QUOTA_RELEASE", nil)
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, map[string]any{
		"waitlistEntryId": entry.ID.String(),
		"rank":            entry.Rank,
	})
}

func (h *Handler) WaitlistPosition(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	eventID, _ := uuid.Parse(chi.URLParam(r, "eventId"))
	categoryID, _ := uuid.Parse(chi.URLParam(r, "categoryId"))
	wl, err := h.waitlistRepo.GetWaitlistByCategory(r.Context(), db.GetWaitlistByCategoryParams{
		EventID: eventID, CategoryID: categoryID,
	})
	if err != nil { apperr.WriteError(w, r, apperr.New(404, "WAITLIST_NOT_FOUND", "no waitlist")); return }
	entry, err := h.waitlistRepo.GetWaitlistEntry(r.Context(), db.GetWaitlistEntryParams{
		WaitlistID: wl.ID, ParticipantID: actor.UserID,
	})
	if err != nil { apperr.WriteError(w, r, apperr.New(404, "NOT_ON_WAITLIST", "not on waitlist")); return }
	position, _ := h.waitlistRepo.CountWaitlistPosition(r.Context(), db.CountWaitlistPositionParams{
		WaitlistID: wl.ID, Rank: entry.Rank,
	})
	apperr.WriteJSON(w, http.StatusOK, map[string]any{
		"position": position + 1,
		"rank":     entry.Rank,
		"status":   entry.Status,
	})
}
```

**Note:** `Handler` needs `priority PriorityChecker`, `waitlistRepo waitlist.Repository`, `waitlistSvc *waitlist.Service` fields. Add them to the `Handler` struct and `NewHandler` constructor.

- [ ] **Step 2: Add routes**

```go
// In RegisterParticipantRoutes:
r.Get("/events/{eventId}/access/priority-window", h.PriorityWindow)
r.Post("/events/{eventId}/categories/{categoryId}/waitlist/join", h.WaitlistJoin)
r.Get("/events/{eventId}/categories/{categoryId}/waitlist/my-position", h.WaitlistPosition)
```

- [ ] **Step 3: Add `CategoryWaitlistJoin` to abuse/model.go**

```go
CategoryWaitlistJoin = "waitlist_join"
```

Add guard on `WaitlistJoin` endpoint: `abuseGuard.Middleware(abusemod.CategoryWaitlistJoin)`.

- [ ] **Step 4: Build + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./... 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/access/handler.go \
        services/api/internal/modules/access/routes.go \
        services/api/internal/modules/abuse/model.go
git commit -m "feat(phase11): priority window endpoint + waitlist join/position endpoints"
```

---

### Task 4: Frontend — Priority Window Countdown + Waitlist Position

**Files:**
- Create: `apps/web/src/components/access/PriorityWindowBanner.astro`
- Create: `apps/web/src/components/access/WaitlistStatus.astro`
- Modify: event page (add banners for PRIORITY_ACCESS and WAITLIST_ONLY modes)

- [ ] **Step 1: Write PriorityWindowBanner.astro**

```astro
---
interface Props {
  eventId: string
  categoryId: string
  windowOpensAt?: string
  windowClosesAt?: string
  isEligible?: boolean
}
const { eventId, categoryId, windowOpensAt, windowClosesAt, isEligible } = Astro.props
---

{isEligible && (
  <div class="bg-amber-50 border border-amber-200 rounded p-4 mb-4">
    {windowOpensAt && new Date(windowOpensAt) > new Date() ? (
      <p class="text-amber-800">
        Priority access opens in
        <span class="font-semibold" data-countdown={windowOpensAt}></span>
      </p>
    ) : (
      <div>
        <p class="text-amber-800 font-semibold mb-2">Priority Access Active</p>
        <a
          href={`/events/${eventId}/access/priority?categoryId=${categoryId}`}
          class="bg-amber-600 text-white px-4 py-2 rounded inline-block"
        >Register Now</a>
      </div>
    )}
  </div>
)}

<script>
  document.querySelectorAll("[data-countdown]").forEach((el) => {
    const target = new Date(el.getAttribute("data-countdown") ?? "")
    const tick = () => {
      const diff = target.getTime() - Date.now()
      if (diff <= 0) { el.textContent = "now"; return }
      const h = Math.floor(diff / 3600000)
      const m = Math.floor((diff % 3600000) / 60000)
      const s = Math.floor((diff % 60000) / 1000)
      el.textContent = ` ${h}h ${m}m ${s}s`
      setTimeout(tick, 1000)
    }
    tick()
  })
</script>
```

- [ ] **Step 2: Write WaitlistStatus.astro**

```astro
---
interface Props {
  eventId: string
  categoryId: string
  token: string
}
const { eventId, categoryId, token } = Astro.props
---

<div id="waitlist-status" class="border rounded p-4">
  <p class="text-gray-600">Checking waitlist position...</p>
</div>

<script define:vars={{ eventId, categoryId, token }}>
  const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080"
  fetch(`${API_URL}/api/v1/events/${eventId}/categories/${categoryId}/waitlist/my-position`, {
    headers: { Authorization: `Bearer ${token}` }
  })
    .then((r) => r.json())
    .then((data) => {
      const el = document.getElementById("waitlist-status")
      if (el) {
        el.innerHTML = data.status === "PROMOTED"
          ? `<p class="text-green-700 font-semibold">You've been promoted! Check your email for your registration link.</p>`
          : `<p class="text-gray-700">Waitlist position: <strong>#${data.position}</strong></p>`
      }
    })
    .catch(() => {})
</script>
```

- [ ] **Step 3: Build frontend**

```bash
cd /Users/kaivy/Coding/ivyticketing/apps/web && npm run build 2>&1
```

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/components/access/PriorityWindowBanner.astro \
        apps/web/src/components/access/WaitlistStatus.astro
git commit -m "feat(phase11): priority window countdown + waitlist position UI"
```

---

### Task 5: Part 3 Full Verification

- [ ] **Step 1**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api
go build ./... 2>&1
go vet ./... 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
cd /Users/kaivy/Coding/ivyticketing/apps/web && npm run build 2>&1
```

- [ ] **Step 2: Commit**

```bash
git commit -m "feat(phase11): part 3 complete — PRIORITY_ACCESS + WAITLIST_ONLY gates wired"
```
