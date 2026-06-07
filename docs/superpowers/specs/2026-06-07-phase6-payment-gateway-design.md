# Spec — Phase 6: Payment Gateway V1

Date: 2026-06-07
Status: Draft (design)
Scope: Phase 6 dari masterplan.md — Payment Gateway (Duitku + Xendit), Callback/Webhook, Idempotency, Reconcile
Depends on: Phase 1 (foundation), Phase 2 (auth/RBAC/multi-tenant), Phase 3 (event/category), Phase 4 (form builder), Phase 5 (orders/inventory/checkout) — semua PRODUCTION BASELINE

## Prinsip: Extend, Don't Rewrite

Phase 1-5 adalah baseline produksi. Phase 6 **hanya menambah**. Dilarang:
mengubah behavior/API/auth/order flow Phase 1-5, refactor besar, rename module, pindah folder.
Phase 6 = modul `payments` BARU + binary `services/webhook` BARU + migrasi BARU + wiring
tambahan di `server.go`. Order state machine Phase 5 dipakai apa adanya: Phase 6 hanya
melakukan transisi `PENDING_PAYMENT → PAID` dan reservasi `ACTIVE → COMPLETED` yang sudah
disiapkan Phase 5.

## Tujuan

Peserta dengan order `PENDING_PAYMENT` (hasil checkout Phase 5) bisa membuat transaksi
pembayaran lewat gateway (Duitku/Xendit) dengan metode QRIS, Virtual Account, atau E-Wallet.
Saat pembayaran sukses, callback gateway (diterima oleh service webhook terpisah) memvalidasi
signature, dideduplikasi (idempotent), lalu mengubah order menjadi `PAID` dan reservasi
menjadi `COMPLETED` secara atomik. Pembayaran yang tidak dibayar mengikuti expiry order
Phase 5. Admin bisa melakukan rekonsiliasi manual (cek status ke gateway / tandai bayar).
**Belum ada**: refund, payment per-organizer, multi-gateway routing dinamis, queue.

## Keputusan yang sudah disepakati

| Topik | Keputusan |
|---|---|
| Gateway | **Duitku + Xendit** sekaligus. Interface `Gateway` bersih; tiap gateway = adapter terpisah. Midtrans menyusul (Phase lain) tinggal tambah adapter. |
| Metode bayar | **QRIS + Virtual Account + E-Wallet** (OVO/DANA/ShopeePay). Kartu kredit ditunda. |
| Penerima callback | **Service terpisah** `services/webhook` (binary baru). Isolasi callback dari API utama. |
| Arsitektur webhook | **A — Shared package + thin webhook binary.** Logika `payments` hidup sekali di `services/api/internal`; `cmd/api` & `cmd/webhook` sama-sama meng-import. Webhook = receiver tipis: **store raw callback dulu, baru proses sinkron**. |
| Refund | **Ditunda** (bukan Phase 6). State `REFUNDED` sudah ada di enum Phase 5 tapi belum dipakai. |
| Kredensial gateway | **Akun platform tunggal dari env** (merchant code/API key). Semua organizer pakai akun gateway platform. Per-organizer routing & settlement menyusul. |
| Pemilihan gateway | Peserta memilih saat create payment (`gateway` + `method` di body). Hanya kombinasi yang dikonfigurasi/aktif yang diterima. |
| Source of truth status | PostgreSQL (`payments`). Callback & reconcile sama-sama menulis lewat jalur idempotent yang sama. |

## Non-Goals (YAGNI Phase 6)

- Tidak ada refund / state REFUNDED (fase berikutnya).
- Tidak ada kredensial gateway per-organizer / enkripsi secret per-org (akun platform tunggal).
- Tidak ada Midtrans (interface siap, adapter menyusul).
- Tidak ada kartu kredit (hanya QRIS/VA/e-wallet).
- Tidak ada UI halaman pembayaran final — endpoint API + (opsional) komponen `PaymentCard` presentational dari Phase 5 saja.
- Tidak ada fallback gateway otomatis (jika gateway down → peserta pilih ulang manual; status page Phase 19).
- Tidak ada multi-item / split payment. Satu order = satu payment aktif.
- Tidak mengubah order expiry Phase 5 (worker Phase 5 tetap meng-expire `PENDING_PAYMENT`).

