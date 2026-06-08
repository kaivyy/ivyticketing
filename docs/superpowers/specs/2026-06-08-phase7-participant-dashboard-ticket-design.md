# Spec — Phase 7: Participant Dashboard & Ticket

Date: 2026-06-08
Status: Draft (design)
Scope: Phase 7 dari masterplan.md — Participant Dashboard (My Orders/Events), Ticket + QR signed, Invoice
Depends on: Phase 1 (foundation), Phase 2 (auth/RBAC/multi-tenant), Phase 3 (event/category), Phase 4 (form builder), Phase 5 (orders/inventory/checkout), Phase 6 (payment gateway) — semua PRODUCTION BASELINE

## Prinsip: Extend, Don't Rewrite

Phase 1-6 adalah baseline produksi. Phase 7 **hanya menambah**. Dilarang:
mengubah behavior/API/auth/order/payment flow Phase 1-6, refactor besar, rename module, pindah folder.
Phase 7 = modul `tickets` BARU + sub-package `tickets/qr` BARU + migrasi BARU + wiring tambahan
di `server.go` + fondasi frontend participant di `apps/web`. Transisi order `PENDING_PAYMENT → PAID`
Phase 6 dipakai apa adanya; Phase 7 hanya **menambahkan issue tiket** ke dalam transaksi PAID
yang sudah ada (lewat interface, tanpa mengubah logika payments).

## Tujuan

Peserta dengan order `PAID` (hasil callback Phase 6) otomatis memperoleh tiket ber-QR. Saat order
menjadi PAID, processor pembayaran meng-issue satu tiket **secara sinkron di dalam transaksi yang
sama** (atomik: PAID ⟺ tiket ada). QR tiket berupa signed token HMAC yang hanya memuat `ticket_id`,
`event_id`, dan versi (tanpa PII), bisa diverifikasi stateless. Peserta dapat melihat My Orders,
My Tickets, detail tiket + QR, timeline order, dan invoice (JSON, printable di browser) lewat
participant dashboard (Astro). Organizer dapat melihat daftar tiket per event (permission
`ticket.view`).

**Belum ada (Phase 7)**: verifikasi/scan QR penuh & check-in (Phase 15), generate PDF di backend,
pembatalan/refund tiket (state `CANCELLED` disiapkan di enum tapi belum dipakai aktif).

## Keputusan yang sudah disepakati

| Topik | Keputusan |
|---|---|
| Scope | Backend Go API + Frontend Astro (participant dashboard di `apps/web`). |
| Arsitektur integrasi | **A — Tickets module + interface hook di processor.** Payments memanggil interface kecil `TicketIssuer.IssueForOrder` di dalam transaksi `applyPaid`. Payments TIDAK import package `tickets` (dependency inversion, pola identik `AuditRecorder` Phase 6). |
| Generate tiket | **Sinkron di processor**, dalam transaksi order→PAID yang sama. INSERT tiket commit/rollback bareng order. PAID ⟺ tiket ada. |
| Isi QR | **Signed token HMAC-SHA256** (`tickets/qr`, pola `security/jwt.go`). Payload hanya `tid`,`eid`,`v`. No PII (aturan non-negotiable struktur.md). |
| Data tiket | **Snapshot ringan** saat issue: `holder_name`,`holder_email` (dari users), `event_title`,`category_name`. Tiket tetap valid walau sumber berubah. |
| Invoice & download | **JSON dulu**; PDF backend ditunda. Frontend print via CSS `@media print`. |
| Verify QR | **Generate & tampilkan saja** di Phase 7. Endpoint verify/scan + check-in ditunda Phase 15. `Sign`/`Verify` tetap diuji unit agar acceptance "QR invalid ditolak" terpenuhi. |
| Secret QR | `TICKET_QR_SECRET` env BARU, terpisah dari `JWT_SECRET`. Fail-fast bila kosong (pola Phase 1-2). |

## Non-Goals (YAGNI Phase 7)

- Tidak ada endpoint verify/scan QR & check-in (Phase 15). `status=USED`/`used_at` disiapkan tapi belum diisi.
- Tidak ada generate PDF di backend (lib render + worker). Invoice/tiket = JSON; print via browser.
- Tidak ada pembatalan/refund tiket. `status=CANCELLED` ada di enum (forward-compat) tapi belum dipakai.
- Tidak ada tabel `ticket_qr_tokens` (QR stateless signed; tak perlu simpan token).
- Tidak ada multi-tiket per order (1 order = 1 slot = 1 peserta; tidak ada quantity/order_items di Phase 5).
- Tidak mengubah order/payment flow Phase 5-6 (extend-only). Worker expiry Phase 5 tetap apa adanya.
- Tidak ada async ticket worker (issue tiket sinkron; render PDF berat baru relevan saat PDF dibuat).

