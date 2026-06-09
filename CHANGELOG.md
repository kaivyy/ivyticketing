# Changelog

All notable changes to ivyticketing are documented here.

---

## [Phase 11] — 2026-06-09

### Added
- Access Engine: full pool type support (RESERVED, COMMUNITY, CORPORATE, SPONSOR, VIP, PARTNER, PRIORITY, ELITE)
- Access codes: create, bulk-generate, revoke, sha256-hashed storage
- Code redemption: atomic reserve + grant issuance, eligibility rules (5 types)
- Corporate module: account management, bulk CSV upload, invoice JSON, approval flow
- Priority window: auto-eligibility grant for PRIORITY_ACCESS mode
- WAITLIST_ONLY mode: grant-based slot promotion via WaitlistEngine
- RAE gate: INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY fully implemented (no more ErrModeNotAvailable)
- Security: code brute-force block (Redis INCR, auto-block after 3 failures/60s), reputation bump (+2) on failed redemption
- Frontend: "I have an access code" modal, priority countdown, waitlist position, corporate management page
- Admin: list codes across events, emergency quota adjustment
- k6 load tests: redemption burst (10k concurrent), quota exhaustion (exactly N grants)
- Docs: ACCESS_ENGINE.md operations guide

### Changed
- registration/gate.go: NewGate gains accessGrant + priority injected interfaces
- access_pools: owner_account_id, is_visible_to_participants, eligibility_rule columns added
- ratelimit.Limiter: added IncrExpire() for brute-force counter support
- abuse.Guard: added WithBruteForce(), TrackCodeFailure(), BumpReputation() public methods

---

## [Phase 10] — 2026-06-09

### Added
- Ballot draw engine: deterministic Fisher-Yates shuffle, seeded with sha256(event|category|nonce)
- Ballot lifecycle: PENDING → OPEN → CLOSED → DRAWN → ANNOUNCED state machine
- Winner grant issuance: RESERVED AccessPool created per draw, grants issued to winners at announce
- Waitlist promotion: lapsed winners auto-promote from ballot waitlist (WinnerExpirer job)
- Participant endpoints: apply, my-entry, withdraw
- BallotAdmitter: ModeBallot wired into RAE gate
- CSV export: full draw results downloadable by organizer
- Result hash verification: public endpoint to verify deterministic draw integrity
- Registration lifecycle: LifecycleChecker fail-open gate for all non-NORMAL modes
- Waitlist module: FIFO/RANDOMIZED rank, join/promote/expire/withdraw
- AccessPool module: typed pools (RESERVED/COMMUNITY/etc), atomic slot reservation
- k6 load test: ballot application burst (2000 VU)

### Changed
- NewGate now accepts BallotAdmitter (4th arg) — pass nil for non-ballot modes

---

## [Phase 9] — 2026-06-08

Anti-bot system: guard middleware chain (blocklist → rate limit → reputation → captcha → queue cap), Redis token-bucket rate limiter, Cloudflare Turnstile adapter, IP reputation scorer, runtime DB-toggled feature flags, super-admin abuse endpoints, and frontend Turnstile widget.

### Added

**Abuse Module**
- Settings cache: reads `platform_settings` from DB on startup; background ticker refreshes every `ABUSE_SETTINGS_REFRESH` (default 30s); fail-safe defaults (all guards on)
- Blocklist: `blocked_subjects` table; per-IP and per-user block/unblock; fail-safe on DB error
- IP allow/deny rules: `ip_rules` table; allow wins over deny; CIDR support
- Reputation scorer: `ip_reputation` table; score bumped on abuse signals; challenge/deny thresholds configurable via env
- Guard middleware chain: blocklist → rate limit → reputation → Turnstile → queue cap; injected via middleware params (modules do not import abuse package)
- Guard applied to: queue-join, auth login/register, checkout

**Rate Limiter** (`platform/ratelimit`)
- Redis fixed-window token bucket (INCR + EXPIRE on first hit)
- Per-category limits: `queue_join` (10/IP, 5/user), `checkout` (20/IP, 10/user), `auth_login` (10/IP, 5/user), `auth_register` (5/IP), `default` (120/IP)
- Key format: `ratelimit:{category}:ip:{ip}` and `ratelimit:{category}:user:{userID}`
- Fail-open on Redis error

