# CLAUDE.md

Guidance for AI agents working in this repo. Read this before making changes.

## What this is

Race-registration / event-ticketing SaaS for high-traffic events. Go modular monolith
API + Astro frontend + scanner PWA. All 27 masterplan phases are complete
(see [`docs/masterplan.md`](docs/masterplan.md), [`CHANGELOG.md`](CHANGELOG.md)).

## Layout

- `services/api` — Go monolith. Module path: `github.com/varin/ivyticketing/services/api`. Go 1.25.
  - `cmd/api`, `cmd/webhook`, `cmd/worker` — three binaries, shared module code + one DB.
  - `internal/modules/<name>` — one folder per bounded context (auth, orders, payments,
    inventory, tickets, queue, results, enterprise, notifications, …).
  - `internal/db` — sqlc-generated code (do not hand-edit; edit `database/queries/*.sql` then `make sqlc`).
  - `internal/platform` — middleware, auth context, shared plumbing.
- `apps/web` — Astro + Tailwind. UI copy is **Indonesian**.
- `apps/scanner` — offline scanner PWA (Vite).
- `packages/ui` — shared design system.
- `database/migrations` — goose (currently at 61). `database/queries` — sqlc sources.
- `ops/` — k6, Prometheus alerts, Grafana dashboard.

## Backend conventions

- **Module pattern**: `errors.go`, `model.go`, `repository.go`, `service.go`, `handler.go`,
  `routes.go`. Repository is `type sqlcRepo struct{ q *db.Queries }` with `NewRepository(pool)`.
- **Router**: Chi (`go-chi/v5`). Register all middleware (`r.Use`) **before** routes.
- **DB**: pgx v5 (pgxpool), pgtype. sqlc for typed queries. goose for migrations —
  every migration needs a tested `-- +goose Down` or an explicit forward-only reason.
- **Error envelope is nested**: `{"error":{"code":...,"message":...,"requestId":...}}`.
  Use `apper.WriteError` / `apper.WriteJSON` / `apper.New`.
- **RBAC**: `middleware.RequirePermission(loader, "perm.key")`. Identity via
  `authctx.FromContext(ctx)` → `Identity{UserID, IsPlatformAdmin}`.
- **Audit**: `audit.Logger.Record`.
- **Cross-module calls**: inject a function/interface from `server.go` rather than importing
  another module directly — avoids import cycles and secret duplication (see invariants).

## Frontend conventions

- Astro pages, `prerender = false` for authed pages. Organizer pages live under
  `apps/web/src/pages/org/[orgId]/events/[eventId]/*.astro` (OrganizerLayout wrapper);
  participant pages under `apps/web/src/pages/participant/*` (ParticipantLayout).
- `authedFetch<T>` from `lib/api.ts` prepends `/api/v1` and JSON-stringifies the body.
- Escape API-origin strings with `esc()` from `lib/safe.ts` before `innerHTML`.
- **Verify frontend with `astro build`, not `astro check`.**

## Golden invariants (never violate)

1. **No oversell** — inventory gated by `FOR UPDATE` in `inventory.CheckAndLock`.
2. **No double payment** — `payments.Processor` is idempotent on the order state machine.
3. **No secret duplication** — `TICKET_QR_SECRET` composes exactly one `qr.Signer`, shared
   by tickets and scanner. Never create a second signer or copy the secret into another module.

## Verifying changes

```bash
make test                # go test ./...
make test-db-setup       # migrate the test db
make test-integration    # -tags=integration suite
cd apps/web && pnpm build # frontend
```

## Commit style

Conventional commits, phase-tagged: `feat(phaseN): …`, `fix(phaseN.x): …`. Only commit
when asked. Push to a new branch, never directly to main/master, unless told otherwise.
