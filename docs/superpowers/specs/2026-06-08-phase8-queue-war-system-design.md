# Spec — Phase 8: Queue / War Ticket System (Registration Access Engine Part 1)

Date: 2026-06-08
Status: Draft (design)
Scope: Phase 8 dari masterplan.md — Queue/War (WAR_QUEUE + RANDOMIZED_QUEUE + HYBRID_QUEUE), Waiting Room, pause/resume, release rate. PLUS fondasi Registration Mode (shared untuk Phase 9-11).
Depends on: Phase 1-7 (foundation, auth/RBAC/multi-tenant, event/category, form builder, orders/inventory/checkout, payment, tickets) — semua PRODUCTION BASELINE.

## Prinsip: Extend, Don't Rewrite

Phase 1-7 adalah baseline produksi. Phase 8 **hanya menambah**. Dilarang: mengubah behavior/API/auth/order/inventory/payment/ticket flow Phase 1-7, refactor besar, rename module, pindah folder.

Phase 8 = modul `registration` BARU (mode foundation) + modul `queue` BARU + `platform/queue` (Redis adapter) BARU + migrasi BARU + worker job BARU + wiring tambahan di `server.go`/`cmd/worker` + halaman waiting room BARU di `apps/web`. Order checkout Phase 5 dipakai apa adanya: Phase 8 hanya menambah **gate admission di depan checkout**. NORMAL mode = perilaku checkout Phase 5 saat ini, identik.

## Tujuan

Membangun pondasi **Registration Access Engine** (menentukan siapa/kapan/bagaimana boleh registrasi) dan mengisinya dengan **queue/war modes**. Saat war, peserta masuk **waiting room**, mendapat **token antrean permanen** (refresh/reconnect/mobile-sleep safe), menunggu di antrean yang adil, lalu **di-release** secara bertahap (release rate) untuk mendapat **admission token** yang memberi hak checkout dalam **checkout window**. Admin bisa pause/resume dan ubah release rate. Inventory lock Phase 5 tetap backstop oversold.

**Belum ada (Phase 8):** anti-bot/Turnstile penuh (hook stub saja, Phase 9), ballot (Phase 10), invitation/priority/community/corporate (Phase 11), waitlist-only mode (Phase 11). Websocket realtime (polling cukup; WS Phase 20+).

## Keputusan yang sudah disepakati

| # | Topik | Keputusan |
|---|---|---|
| Q1 | Queue state store | **Redis + Postgres hybrid.** Redis = posisi/state real-time (skala 100k). Postgres = durable token issuance + audit (sumber kebenaran). Reconcile via worker; Postgres menang saat konflik. |
| Q2 | Phase 8 sequencing | **Foundation-first, 4 part.** Part 1 = registration mode foundation (shared 8-11, NORMAL only, regression-safe). Lalu queue core → release engine → randomized/hybrid. |
| Q3 | Mode coverage | **3 queue modes penuh:** WAR_QUEUE + RANDOMIZED_QUEUE + HYBRID_QUEUE. |
| Q4 | Anti-bot | **Hook stub di Phase 8** (no-op middleware di queue join), implementasi penuh Phase 9. Queue tak perlu rework saat P9 masuk. |
| Q5 | Queue scope | **Per-event.** Satu token antrean per (user, event). UNIQUE(event_id, participant_id). Masuk antrean event sekali, boleh checkout kategori mana pun saat di-release. Sesuai masterplan "user tidak bisa punya banyak antrean untuk event sama". |
| Q6 | Hak checkout setelah release | **Admission token via header `X-Queue-Token`.** Checkout WAR mode wajib admission ACTIVE valid. Reuse endpoint checkout Phase 5 apa adanya (extend, bukan endpoint baru). |
| Q7 | Frontend Phase 8 | **Backend + waiting room frontend** (Astro di apps/web) + organizer pause/resume control. |
| Q8 | Tampilan waiting room | **Posisi absolut + estimasi waktu kasar** (posisi ÷ release_rate, best-effort). |
| Q9 | Release rate basis | **Rate murni (user/interval)**, terlepas dari stok. Inventory lock Phase 5 = backstop kalau stok habis. Admin kendalikan beban server. |
| Q10 | Admission expired | **Expired → kembali ke antrean belakang** (score baru). Slot release berikutnya diisi user lain. Worker job kelola expiry. |

