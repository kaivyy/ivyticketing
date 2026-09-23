# Panduan Integrasi Sistem Timing RFID & RACE RESULT 12

* Document ID: `DOC-TIMING-RR-IMPLEMENTATION-2026-09-17`
* Date: 2026-09-17
* Version: 1.0.0
* Status: Implemented & Verified in Production Monolith
* Target Modules: [`results`](file:///root/ivyticketing/services/api/internal/modules/results), [`tickets`](file:///root/ivyticketing/services/api/internal/modules/tickets), [`web`](file:///root/ivyticketing/apps/web)

---

## 1. Ringkasan Arsitektur & Prinsip Desain

Integrasi sistem timing RFID pada IvyTicketing menambahkan kapabilitas pencatatan waktu lomba lari otomatis berbasis transponder RFID (khususnya ekosistem **RACE RESULT 12**) tanpa menduplikasi modul registrasi, tanpa mengubah arsitektur modular monolith, dan tanpa membuat web results terpisah.

```
┌────────────────────────────────────────────────────────────────────────┐
│                        RACE RESULT 12 Ecosystem                        │
│                                                                        │
│  [RFID Mat/Antenna] ──> [Decoders/Track Boxes] ──> [Timing Module]     │
│                                                            │           │
│                                   (HTTP Forwarding Exporter)           │
└────────────────────────────────────────────────────────────┼───────────┘
                                                             │
                                                             ▼ POST /results/timing/passings
┌────────────────────────────────────────────────────────────────────────┐
│                   IVY Backend (results/timing)                         │
│                                                                        │
│ 1. [HTTP Handler & Token Auth]                                         │
│    Verifikasi token SHA-256 hash & batasan payload 10MB.               │
│                                                                        │
│ 2. [RaceResultAdapter] (Format Converter)                              │
│    Mengurai payload text/json -> []RawObservation                      │
│    (Adapter murni tanpa logika kalkulasi skor atau ranking)            │
│                                                                        │
│ 3. [Raw Persistence] (timing_passings table)                           │
│    Menyimpan observasi mentah secara idempotent (ON CONFLICT DO NOTHING│
│    berdasarkan external_read_id / PassingNo).                          │
│                                                                        │
│ 4. [Deduplication & Mapping Resolver]                                  │
│    Mencocokkan chip_code dengan bib_transponder_mappings               │
│    Memfilter read berulang (debouncing window 15 detik).               │
│                                                                        │
│ 5. [Generic Timing Processor] (processor.go)                           │
│    - Mengambil observasi matras START dan FINISH                       │
│    - Gun time  = finish_time - wave.start_at                           │
│    - Chip time = finish_time - start_time                              │
│    - Menghitung split intermediate (5K, 10K, dll) & pace min/km        │
│                                                                        │
│ 6. [Existing race_results Table]                                       │
│    Upsert ke race_results dengan source='TIMING_API'                   │
│    Menyimpan race_split_times per checkpoint                           │
│                                                                        │
│ 7. [Existing Ranking Engine] (recomputeRanks)                          │
│    Otomatis update RankOverall, RankGender, RankCategory, AgeGroup     │
└────────────────────────────────────────────────────────────┬───────────┘
                                                             │
                                                             ▼
┌────────────────────────────────────────────────────────────────────────┐
│               Public, Participant & Organizer Experience               │
│  - Live Leaderboard di UI Organizer results.astro                      │
│  - Halaman Hasil Peserta (/tickets/{id}/result)                        │
│  - E-Certificate Generator (/tickets/{id}/certificate)                 │
└────────────────────────────────────────────────────────────────────────┘
```

### Prinsip Utama Sistem:
1. **Single Source of Truth untuk BIB:** Nomor dada peserta tetap dikelola secara eksklusif oleh [`tickets.bib_number`](file:///root/ivyticketing/database/migrations/00049_add_tickets_bib_columns.sql) melalui [`tickets.Service`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go). Modul timing tidak membuat generator BIB baru.
2. **Decoupled Transponder Mapping:** Kode chip RFID dipetakan ke nomor BIB pada tabel `bib_transponder_mappings`, sehingga chip fisik dapat diganti di lapangan tanpa mengubah data tiket atau riwayat order.
3. **Pemisahan Telemetri Mentah vs Hasil Akhir:** Deteksi matras mentah disimpan persisten di tabel `timing_passings`. Hasil resmi lomba disimpan di tabel [`race_results`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql) dengan kolom `source = 'TIMING_API'`.
4. **Provider-Agnostic Processor:** Mesin kalkulasi waktu di [`timing.Processor`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go#L74) tidak mengetahui vendor hardware tertentu. Logika scoring berlaku universal baik untuk data dari RACE RESULT, file CSV offline, maupun pembaca RFID UHF native.
5. **Dumb Adapters:** Adapter vendor seperti [`RaceResultAdapter`](file:///root/ivyticketing/services/api/internal/modules/results/timing/raceresult.go#L19) murni bertugas menerjemahkan protokol/format keluar masuk tanpa memproses hasil skor atau peringkat.

---

## 2. Mekanisme Komunikasi Resmi RACE RESULT 12

Berdasarkan arsitektur perangkat lunak RACE RESULT 12, integrasi dibagi menjadi 3 kanal:

### 1. Ingestion Deteksi Mentah (Exporters / Forwarding)
* **Bukan Simple API!** Simple API hanya digunakan untuk membaca daftar juara atau output list yang sudah dipublikasikan.
* Alur ingestion real-time menggunakan fitur **Exporters / Forwarding** di Timing Module RACE RESULT 12:
  * Exporter dikonfigurasi dengan destination HTTP POST ke endpoint IVY:
    `POST /api/v1/organizations/{orgId}/events/{eventId}/results/timing/passings`
  * Exporter dapat mengirimkan data dalam format **JSON** atau **Delimited Text** (semicolon/tab/comma).
  * Field data passing standar:
    * `PassingNo`: ID unik passing dari decoder (digunakan untuk idempotency).
    * `Transponder`: Kode chip RFID (misal: `RR_TAG_1024` atau alphanumeric EPC).
    * `TimingPoint`: Nama titik deteksi matras (misal: `START`, `5K`, `FINISH`).
    * `Time`: Jam deteksi (format `HH:MM:SS.mmm`).
    * `Date`: Tanggal deteksi (`YYYY-MM-DD`).
    * `Hits`: Jumlah deteksi antena per transponder.
    * `RSSI`: Kekuatan sinyal antena (dBm).
    * `Antenna`: Nomor port antena.

### 2. Pertukaran Data Peserta (Participant Export)
* Panitia mengekspor data peserta yang telah memiliki BIB dari IVY ke format file pertukaran peserta RACE RESULT 12:
  * Format CSV: Semicolon-separated.
  * Kolom: `BIB;FIRSTNAME;LASTNAME;GENDER;DATEOFBIRTH;CONTEST;TRANSPONDER1`.
  * File ini diimpor ke dalam software RACE RESULT 12 pada menu **Participants**.

### 3. Keamanan Ingestion
* Request ke endpoint ingestion wajib menyertakan token autentikasi yang di-generate oleh panitia di dashboard IVY.
* Token dapat dikirim melalui:
  * Header HTTP: `Authorization: Bearer <token>`
  * Header HTTP: `X-Timing-Token: <token>`
  * Query Parameter: `?token=<token>`
* Ukuran request dibatasi maksimal **10 MB** (`http.MaxBytesReader`) untuk mencegah serangan Denial of Service (DoS).

---

## 3. Struktur Skema Database (Migration `00062`)

File migrasi: [`database/migrations/00062_create_timing_system.sql`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql)

### 1. `timing_configs`
Menyimpan pengaturan integrasi timing per event:
* `id` UUID PRIMARY KEY
* `organization_id` UUID NOT NULL (Cascade)
* `event_id` UUID NOT NULL UNIQUE (Cascade)
* `provider` TEXT NOT NULL CHECK (`provider IN ('RACE_RESULT', 'CSV', 'NATIVE')`)
* `ingestion_token_hash` TEXT NOT NULL (SHA-256 hex string)
* `ingestion_token_prefix` TEXT NOT NULL (12 karakter pertama)
* `encrypted_api_key` TEXT (Kredensial terenkripsi opsional)
* `external_race_id` TEXT (ID file lomba pada software vendor)
* `is_active` BOOLEAN NOT NULL DEFAULT true
* `settings` JSONB NOT NULL DEFAULT `'{}'`

### 2. `race_waves`
Mendukung start bergelombang (corral) dengan waktu gun start resmi yang independen:
* `id` UUID PRIMARY KEY
* `organization_id` UUID NOT NULL (Cascade)
* `event_id` UUID NOT NULL (Cascade)
* `category_id` UUID REFERENCES `event_categories(id)`
* `code` TEXT NOT NULL (misal: `WAVE_A`, `ELITE`)
* `name` TEXT NOT NULL
* `start_at` TIMESTAMPTZ (Waktu flag-off resmi)
* `order_index` INT NOT NULL DEFAULT 0
* UNIQUE (`event_id`, `code`)

### 3. Relasi Wave ke Tiket dan Hasil
* `ALTER TABLE tickets ADD COLUMN wave_id uuid REFERENCES race_waves(id);`
* `ALTER TABLE race_results ADD COLUMN wave_id uuid REFERENCES race_waves(id);`

### 4. `timing_checkpoints`
Definisi titik matras/antena:
* `id` UUID PRIMARY KEY
* `organization_id` UUID NOT NULL (Cascade)
* `event_id` UUID NOT NULL (Cascade)
* `code` TEXT NOT NULL (misal: `START`, `CP_5K`, `FINISH`)
* `name` TEXT NOT NULL
* `checkpoint_type` TEXT NOT NULL CHECK (`checkpoint_type IN ('START', 'SPLIT', 'FINISH')`)
* `order_index` INT NOT NULL DEFAULT 0
* `distance_meters` INT (Jarak dari start line dalam meter)
* UNIQUE (`event_id`, `code`)

### 5. `bib_transponder_mappings`
Pemetaan nomor BIB ke chip transponder:
* `id` UUID PRIMARY KEY
* `organization_id` UUID NOT NULL (Cascade)
* `event_id` UUID NOT NULL (Cascade)
* `bib_number` TEXT NOT NULL
* `transponder_code` TEXT NOT NULL
* `is_active` BOOLEAN NOT NULL DEFAULT true
* `status` TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (`status IN ('ACTIVE', 'REPLACED', 'REVOKED')`)
* `notes` TEXT
* Partial Unique Index: `(event_id, bib_number) WHERE is_active`
* Partial Unique Index: `(event_id, transponder_code) WHERE is_active`

### 6. `timing_passings` (Buffer Telemetri Mentah)
* `id` BIGSERIAL PRIMARY KEY
* `organization_id` UUID NOT NULL (Cascade)
* `event_id` UUID NOT NULL (Cascade)
* `checkpoint_code` TEXT NOT NULL
* `provider` TEXT NOT NULL
* `external_read_id` TEXT (Passing ID dari hardware/RACE RESULT)
* `chip_code` TEXT NOT NULL
* `bib_number` TEXT (Nullable)
* `observed_at` TIMESTAMPTZ NOT NULL (Waktu pada jam matras)
* `received_at` TIMESTAMPTZ NOT NULL DEFAULT now()
* `raw_payload` TEXT
* `metadata` JSONB NOT NULL DEFAULT `'{}'`
* `processed` BOOLEAN NOT NULL DEFAULT false
* `processed_at` TIMESTAMPTZ
* Idempotency Unique Index: `(event_id, provider, external_read_id) WHERE external_read_id IS NOT NULL`
* Fallback Unique Index: `(event_id, provider, chip_code, checkpoint_code, observed_at) WHERE external_read_id IS NULL`

### 7. `race_split_times` (Intermediate Splits)
* `id` UUID PRIMARY KEY
* `race_result_id` UUID NOT NULL REFERENCES `race_results(id)` ON DELETE CASCADE
* `checkpoint_id` UUID NOT NULL REFERENCES `timing_checkpoints(id)` ON DELETE CASCADE
* `passing_id` BIGINT REFERENCES `timing_passings(id)` ON DELETE SET NULL
* `split_time_ms` BIGINT NOT NULL (Elapsed time dari start dalam milidetik)
* `split_pace_ms_km` BIGINT (Pace segmen dalam milidetik/km)
* `passing_time` TIMESTAMPTZ NOT NULL
* `order_index` INT NOT NULL DEFAULT 0
* UNIQUE (`race_result_id`, `checkpoint_id`)

---

## 4. Logika Perhitungan Waktu (Timing Processor)

File: [`services/api/internal/modules/results/timing/processor.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go)

### 1. Resolusi BIB
Observasi mentah pada `timing_passings` mencocokkan `chip_code` dengan `bib_transponder_mappings` aktif untuk mendapatkan `bib_number`.

### 2. Filter Debouncing (Window 15 Detik)
Untuk mencegah pencatatan berulang ketika pelari berdiri di atas matras start atau finish:
* Pembacaan chip yang sama pada matras yang sama dengan selisih waktu kurang dari 15 detik akan diabaikan (hanya deteksi pertama yang dicatat).

### 3. Perhitungan Net Time (Chip Time)
```
chip_time_ms = finish_observed_at - start_observed_at
```
Jika matras start tidak tersedia, waktu mulai diambil dari `race_waves.start_at`.

### 4. Perhitungan Gross Time (Gun Time)
```
gun_time_ms = finish_observed_at - wave.start_at
```
Jika peserta tidak memiliki wave khusus, gun time sama dengan net time.

### 5. Intermediate Splits & Pace
Untuk setiap checkpoint bertipe `SPLIT`:
```
split_time_ms = split_observed_at - start_observed_at
split_pace_ms_km = split_time_ms / (distance_meters / 1000.0)
```

### 6. Status Finisher
* `FINISHED`: Memiliki waktu start valid dan melintasi matras finish.
* `DNF` (Did Not Finish): Tercatat di matras start, namun tidak ada deteksi di matras finish.
* `DNS` (Did Not Start): Tidak ada deteksi di matras start maupun finish.

---

## 5. Katalog API Endpoints

Semua request manajemen organizer membutuhkan header JWT `Authorization: Bearer <token>` dan hak akses `results.manage`.

### 1. Ingest Passings (RACE RESULT Exporter Ingestion)
* **Method:** `POST`
* **URL:** `/api/v1/organizations/{orgId}/events/{eventId}/results/timing/passings`
* **Header:** `Authorization: Bearer <ingestion_token>` atau `X-Timing-Token: <ingestion_token>`
* **Content-Type:** `application/json` atau `text/plain`
* **Payload Contoh (JSON):**
```json
[
  {
    "PassingNo": 1042,
    "Transponder": "TAG_A102",
    "TimingPoint": "FINISH",
    "Time": "07:15:30.450",
    "Date": "2026-09-17",
    "Hits": 14,
    "RSSI": 130
  }
]
```
* **Payload Contoh (Delimited):**
```text
1042;TAG_A102;07:15:30.450;2026-09-17;14;130;1;FINISH
```
* **Response (200 OK):**
```json
{
  "status": "accepted",
  "ingested": 1
}
```

### 2. Konfigurasi Provider Timing & Generate Token
* **Method:** `POST`
* **URL:** `/api/v1/organizations/{orgId}/events/{eventId}/results/timing/config`
* **Body:**
```json
{
  "provider": "RACE_RESULT",
  "externalRaceId": "marathon_2026",
  "settings": {}
}
```
* **Response (200 OK):**
```json
{
  "config": {
    "id": "uuid",
    "eventId": "uuid",
    "provider": "RACE_RESULT",
    "ingestionTokenPrefix": "ivytt_3a1b2c",
    "externalRaceId": "marathon_2026",
    "isActive": true
  },
  "ingestionToken": "ivytt_3a1b2c4d5e6f7a8b9c0d1e2f"
}
```
*(Catatan: Token rahasia hanya ditampilkan sekali saat di-generate).*

### 3. Impor Mapping BIB ke Transponder Chip
* **Method:** `POST`
* **URL:** `/api/v1/organizations/{orgId}/events/{eventId}/results/timing/mappings/import?replace=true`
* **Content-Type:** `text/csv`
* **Body CSV:**
```csv
bib,chip,notes
1001,TAG_A1001,Paket BIB Reguler
1002,TAG_A1002,Paket BIB Reguler
1003,TAG_A1003,Penggantian chip
```
* **Response (200 OK):**
```json
{
  "imported": 3
}
```

### 4. Eksekusi Perhitungan Hasil (Process Timing)
* **Method:** `POST`
* **URL:** `/api/v1/organizations/{orgId}/events/{eventId}/results/timing/process`
* **Response (200 OK):**
```json
{
  "passingsEvaluated": 1250,
  "resultsUpdated": 620,
  "ranked": true
}
```

---

## 6. Panduan Operasional Lomba (Race Day Runbook)

### H-3 Lomba: Persiapan & Sinkronisasi
1. Buka dashboard hasil di web: `/org/[orgId]/events/[eventId]/results`.
2. Klik tombol **Konfigurasi RACE RESULT**. Masukkan External Race ID jika ada.
3. Salin **Ingestion Token** yang muncul. Simpan di tempat aman.
4. Klik **Impor Mapping BIB ↔ Chip** untuk mengunggah CSV relasi nomor dada pelari ke transponder RFID yang sudah ditempelkan pada BIB fisik.

### H-1 Lomba: Setup Software RACE RESULT 12 di Laptop Lapangan
1. Buka software **RACE RESULT 12**.
2. Masuk ke modul **Timing** > **Exporters / Forwarding**.
3. Tambahkan Exporter baru:
   * **Destination:** `HTTP`
   * **URL:** `http://<ip-or-domain>/api/v1/organizations/<orgId>/events/<eventId>/results/timing/passings`
   * **HTTP Header:** `Authorization: Bearer <ingestion_token>`
   * **Data Format:** `JSON` atau `Standard Delimited`
   * **Timing Points:** Pilih semua timing point (`START`, `SPLIT`, `FINISH`).
4. Uji koneksi dengan melakukan scan chip uji coba. Pastikan counter *Observasi Telemetri* di dashboard IVY bertambah.

### Hari Lomba: Pelaksanaan & Publikasi
1. Saat bendera start dikibarkan, pelari melintasi matras start.
2. Sepanjang lomba, counter passing di kartu *Observasi Telemetri* akan terus bertambah seiring pelari melintasi matras split dan finish.
3. Panitia dapat menekan tombol **Hitung Ulang Timing Telemetri** kapan saja untuk memperbarui klasemen sementara secara live.
4. Setelah pelari terakhir melintasi finish, lakukan evaluasi final untuk memastikan seluruh data telah terproses (`pendingCount: 0`).
5. Hasil otomatis tampil di:
   * Klasemen utama web
   * Halaman hasil masing-masing peserta (`/tickets/{ticketId}/result`)
   * Unduhan e-certificate resmi pelari.

---

## 7. Status Pengujian & Verifikasi

* **Unit Test:** Seluruh test suite pada package [`results/timing`](file:///root/ivyticketing/services/api/internal/modules/results/timing) lulus (`go test ./...`).
* **Regresi Monolith:** Tidak ada regresi pada modul tiket, pemesanan, pembayaran, scanner, maupun racepack.
* **Frontend Build:** Aplikasi web frontend lulus build Astro v4 hybrid mode (`pnpm build`).
* **Service Runtime:** Service API (Port 8081) dan Web (Port 4321) berjalan normal di bawah supervisi PM2.
