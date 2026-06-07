# Spec — Phase 5: Orders, Inventory & Checkout Core

Date: 2026-06-07
Status: Approved (design)
Scope: Phase 5 dari masterplan.md — Orders + Inventory Lock + Reservation + Checkout Foundation + UI Foundation
Depends on: Phase 1 (foundation), Phase 2 (auth/RBAC/multi-tenant), Phase 3 (event/category), Phase 4 (form builder) — semua PRODUCTION BASELINE

## Prinsip: Extend, Don't Rewrite

Phase 1-4 adalah baseline produksi. Phase 5 **hanya menambah**. Dilarang:
mengubah behavior/API/auth/form flow Phase 1-4, refactor besar, rename module, pindah folder.
Semua kerja Phase 5 = file/module/migrasi BARU + wiring tambahan di `server.go`.

## Tujuan

Peserta bisa memilih kategori dan membuat order; sistem me-reserve slot, mengurangi
inventory secara aman (anti-oversold via lock DB), melepas inventory saat order timeout,
mencegah duplicate checkout. Plus fondasi design-system UI (`packages/ui`) tanpa halaman final.
**Belum ada**: payment gateway, queue, ballot, racepack.

## Keputusan yang sudah disepakati

| Topik | Keputusan |
|---|---|
| Order expiration | Default 15 menit (`ORDER_EXPIRATION` env, default `15m`). |
| Peserta | User yang login (auth Phase 2). `participant_id` = `users.id`. Tidak ada guest checkout di Phase 5. |
| max_order_per_user | Di-enforce: total slot AKTIF (reserved/paid) per user per kategori ≤ `event_categories.max_order_per_user`. |
| packages/ui | Dalam monorepo (`packages/ui/`), Tailwind + Radix. Tidak ada halaman final. |
| Worker | Binary terpisah `services/api/cmd/worker/main.go`, ticker interval 1 menit. |
| Oversold prevention | Transaksi + `SELECT ... FOR UPDATE` pada baris kategori. Source of truth = PostgreSQL. |
| Quantity per order | Satu order = satu kategori, quantity = 1 slot (MVP). max_order_per_user membatasi jumlah order aktif. |

## Non-Goals (YAGNI Phase 5)

- Tidak ada payment gateway / payment record (Phase 6). Order berhenti di `PENDING_PAYMENT`.
- Tidak ada queue/waiting room (Phase 8), ballot (Phase 10), racepack (Phase 14).
- Tidak ada form submission persisten (peserta mengisi form) — itu menyusul; Phase 5 checkout belum menautkan jawaban form.
- Tidak ada halaman UI final / landing / dashboard. `packages/ui` hanya komponen design-system.
- Tidak ada multi-item cart. Satu checkout = satu kategori, satu slot.
- Tidak ada Redis untuk inventory counting — source of truth murni PostgreSQL (Redis dipakai nanti untuk queue).

## Model Data (migrasi goose, lanjut dari 00011)

```
orders                                ← migrasi 00012
├─ id (uuid, pk, default gen_random_uuid())
├─ organization_id (uuid, fk → organizations, ON DELETE CASCADE)
├─ event_id (uuid, fk → events, ON DELETE CASCADE)
├─ category_id (uuid, fk → event_categories, ON DELETE RESTRICT)   ← jangan hapus kategori yg punya order
├─ participant_id (uuid, fk → users, ON DELETE RESTRICT)
├─ order_number (text, unique)        ← ORD-YYYYMMDD-XXXXXX
├─ status (text, not null, default 'DRAFT')   ← DRAFT|PENDING_PAYMENT|PAID|EXPIRED|CANCELLED|REFUNDED
├─ subtotal (bigint, not null)        ← harga kategori saat checkout (snapshot, sen)
├─ fee (bigint, not null, default 0)
├─ discount (bigint, not null, default 0)
├─ total (bigint, not null)           ← subtotal + fee - discount
├─ expired_at (timestamptz, nullable) ← diisi saat masuk PENDING_PAYMENT
├─ created_at, updated_at (timestamptz, not null, default now())
CHECK (status IN ('DRAFT','PENDING_PAYMENT','PAID','EXPIRED','CANCELLED','REFUNDED'))
CHECK (subtotal >= 0 AND fee >= 0 AND discount >= 0 AND total >= 0)
INDEX idx_orders_org_event (organization_id, event_id)
INDEX idx_orders_participant (participant_id)
INDEX idx_orders_status_expired (status, expired_at)   ← untuk worker scan

inventory_reservations                ← migrasi 00013
├─ id (uuid, pk, default gen_random_uuid())
├─ organization_id (uuid, fk → organizations, ON DELETE CASCADE)
├─ event_id (uuid, fk → events, ON DELETE CASCADE)
├─ category_id (uuid, fk → event_categories, ON DELETE RESTRICT)
├─ order_id (uuid, fk → orders, ON DELETE CASCADE, UNIQUE)   ← 1 reservasi per order
├─ participant_id (uuid, fk → users, ON DELETE RESTRICT)
├─ status (text, not null, default 'ACTIVE')   ← ACTIVE|EXPIRED|COMPLETED|RELEASED
├─ expires_at (timestamptz, not null)
├─ created_at (timestamptz, not null, default now())
CHECK (status IN ('ACTIVE','EXPIRED','COMPLETED','RELEASED'))
INDEX idx_reservations_category (category_id)
INDEX idx_reservations_status (status)
```