## Non-Goals (YAGNI Phase 8)

- Tidak ada Turnstile/WAF/rate-limit/IP-reputation/fingerprint penuh (Phase 9 — hanya hook stub di join).
- Tidak ada ballot/lottery (Phase 10).
- Tidak ada invitation/priority/community/corporate access (Phase 11).
- Tidak ada WAITLIST_ONLY mode aktif (Phase 11 — enum disiapkan, gate fail-closed).
- Tidak ada websocket realtime (polling 3-5s cukup; WS Phase 20+).
- Tidak mengubah inventory lock, order state machine, payment, atau ticket flow.
- Tidak ada release berbasis stok (rate murni; inventory backstop).
- Tidak ada queue-service terpisah (modular monolith; service split = Phase 25).

## Arsitektur & Seam Admission

Phase 8 = lapisan admission di depan `orders.Checkout`. Dua modul baru + Redis adapter.

```
POST /checkout → RegistrationGate.Admit(ctx, participant, event, cat, admissionToken, now) → [lolos] → orders.Checkout (Phase 5, UNCHANGED)
                       │
                resolveMode(eventSettings, categorySettings)
                ├─ NORMAL         → window check (perilaku checkoutEligible Phase 5 saat ini)
                ├─ WAR/RAND/HYB   → wajib admission ACTIVE valid + milik caller + belum expired
                ├─ CLOSED         → selalu tolak (REGISTRATION_CLOSED)
                └─ BALLOT/INVITATION_ONLY/PRIORITY_ACCESS/WAITLIST_ONLY → REGISTRATION_MODE_NOT_AVAILABLE (fail-closed; Phase 10-11)
```

- `RegistrationGate` interface **dideklarasikan di `orders`**, diimplementasikan oleh modul `registration` (dependency inversion — pola identik `TicketIssuer` Phase 7 & `AuditRecorder` Phase 6). Orders **tidak import** `queue`/`registration` konkret.
- `checkoutEligible` di `orders/validator.go` diperluas memanggil gate. NORMAL path = identik perilaku sekarang → **regresi Phase 5 aman**.
- `registration` gate untuk WAR modes memanggil `queue` (cek admission). `registration` import `queue`; `orders` tidak.
- Sumber kebenaran: **Postgres** (token, admission, control, audit). **Redis** = cache posisi/state real-time. Worker reconcile; saat konflik Postgres menang.

## Modul & Struktur Go

```
services/api/internal/modules/registration/      ← BARU (foundation, shared 8-11)
├── model.go         RegistrationMode enum + konstanta
├── resolver.go      resolveMode(eventSettings, catSettings) → mode (pure fn)
├── gate.go          Gate implementing orders.RegistrationGate; NORMAL/CLOSED + delegasi queue
├── service.go       get/set registration settings (event + category)
├── repository.go    sqlc: event_registration_settings, category_registration_settings
├── handler.go       organizer get/set registration mode
├── routes.go        registration.manage routes
├── dto.go, errors.go
└── tests/

services/api/internal/modules/queue/             ← BARU
├── model.go         QueueStatus, Pool, AdmissionStatus, ControlState enums
├── token.go         issue/get token (idempotent per event+user), reconnect-safe
├── service.go       join, status, admission validate/consume (no HTTP)
├── release.go       release engine core: ReleaseJob(eventID, n) — dipakai worker
├── admission.go     admission expiry handling (expired → back to WAITING)
├── score.go         scoring: FIFO timestamp / seeded-random (reproducible)
├── control.go       pause/resume/set-rate
├── repository.go    sqlc: queue_tokens, queue_admissions, queue_control
├── store.go         Redis ops via platform/queue (write-through)
├── handler.go       participant join/status + organizer pause/resume/rate/stats
├── routes.go        participant + organizer (queue.manage) routes
├── guard.go         queueEntryGuard middleware STUB (no-op; Phase 9 fills)
├── dto.go, errors.go
└── tests/           token, release, admission, score (reproducible), concurrency

services/api/internal/platform/queue/            ← BARU (Redis sorted-set adapter)
├── queue.go         Add, Rank, RangeN, Move(waiting→allowed), Remove, Count primitives
└── queue_test.go

services/api/cmd/worker/main.go                  ← MODIFY: tambah job queue_release + admission_expiry
```

## Registration Mode Foundation (Part 1)