## Model Data (migrasi goose, lanjut dari 00014/00015 Phase 5)

Nomor migrasi final menyesuaikan migrasi terakhir yang sudah ter-commit saat implementasi.

```
payments                              ← migrasi (create_payments)
├─ id (uuid, pk, default gen_random_uuid())
├─ organization_id (uuid, fk → organizations, ON DELETE CASCADE)
├─ event_id (uuid, fk → events, ON DELETE CASCADE)
├─ order_id (uuid, fk → orders, ON DELETE RESTRICT)   ← jangan hapus order yg punya payment
├─ participant_id (uuid, fk → users, ON DELETE RESTRICT)
├─ gateway (text, not null)           ← 'duitku' | 'xendit'
├─ method (text, not null)            ← 'qris' | 'va' | 'ewallet'
├─ channel (text, nullable)           ← detail kanal (mis. 'BCA','OVO','DANA') jika relevan
├─ status (text, not null, default 'PENDING')   ← PENDING|PAID|EXPIRED|FAILED
├─ amount (bigint, not null)          ← snapshot dari orders.total saat create payment (sen)
├─ currency (text, not null, default 'IDR')
├─ gateway_reference (text, nullable) ← id transaksi di sisi gateway (reference/external id)
├─ merchant_reference (text, unique)  ← id yg KITA kirim ke gateway (PAY-YYYYMMDD-XXXXXX)
├─ pay_url (text, nullable)           ← redirect/checkout url (e-wallet/VA page) bila ada
├─ qr_string (text, nullable)         ← payload QRIS (bila method=qris)
├─ va_number (text, nullable)         ← nomor VA (bila method=va)
├─ instructions (jsonb, nullable)     ← instruksi bayar tambahan dari gateway
├─ expires_at (timestamptz, nullable) ← expiry sisi gateway (≤ order.expired_at)
├─ paid_at (timestamptz, nullable)
├─ created_at, updated_at (timestamptz, not null, default now())
CHECK (gateway IN ('duitku','xendit'))
CHECK (method IN ('qris','va','ewallet'))
CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED'))
CHECK (amount >= 0)
INDEX idx_payments_order (order_id)
INDEX idx_payments_status (status)
INDEX idx_payments_gateway_ref (gateway, gateway_reference)
-- satu payment AKTIF (PENDING/PAID) per order:
UNIQUE INDEX uq_payments_order_active ON payments(order_id) WHERE status IN ('PENDING','PAID')

payment_webhooks                      ← migrasi (create_payment_webhooks)
├─ id (uuid, pk, default gen_random_uuid())
├─ gateway (text, not null)           ← 'duitku' | 'xendit'
├─ event_type (text, nullable)        ← tipe event gateway (mis. 'payment.paid')
├─ merchant_reference (text, nullable)← di-extract dari payload utk korelasi
├─ gateway_reference (text, nullable)
├─ signature (text, nullable)         ← signature yg dikirim gateway
├─ signature_valid (bool, not null, default false)
├─ payload (jsonb, not null)          ← raw body callback (selalu disimpan, store-first)
├─ dedupe_key (text, unique)          ← kunci idempotency (lihat bagian Idempotency)
├─ processing_status (text, not null, default 'RECEIVED')  ← RECEIVED|PROCESSED|REJECTED|DUPLICATE|FAILED
├─ processed_payment_id (uuid, nullable, fk → payments)
├─ error_detail (text, nullable)      ← alasan REJECTED/FAILED
├─ received_at (timestamptz, not null, default now())
├─ processed_at (timestamptz, nullable)
INDEX idx_payment_webhooks_ref (merchant_reference)
INDEX idx_payment_webhooks_status (processing_status)
```

