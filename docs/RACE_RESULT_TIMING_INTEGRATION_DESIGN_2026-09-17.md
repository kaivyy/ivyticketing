# RACE RESULT & Timing Provider Integration Design

* Document ID: `DOC-TIMING-RR-2026-09-17`
* Date: 2026-09-17
* Status: Design Approved - Ready for Implementation
* System: IvyTicketing Modular Monolith
* Target Modules: [`results`](file:///root/ivyticketing/services/api/internal/modules/results), [`tickets`](file:///root/ivyticketing/services/api/internal/modules/tickets), [`apps/web`](file:///root/ivyticketing/apps/web)

---

## 1. Executive Summary & Design Principles

Integrasi sistem timing RFID pada IvyTicketing dirancang untuk memperluas modul [`results`](file:///root/ivyticketing/services/api/internal/modules/results) tanpa menduplikasi data registrasi, tanpa membuat web results terpisah, dan tanpa mengikat sistem secara kaku ke satu vendor tertentu (vendor-agnostic).

### Core Principles
1. **Generic Provider Abstraction:** Sistem mendukung provider `RACE_RESULT`, `CSV`, dan `NATIVE` melalui interface bersama.
2. **IVY as Single Source of Truth:** Data peserta, kategori lari, dan alokasi nomor dada tetap dikelola di [`tickets.bib_number`](file:///root/ivyticketing/database/migrations/00049_add_tickets_bib_columns.sql).
3. **Decoupled Transponder Mapping:** Nomor BIB dipetakan ke kode chip/transponder secara terpisah di tabel `bib_transponder_mappings`.
4. **Raw Telemetry vs Final Scores:** Deteksi matras mentah disimpan secara persisten dan idempotent di `timing_passings`. Hasil resmi lomba disimpan di [`race_results`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql) dengan `source = 'TIMING_API'`.
5. **Dumb Adapters & Smart Agnostic Processor:** Adapter vendor hanya mengonversi data format luar ke model internal. Kalkulasi net time, gun time, verifikasi checkpoint, dan status finisher dihitung oleh `processor.go` yang netral vendor.
6. **Unified Results Experience:** Peserta dan panitia tetap menggunakan halaman hasil dan e-certificate yang sudah ada di IVY.

---

## 2. Audit Fondasi Existing & Reuse Map

| Komponen Existing | Lokasi File | Status Reuse | Catatan Integrasi |
| :--- | :--- | :--- | :--- |
| **Final Results Table** | [`database/migrations/00061_create_race_results.sql`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql) | Digunakan Langsung | Kolom `source` sudah mendukung `'TIMING_API'`, waktu tersimpan dalam milidetik (`chip_time_ms`, `gun_time_ms`). |
| **Ranking Engine** | [`services/api/internal/modules/results/service.go`](file:///root/ivyticketing/services/api/internal/modules/results/service.go#L128-L142) | Digunakan Langsung | Method `recomputeRanks` otomatis menghitung overall, gender, category, dan age group rank. |
| **BIB Allocation & Export** | [`services/api/internal/modules/tickets/bib_service.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go) | Digunakan Langsung | Alokasi nomor BIB dan streaming CSV peserta via `StreamTicketsForBibExport`. |
| **Participant Self-Service** | [`services/api/internal/modules/results/handler.go`](file:///root/ivyticketing/services/api/internal/modules/results/handler.go#L13-L16) | Digunakan Langsung | Endpoint `/tickets/{ticketId}/result` dan `/tickets/{ticketId}/certificate`. |
| **Audit Logging** | [`services/api/internal/platform/audit`](file:///root/ivyticketing/services/api/internal/platform/audit) | Digunakan Langsung | Pencatatan audit trail aksi panitia dan sinkronisasi. |
| **Web UI Dashboard** | [`apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro) | Diperluas | Menambahkan panel kontrol timing tanpa membuat halaman web terpisah. |

---

## 3. Verifikasi Mekanisme Resmi RACE RESULT 12

Berdasarkan dokumentasi teknis RACE RESULT 12, interaksi sistem dibagi menjadi 3 kanal yang berbeda:

1. **Raw Timing Ingestion (Exporters / Forwarding):**
   * Mekanisme passing stream real-time menggunakan fitur **Exporters / Forwarding** di modul Timing RACE RESULT 12.
   * Exporter dikonfigurasi dengan destination HTTP POST ke endpoint IVY (`POST /results/timing/passings`).
   * Mengirimkan field deteksi: `PassingNo`, `Transponder`, `TimingPoint`, `Time`, `Hits`, `RSSI`.
2. **Participant Data Synchronization:**
   * Ekspor data peserta ber-BIB dari IVY dalam format pertukaran CSV standar RACE RESULT 12 (`Bib`, `Firstname`, `Lastname`, `Gender`, `Birthdate`, `Contest`, `Transponder1`).
   * Opsional direct sync via Customer Web API jika organizer mengonfigurasi API Key.
3. **Simple API Clarification:**
   * Simple API pada RACE RESULT **hanya digunakan untuk read-only output lists** (melihat daftar juara atau publikasi). Tidak digunakan untuk streaming deteksi mentah.

---

## 4. End-to-End Data Pipeline Architecture

```mermaid
flowchart TD
    subgraph RR["RACE RESULT 12 Ecosystem"]
        Mats["Antena / Timing Matras"] --> Decoders["RACE RESULT Decoder"]
        Decoders --> RRSoftware["RACE RESULT 12 Software"]
        RRSoftware --> Exporter["HTTP Forwarding Exporter"]
    end

    subgraph IVY["IVY Backend (results/timing)"]
        Exporter -->|HTTP POST Payload| Handler["Timing HTTP Handler"]
        Handler --> Adapter["RaceResultAdapter (Parser)"]
        Adapter -->|[]RawObservation| RawStore[("timing_passings (Staging)")]
        
        RawStore -->|Unprocessed Passings| Dedup["Deduplication & Mapping Resolver"]
        Mappings[("bib_transponder_mappings")] -.-> Dedup
        
        Dedup --> Processor["Generic Timing Processor"]
        Checkpoints[("timing_checkpoints")] -.-> Processor
        
        Processor -->|Net & Gun Times| FinalStore[("race_results (Existing)")]
        FinalStore --> RankEngine["recomputeRanks() Engine"]
    end

    subgraph Outputs["Public & Participant Experience"]
        RankEngine --> Leaderboard["Live Leaderboard UI"]
        RankEngine --> MyResult["Participant My Result"]
        RankEngine --> ECert["E-Certificate Generator"]
    end
```

---

## 5. Database Schema Specification

Migration file: [`database/migrations/00062_create_timing_system.sql`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql)

```sql
-- +goose Up

-- 1. Konfigurasi provider timing per event
CREATE TABLE timing_configs (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    provider         text NOT NULL DEFAULT 'RACE_RESULT'
        CHECK (provider IN ('RACE_RESULT', 'CSV', 'NATIVE')),
    api_url          text,
    api_key          text,
    external_race_id text,
    settings         jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id)
);
CREATE INDEX idx_timing_configs_event ON timing_configs(event_id);

-- 2. Titik baca matras/antena (START, 5K, 10K, FINISH)
CREATE TABLE timing_checkpoints (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    code             text NOT NULL,
    name             text NOT NULL,
    checkpoint_type  text NOT NULL DEFAULT 'SPLIT'
        CHECK (checkpoint_type IN ('START', 'SPLIT', 'FINISH')),
    order_index      integer NOT NULL DEFAULT 0,
    distance_meters  integer,
    gun_time         timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, code)
);
CREATE INDEX idx_timing_checkpoints_event ON timing_checkpoints(event_id, order_index);

-- 3. Mapping nomor BIB ke nomor chip/transponder RFID
CREATE TABLE bib_transponder_mappings (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    bib_number       text NOT NULL,
    transponder_code text NOT NULL,
    is_primary       boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, transponder_code)
);
CREATE INDEX idx_bib_transponder_event_bib ON bib_transponder_mappings(event_id, bib_number);

-- 4. Buffer observasi deteksi mentah (Raw Timing Observations)
CREATE TABLE timing_passings (
    id                bigserial PRIMARY KEY,
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id          uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    checkpoint_code   text NOT NULL,
    provider          text NOT NULL,
    external_read_id  text,
    chip_code         text NOT NULL,
    bib_number        text,
    observed_at       timestamptz NOT NULL,
    received_at       timestamptz NOT NULL DEFAULT now(),
    raw_payload       text,
    metadata          jsonb NOT NULL DEFAULT '{}'::jsonb,
    processed         boolean NOT NULL DEFAULT false,
    processed_at      timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);

-- Idempotency constraints
CREATE UNIQUE INDEX uniq_timing_passings_external 
    ON timing_passings (event_id, provider, external_read_id) 
    WHERE external_read_id IS NOT NULL;

CREATE UNIQUE INDEX uniq_timing_passings_fallback 
    ON timing_passings (event_id, provider, chip_code, checkpoint_code, observed_at) 
    WHERE external_read_id IS NULL;

CREATE INDEX idx_timing_passings_unprocessed 
    ON timing_passings (event_id, processed) 
    WHERE NOT processed;

-- +goose Down
DROP TABLE IF EXISTS timing_passings;
DROP TABLE IF EXISTS bib_transponder_mappings;
DROP TABLE IF EXISTS timing_checkpoints;
DROP TABLE IF EXISTS timing_configs;
```

---

## 6. Provider Abstraction & Interface Contract

File: [`services/api/internal/modules/results/timing/provider.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/provider.go)

```go
package timing

import (
    "context"
    "time"
)

type ProviderType string

const (
    ProviderTypeRaceResult ProviderType = "RACE_RESULT"
    ProviderTypeCSV        ProviderType = "CSV"
    ProviderTypeNative     ProviderType = "NATIVE"
)

type RawObservation struct {
    ExternalReadID string         `json:"externalReadId,omitempty"`
    CheckpointCode string         `json:"checkpointCode"`
    ChipCode       string         `json:"chipCode"`
    BibNumber      *string        `json:"bibNumber,omitempty"`
    ObservedAt     time.Time      `json:"observedAt"`
    RawPayload     string         `json:"rawPayload,omitempty"`
    Metadata       map[string]any `json:"metadata,omitempty"`
}

type ParticipantSyncRecord struct {
    BibNumber       string     `json:"bibNumber"`
    ParticipantName string     `json:"participantName"`
    Gender          string     `json:"gender"`
    CategoryName    string     `json:"categoryName"`
    TransponderCode string     `json:"transponderCode,omitempty"`
    DateOfBirth     *time.Time `json:"dateOfBirth,omitempty"`
}

type TimingProvider interface {
    Type() ProviderType
    ParsePassingStream(ctx context.Context, checkpointCode string, payload []byte) ([]RawObservation, error)
    FormatParticipantExport(ctx context.Context, participants []ParticipantSyncRecord) ([]byte, error)
}
```

### Separation of Concerns:
* **`raceresult.go` (Dumb Adapter):**
  * Bertanggung jawab mem-parsing payload HTTP Forwarding dari RACE RESULT 12 menjadi `[]RawObservation`.
  * Bertanggung jawab memformat data peserta ke CSV RACE RESULT 12.
  * **Tidak mengetahui** rumus kalkulasi net time, gun time, status finisher, atau tabel `race_results`.
* **`processor.go` (Agnostic Scoring Pipeline):**
  * Mengambil raw passings dari `timing_passings`.
  * Mengasosiasikan `chip_code` dengan `bib_transponder_mappings`.
  * Memfilter read berulang (debouncing window).
  * Menghitung `chip_time_ms` dan `gun_time_ms` berdasarkan checkpoint `START` dan `FINISH`.
  * Meng-upsert hasil ke `race_results` dengan `source = 'TIMING_API'`.
  * Memanggil `recomputeRanks` yang sudah ada di modul `results`.

---

## 7. Implementation Plan

1. **Migration & Queries:**
   * Tulis migration [`database/migrations/00062_create_timing_system.sql`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql).
   * Tulis query sqlc di [`database/queries/timing.sql`](file:///root/ivyticketing/database/queries/timing.sql).
   * Jalankan migrasi dan compile model db sqlc.
2. **Timing Engine Sub-package:**
   * Implementasikan [`provider.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/provider.go), [`raceresult.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/raceresult.go), [`csv.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/csv.go), dan [`processor.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go).
   * Tulis unit test komprehensif di [`processor_test.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor_test.go).
3. **Module Wiring & Endpoints:**
   * Tambahkan metode di [`service.go`](file:///root/ivyticketing/services/api/internal/modules/results/service.go) dan handler di [`handler.go`](file:///root/ivyticketing/services/api/internal/modules/results/handler.go).
   * Daftarkan rute di [`routes.go`](file:///root/ivyticketing/services/api/internal/modules/results/routes.go).
4. **Frontend Integration:**
   * Perbarui [`results.ts`](file:///root/ivyticketing/apps/web/src/lib/results.ts) dan [`results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro) dengan panel monitoring timing.
5. **Verifikasi End-to-End:**
   * Jalankan test suite Go (`go test ./...`).
   * Verifikasi Astro build dan restart PM2.
