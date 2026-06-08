# Phase 8: Queue / War Ticket System — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Registration Access Engine foundation (mode resolver + per-event/category settings) and fill it with queue/war modes (WAR_QUEUE, RANDOMIZED_QUEUE, HYBRID_QUEUE): waiting room with persistent tokens, a release engine, admission-gated checkout, and admin pause/resume — all extending the Phase 1-7 production baseline without changing existing behavior.

**Architecture:** Two new modules — `registration` (mode foundation, shared with Phase 9-11) and `queue` (token/release/admission) — plus a `platform/queue` Redis sorted-set adapter. Checkout admission is enforced via a `RegistrationGate` interface declared in `orders` (dependency inversion, same pattern as Phase 7 `TicketIssuer`). Postgres is the durable source of truth (tokens, admissions, control, audit); Redis holds real-time position/state for 100k-scale; Postgres wins on conflict. A worker tick releases N waiting users per interval into admission windows.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, `redis/go-redis/v9` (sorted sets), `github.com/google/uuid`. Frontend: Astro 4, Tailwind, TypeScript (reuses Phase 7 auth foundation).

**Reference spec:** `docs/superpowers/specs/2026-06-08-phase8-queue-war-system-design.md`

**EXTEND, DON'T REWRITE.** Phase 1-7 is production baseline. Do not change their API, auth, order/inventory/payment/ticket flow, or move/rename existing files. Phase 8 = new migrations + new `registration` & `queue` modules + `platform/queue` + worker jobs + additive `server.go`/`cmd/worker` wiring + new `apps/web` waiting room only. NORMAL mode = Phase 5 checkout behavior, identical.

**Conventions (verified against the existing codebase):**
- Module path: `github.com/varin/ivyticketing/services/api`.
- Module layout mirrors `payments`/`orders`/`tickets`: `Handler`/`NewHandler`, `Service`/`NewService`, `Repository` interface + `sqlcRepo` with `ExecTx(ctx, func(Repository) error)`, `NewRepository(pool *pgxpool.Pool)`.
- Error envelope: `apperr.New(status, code, message)` + `apperr.WriteError`/`apperr.WriteJSON` from `internal/platform/errors`.
- Identity: `authctx.FromContext(ctx) (Identity, bool)`, `Identity{UserID uuid.UUID, IsPlatformAdmin bool}` from `internal/platform/authctx`.
- authz: `middleware.RequirePermission(loader, "queue.manage")` for organizer routes; participant routes use authn + ownership filter (mismatch → 404).
- Audit: `audit.Logger.Record(ctx, audit.Entry{OrganizationID *uuid.UUID, ActorUserID *uuid.UUID, Action, TargetType, TargetID string, Metadata map[string]any})`.
- sqlc nullable types use `pgtype.*`; nullable uuid FK → `*uuid.UUID`; `bigint`→`int64`; `integer`→`int32`; `timestamptz`→`pgtype.Timestamptz`; nullable text→`pgtype.Text`. **Always check generated `internal/db/*.go` before writing service/repo code.**
- Migrations: `database/migrations/NNNNN_*.sql` (goose up/down). Phase 7 ended at `00019`. Phase 8 continues `00020`.
- sqlc queries: `database/queries/*.sql`. Run sqlc: `make sqlc` (repo root).
- Worker: `worker.New(name, interval, job, log).Run(ctx)`; `Job = func(ctx context.Context) error` from `internal/platform/worker`. Existing `cmd/worker/main.go` runs `expire_orders`.
- Redis: `internal/platform/redis` exposes `*redis.Redis{Client *redis.Client}`. `cmd/api/main.go` builds it and passes to `app.NewRouter` as `system.Checker`. Phase 8 changes `NewRouter` to also accept the concrete `*redis.Redis` for the queue module.
- Orders seam: `orders.Service.Checkout(ctx, participantID, eventID, categoryID)` ([service.go:31]); gate check happens via `checkoutEligible(event, cat, now)` ([validator.go:9]). Handler is `POST /categories/{categoryId}/checkout`.
- Role template slugs (org_id NULL, is_system): `owner`, `manager`, `finance`, `customer-service`, `racepack-staff`.
- Single test pkg: `cd services/api && go test ./internal/<path>/ -run <Name> -v`. Race: `-race`. Integration: `TEST_DATABASE_URL` set, build tag `//go:build integration`, helpers in `services/api/tests/integration/helpers_test.go` (`testPool`, `truncate`, `newTestServer`, `registerAndLogin`, `loginCreateOrg`, `publishEventWithCategory`, `postJSON`).