Catatan:
- **Tidak ada `payment_attempts` terpisah** untuk MVP: satu order punya satu payment aktif
  (UNIQUE partial index). Jika payment EXPIRED/FAILED, peserta bisa create payment baru →
  baris payment baru (yang lama tetap tersimpan sbg histori). Histori attempt = baris-baris
  `payments` per order.
- **`payment_webhooks` selalu menyimpan raw payload** sebelum diproses (store-then-process),
  sehingga tak ada callback hilang walau pemrosesan gagal → bisa di-retry/reconcile.
- **`amount` snapshot** dari `orders.total` saat create payment. Callback yang amount-nya
  tidak cocok → ditolak (`PAYMENT_AMOUNT_MISMATCH`), tidak mengubah order.
- ON DELETE RESTRICT pada `order_id`/`participant_id` melindungi integritas histori.

## Gateway Abstraction

Interface tunggal; tiap gateway adapter mengimplementasikannya. Tidak ada cabang `if duitku`
di service — service hanya tahu `Gateway`.

```
internal/modules/payments/gateway/

type CreateChargeInput struct {
    MerchantReference string        // id kita (PAY-...)
    Amount            int64         // sen
    Method            string        // qris|va|ewallet
    Channel           string        // BCA|OVO|... (opsional, tergantung method)
    CustomerEmail     string
    CustomerName      string
    ExpiresAt         time.Time
}

type CreateChargeResult struct {
    GatewayReference string
    PayURL           string
    QRString         string
    VANumber         string
    Instructions     map[string]any
    ExpiresAt        time.Time
}

type CallbackResult struct {
    MerchantReference string
    GatewayReference  string
    Status            PaymentStatus   // PAID|PENDING|EXPIRED|FAILED (sudah dinormalisasi)
    Amount            int64
    PaidAt            *time.Time
    EventType         string
}

type Gateway interface {
    Name() string                                              // "duitku"|"xendit"
    CreateCharge(ctx, CreateChargeInput) (CreateChargeResult, error)
    VerifySignature(headers http.Header, rawBody []byte) bool  // signature check
    ParseCallback(rawBody []byte) (CallbackResult, error)      // map payload → CallbackResult ternormalisasi
    QueryStatus(ctx, gatewayReference string) (CallbackResult, error)  // untuk reconcile
}
```

Adapter:
- `gateway/duitku/duitku.go` — signature Duitku (MD5/SHA256 sesuai dok), endpoint inquiry/charge,
  mapping status code Duitku → `PaymentStatus`.
- `gateway/xendit/xendit.go` — auth Basic (secret key), callback verification token, mapping
  status Xendit → `PaymentStatus`.
- `gateway/registry.go` — registry `map[string]Gateway` yang di-build dari config saat bootstrap;
  service & webhook resolve gateway by name. Hanya gateway dengan kredensial lengkap yang aktif.

Adapter dibungkus interface kecil agar bisa di-fake di unit test tanpa HTTP nyata.

## Idempotency & Dedup (WAJIB)

Callback gateway BISA datang berkali-kali (retry, duplikat). Efek harus sekali saja.

**Dedupe key** per webhook: `gateway + ":" + gateway_reference + ":" + normalized_status`
(fallback ke `gateway + ":" + merchant_reference + ":" + status` bila gateway_reference kosong).
Disimpan di `payment_webhooks.dedupe_key` (UNIQUE). Insert kedua dengan key sama → konflik →
ditandai `DUPLICATE`, tidak memproses ulang.