**Captcha** (`platform/captcha`)
- Turnstile adapter: verifies `CF-Turnstile-Response` against Cloudflare siteverify API
- Fake adapter for test/dev environments
- Fail-open on Cloudflare API error

**RequirePlatformAdmin Middleware**
- Platform-level super-admin flag on `users` table; separate from org roles
- Required for all `/api/v1/admin/abuse/*` endpoints

**Super-Admin Endpoints** (`/api/v1/admin/abuse/`)
- `GET/PUT /settings` — read and toggle guard feature flags live (no redeploy)
- `POST /block` and `/unblock` — block/unblock user or IP
- `POST /ip-rules` and `DELETE /ip-rules/{id}` — add/remove IP allow/deny rules
- `GET /log` — paginated abuse event log with filters
- `POST /reputation/reset` — manually reset IP reputation score

**Public Endpoint**
- `GET /api/v1/security/config` — returns `captcha_enabled` and `turnstile_site_key` for frontend consumption

**Frontend** (`apps/web`)
- Turnstile widget on queue-join, login, register, and checkout pages; gated by security config response; passes token in request header

**Database** (goose migrations 00026–00030)
- Migration `00026_create_platform_settings`: `platform_settings` key/value table
- Migration `00027_create_blocked_subjects`: `blocked_subjects` (type, subject_id, reason, timestamps)
- Migration `00028_create_ip_rules`: `ip_rules` (cidr, rule_type, note, timestamps)
- Migration `00029_create_abuse_log`: `abuse_log` (subject_type, subject_id, action, metadata, timestamps)
- Migration `00030_create_ip_reputation`: `ip_reputation` (ip, score, last_seen, timestamps)

**Config**
- `TURNSTILE_SECRET` — Cloudflare Turnstile secret key (required when captcha enabled)
- `TURNSTILE_SITE_KEY` — Cloudflare Turnstile site key (served to frontend)
- `MAX_ACTIVE_QUEUE_PER_USER` (default 3) — queue cap per user
- `REPUTATION_CHALLENGE_THRESHOLD` (default 10) — score at which captcha is required
- `REPUTATION_DENY_THRESHOLD` (default 25) — score at which request is blocked
- `ABUSE_SETTINGS_REFRESH` (default 30s) — interval for reloading platform_settings from DB

**Docs**: ANTIBOT, RATE_LIMITING, ABUSE_OPERATIONS, PHASE9_DECISIONS

### Deferred

- Cloudflare WAF edge config (WAF rules, IP reputation feed, bot fight mode) — deployment-layer complement, out of application code
- External IP reputation feed (AbuseIPDB etc.) — behavior-score model sufficient for initial scale
- Client-side JS fingerprinting — server-side UA+IP+Accept-Language hash covers current needs

---

## [Phase 8] — 2026-06-08

Queue / War Ticket System: registration mode foundation, persistent queue tokens with dual-store (Postgres + Redis), seeded pseudo-random scoring, release engine with admission windows, expiry requeue worker, anti-bot guard stub, waiting room frontend, and organizer controls.

### Added

**Registration Mode Foundation**
- 9-mode enum: `NORMAL`, `WAR_QUEUE`, `RANDOMIZED_QUEUE`, `HYBRID_QUEUE`, `BALLOT`, `INVITATION_ONLY`, `PRIORITY_ACCESS`, `WAITLIST_ONLY`, `CLOSED`
- Resolver with category-overrides-event logic: category override > event mode > NORMAL default
- Per-event settings: `PUT /registration` (default mode, feature flags), `GET /registration`
- Per-category settings: `PUT /registration/category` (mode override, override enabled flag)
- `registration.manage` permission (migration 00021, assigned to Owner + Manager templates)

**RegistrationGate Seam**
- `RegistrationGate` interface defined in orders package (dependency inversion: orders does not import registration)
- `noopGate{}` default preserves Phase 5 NORMAL behaviour when no gate is wired (regression-safe)
- Gate resolves mode at checkout time: NORMAL lets through, CLOSED returns `REGISTRATION_CLOSED`, queue modes delegate to `QueueAdmitter.CheckAdmission`
- Deferred modes (BALLOT/INVITATION_ONLY/PRIORITY_ACCESS/WAITLIST_ONLY) return `REGISTRATION_MODE_NOT_AVAILABLE`
- `X-Queue-Token` header read from checkout request, passed through to admission check (stateless, REST-compatible)

