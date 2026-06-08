# Spec — Phase 9: Anti-Bot & Abuse Protection

Date: 2026-06-08
Status: Draft (design)
Scope: Phase 9 dari masterplan.md — Anti-bot & abuse protection (Turnstile, rate limit, IP reputation, duplicate/abuse detection, block/unblock, max queue entry). All in application code.
Depends on: Phase 1-8 (foundation, auth/RBAC, events, forms, orders/inventory, payment, tickets, queue/war) — all PRODUCTION BASELINE.

## Prinsip: Extend, Don't Rewrite

Phase 1-8 adalah baseline produksi. Phase 9 **hanya menambah**. Dilarang: mengubah behavior/API/auth/order/payment/ticket/queue flow Phase 1-8, refactor besar, rename module, pindah folder.

Phase 9 = modul `abuse` BARU + `platform/ratelimit` BARU + `platform/captcha` BARU + `RequirePlatformAdmin` middleware BARU + migrasi BARU + wiring middleware di `server.go` + komponen frontend Turnstile. Mengisi `EntryGuard` stub Phase 8 (no-op → real guard) tanpa mengubah queue logic. Webhook port 8090 TIDAK tersentuh (acceptance: rate limit tak merusak payment callback).

## Tujuan

Mengurangi bot tanpa menyiksa user normal. Saat war/registrasi, request masuk melewati rantai proteksi: blocklist → rate limit (per-IP + per-user, per-kategori endpoint) → Turnstile (CAPTCHA, pada entry sensitif) → IP reputation gate. Perilaku abusive (rate violation, duplicate attempt, blocked hits) menaikkan skor reputasi internal; super admin dapat block/unblock user/IP dan melihat abuse log. **Semua fitur dapat di-on/off oleh super admin secara runtime** lewat `platform_settings` (tanpa restart). Batas: max N antrean aktif per user lintas-event (di atas UNIQUE per-event Phase 8).

## Keputusan yang sudah disepakati

| Topik | Keputusan |
|---|---|
| Scope | Semua di kode aplikasi (Turnstile, rate limit, IP reputation, duplicate detection, block/unblock). Cloudflare WAF = catatan deployment di docs, tidak di-emulasi di Go. |
| Turnstile | Interface `CaptchaVerifier` + adapter Cloudflare Turnstile nyata (siteverify, `TURNSTILE_SECRET`). Fakeable di test. Frontend render widget. On/off via platform_settings. |
| IP reputation | **Behavior-based score internal** (dihitung dari rate violation / duplicate / blocked count, disimpan) **+ manual allow/deny list** (admin-managed CIDR/IP/user). |
| Rate limit | **Per-IP + per-user, per-kategori endpoint** (queue_join, checkout, auth_login, auth_register, default). Redis token bucket. Webhook 8090 dikecualikan. |
| Block/unblock | Tabel `blocked_subjects` (user/IP) + admin endpoint (super admin) + tabel `abuse_log`. Reuse status `BLOCKED` di queue token saat user diblok. |
| Toggle | Tabel `platform_settings` (key/value), super admin ubah runtime via endpoint; cache in-memory refresh berkala (30s) → on/off tanpa restart. |
| Max queue entry | Cross-event active queue cap per user (config `MAX_ACTIVE_QUEUE_PER_USER`, default mis. 5). Di atas UNIQUE(event,participant) Phase 8. |
| Enforcement | chi middleware chain (`abuse.Guard`) dipasang di `server.go` di depan entry sensitif (queue join, auth login/register, checkout). Mengganti `EntryGuard` no-op Phase 8. |
| Fingerprint | Server-side hash ringan `sha256(UA + client_ip + Accept-Language)` sebagai signal abuse log/duplicate. Tanpa client JS fingerprinting. |

## Non-Goals (YAGNI Phase 9)

- Tidak meng-emulasi Cloudflare WAF rules di Go (didokumentasikan sebagai konfigurasi edge).
- Tidak ada feed IP reputation eksternal pihak ketiga (skor murni dari perilaku teramati + manual list).
- Tidak ada client-side JS device fingerprint (canvas/webgl). Hanya hash server-side ringan.
- Tidak mengubah queue/order/payment logic — hanya menambah guard di entry.
- Tidak ada ML/anomaly detection — aturan deterministik (threshold).
- Tidak menyentuh webhook port 8090.
- Tidak ada email/SMS notifikasi abuse (Phase 12 notification).

## Arsitektur & Enforcement

Phase 9 = modul `abuse` + dua platform package + middleware chain.

