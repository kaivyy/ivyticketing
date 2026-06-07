# Phase 4: Custom Registration Form Builder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Backend-only custom registration form builder on top of the Phase 3 event/category foundation — relational form schema + fields, multi-condition AND/OR conditional logic, per-category field scoping, definition validation, and a preview/dry-run validate endpoint. Submission (participant filling the form) is deferred to Phase 5.

**Architecture:** One new business module (`forms`) following the established `handler → service → repository` layering. One new pure platform package (`formschema`) holding field types, definition validation, the conditional-logic AND/OR evaluator, and answer validation — no DB, no HTTP, fully unit-testable. Postgres tables via goose migrations; queries via sqlc. Authz reuses Phase 2 middleware + the seeded `form.manage` permission. Routes mount under `/events/{eventId}/form` via the events module's `mountSubRoutes` callback (same mechanism categories uses).

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, `github.com/google/uuid`.

**Reference spec:** `docs/superpowers/specs/2026-06-07-phase4-form-builder-design.md`

**Conventions (verified against the existing codebase):**
- Module path: `github.com/varin/ivyticketing/services/api`.
- Module layout mirrors `internal/modules/categories`: `Handler`/`NewHandler`, `Service`/`NewService`, `Repository` interface + `sqlcRepo` adapter, `NewRepository(pool *pgxpool.Pool)`.
- Error envelope: `apperr.New(status, code, message)` + `apperr.WriteError`/`apperr.WriteJSON` from `internal/platform/errors`.
- authz: `middleware.RequirePermission(loader, "form.manage")`; authn already applied at the `/organizations/{orgId}` group.
- **sqlc generated types use `pgtype.*` for nullable columns** (NOT pointers): `pgtype.Text` for nullable text, `pgtype.Timestamptz` for timestamptz, `[]byte` for jsonb, `uuid.UUID` for uuid, `int32` for integer, `bool` for boolean. **Always check the generated `internal/db/*.go` before writing service/repository code.**
- jsonb columns (`options`, `validation`, `conditional`, `category_scope`) generate as `[]byte` — the service marshals/unmarshals via the `formschema` types.
- Migrations: `database/migrations/NNNNN_*.sql` (goose up/down). Phase 3 ended at `00009`. Phase 4 continues `00010`.
- sqlc queries: `database/queries/*.sql`, one file per module (`forms.sql`).
- Events module exposes `RegisterRoutes(r, loader, mountSubRoutes ...func(chi.Router))`; forms mounts through the same callback list alongside categories (see server.go).
- Tenant mismatch → 404 (pattern from Phase 3): event must belong to `orgId`, form/field must belong to event/org.
- Run sqlc: `make sqlc`. Single Go test pkg: `cd services/api && go test ./internal/<path>/ -run <Name> -v`.

---

## File Structure

```txt
database/
├── migrations/
│   ├── 00010_create_form_schemas.sql
│   └── 00011_create_form_fields.sql
└── queries/
    └── forms.sql

services/api/
├── internal/
│   ├── app/
│   │   └── server.go                     # MODIFY: build forms handler, mount via events callback
│   ├── db/                                # sqlc-generated (regenerated)
│   ├── platform/
│   │   └── formschema/
│   │       ├── field.go                  # Field type, FieldType allowlist, Validation struct
│   │       ├── conditional.go            # AND/OR tree types, parse, validate, Evaluate
│   │       ├── conditional_test.go
│   │       ├── validate.go               # ValidateFields([]Field) error
│   │       ├── validate_test.go
│   │       ├── answers.go                # ValidateAnswers(fields, answers, categoryID) []FieldError
│   │       └── answers_test.go
│   └── modules/
│       └── forms/    handler service repository dto routes errors (+ *_test.go)
└── tests/integration/                     # form flow, conditional, category scope, tenant
```

---

## Plan Parts

Execute in order — each part assumes the previous parts' code exists. Every task is self-contained (TDD, full code, commit per task).

| Part | Tasks | Scope | File |
|---|---|---|---|
| 1 | 1-3 | Foundation: migrations (form_schemas + form_fields), sqlc queries, `formschema` field types + definition validation | [part1-foundation](2026-06-07-phase4-part1-foundation.md) |
| 2 | 4-5 | `formschema` conditional AND/OR evaluator + ValidateAnswers (pure, unit-tested) | [part2-formschema-logic](2026-06-07-phase4-part2-formschema-logic.md) |
| 3 | 6-8 | forms module: errors/dto/repository, service (upsert, field CRUD, reorder, preview), handler/routes + wiring | [part3-forms-module](2026-06-07-phase4-part3-forms-module.md) |
| 4 | 9-11 | Integration tests, DoD verification, README + CHANGELOG | [part4-integration-verify](2026-06-07-phase4-part4-integration-verify.md) |

---

## Self-Review Notes

**1. Spec coverage** (spec section → task):
- Model Data (form_schemas, form_fields) → Task 1 (migrations), 2 (queries).
- formschema field types + definition validation → Task 3.
- Conditional AND/OR evaluator + validation (acyclic, depth limits) → Task 4.
- ValidateAnswers (preview) → Task 5.
- Form upsert + field CRUD + reorder → Tasks 6-7.
- Preview / dry-run validate endpoints → Task 7-8.
- category_scope per-field filtering → Tasks 5 (filter logic), 7 (service), 9 (integration).
- Tenant isolation (404), super-admin bypass → Tasks 6-8 + Task 9.
- Error codes → Tasks 6 (typed errors), 3-5 (formschema errors mapped in service).
- Testing (unit + integration) → Tasks 3,4,5,6 (unit), Task 9 (integration).
- DoD #1-#12 → Task 10/11 mapping; #12 CHANGELOG → Task 11.

**2. Placeholder scan:** No TODO/TBD. The `file` field type is allowed in definitions but its answer-value handling is explicitly deferred (Phase 5) — documented, not a gap.

**3. Type consistency:** `forms` module uses the same `Repository`+`sqlcRepo` pattern, `apperr` envelope. `formschema` is pure (no DB/HTTP). jsonb columns map to `[]byte`; the service marshals `formschema` types to/from `[]byte`. Each DB-touching site carries a verify-against-generated note.

---

## Execution Handoff

**After all parts: Two execution options —**

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per part (sequential, each based on the previous part's branch), review between.

**2. Inline Execution** — execute tasks in-session with checkpoints.
