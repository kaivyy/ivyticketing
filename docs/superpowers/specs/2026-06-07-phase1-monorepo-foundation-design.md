# Spec — Phase 1: Monorepo & Dev Foundation

Date: 2026-06-07
Status: Approved (design)
Scope: Phase 1 dari masterplan.md — fondasi monorepo

## Tujuan

Membangun fondasi monorepo yang **tipis tapi rantainya hidup**: membuktikan
`frontend (Astro) → API (Go) → Postgres + Redis` benar-benar terhubung dan
terbukti jalan di lokal. Belum ada logika bisnis apa pun.

Ini adalah langkah pertama dari masterplan. Semua fase berikutnya (Auth, Event,
Order, Payment, Queue, dst.) dibangun di atas fondasi ini, satu fase per siklus
brainstorm → spec → plan → kode.

## Keputusan yang sudah disepakati

| Topik | Keputusan |
|---|---|
| Cakupan | Tipis tapi nyambung: `apps/web` + `services/api` + Postgres + Redis. App lain (organizer-dashboard, admin-dashboard, scanner) & `services/worker` ditunda. |
| Run lokal | Postgres + Redis native via Homebrew. App Go & Astro jalan di host (bukan Docker). |
| Docker | Ditunda untuk staging/prod (fase lain). |
| DB access | sqlc (generate kode dari SQL) + goose (migration). Bukan ORM — correctness query adalah inti platform. |
| Router | Chi. |
| Bahasa backend | Go 1.25.x. |
| Frontend | Astro + TypeScript + Tailwind CSS. |

## Non-Goals (YAGNI untuk Fase 1)

- Tidak membuat `packages/`, `infra/` penuh, `tests/` e2e, atau 3 app frontend lain.
- Tidak ada auth, event, order, payment, queue, atau modul bisnis lain.
- Tidak ada Docker/docker-compose, k8s, terraform.
- Tidak ada e2e test (Playwright) atau load test (k6).
- Tidak ada secret (JWT, payment, R2) — baru muncul saat fasenya tiba.

## Struktur Direktori

```txt
ivyticketing/
├── apps/
│   └── web/                      # Astro + TypeScript + Tailwind
│       ├── src/
│       │   ├── layouts/PublicLayout.astro
│       │   ├── pages/index.astro       # panggil API, tampilkan status
│       │   ├── lib/api.ts              # fetch wrapper ke backend
│       │   └── styles/
│       ├── astro.config.mjs
│       ├── package.json
│       └── .env.example                # PUBLIC_API_URL
│
├── services/
│   └── api/                      # Go modular monolith
│       ├── cmd/api/main.go
│       ├── internal/
│       │   ├── app/              # bootstrap, config, server
│       │   ├── platform/
│       │   │   ├── database/     # koneksi Postgres (pgx)
│       │   │   ├── redis/        # koneksi Redis
│       │   │   ├── logger/       # structured logging
│       │   │   └── middleware/   # request ID, recovery
│       │   └── modules/
│       │       └── system/       # health & readiness handler
│       ├── sqlc.yaml
│       ├── go.mod
│       └── .env.example
│
├── database/
│   ├── migrations/               # goose: 000001_*.sql
│   └── queries/                  # sqlc: system.sql
│
├── scripts/dev/
│   ├── setup-local.sh
│   └── start-local.sh
│
├── Makefile
├── .gitignore
├── .env.example
└── README.md
```

Backend tetap memakai pola modul yang sama persis seperti `struktur.md`
(handler/service/repository/...), tapi Fase 1 hanya berisi modul `system`.

## Alur Data & Health Check

Inti Fase 1: membuktikan rantai `frontend → API → Postgres + Redis` hidup.

### Endpoint API (modul `system`)

- `GET /healthz` — **liveness**. Balas `200 {"status":"ok"}` tanpa cek dependensi.
  Menandakan proses API hidup.
- `GET /readyz` — **readiness**. Ping Postgres + ping Redis. Kedua sehat → `200`;
  ada yang mati → `503` dengan detail komponen.

```json
// GET /readyz — sehat (HTTP 200)
{ "status": "ready", "checks": { "postgres": "ok", "redis": "ok" } }

// GET /readyz — Postgres mati (HTTP 503)
{ "status": "not_ready", "checks": { "postgres": "down", "redis": "ok" } }
```

Pemisahan liveness vs readiness mengikuti standar k8s/monitoring (Prometheus/
Grafana di docs). Sekarang untuk dev, tapi polanya benar dari awal.

