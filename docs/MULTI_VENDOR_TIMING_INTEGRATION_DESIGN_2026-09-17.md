# Arsitektur Integrasi Multi-Vendor Race Timing & RFID

* Document ID: `DOC-TIMING-MULTI-VENDOR-2026-09-17`
* Date: 2026-09-17
* Version: 2.0.0
* Status: Multi-Provider Architectural Blueprint
* Target Modules: [`results`](file:///root/ivyticketing/services/api/internal/modules/results), [`tickets`](file:///root/ivyticketing/services/api/internal/modules/tickets), [`apps/web`](file:///root/ivyticketing/apps/web)

---

## 1. Executive Summary & Vision

IvyTicketing diposisikan sebagai **vendor-agnostic race timing integration layer** untuk industri ajang lari (road race, trail, triathlon, fun run). Sistem ini memisahkan secara tegas antara sistem manajemen pendaftaran/tiket peserta dengan sistem pencatatan waktu lomba fisik (RFID/chip timing).

```
                       IVY RACE MANAGEMENT
                               │
                 Existing Event / Ticket / BIB
                               │
                       Timing Integration
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
      RACE RESULT          Vendor Timing        CSV/Excel
          │                    │                    │
          └────────────────────┼────────────────────┘
                               │
                      Normalized Timing Model
                               │
                       Raw Passing Storage
                               │
                    Generic Timing Processor
                               │
                       Existing Results
```

### Prinsip Utama Sistem:
1. **Tidak Ada Vendor Lock-in:** Sistem mendukung berbagai sistem timing: `RACE_RESULT`, `GENERIC_CSV`, `API_VENDOR`, `NATIVE_RFID`, dan `MANUAL`.
2. **Pemisahan Provider vs Transport:**
   * **Provider:** Format data dan entitas sistem (misal RACE RESULT 12, MyLaps, ChronoTrack, Generic).
   * **Transport:** Kanal penerimaan data (misal `HTTP_PUSH`, `HTTP_PULL`, `CSV_UPLOAD`, `SFTP`, `LOCAL_AGENT`).
3. **Single Source of Truth untuk BIB:** Nomor BIB peserta selalu berasal dari [`tickets.bib_number`](file:///root/ivyticketing/database/migrations/00049_add_tickets_bib_columns.sql). Modul timing tidak membuat generator nomor dada baru.
4. **Decoupled Transponder Mapping:** Nomor BIB dipetakan secara terpisah ke kode transponder RFID pada `bib_transponder_mappings`.
5. **Non-Destructive Raw Ingestion:** Semua sinyal deteksi antena matras disimpan utuh di `timing_passings` tanpa pembuangan dini (no destructive filtering at ingestion).
6. **Provider-Agnostic Scoring Processor:** Perhitungan waktu net/gun, debouncing, intermediate split, dan ranking dilakukan secara terpusat oleh engine netral yang tidak mengetahui nama vendor tertentu.
7. **Single Source of Configuration:** Admin Event Studio (`/admin/events/edit`) dan Organizer Results (`/org/.../results`) berbagi entitas konfigurasi database yang sama.

---

## 2. Audit Implementasi Existing & Peta Reuse

| Komponen Existing | Lokasi File | Status Reuse | Evaluasi & Tindakan |
| :--- | :--- | :--- | :--- |
| **BIB Allocation** | [`services/api/internal/modules/tickets/bib_service.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go) | Dipertahankan Utuh | Metode `AssignNextBib`, `SetBib`, `BulkAssignBib`, dan `StreamTicketsForBibExport` tetap menjadi sumber utama penomoran pelari. |
| **Final Results & Ranks** | [`database/migrations/00061_create_race_results.sql`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql) | Dipertahankan Utuh | Tabel `race_results` menampung hasil resmi (`source='TIMING_API'`), dievaluasi oleh `recomputeRanks()`. |
| **Start Waves & Splits** | [`database/migrations/00062_create_timing_system.sql`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql) | Diperluas | Tabel `race_waves` dan `race_split_times` digunakan untuk flag-off gun time dan pencatatan split per segmen. |
| **Participant Results** | [`services/api/internal/modules/results/handler.go`](file:///root/ivyticketing/services/api/internal/modules/results/handler.go) | Dipertahankan Utuh | Rute `/tickets/{ticketId}/result` dan `/tickets/{ticketId}/certificate` tetap melayani pelari secara transparan. |
| **Gate Check-in** | [`services/api/internal/modules/scanner/*`](file:///root/ivyticketing/services/api/internal/modules/scanner) | Terisolasi | Check-in tiket gerbang venue (`VALID` -> `USED`) tetap terpisah dari matras timing. |
| **Timing Config** | [`database/migrations/00062_create_timing_system.sql`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql) | Diperluas di `00063` | Menambahkan kolom `transport`, `sync_mode`, dan `policy` JSONB untuk konfigurasi multi-vendor. |
| **Admin Studio** | [`apps/web/src/pages/admin/events/edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro) | Diperluas | Menambahkan Tab 10 (Timing & Race RFID) mengikuti pattern 9 tab existing. |

---

## 3. Multi-Provider Domain Design

Sistem membedakan secara tegas antara identitas vendor dengan metode penerimaan paket data:

### 1. Provider (Format & Format Converter)
* `RACE_RESULT`: Format Exporter / Passing Stream dari software RACE RESULT 12 (JSON array atau delimited text).
* `GENERIC_CSV`: Format tabel spreadsheet standar (`bib,checkpoint,time` atau `chip,checkpoint,time`).
* `API_VENDOR`: Vendor pihak ketiga yang mengirimkan payload JSON terstruktur sesuai spesifikasi API terbuka IVY.
* `NATIVE_RFID`: Antena pembaca RFID UHF (LLRP / TCP reader box) yang mengirim raw EPC reads via local agent.
* `MANUAL`: Pencatatan waktu manual oleh petugas stopwatch lapangan.

### 2. Transport (Metode Pengiriman Data)
* `HTTP_PUSH`: Vendor / Exporter mendorong data real-time via HTTP POST ke endpoint IVY.
* `HTTP_PULL`: Worker IVY menarik data berkala dari server vendor via REST API.
* `CSV_UPLOAD`: Panitia mengunggah file hasil/passing secara manual melalui antarmuka web.
* `SFTP`: File log diletakkan pada folder SFTP dan diproses otomatis oleh worker.
* `LOCAL_AGENT`: Agen perangkat lunak lokal di lokasi lomba yang mengagregasi data antena lalu mengirimkannya ke cloud.

### 3. Sync Mode (Target Alur Pemrosesan)
* `RAW_PASSINGS`: Data yang masuk adalah deteksi antena mentah -> disimpan ke `timing_passings` -> diproses oleh generic scoring processor -> `race_results`.
* `FINAL_RESULTS`: Data yang masuk adalah hasil akhir resmi dari vendor timing eksternal -> langsung di-upsert ke `race_results` -> menjalankan `recomputeRanks`.

---

## 4. Normalized Timing Contract

Semua adapter vendor harus menerjemahkan data eksternal ke model internal Go berikut:

```go
package timing

import (
    "time"
)

// TimingPassing merepresentasikan sebuah observasi deteksi transponder RFID ternormalisasi.
type TimingPassing struct {
    ExternalID      string           `json:"externalId,omitempty"`      // PassingNo / ReadSeq dari vendor
    CheckpointCode  string           `json:"checkpointCode"`            // START, CP_5K, FINISH
    ChipCode        string           `json:"chipCode"`                  // Uppercase alphanumeric normalized
    RawChipCode     string           `json:"rawChipCode,omitempty"`     // Nilai asli dari hardware
    BibNumber       *string          `json:"bibNumber,omitempty"`       // Terisi jika sudah dipetakan
    ObservedAt      time.Time        `json:"observedAt"`                // Waktu jam decoder
    ReceivedAt      time.Time        `json:"receivedAt"`                // Waktu tiba di server IVY
    SourceProvider  string           `json:"sourceProvider"`            // RACE_RESULT, CSV, dll
    TransportMethod string           `json:"transportMethod"`           // HTTP_PUSH, CSV_UPLOAD, dll
    RawPayload      string           `json:"rawPayload,omitempty"`      // String asli untuk forensik
    Telemetry       PassingTelemetry `json:"telemetry,omitempty"`       // Sinyal radio & antena
}

type PassingTelemetry struct {
    Hits     int    `json:"hits,omitempty"`
    RSSI     int    `json:"rssi,omitempty"`     // Kekuatan sinyal (dBm)
    Antenna  int    `json:"antenna,omitempty"`  // Port antena
    DeviceID string `json:"deviceId,omitempty"` // ID perangkat decoder
}

// NormalizedResult merepresentasikan hasil akhir balapan dari vendor tanpa raw passings.
type NormalizedResult struct {
    BibNumber       string     `json:"bibNumber"`
    ParticipantName string     `json:"participantName"`
    Gender          string     `json:"gender"`
    Age             *int       `json:"age,omitempty"`
    AgeGroup        string     `json:"ageGroup,omitempty"`
    Status          string     `json:"status"` // FINISHED, DNF, DNS, DSQ
    ChipTimeMs      *int64     `json:"chipTimeMs,omitempty"`
    GunTimeMs       *int64     `json:"gunTimeMs,omitempty"`
    FinishedAt      *time.Time `json:"finishedAt,omitempty"`
}
```

---

## 5. Provider Capability Interfaces (Small & Idiomatic Go)

Setiap vendor hanya mengimplementasikan interface kecil (capability) yang relevan:

```go
package timing

import (
    "context"
)

// PassingParser diimplementasikan oleh vendor yang mengirim stream deteksi matras.
type PassingParser interface {
    ProviderName() string
    ParsePassings(ctx context.Context, payload []byte, defaultPoint string) ([]TimingPassing, error)
}

// ParticipantExporter diimplementasikan jika vendor membutuhkan daftar peserta untuk konsol timing.
type ParticipantExporter interface {
    ProviderName() string
    ExportParticipants(ctx context.Context, participants []ParticipantRecord) ([]byte, error)
}

// FinalResultParser diimplementasikan jika vendor hanya mengunggah hasil akhir balapan.
type FinalResultParser interface {
    ProviderName() string
    ParseFinalResults(ctx context.Context, payload []byte) ([]NormalizedResult, error)
}

// MappingParser diimplementasikan jika vendor menyediakan file asosiasi nomor dada ke chip RFID.
type MappingParser interface {
    ProviderName() string
    ParseChipMappings(ctx context.Context, payload []byte) ([]ChipMappingRecord, error)
}
```

### Matriks Kemampuan Provider (Capability Matrix):
| Provider | PassingParser | ParticipantExporter | FinalResultParser | MappingParser |
| :--- | :---: | :---: | :---: | :---: |
| **RACE_RESULT** | Ya | Ya (CSV RR12) | Tidak | Ya |
| **GENERIC_CSV** | Ya | Ya (CSV Umum) | Ya | Ya |
| **API_VENDOR** | Ya (JSON) | Ya (JSON) | Ya (JSON) | Ya |
| **NATIVE_RFID** | Ya (LLRP Stream)| Tidak | Tidak | Tidak |
| **MANUAL** | Tidak | Tidak | Ya | Tidak |

---

## 6. Model Asosiasi BIB & Transponder (Model A, B, C)

Sistem mendukung tiga skenario penomoran dada dan chip RFID di lapangan:

### Model A: IVY Menentukan BIB (Standard Flow)
1. Peserta terdaftar di IVY dan mendapatkan nomor dada otomatis (`tickets.bib_number`).
2. Panitia mengekspor data peserta via `ParticipantExporter`.
3. Vendor menempelkan stiker chip RFID fisik pada nomor dada fisik sesuai daftar.
4. Vendor mengirimkan file mapping `(bib, chip)`.
5. File diimpor ke tabel `bib_transponder_mappings`.

### Model B: Vendor Menentukan Chip, IVY Melakukan Mapping On-Demand
1. Nomor dada peserta telah dialokasikan di IVY.
2. Saat peserta mengambil racepack di RPC, petugas mengambil BIB fisik dan memindai barcode nomor dada, lalu memindai chip transponder acak.
3. IVY mencatat relasi via endpoint `POST /timing/mappings`.

### Model C: Vendor Menyediakan Paket BIB + Chip Pre-Mapped
1. Vendor menyediakan paket nomor dada yang sudah memiliki chip tertanam dengan nomor tetap.
2. Panitia mengimpor file mapping sebelum balapan dimulai.
3. IVY mencatat pasangan tersebut dan mengaktifkannya sebagai mapping resmi balapan.

### Penggantian Chip (Chip Replacement):
* Satu nomor BIB hanya boleh memiliki **satu chip aktif** dalam satu event.
* Jika chip rusak, chip lama diubah menjadi `is_active = false, status = 'REPLACED'`.
* Chip baru disimpan sebagai `is_active = true, status = 'ACTIVE'`.
* Histori chip lama tetap tersimpan untuk audit forensik.

---

## 7. Alias Mapping Titik Matras (Checkpoint Mapping)

Vendor timing sering menggunakan penamaan titik matras yang berbeda dari standar IVY.
Tabel `timing_checkpoints` dilengkapi dengan array alias:

```text
IVY Checkpoint Code: FINISH
Aliases: ["FinishLine", "Finish Mat", "Mat 1", "Ch1", "T1_Finish"]

IVY Checkpoint Code: START
Aliases: ["StartLine", "Start Mat", "Gun Mat", "T0_Start"]
```

Saat observasi passing masuk, processor memeriksa kecocokan string `TimingPoint` dari vendor terhadap `code` resmi maupun daftar `aliases`. Hal ini mengeliminasi kebutuhan penyesuaian manual pada konfigurasi decoder lapangan.

---

## 8. Kebijakan Balapan Dinamis (Configurable Event Policy)

Tabel `timing_configs` menyimpan objek `policy` (JSONB) yang memungkinkan panitia menyesuaikan aturan lomba:

```json
{
  "debounce_window_seconds": 15,
  "cutoff_minutes": 420,
  "start_mode": "CHIP_PREFERRED",
  "mandatory_checkpoints": ["START", "FINISH"],
  "allow_missing_start": true,
  "scoring_basis": "NET_TIME"
}
```

* `debounce_window_seconds`: Durasi pengabaian deteksi berulang pada matras yang sama (default: 15 detik).
* `start_mode`:
  * `CHIP_PREFERRED`: Gunakan waktu matras start; jika tidak ada, fallback ke `race_waves.start_at`.
  * `GUN_ONLY`: Semua peserta dinilai berdasarkan waktu flag-off wave.
  * `CHIP_MANDATORY`: Pelari yang tidak melintasi matras start otomatis berstatus `DNS`.
* `allow_missing_start`: Mengontrol apakah pelari tanpa deteksi start boleh dihitung waktunya (dengan penanda review).

---

## 9. Pemisahan Tanggung Jawab Antarmuka (UI Separation)

```
┌────────────────────────────────────────────────────────────────────────┐
│                ADMIN EVENT STUDIO (admin/events/edit.astro)            │
│                         "SETUP & KONFIGURASI"                          │
├────────────────────────────────────────────────────────────────────────┤
│ • Pilih Provider (RACE RESULT / CSV / API Vendor / Native)             │
│ • Pilih Transport (HTTP Push / CSV Upload / Local Agent)               │
│ • Status Integrasi Timing: Aktif / Nonaktif                            │
│ • Pengaturan Race Waves (Corral A, B, C beserta waktu flag-off)        │
│ • Penataan Titik Matras Checkpoint (START, Split 5K, FINISH)           │
│ • Konfigurasi Alias Nama Matras Vendor                                 │
│ • Kebijakan Lomba (Debounce Window, Cutoff, Start Mode)                │
│ • Generate / Reset Token Ingestion Aman                                │
└────────────────────────────────────────────────────────────────────────┘
                                    │
                       (Satu Sumber Konfigurasi)
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│               ORGANIZER RESULTS (org/.../results.astro)                │
│                        "OPERASIONAL & MONITORING"                      │
├────────────────────────────────────────────────────────────────────────┤
│ • Indikator Koneksi & Counter Telemetri Live (Total, Terproses, Antre) │
│ • Deteksi Chip Belum Terpetakan (Unmapped Transponders Alert)          │
│ • Upload / Koreksi Mapping BIB ↔ Chip di Hari-H                        │
│ • Tombol Hitung Ulang Telemetri Timing (Process Passings)              │
│ • Review Kasus Khusus (Pelari DNF, DNS, Missing Start)                 │
│ • Manajemen Desain Sertifikat & Publikasi Hasil Resmi                  │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 10. Observabilitas & Telemetri Real-time

Modul timing melacak metrik operasional per event tanpa membocorkan kredensial:

```json
{
  "total_passings": 12450,
  "processed_passings": 12400,
  "pending_passings": 50,
  "unmapped_passings": 12,
  "duplicate_passings_filtered": 840,
  "results_updated": 3120,
  "last_received_at": "2026-09-17T08:15:32Z",
  "last_processed_at": "2026-09-17T08:15:35Z",
  "processing_errors": 0
}
```

Panitia di lapangan dapat langsung memantau apakah sinyal matras masuk, berapa pelari yang sudah melintasi finish, dan apakah ada pelari yang menggunakan chip yang belum terdaftar di sistem.