## Arsitektur & Seam Atomicity (celah utama yang ditutup)

Tiket dibuat sinkron di dalam transaksi PAID milik payments. Mekanisme:

```
payments.Processor.applyPaid(ctx, tx, ...)        // tx = sqlcRepo terikat pgx.Tx (db.New(tx))
   ├─ MarkPaymentPaid                 (payments,  via tx)
   ├─ UpdateOrderStatus → PAID        (orders,    via tx)   guard status='PENDING_PAYMENT'
   ├─ CompleteReservationsForOrder    (inventory, via tx)
   └─ ticketIssuer.IssueForOrder(ctx, q, order)   ← BARU; q = *db.Queries dari tx YANG SAMA
```

- Payments mendeklarasikan interface kecil (lokal di package payments), payments **tidak import** `tickets`:
  ```go
  type TicketIssuer interface {
      // IssueForOrder issues a ticket for a just-PAID order, using the SAME tx querier.
      // Must be idempotent: if a ticket for order already exists (UNIQUE order_id), no-op.
      IssueForOrder(ctx context.Context, q *db.Queries, order db.Order) error
  }
  ```
- `tickets.Issuer` mengimplementasikan interface ini. Karena `q` adalah `*db.Queries` yang dibungkus
  `pgx.Tx` (lihat `repository.go:55-65` ExecTx → `db.New(tx)`), INSERT tiket ikut commit/rollback
  bersama transisi order. **Tidak ada koneksi/pool baru** → tidak ada celah atomicity.
- **Idempotency**: `UNIQUE(order_id)` di tabel tickets. `IssueForOrder` melakukan
  insert-on-conflict-do-nothing (atau cek-lalu-insert dalam tx). Callback dobel / reconcile yang
  sudah ditangani Phase 6 → tetap tepat satu tiket.
- **Rollback (penjamin celah)**: bila `IssueForOrder` mengembalikan error, `applyPaid` mengembalikan
  error → `ExecTx` rollback → order TIDAK jadi PAID, payment TIDAK jadi PAID. Tidak ada state
  "PAID tanpa tiket". Diuji eksplisit.
- **Wiring** (`server.go`): `issuer := ticketsmod.NewIssuer(...)`; di-inject ke
  `paymentsmod.NewProcessor(repo, auditLog, issuer)`. Reconcile path Phase 6 (yang juga memanggil
  `applyPaid` lewat `Apply`) otomatis ikut meng-issue tiket — konsisten.

Order state machine & oversold Phase 5/6 tidak berubah: Phase 7 hanya menumpang transaksi PAID
untuk satu INSERT tambahan.

## Model Data (migrasi goose, lanjut dari 00017 Phase 6)

Nomor migrasi final menyesuaikan migrasi terakhir yang ter-commit saat implementasi.

```
tickets                               ← migrasi (create_tickets)
├─ id (uuid, pk, default gen_random_uuid())
├─ organization_id (uuid, fk → organizations, ON DELETE CASCADE)
├─ event_id (uuid, fk → events, ON DELETE CASCADE)
├─ category_id (uuid, fk → event_categories, ON DELETE RESTRICT)
├─ order_id (uuid, fk → orders, ON DELETE RESTRICT)        ← histori terlindungi
├─ participant_id (uuid, fk → users, ON DELETE RESTRICT)
├─ ticket_number (text, not null, unique)   ← TIX-YYYYMMDD-XXXXXX (pola ordernum.go Phase 5)
├─ status (text, not null, default 'VALID') ← VALID | USED | CANCELLED
├─ holder_name (text, not null)             ← snapshot dari users saat issue
├─ holder_email (text, not null)            ← snapshot dari users saat issue
├─ event_title (text, not null)             ← snapshot dari events
├─ category_name (text, not null)           ← snapshot dari event_categories
├─ qr_version (int, not null, default 1)    ← versi skema/secret payload QR (rotasi tanpa migrasi)
├─ issued_at (timestamptz, not null, default now())
├─ used_at (timestamptz, nullable)          ← diisi Phase 15 saat check-in
├─ created_at, updated_at (timestamptz, not null, default now())
CHECK (status IN ('VALID','USED','CANCELLED'))
UNIQUE (order_id)                            ← 1 order = 1 tiket; penjamin idempotency issue
INDEX idx_tickets_participant (participant_id)
INDEX idx_tickets_event (event_id)
INDEX idx_tickets_status (status)
```

