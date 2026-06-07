# Phase 2: Auth, RBAC & Multi-Tenant Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a backend-only SaaS foundation — hybrid-token auth, multi-tenant organizations/membership, and custom-role RBAC with granular permissions — proven by unit + integration tests.

**Architecture:** Extends the Phase 1 Go modular monolith (`services/api`). Four new business modules (`auth`, `organizations`, `members`, `roles`) follow the established `handler → service → repository` layering. New platform packages provide password hashing (bcrypt), JWT signing (HS256), opaque refresh tokens (SHA-256), an auth-identity context, and authn/authz middleware. Postgres tables added via goose migrations; queries go through sqlc. Permission catalog + system role templates are seeded via migration; org creation copies templates into org-owned roles.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, `golang.org/x/crypto/bcrypt`, `github.com/golang-jwt/jwt/v5`, `github.com/google/uuid`.

**Reference spec:** `docs/superpowers/specs/2026-06-07-phase2-auth-rbac-multitenant-design.md`

**Conventions (verified against the existing codebase):**
- Module path: `github.com/varin/ivyticketing/services/api`.
- Module layout mirrors `internal/modules/system`: a `Handler` struct, `NewHandler(...)`, and a `RegisterRoutes(r chi.Router)` method (see [struktur.md](../../struktur.md) §4).
- Existing JSON error envelope target (struktur.md §19): `{ "error": { code, message, requestId } }`. Phase 1 had no shared error helper — this plan adds one in `internal/platform/errors`.
- DB naming: `snake_case` columns, `camelCase` JSON, `PascalCase` Go structs.
- sqlc currently emits raw `pgtype.*`. Phase 2 has many UUID FKs, so Task 1 adds sqlc `overrides` (uuid→`google/uuid`, timestamptz→`time.Time`, citext→`string`) to keep generated code ergonomic.
- Migrations live in `database/migrations/NNNNN_*.sql` (goose, up/down). Phase 1 used `00001_`. Phase 2 continues `00002_…`.
- sqlc queries live in `database/queries/*.sql`, one file per module.
- Run sqlc: `cd services/api && "$(go env GOPATH)/bin/sqlc" generate`. Run a single Go test pkg: `cd services/api && go test ./internal/<path>/ -run <Name> -v`.

**Env added (spec §Env Baru):**
```
JWT_SECRET=            # REQUIRED, no default — API fails to start if empty
ACCESS_TOKEN_TTL=15m   # default
REFRESH_TOKEN_TTL=168h # default (7 days)
```

---

## File Structure

New and modified files, grouped by responsibility.

```txt
database/
├── migrations/
│   ├── 00002_create_users.sql                 # users
│   ├── 00003_create_organizations.sql         # organizations, organization_members
│   ├── 00004_create_rbac.sql                  # roles, permissions, role_permissions, member_roles
│   ├── 00005_create_refresh_tokens.sql        # refresh_tokens
│   ├── 00006_create_audit_logs.sql            # audit_logs
│   └── 00007_seed_rbac_catalog.sql            # permission catalog + system role templates (idempotent)
└── queries/
    ├── users.sql
    ├── organizations.sql
    ├── members.sql
    ├── roles.sql
    ├── refresh_tokens.sql
    └── audit_logs.sql

services/api/
├── sqlc.yaml                                  # MODIFY: add overrides
├── .env.example                               # MODIFY: add JWT_SECRET, *_TTL
├── internal/
│   ├── app/
│   │   ├── config.go                          # MODIFY: parse JWT secret + TTLs
│   │   └── server.go                          # MODIFY: build deps, mount module routes
│   ├── db/                                     # sqlc-generated (regenerated)
│   ├── platform/
│   │   ├── errors/errors.go                   # shared API error + JSON writer
│   │   ├── security/
│   │   │   ├── password.go                    # bcrypt hash/verify
│   │   │   ├── jwt.go                          # HS256 sign/verify access token
│   │   │   └── token.go                        # opaque refresh token + SHA-256 hash
│   │   ├── authctx/authctx.go                  # identity in context.Context
│   │   └── middleware/
│   │       ├── authn.go                        # verify access JWT → context
│   │       └── authz.go                        # permission check within org context
│   └── modules/
│       ├── auth/        handler service repository dto routes errors (+ *_test.go)
│       ├── organizations/  handler service repository dto routes errors (+ *_test.go)
│       ├── members/        handler service repository dto routes errors (+ *_test.go)
│       └── roles/          handler service repository dto routes errors (+ *_test.go)
└── tests/integration/                          # full-flow + tenant-isolation tests
```

---


## Plan Parts

