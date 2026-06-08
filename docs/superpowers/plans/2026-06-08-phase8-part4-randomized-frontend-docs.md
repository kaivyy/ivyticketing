# Phase 8 Plan — Part 4: Randomized + Hybrid + Frontend + Docs

> Part of the Phase 8 implementation plan. Index: [2026-06-08-phase8-queue-war-system.md](2026-06-08-phase8-queue-war-system.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** Assumes Parts 1-3 (WAR_QUEUE end-to-end). This part adds RANDOMIZED_QUEUE + HYBRID_QUEUE scoring, the waiting room frontend, a load-test scaffold, docs, and final verification.

---

## Task 23: Randomized + Hybrid scoring in Join

**Files:**
- Modify: `services/api/internal/modules/queue/service.go` (Join chooses pool+score by mode)
- Create: `services/api/internal/modules/queue/tests/join_mode_test.go`

The Join in Part 2 always used FIFO. Now Join branches on the resolved event mode + sale window.

- [ ] **Step 1: Add a mode/seed resolver dependency to queue.Service**

Define in queue an interface for what it needs from registration + control:
```go
// EventModeResolver returns the resolved registration mode for an event.
type EventModeResolver interface {
	ResolveEventMode(ctx context.Context, eventID uuid.UUID) (string, error)
}
```
Add `resolver EventModeResolver` to `Service` (nil-safe: nil → treat as WAR_QUEUE/FIFO for tests). Add `ResolveEventMode` to `registration.Service`:
```go
func (s *Service) ResolveEventMode(ctx context.Context, eventID uuid.UUID) (string, error) {
	ev, err := s.repo.GetEventSettings(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return string(ModeNormal), nil
		}
		return "", err
	}
	return ev.DefaultMode, nil
}
```
Wire `registrationSvc` as the queue resolver in `server.go` and `cmd/worker` (extend `NewService` signature). `registration.Service` satisfies `queue.EventModeResolver`.

- [ ] **Step 2: Write the failing test**

Create `services/api/internal/modules/queue/tests/join_mode_test.go`:
```go
package queue_test

import "testing"

func TestJoin_RandomizedUsesPresalePoolBeforeSale(t *testing.T) {
	// resolver returns RANDOMIZED_QUEUE; control has sale_start in the future +
	// a seed. Join → token.pool == PRESALE, score == PresaleScore(seed, participant).
	t.Skip("implement fake resolver + control; assert PRESALE pool + deterministic score")
}

func TestJoin_RandomizedAfterSaleUsesFifo(t *testing.T) {
	// sale_start in the past → token.pool == FIFO, score == FifoScore-ish (monotonic).
	t.Skip("implement; assert FIFO pool after sale start")
}
```

> Implement fakes. Assert pool + score selection. HYBRID behaves identically to RANDOMIZED for join (presale randomized, post-sale FIFO) — add a HYBRID case asserting same behavior.

- [ ] **Step 2b: Run to verify fail**

```bash
cd services/api && go test ./internal/modules/queue/tests/ -run TestJoin_Randomized -v; cd ../..
```
Expected: FAIL.

- [ ] **Step 3: Update Join to branch by mode**

In `service.go`, replace the fixed FIFO scoring with mode-aware logic:
```go
func (s *Service) Join(ctx context.Context, orgID, eventID, participantID uuid.UUID) (JoinResponse, error) {
	pool := PoolFifo
	score := FifoScore(time.Now())

	mode := string(registrationModeWarFallback) // "WAR_QUEUE" default when resolver nil
	if s.resolver != nil {
		m, err := s.resolver.ResolveEventMode(ctx, eventID)
		if err != nil {
			return JoinResponse{}, err
		}
		mode = m
	}
	if mode == "RANDOMIZED_QUEUE" || mode == "HYBRID_QUEUE" {
		ctrl, err := s.repo.GetControl(ctx, eventID)
		if err == nil && ctrl.SaleStartAt.Valid && time.Now().Before(ctrl.SaleStartAt.Time) {
			seed := ""
			if ctrl.RandomizationSeed.Valid {
				seed = ctrl.RandomizationSeed.String
			}
			pool = PoolPresale
			score = PresaleScore(seed, participantID)
		}
		// after sale start → FIFO (defaults above)
	} else if mode != "WAR_QUEUE" {
		return JoinResponse{}, ErrNotEnabled // non-queue mode cannot join
	}

	tok, err := s.repo.CreateToken(ctx, db.CreateQueueTokenParams{
		OrganizationID: orgID, EventID: eventID, ParticipantID: participantID,
		Pool: pool, Score: score,
	})
	// ... rest unchanged (ErrNoRows → idempotent existing token; AddWaiting; audit) ...
}
```
Define `const registrationModeWarFallback = "WAR_QUEUE"` or inline the string. Keep the idempotent/existing-token branch and `AddWaiting`/audit exactly as Part 2.

> Redis ranking already orders PRESALE before FIFO because `ListWaiting`/release uses `ORDER BY pool DESC, score ASC` ('PRESALE' > 'FIFO' lexically with DESC). But the Redis sorted set is a single set scored by `score` only — presale scores (hashed, large range) and FIFO scores (unix nano) may interleave. FIX: make Redis position authoritative for display only; release pulls from Postgres `ListWaiting` (already pool-ordered). For waiting-room POSITION shown to user, prefer Postgres rank for randomized/hybrid OR encode pool into the Redis score namespace (e.g., presale scores in a lower band). SIMPLEST + correct: release reads Postgres `ListWaiting` (pool-ordered) — already the case in Part 3 `Release`. For position display, compute rank from Postgres for PRESALE pool. Document this: Redis = WAR fast-path position; randomized/hybrid position derived from Postgres pool-ordered rank. Add a `PositionFromDB(ctx, token)` path used when pool=PRESALE.

- [ ] **Step 4: Run to verify pass**

```bash
cd services/api && go test ./internal/modules/queue/... -race; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/queue services/api/internal/modules/registration/service.go services/api/internal/app/server.go services/api/cmd/worker/main.go
git commit -m "feat(phase8): randomized + hybrid queue scoring (presale pool, seeded)"
```

---

## Task 24: Queue control set seed + sale window (organizer)

**Files:**
- Modify: `services/api/internal/modules/queue/control.go` (SetSchedule)
- Modify: `services/api/internal/modules/queue/handler.go` + `routes.go`

- [ ] **Step 1: SetSchedule on Service**

```go
// SetSchedule sets sale window + randomization seed for randomized/hybrid modes.
func (s *Service) SetSchedule(ctx context.Context, eventID uuid.UUID, seed string, saleStart, presaleOpen *time.Time) error {
	ctrl, _ := s.repo.GetControl(ctx, eventID) // current or zero
	_, err := s.repo.UpsertControl(ctx, db.UpsertQueueControlParams{
		EventID:            eventID,
		State:              orDefault(ctrl.State, StateRunning),
		ReleaseRate:        orDefaultRate(ctrl.ReleaseRate, s.defaultRate),
		RandomizationSeed:  pgText(seed),
		SaleStartAt:        pgTimestamptzPtr(saleStart),
		PresalePoolOpenAt:  pgTimestamptzPtr(presaleOpen),
	})
	return err
}
```
Add small helpers `orDefault`, `orDefaultRate`, `pgText`, `pgTimestamptzPtr`. Seed should be auto-generated (e.g., random hex) if empty, and stored so the draw is reproducible/auditable.

- [ ] **Step 2: Handler + route**

Add `SetSchedule` handler decoding `{ "seed"?, "saleStartAt"?, "presalePoolOpenAt"? }`; mount `PUT /queue/schedule` under `queue.manage` in `RegisterOrgRoutes`.

- [ ] **Step 3: Build + test + commit**

```bash
cd services/api && go build ./... && go test ./internal/modules/queue/... -race; cd ../..
git add services/api/internal/modules/queue
git commit -m "feat(phase8): organizer set queue schedule + randomization seed"
```

---

## Task 25: Frontend — queue lib + waiting room page

**Files:**
- Create: `apps/web/src/lib/queue.ts`
- Create: `apps/web/src/pages/events/[slug]/queue.astro`
- Create: `apps/web/src/components/queue/WaitingRoom.astro`
- Create: `apps/web/src/components/queue/QueueStatus.astro`

Reuse Phase 7 auth foundation (`authedFetch` in `lib/api.ts`, sessionStorage token).

- [ ] **Step 1: lib/queue.ts**

```ts
import { authedFetch } from "./api";

export interface JoinResponse {
  tokenId: string;
  status: string;
  position: number;
}

export interface QueueStatusResponse {
  tokenId: string;
  status: string;
  position: number;
  estimatedWaitSeconds: number;
  systemState: string;
  admissionToken?: string;
  checkoutExpiresAt?: string;
}

export function joinQueue(eventId: string): Promise<JoinResponse> {
  return authedFetch<JoinResponse>(`/events/${eventId}/queue/join`, { method: "POST" });
}

export function getQueueStatus(eventId: string): Promise<QueueStatusResponse> {
  return authedFetch<QueueStatusResponse>(`/events/${eventId}/queue/status`);
}
```

> Phase 7's `authedFetch` was GET-only (`authedFetch<T>(path)`). Extend it to accept an optional `{ method?, body? }` second arg, defaulting to GET, so POST join works. Update `apps/web/src/lib/api.ts` accordingly (additive — keep existing GET callers working). Verify Phase 7 signature before editing.

- [ ] **Step 2: WaitingRoom.astro** (auto-poll, refresh/reconnect/sleep-safe)

```astro
---
const { eventId } = Astro.props;
---
<div id="waiting-room" data-event-id={eventId} class="rounded-lg border border-slate-200 bg-white p-6 text-center">
  <p class="text-slate-500">Menyiapkan antrean…</p>
</div>
<script>
  import { joinQueue, getQueueStatus } from "../../lib/queue";
  const root = document.getElementById("waiting-room")!;
  const eventId = root.dataset.eventId!;
  let timer: number | undefined;

  function render(s: { status: string; position: number; estimatedWaitSeconds: number; systemState: string; admissionToken?: string }) {
    if (s.systemState === "PAUSED") {
      root.innerHTML = `<p class="text-amber-600 font-medium">Antrean dijeda sementara. Posisimu tetap aman.</p>`;
      return;
    }
    if (s.status === "ALLOWED" && s.admissionToken) {
      sessionStorage.setItem("ivy_admission_" + eventId, s.admissionToken);
      root.innerHTML = `<p class="text-green-600 font-semibold mb-3">Giliranmu! Selesaikan checkout.</p>
        <a href="/events/${eventId}/checkout" class="inline-block rounded bg-slate-900 px-4 py-2 text-white">Lanjut Checkout</a>`;
      if (timer) clearInterval(timer);
      return;
    }
    if (s.status === "EXPIRED") {
      root.innerHTML = `<p class="text-red-600">Waktu checkout habis. Kamu dikembalikan ke antrean.</p>`;
      return;
    }
    const mins = Math.max(1, Math.ceil(s.estimatedWaitSeconds / 60));
    root.innerHTML = `<p class="text-lg font-semibold">Posisi antrean: ${s.position + 1}</p>
      <p class="text-slate-600">Perkiraan tunggu ~${mins} menit. Jangan tutup halaman ini.</p>`;
  }

  async function poll() {
    try { render(await getQueueStatus(eventId)); }
    catch { root.innerHTML = `<p class="text-slate-500">Sistem sedang padat. Posisimu tetap aman.</p>`; }
  }

  async function start() {
    try { await joinQueue(eventId); } catch { /* already queued → status will show */ }
    await poll();
    timer = window.setInterval(poll, 4000);
  }

  document.addEventListener("visibilitychange", () => { if (!document.hidden) poll(); }); // mobile-sleep safe
  start();
</script>
```

- [ ] **Step 3: QueueStatus.astro** (small badge component — optional reuse) and **queue.astro** page

`apps/web/src/pages/events/[slug]/queue.astro`:
```astro
---
export const prerender = false;
import ParticipantLayout from "../../../layouts/ParticipantLayout.astro";
import WaitingRoom from "../../../components/queue/WaitingRoom.astro";
// The page needs the event UUID. If the route uses slug, the page must resolve
// slug→eventId via a public endpoint, or pass eventId as a query param. SIMPLEST:
// read eventId from query string (?eventId=) set by the event page CTA.
const eventId = Astro.url.searchParams.get("eventId") ?? "";
---
<ParticipantLayout title="Antrean">
  <h1 class="text-xl font-bold mb-4">Ruang Tunggu</h1>
  {eventId
    ? <WaitingRoom eventId={eventId} />
    : <p class="text-red-600">Event tidak ditemukan.</p>}
</ParticipantLayout>
```

> Verify how Phase 5/7 public pages resolve event slug→id. If a public `GET /events/{slug}` exists, prefer resolving server-side. Otherwise the query-param approach is acceptable for MVP — note it. Keep QueueStatus.astro minimal or fold into WaitingRoom (YAGNI) — create it only if it earns its place.

- [ ] **Step 4: Build**

```bash
cd apps/web && npm run build 2>&1 | tail -10; cd ../..
```
Expected: succeeds (apply `prerender=false` already set).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/lib/queue.ts apps/web/src/lib/api.ts apps/web/src/components/queue "apps/web/src/pages/events/[slug]/queue.astro"
git commit -m "feat(phase8): waiting room frontend (auto-poll, refresh/sleep-safe)"
```

---

## Task 26: Organizer queue controls UI

**Files:**
- Create: queue control page in the organizer dashboard app (verify which app: `apps/organizer-dashboard` or organizer section of `apps/web`).

- [ ] **Step 1: Check organizer dashboard app state**

```bash
ls apps/ && ls apps/organizer-dashboard/src 2>/dev/null
```
If `apps/organizer-dashboard` is scaffolded, add the queue control page there; if minimal/absent, add a minimal organizer queue page under `apps/web` mirroring the participant pattern. Pick based on actual state and note the decision.

- [ ] **Step 2: Implement control page** — pause/resume buttons, release-rate input, live stats (poll `/queue/stats`). Uses `authedFetch` with the organizer's token + org/event ids. Mirror WaitingRoom's poll pattern for stats.

- [ ] **Step 3: Build + commit**

```bash
cd apps/web && npm run build 2>&1 | tail -10; cd ../..
git add apps/
git commit -m "feat(phase8): organizer queue controls UI (pause/resume/rate/stats)"
```

---

## Task 27: Load test scaffold (k6)

**Files:**
- Create: `tests/load/queue-war.js`

- [ ] **Step 1: Write a k6 scenario** simulating waiting-room join + status polling ramping to 10k/50k/100k virtual users (staged). Assert no 5xx, status endpoint p95 within target. This is a scaffold — actual 100k run is environment-dependent.

```js
import http from "k6/http";
import { check, sleep } from "k6";

const BASE = __ENV.API_URL || "http://localhost:8080";
const EVENT_ID = __ENV.EVENT_ID;
const TOKEN = __ENV.ACCESS_TOKEN; // a pre-provisioned participant token

export const options = {
  stages: [
    { duration: "1m", target: 10000 },
    { duration: "2m", target: 50000 },
    { duration: "2m", target: 100000 },
    { duration: "1m", target: 0 },
  ],
};

export default function () {
  const headers = { Authorization: `Bearer ${TOKEN}` };
  http.post(`${BASE}/api/v1/events/${EVENT_ID}/queue/join`, null, { headers });
  const res = http.get(`${BASE}/api/v1/events/${EVENT_ID}/queue/status`, { headers });
  check(res, { "status 200": (r) => r.status === 200 });
  sleep(4);
}
```

> Note in commit: real load test requires per-VU distinct tokens (one queue token per user); this scaffold uses a shared token for smoke-level ramp. A full run needs a token-provisioning setup stage. Document as best-effort per spec (acceptance: load test 100k is a target, environment-dependent).

- [ ] **Step 2: Commit**

```bash
git add tests/load/queue-war.js
git commit -m "test(phase8): k6 waiting-room load scaffold (10k/50k/100k stages)"
```

---

## Task 28: Docs

**Files:**
- Create: `docs/REGISTRATION_MODES.md`, `docs/QUEUE_MODES.md`, `docs/QUEUE_OPERATIONS.md`, `docs/PHASE8_DECISIONS.md`

- [ ] **Step 1: REGISTRATION_MODES.md** — enum, resolver (override event/category), gate seam, fail-closed for Phase 10-11 modes, how NORMAL stays identical.

- [ ] **Step 2: QUEUE_MODES.md** — WAR/RANDOMIZED/HYBRID; text sequence diagram (join → waiting → release → admission → checkout → complete); token + admission state machines; scoring (FIFO unix-nano, presale seeded SHA256); release engine (pure rate); pool ordering (PRESALE before FIFO via Postgres ListWaiting).

- [ ] **Step 3: QUEUE_OPERATIONS.md** — pause/resume/set-rate/set-schedule; Redis↔Postgres reconcile (Postgres authoritative; Redis rebuildable from WAITING tokens); Redis-down recovery; war-day runbook notes; admission expiry → requeue.

- [ ] **Step 4: PHASE8_DECISIONS.md** — the 10 decisions (Q1-Q10) with Why/Tradeoff: hybrid store, foundation-first, 3 modes, anti-bot stub, per-event scope, admission via header, frontend included, position+estimate, pure rate, expired→back-of-line.

- [ ] **Step 5: Commit**

```bash
git add docs/REGISTRATION_MODES.md docs/QUEUE_MODES.md docs/QUEUE_OPERATIONS.md docs/PHASE8_DECISIONS.md
git commit -m "docs(phase8): registration modes, queue modes, operations, decisions"
```

---

## Task 29: Update CHANGELOG

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Prepend Phase 8 section** (match Phase 6/7 format). Include: registration mode foundation (resolver + settings, override pattern); queue module (WAR/RANDOMIZED/HYBRID); Redis sorted-set adapter; persistent tokens (idempotent, refresh/reconnect/sleep-safe); release engine + admission window; admin pause/resume/rate/schedule; admission-gated checkout via X-Queue-Token; anti-bot hook stub (Phase 9); waiting room frontend; migrations 00020-00025; permissions registration.manage/queue.manage; env QUEUE_*; deferred: anti-bot full (P9), ballot (P10), invitation/priority/community/corporate + waitlist (P11), websocket realtime.

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(phase8): update CHANGELOG"
```

---

## Task 30: Final verification + DoD checklist

**Files:** none (verification).

- [ ] **Step 1: sqlc + vet + build**

```bash
make sqlc
cd services/api && go vet ./... && go build ./...; cd ../..
```
Expected: clean, no diff.

- [ ] **Step 2: Full test suite (race)**

```bash
cd services/api && go test ./... -race 2>&1 | tail -20; cd ../..
```
Expected: PASS (Phase 1-7 unaffected; Phase 8 unit/concurrency green; integration skips without DB/Redis).

- [ ] **Step 3: Integration (if DB+Redis available)**

```bash
cd services/api && go test -tags=integration -race ./tests/integration/ -run TestPhase8 -v; cd ../..
```
Expected: PASS or documented SKIP.

- [ ] **Step 4: Migration roundtrip**

```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: 00020-00025 clean.

- [ ] **Step 5: Frontend build**

```bash
cd apps/web && npm run build 2>&1 | tail -10; cd ../..
```
Expected: succeeds.

- [ ] **Step 6: Walk the Definition of Done** (from spec). Verify each ✅/❌; fix any ❌:
1. Migrations roundtrip + seeds idempotent.
2. Mode resolver + override; NORMAL = Phase 5 identical (regression green).
3. Join idempotent; refresh/reconnect/sleep-safe; UNIQUE 1/user/event.
4. Release engine (pure rate) + pause/resume + set-rate.
5. Admission gate checkout (X-Queue-Token); expired → back of line.
6. No oversold + no queue reset + no duplicate token (concurrency -race green).
7. RANDOMIZED + HYBRID seeded reproducible + fairness auditable.
8. Anti-bot hook stub at join (no-op, ready P9).
9. Frontend waiting room verified; organizer pause/resume.
10. Audit complete; human error messages; webhook port untouched.
11. `go test ./... -race` + integration green; sqlc/vet clean.
12. No Phase 1-7 behavior change; docs + CHANGELOG updated.

- [ ] **Step 7: Finishing the branch**

Invoke `superpowers:finishing-a-development-branch` to decide merge/PR/cleanup.

---

Part 4 complete. **Phase 8 done** when the DoD checklist is all green. Next phases: P9 fills the anti-bot guard stub; P10 ballot reuses the registration foundation; P11 adds invitation/priority/community/corporate + waitlist as gate variants.