**Queue Module**
- Three queue modes: `WAR_QUEUE` (pure FIFO), `RANDOMIZED_QUEUE` (presale seeded random + post-sale FIFO), `HYBRID_QUEUE` (same ordering as RANDOMIZED)
- Persistent tokens: `UNIQUE (event_id, participant_id)` with `ON CONFLICT DO NOTHING` -- idempotent join, safe for refresh/reconnect/mobile sleep
- Token state machine: `WAITING` → `ALLOWED` → `COMPLETED`, with `BLOCKED` reserved for Phase 9
- Admission lifecycle: `ACTIVE` → `CONSUMED` (on checkout) / `EXPIRED` (on timeout)

**Scoring**
- `FifoScore(now) = now.UnixNano()` -- monotonic wall-clock join ordering
- `PresaleScore(seed, participantID) = SHA256(seed || participantID) >> 1` -- deterministic, seeded, non-negative, reproducible
- Pool ordering: `ORDER BY pool DESC, score ASC` -- PRESALE pool sorts before FIFO

**Release Engine**
- Worker-driven job (`ReleaseJob`) runs every `QUEUE_RELEASE_INTERVAL` (default 10s)
- Pure-rate promotion: `ListWaiting ORDER BY pool DESC, score ASC LIMIT rate` → `MarkAllowed` + `CreateAdmission`
- Idempotent: `MarkAllowed WHERE status='WAITING'` no-ops if already promoted concurrently
- Paused events skipped; `rate <= 0` events skipped
- `QUEUE_DEFAULT_RELEASE_RATE` default: 100 tokens/tick

**Admission Expiry Worker**
- `AdmissionExpiryJob` runs every `QUEUE_RELEASE_INTERVAL`, scans `ACTIVE` admissions past `checkout_expires_at`
- Expired admission → token requeued to **back of WAITING line** with new FIFO score (decision Q10)
- `QUEUE_CHECKOUT_WINDOW` default: 5 minutes

**Redis Sorted-Set Adapter** (`platform/queue`)
- `queue:{eventID}:waiting` -- sorted set of participant UUIDs scored by join score
- `queue:{eventID}:allowed` -- sorted set scored by checkout expiration Unix timestamp
- Atomic move operations: `MoveToAllowed`, `MoveToWaiting` (pipeline: `ZREM` + `ZADD`)
- Best-effort side effect after Postgres writes; position degrades gracefully if Redis is down
- Fully rebuildable from Postgres `WAITING` tokens

**Participant Endpoints**
- `POST /events/{eventId}/queue/join` -- join the queue (idempotent, returns token + position)
- `GET /events/{eventId}/queue/status` -- status, position, ETA, admission token (if ALLOWED), checkout expiry
- Both pass through `EntryGuard` middleware (anti-bot stub, Phase 9 fills)

**Admin Controls** (require `queue.manage`, migration 00025)
- `POST .../queue/pause` -- pause release engine
- `POST .../queue/resume` -- resume release engine
- `PUT .../queue/release-rate` -- adjust per-tick release rate live
- `GET .../queue/stats` -- waiting/allowed counts, release rate, state
- `PUT .../queue/schedule` -- set randomization seed, sale start, presale pool open

**Anti-Bot Guard**
- `EntryGuard` middleware at join entry point -- current no-op pass-through; Phase 9 implements Turnstile + rate limit + duplicate detection

**Frontend Waiting Room** (`apps/web`)
- `pages/events/[eventId]/queue.astro`: auto-poll 4s, visibility-change re-poll on tab focus, position/ETA display, ALLOWED redirect with admission token
- `components/queue/WaitingRoom.astro`: shared waiting room component
- `pages/organizations/[orgId]/events/[eventId]/queue-controls.astro`: organizer pause/resume, rate slider, live stats

**Database** (goose migrations 00020-00025)
- Migration `00020_create_registration_settings`: `event_registration_settings`, `category_registration_settings`
- Migration `00021_seed_registration_permissions`: `registration.manage` permission
- Migration `00022_create_queue_tokens`: `queue_tokens` table with `UNIQUE (event_id, participant_id)`
- Migration `00023_create_queue_admissions`: `queue_admissions` table with FK to `queue_tokens`
- Migration `00024_create_queue_control`: `queue_control` per-event control row
- Migration `00025_seed_queue_manage`: `queue.manage` permission (Owner + Manager templates)

