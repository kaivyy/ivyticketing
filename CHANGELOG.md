# Changelog

All notable changes to ivyticketing are documented here.

---

## [Phase 2] — 2026-06-07

Auth, RBAC, and multi-tenant core. Backend-only.

### Added

**Auth**
- Register, login, logout endpoints
- Hybrid token: access JWT (HS256, 15m TTL) + opaque refresh token (SHA-256 hashed, 7d TTL)
- Refresh token rotation — old token revoked on every refresh
- HttpOnly cookie for refresh token (`/api/v1/auth` path, SameSite=Lax)
- `GET /api/v1/auth/me` — returns user + all org memberships with role slugs and permissions
- JWT config via env: `JWT_SECRET` (required), `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`

**Multi-Tenant Organizations**
- `POST /api/v1/organizations` — create org, copies all role templates, assigns creator as Owner (single transaction)
- `GET /api/v1/organizations` — list orgs the caller belongs to
- `GET /api/v1/organizations/:orgId` — get org (member or platform admin only)

**Members**
- `GET /api/v1/organizations/:orgId/members` — list members with roles
- `POST /api/v1/organizations/:orgId/members` — add member by email, assign roles
- `DELETE /api/v1/organizations/:orgId/members/:memberId` — remove member
- `PUT /api/v1/organizations/:orgId/members/:memberId/roles` — replace member roles
- Last-Owner guard: reject removing or demoting the last Owner

**RBAC**
- `GET /api/v1/organizations/:orgId/roles` — list org roles with permission keys
- `POST /api/v1/organizations/:orgId/roles` — create custom role
- `PUT /api/v1/organizations/:orgId/roles/:roleId` — update role name/permissions
- `DELETE /api/v1/organizations/:orgId/roles/:roleId` — delete role (blocked if in use)
- `GET /api/v1/organizations/:orgId/permissions` — list full permission catalog
- 21 seeded permissions (`member.manage`, `role.manage`, `event.create`, etc.)
- 5 seeded role templates: Owner, Manager, Finance, Customer Service, Racepack Staff
- Template roles copied per org on creation — orgs own their role definitions

**Platform**
- `authn` middleware: Bearer token → identity in context
- `authz` middleware: membership check + permission check + platform admin bypass
- Shared JSON error envelope: `{ "error": { "code", "message", "requestId" } }`
- Audit logging on sensitive member actions (add/remove/update roles)
- bcrypt password hashing, JWT signing/verification, opaque token generation

**Database** (goose migrations 00002–00007)
- Tables: `users`, `organizations`, `organization_members`, `roles`, `permissions`, `role_permissions`, `member_roles`, `refresh_tokens`, `audit_logs`

**Tests**
- Unit tests: security primitives, error envelope, authctx, authn/authz middleware, auth service, organizations service, roles service, members service
- Integration tests (tag: `integration`, DB: `ivyticketing_test`): full register→login→create org→add member flow, tenant isolation (403), seed verification

---

## [Phase 1] — 2026-06-07

Monorepo foundation. Thin-but-live: `Astro web → Go API → Postgres + Redis`.

### Added

- Go modular monolith (`services/api`): Chi router, pgx v5, go-redis v9, sqlc, goose
- Astro frontend (`apps/web`): calls API readiness endpoint, renders dependency health
- `GET /healthz` and `GET /readyz` with Postgres + Redis ping checks
- Homebrew-native Postgres 16 + Redis (no Docker)
- `make setup`, `make dev`, `make migrate-up/down`, `make sqlc`
- RequestID middleware, structured logging (slog)
