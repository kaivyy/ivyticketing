# IvyTicketing Organizer Audit

## Executive Summary

Audit ini dilakukan secara menyeluruh terhadap seluruh modul **Organizer / Event Management** pada repositori IvyTicketing. Tujuannya adalah memverifikasi apakah platform ini sudah memenuhi standar platform manajemen lomba atletik dan marathon internasional modern (sekelas RunSignup, ActiveWorks, Race Roster, World Marathon Majors), bukan sekadar aplikasi penjualan tiket konser/seminar.

### Ringkasan Kondisi Sistem

1. **Fondasi Core Engine Sangat Kuat**:
   Backend IvyTicketing memiliki arsitektur berperforma tinggi yang matang:
   - Queue engine token-bucket anti-lonjakan (*traffic war*) dengan Redis.
   - Sistem undian kuota transparan (*ballot engine*) dengan seed deterministik dan verifikasi hash sha256.
   - Pengelolaan nomor dada BIB (*auto-assignment*, prefix kategori, format export percetakan/chip).
   - Racepack Collection (RPC) lengkap dengan manajemen loket (*counters*), reservasi slot jadwal temu pelari, dan eskalasi *problem desk*.
   - Integrasi telemetri timing RFID real-time (HTTP push receiver untuk RACE RESULT 12, mapping transponder chip, rekalkulasi *gun time* dan *chip time*).
   - Generator sertifikat *finisher* berbasis CSS print dinamis.
   - Worker background asinkron (6 job loops) dan pelaporan finansial ekspor CSV.

2. **Temuan Kritis (Gaps & Mismatches)**:
   Meskipun backend engine sangat kaya, terdapat beberapa celah arsitektural dan keterputusan integrasi penting:
   - **Broken Route Mounting di Backend**: Rute organizer untuk modul `ballot` dan `access/corporate` terdaftar di dalam subrouter `events/{eventId}` yang menghasilkan duplikasi path URL (`/organizations/{orgId}/events/{eventId}/org/{orgId}/...` dan `/access/corporate` terkurung di level event, padahal corporate adalah entitas organisasi).
   - **UUID vs Slug Mismatch pada Authorization Middleware**: `RequirePermission` mewajibkan parameter `orgId` berupa valid UUID (`uuid.Parse`). Ketika frontend memanggil menggunakan slug organisasi (contoh: `ivy-sports`), middleware menolak dengan `400 INVALID_ORG_ID`.
   - **Runtime Script Crash pada 6 Halaman Organizer**: File Astro seperti `categories.astro`, `form.astro`, `index.astro`, `payments.astro`, `members.astro`, dan `settings.astro` menggunakan tag `<script define:vars={{ ... }}>` bersamaan dengan ES module `import`, yang di browser menghasilkan error fatal `Uncaught SyntaxError: Cannot use import statement outside a module`, menyebabkan halaman macet pada status "Memuat data...".
   - **Absennya Halaman Manajemen Peserta Terdedikasi**: Organizer hanya memiliki halaman `tickets.astro` sederhana. Tidak ada pencarian pelari, filter multi-kriteria, edit profil/kontak darurat, transfer tiket, ganti kategori (upgrade/downgrade), atau penundaan lomba (*deferral*).
   - **Data Kustom Atlet Tidak Tersimpan ke Database**: Modul Form Builder memungkinkan organizer membuat pertanyaan khusus (ukuran jersey, riwayat medis, kontak darurat, golongan darah), tetapi modul checkout `orders` tidak menyimpan *answers* tersebut ke dalam tabel PostgreSQL `orders` atau `tickets`. Data kustom hanya tersimpan di `localStorage` browser peserta.
   - **Absennya Antarmuka Scanner / Check-in Hari-H**: Backend telah memiliki endpoint verifikasi QR HMAC dan pencatatan check-in (`/scan/verify` dan `/scan/check-in`), namun tidak ada halaman antarmuka kamera scanner web untuk petugas gate di lapangan.
   - **Absennya Mesin Refund & Payout Finansial**: Hak akses `order.refund` dan `payment.refund` terdaftar pada RBAC, namun belum ada endpoint API maupun antarmuka organizer untuk memproses refund penuh/parsial, rekonsiliasi pengembalian dana, maupun *payout* pencairan dana tiket ke rekening bank organizer.
   - **Gelombang Lomba (Waves/Corrals) & Titik Timing (Checkpoints) Masih Backend-Only**: Tabel database dan API backend sudah mendukung wave dan split timing, tetapi belum memiliki antarmuka di portal organizer.

---

## Existing Architecture

Arsitektur IvyTicketing dibangun dengan pemisahan tanggung jawab yang rapi:

```
+-------------------------------------------------------------------------------+
|                                IvyTicketing Web                                |
|   (Astro 4 SSR, Tailwind CSS, TypeScript Modules, Responsive Viewports)        |
+---------------------------------------+---------------------------------------+
                                        | HTTP / JSON (Authed via Bearer JWT)
+---------------------------------------v---------------------------------------+
|                              IvyTicketing API                                 |
|      (Go Chi Router, Clean Architecture, Role-Based Access Control)           |
+-------------------+-------------------+-------------------+-------------------+
| Modules:          |                   |                   |                   |
| - events          | - orders          | - ballot          | - results/timing  |
| - categories      | - payments        | - access/corps    | - racepack        |
| - forms           | - tickets         | - lifecycle       | - scanner         |
| - queue           | - reporting       | - billing         | - enterprise      |
+-------------------+-------------------+-------------------+-------------------+
          |                                       |                     |
+---------v----------+                 +----------v----------+ +--------v-------+
|  PostgreSQL 16     |                 |  Redis 7            | | Worker Daemon  |
|  (71 Public Tables)|                 |  (Rate limits,      | | (6 background  |
|  Strict Foreign    |                 |   Queue buckets,    | |  cron jobs)    |
|  Keys & Checks)    |                 |   Token TTL)        | +----------------+
+--------------------+                 +---------------------+
```