**Config**
- `QUEUE_RELEASE_INTERVAL` (default 10s)
- `QUEUE_DEFAULT_RELEASE_RATE` (default 100)
- `QUEUE_CHECKOUT_WINDOW` (default 5m)

**Load Test**
- `tests/load/queue-war.js`: k6 scaffold with 10k/50k/100k VU stages, join + status poll loop

**Docs**: REGISTRATION_MODES, QUEUE_MODES, QUEUE_OPERATIONS, PHASE8_DECISIONS

### Deferred

- Anti-bot full implementation → Phase 9 (Turnstile + rate limit + duplicate detection)
- Ballot mode → Phase 10
- INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY, community/corporate modes → Phase 11
- WebSocket realtime position updates → future phase
- Redis rebuild script → future phase
- Per-organization default settings → future phase

---

## [Phase 7] — 2026-06-08

Tickets module: atomic ticket issuance on payment, HMAC-signed QR tokens, participant dashboard + organizer ticket list, minimal web auth foundation.

### Added

**Tickets**
- New `tickets` package: `CreateTicket`, `GetTicketByID`, `GetTicketByOrderID`, `ListTicketsByParticipant`, `ListTicketsByEvent`
- `tickets/qr` package: HMAC-SHA256 signed token (`v.base64url(payload).base64url(sig)`), payload contains only UUIDs + version (no PII)
- `qr.Sign` and `qr.Verify` — stateless; DB not required for signature check
- Ticket state machine: `VALID` (issued), `USED` (Phase 15 scan), `CANCELLED` (reserved for refund)

**Atomic issuance**
- `TicketIssuer` interface wired into `payments.Processor.applyPaid` — ticket `INSERT` runs inside the same transaction as `MarkPaymentPaid` + `UpdateOrderStatus` + `CompleteReservations`
- PAID ⟺ ticket exists: issuer error triggers full rollback; order stays `PENDING_PAYMENT`
- Idempotent: `UNIQUE (order_id)` + `ON CONFLICT DO NOTHING` — duplicate callbacks produce exactly one ticket
- Verified by `TestProcessor_ApplyPaid_IssuerError_RollsBack`

**Participant endpoints**
- `GET /api/v1/tickets` — list my tickets
- `GET /api/v1/tickets/{ticketId}` — ticket detail + QR token
- `GET /api/v1/tickets/{ticketId}/qr` — QR token only
- `GET /api/v1/orders/{orderId}/ticket` — ticket for order
- `GET /api/v1/orders/{orderId}/invoice` — invoice JSON (PAID orders only; `INVOICE_NOT_AVAILABLE` otherwise)
- Ownership: participant resources filtered by `participant_id = caller`; mismatch → 404

**Organizer endpoints**
- `GET /api/v1/organizations/{orgId}/events/{eventId}/tickets` — list event tickets (requires `ticket.view`)

**Database** (goose migrations 00018–00019)
- Migration `00018_create_tickets`: `tickets` table (`id`, `order_id` UNIQUE, `participant_id`, `event_id`, `category_id`, `qr_token`, `qr_version`, `status`, timestamps)
- Migration `00019_seed_ticket_view`: permission `ticket.view` assigned to Owner, Manager, Customer Service role templates

**Frontend** (`apps/web`)
- Participant dashboard: login, dashboard overview, orders list/detail, tickets list/detail
- Minimal auth: access token in `sessionStorage` + HttpOnly refresh cookie; silent refresh on 401
- Client-side QR rendering via `qrcode` library (`<canvas>`)
- Invoice print via browser `@media print` CSS (no server-side PDF)

**Config**
- `TICKET_QR_SECRET` (required, separate from `JWT_SECRET`)

**Docs**: TICKET_FLOW, QR_TICKET, PARTICIPANT_DASHBOARD, PHASE7_DECISIONS

### Deferred

- QR verify/scan endpoint → Phase 15 (Scanner PWA)
- PDF invoice backend → future phase
- Ticket cancellation/refund → future phase (status reserved)

---

## [Phase 6] — 2026-06-07

Payment Gateway V1: Duitku + Xendit (QRIS/VA/e-wallet), idempotent callback processing, separate webhook binary.

### Added

