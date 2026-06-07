# Phase 3: Event & Category Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Backend-only event & category management on top of the Phase 2 org/RBAC foundation — event CRUD with draft/published/archived lifecycle, category CRUD with validation, pluggable storage (local now, presigned cloud later), and public read-only catalog.

**Architecture:** Three new business modules (`events`, `categories`, `publiccatalog`) following the established `handler → service → repository` layering. One new platform package (`storage`) with a pluggable `Storage` interface: a full `local` disk driver, plus an S3-compatible (R2/Tencent) driver stubbed with a clear contract for presigned uploads. Postgres tables via goose migrations; queries via sqlc. Authz reuses Phase 2 middleware and seeded permissions (`event.*`, `category.manage`).

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, `github.com/google/uuid`. (Cloud driver later: AWS SDK Go v2 S3-compatible.)

**Reference spec:** `docs/superpowers/specs/2026-06-07-phase3-event-category-management-design.md`

**Conventions (verified against the existing codebase):**
- Module path: `github.com/varin/ivyticketing/services/api`.
- Module layout mirrors `internal/modules/organizations`: `Handler`/`NewHandler`, `Service`/`NewService`, `Repository` interface + `sqlcRepo` adapter with `ExecTx(ctx, func(Repository) error)` for transactions, `NewRepository(pool *pgxpool.Pool)`.
- Error envelope: `apperr.New(status, code, message)` + `apperr.WriteError`/`apperr.WriteJSON` from `internal/platform/errors`.
- authn: `middleware.Authn(signer)`; authz: `middleware.RequirePermission(loader, "perm.key")` reading `orgId` URL param.
- Identity in context: `authctx.FromContext(ctx) (Identity, bool)` with `Identity{UserID, IsPlatformAdmin}`.
- Audit: `audit.Logger.Record(ctx, audit.Entry{OrganizationID *uuid.UUID, ActorUserID *uuid.UUID, Action, TargetType, TargetID string, Metadata map[string]any})`.
- **sqlc generated types use `pgtype.*` for nullable columns** (NOT Go pointers), `uuid.UUID` for uuid, `pgtype.Timestamptz` for timestamptz, `pgtype.Text` for nullable text, `int64`/`int32` for bigint/int. Nullable uuid (FK) generates `*uuid.UUID`. **Always check the generated `internal/db/*.go` before writing service/repository code.**
- Migrations: `database/migrations/NNNNN_*.sql` (goose up/down). Phase 2 ended at `00007`. Phase 3 continues `00008`.
- sqlc queries: `database/queries/*.sql`, one file per module.
- Run sqlc: `make sqlc`. Run a single Go test pkg: `cd services/api && go test ./internal/<path>/ -run <Name> -v`.
- Slug helper: `organizations` has an internal `slugify`. Phase 3 needs it in `events`; copy the same impl (small, package-private) — do NOT export/refactor the org one.

**Env added (spec §Env Baru):**
```
STORAGE_DRIVER=local                       # local | r2 | tencent | s3
STORAGE_LOCAL_PATH=./var/media
STORAGE_PUBLIC_BASE_URL=http://localhost:8080
STORAGE_UPLOAD_MAX_BYTES=5242880           # 5MB
STORAGE_BUCKET=                            # cloud only
STORAGE_ENDPOINT=
STORAGE_ACCESS_KEY=
STORAGE_SECRET_KEY=
STORAGE_REGION=
```

---

## File Structure

```txt
database/
├── migrations/
│   ├── 00008_create_events.sql
│   └── 00009_create_event_categories.sql
└── queries/
    ├── events.sql
    └── event_categories.sql

services/api/
├── .env.example                          # MODIFY: add STORAGE_* vars
├── internal/
│   ├── app/
│   │   ├── config.go                     # MODIFY: parse storage config
│   │   └── server.go                     # MODIFY: build storage, mount events/categories/public + /media
│   ├── db/                                # sqlc-generated (regenerated)
│   ├── platform/
│   │   └── storage/
│   │       ├── storage.go                # Storage interface, PutTicket, Config, factory
│   │       ├── local.go                  # disk driver (full)
│   │       ├── local_test.go
│   │       └── s3.go                     # S3-compatible driver (stub, contract only)
│   └── modules/
│       ├── events/        handler service repository dto routes errors media (+ *_test.go)
│       ├── categories/    handler service repository dto routes errors (+ *_test.go)
│       └── publiccatalog/ handler service repository dto routes (+ *_test.go)
└── tests/integration/                     # event flow, tenant isolation, public, upload
```

---

## Plan Parts

Execute in order — each part assumes the previous parts' code exists. Every task is self-contained (TDD, full code, commit per task).

| Part | Tasks | Scope | File |
|---|---|---|---|
| 1 | 1-4 | Foundation: storage config, migrations (events + categories), storage interface + local driver + S3 stub, sqlc queries | [part1-foundation](2026-06-07-phase3-part1-foundation.md) |
| 2 | 5-7 | Events module: errors/dto/repository, service (CRUD + lifecycle + tenant guard), handler/routes + wiring | [part2-events](2026-06-07-phase3-part2-events.md) |
| 3 | 8-11 | Categories module, event media upload flow, public catalog, full router wiring | [part3-categories-media-public](2026-06-07-phase3-part3-categories-media-public.md) |
| 4 | 12-14 | Integration tests (real Postgres), DoD verification, README + CHANGELOG | [part4-integration-verify](2026-06-07-phase3-part4-integration-verify.md) |

---

## Self-Review Notes

**1. Spec coverage** (spec section → task):
- Model Data (events, event_categories) → Tasks 2 (migrations), 4 (queries).
- Storage abstraction (interface, local, S3 stub) → Task 3; config → Task 1.
- Events CRUD + lifecycle (draft/published/archived, publish-needs-category) → Tasks 5-7.
- Categories CRUD + validation → Task 8.
- Media upload flow (ticket → upload → confirm, anti-tamper) → Task 9.
- Public endpoints (published only, by slug) → Task 10; `/media` serve → Tasks 3+11.
- Tenant isolation (404), super-admin bypass → Tasks 5,7 (authz wiring) + Task 12.
- Audit on sensitive actions → Task 7 (events service).
- Error codes → Tasks 5,8,9 (typed errors).
- Env config → Task 1.
- Testing (unit + integration) → Tasks 3,5,8,9 (unit), Task 12 (integration).
- DoD #1-#13 → Task 13/14 mapping; #13 CHANGELOG → Task 14.

**2. Placeholder scan:** The S3 driver is a deliberate stub (signatures + errors final, body returns `ErrNotConfigured`) — documented as a decision, not a TODO. Every other step has full code.

**3. Type consistency:** All modules use the same `Repository`+`ExecTx` pattern, `apperr` envelope, `authctx.Identity`, `audit.Entry`. sqlc nullable types (`pgtype.*`, `*uuid.UUID`) are flagged at each DB-touching site with a verify-against-generated note.

---

## Execution Handoff

**After all parts: Two execution options —**

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per part (sequential, each based on the previous part's branch), review between.

**2. Inline Execution** — execute tasks in-session with checkpoints.