Alur idempotent pemrosesan callback (store-then-process, semua transisi dalam SATU transaksi):
```
1. (selalu) INSERT payment_webhooks (payload, gateway, signature, ...) status=RECEIVED
   - signature invalid → set signature_valid=false, processing_status=REJECTED, STOP (HTTP 200/4xx sesuai gateway)
2. parse payload → CallbackResult (gateway.ParseCallback)
3. BEGIN
     INSERT dedupe_key  -- jika konflik UNIQUE → mark DUPLICATE, COMMIT, STOP (idempotent no-op)
     SELECT payment WHERE merchant_reference = ref FOR UPDATE
       - tidak ada → REJECTED (PAYMENT_NOT_FOUND)
     validasi amount cocok → else REJECTED (PAYMENT_AMOUNT_MISMATCH)
     jika status callback = PAID dan payment.status = PENDING:
         UPDATE payments SET status=PAID, paid_at=..., gateway_reference=... 
         -- transisi order Phase 5 (lewat orders/inventory service dengan tx yg sama):
         SELECT order FOR UPDATE; guard order.status='PENDING_PAYMENT'
         UPDATE orders SET status='PAID'
         UPDATE inventory_reservations SET status='COMPLETED' WHERE order_id=... AND status='ACTIVE'
     jika status callback = EXPIRED/FAILED dan payment.status = PENDING:
         UPDATE payments SET status=<EXPIRED|FAILED>
         -- TIDAK menyentuh order: order expiry diurus worker Phase 5 (hindari double-handling)
     jika payment.status sudah final (PAID/EXPIRED/FAILED): no-op (idempotent)
     mark webhook PROCESSED, processed_payment_id=...
   COMMIT
```

**Catatan konsistensi dengan Phase 5:** order yang sudah `PAID` tidak akan di-expire worker
(guard `status='PENDING_PAYMENT'`). Sebaliknya, jika order keburu `EXPIRED` oleh worker
sebelum callback PAID tiba (peserta bayar lewat dari TTL), callback PAID akan menemukan
order tidak lagi `PENDING_PAYMENT` → payment ditandai PAID tapi order TIDAK berubah, dan
webhook ditandai `PROCESSED` dengan `error_detail='ORDER_ALREADY_EXPIRED'`. Kasus ini masuk
daftar reconcile manual (uang masuk tapi slot sudah lepas) — admin memutuskan refund/alokasi
manual (refund = fase berikutnya). Ini **race yang diketahui & ditangani secara eksplisit**,
bukan korupsi data.

## Oversold Interaction

Phase 5 sudah mencegah oversold saat reserve. Phase 6 hanya memindahkan reservasi `ACTIVE`
→ `COMPLETED` (tidak menambah slot terpakai: formula Phase 5 menghitung `reserved (ACTIVE)`
+ `paid (orders PAID)`). Karena transisi PAID dilakukan dalam transaksi yang me-lock order,
tidak ada jendela di mana slot terhitung ganda atau hilang.

## Struktur Modul Go

```
services/api/internal/modules/payments/
├── handler.go        create payment, get payment status, list payment by order (peserta)
├── service.go        business logic: create charge, process callback, reconcile (no HTTP)
├── repository.go     query sqlc: payments + payment_webhooks (+ akses order/reservation via dep)
├── processor.go      ProcessCallback(rawBody, headers) — dipakai api & webhook (shared core)
├── model.go          domain types, PaymentStatus enum + normalisasi
├── dto.go            request/response structs
├── validator.go      validasi create payment (gateway/method aktif, order milik caller, dll)
├── routes.go         route registration (peserta + organizer)
├── errors.go         typed errors → error codes
├── reconcile.go      manual reconcile: query gateway + apply via processor
├── gateway/
│   ├── gateway.go    interface + tipe (CreateChargeInput/Result, CallbackResult, PaymentStatus)
│   ├── registry.go   build map[string]Gateway dari config; resolve by name
│   ├── duitku/duitku.go
│   └── xendit/xendit.go
└── tests/            service_test, processor_test (idempotency/dedup), reconcile_test,
                      gateway adapter tests (signature, parse, status mapping)

services/api/cmd/webhook/main.go      ← BINARY BARU (thin receiver, port terpisah 8090)
                                      load config, connect DB, build gateway registry,
                                      build payments processor, serve callback routes

services/api/internal/modules/payments/webhook/http/   ← receiver HTTP webhook
├── server.go                 chi router + request-id + recover; route per gateway
├── duitku_handler.go         POST /webhooks/duitku → store raw → processor.ProcessCallback
├── xendit_handler.go         POST /webhooks/xendit → store raw → processor.ProcessCallback
└── server_test.go
```