**Payments**
- `POST /api/v1/orders/:orderId/payments` — create payment (QRIS/VA/e-wallet), returns pay_url/qr_string/va_number
- `GET /api/v1/orders/:orderId/payments` — payment history for order
- `GET /api/v1/payments/:paymentId` — payment status (participant-owned)
- `GET /api/v1/organizations/:orgId/events/:eventId/payments` — org payment list (payment.view)
- `POST /api/v1/organizations/:orgId/payments/:paymentId/reconcile` — manual reconcile (payment.manage)

**Callback processing**
- Separate webhook binary `services/api/cmd/webhook` (port 8090) — `make webhook`
- Store-then-process: raw callback always persisted before validation
- Two-layer idempotency: dedupe_key + DB status guards
- `POST /webhooks/duitku`, `POST /webhooks/xendit`
- Race handling: order expired before callback → payment PAID, order unchanged, noted for reconcile

**Gateway abstraction**
- `Gateway` interface: `CreateCharge`, `VerifySignature`, `ParseCallback`, `QueryStatus`
- `BuildPaymentRegistry` from config; fail-fast if gateway enabled with missing credentials
- Duitku adapter: MD5 signature, form-encoded callback, status codes 00/01/02
- Xendit adapter: x-callback-token header, JSON payload, status mapping

**Database** (goose migrations 00015–00017)
- Tables: `payments`, `payment_webhooks`
- Permission: `payment.manage` (assigned to Owner + Finance templates)

**Config**: `WEBHOOK_PORT`, `PAYMENT_CALLBACK_BASE_URL`, `PAYMENT_DEFAULT_EXPIRY`, `DUITKU_*`, `XENDIT_*`

**Docs**: PAYMENT_FLOW, WEBHOOK_PROCESSING, GATEWAY_INTEGRATION, PAYMENT_RECONCILIATION, PHASE6_DECISIONS, docs/payment/DUITKU, XENDIT, CALLBACK_SECURITY

---

## [Phase 5] — 2026-06-07

Orders, inventory, reservation, and checkout foundation + UI design system. Backend + UI; no payment yet.

### Added

**Orders**
- Checkout: `POST /api/v1/organizations/:orgId/events/:eventId/categories/:categoryId/checkout` → PENDING_PAYMENT order + reservation (atomic)
- `GET /api/v1/orders`, `GET /api/v1/orders/:id`, `DELETE /api/v1/orders/:id` (participant-owned)
- `GET /api/v1/organizations/:orgId/events/:eventId/orders` (organizer, order.view)
- Status machine: DRAFT/PENDING_PAYMENT/PAID/EXPIRED/CANCELLED/REFUNDED
- Order number `ORD-YYYYMMDD-XXXXXX` (unique, crypto-random)

**Inventory & Reservation**
- Source of truth = PostgreSQL: `remaining = capacity - active_reservations - paid_orders`
- Oversold prevention via `SELECT ... FOR UPDATE` on the category row inside a transaction
- max_order_per_user enforced
- Reservation lifecycle ACTIVE → EXPIRED/RELEASED/COMPLETED, one per order

**Expiration worker** (`services/api/cmd/worker`)
- Ticker (`WORKER_INTERVAL`, default 1m) expires PENDING_PAYMENT orders past `expired_at`, releases reservations; idempotent (`FOR UPDATE SKIP LOCKED` + status guards)
- `make worker`

**Audit**
- ORDER_CREATED, ORDER_EXPIRED, ORDER_CANCELLED, RESERVATION_CREATED, RESERVATION_EXPIRED

**UI foundation** (`packages/ui`)
- Tailwind + Radix design system: Button, Input, Select, Textarea, Checkbox, Radio, Badge, Alert, Card, Modal, Dialog, Table, EmptyState, LoadingState, ErrorState, QueueCard, PaymentCard, TicketCard
- Theme tokens + README

**Database** (goose migrations 00012–00014)
- Tables: `orders`, `inventory_reservations`; permissions `order.create`, `order.manage`

**Config**: `ORDER_EXPIRATION`, `WORKER_INTERVAL`

**Docs**: ORDER_FLOW, INVENTORY, RESERVATION_SYSTEM, CHECKOUT_FLOW, PHASE5_DECISIONS

**Tests**
- Unit: stock formula, order-number generator, orders service (checkout/cancel/max-order/ownership), expiration idempotency, worker ticker
- Integration: HTTP checkout flow, ownership isolation
- Concurrency (`-race`): 200 vs capacity 100 → no oversell, unique order numbers, worker idempotent

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
