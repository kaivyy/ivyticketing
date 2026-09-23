# Arsitektur Sistem Timing & Hasil Lomba (Race Timing Engine)

Dokumen ini merinci arsitektur pencatatan waktu, protokol penerimaan telemetri RFID, pemrosesan matras lintasan (*timing checkpoints*), dan algoritma penghitungan peringkat atlet pada platform IvyTicketing.

---

## 1. Ikhtisar Mesin Timing

IvyTicketing mengadopsi model terpadu untuk pencatatan waktu lomba lari dengan dua jalur input data:
1. **Push Telemetri Real-time**: Penerimaan passing transponder RFID secara langsung dari decoder lapangan (RACE RESULT 12, MyLaps BibTag, ChronoTrack).
2. **Impor Berkas Batch (CSV)**: Pengunggahan hasil rekaman waktu pasca-lomba untuk event tanpa koneksi internet langsung di lokasi rute.

```
+-----------------------------------------------------------------------------+
|                          PIPELINE SISTEM TIMING                             |
|                                                                             |
|  [Transponder Lapangan]                                                     |
|       |                                                                     |
|       v                                                                     |
|  [Decoder RACE RESULT / MyLaps]                                             |
|       | (HTTP Push / TCP Telemetri)                                         |
|       v                                                                     |
|  POST /timing/telemetry                                                     |
|       |                                                                     |
|       v                                                                     |
|  [Pencocokan Transponder -> BIB -> Tiket]                                  |
|       |                                                                     |
|       +---> Matras START    : Waktu Chip Start                              |
|       +---> Matras CHECKPOINT: Split Time (5K, 10K, 21K, 30K)                |
|       +---> Matras FINISH   : Waktu Finish Bruto & Bersih                   |
|       |                                                                     |
|       v                                                                     |
|  [Algoritma recomputeRanks()]                                               |
|       |                                                                     |
|       +---> Overall Rank (Peringkat Umum Lomba)                             |
|       +---> Gender Rank  (Peringkat Kategori Pria / Wanita)                 |
|       +---> Age Group    (Peringkat Kelompok Umur, misal: M30-39)           |
|       |                                                                     |
|       v                                                                     |
|  [Penerbitan e-Certificate Dinamis]                                         |
+-----------------------------------------------------------------------------+
```

---

## 2. Struktur Data Matras & Titik Lintasan (Checkpoints)

Tabel basis data `timing_checkpoints` mendefinisikan seluruh matras sensor di sepanjang lintasan lomba:

```sql
CREATE TABLE timing_checkpoints (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id),
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    code             text NOT NULL,       -- contoh: START, SPLIT_10K, FINISH
    name             text NOT NULL,       -- contoh: "KM 10 Refreshment Point"
    distance_km      numeric(5,2),        -- contoh: 10.00
    checkpoint_order int NOT NULL,        -- urutan lintasan: 1, 2, 3...
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE(event_id, code)
);
```

### Validasi Integritas Rute
Pencatatan waktu di titik perantara (*split checkpoint*) berfungsi memastikan atlet menempuh rute resmi secara lengkap:
- Jika seorang pelari memiliki catatan di matras `START` dan `FINISH` namun melewatkan matras perantara `SPLIT_10K` dan `SPLIT_21K`, status pelari ditandai sebagai indikasi anomali rute (*missed split / DNF*).

---

## 3. Perhitungan Waktu: Gun Time vs Net (Chip) Time

1. **Gun Time (Waktu Bruto)**:
   - Dihitung dari saat pistol start ditembakkan (*Gun Start*) dari gelombang lomba yang ditetapkan untuk atlet tersebut:
   `Gun Time = Waktu Matras Finish - Gun Start Wave`
   - Digunakan untuk penentuan juara podium resmi menurut regulasi World Athletics / PASI.

2. **Net Time / Chip Time (Waktu Bersih)**:
   - Dihitung dari saat atlet melintasi matras garis start:
   `Net Time = Waktu Matras Finish - Waktu Matras Start`
   - Digunakan untuk pencatatan waktu pribadi atlet (*Personal Best*) dan penentuan kualifikasi lomba lari internasional (contoh: Boston Marathon Qualifier).

3. **Pace Rata-rata**:
   - Dihitung berdasarkan jarak kategori lomba (`event_categories.distance_km`):
   `Pace (detik/km) = Net Time (detik) / Jarak Kategori (km)`

---

## 4. Algoritma Pemeringkatan (Ranking Algorithm)

Fungsi rekalkulasi peringkat dieksekusi secara server-side melalui modul [`services/api/internal/modules/results/`](file:///root/ivyticketing/services/api/internal/modules/results/):

```go
// Logika pemeringkatan atlet:
// 1. Urutkan seluruh atlet berstatus FINISHED berdasarkan Net Time ASC.
// 2. Berikan nomor urut 1..N untuk overall_rank.
// 3. Kelompokkan per gender (M/F) dan urutkan untuk gender_rank.
// 4. Kelompokkan per kelompok umur (M20-29, M30-39, W30-39, dst.) untuk age_group_rank.
// 5. Peserta dengan status DNF, DNS, atau DSQ tidak mendapatkan peringkat numerik.
```

Status pelari terstandardisasi ke dalam 4 kode:
- `FINISHED`: Pelari menyelesaikan rute secara lengkap sebelum batas waktu (Cut-Off Time).
- `DNF` (*Did Not Finish*): Pelari memulai lomba tetapi tidak menyelesaikan rute atau melampaui batas waktu cut-off.
- `DNS` (*Did Not Start*): Pelari memiliki tiket dan BIB tetapi tidak terdeteksi melintasi matras start.
- `DSQ` (*Disqualified*): Pelari didiskualifikasi oleh panitia juri akibat pelanggaran regulasi.

---

## 5. Penerbitan Sertifikat Finisher

Sertifikat diterbitkan secara dinamis tanpa perlu penyimpanan berkas statis PDF:
1. Alamat akses sertifikat: `/participant/certificate/{ticketId}`
2. Halaman ini memvalidasi keabsahan tiket pelari melalui modul tickets.
3. Data waktu bersih, waktu bruto, nomor BIB, dan peringkat diinjeksi ke dalam kanvas sertifikat.
4. CSS print dioptimalkan untuk cetak definisi tinggi (High-DPI print) dan unduh instan format PDF dari peramban.