```
request (API :8080) → abuse.Guard(category) middleware chain → handler
  1. Blocklist check       → blocked subject (user/IP/CIDR) → 403 USER_BLOCKED + abuse_log
  2. Rate limit            → token bucket per (IP, user, category) exceeded → 429 RATE_LIMITED + reputation++
  3. Turnstile verify      → (category needs captcha) header token invalid → 403 CAPTCHA_REQUIRED + abuse_log
  4. Reputation gate       → score ≥ deny threshold → 403 / ≥ challenge threshold → require captcha
  each step gated by platform_settings toggle (super admin on/off runtime); disabled step = pass-through
webhook (:8090) → NO abuse middleware (isolated)
```

Coupling: middleware dipasang di `server.go`, bukan di dalam modul queue/orders. `queue.EntryGuard` (no-op Phase 8) **dihapus dari route registration**; `server.go` membungkus queue-join route dengan `abuseGuard.Middleware("queue_join")`. Queue/orders tidak import abuse.

## Modul & Struktur Go

```
services/api/internal/modules/abuse/                ← BARU
├── model.go         enums (subject type, category, abuse action), thresholds
├── settings.go      PlatformSettings cache (load from DB, in-memory, refresh 30s), Get(key)/IsEnabled(feature)
├── settings_test.go
├── blocklist.go     IsBlocked(ctx, userID, ip), Block/Unblock (admin)
├── reputation.go    Score(ctx, ip|user), Bump(ctx, subject, delta, reason); deny/challenge thresholds
├── ratelimit.go     wraps platform/ratelimit; per-category limits config
├── guard.go         Guard struct; Middleware(category) http.Handler chain (blocklist→ratelimit→captcha→reputation)
├── fingerprint.go   Hash(r *http.Request) string  (sha256 UA+IP+Accept-Language)
├── clientip.go      ClientIP(r) string  (X-Forwarded-For first hop / RemoteAddr)
├── service.go       admin ops: block, unblock, list abuse log, set platform setting, list/set allow-deny, queue-entry cap check
├── repository.go    sqlc: platform_settings, blocked_subjects, abuse_log, ip_rules, (read queue_tokens for cap)
├── handler.go       super-admin endpoints (settings, block/unblock, abuse log, ip rules)
├── routes.go        RegisterAdminRoutes (RequirePlatformAdmin)
├── dto.go, errors.go
└── tests/           guard, ratelimit, blocklist, reputation, settings-toggle, captcha-fake

services/api/internal/platform/ratelimit/           ← BARU
├── ratelimit.go     Redis token-bucket limiter: Allow(ctx, key, limit, window) (bool, error)
└── ratelimit_test.go

services/api/internal/platform/captcha/              ← BARU
├── captcha.go       Verifier interface; Verify(ctx, token, remoteIP) (bool, error)
├── turnstile.go     Cloudflare Turnstile adapter (siteverify HTTP call)
├── fake.go          FakeVerifier (always pass/fail) for tests/dev
└── captcha_test.go

services/api/internal/platform/middleware/
└── platformadmin.go  ← BARU: RequirePlatformAdmin (403 if !IsPlatformAdmin)
```

## Model Data (migrasi goose, lanjut dari 00025 Phase 8)