Catatan:
- **Inventory tidak punya tabel sendiri.** Kapasitas = `event_categories.capacity` (Phase 3). Hitungan reserved/paid dihitung dari `orders`/`inventory_reservations` secara live, di dalam transaksi terkunci.
- **`order_id UNIQUE`** di reservations → satu order maksimal satu reservasi (cegah duplicate reservation).
- **`subtotal`/`total` snapshot** harga saat checkout — kalau organizer ubah harga kategori nanti, order lama tak berubah.
- ON DELETE RESTRICT pada `category_id`/`participant_id` melindungi integritas histori order.

## Inventory Model & Formula

Kapasitas adalah `event_categories.capacity`. Hitungan saat checkout (dalam transaksi):

```
reserved_count = COUNT(inventory_reservations
                       WHERE category_id = X AND status = 'ACTIVE')
paid_count     = COUNT(orders
                       WHERE category_id = X AND status = 'PAID')
remaining      = capacity - reserved_count - paid_count
```

**Catatan konsistensi:** Saat order PAID (Phase 6 nanti), reservasi jadi `COMPLETED` dan
order jadi `PAID` — agar tidak dihitung dua kali, `reserved_count` hanya menghitung
reservasi `ACTIVE`, dan `paid_count` menghitung order `PAID`. Reservasi `COMPLETED`
tidak dihitung di reserved (sudah pindah ke paid). Di Phase 5 (belum ada payment), order
maksimal sampai `PENDING_PAYMENT`, jadi `paid_count` selalu 0 — tapi formula sudah benar
untuk Phase 6.

**Dilarang** menghitung inventory dari frontend. Endpoint preview kategori (Phase 3 public)
boleh menampilkan `remaining` read-only, tapi keputusan reserve selalu di server dalam lock.

## Oversold Prevention (WAJIB)

Alur checkout (semua dalam SATU transaksi):
```
BEGIN
  -- lock baris kategori agar checkout konkuren ke kategori sama ter-serialize
  SELECT * FROM event_categories WHERE id = $catID FOR UPDATE;

  -- validasi: kategori ada, milik event/org, registration window terbuka
  -- hitung reserved + paid (query di atas)
  -- cek remaining > 0  → else ERR_SOLD_OUT
  -- cek max_order_per_user: COUNT order aktif user di kategori < max → else ERR_MAX_ORDER_EXCEEDED
  -- cek duplicate: user belum punya order aktif (DRAFT/PENDING_PAYMENT) utk kategori? (opsi, lihat bawah)

  INSERT INTO orders (...) status=PENDING_PAYMENT, expired_at=now()+TTL;
  INSERT INTO inventory_reservations (...) status=ACTIVE, expires_at=now()+TTL;
COMMIT
```

`SELECT ... FOR UPDATE` pada baris kategori menjadi titik serialisasi: dua checkout
konkuren ke kategori sama akan antri, sehingga `remaining` selalu dibaca setelah commit
sebelumnya. Tidak akan oversold / negative / duplicate reservation.

**Acceptance**: capacity=100, 200 request checkout konkuren → `reserved+paid ≤ 100`,
tak ada inventory negatif, tak ada reservasi ganda.

## Order Number

Format: `ORD-YYYYMMDD-XXXXXX` di mana `XXXXXX` = 6 char alfanumerik uppercase acak
(crypto/rand). Unik (kolom UNIQUE). Pada tabrakan (sangat jarang), retry generate
dalam batas (mis. 5x) lalu error.

## Order State Machine

```
DRAFT ──────────► PENDING_PAYMENT ──► PAID (Phase 6)
                       │
                       ├──► EXPIRED   (worker: expired_at lewat)
                       └──► CANCELLED (peserta DELETE / cancel)
PAID ──► REFUNDED (Phase 6)
```
Phase 5: checkout langsung membuat order `PENDING_PAYMENT` (DRAFT dilewati — disiapkan di
enum untuk masa depan multi-step). Transisi yang valid di Phase 5:
- `PENDING_PAYMENT → EXPIRED` (worker)
- `PENDING_PAYMENT → CANCELLED` (peserta)
Saat EXPIRED/CANCELLED → reservasi terkait jadi `EXPIRED`/`RELEASED`, slot kembali (otomatis
karena reserved_count hanya hitung ACTIVE).

