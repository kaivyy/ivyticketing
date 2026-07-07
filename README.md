<div align="center">

# 🏁 ivyticketing

**Platform registrasi & tiket event lari skala besar — dibangun untuk "war ticket".**

Tens of thousands of runners hitting checkout in the same second — no oversell, no double
payment, no calo. A Go modular monolith, an Astro frontend, and an offline-capable
scanner PWA.

<br />

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![Astro](https://img.shields.io/badge/Astro-frontend-FF5D01?logo=astro&logoColor=white)
![Postgres](https://img.shields.io/badge/PostgreSQL-pgx%20v5-4169E1?logo=postgresql&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-queue-DC382D?logo=redis&logoColor=white)
![Phases](https://img.shields.io/badge/masterplan-27%2F27%20complete-brightgreen)
![License](https://img.shields.io/badge/license-see%20LICENSE-lightgrey)

</div>

---

## Kenapa ivyticketing

Registrasi lomba lari besar itu ujian rekayasa, bukan sekadar form. Ribuan orang menekan
"daftar" di detik yang sama saat slot dibuka. Platform ini dirancang khusus untuk momen itu:

- **⚡ Antrean adil (war ticket).** Redis-backed queue menahan lonjakan tanpa server jatuh.
- **🔒 Tanpa oversell.** Kuota dikunci di level database — slot habis benar-benar habis.
- **💳 Tanpa dobel bayar.** Pemrosesan callback gateway idempoten pada state machine order.
- **🎫 E-ticket QR instan.** Tiket ber-tanda-tangan HMAC terbit begitu pembayaran terkonfirmasi.
- **🏅 Sertifikat digital.** Hasil waktu tempuh & sertifikat finisher otomatis untuk peserta.
- **📱 Scanner offline.** PWA untuk panitia lapangan; scan tersimpan di IndexedDB, sync exactly-once.

All 27 masterplan phases are complete. See [`docs/masterplan.md`](docs/masterplan.md)
for the phase-by-phase plan and [`CHANGELOG.md`](CHANGELOG.md) for what shipped.

## Tech stack

| Layer | Technology |
| --- | --- |
| **Backend** | Go 1.25 · Chi router · pgx v5 · sqlc · goose migrations |
| **Frontend** | Astro · Tailwind CSS · TypeScript |
| **Scanner** | Vite PWA · IndexedDB (offline queue) |
| **Data** | PostgreSQL · Redis (queue / rate-limit) |
| **Payments** | Duitku · Xendit (QRIS, VA, e-wallet) |
| **Ops** | k6 (load test) · Prometheus · Grafana · Sentry |

## Architecture

```
ivyticketing/
├── services/api/       Go modular monolith — one codebase, three binaries
│   ├── cmd/api         request-path API                        (:8080)
│   ├── cmd/webhook     payment-callback receiver, isolated      (:8090)
│   └── cmd/worker      background runners: notify, retry, expire
├── apps/web/           Astro + Tailwind — public site, participant,
│                       organizer console, super-admin. UI in Indonesian.
├── apps/scanner/       offline scanner PWA — racepack pickup + check-in
├── packages/ui/        shared design-system foundation
├── database/           goose migrations + sqlc query sources
├── ops/                k6 scenarios, Prometheus alerts, Grafana dashboard
└── docs/               PRD, masterplan, flow docs, runbooks
```

The API is one codebase deployed as three binaries. `cmd/webhook` and `cmd/worker` are
split out so a checkout spike can never drop a gateway callback or starve background
work of request-path latency — the proof that module boundaries are extraction-ready
(see [`docs/SCALE_SPLIT.md`](docs/SCALE_SPLIT.md)).

### Golden invariants

These are enforced in code and must survive any refactor or future service split
(see [`docs/SCALE_SPLIT.md`](docs/SCALE_SPLIT.md)):

- **No oversell.** Inventory is gated by a `FOR UPDATE` row lock in `inventory.CheckAndLock`.
- **No double payment.** `payments.Processor` is idempotent on the order state machine;
  duplicate gateway callbacks are safe.
- **No secret duplication.** `TICKET_QR_SECRET` composes exactly one `qr.Signer`, shared
  by the tickets and scanner modules. Never mint a second signer or copy the secret.

## Prerequisites

- macOS with Homebrew (Linux works with equivalent Postgres/Redis)
- Go 1.25+
- Node 20+ and pnpm

## Setup from zero

```bash
cp .env.example .env    # then set JWT_SECRET and TICKET_QR_SECRET
make setup              # install + start Postgres/Redis, create db, migrate, pnpm install
make dev                # API on :8080, web on :4321
```

Open http://localhost:4321 — you should see Postgres ✅ and Redis ✅.

Two secrets are required before the API will start:

- `JWT_SECRET` — access/refresh token signing.
- `TICKET_QR_SECRET` — HMAC secret for QR ticket signing (separate from `JWT_SECRET`).

See [`.env.example`](.env.example) for the full list, grouped by phase.

## Make targets

| Target | What it does |
| --- | --- |
| `setup` | Install deps, start Postgres/Redis, create db, migrate, pnpm install |
| `dev` | Run API + web together |
| `api` / `worker` / `webhook` | Run a single backend binary |
| `web` | Run the Astro dev server |
| `migrate-up` / `migrate-down` | Apply / roll back goose migrations |
| `sqlc` | Regenerate typed query code |
| `test` | `go test ./...` |
| `test-db-setup` / `test-integration` | Migrate the test db, run the `-tags=integration` suite |
| `lint` / `fmt` | `go vet` / `go fmt` + prettier |

### Scanner PWA

```bash
pnpm --filter @ivyticketing/scanner dev        # Vite dev server
pnpm --filter @ivyticketing/scanner build      # production PWA build
pnpm --filter @ivyticketing/scanner test       # unit + property tests
```

Set `VITE_API_BASE_URL` (see `apps/scanner/.env.example`) to point the app at your API.

## Smoke test (verify the chain is live)

```bash
curl -s localhost:8080/healthz          # {"status":"ok"}
curl -s localhost:8080/readyz | jq      # db + redis checks "ok"
```

## What's built (by phase)

| Phase | Area | Phase | Area |
| --- | --- | --- | --- |
| 1 | Monorepo & dev foundation | 15 | Scanner PWA |
| 2 | Auth, RBAC & multi-tenant | 16 | Reporting & CSV export |
| 3 | Event & category management | 17 | Super-admin platform billing |
| 4 | Custom registration form builder | 18 | White label & custom domain |
| 5 | Inventory, order & checkout | 19 | Public status & incident system |
| 6 | Payment gateway (Duitku, Xendit) | 20 | Observability & war room |
| 7 | Participant dashboard & ticket | 21 | Load testing & reliability |
| 8 | Queue / war ticket system | 22 | Security hardening |
| 9 | Anti-bot & abuse protection | 23 | Enterprise API & integration |
| 10 | Ballot / lottery system | 24 | Result, certificate & timing |
| 11 | Coupon, invitation & community slot | 25 | Enterprise scale split (guide) |
| 12 | Notification system | 26 | Production launch checklist |
| 13 | BIB management | 27 | Continuous improvement |
| 14 | Racepack pickup system | | |

## Documentation

Start with [`docs/README.md`](docs/README.md) for the full index. Highlights:

- **Product & plan:** [`prd.md`](docs/prd.md), [`masterplan.md`](docs/masterplan.md),
  [`struktur.md`](docs/struktur.md)
- **Core flows:** [`CHECKOUT_FLOW.md`](docs/CHECKOUT_FLOW.md),
  [`ORDER_FLOW.md`](docs/ORDER_FLOW.md), [`PAYMENT_FLOW.md`](docs/PAYMENT_FLOW.md),
  [`INVENTORY.md`](docs/INVENTORY.md), [`QUEUE_MODES.md`](docs/QUEUE_MODES.md)
- **Ops & launch:** [`LAUNCH_CHECKLIST.md`](docs/LAUNCH_CHECKLIST.md),
  [`INCIDENT_RUNBOOK.md`](docs/INCIDENT_RUNBOOK.md),
  [`SCALE_SPLIT.md`](docs/SCALE_SPLIT.md),
  [`POST_EVENT_REPORT.md`](docs/POST_EVENT_REPORT.md)
- **Integration:** [`ENTERPRISE_API.md`](docs/ENTERPRISE_API.md),
  [`WEBHOOK_PROCESSING.md`](docs/WEBHOOK_PROCESSING.md),
  [`GATEWAY_INTEGRATION.md`](docs/GATEWAY_INTEGRATION.md)