```
platform_settings                       ← migrasi (create_platform_settings)
├─ key (text, pk)                       ← 'turnstile_enabled','rate_limit_enabled','ip_reputation_enabled','blocklist_enabled'
├─ value (text, not null)               ← 'true'/'false' atau angka threshold
├─ updated_by (uuid, nullable, fk users)
├─ updated_at (timestamptz, not null, default now())
-- seed default rows (semua fitur 'true' atau sesuai default aman)

blocked_subjects                        ← migrasi (create_blocked_subjects)
├─ id (uuid, pk, default gen_random_uuid())
├─ subject_type (text, not null)        ← 'user' | 'ip'
├─ subject_value (text, not null)       ← user_id (uuid string) | ip address
├─ reason (text, nullable)
├─ blocked_by (uuid, nullable, fk users)
├─ created_at (timestamptz, not null, default now())
├─ expires_at (timestamptz, nullable)   ← null = permanen
UNIQUE (subject_type, subject_value)
INDEX idx_blocked_subjects_lookup (subject_type, subject_value)

ip_rules                                ← migrasi (create_ip_rules) — manual allow/deny list
├─ id (uuid, pk, default gen_random_uuid())
├─ cidr (text, not null)                ← IP atau CIDR (mis. '203.0.113.0/24')
├─ rule (text, not null)                ← 'allow' | 'deny'
├─ note (text, nullable)
├─ created_by (uuid, nullable, fk users)
├─ created_at (timestamptz, not null, default now())
UNIQUE (cidr, rule)

abuse_log                               ← migrasi (create_abuse_log)
├─ id (uuid, pk, default gen_random_uuid())
├─ subject_type (text, nullable)        ← 'user'|'ip'
├─ subject_value (text, nullable)
├─ action (text, not null)              ← 'RATE_LIMITED'|'BLOCKED_HIT'|'CAPTCHA_FAIL'|'DUPLICATE_QUEUE'|'REPUTATION_DENY'|'BLOCK_SET'|'UNBLOCK'
├─ category (text, nullable)            ← endpoint kategori (queue_join,checkout,...)
├─ fingerprint (text, nullable)         ← hash ringan
├─ ip (text, nullable)
├─ user_id (uuid, nullable)
├─ detail (jsonb, nullable)
├─ created_at (timestamptz, not null, default now())
INDEX idx_abuse_log_created (created_at DESC)
INDEX idx_abuse_log_subject (subject_type, subject_value)

ip_reputation                           ← migrasi (create_ip_reputation) — behavior score
├─ subject_type (text, not null)        ← 'ip' | 'user'
├─ subject_value (text, not null)
├─ score (int, not null, default 0)     ← naik saat abuse; thresholds di platform_settings/config
├─ updated_at (timestamptz, not null, default now())
PRIMARY KEY (subject_type, subject_value)
```

Redis (rate limit, ephemeral): `ratelimit:{category}:{ip}` dan `ratelimit:{category}:user:{userID}` — token bucket counter dengan TTL = window. Tidak durable (rate limit boleh reset saat Redis restart).

## Platform Settings (runtime toggle)

`abuse.Settings` memuat semua baris `platform_settings` ke map in-memory saat start, refresh tiap 30s (goroutine ticker) DAN segera setelah super admin menulis (write-through). `IsEnabled(feature) bool` membaca cache. Default fail-safe: jika cache kosong/error → fitur dianggap **enabled** untuk proteksi (kecuali Turnstile yang default **disabled** agar tak memblok bila secret belum diset — aman untuk dev). Toggle keys:
- `turnstile_enabled` (default false), `rate_limit_enabled` (default true), `ip_reputation_enabled` (default true), `blocklist_enabled` (default true).

## Rate Limit

`platform/ratelimit`: Redis `INCR` + `EXPIRE` token bucket. `Allow(ctx, key, limit int, window time.Duration) (bool, error)`. Fail-open jika Redis error (jangan blok user normal karena infra; log saja).

Per-kategori limit (config, default values):
- `queue_join`: 10/menit per IP, 5/menit per user
- `checkout`: 20/menit per IP, 10/menit per user
- `auth_login`: 10/menit per IP, 5/menit per user
- `auth_register`: 5/menit per IP
- `default`: 120/menit per IP

Limit per-kategori disimpan sebagai konstanta config (bukan per-org). Webhook tak melewati middleware ini.

## IP Reputation

`reputation.Score(ctx, subjectType, subjectValue) int`. `Bump(ctx, subject, delta, reason)` dipanggil saat rate violation (+2), captcha fail (+3), blocked hit (+5), duplicate queue (+1). Thresholds (config):
- `REPUTATION_CHALLENGE_THRESHOLD` (default 10): paksa Turnstile meski kategori biasanya tak butuh.
- `REPUTATION_DENY_THRESHOLD` (default 25): tolak 403 + abuse_log REPUTATION_DENY.
Manual `ip_rules`: `allow` rule → bypass reputation/rate (mis. office IP); `deny` rule → 403 langsung. Allow menang atas deny bila tumpang tindih (paling spesifik diabaikan untuk MVP — allow-first).

## Block/Unblock & Max Queue Entry

- `blocklist.IsBlocked(ctx, userID, ip)` cek `blocked_subjects` (cache ringan opsional; query langsung untuk MVP) + `ip_rules` deny. Subjek terblok → 403 `USER_BLOCKED`, abuse_log `BLOCKED_HIT`. Bila user diblok saat punya queue token aktif → set token `BLOCKED` (reuse status Phase 8) via abuse service (best-effort).
- Admin (super admin): `POST /api/v1/admin/abuse/block` {subjectType, subjectValue, reason, expiresAt?}, `POST /api/v1/admin/abuse/unblock`, audit `BLOCK_SET`/`UNBLOCK`.
- **Max queue entry**: `abuse.Service.CheckQueueEntryCap(ctx, userID)` menghitung queue_tokens user dengan status WAITING/ALLOWED across all events; ≥ `MAX_ACTIVE_QUEUE_PER_USER` → tolak. Dipanggil oleh queue join lewat interface (queue mendeklarasikan `EntryCapChecker`, di-inject; nil = no cap) ATAU oleh abuse guard kategori queue_join (lebih bersih: guard, tapi butuh userID dari context — tersedia karena authn sudah jalan). **Keputusan: di abuse guard `queue_join`** (punya akses userID dari authctx + repo queue read-only). Hindari queue→abuse import dengan query langsung tabel queue_tokens via abuse repository.

