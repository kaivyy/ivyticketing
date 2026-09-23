# Arsitektur & Panduan Portal Organizer IvyTicketing

Dokumen ini menjelaskan arsitektur, hierarki routing, alur kerja, dan fitur-fitur yang tersedia di dalam portal Organizer IvyTicketing.

---

## 1. Ikhtisar Arsitektur

Portal Organizer dirancang untuk mendukung operasional penyelenggaraan lomba lari modern (marathon, half-marathon, 10K, trail run) dari tahap persiapan hingga pasca-lomba (race day & post-race analysis).

```
+-----------------------------------------------------------------------------+
|                             PORTAL ORGANIZER                                |
|                                                                             |
|  [Pra-Lomba]                [Hari Lomba / Race Day]    [Pasca-Lomba]         |
|  - Event Studio             - Race Day Scanner         - Hasil & Klasemen   |
|  - Kategori & Kuota         - Check-in Gerbang Start   - Recompute Rank     |
|  - Form Khusus Atlet        - Telemetri RFID           - Sertifikat Finisher|
|  - Nomor BIB                - Live Timing Mat          - Laporan Finansial  |
|  - Gelombang (Waves)        - Problem Desk             - Permohonan Payout  |
+-----------------------------------------------------------------------------+
```

### Prinsip Utama Sistem
1. **Single Source of Truth**:
   - Tiket dan peserta tersimpan terpusat di tabel `tickets`.
   - Nomor BIB tersimpan langsung pada kolom `tickets.bib_number`.
   - Jawaban formulir pendaftaran tersimpan di `tickets.form_answers` dan `orders.form_answers`.
2. **Isolasi Multi-Tenant**:
   - Setiap query data dibatasi oleh `organization_id`.
   - Middleware otorisasi memverifikasi keanggotaan dan izin RBAC sebelum eksekusi handler.
3. **Dukungan Slug dan UUID**:
   - URL portal organizer mendukung format slug ramah pengguna (contoh: `/org/ivy-sports`) maupun UUID (contoh: `/org/00000000-0000-0000-0000-000000000001`).

---

## 2. Peta Rute Halaman Web (Frontend)

Semua halaman organizer terletak di dalam [`apps/web/src/pages/org/[orgId]/`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/):