### Identifikasi Sumber Kebenaran Data (Source of Truth)

1. **Tiket & Kepesertaan Atlet**:
   - Sumber kebenaran tunggal di backend: Tabel `tickets` yang mereferensikan `orders(id)`, `events(id)`, `event_categories(id)`, dan `users(id)`.
   - Tabel turunan yang menunjuk ke `tickets`: `race_results`, `racepack_pickup_records`, `racepack_problem_cases`, dan `racepack_proxy_authorizations`.
2. **Nomor BIB**:
   - Sumber kebenaran utama: Kolom `tickets.bib_number` (dengan constraint unik `uniq_tickets_event_bib` per event).
   - Catatan arsitektur: Tabel `race_results` juga memiliki kolom `bib_number`. Hasil timing CSV mencocokkan nomor BIB ke `tickets.bib_number`.
3. **Katalog Event**:
   - Di backend: Tabel `events` dan `event_categories`.
   - Di frontend: Terdapat layer `events-store.ts` di `localStorage` (`ivy_platform_marathon_events`). Kustomisasi layout visual disimpan secara hibrida ke backend API (`PUT /organizations/{orgId}/events/{eventId}`) dan `localStorage`.

---

## Capability Matrix

Tabel perbandingan kapabilitas modul Organizer IvyTicketing:

