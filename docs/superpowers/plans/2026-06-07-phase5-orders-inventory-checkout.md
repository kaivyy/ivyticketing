# Phase 5: Orders, Inventory & Checkout Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Backend orders + inventory + reservation with atomic oversold-prevention, an expiration worker, and a `packages/ui` design-system foundation — all extending the Phase 1-4 production baseline without changing existing behavior.

**Architecture:** Two new business-domain packages (`orders` with HTTP, `inventory` domain-only) under the modular monolith. Checkout runs in a single Postgres transaction using `SELECT ... FOR UPDATE` on the category row as the serialization point. A separate worker binary (`cmd/worker`) runs a 1-minute ticker to expire stale orders and release reservations (idempotent, `FOR UPDATE SKIP LOCKED`). A `packages/ui` Tailwind+Radix design system holds presentational components. Audit via the existing Phase 2 logger.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, `github.com/google/uuid`. UI: Tailwind, Radix, TypeScript.

**Reference spec:** `docs/superpowers/specs/2026-06-07-phase5-orders-inventory-checkout-design.md`

**EXTEND, DON'T REWRITE.** Phase 1-4 is production baseline. Do not change their API, auth, form flow, behavior, or move/rename existing files. Phase 5 = new migrations + new modules + additive `server.go` wiring only.

**Conventions (verified against the existing codebase):**
- Module path: `github.com/varin/ivyticketing/services/api`.
- Module layout mirrors `internal/modules/events`: `Handler`/`NewHandler`, `Service`/`NewService`, `Repository` interface + `sqlcRepo` adapter with `ExecTx(ctx, func(Repository) error)` for transactions, `NewRepository(pool *pgxpool.Pool)`.
- Error envelope: `apperr.New(status, code, message)` + `apperr.WriteError`/`apperr.WriteJSON` from `internal/platform/errors`.
- Identity: `authctx.FromContext(ctx) (Identity, bool)`, `Identity{UserID uuid.UUID, IsPlatformAdmin bool}`.
- authz: `middleware.RequirePermission(loader, "order.view")` for organizer routes; participant routes use authn only + ownership filter.
- Audit: `audit.Logger.Record(ctx, audit.Entry{OrganizationID *uuid.UUID, ActorUserID *uuid.UUID, Action, TargetType, TargetID string, Metadata map[string]any})`.
- **sqlc generated types use `pgtype.*` for nullable columns** (NOT pointers); nullable uuid FK → `*uuid.UUID`; `bigint` → `int64`; `integer` → `int32`; `timestamptz` → `pgtype.Timestamptz`; nullable text → `pgtype.Text`. **Always check generated `internal/db/*.go` before writing service/repository code.**
- Migrations: `database/migrations/NNNNN_*.sql` (goose up/down). Phase 4 ended at `00011`. Phase 5 continues `00012`.
- sqlc queries: `database/queries/*.sql`, one file per module.
- `EventCategory` (Phase 3) has `Capacity int32`, `MaxOrderPerUser int32`, `Price int64`, `Status` on `Event` is `"draft"|"published"|"archived"`. Category has `RegistrationOpensAt`/`RegistrationClosesAt pgtype.Timestamptz`.
- Existing `GetCategoryByID`, `GetEventByID` queries exist (Phase 3) — reuse via new repo methods that call `db.Queries`.
- Run sqlc: `make sqlc`. Single test pkg: `cd services/api && go test ./internal/<path>/ -run <Name> -v`. Race: `go test -race`.

**Env added:**
```
ORDER_EXPIRATION=15m      # default order PENDING_PAYMENT TTL
WORKER_INTERVAL=1m        # expiration worker tick (worker binary only)
```

---

## File Structure