| Rute URL | Berkas Komponen | Deskripsi & Fungsi |
|---|---|---|
| `/org/[orgId]/dashboard` | [`dashboard.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/dashboard.astro) | Ringkasan metrik penjualan, kuota event aktif, dan aksi cepat. |
| `/org/[orgId]/events` | [`events/index.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/index.astro) | Daftar seluruh event yang dikelola organisasi. |
| `/org/[orgId]/events/new` | [`events/new.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/new.astro) | Pembuatan event baru dengan template lari (Marathon, Half Marathon, 10K, Fun Run). |
| `/org/[orgId]/events/[eventId]` | [`events/[eventId]/index.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/index.astro) | Pusat kendali (Event Studio) untuk mengelola satu event spesifik. |
| `/org/[orgId]/events/[eventId]/categories` | [`events/[eventId]/categories.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/categories.astro) | Manajemen kategori lomba, kuota, tier harga, jarak km, dan cut-off time. |
| `/org/[orgId]/events/[eventId]/participants` | [`events/[eventId]/participants.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/participants.astro) | Direktori atlet, pencarian langsung, drawer profil medis/darurat, refund, dan nomor BIB. |
| `/org/[orgId]/events/[eventId]/scanner` | [`events/[eventId]/scanner.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/scanner.astro) | Pemindai check-in barcode/QR kamera web dan barcode scanner gun dengan feedback audio. |
| `/org/[orgId]/events/[eventId]/results` | [`events/[eventId]/results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/results.astro) | Hasil lomba, impor CSV timing, live ranking, konfigurasi race waves, dan titik matras (checkpoints). |
| `/org/[orgId]/events/[eventId]/reports` | [`events/[eventId]/reports.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/events/%5BeventId%5D/reports.astro) | Laporan agregat penjualan tiket, kupon, dan ekspor data peserta CSV. |
| `/org/[orgId]/notifications` | [`notifications.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/notifications.astro) | Broadcast pengumuman massal ke email pelari dan status notifikasi sistem. |
| `/org/[orgId]/billing` | [`billing.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/billing.astro) | Rekapitulasi saldo tiket bersih, rekening pencairan bank, dan penarikan dana (payout). |
| `/org/[orgId]/settings` | [`settings.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/settings.astro) | Profil organisasi dan riwayat audit aktivitas tim operasional (Audit Trail). |
| `/org/[orgId]/members` | [`members.astro`](file:///root/ivyticketing/apps/web/src/pages/org/%5BorgId%5D/members.astro) | Manajemen tim panitia dan penetapan peran RBAC. |

---

## 3. Peta Endpoint Backend API

Semua rute backend terdaftar di [`services/api/internal/app/server.go`](file:///root/ivyticketing/services/api/internal/app/server.go) di bawah prefix `/api/v1/organizations/{orgId}` dan alias `/api/v1/org/{orgId}`:

### Modul Peserta & Tiket
- `GET /organizations/{orgId}/events/{eventId}/tickets`: Mengambil seluruh tiket/peserta event.
- `PUT /organizations/{orgId}/events/{eventId}/tickets/{ticketId}/participant`: Memperbarui data kontak dan form atlet.
- `POST /organizations/{orgId}/events/{eventId}/tickets/bib`: Menetapkan atau menghapus nomor BIB pelari.
- `POST /organizations/{orgId}/events/{eventId}/tickets/bib/auto`: Penomoran BIB otomatis berurutan sesuai kategori.

### Modul Check-in & Scanner Hari Lomba
- `POST /scan/verify`: Memvalidasi keabsahan tiket QR tanpa mengubah status tiket.
- `POST /scan/check-in`: Melakukan check-in fisik pelari (transisi status VALID -> USED secara idempoten).

### Modul Hasil Lomba & Timing
- `GET /organizations/{orgId}/events/{eventId}/timing/waves`: Mengambil daftar gelombang lomba (wave).
- `POST /organizations/{orgId}/events/{eventId}/timing/waves`: Membuat gelombang lomba baru.
- `GET /organizations/{orgId}/events/{eventId}/timing/checkpoints`: Mengambil daftar titik matras timing.
- `POST /organizations/{orgId}/events/{eventId}/timing/checkpoints`: Menambahkan titik matras baru.
- `POST /organizations/{orgId}/events/{eventId}/timing/telemetry`: Endpoint penerima push data RFID timing.
- `POST /organizations/{orgId}/events/{eventId}/results/import`: Impor data hasil lomba via CSV.
- `POST /organizations/{orgId}/events/{eventId}/results/recompute`: Rekalkulasi peringkat overall, gender, dan age group.

### Modul Komunikasi & Broadcast
- `GET /organizations/{orgId}/broadcast/preview`: Menghitung estimasi jumlah penerima broadcast aktif.
- `POST /organizations/{orgId}/broadcast`: Mengirim broadcast email massal ke seluruh peserta atau per kategori.

### Modul Keuangan & Payout
- `GET /organizations/{orgId}/billing/balance`: Menghitung omset kotor, potongan platform fee, refund, dan saldo bersih siap tarik.
- `GET /organizations/{orgId}/billing/payout-accounts`: Daftar rekening bank penampung pencairan.
- `POST /organizations/{orgId}/billing/payout-accounts`: Mendaftarkan rekening bank baru.
- `DELETE /organizations/{orgId}/billing/payout-accounts/{accountId}`: Menghapus rekening bank.
- `GET /organizations/{orgId}/billing/payouts`: Riwayat permohonan pencairan dana.
- `POST /organizations/{orgId}/billing/payouts`: Mengajukan permohonan pencairan dana tiket.

### Modul Audit Trail
- `GET /organizations/{orgId}/audit-logs`: Menampilkan catatan log audit aktivitas anggota organisasi.