| Area | Existing Feature | Backend | API | Frontend | DB | Permission | Status | Gap File / Module |
| :--- | :--- | :---: | :---: | :---: | :---: | :---: | :---: | :--- |
| **Organizer Dashboard** | Metrik ringkasan event, omset kotor, total peserta, dan status antrean | Ya | Ya | Ya | Ya | `report.view` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/dashboard.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/dashboard.astro) |
| **Event Studio / Kustomisasi** | Studio visual tata letak hero, bentuk tombol, palet warna lomba, toggle seksi | Ya | Ya | Ya | Ya | `event.edit` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/events/[eventId]/customize.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/customize.astro) |
| **Participant Directory** | Pencarian peserta, filter status/kategori, detail formulir, dan ekspor | Parsial | Parsial | UI Sederhana | Ya | `participant.view` | **PARTIAL** | Belum ada halaman dedicated `participants.astro`; hanya ada tabel tiket dasar di `tickets.astro` |
| **Participant Actions** | Edit profil, ganti kategori (upgrade/downgrade), transfer tiket ke pelari lain, deferral | Tidak | Tidak | Tidak | Ya | `ticket.view` | **MISSING** | Tidak ada endpoint transfer/deferral di [`services/api/internal/modules/tickets/`](file:///root/ivyticketing/services/api/internal/modules/tickets/) |
| **Kategori Lomba** | Tambah/ubah nama kelas lomba, kuota slot, batas usia, dan awalan BIB | Ya | Ya | Ya | Ya | `category.manage` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/events/[eventId]/categories.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/categories.astro) |
| **Pricing Tiers** | Penjadwalan harga bertingkat (Early Bird, Regular, Late) otomatis berbasis tanggal/kuota | Parsial | Tidak | Mock/Store | Tidak | `category.manage` | **UI ONLY** | DB `event_categories` hanya memiliki 1 kolom `price`; belum ada tabel `category_price_tiers` |
| **Antrean Tiket War** | Buka/jeda antrean, atur laju rilis per menit, live monitoring throughput | Ya | Ya | Ya | Ya | `queue.manage` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/events/[eventId]/queue-controls.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/queue-controls.astro) |
| **Sistem Ballot (Undian)** | Pengundian acak, kuota pemenang, konversi tiket, auto-expire pemenang, waitlist promotion | Ya | Ya (Salah Mount) | Parsial | Ya | `ballot.manage` | **BROKEN / PARTIAL** | Rute backend `ballot` salah mount di [`services/api/internal/modules/ballot/routes.go`](file:///root/ivyticketing/services/api/internal/modules/ballot/routes.go) |
| **Form Builder (Kustom Pertanyaan)** | Desain pertanyaan khusus (jersey, kontak darurat, riwayat medis, checkbox, dropdown) | Ya | Ya | Ya | Ya | `form.manage` | **PARTIAL** | Form dibuat dan divalidasi, namun jawaban checkout tidak disimpan ke tabel DB `orders`/`tickets` |
| **Waiver & Terms Lomba** | Klausul persetujuan terms & waiver atlet saat checkout | Ya | Ya | Ya | Ya | `order.create` | **COMPLETE** | Kolom `terms_accepted_at` dan `waiver_accepted_at` di tabel `orders` |
| **Corporate & Komunitas** | Alokasi kuota grup, kode akses khusus, upload data rombongan, invoice tagihan | Ya | Ya (Salah Mount) | Ya | Ya | `access.manage` | **BROKEN / PARTIAL** | Rute `/access/corporate` terkurung di level sub-event di [`services/api/internal/modules/access/routes.go`](file:///root/ivyticketing/services/api/internal/modules/access/routes.go) |
| **Nomor Dada (BIB)** | Penomoran otomatis, override manual, clear BIB, bulk assignment, ekspor CSV percetakan | Ya | Ya | Ya | Ya | `bib.manage` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/events/[eventId]/tickets.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/tickets.astro) |
| **Racepack Collection (RPC)** | Manajemen loket counter, slot jadwal temu atlet, live monitoring pickup, problem desk | Ya | Ya | Ya | Ya | `racepack.manage` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/events/[eventId]/racepack-dashboard.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/racepack-dashboard.astro) |
| **Check-in / Gate Scanner** | Scan QR tiket hari-H, verifikasi status, pencegahan tiket ganda, check-in audit | Ya | Ya | Tidak | Ya | `checkin.execute` | **BACKEND ONLY** | Backend scanner siap di [`services/api/internal/modules/scanner/`](file:///root/ivyticketing/services/api/internal/modules/scanner/), belum ada UI kamera scanner |
| **Gelombang Start (Waves/Corrals)** | Pengelompokan pelari (Wave A, B, C), waktu start wave, mapping wave ke BIB/tiket | Ya | Ya | Tidak | Ya | `timing.manage` | **BACKEND ONLY** | Tabel `race_waves` dan API backend ada, tetapi tidak ada UI di portal organizer |
| **Titik Matras (Checkpoints)** | Titik start, 5K, 10K, 21K, finish split, estimasi jarak dan transponder aliases | Ya | Ya | Tidak | Ya | `timing.manage` | **BACKEND ONLY** | Tabel `timing_checkpoints` dan API backend ada, belum ada UI di portal organizer |
| **Integrasi RFID Timing** | Receiver telemetri push real-time RACE RESULT 12, mapping BIB transponder, rekalkulasi | Ya | Ya | Ya | Ya | `results.manage` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/results.astro) |
| **Hasil & Klasemen (Results)** | Impor manual CSV, perhitungan rank overall, gender, age group, status DNF/DSQ/DNS | Ya | Ya | Ya | Ya | `results.manage` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/results.astro) |
| **Sertifikat Finisher** | Template sertifikat dinamis dengan variabel atlet, gambar latar kustom, CSS print | Ya | Ya | Ya | Ya | `results.manage` | **COMPLETE** | [`apps/web/src/pages/participant/certificate/[ticketId].astro`](file:///root/ivyticketing/apps/web/src/pages/participant/certificate/%5BticketId%5D.astro) |
| **Komunikasi & Broadcast** | Notifikasi transaksional otomatis, email broadcast kustom ke seluruh pelari | Parsial | Parsial | UI Statis | Ya | `broadcast.send` | **PARTIAL** | Backend memiliki email service; frontend `notifications.astro` hanya menampilkan daftar statis |
| **Laporan & Ekspor CSV** | Agregat Penjualan, Peserta, Kupon, Pembayaran, Antrean, Ballot, dan download CSV | Ya | Ya | Ya | Ya | `report.view` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/events/[eventId]/reports.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/reports.astro) |
| **Manajemen Tim & RBAC** | Undang anggota tim, tetapkan peran (Owner, Manager, Finance, Staff, Timing Operator) | Ya | Ya | Ya | Ya | `member.manage` | **COMPLETE** | [`apps/web/src/pages/org/[orgId]/members.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/members.astro) |
| **Audit Trail Organizer** | Riwayat log perubahan konfigurasi, penetapan BIB, pembatalan pesanan | Ya | Tidak | Tidak | Ya | `organization.manage` | **BACKEND ONLY** | Tabel `audit_logs` mencatat semua aksi, namun belum ada halaman visual audit log organizer |
| **Refund & Pembatalan** | Refund penuh/parsial, pembatalan pesanan, pengembalian slot kuota | Tidak | Tidak | Tidak | Parsial | `order.refund` | **MISSING** | Belum ada implementasi refund handler di backend gateway pembayaran |
| **Pencairan Dana (Payout)** | Rekonsiliasi rekening bank penampung, request payout pendapatan tiket | Tidak | Tidak | Tidak | Tidak | `payment.manage` | **MISSING** | Tidak ada tabel atau endpoint disbursement/payout untuk organizer |
| **Lifecycle Multi-Fase** | Penjadwalan fase otomatis (Priority Access -> Ballot -> War Queue -> Closed) | Ya | Tidak Dimount | Tidak | Ya | `registration.manage` | **BACKEND ONLY** | Service Go `lifecycle` dibuat, tetapi routernya tidak dimount di `server.go` |

---

## Participant Management

### Kondisi Saat Ini
Saat ini pengelolaan peserta hanya diwakili oleh halaman [`tickets.astro`](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/tickets.astro):
- Menampilkan tabel datar: Nomor BIB, Nomor Tiket, Nama Pemegang, Email, Kategori, Status Tiket, dan tombol aksi BIB (Assign, Ubah, Hapus).
- Bulk action yang tersedia hanyalah: *Bulk Assign BIB* dan *Export CSV*.

### Kebutuhan Standar Race Management yang Belum Terpenuhi
1. **Search & Multi-Filter**: Belum ada filter nama, email, nomor BIB, status pembayaran, gender, atau kelompok umur.
2. **Detail Profil Atlet**: Organizer tidak dapat mengklik nama peserta untuk melihat data formulir pendaftaran (ukuran baju jersey, nomor kontak darurat, riwayat penyakit, golongan darah, nama klub lari).
3. **Koreksi Data Peserta**: Tidak ada fitur bagi organizer atau *customer service* untuk membetulkan kesalahan ketik nama atau email atlet.
4. **Perubahan Kategori (Category Transfer)**: Atlet yang ingin pindah dari kategori Half Marathon ke Full Marathon (atau sebaliknya) tidak memiliki alur perpindahan resmi, penyesuaian selisih biaya, atau update kuota antar kategori.
5. **Transfer Kepesertaan (Person-to-Person Transfer)**: Pemindahtanganan tiket resmi antar atlet belum didukung (saat ini tiket terikat paten pada pemesan pertama).
6. **Penundaan Lomba (Deferral)**: Tidak ada status atau mekanisme untuk memindahkan slot peserta ke event tahun berikutnya saat pelari mengalami cedera.

---

## Registration

### Analisis Alur Pendaftaran & Keamanan Kuota
1. **Pencegahan Overselling & Race Condition**:
   - Sistem menggunakan tabel `inventory_reservations` dengan status TTL pendek (15 menit) saat checkout.
   - Pengecekan kuota dikunci secara transaksional di database (`services/api/internal/modules/orders/service.go`) dan disinkronkan dengan antrean Redis. Kuota tidak dapat ditembus oleh serangan konkurensi tinggi.
2. **Kategori Pendaftaran**:
   - Backend mendukung mode pendaftaran: `NORMAL`, `WAR_QUEUE`, `RANDOMIZED_QUEUE`, `HYBRID_QUEUE`, `BALLOT`, `INVITATION_ONLY`, `PRIORITY_ACCESS`, `WAITLIST_ONLY`, dan `CLOSED`.
   - Namun, modal pembuatan kategori di portal organizer (`categories.astro`) hanya mengekspos 3 pilihan: `WAR_QUEUE`, `BALLOT`, dan `NORMAL`.
3. **Pricing Tiers (Harga Bertingkat)**:
   - Standar lomba lari umumnya memiliki *Early Bird*, *Normal/Standard*, dan *Late Registration*.
   - Saat ini tabel `event_categories` hanya memiliki kolom tunggal `price` (bigint). Perubahan harga harus dilakukan secara manual oleh organizer dengan mengedit kategori di hari tertentu.
4. **Custom Questions & Formulir Pendaftaran**:
   - Modul `forms` di backend dan frontend sangat canggih (mendukung reordering, tipe field dinamis, regex validation, preview).
   - **Celah Utama**: Jawaban formulir (*answers*) yang diisi pelari saat checkout tidak dimasukkan ke dalam parameter pesanan backend (`orders.GuestCheckoutRequest`) sehingga data penting seperti ukuran jersey dan kontak darurat hilang dari database.

---

## Orders & Payments

### Lifecycle Pesanan
- Status pesanan: `DRAFT` -> `PENDING_PAYMENT` -> `PAID` (atau `EXPIRED`, `CANCELLED`, `REFUNDED`).
- Gateway pembayaran: Duitku dan Xendit (metode QRIS, Virtual Account, E-Wallet).
- Callback/Webhook: Diproses melalui server terpisah (`cmd/webhook/main.go`), memverifikasi merchant code/signature, dan mengubah status menjadi `PAID` secara idempoten melalui tabel `payment_webhooks`.
- Pembayaran otomatis memicu penerbitan tiket via `ticketIssuer` dan pencatatan potongan biaya platform ke `platform_fee_ledger`.

### Keterbatasan Finansial Organizer
1. **Refund**:
   - Meskipun peran `Finance` dan `Owner` memiliki permission `order.refund` dan `payment.refund`, tidak ada endpoint controller atau logika integrasi gateway untuk memproses pengembalian dana (baik refund parsial maupun pembatalan tiket).
2. **Payout / Pencairan Hasil Penjualan**:
   - Uang tiket yang masuk melalui payment gateway tidak memiliki modul *payout* atau *disbursement* otomatis ke rekening bank organizer.
   - Tidak ada antarmuka bagi organizer untuk mendaftarkan nomor rekening bank legal, melihat saldo tertahan (*escrow*), maupun mengajukan penarikan dana lomba.

---

## BIB & Race Day

### Manajemen Nomor Dada (BIB)
- Implementasi BIB di [`services/api/internal/modules/tickets/`](file:///root/ivyticketing/services/api/internal/modules/tickets/) sudah sangat baik:
  - Mendukung auto-assign berbasis prefix kategori (contoh: `FM0001`, `HM1001`).
  - Mendukung penentuan manual (override) untuk atlet VIP/Elite.
  - Memiliki proteksi constraint unik di database (`uniq_tickets_event_bib`), mencegah nomor BIB ganda dalam 1 event.
  - Ekspor CSV data BIB siap cetak untuk diserahkan ke vendor percetakan kain/nomor dada.

### Racepack Collection (RPC)
- Modul Racepack di IvyTicketing adalah salah satu modul paling matang:
  - Penyelenggara dapat membuat loket counter (*Counter Loket*) untuk membagi antrean pengambilan paket lomba berdasarkan kategori atau nomor BIB.
  - Slot jadwal temu (*appointment slots*) memungkinkan pelari memilih jam kedatangan di expo untuk mencegah penumpukan massa.
  - Modul *Problem Desk* memungkinkan staf expo mencatat dan menangani kendala seperti ketidaksesuaian ukuran baju, surat kuasa pengambilan, atau BIB hilang.

### Check-in Hari Lomba
- Backend memiliki service scanner (`services/api/internal/modules/scanner/`) yang memvalidasi QR tiket ber-HMAC dan melakukan transisi status tiket dari `VALID` ke `USED`.
- Namun, belum ada halaman antarmuka web khusus bagi petugas pintu start untuk memindai QR menggunakan kamera handphone atau scanner barcode USB.

---

## Timing & Results

### Arsitektur Pipeline Timing
IvyTicketing menggunakan arsitektur timing terintegrasi yang bersih di bawah [`services/api/internal/modules/results/`](file:///root/ivyticketing/services/api/internal/modules/results/):
1. **Provider Ingestion**:
   - Endpoint: `POST /organizations/{orgId}/events/{eventId}/results/timing/passings`
   - Mendukung integrasi HTTP push dari software **RACE RESULT 12 Exporter** atau forwarder data transponder matras.
   - Observasi telemetri mentah disimpan ke tabel `timing_passings` dengan idempotency key.
2. **Transponder Chip Mapping**:
   - Tabel `bib_transponder_mappings` memetakan nomor dada pelari ke kode chip RFID (UHF/Active chip).
   - Mendukung impor file CSV mapping chip di portal organizer.
3. **Perhitungan Waktu Lomba**:
   - Menghitung *Gun Time* (waktu peluit start utama) dan *Chip Time / Net Time* (waktu pelari menginjak matras start hingga matras finish).
   - Menentukan peringkat keseluruhan (*Overall Rank*), peringkat gender (*Gender Rank*), dan peringkat kelompok umur (*Age Group Rank*).
   - Mendukung status hasil atletik resmi: `FINISHED`, `DNF` (*Did Not Finish*), `DNS` (*Did Not Start*), `DSQ` (*Disqualified*), dan `OTL` (*Over Time Limit*).
4. **Impor Manual & Fallback CSV**:
   - Jika organizer menyewa vendor timing lokal yang hanya menyediakan rekapan file CSV akhir, tersedia fitur upload CSV manual yang langsung merekonstruksi klasemen dan menerbitkan sertifikat.
5. **Keselarasan dengan Admin**:
   - Tidak ada duplikasi pipeline antara Admin dan Organizer. Pipeline timing sepenuhnya berada di level event organisasi.

---

## RBAC & Security

### Evaluasi Hak Akses (Role-Based Access Control)
Sistem memiliki 6 peran standar di tabel `roles`:
1. `Owner`: 38 permissions (akses menyeluruh, termasuk billing dan penghapusan event).
2. `Manager`: 24 permissions (pengelolaan teknis event, kategori, form, kuota, antrean, racepack, dan hasil timing).
3. `Finance`: 8 permissions (laporan finansial, rekonsiliasi pembayaran, ekspor penjualan).
4. `Customer Service`: 4 permissions (membaca data tiket, pesanan, dan verifikasi pelari).
5. `Racepack Staff`: 6 permissions (eksekusi loket racepack, scan tiket, dan problem desk).
6. `Timing Operator`: 3 permissions (manajemen timing, mapping transponder chip, dan hasil lomba).

### Temuan Keamanan & Otorisasi
1. **Otorisasi Terbatas di Level Organisasi**:
   - Middleware `RequirePermission` mengecek keanggotaan dan izin di level `organization_id`.
   - Belum ada pembatasan di level event (*Event-Scoped RBAC*). Staf yang diberi peran `Manager` otomatis memiliki akses manajerial ke seluruh event di bawah organisasi tersebut.
2. **Pengecekan IDOR (Insecure Direct Object References)**:
   - Query sqlc di backend konsisten menyertakan klausa `WHERE organization_id = $1 AND event_id = $2`, sehingga organizer tidak dapat membaca atau memanipulasi data organisasi lain.
3. **Audit Trail**:
   - Perubahan penting (publikasi event, penetapan BIB, perubahan antrean, impor hasil) dicatat ke tabel `audit_logs` dengan `actor_user_id` dan payload metadata.

---

## Database

### Evaluasi 71 Tabel Skema PostgreSQL
- **Kekuatan Skema**: Seluruh tabel menggunakan UUIDv4 sebagai primary key, memiliki foreign key dengan aturan `ON DELETE RESTRICT` atau `CASCADE` yang tepat, serta index komposit pada kolom pencarian intensif (`idx_orders_org_event`, `idx_tickets_event`, `uniq_tickets_event_bib`).
- **Anomali & Celah Skema**:
  1. `event_categories`: Tidak memiliki kolom `distance_km` dan `cutoff_time`. Akibatnya, data jarak lomba dan cut-off time yang dimasukkan organizer tidak tersimpan di database relasional.
  2. `event_categories`: Hanya mendukung 1 kolom `price`. Tidak ada skema pendukung *early bird* atau harga bertingkat (*tier pricing*).
  3. `orders`: Tidak memiliki kolom relasi atau JSONB untuk menyimpan jawaban form kustom pendaftaran pelari.
  4. `payments`: Check constraint status `payments_status_check` tidak memiliki nilai `'REFUNDED'`, sehingga status refund tidak konsisten dengan tabel `orders`.

---

## API

### Inventaris Endpoint Kunci Organizer

| Method | Path | Auth | Permission | Fungsi & Keterangan |
| :--- | :--- | :---: | :---: | :--- |
| `GET` | `/organizations/{orgId}/events` | Bearer JWT | `event.edit` | Mengambil seluruh daftar event milik organisasi |
| `POST` | `/organizations/{orgId}/events` | Bearer JWT | `event.create` | Membuat event olahraga baru |
| `GET` | `/organizations/{orgId}/events/{eventId}` | Bearer JWT | `event.edit` | Membaca detail konfigurasi event |
| `PUT` | `/organizations/{orgId}/events/{eventId}` | Bearer JWT | `event.edit` | Memperbarui nama, venue, jadwal, dan layout visual |
| `POST` | `/organizations/{orgId}/events/{eventId}/publish` | Bearer JWT | `event.publish` | Mempublikasikan event ke katalog publik |
| `GET` | `/organizations/{orgId}/events/{eventId}/categories` | Bearer JWT | `category.manage` | Mendapatkan daftar kategori tiket lomba |
| `POST` | `/organizations/{orgId}/events/{eventId}/categories` | Bearer JWT | `category.manage` | Membuat kategori lomba baru beserta kuotanya |
| `GET` | `/organizations/{orgId}/events/{eventId}/tickets` | Bearer JWT | `ticket.view` | Mengambil daftar seluruh tiket yang sudah terbit |
| `POST` | `/organizations/{orgId}/events/{eventId}/tickets/bib/bulk-assign` | Bearer JWT | `bib.manage` | Menetapkan nomor BIB otomatis ke seluruh tiket valid |
| `GET` | `/organizations/{orgId}/events/{eventId}/tickets/bib/export` | Bearer JWT | `bib.manage` | Men-download file CSV nomor BIB untuk vendor cetak |
| `GET` | `/organizations/{orgId}/events/{eventId}/form` | Bearer JWT | `form.manage` | Membaca skema form builder pendaftaran |
| `POST` | `/organizations/{orgId}/events/{eventId}/form/fields` | Bearer JWT | `form.manage` | Menambahkan pertanyaan kustom pendaftaran |
| `GET` | `/organizations/{orgId}/events/{eventId}/reports/{type}/summary` | Bearer JWT | `report.view` | Membaca ringkasan laporan (Sales, Ballot, Peserta, dll) |
| `POST` | `/organizations/{orgId}/reports/exports` | Bearer JWT | `report.export` | Membuat antrean pekerjaan ekspor CSV asinkron |
| `GET` | `/organizations/{orgId}/events/{eventId}/results/timing/config` | Bearer JWT | `results.manage` | Membaca konfigurasi provider timing RFID |
| `POST` | `/organizations/{orgId}/events/{eventId}/results/timing/passings` | Token Auth | Public/Token | Receiver push deteksi chip dari software matras RR12 |
| `POST` | `/organizations/{orgId}/events/{eventId}/results/import` | Bearer JWT | `results.manage` | Mengimpor file CSV rekapan hasil lomba manual |

---

## UI / Mobile

### Audit Antarmuka Organizer (Desktop & Mobile)
1. **Desktop**:
   - Layout panel samping (*OrganizerLayout*) menggunakan hirarki yang jelas antara menu organisasi umum dan menu event aktif.
   - Studio kustomisasi visual (`customize.astro`) dan ringkasan lomba (`index.astro`) dirancang intuitif dengan kartu-kartu aksi cepat.
2. **Mobile (Viewport 390px - 414px Smartphone)**:
   - Sebagian besar halaman utama telah menerapkan `min-h-[44px]` untuk touch target tombol dan layout responsif satu kolom.
   - Tabel tiket (`tickets.astro`) dan laporan (`reports.astro`) telah dilengkapi pembungkus `overflow-x-auto` sehingga tidak membocorkan scroll horizontal ke viewport body utama.
   - Hasil timing (`results.astro`) telah dilengkapi kartu tampilan khusus mobile yang menggantikan tabel desktop saat layar `< 640px`.

---

## Duplicate / Legacy Architecture

1. **Dual Storage Event Katalog**:
   - Terdapat ketergantungan historis pada `events-store.ts` di `localStorage` (`apps/web/src/lib/events-store.ts`) yang berdampingan dengan tabel database PostgreSQL `events`.
   - Halaman publik memeriksa data lokal terlebih dahulu sebelum fallback ke API, sehingga event yang baru dibuat di backend harus disinkronkan secara eksplisit agar tampil sempurna di landing page publik.
2. **Duplikasi Kolom Nomor BIB**:
   - Nomor BIB tercatat di tabel `tickets` (`bib_number`) dan tabel `race_results` (`bib_number`). Jika organizer mengubah nomor BIB di modul tiket pasca-lomba, tabel hasil balap tidak terupdate otomatis kecuali recompute timing dijalankan ulang.

---

## P0 Findings (Kritis / Broken Core Workflow)

### Finding P0-1: Broken Route Mounting pada Modul Ballot Organizer
- **Problem**: Rute API organizer untuk manajemen ballot tidak dapat diakses (menghasilkan 404 Not Found).
- **Evidence**: Di file [`services/api/internal/modules/ballot/routes.go`](file:///root/ivyticketing/services/api/internal/modules/ballot/routes.go#L9), handler mendaftarkan `r.Route("/org/{orgId}", ...)`. Namun di [`services/api/internal/app/server.go`](file:///root/ivyticketing/services/api/internal/app/server.go#L371-L381), fungsi ini dipanggil di dalam `eventHandler.RegisterRoutes` yang jalurnya sudah diawali `/organizations/{orgId}/events/{eventId}`.
- **Impact**: URL yang terbentuk di Chi router menjadi `/organizations/{orgId}/events/{eventId}/org/{orgId}/events/...`, sehingga pemanggilan normal organizer ke fitur undian ballot gagal total.
- **Recommended Fix**: Ubah penamaan rute di `ballot/routes.go` agar relatif terhadap router event yang menaunginya, atau pisahkan pendaftarannya di level organisasi.
- **Relevant Files**:
  - [`services/api/internal/modules/ballot/routes.go`](file:///root/ivyticketing/services/api/internal/modules/ballot/routes.go)
  - [`services/api/internal/app/server.go`](file:///root/ivyticketing/services/api/internal/app/server.go)

### Finding P0-2: Broken Route Mounting pada Modul Corporate Account Organizer
- **Problem**: Modul pendaftaran rombongan korporat / klub tidak dapat diakses.
- **Evidence**: Di file [`apps/web/src/lib/corporate.ts`](file:///root/ivyticketing/apps/web/src/lib/corporate.ts#L29), frontend memanggil `/organizations/${orgId}/access/corporate`. Namun di [`services/api/internal/app/server.go`](file:///root/ivyticketing/services/api/internal/app/server.go#L382), `accessHandler.RegisterOrganizerRoutes` dipasang di dalam router sub-event `events/{eventId}`.
- **Impact**: Backend mengharapkan `/organizations/{orgId}/events/{eventId}/access/corporate`, sehingga pemanggilan frontend menghasilkan error 404.
- **Recommended Fix**: Pindahkan pemanggilan `accessHandler.RegisterOrganizerRoutes` langsung ke router `/organizations/{orgId}` di `server.go`.
- **Relevant Files**:
  - [`services/api/internal/modules/access/routes.go`](file:///root/ivyticketing/services/api/internal/modules/access/routes.go)
  - [`services/api/internal/app/server.go`](file:///root/ivyticketing/services/api/internal/app/server.go)

### Finding P0-3: Kegagalan Script Frontend Akibat `define:vars` dengan ES Import
- **Problem**: Beberapa halaman web konsol organizer macet saat dibuka di browser.
- **Evidence**: Penggunaan sintaks `<script define:vars={{ ... }}>` yang di dalamnya terdapat baris `import { ... }` pada:
  - [`apps/web/src/pages/org/[orgId]/events/[eventId]/categories.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/categories.astro#L202)
  - [`apps/web/src/pages/org/[orgId]/events/[eventId]/form.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/form.astro#L221)
  - [`apps/web/src/pages/org/[orgId]/events/[eventId]/index.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/index.astro#L196)
  - [`apps/web/src/pages/org/[orgId]/events/[eventId]/payments.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/payments.astro#L119)
  - [`apps/web/src/pages/org/[orgId]/members.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/members.astro#L83)
  - [`apps/web/src/pages/org/[orgId]/settings.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/settings.astro#L59)
- **Impact**: Kompilator Astro merender tag `<script>` non-module inline, memicu browser melempar exception `Uncaught SyntaxError: Cannot use import statement outside a module` dan membatalkan seluruh inisialisasi tabel/data.
- **Recommended Fix**: Hapus atribut `define:vars` dan operkan parameter konteks melalui atribut `data-org-id` / `data-event-id` pada elemen DOM HTML.

### Finding P0-4: Jawaban Form Kustom Pendaftaran Atlet Tidak Tersimpan
- **Problem**: Pertanyaan khusus yang dibuat organizer (ukuran jersey, kontak darurat, riwayat medis) hilang dan tidak tersimpan ke database setelah checkout.
- **Evidence**: Di file [`services/api/internal/modules/orders/dto.go`](file:///root/ivyticketing/services/api/internal/modules/orders/dto.go#L26-L35), struct `GuestCheckoutRequest` dan `CreateOrderRequest` tidak memiliki field untuk menerima payload jawaban form schemas.
- **Impact**: Panitia tidak memiliki data ukuran baju pelari dan kontak darurat pada laporan ekspor untuk asuransi atau pengadaan logistik medali/jersey.
- **Recommended Fix**: Tambahkan kolom `form_answers jsonb` pada tabel `orders` dan `tickets`, sertakan validasi schema form saat checkout, dan ekspos data tersebut pada laporan ekspor peserta.
- **Relevant Files**:
  - [`services/api/internal/modules/orders/dto.go`](file:///root/ivyticketing/services/api/internal/modules/orders/dto.go)
  - [`services/api/internal/modules/orders/service.go`](file:///root/ivyticketing/services/api/internal/modules/orders/service.go)
  - [`services/api/internal/db/orders.sql.go`](file:///root/ivyticketing/services/api/internal/db/orders.sql.go)

---

## P1 Findings (Fungsionalitas Utama Belum Lengkap)

### Finding P1-1: Absennya Modul Direktori Peserta (Participant Management)
- **Problem**: Organizer tidak dapat mencari, memfilter, melihat profil lengkap, atau mengedit data kontak peserta.
- **Evidence**: Satu-satunya antarmuka yang menampilkan peserta adalah [`tickets.astro`](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/tickets.astro) yang hanya fokus pada penomoran BIB.
- **Impact**: Layanan pelanggan (*Customer Service*) lomba kesulitan mencari peserta saat ada komplain atau permintaan koreksi nama/email atlet.
- **Recommended Fix**: Bangun halaman `participants.astro` dengan fitur pencarian real-time, filter status pembayaran dan kategori, serta drawer detail data peserta lengkap.

### Finding P1-2: Absennya Antarmuka Scanner Check-in Hari Lomba
- **Problem**: Petugas gate start/expo tidak memiliki aplikasi scanner berbasis browser untuk memindai tiket peserta.
- **Evidence**: Modul backend [`services/api/internal/modules/scanner/`](file:///root/ivyticketing/services/api/internal/modules/scanner/) telah selesai dibuat lengkap dengan verifikasi tanda tangan kriptografi QR, namun tidak ada file antarmuka scanner di `apps/web/src/pages/`.
- **Impact**: Verifikasi check-in kehadiran peserta tidak dapat dilakukan langsung di venue lomba melalui platform web.
- **Recommended Fix**: Buat antarmuka scanner kamera HTML5/WebRTC terintegrasi yang memanggil endpoint `POST /scan/verify` dan `POST /scan/check-in`.

### Finding P1-3: Kolom Jarak Lomba dan Cut-Off Time Tidak Masuk ke Database
- **Problem**: Parameter teknis maraton seperti `distance_km` dan `cutoff_time` yang diinput saat membuat kategori tiket tidak tersimpan ke database.
- **Evidence**: Tabel `event_categories` di [`services/api/internal/db/categories.sql.go`](file:///root/ivyticketing/services/api/internal/db/categories.sql.go) tidak memiliki kolom untuk jarak dan COT.
- **Impact**: Data jarak dan cut-off time hilang saat data diambil dari backend API (hanya ada pada template dummy lokal).
- **Recommended Fix**: Tambahkan migration database untuk menyertakan `distance_km numeric(5,2)` dan `cutoff_time text` pada tabel `event_categories`.

### Finding P1-4: Modul Gelombang Lomba (Race Waves) Belum Memiliki Antarmuka
- **Problem**: Organizer tidak dapat mengatur pembagian wave pelari melalui antarmuka web.
- **Evidence**: Tabel `race_waves` dan endpoint backend `POST /timing/waves` sudah tersedia, tetapi di [`results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro) tidak terdapat tombol atau modal untuk mengelola gelombang lomba.
- **Impact**: Penyelenggara lomba skala besar (>10.000 peserta) tidak dapat membagi pelari ke dalam Wave 1, 2, dan 3 untuk start bertahap.
- **Recommended Fix**: Tambahkan seksi manajemen gelombang (*Race Waves*) di dalam halaman hasil/timing organizer.

---

## P2 Findings (Penyempurnaan Operasional)

### Finding P2-1: Tidak Ada Antarmuka Payout & Rekening Bank Penyelenggara
- **Problem**: Tidak ada sarana bagi penyelenggara untuk melihat akumulasi dana penjualan bersih dan mengajukan pencairan dana tiket.
- **Recommended Fix**: Tambahkan tab pengaturan rekening pencairan pada `billing.astro` atau `settings.astro` serta rekapitulasi saldo bersih setelah dipotong platform fee.

### Finding P2-2: Tidak Ada Fitur Email Broadcast Kustom
- **Problem**: Organizer tidak bisa mengirim pengumuman darurat (misal: perubahan rute cuaca buruk atau panduan pengambilan BIB) ke seluruh peserta terdaftar.
- **Recommended Fix**: Sediakan form broadcast email sederhana di `notifications.astro` yang memanggil `notifSvc.SendCustomBroadcast`.

### Finding P2-3: Audit Trail Organizer Tidak Memiliki Antarmuka Visual
- **Problem**: Organizer tidak dapat menginspeksi log audit aktivitas tim operasional mereka sendiri.
- **Recommended Fix**: Tambahkan tab riwayat aktivitas (*Audit Log*) di menu pengaturan organisasi.

---

## Recommended Implementation Order

Untuk menyempurnakan modul Organizer tanpa merusak fitur yang sudah bekerja, disarankan urutan pengerjaan berikut:

```
+-----------------------------------------------------------------------------------+
| Tahap 1: Perbaikan Stabilitas & Route Mounting (P0)                               |
| - Perbaiki broken route mounting modul ballot & access/corporate di server.go     |
| - Konversi script define:vars bermasalah di 6 halaman Astro organizer             |
| - Dukung pencarian orgId by UUID dan by slug di middleware authz                  |
+-----------------------------------------+-----------------------------------------+
                                          |
+-----------------------------------------v-----------------------------------------+
| Tahap 2: Integrasi Form Pendaftaran Atlet (P0)                                    |
| - Tambahkan kolom form_answers jsonb ke tabel orders & tickets                    |
| - Hubungkan payload checkout peserta agar menyimpan jawaban form kustom           |
| - Tampilkan jawaban kustom pada ekspor CSV laporan peserta                        |
+-----------------------------------------+-----------------------------------------+
                                          |
+-----------------------------------------v-----------------------------------------+
| Tahap 3: Modul Direktori Peserta Dedikasi (P1)                                    |
| - Buat halaman /org/[orgId]/events/[eventId]/participants.astro                   |
| - Tambahkan fitur search, filter status/kategori, dan drawer profil atlet         |
| - Sediakan aksi cepat edit kontak & cetak ulang etiket QR                        |
+-----------------------------------------+-----------------------------------------+
                                          |
+-----------------------------------------v-----------------------------------------+
| Tahap 4: Antarmuka Scanner Check-in Hari Lomba (P1)                               |
| - Buat halaman scanner responsif mobile untuk staf gate start                      |
| - Hubungkan dengan kamera web/barcode reader ke endpoint scanner existing         |
+-----------------------------------------+-----------------------------------------+
                                          |
+-----------------------------------------v-----------------------------------------+
| Tahap 5: Penyempurnaan Teknis Lomba & Gelombang (P1 / P2)                         |
| - Tambahkan kolom distance_km & cutoff_time ke tabel event_categories             |
| - Tampilkan antarmuka pengelolaan Wave / Corrals pelari di modul timing           |
| - Tambahkan fitur broadcast pengumuman resmi ke seluruh pelari                    |
+-----------------------------------------------------------------------------------+
```

---

## Files That Need Changes

Berikut adalah daftar berkas yang teridentifikasi memerlukan penyesuaian:

1. **Routing & Backend Core**:
   - [`services/api/internal/app/server.go`](file:///root/ivyticketing/services/api/internal/app/server.go): Memperbaiki hierarki mounting router `ballot` dan `access/corporate`.
   - [`services/api/internal/modules/ballot/routes.go`](file:///root/ivyticketing/services/api/internal/modules/ballot/routes.go): Menyelaraskan prefix rute undian ballot organizer.
   - [`services/api/internal/modules/access/routes.go`](file:///root/ivyticketing/services/api/internal/modules/access/routes.go): Memindahkan endpoint corporate ke router level organisasi.
   - [`services/api/internal/platform/middleware/authz.go`](file:///root/ivyticketing/services/api/internal/platform/middleware/authz.go): Menambahkan fallback resolusi slug organisasi ketika `uuid.Parse` gagal.

2. **Data Model & Checkout Peserta**:
   - [`services/api/internal/modules/orders/dto.go`](file:///root/ivyticketing/services/api/internal/modules/orders/dto.go): Menambahkan field `Answers map[string]any` pada payload checkout.
   - [`services/api/internal/modules/orders/service.go`](file:///root/ivyticketing/services/api/internal/modules/orders/service.go): Memvalidasi dan menyimpan jawaban form schemas ke database.
   - [`services/api/internal/db/orders.sql.go`](file:///root/ivyticketing/services/api/internal/db/orders.sql.go): Menambahkan query insert dan update kolom form answers.

3. **Frontend Script Stability (Astro Module Fixes)**:
   - [`apps/web/src/pages/org/[orgId]/events/[eventId]/categories.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/categories.astro): Menghapus `define:vars` dan beralih ke module script murni.
   - [`apps/web/src/pages/org/[orgId]/events/[eventId]/form.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/form.astro): Menghapus `define:vars`.
   - [`apps/web/src/pages/org/[orgId]/events/[eventId]/index.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/index.astro): Menghapus `define:vars`.
   - [`apps/web/src/pages/org/[orgId]/events/[eventId]/payments.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/payments.astro): Menghapus `define:vars`.
   - [`apps/web/src/pages/org/[orgId]/members.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/members.astro): Menghapus `define:vars`.
   - [`apps/web/src/pages/org/[orgId]/settings.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/settings.astro): Menghapus `define:vars`.

4. **Halaman & Komponen Baru yang Direkomendasikan**:
   - `apps/web/src/pages/org/[orgId]/events/[eventId]/participants.astro`: Halaman direktori dan manajemen peserta terdedikasi.
   - `apps/web/src/pages/org/[orgId]/events/[eventId]/scanner.astro`: Halaman scanner kamera web untuk verifikasi dan check-in hari lomba.