Enum `registration_mode` (CHECK constraint, bukan tipe PG native — konsisten pola status existing):
`NORMAL | WAR_QUEUE | RANDOMIZED_QUEUE | HYBRID_QUEUE | BALLOT | INVITATION_ONLY | PRIORITY_ACCESS | WAITLIST_ONLY | CLOSED`

Resolver (pure):
```
resolveMode(eventSettings, categorySettings):
  if categorySettings.override_enabled AND categorySettings.registration_mode != null:
      return categorySettings.registration_mode
  return eventSettings.default_mode    // default 'NORMAL' bila row tak ada
```
Tidak ada settings row → NORMAL → event/kategori lama berperilaku identik (regresi aman).

## Model Data (migrasi goose, lanjut dari 00019 Phase 7)

Nomor migrasi final menyesuaikan migrasi terakhir saat implementasi.

```
event_registration_settings                ← migrasi (create_registration_settings)
├─ event_id (uuid, pk, fk → events ON DELETE CASCADE)
├─ default_mode (text, not null, default 'NORMAL')
├─ queue_enabled (bool, not null, default false)
├─ ballot_enabled (bool, not null, default false)
├─ priority_enabled (bool, not null, default false)
├─ waitlist_enabled (bool, not null, default false)
├─ created_at, updated_at (timestamptz, not null, default now())
CHECK (default_mode IN ('NORMAL','WAR_QUEUE','RANDOMIZED_QUEUE','HYBRID_QUEUE','BALLOT','INVITATION_ONLY','PRIORITY_ACCESS','WAITLIST_ONLY','CLOSED'))

category_registration_settings             ← migrasi (sama atau terpisah)
├─ category_id (uuid, pk, fk → event_categories ON DELETE CASCADE)
├─ registration_mode (text, nullable)      ← null = inherit event
├─ override_enabled (bool, not null, default false)
├─ created_at, updated_at
CHECK (registration_mode IS NULL OR registration_mode IN (...sama enum...))

queue_tokens                               ← migrasi (create_queue_tokens)
├─ id (uuid, pk, default gen_random_uuid())
├─ organization_id (uuid, fk → organizations ON DELETE CASCADE)
├─ event_id (uuid, fk → events ON DELETE CASCADE)
├─ participant_id (uuid, fk → users ON DELETE RESTRICT)
├─ status (text, not null, default 'WAITING')   ← WAITING|ALLOWED|EXPIRED|COMPLETED|BLOCKED
├─ pool (text, not null, default 'FIFO')         ← PRESALE|FIFO
├─ score (bigint, not null)                      ← ordering (FIFO=unix nano, PRESALE=seeded random)
├─ joined_at (timestamptz, not null, default now())
├─ allowed_at, expired_at, completed_at (timestamptz, nullable)
├─ created_at, updated_at (timestamptz, not null, default now())
CHECK (status IN ('WAITING','ALLOWED','EXPIRED','COMPLETED','BLOCKED'))
CHECK (pool IN ('PRESALE','FIFO'))
UNIQUE (event_id, participant_id)                ← 1 antrean/user/event (non-negotiable masterplan)
INDEX (event_id, status)
INDEX (event_id, pool, score)

queue_admissions                           ← migrasi (create_queue_admissions)
├─ id (uuid, pk, default gen_random_uuid())
├─ token_id (uuid, fk → queue_tokens ON DELETE CASCADE)
├─ event_id (uuid, fk → events ON DELETE CASCADE)
├─ participant_id (uuid, fk → users ON DELETE RESTRICT)
├─ checkout_expires_at (timestamptz, not null)
├─ status (text, not null, default 'ACTIVE')     ← ACTIVE|CONSUMED|EXPIRED
├─ created_at (timestamptz, not null, default now())
CHECK (status IN ('ACTIVE','CONSUMED','EXPIRED'))
INDEX (event_id, status, checkout_expires_at)
-- satu admission ACTIVE per token:
UNIQUE INDEX uq_admission_active ON queue_admissions(token_id) WHERE status = 'ACTIVE'

queue_control                              ← migrasi (create_queue_control)
├─ event_id (uuid, pk, fk → events ON DELETE CASCADE)
├─ state (text, not null, default 'RUNNING')     ← RUNNING|PAUSED
├─ release_rate (int, not null, default 100)     ← user per interval
├─ randomization_seed (text, nullable)           ← reproducible (randomized/hybrid)
├─ sale_start_at (timestamptz, nullable)
├─ presale_pool_open_at (timestamptz, nullable)
├─ updated_at (timestamptz, not null, default now())
CHECK (state IN ('RUNNING','PAUSED'))
CHECK (release_rate >= 0)
```

