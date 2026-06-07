# ivyticketing

Race registration & event ticketing platform. Go modular monolith + Astro frontend.

## Phase 1 — Foundation

Thin-but-live monorepo proving `web → api → Postgres + Redis`.

## Prerequisites

- macOS with Homebrew
- Go 1.25+
- Node 20+ and pnpm

## Setup from zero

```bash
make setup    # install + start Postgres/Redis, create db, install tools, migrate, pnpm install
make dev      # API on :8080, web on :4321
```

Open http://localhost:4321 — you should see Postgres ✅ and Redis ✅.

## Smoke test (verify the chain is live)

```bash
curl -s localhost:8080/healthz          # {"status":"ok"}
curl -s localhost:8080/readyz | jq      # both checks "ok"

brew services stop redis
curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/readyz   # 503
brew services start redis
```

## Project structure

- `apps/web` — Astro frontend (public site, participant UI)
- `services/api` — Go modular monolith (Chi, pgx, sqlc)
- `database/migrations` — goose migrations
- `database/queries` — sqlc query sources
- `scripts/dev` — local setup/run scripts
- `docs/` — PRD, struktur, masterplan, specs, plans

## Make targets

`setup`, `dev`, `api`, `web`, `migrate-up`, `migrate-down`, `sqlc`, `test`, `lint`, `fmt`

## Next phase

Phase 2 — Auth, RBAC & multi-tenant core. See `docs/masterplan.md`.
