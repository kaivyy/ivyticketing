# Katalog Peran & Izin Akses (RBAC System Architecture)

Dokumen ini merinci arsitektur kendali akses berbasis peran (*Role-Based Access Control* / RBAC) yang diterapkan pada tingkat platform dan organisasi di IvyTicketing.

---

## 1. Prinsip Otorisasi Multi-Tenant

1. **Pemisahan Tenant Mutlak**:
   - Seluruh izin akses organisasi dievaluasi dalam konteks `organization_id` tertentu.
   - Anggota dengan peran *Owner* pada Organisasi A tidak memiliki hak akses apa pun pada Organisasi B.
2. **Platform Admin vs Organizer**:
   - Pengguna dengan tanda `is_platform_admin = true` memiliki hak administratif global lintas tenant untuk kebutuhan pengawasan platform dan audit.
3. **Pemberian Hak Granular**:
   - Setiap endpoint backend diproteksi oleh middleware `RequirePermission(loader, "permission.key")`.

---

## 2. Katalog Hak Izin (Permissions Catalog)

Tabel berikut memuat seluruh kunci izin yang terdaftar pada tabel basis data `permissions`:

| Kunci Izin (*Permission Key*) | Modul Terkait | Deskripsi Fungsional |
|---|---|---|
| `organization.manage` | Organisasi & Pengaturan | Mengubah profil organisasi, melihat riwayat audit trail. |
| `member.manage` | Anggota Tim | Mengundang panitia baru dan mengelola keanggotaan. |
| `role.manage` | Peran Kustom | Membuat dan mengonfigurasi peran baru khusus organisasi. |
| `event.create` | Event Studio | Membuat event lomba baru. |
| `event.edit` | Event Studio | Memperbarui rincian event, lokasi rute, dan banner. |
| `event.publish` | Event Lifecycle | Membuka, mempublikasikan, menutup, atau mengarsipkan event. |
| `event.delete` | Event Lifecycle | Menghapus draft event yang belum memiliki transaksi. |
| `category.manage` | Kategori Lomba | Mengelola kategori lomba, kuota, tier harga, dan cut-off time. |
| `form.manage` | Formulir Atlet | Menambah dan mengubah kolom formulir kustom atlet. |
| `participant.view` | Peserta & Atlet | Melihat daftar atlet dan detail biodata peserta. |
| `participant.export` | Peserta & Laporan | Mengunduh ekspor data peserta lengkap ke berkas CSV. |
| `bib.manage` | Manajemen BIB | Menetapkan nomor BIB secara manual atau otomatis. |
| `racepack.scan` | Pengambilan Paket | Memindai dan menyerahkan race pack kepada pelari. |
| `racepack.manage` | Pengambilan Paket | Mengonfigurasi lokasi dan jadwal race pack collection. |
| `order.view` | Pesanan & Tiket | Melihat daftar pesanan tiket masuk. |
| `order.refund` | Keuangan & Pesanan | Membatalkan pesanan dan memproses refund tiket. |
| `payment.view` | Keuangan & Billing | Melihat histori transaksi pembayaran dan saldo tiket. |
| `payment.refund` | Keuangan & Billing | Menyetujui pengembalian dana pembayaran. |
| `payment.manage` | Keuangan & Payout | Mendaftarkan rekening bank dan mengajukan payout. |
| `broadcast.send` | Komunikasi | Mengirim broadcast pengumuman massal ke email pelari. |
| `results.manage` | Timing & Hasil | Mengimpor CSV waktu, membuat wave, dan menghitung ranking. |
| `checkin.execute` | Gerbang Start | Melakukan verifikasi dan check-in hari lomba via scanner. |
| `branding.manage` | Desain & Domain | Mengonfigurasi warna brand, logo kustom, dan domain. |
| `report.view` | Laporan & Analitik | Melihat grafik agregat penjualan, konversi, dan laporan. |

---

## 3. Peran Standar Sistem (System Roles)

Saat sebuah organisasi baru dibuat, sistem secara otomatis mereplikasi template peran bawaan berikut ke dalam lingkup organisasi tersebut:

### 1. Owner (Pemilik Organisasi)
- **Cakupan Hak**: Memiliki **seluruh izin** yang tersedia pada katalog tanpa terkecuali.
- **Tanggung Jawab**: Direktur lomba (*Race Director*) atau penanggung jawab utama institusi.

### 2. Manager (Manajer Operasional Lomba)
- **Cakupan Hak**:
  `event.create`, `event.edit`, `event.publish`, `event.delete`,
  `category.manage`, `form.manage`, `participant.view`, `participant.export`,
  `order.view`, `report.view`, `broadcast.send`, `coupon.manage`,
  `bib.manage`, `results.manage`, `checkin.execute`
- **Tanggung Jawab**: Manajemen operasional teknis lomba dan komunikasi peserta.

### 3. Finance (Manajer Keuangan)
- **Cakupan Hak**:
  `order.view`, `order.refund`, `payment.view`, `payment.refund`,
  `payment.manage`, `report.view`
- **Tanggung Jawab**: Rekonsiliasi transaksi tiket, penanganan pembatalan dana, dan pengajuan payout.

### 4. Customer Service / Problem Desk
- **Cakupan Hak**:
  `participant.view`, `order.view`, `bib.manage`
- **Tanggung Jawab**: Penanganan kendala peserta di meja bantuan (problem desk), perbaikan kontak peserta.

### 5. Racepack Staff
- **Cakupan Hak**:
  `racepack.scan`, `racepack.manage`, `participant.view`
- **Tanggung Jawab**: Petugas di gerai pengambilan jersey dan paket lomba.

### 6. Timing & Gate Operator
- **Cakupan Hak**:
  `results.manage`, `checkin.execute`, `participant.view`
- **Tanggung Jawab**: Petugas pencatat waktu di matras sensor dan pemeriksa tiket di gerai start corral.

---

## 4. Mekanisme Verifikasi Middleware

Implementasi otorisasi di dalam kode Go terletak pada berkas [`services/api/internal/platform/middleware/authz.go`](file:///root/ivyticketing/services/api/internal/platform/middleware/authz.go):

```go
// 1. Ekstraksi orgId dari URL chi router (mendukung UUID maupun slug organisasi).
// 2. Jika orgId berbentuk slug, resolusikan ke UUID tabel organizations.
// 3. Muat permissions anggota organisasi dari basis data (dengan cache per request).
// 4. Periksa apakah permission yang dibutuhkan ada di dalam map permissions pengguna.
// 5. Jika tidak terpenuhi, kembalikan respon HTTP 403 Forbidden.
```