Redis (ephemeral, key per event):
- `queue:{event_id}:waiting` — sorted set (member=participant_id, score). Posisi via ZRANK, release via ZRANGE.
- `queue:{event_id}:allowed` — sorted set (member=participant_id, score=checkout_expires_at unix).
- `queue:{event_id}:control` — hash cache state/rate (write-through dari Postgres).

Reconcile: Postgres otoritas. Redis di-rebuild dari `queue_tokens WHERE status='WAITING'` saat startup/recovery (Risk: Redis down → posisi tidak hilang permanen). Status final & token issuance ditulis Postgres dulu, Redis menyusul.

## Permissions (migrasi, idempotent)

Tambah (pola seed Phase 2/5/6/7):
- `registration.manage` — "Manage registration mode & settings"
- `queue.manage` — "Pause/resume queue & set release rate"
Assign ke role template Owner & Manager (org_id NULL, is_system).

## Alur

### Join
```
POST /api/v1/events/{eventId}/queue/join  (access token)
  → queueEntryGuard stub (no-op Phase 8)
  → resolveMode != WAR/RAND/HYBRID → QUEUE_NOT_ENABLED
  → existing token (UNIQUE event+user)? return token + posisi (IDEMPOTEN; refresh/reconnect safe)
  → pool + score:
      WAR_QUEUE:        pool=FIFO,    score=unix_nano(now)
      RANDOMIZED/HYBRID: now < sale_start → pool=PRESALE, score=seeded_random(seed, participant_id)
                         now >= sale_start → pool=FIFO,   score=unix_nano(now)  (di belakang presale)
  → INSERT queue_tokens (Postgres) → ZADD Redis → return {token, posisi, status}
```

### Status
```
GET /api/v1/events/{eventId}/queue/status  (access token; token milik caller)
  → posisi absolut (ZRANK; PRESALE diurut sebelum FIFO), estimasi = ceil(posisi / release_rate) interval
  → status token + state sistem (RUNNING|PAUSED)
  → status=ALLOWED → sertakan admission token (id) + sisa checkout window
```

### Release engine (worker `queue_release`, pola worker.Runner)
```
tiap QUEUE_RELEASE_INTERVAL, untuk tiap event dengan queue aktif & state=RUNNING:
  n = release_rate
  ambil N teratas WAITING (Redis ZRANGE; PRESALE dulu lalu FIFO by score)
  per user (transaksi Postgres):
    UPDATE queue_tokens SET status=ALLOWED, allowed_at=now WHERE id=.. AND status='WAITING'
    INSERT queue_admissions (token_id, checkout_expires_at=now+QUEUE_CHECKOUT_WINDOW, status='ACTIVE')
    Redis: ZREM waiting, ZADD allowed
  state=PAUSED → skip event (tidak release)
```

### Admission expiry (worker `queue_admission_expiry`)
```
admission ACTIVE dengan checkout_expires_at < now (FOR UPDATE SKIP LOCKED):
  UPDATE queue_admissions SET status='EXPIRED'
  UPDATE queue_tokens SET status='WAITING', score=unix_nano(now)  ← kembali ke belakang antrean
  Redis: ZREM allowed, ZADD waiting (score baru)
```

### Checkout (seam Phase 5)
```
POST /api/v1/organizations/.../checkout + header X-Queue-Token  (access token)
  → orders.Checkout memanggil RegistrationGate.Admit
  → mode WAR/RAND/HYBRID: butuh admission ACTIVE, belum expired, token milik caller, event cocok
       gagal → ADMISSION_REQUIRED / ADMISSION_EXPIRED
  → lolos → orders.Checkout Phase 5 (UNCHANGED) berjalan
  → checkout sukses: admission → CONSUMED, queue_token → COMPLETED (transaksi yang sama bila memungkinkan; else write-through pasca-commit dengan reconcile)
  → inventory lock Phase 5 = backstop oversold (stok habis walau ALLOWED → checkout gagal normal)
```

## Endpoint