## Endpoint (super admin)

```
# Super admin only (RequirePlatformAdmin)
GET  /api/v1/admin/abuse/settings              → list platform_settings
PUT  /api/v1/admin/abuse/settings              → set {key,value} (toggle on/off runtime)
POST /api/v1/admin/abuse/block                 → block {subjectType,subjectValue,reason,expiresAt?}
POST /api/v1/admin/abuse/unblock               → unblock {subjectType,subjectValue}
GET  /api/v1/admin/abuse/blocked               → list blocked subjects
GET  /api/v1/admin/abuse/log                   → list abuse log (paginated, recent first)
GET  /api/v1/admin/abuse/ip-rules              → list allow/deny rules
POST /api/v1/admin/abuse/ip-rules              → add {cidr,rule,note?}
DELETE /api/v1/admin/abuse/ip-rules/{id}       → remove rule
```

`RequirePlatformAdmin` middleware: `authctx.FromContext` → jika `!IsPlatformAdmin` → 403 `FORBIDDEN`. Reuse di server.go untuk grup `/api/v1/admin/abuse`.

## Middleware enforcement (server.go wiring)

- Build `abuseGuard` (Settings + blocklist + ratelimit + captcha + reputation + repo).
- Queue join: ganti `r.With(EntryGuard)` → mount via server.go dengan `abuseGuard.Middleware("queue_join")`. Hapus `queue.EntryGuard` usage (boleh tinggalkan fungsi stub atau hapus; spec: hapus dari route, fungsi boleh dihapus).
- Auth login/register: bungkus dengan `abuseGuard.Middleware("auth_login"/"auth_register")`.
- Checkout: bungkus dengan `abuseGuard.Middleware("checkout")`.
- Guard membaca `clientIP(r)`, `userID` (dari authctx bila ada — auth login belum punya user, pakai IP saja), fingerprint.

## Config / Env Baru

```
TURNSTILE_SECRET=                         # Cloudflare Turnstile secret (siteverify)
TURNSTILE_SITE_KEY=                       # public, untuk frontend widget (PUBLIC_TURNSTILE_SITE_KEY di web)
MAX_ACTIVE_QUEUE_PER_USER=5               # cross-event active queue cap
REPUTATION_CHALLENGE_THRESHOLD=10
REPUTATION_DENY_THRESHOLD=25
ABUSE_SETTINGS_REFRESH=30s                # platform_settings cache refresh interval
```
Toggle aktual (turnstile/rate/reputation/blocklist on-off) di DB `platform_settings`, BUKAN env (runtime). Turnstile secret di env; jika `turnstile_enabled=true` tapi secret kosong → verify fail-open dengan warning log (jangan blok semua user). Tidak ada secret di-log.

## Error Codes (tambahan, envelope Phase 2)

`USER_BLOCKED` (403), `RATE_LIMITED` (429, sertakan Retry-After), `CAPTCHA_REQUIRED` (403), `CAPTCHA_INVALID` (403), `REPUTATION_DENIED` (403), `QUEUE_ENTRY_CAP_EXCEEDED` (429/409), `INVALID_SETTING` (400). Pesan manusiawi, bukan stack trace.

## Audit & Abuse Log

- `audit.Logger` (Phase 2): `ABUSE_BLOCK_SET`, `ABUSE_UNBLOCK`, `ABUSE_SETTING_CHANGED`, `ABUSE_IP_RULE_ADDED`, `ABUSE_IP_RULE_REMOVED` (actor = super admin).
- `abuse_log` (tabel baru): event otomatis dari guard (RATE_LIMITED, BLOCKED_HIT, CAPTCHA_FAIL, DUPLICATE_QUEUE, REPUTATION_DENY) — high-volume, terpisah dari audit_logs.

## Frontend (apps/web)