Catatan:
- **UNIQUE(order_id)** = idempotency level DB. Issue dobel → konflik → no-op.
- **Snapshot** kolom holder/event/category diambil saat issue (keputusan disepakati).
- **Tidak ada `ticket_qr_tokens`**: QR stateless signed HMAC (bagian QR). `qr_version` untuk rotasi.
- **`status`**: hanya `VALID` aktif di Phase 7. `USED` (Phase 15 scan), `CANCELLED` (refund, fase
  berikutnya) disiapkan di enum agar forward-compatible (pola `REFUNDED` Phase 6).
- ON DELETE RESTRICT pada `order_id`/`participant_id` melindungi integritas histori.

## QR Signed Token (HMAC-SHA256)

Sub-package `tickets/qr/` — pola seperti `security/jwt.go`, secret & audience terpisah.

```
Format: <qr_version> "." base64url(payload_json) "." base64url(hmac_sha256(secret, version+"."+payload))

payload_json = { "tid": "<ticket uuid>", "eid": "<event uuid>", "v": 1 }
```

- **No PII di QR** — hanya UUID + versi (aturan non-negotiable: "No sensitive data inside QR").
- **Stateless verify**: `Verify(token) (TicketRef, error)` cek signature dengan `TICKET_QR_SECRET`.
  Tidak perlu DB untuk validasi signature. Lookup status (VALID/USED) baru di Phase 15 saat scan.
- **Secret terpisah** `TICKET_QR_SECRET` (env baru). Bocornya JWT secret tidak membocorkan QR, dan
  sebaliknya. Fail-fast saat start bila kosong (pola `JWT_SECRET` Phase 1-2).
- **`qr_version`** di payload + kolom DB → rotasi secret/skema tanpa menginvalidasi tiket lama
  (verifier memilih secret berdasarkan versi).
- **Tidak ada `exp`**: tiket valid sampai event; validitas waktu/status diserahkan ke pengecekan
  status DB di Phase 15, bukan ke expiry token.
- Phase 7: API menghasilkan & menampilkan token string (frontend merender jadi gambar QR client-side).
  `Sign`/`Verify` diuji unit (valid/invalid/tampered/versi salah) — memenuhi acceptance "QR invalid ditolak".

## Struktur Modul Go

```
services/api/internal/modules/tickets/
├── handler.go        list my tickets, get ticket+qr, ticket by order, invoice (peserta) + list event (organizer)
├── service.go        business logic: issue (dipakai issuer), get, list, build invoice (no HTTP)
├── repository.go     query sqlc: tickets (+ ExecTx pattern bila perlu)
├── issuer.go         Issuer.IssueForOrder(ctx, *db.Queries, order) — dipanggil payments processor (tx sama)
├── model.go          domain types, TicketStatus enum
├── dto.go            request/response structs (ticket, invoice, timeline)
├── ticketnum.go      generate TIX-YYYYMMDD-XXXXXX (pola ordernum.go)
├── validator.go      ownership guards, order-PAID guard untuk invoice
├── routes.go         route registration (peserta + organizer)
├── errors.go         typed errors → error codes
├── qr/
│   ├── qr.go         Sign(ticketID, eventID, version) string; Verify(token) (TicketRef, error)
│   └── qr_test.go    sign/verify roundtrip, invalid/tampered/version
└── tests/            issuer_test (idempotency), service_test (ownership, invoice gating), handler tests

Perubahan di payments (extend, bukan rewrite):
- payments: tambah interface lokal TicketIssuer + field di Processor; panggil di applyPaid (tx sama).
- server.go: build tickets.Issuer, inject ke payments.NewProcessor; mount tickets routes.
```

## Endpoint

### API utama (`services/api`, port 8080)