### Alur frontend

Halaman `index.astro` saat dimuat memanggil `/readyz` lewat `lib/api.ts`, lalu
menampilkan status tiap komponen (Postgres ✅/❌, Redis ✅/❌). Bukti visual rantai
nyambung: matikan Postgres → halaman langsung menunjukkannya.

### Query database

Lewat sqlc — satu query sederhana (`SELECT 1` atau `SELECT version()`) untuk
membuktikan koneksi & generate kode jalan. Migration goose pertama membuat tabel
sepele untuk membuktikan alur migrate berjalan.

## Config & Environment

`internal/app/config.go` membaca dari environment variables (di-load dari `.env`
saat dev). Tiap var punya default aman; var wajib yang kosong → API gagal start
dengan pesan jelas (bukan panic mentah), sesuai aturan docs "no raw error".

```bash
# .env (root) — services/api
APP_ENV=local
APP_NAME=ivyticketing
API_PORT=8080
DATABASE_URL=postgres://localhost:5432/ivyticketing?sslmode=disable
REDIS_URL=redis://localhost:6379

# apps/web/.env — Astro (prefix PUBLIC_ wajib agar terbaca browser)
PUBLIC_API_URL=http://localhost:8080
```

Aturan:
- `.env.example` di-commit (template tanpa nilai rahasia); `.env` masuk `.gitignore`.
- Hanya var yang dipakai Fase 1. Secret lain menyusul saat fasenya tiba.
- **CORS**: API mengizinkan origin Astro (`localhost:4321`) untuk dev; diperketat nanti.

## Developer Workflow

### Script (`scripts/dev/`)

`setup-local.sh` (sekali jalan):
- cek/`brew install postgresql@16 redis` jika belum ada
- `brew services start postgresql@16 && brew services start redis`
- `createdb ivyticketing` (skip jika sudah ada)
- install tool Go: `goose` + `sqlc`
- jalankan `migrate-up`
- `pnpm install` di `apps/web`

### Makefile (target utama)

```make
setup        # scripts/dev/setup-local.sh
api          # cd services/api && go run ./cmd/api
web          # cd apps/web && pnpm dev
dev          # api + web paralel
migrate-up   # goose -dir database/migrations up
migrate-down # goose -dir database/migrations down
sqlc         # sqlc generate
test         # go test ./...
lint         # go vet ./...
fmt          # go fmt + prettier
```

### Alur dev baru

```
1. make setup     # pasang & nyalakan pg+redis, createdb, migrate, pnpm install
2. make dev       # api di :8080, web di :4321
3. buka localhost:4321 → lihat status Postgres ✅ Redis ✅
```

`make dev` menjalankan dua proses paralel via cara sederhana (`&` + `wait`).
Tool seperti overmind/foreman ditunda (YAGNI).

## Testing

- **Unit test Go** modul `system`: `/healthz` balas 200 + body benar (`httptest`,
  tanpa DB).
- **Test readiness dengan checker tiruan**: `/readyz` saat Postgres "down" balas
  503 + JSON benar. Menguji logika status, bukan koneksi asli.
- **Smoke test manual** (di README): `make dev`, `curl localhost:8080/readyz`,
  matikan Redis, lihat status berubah 503.

Tidak ada e2e/load test di fase ini (YAGNI; relevan saat queue/checkout, Fase 5+).

## Definition of Done

1. `git init` + commit awal; `.gitignore` benar (tak ada `.env`/`node_modules`/binary).
2. `make setup` jalan dari nol di mesin bersih (pg+redis nyala, db dibuat, migrate sukses).
3. `make migrate-up` & `migrate-down` jalan; `sqlc generate` tanpa error.
4. `make api` → `GET /healthz` 200, `GET /readyz` 200 saat sehat.
5. Matikan Redis → `/readyz` balas 503 dengan `redis: "down"`.
6. `make web` → `localhost:4321` menampilkan status Postgres & Redis (live dari API).
7. `go test ./...` hijau.
8. README berisi langkah setup dari nol + struktur project + arah fase berikutnya.
9. Tidak ada secret hardcoded; semua via env.

## Setelah Fase 1

Siklus berulang per fase berikutnya (masing-masing brainstorm → spec → plan → kode):
Fase 2 Auth/RBAC/multi-tenant → Fase 3 Event/Category → Fase 4 Form builder →
Fase 5 Inventory/Order → Fase 6 Payment → dst. sesuai masterplan.md.
