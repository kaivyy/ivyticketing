# ivyticketing

Race registration & event ticketing platform. Go modular monolith + Astro frontend.

## Phase 6 — Payment Gateway V1

Duitku + Xendit payment integration. Order `PENDING_PAYMENT` → `PAID` via callback. Webhook receiver as a separate binary on a different port. Idempotent callback processing.

### New env
```bash
WEBHOOK_PORT=8090
PAYMENT_CALLBACK_BASE_URL=http://localhost:8090
PAYMENT_DEFAULT_EXPIRY=15m

DUITKU_ENABLED=false
DUITKU_MERCHANT_CODE=
DUITKU_API_KEY=
DUITKU_ENV=sandbox

XENDIT_ENABLED=false
XENDIT_SECRET_KEY=
XENDIT_CALLBACK_TOKEN=
XENDIT_ENV=sandbox
```

### Run the webhook receiver
```bash
make webhook   # starts on WEBHOOK_PORT (default 8090)
```

### Smoke test (with a gateway enabled)
```bash
# Create payment for an existing PENDING_PAYMENT order
curl -s -X POST localhost:8080/api/v1/orders/<orderId>/payments \
  -H "authorization: Bearer <accessToken>" \
  -H "content-type: application/json" \
  -d '{"gateway":"duitku","method":"qris"}'
# → 201 { status: "PENDING", merchantReference: "PAY-...", qrString: "..." }

# Check payment status
curl -s localhost:8080/api/v1/payments/<paymentId> \
  -H "authorization: Bearer <accessToken>"
```

## Phase 5 — Orders, Inventory & Checkout

Participant checkout with atomic oversold-prevention, reservation system, and an
expiration worker. Plus `packages/ui` design-system foundation. No payment yet (Phase 6).

### New env

```bash
ORDER_EXPIRATION=15m
WORKER_INTERVAL=1m
```

### Run the expiration worker

```bash
make worker   # ticks every WORKER_INTERVAL, expires stale PENDING_PAYMENT orders
```

### Smoke test

```bash
# as a logged-in participant (access token from login)
curl -s -X POST localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/categories/<categoryId>/checkout \
  -H "authorization: Bearer <accessToken>"
# → 201 { orderNumber: "ORD-...", status: "PENDING_PAYMENT", total: 100000 }

curl -s localhost:8080/api/v1/orders -H "authorization: Bearer <accessToken>"
curl -s -X DELETE localhost:8080/api/v1/orders/<orderId> -H "authorization: Bearer <accessToken>"
```

## Phase 4 — Custom Registration Form Builder

Per-event form builder: fields (text/email/dropdown/etc), per-field validation,
AND/OR conditional logic, per-category field scoping, and a preview/dry-run validator.
Builder only — participant submission arrives in Phase 5.

### Smoke test

```bash
# auto-create form + add a field (needs form.manage; use an Owner token)
curl -s localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/form \
  -H "authorization: Bearer <accessToken>"

curl -s -X POST localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/form/fields \
  -H "authorization: Bearer <accessToken>" -H 'content-type: application/json' \
  -d '{"fieldType":"dropdown","label":"WNA","fieldKey":"wna","options":["Ya","Tidak"]}'

# dry-run validate
curl -s -X POST "localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/form/preview/validate" \
  -H "authorization: Bearer <accessToken>" -H 'content-type: application/json' \
  -d '{"answers":{"wna":"Ya"}}'
```

## Phase 3 — Event & Category Management

Event CRUD with draft/published/archived lifecycle, categories, pluggable storage
(local now; R2/Tencent via presigned URL later), and a public read-only catalog.

### New env

```bash
STORAGE_DRIVER=local                       # local | r2 | tencent | s3
STORAGE_LOCAL_PATH=./var/media
STORAGE_PUBLIC_BASE_URL=http://localhost:8080
STORAGE_UPLOAD_MAX_BYTES=5242880
# cloud (when used): STORAGE_BUCKET, STORAGE_ENDPOINT, STORAGE_ACCESS_KEY, STORAGE_SECRET_KEY, STORAGE_REGION
```

### Smoke test

```bash
# create event (needs event.create; use an Owner access token from Phase 2 login)
curl -s -X POST localhost:8080/api/v1/organizations/<orgId>/events \
  -H "authorization: Bearer <accessToken>" -H 'content-type: application/json' \
  -d '{"name":"Jakarta Marathon 2026","eventType":"marathon"}'

# add a category
curl -s -X POST localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/categories \
  -H "authorization: Bearer <accessToken>" -H 'content-type: application/json' \
  -d '{"name":"42K","price":350000,"capacity":2000,"registrationOpensAt":"2026-07-01T00:00:00Z","registrationClosesAt":"2026-08-01T00:00:00Z","maxOrderPerUser":1}'

# publish, then view publicly
curl -s -X POST localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/publish \
  -H "authorization: Bearer <accessToken>"
curl -s localhost:8080/api/v1/public/organizations/<orgSlug>/events
```

## Phase 2 — Auth, RBAC & Multi-Tenant

Backend auth (hybrid token), multi-tenant orgs, and custom-role RBAC.

### New env vars

```bash
JWT_SECRET=change-me        # REQUIRED — API won't start without it
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
```

Add these to your `.env` file.

### Smoke test

```bash
# register + login
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'content-type: application/json' \
  -d '{"email":"a@b.com","password":"pw123456","fullName":"A"}'

curl -s -c cookies.txt -X POST localhost:8080/api/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"email":"a@b.com","password":"pw123456"}'
# → { "accessToken": "...", "expiresIn": 900, "user": {...} }

# create org (use the accessToken from login)
curl -s -X POST localhost:8080/api/v1/organizations \
  -H "authorization: Bearer <accessToken>" \
  -H 'content-type: application/json' \
  -d '{"name":"Jakarta Marathon"}'
```

### Integration tests

```bash
make test-db-setup       # create + migrate ivyticketing_test
make test-integration    # run -tags=integration suite
```

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