```
# Peserta (perlu access token; resource milik caller)
GET /api/v1/tickets                         → daftar tiket milik caller (My Tickets)
GET /api/v1/tickets/{ticketId}              → detail tiket + QR token (milik caller)
GET /api/v1/tickets/{ticketId}/qr           → QR token string (untuk dirender frontend)
GET /api/v1/orders/{orderId}/ticket         → tiket untuk order tsb (milik caller)
GET /api/v1/orders/{orderId}/invoice        → invoice JSON (milik caller, order PAID)

# Sudah ada Phase 5 (dipakai dashboard, TIDAK diubah):
GET /api/v1/orders                          → My Orders
GET /api/v1/orders/{orderId}                → detail order (+ timeline diturunkan)

# Organizer (perlu access token + authz konteks org/event)
GET /api/v1/organizations/{orgId}/events/{eventId}/tickets   → daftar tiket event (perlu ticket.view)
```

Catatan otorisasi & data:
- **Ownership guard**: `ticket.participant_id`/`order.participant_id = caller.UserID`; bukan → **404**
  (pola Phase 5/6, tidak membocorkan eksistensi).
- **Invoice JSON**: snapshot order (order_number, event, kategori, subtotal/fee/discount/total,
  status, paid_at, ringkasan payment terkait, holder). Murni baca; hanya untuk order PAID
  (else `INVOICE_NOT_AVAILABLE`).
- **Order timeline**: dirakit dari timestamp existing (created_at → expired_at/paid → ticket.issued_at).
  Tidak ada tabel event baru.
- **Verify/scan QR**: TIDAK ada di Phase 7 (Phase 15).

## Permissions (migrasi, idempotent)

Tambah:
- `ticket.view` — "View tickets in org/event".
Assign `ticket.view` ke role template Owner, Manager, Customer Service. (Org lama tak otomatis dapat
— copy template hanya saat org dibuat; diterima untuk MVP, sama seperti catatan Phase 5/6.)

## Frontend Astro (`apps/web`)

**State saat ini (diverifikasi):** `apps/web` minimal — hanya `index.astro`, `lib/api.ts`
(`fetchReadiness` saja), `PublicLayout.astro`. **Belum ada auth/session/login frontend.** Maka
Phase 7 frontend termasuk membangun fondasi auth participant minimal.

```
apps/web/src/
├─ pages/participant/
│  ├─ dashboard.astro          → ringkasan My Events + order/tiket terbaru
│  ├─ orders.astro             → My Orders (list + status)
│  ├─ orders/[orderId].astro   → detail order + timeline + invoice + link tiket
│  ├─ tickets.astro            → My Tickets (list)
│  └─ tickets/[ticketId].astro → detail tiket + QR
├─ pages/login.astro           → login participant (fondasi auth)
├─ components/ticket/
│  ├─ TicketCard.astro
│  ├─ QrDisplay.astro          → render token jadi gambar QR (lib client-side, mis. qrcode)
│  ├─ OrderTimeline.astro
│  └─ InvoiceView.astro        → printable via CSS @media print
├─ layouts/ParticipantLayout.astro  → layout auth-gated
├─ middleware.ts               → redirect ke login bila tak ada session
└─ lib/
   ├─ auth.ts                  → login → simpan access token, helper authed fetch (BARU)
   ├─ api.ts                   → diperluas: authed fetch + parse error envelope Phase 2
   ├─ tickets.ts               → fetch tickets/qr
   └─ invoice.ts               → fetch invoice
```

Catatan:
- **Fondasi auth minimal**: login form → akses token (auth Phase 2), simpan (cookie/localStorage
  sesuai pola aman), attach ke fetch; route guard di layout/middleware. Token QR tidak pernah dibuat
  di frontend, hanya ditampilkan.
- **QR render client-side** dari token string (lib JS ringan). **Invoice "download"** = CSS print
  + tombol Print (browser → save as PDF). Tidak ada PDF backend.
- **Reuse** `lib/api.ts` & `PUBLIC_API_URL` existing; ikuti error envelope Phase 2.

## Config / Env Baru

```
TICKET_QR_SECRET=                 # HMAC secret QR; fail-fast bila kosong; terpisah dari JWT_SECRET
# Frontend:
PUBLIC_API_URL=                   # sudah ada polanya di apps/web
```

Aturan: secret tidak pernah di-log. Fail fast saat start bila `TICKET_QR_SECRET` kosong (pola Phase 1-2).

## Error Codes (tambahan, envelope Phase 2)

`TICKET_NOT_FOUND`, `TICKET_NOT_AVAILABLE` (order belum PAID / tiket belum ter-issue),
`INVOICE_NOT_AVAILABLE` (order belum PAID), `INVALID_QR` (signature/format QR — dipakai unit test
& Phase 15).

## Audit Log