```
# Peserta (access token)
POST /api/v1/events/{eventId}/queue/join          → join (idempoten) → token + posisi
GET  /api/v1/events/{eventId}/queue/status         → posisi, estimasi, status, state, admission (jika ALLOWED)
POST /api/v1/organizations/{orgId}/events/{eventId}/categories/{categoryId}/checkout  → Phase 5 + header X-Queue-Token

# Organizer (registration.manage)
GET  /api/v1/organizations/{orgId}/events/{eventId}/registration   → baca settings (event+category)
PUT  /api/v1/organizations/{orgId}/events/{eventId}/registration   → set mode + settings

# Organizer (queue.manage)
POST /api/v1/organizations/{orgId}/events/{eventId}/queue/pause
POST /api/v1/organizations/{orgId}/events/{eventId}/queue/resume
PUT  /api/v1/organizations/{orgId}/events/{eventId}/queue/release-rate
GET  /api/v1/organizations/{orgId}/events/{eventId}/queue/stats     → waiting/allowed count, rate, state
```

Otorisasi & isolasi:
- Token milik caller (`token.participant_id = caller`); token lain → 404.
- Webhook port 8090 (Phase 6) TIDAK tersentuh queue/rate-limit — terisolasi proses & port.
- Endpoint organizer: `RequirePermission(loader, "registration.manage"|"queue.manage")`; super admin lolos.

## Anti-Bot Hook Stub (Phase 9 placeholder)

`queue.guard.queueEntryGuard` = chi middleware no-op di Phase 8 (langsung `next`). Dipasang di route join. Phase 9 mengisi: Turnstile verify, rate-limit (Redis token bucket), duplicate detection, abuse flag. Queue tidak perlu rework.

## Error Codes (tambahan, envelope Phase 2)

`QUEUE_NOT_ENABLED`, `QUEUE_TOKEN_NOT_FOUND`, `QUEUE_NOT_ALLOWED` (belum di-release), `ADMISSION_REQUIRED` (checkout WAR tanpa admission valid), `ADMISSION_EXPIRED`, `REGISTRATION_MODE_NOT_AVAILABLE` (mode Phase 10-11), `QUEUE_PAUSED` (info, bukan error keras). Pesan manusiawi (masterplan: "Sistem sedang padat. Posisi antrean kamu tetap aman."), bukan stack trace.

## Audit Log (Phase 2 logger)

`QUEUE_MODE_CHANGED`, `QUEUE_PAUSED`, `QUEUE_RESUMED`, `QUEUE_RATE_CHANGED`, `QUEUE_TOKEN_ISSUED`, `QUEUE_RELEASED` (batch dengan count), `QUEUE_ADMISSION_EXPIRED`.

## Frontend (apps/web)

Reuse fondasi auth participant Phase 7 (sessionStorage token, authedFetch, ParticipantLayout).

```
apps/web/src/
├─ pages/events/[slug]/queue.astro     → waiting room (prerender=false)
├─ lib/queue.ts                        → joinQueue(), getQueueStatus() via authedFetch
└─ components/queue/
   ├─ WaitingRoom.astro                → countdown, posisi, estimasi, status sistem, banner PAUSED
   └─ QueueStatus.astro                → badge status
```

Perilaku (masterplan non-negotiable):
- Auto-poll status 3-5s. Refresh-safe & reconnect-safe (posisi dari token server-side; join idempoten). Mobile-sleep-safe (`visibilitychange` → poll ulang).
- ALLOWED → tombol "Lanjut Checkout" + sisa window; arahkan ke checkout dengan admission token.
- EXPIRED → "waktu checkout habis, kamu kembali ke antrean" + posisi baru.
- PAUSED → banner "antrean dijeda, posisimu aman".
- Organizer dashboard: pause/resume, set release rate, stats (waiting/allowed).

## Config / Env Baru

```
QUEUE_RELEASE_INTERVAL=10s        # tick worker queue_release
QUEUE_DEFAULT_RELEASE_RATE=100    # user per interval default
QUEUE_CHECKOUT_WINDOW=5m          # TTL admission (≤ ORDER_EXPIRATION Phase 5)
# REDIS_URL sudah ada (Phase 1) — Phase 8 mengaktifkan pemakaiannya (sebelumnya hanya health ping)
```
`QUEUE_CHECKOUT_WINDOW` di-clamp agar ≤ sisa waktu order expiry Phase 5.