This plan is split into four parts by dependency boundary. Execute in order — each part assumes the previous parts' code exists. Every task is self-contained (TDD, full code, commit per task).

| Part | Tasks | Scope | File |
|---|---|---|---|
| 1 | 1-5 | Foundation: deps + config (JWT/TTL), 6 migrations + RBAC seed, security primitives (bcrypt/JWT/refresh), error envelope + authctx, sqlc queries | [part1-foundation](2026-06-07-phase2-part1-foundation.md) |
| 2 | 6-8 | Auth module (register/login/refresh/logout) + handler/cookie/authn middleware, organizations (create→copy templates→assign Owner) | [part2-auth-org](2026-06-07-phase2-part2-auth-org.md) |
| 3 | 9-12 | RBAC: authz middleware (membership/permission/super-admin), permission loader + audit helper, roles CRUD, members (+ last-Owner guard) | [part3-rbac](2026-06-07-phase2-part3-rbac.md) |
| 4 | 13-16 | Handlers + full router/main wiring, enriched `/me` + audit, integration tests (real Postgres), README + DoD verification | [part4-wiring-verify](2026-06-07-phase2-part4-wiring-verify.md) |

## Self-Review Notes

**1. Spec coverage** (each spec section → task):
- Model Data (9 tables) → Task 2 (migrations 00002-00006).
- Seeding & permission catalog → Task 2 (00007), verified Task 15 seed_test.
- Go module structure (auth/orgs/members/roles + platform security/authctx/middleware) → Tasks 3-13.
- Token hybrid flow (register/login/refresh/logout/me) → Tasks 6, 7, 14.
- Refresh rotation + SHA-256 hashing + revoke → Task 3 (token), Task 6 (rotation).
- JWT minimal claims (sub/exp/iat/is_platform_admin), no permissions in JWT → Task 3 (jwt.go).
- Refresh cookie attributes (HttpOnly/Secure/SameSite=Lax/Path=/api/v1/auth) → Task 7 (handler).
- Error envelope `{error:{code,message,requestId}}` → Task 4 (errors.go).
- Multi-tenant endpoints + authz by permission in org context → Tasks 9, 13.
- Tenant isolation (member A ≠ org B → 403) → Task 9 + Task 15.
- Super admin bypass → Task 9 (authz).
- Last-Owner guard → Task 12 (members service).
- Reject delete is_system / role-in-use → Task 11 (roles service).
- Template copy on org create → Task 8.
- Audit on sensitive actions → Task 14.
- Env (JWT_SECRET required, TTL defaults) → Task 1 (config).
- Testing (unit per spec list + integration full flow / isolation / seed) → Tasks 3,6,8,9,11,12 (unit), Task 15 (integration).
- Definition of Done #1-#12 → Task 16 mapping.

**2. Placeholder scan:** No "TODO/TBD/implement later". Every code step contains full code. The `var _ =` guards and pointer/type "Notes" are explicit instructions tied to a build step that will confirm or flag — not vague placeholders.

**3. Type consistency:**
- `security.NewJWTSigner(secret string, ttl Duration)`, `Sign(uuid.UUID, bool)`, `Verify→Claims{UserID, IsPlatformAdmin}` — consistent across Tasks 3, 7, 13.
- `security.GenerateRefreshToken()→(raw,hash,err)`, `HashToken(raw)→hash` — Tasks 3, 6.
- `authctx.Identity{UserID, IsPlatformAdmin}`, `WithIdentity`/`FromContext` — Tasks 4, 7, 9, 13.
- `apperr.New(status, code, message)`, `WriteError`, `WriteJSON` — Tasks 4, 7, 11, 12, 13.
- `middleware.Authn(signer)`, `middleware.RequirePermission(loader, perm)`, `PermissionLoader.LoadPermissions(ctx, orgID, userID)→(map, bool, err)` — Tasks 7, 9, 10, 13.
- Repository `ExecTx(ctx, func(Repository) error)` pattern — Tasks 8, 11, 12.
- sqlc param/row types depend on Task 1 overrides (uuid→`uuid.UUID`, nullable→pointer, timestamptz→`time.Time`); every module touching generated types carries a Note to verify against the generated file. This is the single biggest execution risk and is flagged at each site.

**Known coupling to verify at runtime (not a plan gap, but call it out):** the exact generated names — `ListMembersByOrgRow`, `CreateUserParams.PasswordHash *string`, `CreateAuditLogParams.Metadata []byte`, `ListRolesByOrg` param pointer-ness — are predicted from the sqlc config but only confirmed after `make sqlc` in Task 5. Each dependent task includes a Note to adjust if sqlc emits a different shape.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-06-07-phase2-auth-rbac-multitenant.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