**Keputusan penempatan binary webhook (penting, hindari ambiguitas):**
- **`processor.go` adalah inti yang dibagi.** `cmd/api` (untuk reconcile/test) dan binary
  webhook (untuk callback live) memanggil `payments.Processor.ProcessCallback` yang SAMA →
  tidak ada duplikasi logika kritikal.
- Binary webhook untuk Phase 6 = **`services/api/cmd/webhook/main.go`** (satu Go module dengan
  API, persis pola `cmd/worker` Phase 5). Alasannya: `payments` ada di `services/api/internal/`,
  dan package `internal/` hanya bisa di-import dari dalam module yang sama. Menaruh binary di
  module `services/api` membuat import valid tanpa menduplikasi kode atau memublikasikan package.
- **Isolasi tetap tercapai di level proses & port**: webhook adalah binary terpisah, deploy &
  port terpisah (8090), bisa di-scale sendiri. Yang TIDAK dipecah adalah Go module-nya.
- Folder top-level `services/webhook/` (dari struktur.md) **tidak dibuat di Phase 6**. Itu
  disiapkan untuk pemisahan module penuh di Phase 25 bila traffic membuktikan perlu. Struktur
  `services/webhook/internal/http/...` di atas dibaca sebagai paket
  `services/api/internal/modules/payments/webhook/http/...` untuk Phase 6.

## Endpoint

### API utama (`services/api`, port 8080)

```
# Peserta (perlu access token; payment untuk order milik caller)
POST   /api/v1/orders/{orderId}/payments
       body: { gateway: "duitku"|"xendit", method: "qris"|"va"|"ewallet", channel?: "BCA"|"OVO"|... }
       → 201 payment (PENDING + pay_url/qr_string/va_number). Order harus PENDING_PAYMENT & milik caller.
GET    /api/v1/orders/{orderId}/payments        → daftar payment utk order (histori attempt)
GET    /api/v1/payments/{paymentId}             → detail/status payment milik caller

# Organizer (perlu access token + authz dalam konteks org)
GET    /api/v1/organizations/{orgId}/events/{eventId}/payments   → daftar payment event (perlu payment.view)
POST   /api/v1/organizations/{orgId}/payments/{paymentId}/reconcile  → reconcile manual (perlu payment.manage)
       → query status ke gateway, terapkan lewat processor (idempotent)
```

### Webhook (`services/api/cmd/webhook`, port terpisah mis. 8090)
```
POST   /webhooks/duitku     terima callback Duitku
POST   /webhooks/xendit     terima callback Xendit
GET    /healthz             liveness
```

Catatan otorisasi:
- Create payment & get status: `payment.participant_id`/`order.participant_id = caller.UserID`.
  Order/payment milik user lain → 404.
- Endpoint organizer: `RequirePermission(loader, "payment.view"|"payment.manage")`; super admin lolos.
- Endpoint webhook: **tanpa auth user** — keamanan via **signature gateway** (bukan token).
  Tidak ada data sensitif di response; balas sesuai ekspektasi gateway (mis. Duitku butuh body
  tertentu, Xendit cukup 200).

## Permissions (migrasi, idempotent)

`payment.view` & `payment.refund` sudah di-seed (Phase 2, migrasi 00007). Tambah:
- `payment.manage` — "Reconcile/manage payments in org"
Assign `payment.manage` ke role template Owner & Finance. (Org lama tak otomatis dapat—copy
template hanya saat org dibuat; diterima untuk MVP, sama seperti catatan Phase 5.)

## Config / Env Baru