- `components/security/Turnstile.astro` — render Cloudflare Turnstile widget (script + sitekey dari `PUBLIC_TURNSTILE_SITE_KEY`), set hidden token ke form/sessionStorage.
- Waiting room join (`WaitingRoom.astro`) + login: kirim token via header `X-Turnstile-Token` ke endpoint terkait. Widget hanya muncul bila `turnstile_enabled` (frontend cek via endpoint publik ringan `GET /api/v1/security/config` → {turnstileEnabled, siteKey}).
- Tangani 429 (rate limited) & 403 (blocked/captcha) dengan pesan manusiawi.

## Testing

**Unit:**
- `platform/ratelimit`: Allow within limit / exceeds → false; window reset; fail-open on redis error (fake).
- `platform/captcha`: FakeVerifier pass/fail; turnstile adapter dengan HTTP fake (siteverify success/fail mapping).
- `abuse/settings`: cache load, IsEnabled toggle, refresh picks up change, fail-safe defaults.
- `abuse/blocklist`: IsBlocked true/false, expired block ignored, ip_rules deny/allow precedence.
- `abuse/reputation`: Bump increments, threshold challenge/deny boundaries.
- `abuse/guard`: chain order; each step skipped when toggle off; blocked→403; rate exceeded→429; captcha fail→403; reputation deny→403; clean request→pass. Fingerprint stable.
- `abuse/queue cap`: under cap pass, at/over cap reject.

**Integration (Postgres + Redis test):**
- Block user via admin endpoint → subsequent queue join → 403 USER_BLOCKED + abuse_log row.
- Toggle rate_limit off via settings → rapid requests pass; toggle on → 429 after limit.
- Turnstile enabled + fake verifier fail → join 403; pass → join 201.
- ip_rules deny CIDR → request from that IP 403; allow → bypass.
- Super admin only: non-admin hits /admin/abuse/* → 403.
- Max queue entry cap: user joins N events; (N+1)th → QUEUE_ENTRY_CAP_EXCEEDED.
- **Webhook untouched**: callback to :8090 path not rate-limited (verify guard not mounted there).

**Concurrency (`-race`):**
- Rate limiter under concurrent hits: exactly `limit` allowed within window (token bucket atomic via Redis INCR).
- Reputation Bump concurrent: no lost updates (atomic upsert).

## Definition of Done

1. Migrasi roundtrip: platform_settings (+seed), blocked_subjects, ip_rules, abuse_log, ip_reputation.
2. `platform/ratelimit` Redis token bucket; fail-open on error; per-category limits.
3. `platform/captcha` Verifier interface + Turnstile adapter + fake; fail-open if enabled-but-no-secret.
4. `abuse.Settings` runtime toggle from DB, refresh 30s + write-through; fail-safe defaults.
5. `abuse.Guard` middleware chain (blocklist→ratelimit→captcha→reputation), each step toggle-gated.
6. EntryGuard stub Phase 8 replaced by abuse guard on queue_join; auth login/register + checkout guarded.
7. Block/unblock + ip_rules + abuse log + reputation via super-admin endpoints (RequirePlatformAdmin).
8. Max active queue entry per user (cross-event) enforced.
9. Blocked user with active queue token → token set BLOCKED (reuse Phase 8 status).
10. Webhook port 8090 NOT rate-limited / NOT guarded (acceptance: rate limit tak merusak payment callback).
11. Frontend Turnstile widget (gated by turnstile_enabled via /security/config); 429/403 handled humanely.
12. Audit (block/unblock/setting/ip-rule) + abuse_log (auto guard events).
13. `go test ./... -race` (unit + integration + concurrency) hijau; sqlc/vet bersih.
14. Tidak ada perubahan behavior/API Phase 1-8 (extend-only). Docs + CHANGELOG diperbarui.

## Documentation (`docs/`)

- `ANTIBOT.md` — guard chain, enforcement points, toggle, fail-open/fail-safe philosophy.
- `RATE_LIMITING.md` — per-category limits, Redis token bucket, webhook exclusion.
- `ABUSE_OPERATIONS.md` — super-admin runbook: block/unblock, ip rules, reading abuse log, toggling features during incident, Cloudflare WAF deployment notes (edge config, out of code).
- `PHASE9_DECISIONS.md` — keputusan + tradeoff (app-layer scope, runtime toggle, fail-open rate vs fail-safe blocklist, no client fingerprint).
- Update CHANGELOG.md.

## Setelah Phase 9

Phase 10 — Ballot/Lottery: reuse registration foundation (Phase 8) + abuse guard pada ballot submission.
Phase 11 — Invitation/Priority/Community/Corporate + WAITLIST_ONLY: gate variants di registration foundation; invitation code redemption dilindungi abuse guard.
