# Changelog

All notable changes to ivyticketing are documented here.

---

## [Phase 4] — 2026-06-07

Custom registration form builder. Backend-only (builder; submission deferred to Phase 5).

### Added

**Form builder**
- One form per event (auto-created on first `GET /form`)
- Field CRUD: `POST/PUT/DELETE .../events/:eventId/form/fields[/:fieldId]`
- Reorder: `PUT .../form/fields/reorder { fieldIds }`
- Field types: text, email, phone, number, date, dropdown, radio, checkbox, textarea, file
- Per-field validation rules (minLength/maxLength/pattern for text; min/max for number/date)

**Conditional logic**
- Multi-condition AND/OR tree (`{op:"and"|"or", rules:[...]}` + leaves `{field, op, value}`)
- Operators: equals, notEquals, in, notIn, gt, gte, lt, lte
- Acyclic (refs earlier fields only), depth ≤ 3, ≤ 20 leaves/field

**Per-category scoping**
- `categoryScope` limits a field to specific categories (null = all)

**Preview / dry-run**
- `GET .../form/preview?categoryId=` — effective visible fields for a category
- `POST .../form/preview/validate?categoryId=` — runs conditional + validation over sample answers

**Pure logic package** `formschema`
- `ValidateFields` (definition validation), `Evaluate` (conditional), `ValidateAnswers` (preview) — no DB, fully unit-tested

**Database** (goose migrations 00010–00011)
- Tables: `form_schemas`, `form_fields`

**Tests**
- Unit: formschema (validate, conditional AND/OR, answers), forms service (upsert, CRUD, reorder, tenant guard, referenced-field delete)
- Integration: full form flow, conditional show/hide, category scope, tenant isolation (404/403)

---

## [Phase 3] — 2026-06-07

Event & category management. Backend-only.

### Added

**Events**
- CRUD: `POST/GET/PUT/DELETE /api/v1/organizations/:orgId/events[/:eventId]`
- Lifecycle: `publish` (rejects if no categories), `unpublish`, `archive`
- Status: draft → published → archived
- Auto slug from name (unique per org)
- Audit logging on publish/unpublish/archive/delete

**Categories**
- CRUD: `.../events/:eventId/categories[/:categoryId]`
- Fields: price (minor units), capacity, registration window, bib prefix, min age, max order per user
- Validation: price ≥ 0, capacity > 0, opens < closes, max order ≥ 1
- No inventory/stock logic yet (Phase 5) — capacity is a stored number

**Media**
- Pluggable `Storage` interface: full `local` disk driver; S3-compatible (R2/Tencent) stub with presigned-upload contract
- Upload flow: request ticket → (cloud: presigned PUT direct-to-storage; local: multipart to API) → confirm
- Object keys namespaced per tenant (`org/{orgId}/event/{eventId}/{kind}/`), confirm validates prefix (anti-tamper)
- Local media served at `/media/{key}`

**Public catalog** (no auth)
- `GET /api/v1/public/organizations/:orgSlug/events` — published only
- `GET /api/v1/public/organizations/:orgSlug/events/:eventSlug` — detail + categories

**Database** (goose migrations 00008–00009)
- Tables: `events`, `event_categories`

**Config**
- `STORAGE_DRIVER`, `STORAGE_LOCAL_PATH`, `STORAGE_PUBLIC_BASE_URL`, `STORAGE_UPLOAD_MAX_BYTES`, and cloud credential vars

**Tests**
- Unit: events service (lifecycle, tenant guard), categories service (validation), storage local driver, media key validation
- Integration: full event→category→publish→public flow, tenant isolation (404/403), local media upload end-to-end

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