## Struktur Modul Go

```
internal/modules/
├── orders/          checkout, list, get, cancel
│   handler.go service.go repository.go model.go dto.go validator.go routes.go errors.go + tests
└── inventory/       lock + reserve + release + counting (dipanggil orders service, no HTTP)
    service.go repository.go reservation.go expiration.go stock.go lock.go + tests

internal/platform/
└── worker/          ticker runner (reusable)
    worker.go worker_test.go

cmd/
└── worker/main.go   binary worker: load config, connect DB, run expire_orders job
```

Catatan: `inventory` tidak punya handler/routes — ia paket domain yang dipakai `orders`
service dan worker. `orders` service memanggil `inventory` di dalam transaksinya (lewat
repository yang berbagi `pgx.Tx`).

## Reservation & Inventory Interaction (atomic)

`orders.Service.Checkout` membuka transaksi via `ExecTx` dan memanggil `inventory`
operations dengan `Repository` yang dibungkus tx yang sama:
```
ExecTx(ctx, func(txRepo) {
    cat := txRepo.LockCategory(catID)          // SELECT ... FOR UPDATE
    counts := txRepo.CountReservedAndPaid(catID)
    if cat.Capacity - counts.Reserved - counts.Paid <= 0 { return ErrSoldOut }
    if txRepo.CountActiveOrdersForUser(catID, userID) >= cat.MaxOrderPerUser { return ErrMaxOrder }
    order := txRepo.CreateOrder(...)
    txRepo.CreateReservation(order.ID, ...)
})
```

## Expiration Worker

`cmd/worker/main.go`: load config, connect Postgres, jalankan loop ticker 1 menit memanggil
`expire_orders` job. Job (idempotent, dalam transaksi per batch):
```
SELECT id FROM orders
  WHERE status = 'PENDING_PAYMENT' AND expired_at < now()
  FOR UPDATE SKIP LOCKED
  LIMIT 100;
-- untuk tiap order:
UPDATE orders SET status='EXPIRED', updated_at=now() WHERE id=$id AND status='PENDING_PAYMENT';
UPDATE inventory_reservations SET status='EXPIRED' WHERE order_id=$id AND status='ACTIVE';
-- audit: ORDER_EXPIRED, RESERVATION_EXPIRED
```
`FOR UPDATE SKIP LOCKED` membuat aman dijalankan paralel/berkali-kali. Guard `AND
status='PENDING_PAYMENT'` membuat idempotent (order yang sudah EXPIRED/CANCELLED dilewati).

## Endpoint

```
# Peserta (perlu access token; participant = caller user)
POST   /api/v1/events/{eventId}/categories/{categoryId}/checkout
       → 201 order (PENDING_PAYMENT + reservasi). Body kosong (slot=1).
GET    /api/v1/orders                          → daftar order milik caller (semua status)
GET    /api/v1/orders/{orderId}                → detail order milik caller
DELETE /api/v1/orders/{orderId}                → cancel order milik caller (PENDING_PAYMENT→CANCELLED)

# Organizer (perlu access token + authz order.view dalam konteks org)
GET    /api/v1/organizations/{orgId}/events/{eventId}/orders   → daftar order event milik org
```

Catatan otorisasi:
- Endpoint peserta: order difilter `participant_id = caller.UserID`. Order milik user lain → 404.
- Endpoint organizer: `middleware.RequirePermission(loader, "order.view")`; super admin lolos.
- Checkout: kategori harus milik event yang `published`, registration window terbuka,
  kategori valid. Event/kategori tak ada/tak published → 404.

## Permissions (migrasi 00014, idempotent)

`order.view` & `order.refund` sudah di-seed (00007). Tambah:
- `order.create` — "Create orders on behalf" (untuk organizer manual order, future)
- `order.manage` — "Manage/cancel any order in org"
Assign `order.create`+`order.manage` ke role template Owner & Manager (org yang sudah ada
meng-copy template Phase 2 hanya saat dibuat — tambahkan ke template global; org lama tak
otomatis dapat, itu diterima untuk MVP). Endpoint peserta checkout TIDAK butuh permission
RBAC org (peserta bukan staff) — cukup authenticated + ownership.

## Error Codes (tambahan, envelope Phase 2)

`ORDER_NOT_FOUND`, `CATEGORY_NOT_FOUND`, `EVENT_NOT_PUBLISHED`, `REGISTRATION_CLOSED`,
`SOLD_OUT`, `MAX_ORDER_EXCEEDED`, `DUPLICATE_ACTIVE_ORDER`, `INVALID_ORDER_STATE`,
`ORDER_NUMBER_GENERATION_FAILED`.

## Audit Log

