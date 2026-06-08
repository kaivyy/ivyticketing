# Phase 9: Anti-Bot & Abuse Protection — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an application-layer anti-bot/abuse protection chain (blocklist → rate limit → Turnstile → IP reputation) enforced as middleware on sensitive entry points, all runtime-toggleable by super admin via DB settings — extending the Phase 1-8 baseline without changing existing behavior, and never touching the webhook port.

**Architecture:** A new `abuse` module (settings cache, blocklist, reputation, rate-limit wrapper, guard middleware, super-admin endpoints) plus two platform packages (`platform/ratelimit` Redis token bucket, `platform/captcha` Turnstile verifier + fake) and a `RequirePlatformAdmin` middleware. The guard is mounted in `server.go` in front of queue-join, auth login/register, and checkout — replacing the Phase 8 `EntryGuard` no-op. Postgres holds settings/blocklist/ip-rules/abuse-log/reputation; Redis holds ephemeral rate counters. Webhook (:8090) is never guarded.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, `redis/go-redis/v9`, `crypto/sha256`, `net/http`. Frontend: Astro 4 (Turnstile widget).

**Reference spec:** `docs/superpowers/specs/2026-06-08-phase9-antibot-abuse-design.md`

**EXTEND, DON'T REWRITE.** Phase 1-8 is production baseline. Do not change their API/auth/order/payment/ticket/queue behavior or move/rename files. Phase 9 = new migrations + new `abuse` module + `platform/ratelimit` + `platform/captcha` + `RequirePlatformAdmin` + additive `server.go` middleware wiring + frontend Turnstile widget only.

**Conventions (verified against the existing codebase):**
- Module path: `github.com/varin/ivyticketing/services/api`.
- Module layout mirrors `payments`/`queue`: `Handler`/`NewHandler`, `Service`/`NewService`, `Repository` interface + `sqlcRepo` with `NewRepository(pool *pgxpool.Pool)`. Use `ExecTx` only where multi-write transactions are needed.
- Error envelope: `apperr.New(status, code, message)` + `apperr.WriteError`/`apperr.WriteJSON` from `internal/platform/errors`.
- Identity: `authctx.FromContext(ctx) (Identity, bool)`, `Identity{UserID uuid.UUID, IsPlatformAdmin bool}` from `internal/platform/authctx`.
- Super admin: `id.IsPlatformAdmin` bool on the identity (see `middleware/authz.go:35`). No `RequirePlatformAdmin` exists yet — Task creates it.
- Audit: `audit.Logger.Record(ctx, audit.Entry{OrganizationID *uuid.UUID, ActorUserID *uuid.UUID, Action, TargetType, TargetID string, Metadata map[string]any})`.
- Config helpers: `getEnv(key, fallback string) string`, `getDuration(key, fallback)`, `getInt64(key, fallback int64)`. No `getInt` — use `getInt64` + cast. (`config.go:167-186`.)
- sqlc nullable types use `pgtype.*`; nullable uuid FK → `*uuid.UUID`; `bigint`→`int64`; `integer`→`int32`; `timestamptz`→`pgtype.Timestamptz`; nullable text→`pgtype.Text`; `jsonb`→`[]byte`. **Always check generated `internal/db/*.go` before writing service/repo code.**
- Migrations: `database/migrations/NNNNN_*.sql` goose up/down. Phase 8 ended at `00025`. Phase 9 continues `00026`.
- sqlc queries: `database/queries/*.sql`. Run sqlc: `make sqlc` (repo root). Migrate: `make migrate-up`/`migrate-down`.
- Redis: `internal/platform/redis` exposes `*redis.Redis{Client *redis.Client}`. `NewRouter` already receives a `*goredis.Client` (Phase 8). Reuse it for ratelimit.
- Queue: `queue.EntryGuard` is a no-op stub in `services/api/internal/modules/queue/guard.go`, applied in `queue/routes.go` as `r.With(EntryGuard).Post("/events/{eventId}/queue/join", h.Join)`. Phase 9 removes that `With(EntryGuard)` and the server mounts the abuse guard instead.
- Auth routes self-mount via `authHandler.RegisterRoutes(r, signer)` (login/register/refresh/logout/me). To guard login/register, pass an optional middleware into RegisterRoutes OR wrap in server.go (Task details below).
- Single test pkg: `cd services/api && go test ./internal/<path>/ -run <Name> -v`. Race: `-race`. Integration: `//go:build integration`, `TEST_DATABASE_URL` + Redis; helpers in `services/api/tests/integration/helpers_test.go`.