**Env added:**
```
QUEUE_RELEASE_INTERVAL=10s        # worker queue_release tick
QUEUE_DEFAULT_RELEASE_RATE=100    # users per interval default
QUEUE_CHECKOUT_WINDOW=5m          # admission TTL (clamped ≤ ORDER_EXPIRATION)
# REDIS_URL already exists (Phase 1)
```

---

## File Structure

```txt
database/
├── migrations/
│   ├── 00020_create_registration_settings.sql
│   ├── 00021_seed_registration_permissions.sql
│   ├── 00022_create_queue_tokens.sql
│   ├── 00023_create_queue_admissions.sql
│   ├── 00024_create_queue_control.sql
│   └── 00025_seed_queue_manage.sql
└── queries/
    ├── registration.sql
    └── queue.sql

services/api/
├── internal/
│   ├── app/
│   │   ├── config.go            # MODIFY: QUEUE_* config
│   │   ├── config_test.go       # MODIFY: queue config tests
│   │   └── server.go            # MODIFY: accept *redis.Redis; wire registration+queue; gate into orders
│   ├── db/                       # sqlc-regenerated
│   ├── platform/queue/           # NEW: Redis sorted-set adapter
│   │   ├── queue.go  queue_test.go
│   └── modules/
│       ├── orders/
│       │   ├── validator.go      # MODIFY: checkoutEligible → delegates to RegistrationGate
│       │   ├── service.go        # MODIFY: Checkout accepts admissionToken; calls gate
│       │   ├── gate.go           # NEW: RegistrationGate interface (declared here)
│       │   └── handler.go        # MODIFY: read X-Queue-Token header
│       ├── registration/         # NEW: mode foundation
│       │   ├── model.go resolver.go resolver_test.go gate.go service.go
│       │   ├── repository.go handler.go routes.go dto.go errors.go
│       │   └── tests/
│       └── queue/                # NEW
│           ├── model.go score.go score_test.go token.go service.go
│           ├── release.go admission.go control.go store.go
│           ├── repository.go handler.go routes.go guard.go dto.go errors.go
│           └── tests/
├── cmd/
│   ├── api/main.go               # MODIFY: pass *redis.Redis to NewRouter
│   └── worker/main.go            # MODIFY: add queue_release + queue_admission_expiry jobs
└── tests/integration/            # phase8_*_test.go

apps/web/src/
├── lib/queue.ts                  # NEW
├── pages/events/[slug]/queue.astro  # NEW (prerender=false)
└── components/queue/ WaitingRoom.astro QueueStatus.astro  # NEW

docs/
├── REGISTRATION_MODES.md QUEUE_MODES.md QUEUE_OPERATIONS.md PHASE8_DECISIONS.md
└── CHANGELOG.md (MODIFY)
```

---

## Plan Parts

Execute in order — each part assumes previous parts exist. Foundation-first (decision Q2).

1. **[Part 1: Registration Mode Foundation](2026-06-08-phase8-part1-registration-foundation.md)** (Tasks 1-7)
   Config; migrations (settings); sqlc; `registration` module (enum, resolver, settings service/handler); `RegistrationGate` interface in orders; gate wired into `checkoutEligible` for NORMAL/CLOSED; organizer settings endpoint. NORMAL stays identical (regression-safe).

2. **[Part 2: Queue Core + Waiting Room](2026-06-08-phase8-part2-queue-core.md)** (Tasks 8-15)
   Queue migrations (tokens/admissions/control); `platform/queue` Redis adapter; `queue` module (scoring, token issue idempotent, join/status service); WAR_QUEUE join + status endpoints; anti-bot guard stub; admission validate. No release engine yet.

3. **[Part 3: Release Engine + Admission Gate + Admin](2026-06-08-phase8-part3-release-admin.md)** (Tasks 16-22)
   Release engine + worker job; admission expiry worker; checkout admission gate (X-Queue-Token, WAR mode); pause/resume/set-rate + stats (queue.manage); integration + concurrency tests (no oversold, no queue reset, no duplicate).

4. **[Part 4: Randomized + Hybrid + Frontend + Docs](2026-06-08-phase8-part4-randomized-frontend-docs.md)** (Tasks 23-30)
   Seeded-random scoring (reproducible) for RANDOMIZED_QUEUE + HYBRID_QUEUE presale pool; fairness audit; waiting room frontend (Astro); organizer queue controls UI; load test scaffold (k6); docs + CHANGELOG; final verification + DoD.