Aksi tercatat via `audit.Logger` (Phase 2):
- `ORDER_CREATED` (saat checkout), `ORDER_EXPIRED` (worker), `ORDER_CANCELLED` (cancel)
- `RESERVATION_CREATED` (checkout), `RESERVATION_EXPIRED` (worker)

## UI Foundation (`packages/ui`)

Design-system foundation, **bukan halaman final**. Monorepo package, dipakai apps Astro nanti.

Komponen: Button, Input, Select, Textarea, Checkbox, Radio, Badge, Alert, Card, Modal,
Dialog, Table, EmptyState, LoadingState, ErrorState, QueueCard, PaymentCard, TicketCard,
plus `index.ts` barrel export.

Stack: Tailwind CSS, Radix primitives (untuk Modal/Dialog/Select aksesibel), TypeScript.

Theme tokens:
```
Primary    #0B3D2E   Secondary  #111827   Accent   #D6A84F
Background #F8F7F2   Success    #16A34A   Warning  #F59E0B   Danger #DC2626
Typography Inter
```

`packages/ui/README.md`: usage tiap komponen, props, contoh.

Catatan: komponen domain (QueueCard/PaymentCard/TicketCard) dibuat sebagai presentational
shell dengan props — belum terhubung data/API (queue & payment belum ada). Murni visual.

## Documentation (`docs/`)

Tambah (markdown, dengan sequence diagram teks + state machine + failure/recovery scenario):
- `ORDER_FLOW.md` — alur checkout end-to-end
- `INVENTORY.md` — formula, source of truth, anti-oversold
- `RESERVATION_SYSTEM.md` — lifecycle reservasi, expiry
- `CHECKOUT_FLOW.md` — sequence diagram checkout
- `PHASE5_DECISIONS.md` — keputusan & tradeoff (kenapa FOR UPDATE, kenapa no Redis, dll)

## Testing

**Unit (tanpa DB, fake repo):**
- `inventory/stock`: formula remaining; reserved hanya ACTIVE; paid hanya PAID.
- `orders/service`: checkout sukses; sold-out ditolak; max_order ditolak; cancel transisi; tenant/ownership guard.
- `orders` order-number generator: format benar; retry pada tabrakan.
- `worker`: ticker memanggil job; job idempotent (jalankan 2x → efek sama).

**Integration (Postgres `ivyticketing_test`, truncate per test):**
- Full checkout: login → org → event(+publish) → kategori → checkout → order PENDING_PAYMENT + reservasi ACTIVE.
- Cancel: checkout → DELETE → CANCELLED + reservasi RELEASED + slot kembali.
- Expiration: checkout dengan TTL pendek → jalankan job → EXPIRED + reservasi EXPIRED + slot kembali.
- Ownership: order user A → 404 untuk user B.
- Organizer list: order tampil di endpoint org dengan `order.view`.

**Concurrency (WAJIB, integration dengan goroutine):**
- `inventory_concurrency_test`: capacity=100, 200 goroutine checkout → sukses ≤100, sisanya SOLD_OUT, `reserved+paid ≤ 100`, tak ada negatif.
- `reservation_concurrency_test`: tak ada reservasi ganda per order.
- `order_creation_test`: order_number unik di bawah konkurensi.
- `expiration_worker_test`: worker idempotent + race-free (`go test -race`).

## Definition of Done

1. Migrasi roundtrip (up/down): `orders`, `inventory_reservations`, seed permissions.
2. Checkout membuat order PENDING_PAYMENT + reservasi ACTIVE secara atomik.
3. Oversold prevention: 200 konkuren vs capacity 100 → tak pernah oversold/negatif/duplikat.
4. max_order_per_user di-enforce.
5. Cancel order melepas reservasi & mengembalikan slot.
6. Worker `expire_orders` mengubah PENDING_PAYMENT kadaluwarsa → EXPIRED, melepas reservasi; idempotent.
7. Ownership: peserta hanya akses order sendiri; organizer lihat order org via `order.view`.
8. Audit: ORDER_CREATED/EXPIRED/CANCELLED, RESERVATION_CREATED/EXPIRED tercatat.
9. `packages/ui`: semua komponen + README; build bersih.
10. Docs: ORDER_FLOW/INVENTORY/RESERVATION_SYSTEM/CHECKOUT_FLOW/PHASE5_DECISIONS lengkap.
11. `go test ./...` hijau (unit) + integration + concurrency (`-race`) hijau.
12. `sqlc generate` bersih; semua query lewat sqlc; `go vet` bersih.
13. Tidak ada perubahan behavior/API Phase 1-4 (extend-only).
14. CHANGELOG.md diperbarui dengan entry Phase 5.

## Setelah Phase 5

Phase 6 — Payment Gateway V1. Order `PENDING_PAYMENT` → `PAID`/`EXPIRED` lewat callback;
reservasi `ACTIVE` → `COMPLETED`; memakai state machine & inventory yang sudah ada.