Aksi tercatat via `audit.Logger` (Phase 2):
- `TICKET_ISSUED` (saat issue di transaksi PAID, dengan order_id/ticket_id).

## Documentation (`docs/`)

Tambah (markdown, dengan sequence diagram teks + state machine + failure/recovery):
- `TICKET_FLOW.md` — order PAID → issue tiket sinkron (atomik, rollback) → tampil ke peserta (sequence diagram)
- `QR_TICKET.md` — skema payload, HMAC signing, no-PII, versioning, kenapa stateless, verify ditunda Phase 15
- `PARTICIPANT_DASHBOARD.md` — endpoint My Orders/Tickets/Invoice, ownership, timeline, frontend
- `PHASE7_DECISIONS.md` — keputusan & tradeoff (sinkron-di-processor + seam tx, signed-vs-opaque QR, JSON-vs-PDF, verify ditunda, secret terpisah)
- Update `CHANGELOG.md`

## Testing

**Unit (tanpa DB/HTTP nyata bila memungkinkan, fake querier/repo):**
- `tickets/qr`: Sign→Verify roundtrip; signature invalid ditolak; payload tampered ditolak;
  versi salah ditolak; format rusak ditolak ("QR invalid ditolak" — acceptance Phase 7).
- `tickets/issuer`: issue snapshot benar; **idempotent** (issue dua kali untuk order sama → 1 tiket,
  UNIQUE order_id no-op); ticket_number unik.
- `tickets/service`: ownership guard; invoice hanya untuk order PAID (else INVOICE_NOT_AVAILABLE).
- `payments/processor` (diperluas): callback PAID → tiket ter-issue dalam tx yang sama;
  **idempotent** (callback dobel → tetap 1 tiket); **rollback** (issuer error → order TIDAK PAID,
  payment TIDAK PAID — celah tertutup, diuji eksplisit).

**Integration (Postgres `ivyticketing_test`, truncate per test):**
- Full happy path: login → checkout (Phase 5) → create payment (Phase 6) → callback PAID →
  tiket VALID + order PAID + reservasi COMPLETED + QR token terverifikasi via qr.Verify.
- Duplicate callback PAID → tetap satu tiket (order PAID sekali).
- Ownership: tiket/invoice user A → 404 untuk user B.
- Invoice sebelum PAID → INVOICE_NOT_AVAILABLE; sesudah PAID → sukses.
- Organizer list tiket event butuh `ticket.view`.

**Concurrency (WAJIB, `-race`):**
- `issue_concurrency_test`: 50 callback PAID konkuren untuk satu order → tepat satu tiket.

**Frontend:**
- Dev server dijalankan; golden path diuji manual di browser (login → order PAID → tiket → QR tampil
  → invoice print). Dinyatakan eksplisit apa yang bisa/tidak bisa diverifikasi otomatis.

## Definition of Done

1. Migrasi `tickets` roundtrip (up/down) + seed `ticket.view` idempotent.
2. Order→PAID meng-issue tiket **atomik dalam transaksi yang sama**; PAID ⟺ tiket ada; idempotent (UNIQUE order_id).
3. **Rollback**: bila issue tiket gagal, order & payment TIDAK menjadi PAID (celah tertutup, diuji).
4. QR signed HMAC; no PII; `Sign`/`Verify` diuji; QR invalid/tampered/versi-salah ditolak.
5. Endpoint peserta (tickets, ticket+qr, order ticket, invoice JSON) dengan ownership guard (404).
6. Endpoint organizer list tiket event butuh `ticket.view`; super admin lolos.
7. Frontend: participant dashboard (My Orders/Tickets, detail, timeline) + fondasi auth minimal;
   QR tampil; invoice printable; diverifikasi di browser.
8. Audit: `TICKET_ISSUED` tercatat.
9. `go test ./...` (unit) + integration + concurrency (`-race`) hijau.
10. `sqlc generate` bersih; semua query lewat sqlc; `go vet` bersih.
11. Config fail-fast bila `TICKET_QR_SECRET` kosong; tidak ada secret hardcoded / ter-log.
12. Tidak ada perubahan behavior/API Phase 1-6 (extend-only). Docs + CHANGELOG diperbarui.

## Setelah Phase 7

Phase 8 — Queue/War Ticket System. Verifikasi/scan QR + check-in (`status=USED`, `used_at`) di
Phase 15 (Scanner PWA) memakai `qr.Verify` yang sudah dibuat di sini. Refund (`status=CANCELLED`)
menyusul memakai jalur idempotent yang sama.
