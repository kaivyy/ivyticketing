# Buku Panduan Operasional Penyelenggara (Organizer Operations Playbook)

Buku panduan ini merinci prosedur standar operasional (SOP) penyelenggaraan event lari menggunakan platform IvyTicketing, mulai dari fase registrasi hingga penyelesaian pasca-lomba.

---

## 1. Fase Pra-Lomba (Pre-Race Setup)

### A. Konfigurasi Kategori & Cut-Off Time
1. Buka menu **Kategori Tiket** di `/org/[orgId]/events/[eventId]/categories`.
2. Tentukan nama kategori (contoh: *Marathon 42K*, *Half Marathon 21K*, *10K Run*).
3. Masukkan spesifikasi teknis lomba:
   - **Jarak (km)**: Diperlukan untuk perhitungan pace rata-rata atlet.
   - **Batas Waktu (Cut-Off Time)**: Format jam dan menit (contoh: `07:00:00` untuk Marathon).
4. Atur tier harga (Early Bird, Regular, Late) dan kuota kapasitas maksimal.

### B. Kustomisasi Formulir Data Atlet
1. Buka menu **Formulir Pendaftaran** di `/org/[orgId]/events/[eventId]/form`.
2. Tambahkan pertanyaan wajib untuk kebutuhan medis dan keselamatan:
   - Golongan Darah (A, B, AB, O).
   - Riwayat Penyakit / Alergi.
   - Kontak Darurat (Nama dan Nomor WhatsApp/Telepon).
   - Ukuran Jersey / Race Tee (XS, S, M, L, XL, XXL).
   - Target Waktu Finish (Estimated Finish Time) untuk penempatan wave start.

### C. Alokasi Nomor BIB Pelari
1. Masuk ke menu **Peserta & Atlet** di `/org/[orgId]/events/[eventId]/participants`.
2. Tinjau peserta yang telah berstatus pembayaran valid.
3. Alokasi BIB dapat dilakukan dengan dua cara:
   - **Manual**: Buka drawer peserta, klik *Ubah Nomor BIB*, dan ketikkan nomor BIB atlet.
   - **Otomatis**: Gunakan tombol *Alokasi BIB Otomatis* untuk memberi nomor secara urut per kategori (contoh: kategori 42K dimulai dari `1001`, 21K dimulai dari `2001`).

---

## 2. Fase Pengambilan Paket Lomba (Race Pack Collection)

### A. Meja Penyerahan Normal
1. Petugas membuka menu **Racepack Pickup** di `/org/[orgId]/events/[eventId]/racepack`.
2. Staf memindai QR Code tiket peserta yang tertera pada email konfirmasi atau dashboard pelari.
3. Sistem memvalidasi keaslian tiket dan menampilkan ukuran jersey yang dipesan.
4. Klik tombol **Serahkan Race Pack**. Status penyerahan tercatat secara permanen di database.

### B. Meja Masalah (Problem Desk)
Jika peserta menghadapi kendala seperti salah ukuran baju, pergantian pelari darurat, atau BIB tertinggal:
1. Buka menu **Peserta & Atlet** di `/org/[orgId]/events/[eventId]/participants`.
2. Cari nama atau email peserta melalui kotak pencarian cepat.
3. Klik pada baris peserta untuk membuka drawer detail.
4. Lakukan koreksi data kontak melalui tombol *Edit Kontak Peserta*, atau perbarui nomor BIB pengganti.

---

## 3. Fase Hari Lomba (Race Day Start & Check-in)

### A. Pengelompokan Gelombang (Race Waves / Corrals)
1. Buka menu **Hasil & Timing** di `/org/[orgId]/events/[eventId]/results`.
2. Pada panel *Gelombang Lomba (Race Waves)*, pastikan seluruh gelombang telah dijadwalkan:
   - **Wave A (Elit & Sub-3)**: Gun Start 05:00:00 WIB.
   - **Wave B (Reguler)**: Gun Start 05:15:00 WIB.
   - **Wave C (Master & Open)**: Gun Start 05:30:00 WIB.
