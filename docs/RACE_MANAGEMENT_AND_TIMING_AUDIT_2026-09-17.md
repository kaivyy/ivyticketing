# Audit Arsitektur IvyTicketing: Race Management, Ticketing, & Integrasi RFID Timing
**Tanggal Audit**: 17 September 2026  
**Status**: Lengkap & Diverifikasi  
**Cakupan**: Platform Event Lari, Organizer Console, Participant Portal, Ticketing Engine, Gate Check-In, Logistik Racepack, dan Kesiapan RFID Timing  

---

## 1. Stack Teknologi yang Digunakan

### Backend (API, Webhook, Worker)
- **Bahasa & Runtime**: Go 1.25 (Pola Modular Monolith)
- **HTTP Routing**: [`go-chi/chi/v5`](file:///root/ivyticketing/services/api/internal/app/server.go#L9)
- **Database Driver & Pooling**: `pgx/v5` (`pgxpool`)
- **Query Layer**: `sqlc` (type-safe SQL compiler otomatis dari direktori `database/queries`)
- **Database Migrations**: `goose` (61 migration files terurut di `database/migrations`)
- **In-Memory Cache & Queue**: Redis (`go-redis/v9`) untuk high-traffic queue waiting room dan distributed rate limiter
- **Asynchronous Background Processing**: Native Go routine runners dengan configurable interval tickers di `services/api/cmd/worker`
- **Observability & Metrics**: Prometheus client library (`/metrics` endpoint dengan histogram latensi dan counter transaksi)
- **Logging**: Go `log/slog` dengan structured JSON/text logger

### Frontend Web Utama (`apps/web`)
- **Framework**: Astro v5 (Server-Side Rendering dengan adapter `@astrojs/node`)
- **Styling**: Tailwind CSS v3
- **Komponen & Viewport**: Responsive design berbasis breakpoint Tailwind, high contrast WCAG AA, dan touch target minimum 44px
- **Arsitektur Halaman**:
  - Halaman Publik: `/`, `/events`, `/events/[eventId]`, `/login`, `/register`
  - Portal Peserta: `/participant/dashboard`, `/participant/orders`, `/participant/tickets`, `/participant/certificate/[ticketId]`
  - Konsol Organizer: `/org/[orgId]/dashboard`, `/org/[orgId]/events/[eventId]/*`
  - Platform Super Admin: `/admin/events`, `/admin/events/edit`, `/admin/warroom`, `/admin/status`, `/admin/billing`, `/admin/payments`

### Scanner Mobile PWA (`apps/scanner`)
- **Framework**: Svelte 5 + Vite
- **Offline Storage**: IndexedDB via custom wrapper (`offline-db.ts`)
- **Sync Engine**: Background replay sync dengan exponential backoff (`sync.ts`)
- **Keamanan Tiket**: Verifikasi token QR berbasis signature HMAC-SHA256 tanpa mengekspos secret server

---

## 2. Struktur Modul Utama

Backend tersusun dalam 29 bounded context di dalam [`services/api/internal/modules/`](file:///root/ivyticketing/services/api/internal/modules):

```
services/api/
├── cmd/
│   ├── api/          : HTTP REST API utama (port 8081)
│   ├── webhook/      : Listener callback payment gateway independen
│   └── worker/       : Background processor (order expiry, queue release, notifikasi, export)
└── internal/
    ├── app/          : Assembly router (server.go) & wiring dependensi antar-modul
    ├── db/           : Kode Go hasil generate sqlc
    ├── platform/     : Infrastruktur bersama (audit, authctx, metrics, queue, rbac, storage)
    └── modules/
        ├── auth/           : JWT token, session, login, registrasi
        ├── organizations/  : Multi-tenant organizer & branding
        ├── events/         : Manajemen data master event, lokasi, tanggal, jadwal
        ├── categories/     : Kategori lari (42K, 21K, 10K, 5K), kuota, harga, bib_prefix
        ├── forms/          : Custom dynamic runner form builder (jersey, medis, BIB, kontak darurat)
        ├── inventory/      : Pessimistic locking (SELECT FOR UPDATE anti-oversell)
        ├── queue/          : Waiting room antrean war ticket berbasis Redis
        ├── orders/         : Order lifecycle & state machine (PENDING -> PAID/EXPIRED)
        ├── payments/       : Integrasi payment gateway, webhook parser, & rekonsiliasi
        ├── tickets/        : Penerbitan e-tiket, QR signer, & manajemen nomor BIB
        ├── racepack/       : Slot pengambilan race pack, konter booth, surat kuasa, problem desk
        ├── scanner/        : Verifikasi QR & eksekusi check-in gerbang masuk (VALID -> USED)
        ├── results/        : Hasil lomba, ranking otomatis, & template sertifikat finisher
        ├── ballot/         : Sistem undian pendaftaran lari mayor (seperti Tokyo/London Marathon)
        ├── access/         : Priority registration & corporate invitation pools
        └── notifications/  : Worker email notifikasi transaksional
```

---

## 3. Entity Database yang Ada dan Relasinya

### Skema Relasional Inti

```
[organizations]
       │ 1:N
   [events] ──────────────┐ 1:N
       │ 1:N              │
[event_categories]        ├─► [racepack_counters]
       │ 1:N              │
    [orders]              ├─► [racepack_pickup_slots]
       │ 1:1              │
   [payments]             ├─► [racepack_pickup_records] (link ke ticket_id)
       │ 1:1              │
   [tickets] (bib_number) ├─► [certificate_templates]
       │                  │
       └──────────────────┴─► [race_results] (event_id, bib_number, chip_time, gun_time)
```

1. **[`events`](file:///root/ivyticketing/database/migrations/00008_create_events.sql#L2)**:
   - Dimiliki oleh `organization_id`.
   - Kolom: `name`, `slug`, `event_type`, `status` (`draft`, `published`, `archived`), `venue_name`, `venue_address`, `starts_at`, `ends_at`, `terms`, `waiver`.
2. **[`event_categories`](file:///root/ivyticketing/database/migrations/00009_create_event_categories.sql#L2)**:
   - Terikat ke `event_id`.
   - Kolom: `name`, `price`, `capacity`, `registration_opens_at`, `registration_closes_at`, `bib_prefix` (misal: "FM", "HM", "10K"), `min_age`, `max_order_per_user`.
3. **[`orders`](file:///root/ivyticketing/database/migrations/00012_create_orders.sql#L2)**:
   - Menyimpan `participant_id`, `category_id`, `status` (`PENDING_PAYMENT`, `PAID`, `EXPIRED`, `CANCELLED`), `subtotal`, `fee`, `discount`, `total`, `expired_at`.
4. **[`inventory_reservations`](file:///root/ivyticketing/database/migrations/00013_create_inventory_reservations.sql#L2)**:
   - Mengunci kuota secara real-time selama sesi pembayaran aktif (default 15 menit).
5. **[`tickets`](file:///root/ivyticketing/database/migrations/00018_create_tickets.sql#L2)**:
   - Terbit saat order berstatus `PAID` (relasi 1:1 dengan order via `tickets_order_unique`).
   - Menyimpan `ticket_number` unik, `holder_name`, `holder_email`, `status` (`VALID`, `USED`, `CANCELLED`), `qr_version`, `used_at`.
   - **Kolom BIB** ([`00049_add_tickets_bib_columns.sql`](file:///root/ivyticketing/database/migrations/00049_add_tickets_bib_columns.sql#L2)):
     - `bib_number`: Nomor dada resmi pelari (Unique per event melalui partial index `uniq_tickets_event_bib`).
     - `bib_assigned_at`: Waktu penomoran BIB.
     - `bib_assigned_by`: User admin/panitia yang menetapkan.
     - `bib_assignment_method`: `AUTO`, `MANUAL`, atau `OVERRIDE`.
6. **Logistik Racepack** ([`00050_create_racepack_pickup.sql`](file:///root/ivyticketing/database/migrations/00050_create_racepack_pickup.sql#L1)):
   - `racepack_counters`: Konter/booth pembagian paket lomba per event.
   - `racepack_pickup_slots`: Kuota kapasitas per hari dan jendela jam pengambilan.
   - `racepack_pickup_records`: Bukti pengambilan resmi (`SELF`, `PROXY`, `MANUAL_OVERRIDE`) dengan anti-duplicate guard (`uniq_racepack_pickup_records_ticket_active`).
   - `racepack_proxy_authorizations`: Data dokumen dan kuasa perwakilan pengambilan.
   - `racepack_problem_cases`: Kasus masalah BIB/tiket di meja problem desk (`OPEN`, `UNDER_REVIEW`, `RESOLVED`, `ESCALATED`).
7. **Hasil Lomba & Sertifikat** ([`00061_create_race_results.sql`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql#L10)):
   - `race_results`: Terikat ke `event_id` dan `bib_number` (Unique).
   - Menyimpan demografi lomba: `participant_name`, `gender` (`M`, `F`, `X`), `age`, `age_group`.
   - Waktu dalam integer milidetik: `chip_time_ms` (net time mat-ke-mat) dan `gun_time_ms` (gross time tembakan start).
   - Ranking: `rank_overall`, `rank_gender`, `rank_category`, `rank_age_group`.
   - `status`: `FINISHED`, `DNF`, `DNS`.
   - `source`: `CSV` atau `TIMING_API`.
   - `certificate_templates`: Desain sertifikat kelulusan dinamis berbasis token string.

---

## 4. Alur Kerja: Pendaftaran -> Pembayaran -> E-Tiket

```
[1. Akses Event & Kategori]
            │
            ▼
[2. Registration Gate Check]
    ├── Mode Normal: langsung checkout
    ├── Mode War Queue: verifikasi tiket antrean & admission token dari Redis
    ├── Mode Ballot: verifikasi status pemenang undian (winner grant)
    └── Mode Priority: verifikasi membership / access code
            │
            ▼
[3. Transaksi Pemesanan (Checkout)]
    ├── Pessimistic Lock: inventory.CheckAndLock (SELECT FOR UPDATE)
    ├── Validasi Limit: max_order_per_user
    ├── Buat Record Order: status PENDING_PAYMENT + TTL ExpiredAt
    ├── Buat Inventory Reservation
    └── Konsumsi Admission Token (Mencegah reusable token)
            │
            ▼
[4. Pembayaran (Payment Gateway)]
    ├── Request payment token / redirect URL
    └── Pelari menyelesaikan pembayaran di bank/QRIS/kartu
            │
            ▼
[5. Notifikasi Callback / Webhook]
    ├── Verifikasi signature gateway (HMAC / RSA)
    ├── Simpan raw payload ke payment_webhooks
    └── payments.Processor mengeksekusi transisi status -> PAID (Idempotent)
            │
            ▼ (Atomic Database Transaction)
[6. Penerbitan Tiket (tickets.Issuer)]
    ├── Generate nomor tiket unik
    ├── Insert ke tabel tickets (status VALID)
    ├── Generate QR Code bertanda tangan kriptografis HMAC
    ├── Catat log audit (TICKET_ISSUED)
    └── Enqueue notifikasi email konfirmasi & invoice
```

---

## 5. Bagian yang Sudah Siap untuk Race Management

1. **Alokasi Nomor BIB Pelari**:
   - Backend [`services/api/internal/modules/tickets/bib_handler.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_handler.go):
     - Auto-assign nomor urut BIB berikutnya (`AssignNextBib`).
     - Manual override BIB per tiket (`SetBib`).
     - Bulk auto-assign untuk seluruh tiket valid yang belum memiliki BIB (`BulkAssignBib`).
     - Export data pelari dan nomor BIB ke format CSV (`ExportBibsCSV`) untuk integrasi ke vendor percetakan BIB fisik.
2. **Logistik Pengambilan Racepack (RPC)**:
   - Modul [`racepack`](file:///root/ivyticketing/services/api/internal/modules/racepack/service.go):
     - Pemilihan slot hari dan jam pengambilan oleh peserta.
     - Penjadwalan kuota konter pengambilan di race village.
     - Validasi perwakilan pengambilan dengan dokumen surat kuasa.
     - Manajemen problem desk untuk penanganan BIB tertukar atau rusak.
3. **Pemeriksaan Tiket Masuk (Gate Check-In)**:
   - Modul [`scanner`](file:///root/ivyticketing/services/api/internal/modules/scanner/service.go) dan frontend [`apps/scanner`](file:///root/ivyticketing/apps/scanner):
     - PWA mandiri yang siap digunakan petugas gerbang via ponsel.
     - Mode offline-first berbasis IndexedDB dengan auto-replay saat kembali online.
     - Transisi status tiket `VALID` ke `USED` dengan guard anti-duplikasi ganda.
4. **Hasil Lomba & Finisher Certificate**:
   - Modul [`results`](file:///root/ivyticketing/services/api/internal/modules/results/service.go):
     - Import file hasil lomba CSV dari operator timing.
     - Kalkulasi ranking otomatis (overall, gender, kategori, age group).
     - Unduh e-sertifikat lomba resmi ber-watermark untuk peserta di portal peserta.

---

## 6. Bagian yang Belum Ada untuk RFID Timing & Live Result

Untuk membangun sistem timing berbasis chip RFID mandiri (karpet timing mat start, split, dan finish):

1. **Skema & Relasi RFID Tag (EPC Transponder Mapping)**:
   - Belum ada tabel pemetaan kode fisik chip RFID ke nomor BIB (contoh: EPC tag hex 96-bit yang tertempel di balik nomor dada atau gelang kaki pelari).
2. **Intermediate Checkpoint & Split Timing**:
   - Tabel `race_results` saat ini hanya mencatat satu pasang waktu finish (`chip_time_ms` dan `gun_time_ms`).
   - Belum ada entitas `timing_checkpoints` (misal: Start, 5K, 10K, Half 21.1K, 30K, Finish) dan tabel pencatatan split waktu intermediate `race_split_times`.
3. **Penerima Stream Data RFID (Raw Read Ingestion Engine)**:
   - Belum ada listener real-time (TCP socket, MQTT broker, atau REST ingestion endpoint berlatensi rendah) untuk menerima stream bacaan langsung dari controller RFID box (seperti Impinj, Zebra, RaceResult, atau Alien).
4. **Kalkulasi Pace & Estimasi Waktu (Pace & Predictive Finish)**:
   - Belum ada kalkulator pace menit/km otomatis dan estimasi waktu tiba (ETA) di garis finish berdasarkan kecepatan di split sebelumnya.
5. **Manajemen Wave Start & Gun Time Timestamp**:
   - Belum ada tabel gelombang start (`race_waves`) dengan pencatatan timestamp mikrodetik presisi saat tembakan start dilepas untuk masing-masing kelompok pelari.
6. **Live Leaderboard & Push Channel Realtime**:
   - Hasil saat ini disajikan secara pasif melalui REST API. Belum ada WebSocket atau Server-Sent Events (SSE) channel untuk mem-push pergerakan pelari secara langsung ke leaderboard publik atau dashboard pemantauan panggung finish.
7. **Status Diskualifikasi & Penalti**:
   - Status saat ini terbatas pada `FINISHED`, `DNF`, dan `DNS`. Belum mendukung `DSQ` (Disqualified) dan `OTL` (Over Time Limit / melebihi Cut-Off Time).

---

## 7. File Penting yang Perlu Dipahami Sebelum Implementasi

### A. Database & Query
- [`database/migrations/00049_add_tickets_bib_columns.sql`](file:///root/ivyticketing/database/migrations/00049_add_tickets_bib_columns.sql): Skema kolom BIB pada tabel tiket.
- [`database/migrations/00050_create_racepack_pickup.sql`](file:///root/ivyticketing/database/migrations/00050_create_racepack_pickup.sql): Skema lengkap alur logistik racepack.
- [`database/migrations/00061_create_race_results.sql`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql): Skema hasil waktu dan sertifikat.
- [`database/queries/results.sql`](file:///root/ivyticketing/database/queries/results.sql): Kueri SQL sqlc untuk upsert dan kalkulasi ranking.
- [`database/queries/tickets.sql`](file:///root/ivyticketing/database/queries/tickets.sql): Kueri alokasi dan mutasi nomor BIB.

### B. Bisnis Logika Backend
- [`services/api/internal/modules/tickets/service.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/service.go) & [`bib_handler.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_handler.go): Logika penetapan nomor BIB otomatis dan manual.
- [`services/api/internal/modules/results/service.go`](file:///root/ivyticketing/services/api/internal/modules/results/service.go): Algoritma kalkulasi ranking berdasarkan net time integer milidetik.
- [`services/api/internal/modules/scanner/service.go`](file:///root/ivyticketing/services/api/internal/modules/scanner/service.go): Verifikasi tanda tangan QR tiket dan transisi status check-in.
- [`services/api/internal/modules/orders/service.go`](file:///root/ivyticketing/services/api/internal/modules/orders/service.go): Transaksi checkout dan locking kuota anti-oversell.
- [`services/api/internal/app/server.go`](file:///root/ivyticketing/services/api/internal/app/server.go): Registrasi route chi, dependency injection, dan proteksi middleware.

### C. Frontend & Scanner
- [`apps/web/src/pages/admin/events/edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro): Halaman studio kustomisasi event lengkap (9 tab builder).
- [`apps/web/src/lib/events-store.ts`](file:///root/ivyticketing/apps/web/src/lib/events-store.ts): Data store untuk template lomba, profil rute, elevasi, dan konfigurasi form runner.
- [`apps/scanner/src/lib/offline-db.ts`](file:///root/ivyticketing/apps/scanner/src/lib/offline-db.ts) & [`sync.ts`](file:///root/ivyticketing/apps/scanner/src/lib/sync.ts): Engine pemindaian offline IndexedDB dan sinkronisasi data lapangan.

---

## 8. Catatan Khusus Viewport Halaman Event Kustom

Sesuai observasi audit, halaman event kustom publik (`/events/[eventId]`) dan studio editor admin (`/admin/events/edit`) perlu dipastikan terkunci pada lebar viewport browser (`max-w-full overflow-x-hidden`) agar tidak terjadi pergeseran horizontal (horizontal scrolling) pada layar tablet maupun desktop:
- Pastikan container terluar menggunakan class `w-full max-w-full overflow-x-hidden box-border`.
- Pastikan tabel kategori dan profil elevasi yang lebar memiliki wrapper scrolling internal mandiri (`overflow-x-auto`) tanpa mendesak body browser keluar dari batas layar.