```
# Webhook service
WEBHOOK_PORT=8090
PAYMENT_CALLBACK_BASE_URL=                 # base URL publik webhook (utk daftar ke gateway)

# Duitku (akun platform)
DUITKU_ENABLED=true
DUITKU_MERCHANT_CODE=
DUITKU_API_KEY=
DUITKU_ENV=sandbox                         # sandbox|production
DUITKU_CALLBACK_URL=                        # = PAYMENT_CALLBACK_BASE_URL + /webhooks/duitku

# Xendit (akun platform)
XENDIT_ENABLED=true
XENDIT_SECRET_KEY=
XENDIT_CALLBACK_TOKEN=                      # verification token callback Xendit
XENDIT_ENV=sandbox

PAYMENT_DEFAULT_EXPIRY=15m                  # ≤ ORDER_EXPIRATION Phase 5; payment expiry sisi gateway
```

Aturan config:
- Gateway hanya masuk registry bila `*_ENABLED=true` DAN kredensial wajibnya terisi; bila
  `ENABLED=true` tapi kredensial kosong → API/webhook **gagal start** (fail fast, seperti
  pola `JWT_SECRET`/storage Phase 1-2).
- `PAYMENT_DEFAULT_EXPIRY` di-clamp agar tidak melebihi sisa waktu `order.expired_at`.
- Secret tidak pernah di-log. Callback yang di-log = metadata (gateway, ref, status), bukan
  signature/secret.

## Error Codes (tambahan, envelope Phase 2)

`PAYMENT_NOT_FOUND`, `ORDER_NOT_PAYABLE` (order bukan PENDING_PAYMENT), `PAYMENT_ALREADY_ACTIVE`
(sudah ada payment PENDING/PAID untuk order), `GATEWAY_NOT_AVAILABLE` (gateway/method tak aktif),
`UNSUPPORTED_METHOD`, `GATEWAY_ERROR` (gagal create charge ke gateway), `PAYMENT_AMOUNT_MISMATCH`,
`INVALID_SIGNATURE`, `RECONCILE_FAILED`.

## Audit Log

Aksi tercatat via `audit.Logger` (Phase 2):
- `PAYMENT_CREATED` (create charge), `PAYMENT_PAID` (callback sukses), `PAYMENT_FAILED`/`PAYMENT_EXPIRED`
- `PAYMENT_CALLBACK_RECEIVED` (setiap callback, dengan signature_valid & dedupe), 
- `PAYMENT_CALLBACK_REJECTED` (signature/amount/not-found),
- `PAYMENT_RECONCILED` (reconcile manual oleh admin, dengan actor_user_id)

## Documentation (`docs/`)

Tambah (markdown, dengan sequence diagram teks + state machine + failure/recovery scenario):
- `PAYMENT_FLOW.md` — create payment → bayar → callback → PAID end-to-end (sequence diagram)
- `WEBHOOK_PROCESSING.md` — store-then-process, idempotency, dedupe key, race order-expired
- `GATEWAY_INTEGRATION.md` — interface Gateway, cara menambah adapter baru (Midtrans), env
- `PAYMENT_RECONCILIATION.md` — kapan & bagaimana reconcile manual, skenario uang-masuk-slot-lepas
- `PHASE6_DECISIONS.md` — keputusan & tradeoff (webhook terpisah, akun platform tunggal, no-refund)
- `docs/payment/DUITKU.md`, `docs/payment/XENDIT.md`, `docs/payment/CALLBACK_SECURITY.md` (struktur)

## Testing

**Unit (tanpa DB/HTTP nyata, fake gateway & fake repo):**
- `gateway/duitku`: VerifySignature (valid/invalid), ParseCallback (map payload→CallbackResult),
  status mapping (semua kode Duitku → PaymentStatus). CreateCharge dengan HTTP client fake.
- `gateway/xendit`: idem (callback token, status mapping).
- `payments/service`: create payment sukses; order bukan PENDING_PAYMENT ditolak; gateway/method
  tak aktif ditolak; ownership guard; amount snapshot benar.