3. Waktu *Gun Start* ini menjadi acuan waktu bruto (*Gun Time*) saat penghitungan klasemen pelari.

### B. Pemindaian Gerbang Start (Race Day Scanner)
1. Petugas gerbang membuka menu **Race Day Scanner** di `/org/[orgId]/events/[eventId]/scanner`.
2. Aktifkan kamera perangkat (smartphone/laptop) atau hubungkan perangkat barcode gun USB.
3. Kamera akan mendeteksi QR code pelari secara instan:
   - **Bunyi Beep Tinggi + Hijau**: Tiket valid, atlet diizinkan memasuki corral start.
   - **Bunyi Beep Rendah + Merah**: Tiket sudah pernah digunakan atau tidak terdaftar.
4. Metrik kehadiran atlet terhitung secara langsung pada kartu penghitung (*Live Counter*).

---

## 4. Fase Pencatatan Waktu & Matras Lintasan (Live Timing)

### A. Titik Matras Checkpoint
Pastikan semua matras sensor timing terdaftar pada sistem di `/org/[orgId]/events/[eventId]/results`:
- `START`: Matras garis start pelari (mencatat waktu Chip Start).
- `SPLIT_5K`, `SPLIT_10K`, `SPLIT_21K`: Matras perantara untuk mencegah kecurangan rute (potong kompas).
- `FINISH`: Matras garis finish akhir.

### B. Penerimaan Telemetri Real-time
Sistem IvyTicketing mendukung integrasi push otomatis protokol telemetri RACE RESULT 12 melalui endpoint:
`POST /api/v1/organizations/{orgId}/events/{eventId}/timing/telemetry`
Data passing transponder diproses dan dicocokkan dengan BIB tiket atlet secara langsung.

---

## 5. Fase Pasca-Lomba (Post-Race & Finance)

### A. Verifikasi Hasil & Rekalkulasi Peringkat
1. Jika pencatatan waktu dilakukan offline, unggah berkas CSV hasil lomba di menu **Hasil & Klasemen**.
2. Klik tombol **Hitung Ulang Peringkat (Recompute Ranks)**.
3. Algoritma server memproses seluruh waktu bersih (*Net Time*) dan waktu tembakan (*Gun Time*), lalu menetapkan:
   - Peringkat Keseluruhan (*Overall Rank*).
   - Peringkat Kategori Gender (*Gender Rank*).
   - Peringkat Kelompok Usia (*Age Group Rank*).
4. Status pelari yang tidak melewati matras lengkap akan otomatis ditandai sebagai `DNF` (Did Not Finish).

### B. Penerbitan Sertifikat Finisher
1. Pelari yang berstatus `FINISHED` dapat langsung mengunduh e-Certificate mereka melalui dashboard peserta atau link:
   `/participant/certificate/[ticketId]`
2. Sertifikat otomatis memuat nama atlet, nomor BIB, catatan waktu finish, ranking kategori, dan pace rata-rata.

### C. Rekonsiliasi Finansial & Permohonan Pencairan Dana (Payout)
1. Buka menu **Tagihan & Payout** di `/org/[orgId]/billing`.
2. Periksa saldo bersih:
   - `Gross Revenue`: Total penjualan tiket lari yang sukses dibayar.
   - `Platform Fee`: Biaya administrasi platform per transaksi tiket.
   - `Refunds`: Pengurangan dana akibat tiket yang dibatalkan.
   - `Available Balance`: Dana bersih yang siap ditarik.
3. Pastikan rekening bank penampung organisasi sudah terdaftar dan terverifikasi.
4. Masukkan nominal penarikan, lalu klik **Ajukan Payout**. Riwayat pencairan dan status transfer dana dapat dipantau di tabel riwayat payout.
