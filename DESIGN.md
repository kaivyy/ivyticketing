# DESIGN.md

> Panduan arah desain dan identitas visual untuk IvyTicketing.
> Dokumen ini menyediakan fondasi visual untuk UI: antislop berperan sebagai filter, dan dokumen ini adalah sumber arah (direction).

---

## 1. Identitas & Audiens Produk

- **Produk**: IvyTicketing, platform SaaS pendaftaran dan ticketing event olahraga performa tinggi (marathon, trail run, triathlon, renang terbuka, balap sepeda, dsb).
- **Audiens Utama**:
  - *Peserta / Atlet*: Pelari dan pegiat olahraga yang mencari informasi kategori, rute race, mendaftar war ticket cepat, atau mengikuti undian ballot.
  - *Organizer & Race Director*: Penyelenggara event yang membutuhkan manajemen kuota, kustomisasi template halaman event, dan monitoring antrean pendaftaran.
- **Karakter Kunci**: Berorientasi aksi, atletik, presisi, siap menangani beban tinggi saat sistem War dan Ballot dibuka.

---

## 2. Referensi & Kepribadian Brand

- **Inspirasi Visual**: TCS Sydney Marathon (estetika World Marathon Major: visual hero yang gagah, kontras tinggi, navigasi race category yang tegas, kartu rute/kategori yang terstruktur, dan penanda status registrasi yang jelas).
- **Fleksibilitas Template**: Sistem UI harus modular dan mendukung kustomisasi tema per organizer/event (misal: warna aksen race, layout kartu kategori, banner visual, tanpa kehilangan konsistensi fungsional).
- **Kata Kunci Kepribadian**:
  - *Athletic*: Gagah, energik, dinamis.
  - *Precision*: Angka, rute, cut-off time, dan waktu pendaftaran tersaji akurat dan jelas.
  - *High-Trust*: Transparan dalam status kuota tiket, antrean war, dan hasil ballot.
  - *Modern*: Tidak menggunakan ornamen basi/slop AI (tidak ada glow berlebihan, tidak ada mesh gradient sembarangan, tidak ada font monospace yang dipaksakan di luar data teknis).

---

## 3. Palet Warna & Tema

Sistem warna bertumpu pada fondasi kontras tinggi untuk keterbacaan di lapangan dan perangkat mobile:

- **Neutral Base**:
  - Light Background: `#F8F9FA` atau `#FFFFFF` (bersih, keterbacaan tinggi saat outdoor).
  - Dark / Contrast Surfaces: `#0F172A` (deep slate) atau `#111827` untuk header hero, footer, atau kartu kategori utama.
  - Text Primary: `#0F172A` (rasio kontras > 7:1 terhadap background terang).
  - Text Muted: `#475569` (memenuhi standar WCAG AA > 4.5:1).
- **Core Brand Colors**:
  - Brand Primary: `#0B3D2E` (deep athletic racing green / ivy) atau Deep Navy `#0A2540`.
  - Secondary: `#1E293B` untuk card borders dan struktur sekunder.
- **Deliberate Accent**:
  - Satu aksen tegas untuk tombol tindakan utama (CTA), status "War Live", dan penanda kategori unggulan: Electric Coral / Vivid Orange (`#EA580C` atau `#F97316`) atau Energetic Gold/Yellow (`#D97706`).
  - Digunakan secara selektif pada elemen fokus terpenting saja (tidak disebar di setiap elemen).
- **State Colors**:
  - Tersedia (Open): `#16A34A`
  - Terbatas / War Active: `#EA580C`
  - Ballot / Undian: `#2563EB`
  - Kuota Habis (Sold Out): `#64748B`
  - Danger / Error: `#DC2626`

---

## 4. Tipografi

- **Font Utama (Body & UI)**:
  - Font keluarga sans-serif modern (`Inter`, `system-ui`, `-apple-system`).
  - Proporsi tegas, mudah dibaca cepat saat peserta terburu-buru melakukan registrasi war.
- **Headings (Judul & Display)**:
  - Bold, impactful, dengan visual weight yang mantap untuk nama event dan kategori race (misal 42K Full Marathon, 21K Half Marathon, 10K, Triathlon Olympic Distance).
- **Technical Accents (Mono)**:
  - Digunakan secara terarah khusus untuk data angka: nomor BIB, waktu cut-off (COT), countdown timer war, dan kode booking/tiket.

---

## 5. Dials (Tingkat Energi & Dinamika)

Sesuai standar antislop Part 3:

```text
Dial: ENERGY 3 / RHYTHM 2 / MOTION 2
```

- **ENERGY 3 (Bold / Athletic)**: Desain tampil percaya diri dan bertenaga seperti event marathon dunia, bukan sekadar dashboard korporat dingin.
- **RHYTHM 2 (Structured with Breaks)**: Tata letak terstruktur kuat (jadwal event, kartu kategori race, kalkulator COT, FAQ, countdown) dengan variasi antar bagian yang bermakna, menghindari layout monoton kartu seragam.
- **MOTION 2 (Responsive & Informative)**: Transisi halus, indikator real-time antrean war (progress bar, posisi antrean, countdown ticker) tanpa animasi hiasan lambat atau template bounce slop.

---

## 6. Pola Fungsional Khusus

1. **War Registration**:
   - Status antrean transparan (posisi antrean, estimasi waktu tunggu).
   - Indikator kuota real-time (Tersedia, Kritis, Sold Out).
   - Tombol registrasi yang jelas dan reaktif.
2. **Ballot Registration**:
   - Jadwal pembukaan dan pengumuman undian yang jelas.
   - Penjelasan alur tanpa bahasa marketing berbelit.
3. **Kategori Olahraga & Template Custom**:
   - Setiap kategori olahraga (lari, sepeda, triathlon, renang) memiliki metadata spesifik (jarak, cut-off time, elevasi, fasilitas race pack) yang disajikan dalam komponen modular.