- `payments/processor` (INTI): 
  - callback PAID → payment PAID + order PAID + reservasi COMPLETED.
  - **idempotent**: proses callback PAID dua kali → efek sekali (dedupe DUPLICATE, no double).
  - signature invalid → REJECTED, tak mengubah payment/order.
  - amount mismatch → REJECTED.
  - payment_not_found → REJECTED.
  - race: order sudah EXPIRED → payment PAID, order tak berubah, webhook PROCESSED + error_detail.
- `reconcile`: query status gateway → apply lewat processor (idempotent dengan callback).

**Integration (Postgres `ivyticketing_test`, truncate per test):**
- Full happy path: login → checkout (Phase 5) → create payment → simulasi callback PAID →
  order PAID + reservasi COMPLETED + payment PAID + webhook PROCESSED.
- Duplicate callback: kirim callback PAID 2x → order tetap PAID sekali, webhook ke-2 DUPLICATE.
- Invalid signature callback → REJECTED, order tetap PENDING_PAYMENT.
- Expired-then-paid race: expire order via worker Phase 5, lalu callback PAID → ditangani sesuai spec.
- Reconcile: payment PENDING + gateway melaporkan PAID → reconcile → order PAID.
- Ownership: payment user A → 404 untuk user B.
- Organizer list/reconcile: butuh payment.view/payment.manage.

**Concurrency (WAJIB, `-race`):**
- `callback_concurrency_test`: 50 goroutine mengirim callback PAID yang sama untuk satu payment
  → tepat satu transisi (order PAID sekali, reservasi COMPLETED sekali, sisanya DUPLICATE/no-op).
- `webhook_store_test`: callback store-first race-free.

## Definition of Done

1. Migrasi roundtrip (up/down): `payments`, `payment_webhooks`, seed `payment.manage`.
2. Create payment membuat charge ke gateway (Duitku & Xendit) untuk QRIS/VA/e-wallet,
   mengembalikan pay_url/qr_string/va_number, payment PENDING.
3. Callback diterima service webhook terpisah; signature divalidasi; raw payload selalu tersimpan.
4. Callback PAID → payment PAID + order PAID + reservasi COMPLETED, atomik.
5. **Idempotency**: callback dobel/identik tidak menimbulkan efek ganda (dedupe + guard status).
6. Callback signature invalid / amount mismatch / payment tak ditemukan → REJECTED, order tak berubah.
7. Race order-expired-vs-paid ditangani eksplisit (payment PAID, order tak berubah, ditandai reconcile).
8. Reconcile manual (organizer payment.manage) menyinkronkan status dari gateway secara idempotent.
9. Gateway abstraction: menambah gateway baru = tambah adapter, tanpa mengubah service/processor.
10. Ownership & RBAC: peserta hanya payment sendiri; organizer butuh permission.
11. Audit: PAYMENT_CREATED/PAID/FAILED/EXPIRED, CALLBACK_RECEIVED/REJECTED, RECONCILED tercatat.
12. Docs lengkap (PAYMENT_FLOW/WEBHOOK_PROCESSING/GATEWAY_INTEGRATION/PAYMENT_RECONCILIATION/PHASE6_DECISIONS).
13. `go test ./...` hijau (unit) + integration + concurrency (`-race`) hijau.
14. `sqlc generate` bersih; semua query lewat sqlc; `go vet` bersih.
15. Config fail-fast bila gateway ENABLED tapi kredensial kosong; tidak ada secret hardcoded / ter-log.
16. Tidak ada perubahan behavior/API Phase 1-5 (extend-only). CHANGELOG.md diperbarui.

## Setelah Phase 6

Phase 7 — Participant Dashboard & Ticket: order PAID menghasilkan tiket QR signed.
Refund (state REFUNDED) menyusul memakai jalur idempotent & reconcile yang sama.