**Env added:**
```
TURNSTILE_SECRET=                    # Cloudflare Turnstile secret (siteverify); empty → verify fail-open
TURNSTILE_SITE_KEY=                  # public site key (frontend uses PUBLIC_TURNSTILE_SITE_KEY)
MAX_ACTIVE_QUEUE_PER_USER=5          # cross-event active queue cap
REPUTATION_CHALLENGE_THRESHOLD=10
REPUTATION_DENY_THRESHOLD=25
ABUSE_SETTINGS_REFRESH=30s           # platform_settings cache refresh interval
```
Feature on/off toggles live in DB `platform_settings` (runtime), NOT env.

---

## File Structure

```txt
database/
├── migrations/
│   ├── 00026_create_platform_settings.sql   (+ seed default rows)
│   ├── 00027_create_blocked_subjects.sql
│   ├── 00028_create_ip_rules.sql
│   ├── 00029_create_abuse_log.sql
│   └── 00030_create_ip_reputation.sql
└── queries/
    └── abuse.sql

services/api/
├── internal/
│   ├── app/
│   │   ├── config.go        # MODIFY: TURNSTILE_*, MAX_ACTIVE_QUEUE_PER_USER, REPUTATION_*, ABUSE_SETTINGS_REFRESH
│   │   ├── config_test.go   # MODIFY: tests
│   │   └── server.go        # MODIFY: build abuse guard; mount on queue-join/auth/checkout; mount admin routes; remove queue EntryGuard usage
│   ├── db/                   # sqlc-regenerated
│   ├── platform/
│   │   ├── ratelimit/ ratelimit.go ratelimit_test.go        # NEW
│   │   ├── captcha/ captcha.go turnstile.go fake.go captcha_test.go  # NEW
│   │   └── middleware/ platformadmin.go platformadmin_test.go  # NEW
│   └── modules/
│       ├── queue/
│       │   ├── guard.go      # MODIFY: remove/retire EntryGuard (or leave unused)
│       │   └── routes.go     # MODIFY: drop With(EntryGuard); join guarded by server.go
│       └── abuse/            # NEW
│           ├── model.go settings.go blocklist.go reputation.go ratelimit.go
│           ├── fingerprint.go clientip.go guard.go service.go repository.go
│           ├── handler.go routes.go dto.go errors.go securityconfig.go
│           └── tests/
└── tests/integration/        # phase9_abuse_test.go

apps/web/src/
├── components/security/Turnstile.astro   # NEW
└── lib/security.ts                        # NEW (fetch /security/config, helpers)

docs/
├── ANTIBOT.md RATE_LIMITING.md ABUSE_OPERATIONS.md PHASE9_DECISIONS.md
└── CHANGELOG.md (MODIFY)
```

---

## Plan Parts

Execute in order — each part assumes previous parts exist.

1. **[Part 1: Foundation — config, migrations, platform packages](2026-06-08-phase9-part1-foundation.md)** (Tasks 1-8)
   Config; `RequirePlatformAdmin`; migrations (settings/blocklist/ip_rules/abuse_log/ip_reputation) + sqlc; `platform/ratelimit` (Redis token bucket); `platform/captcha` (Turnstile + fake); abuse `model`/`clientip`/`fingerprint`.

2. **[Part 2: Abuse module — settings, blocklist, reputation, guard](2026-06-08-phase9-part2-abuse-module.md)** (Tasks 9-15)
   `settings` cache (runtime toggle); `blocklist` + ip_rules; `reputation`; `ratelimit` wrapper; `guard` middleware chain; `service` (admin ops + queue cap); abuse `repository`.

3. **[Part 3: Endpoints, wiring, frontend, docs](2026-06-08-phase9-part3-wiring-frontend-docs.md)** (Tasks 16-23)
   Super-admin handler/routes; `/security/config` public endpoint; `server.go` wiring (mount guard on queue-join/auth/checkout, admin routes); remove queue EntryGuard; frontend Turnstile widget; integration + concurrency tests; docs + CHANGELOG; final verification + DoD.