```txt
database/
├── migrations/
│   ├── 00012_create_orders.sql
│   ├── 00013_create_inventory_reservations.sql
│   └── 00014_seed_order_permissions.sql
└── queries/
    ├── orders.sql
    └── inventory.sql

services/api/
├── cmd/
│   └── worker/main.go                    # worker binary
├── internal/
│   ├── app/
│   │   ├── config.go                     # MODIFY: add OrderExpiration, WorkerInterval
│   │   └── server.go                     # MODIFY: build orders handler, mount routes
│   ├── db/                                # sqlc-generated (regenerated)
│   ├── platform/
│   │   └── worker/
│   │       ├── worker.go                 # ticker runner
│   │       └── worker_test.go
│   └── modules/
│       ├── orders/   handler service repository model dto validator routes errors (+ tests)
│       └── inventory/ service repository reservation expiration stock lock (+ tests)
└── tests/integration/                     # checkout, cancel, expiration, concurrency

packages/ui/                               # design system (Tailwind + Radix + TS)
├── package.json  tailwind.config.cjs  tsconfig.json  README.md
├── src/theme.css  src/index.ts
└── src/components/{Button,Input,Select,...}.tsx

docs/
├── ORDER_FLOW.md  INVENTORY.md  RESERVATION_SYSTEM.md  CHECKOUT_FLOW.md  PHASE5_DECISIONS.md
```

---

## Plan Parts

Execute in order — each part assumes the previous parts' code exists. Every task is self-contained (TDD, full code, commit per task). Parts 1-3 + 5 are backend (sequential dependency). Part 4 (`packages/ui`) is independent and may run anytime.

| Part | Tasks | Scope | File |
|---|---|---|---|
| 1 | 1-4 | Config + migrations (orders, reservations, permissions) + sqlc queries + inventory domain (lock/stock/reservation) | [part1-foundation-inventory](2026-06-07-phase5-part1-foundation-inventory.md) |
| 2 | 5-7 | Orders module: errors/dto/model/repository, service (checkout atomic, cancel, list), handler/routes + wiring | [part2-orders-module](2026-06-07-phase5-part2-orders-module.md) |
| 3 | 8-9 | Expiration worker (platform/worker + cmd/worker) + concurrency tests | [part3-worker-concurrency](2026-06-07-phase5-part3-worker-concurrency.md) |
| 4 | 10-11 | packages/ui design system (theme, primitives, domain shells, README) | [part4-ui-foundation](2026-06-07-phase5-part4-ui-foundation.md) |
| 5 | 12-14 | Integration tests + docs (5 md) + DoD verification + CHANGELOG | [part5-integration-docs-verify](2026-06-07-phase5-part5-integration-docs-verify.md) |

---

## Self-Review Notes

**1. Spec coverage** (spec section → task):
- orders + inventory_reservations tables → Task 2; permissions → Task 2; queries → Task 3.
- inventory formula + lock + stock → Task 4.
- order number generator → Task 5.
- checkout atomic (FOR UPDATE) + max_order + oversold → Task 6.
- order CRUD + ownership + organizer list → Tasks 6-7.
- expiration worker idempotent → Task 8.
- concurrency (200 vs 100) → Task 9.
- audit ORDER_*/RESERVATION_* → Tasks 6, 8.
- packages/ui → Tasks 10-11.
- docs (5 md) → Task 13.
- integration tests → Task 12.
- DoD #1-#14 → Task 14; CHANGELOG → Task 14.

**2. Placeholder scan:** No TODO/placeholder logic. The `DRAFT` status and `PAID`/`REFUNDED` transitions exist in the enum for Phase 6 but are explicitly not exercised in Phase 5 — documented, not dead code. UI domain cards (Queue/Payment/Ticket) are presentational shells by design (no data layer yet).

**3. Type consistency:** orders/inventory share the `ExecTx`+`Repository` pattern; checkout calls inventory ops within the same tx-wrapped repo. `apperr` envelope, `authctx.Identity`, `audit.Entry` reused. Each DB-touching site carries a verify-against-generated note.

---

## Execution Handoff

**After all parts: Two execution options —**

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per part (Parts 1→2→3→5 sequential on the same branch chain; Part 4 independent), review between.

**2. Inline Execution** — execute tasks in-session with checkpoints.