## Testing

**Unit (tanpa DB/Redis nyata, fake repo/store):**
- resolver: matrix override (event-only, category-override, no-settings→NORMAL).
- gate: admit/deny per mode; NORMAL window check; CLOSED tolak; Phase 10-11 modes → NOT_AVAILABLE.
- score: FIFO ordering, seeded-random reproducible (seed sama → urutan sama), PRESALE sebelum FIFO.
- release math, estimasi posisi.

**Integration (Postgres `ivyticketing_test`, truncate per test):**
- NORMAL checkout regresi (Phase 5 tetap hijau — KRITIS).
- WAR full: join → status → release → admission → checkout (+X-Queue-Token) → PAID → token COMPLETED.
- join idempoten (2x = 1 token, posisi sama); duplicate ditolak (UNIQUE).
- pause hentikan release; resume lanjut.
- admission expired → token kembali WAITING posisi belakang; slot diisi user lain.
- checkout WAR tanpa admission valid → ADMISSION_REQUIRED; expired → ADMISSION_EXPIRED.
- RANDOMIZED: presale pool seeded, reproducible; HYBRID: FIFO di belakang presale.
- organizer endpoints butuh registration.manage/queue.manage.

**Concurrency (WAJIB, `-race`):**
- N goroutine join event sama → posisi unik, tepat 1 token/user (UNIQUE), no race.
- N admission serbu checkout → no oversold (inventory backstop Phase 5), tidak double-consume.
- release idempoten (2 tick paralel tidak double-allow user yang sama).

**Load (k6, `tests/load/`):** waiting room 10k / 50k / 100k. Target: no queue reset, posisi konsisten, no oversold. Hasil & batasan environment dicatat eksplisit (best-effort sesuai mesin).

## Documentation (`docs/`)

- `REGISTRATION_MODES.md` — enum, resolver, override event/category, gate, fail-closed Phase 10-11.
- `QUEUE_MODES.md` — WAR/RANDOMIZED/HYBRID, sequence diagram teks, state machine token+admission, release engine, scoring/seed.
- `QUEUE_OPERATIONS.md` — pause/resume/rate, reconcile Redis↔Postgres, Redis-down recovery, war-day notes.
- `PHASE8_DECISIONS.md` — 10 keputusan (Q1-Q10) + tradeoff.
- Update CHANGELOG.md.

## Definition of Done

1. Migrasi roundtrip (up/down): event/category registration settings, queue_tokens, queue_admissions, queue_control; seed `registration.manage` + `queue.manage`.
2. Mode resolver + override; **NORMAL = perilaku checkout Phase 5 identik** (regresi integration hijau).
3. Join idempoten; refresh/reconnect/mobile-sleep safe; UNIQUE 1 antrean/user/event.
4. Release engine (rate murni, user/interval) + pause/resume + set release rate (queue.manage).
5. Admission token gate checkout (WAR/RAND/HYBRID) via X-Queue-Token; expired → kembali ke antrean belakang.
6. **No oversold** (inventory backstop) + **no queue reset** (refresh/reconnect) + **no duplicate token** — concurrency `-race` hijau.
7. RANDOMIZED + HYBRID: presale pool seeded reproducible + fairness auditable (seed tersimpan).
8. Anti-bot hook stub terpasang di join (no-op, siap Phase 9).
9. Frontend waiting room diverifikasi di browser (join → release → ALLOWED → checkout); organizer pause/resume.
10. Audit lengkap (mode change, pause/resume, rate, token issued, released, admission expired); error manusiawi; webhook port tak tersentuh.
11. `go test ./... -race` (unit + integration + concurrency) hijau; `sqlc generate` & `go vet` bersih.
12. Tidak ada perubahan behavior/API Phase 1-7 (extend-only). Docs + CHANGELOG diperbarui.

## Setelah Phase 8

Phase 9 — Anti-Bot: mengisi `queueEntryGuard` stub (Turnstile, rate-limit, duplicate detection, abuse flag, block/unblock). Reuse status BLOCKED di queue_tokens.
Phase 10 — Ballot: reuse registration foundation + worker pattern; mode BALLOT.
Phase 11 — Invitation/Priority/Community/Corporate + WAITLIST_ONLY: semua = admission gate variants di registration foundation.
