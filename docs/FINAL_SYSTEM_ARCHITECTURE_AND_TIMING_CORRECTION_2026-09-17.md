# Desain Arsitektur Final & Koreksi Sistem IvyTicketing

* Document ID: `DOC-FINAL-ARCHITECTURE-CORRECTION-2026-09-17`
* Date: 2026-09-17
* Version: 3.1.0
* Status: Final Design Specification (Ready for Incremental Implementation)
* Target Modules: [`orders`](file:///root/ivyticketing/services/api/internal/modules/orders), [`tickets`](file:///root/ivyticketing/services/api/internal/modules/tickets), [`results/timing`](file:///root/ivyticketing/services/api/internal/modules/results/timing), [`apps/web`](file:///root/ivyticketing/apps/web)

---

## 1. Final Architecture

```text
                       IVY RACE MANAGEMENT & TICKETING
                                      │
              ┌───────────────────────┴───────────────────────┐
              ▼                                               ▼
       Public Experience                              Workspace Management
              │                                               │
   ┌──────────┴──────────┐                         ┌──────────┴──────────┐
   ▼                     ▼                         ▼                     ▼
Guest Checkout     Participant (Opt)          Admin Studio         Organizer Results
• No fake user     • Login/Register           • Tab 10: Timing     • Race Day Ops
• Re-enter email   • Own tickets & certs      • Setup waves/mats   • Live telemetry
• Real waiver      • Claim guest orders       • Token generation   • Reprocess trigger
   │                     │                         │                     │
   └──────────┬──────────┘                         └──────────┬──────────┘
              ▼                                               ▼
      Orders & Tickets                            Timing Config Entity
      • Nullable participant_id                   • Provider, Transport, Policy
      • Single BIB Engine                         • Checkpoint Aliases & Waves
              │                                               │
              └───────────────────────┬───────────────────────┘
                                      ▼
                           Timing Ingestion Layer
                                      │
                 ┌────────────────────┼────────────────────┐
                 ▼                    ▼                    ▼
            RACE RESULT           CSV / Excel          Vendor API
          (Passing Stream)      (Raw / Final)         (Open JSON)
                 │                    │                    │
                 └────────────────────┼────────────────────┘
                                      ▼
                        Normalized Timing Contract
                                      │
                       ┌──────────────┴──────────────┐
                       ▼                             ▼
                  RAW PASSINGS                  FINAL RESULTS
                       │                             │
                       ▼                             │
               timing_passings                       │
             (Non-destructive)                       │
                       │                             │
                       ▼                             │
           Generic Scoring Processor                 │
         (Debounce, Net/Gun, Aliases)                │
                       │                             │
                       └──────────────┬──────────────┘
                                      ▼
                                 race_results
                           (Unified Official Table)
                                      │
                       ┌──────────────┴──────────────┐
                       ▼                             ▼
                  Live Results                  E-Certificate
              (Leaderboard & SSE)          (Same Template Engine)
```

### Prinsip Utama:
1. **Pemisahan Peran Antarmuka:** Admin Event Studio ([`apps/web/src/pages/admin/events/edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro)) bertindak sebagai pusat konfigurasi, sedangkan Organizer Results ([`apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro)) menangani operasional hari-H. Keduanya memakai entitas database yang sama.
2. **Scoring Processor Netral:** Modul scoring ([`services/api/internal/modules/results/timing/processor.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go)) sama sekali tidak memiliki cabang kondisi nama vendor (`no if provider == ...`).
3. **Penyimpanan Observasi Mentah:** Ingesti matras menyimpan data apa adanya di `timing_passings`. Evaluasi debouncing, filter duplikasi, validasi gelombang, dan titik rute dijalankan oleh scoring processor sehingga data mentah dapat dihitung ulang kapan saja.

---

## 2. Final Schema Corrections (Migrasi `00063_extend_timing_multi_vendor.sql`)

### A. Konsistensi Provider Constraint pada `timing_configs`
Pemeriksaan [`database/migrations/00062_create_timing_system.sql`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql#L9-L10) membuktikan konstrain lama hanya mengizinkan `('RACE_RESULT', 'CSV', 'NATIVE')`. Di migrasi `00063`, konstrain diperbarui secara ketat di level database:

```sql
-- 1. Perbaiki constraint timing_configs.provider agar konsisten dengan domain model
ALTER TABLE timing_configs
    DROP CONSTRAINT IF EXISTS timing_configs_provider_check;

ALTER TABLE timing_configs
    ADD CONSTRAINT timing_configs_provider_check
    CHECK (provider IN ('RACE_RESULT', 'GENERIC_CSV', 'VENDOR_API', 'NATIVE_RFID', 'MANUAL'));

-- 2. Tambah kolom transport, sync_mode, dan policy per-event
ALTER TABLE timing_configs
    ADD COLUMN IF NOT EXISTS transport text NOT NULL DEFAULT 'HTTP_PUSH'
        CHECK (transport IN ('HTTP_PUSH', 'HTTP_PULL', 'CSV_UPLOAD', 'LOCAL_AGENT', 'SFTP')),
    ADD COLUMN IF NOT EXISTS sync_mode text NOT NULL DEFAULT 'RAW_PASSINGS'
        CHECK (sync_mode IN ('RAW_PASSINGS', 'FINAL_RESULTS')),
    ADD COLUMN IF NOT EXISTS policy jsonb NOT NULL DEFAULT '{}'::jsonb;
```

### B. Konsistensi Sumber Hasil pada `race_results.source`
Membebaskan pembatasan kaku `CHECK (source IN ('CSV','TIMING_API'))` dari migrasi `00061` agar dapat mencatat identitas sumber hasil tanpa duplikasi tabel:

```sql
-- 3. Perluas sumber hasil resmi balapan
ALTER TABLE race_results
    DROP CONSTRAINT IF EXISTS race_results_source_check;

ALTER TABLE race_results
    ADD CONSTRAINT race_results_source_check
    CHECK (source IN ('CSV', 'RACE_RESULT', 'VENDOR_API', 'NATIVE_RFID', 'MANUAL', 'TIMING_API'));

-- 4. Perluas status balapan resmi
ALTER TABLE race_results
    DROP CONSTRAINT IF EXISTS race_results_status_check;

ALTER TABLE race_results
    ADD CONSTRAINT race_results_status_check
    CHECK (status IN ('FINISHED', 'DNF', 'DNS', 'DSQ', 'OTL'));
```

### C. Checkpoint Mapping Provider-Specific
Mencegah ambiguitas alias antar-provider dengan menyediakan pemetaan berbasis JSONB berstruktur vendor:

```sql
-- 5. Dukungan alias checkpoint global dan pemetaan provider-spesifik
ALTER TABLE timing_checkpoints
    ADD COLUMN IF NOT EXISTS aliases text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS provider_aliases jsonb NOT NULL DEFAULT '{}'::jsonb;

-- 6. Penyimpanan kode chip fisik mentah
ALTER TABLE timing_passings
    ADD COLUMN IF NOT EXISTS raw_chip_code text;
```

### D. Skema Order untuk Guest & Persetujuan Nyata (Tanpa Default True)
Audit menunjukkan bahwa satu order di IvyTicketing tepat memiliki satu tiket ([`tickets_order_unique UNIQUE (order_id)`](file:///root/ivyticketing/database/migrations/00018_create_tickets.sql#L21)), sehingga consent di level order adalah arsitektur yang 100% tepat:

```sql
-- 7. Dukungan guest checkout murni tanpa fake user
ALTER TABLE orders
    ALTER COLUMN participant_id DROP NOT NULL;

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS guest_email citext,
    ADD COLUMN IF NOT EXISTS guest_name text,
    ADD COLUMN IF NOT EXISTS guest_phone text;

ALTER TABLE orders
    ADD CONSTRAINT orders_identity_check
    CHECK (participant_id IS NOT NULL OR (guest_email IS NOT NULL AND guest_name IS NOT NULL));

-- 8. Persetujuan T&C dan Waiver wajib divalidasi server, TANPA DEFAULT true
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS terms_accepted_at timestamptz,
    ADD COLUMN IF NOT EXISTS terms_version text,
    ADD COLUMN IF NOT EXISTS waiver_accepted_at timestamptz,
    ADD COLUMN IF NOT EXISTS waiver_version text;

-- 9. Tiket mendukung pemegang guest
ALTER TABLE tickets
    ALTER COLUMN participant_id DROP NOT NULL;
```

### E. Ekstensi RBAC & Role Timing
```sql
-- 10. Permission spesifik konfigurasi dan operasional timing
INSERT INTO permissions (key, description) VALUES
    ('timing.manage', 'Configure timing providers, mat checkpoints, waves, and process passings')
ON CONFLICT (key) DO NOTHING;

-- Sambungkan ke template role Owner dan Manager
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.organization_id IS NULL AND r.slug IN ('owner', 'manager') AND p.key = 'timing.manage'
ON CONFLICT DO NOTHING;

-- Tambahkan role sistem khusus TIMING_OPERATOR
INSERT INTO roles (organization_id, name, slug, is_system) VALUES
    (NULL, 'Timing Operator', 'timing-operator', true)
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'timing.manage', 'results.manage', 'participant.view'
)
WHERE r.organization_id IS NULL AND r.slug = 'timing-operator'
ON CONFLICT DO NOTHING;
```

---

## 3. Final Guest Identity Flow

Audit terhadap [`tickets/issuer.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/issuer.go), [`payments/processor.go`](file:///root/ivyticketing/services/api/internal/modules/payments/processor.go), dan [`notifications/service.go`](file:///root/ivyticketing/services/api/internal/modules/notifications/service.go) membuktikan bahwa jika `order.participant_id` bernilai NULL, sistem harus mengalirkan data kontak guest secara aman:

1. **Checkout:**
   * Guest mengisi formulir pendaftaran, memasukkan email dua kali (konfirmasi ulang), serta menyetujui T&C dan Waiver.
   * Server memvalidasi keberadaan `guest_email`, `guest_name`, `terms_accepted_at = now()`, dan `waiver_accepted_at = now()`.
   * Order dibuat dengan `participant_id = NULL`. **Tidak ada user dummy di tabel `users`.**
2. **Penerbitan Tiket Saat Lunas (`PAID`):**
   * [`Issuer.IssueWith`](file:///root/ivyticketing/services/api/internal/modules/tickets/issuer.go#L46-L77) memeriksa:
     * Jika `order.ParticipantID != nil`: mengambil data dari `users`.
     * Jika `order.ParticipantID == nil`: langsung menggunakan `order.guest_name` dan `order.guest_email`.
   * Nomor BIB otomatis ([`AssignNextBib`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go#L95)) tetap dialokasikan pada tiket tanpa perbedaan mekanisme.
3. **Notifikasi Pembayaran & Tiket:**
   * Modul notifikasi diperluas untuk menerima `recipientEmail` dan `recipientName` langsung dari order saat `participant_id` bernilai NULL, sehingga email konfirmasi pembayaran dan e-tiket tetap terkirim tanpa gagal query `users`.
4. **Klaim Akun Peserta (Account Linking):**
   * Pengguna mendaftar/login sebagai Participant dan memanggil `POST /participant/orders/claim` dengan token verifikasi email.
   * Backend mengupdate relasi:
     * `UPDATE orders SET participant_id = $userId, guest_email = NULL WHERE id = $orderId`
     * `UPDATE tickets SET participant_id = $userId WHERE order_id = $orderId`
   * **Tidak ada order atau tiket baru yang dibuat.** Seluruh histori pembayaran, BIB, racepack, dan hasil lomba otomatis terhubung ke akun.

---

## 4. Final Provider & Capability Model

### A. Pemisahan Provider vs Transport vs Sync Mode
* **Provider (Format Data & Konversi):**
  * `RACE_RESULT`: Format Exporter/Forwarding RACE RESULT 12 (format passing stream terverifikasi).
  * `GENERIC_CSV`: Format tabular standar CSV/Excel.
  * `VENDOR_API`: Format payload JSON terbuka IVY untuk sistem timing pihak ketiga.
  * `NATIVE_RFID`: Stream antena UHF (LLRP/TCP).
  * `MANUAL`: Entri hasil manual panitia.
* **Transport (Kanal Komunikasi):**
  * `HTTP_PUSH`: Vendor mengirim data ke endpoint webhook IVY.
  * `HTTP_PULL`: IVY mengambil data berkala dari API vendor.
  * `CSV_UPLOAD`: Pengunggahan file melalui web interface.
  * `LOCAL_AGENT`: Agent lokal di area lomba yang meneruskan sinyal matras.
* **Sync Mode:**
  * `RAW_PASSINGS`: Alur deteksi antena mentah -> disimpan ke `timing_passings` -> diproses oleh generic scoring processor -> `race_results`.
  * `FINAL_RESULTS`: Alur rekap hasil resmi -> langsung di-upsert ke `race_results` -> menjalankan `recomputeRanks`.

### B. Small & Idiomatic Go Capability Interfaces
Bukan interface monster, melainkan capability terisolasi:

```go
package timing

import "context"

// PassingParser menguraikan stream deteksi antena mentah.
type PassingParser interface {
    ProviderName() string
    ParsePassings(ctx context.Context, payload []byte, defaultPoint string) ([]TimingPassing, error)
}

// ParticipantExporter mengekspor daftar pelari untuk konsol timing hardware.
type ParticipantExporter interface {
    ProviderName() string
    ExportParticipants(ctx context.Context, participants []ParticipantRecord) ([]byte, error)
}

// FinalResultParser membaca hasil akhir balapan dari vendor tanpa raw passings.
type FinalResultParser interface {
    ProviderName() string
    ParseFinalResults(ctx context.Context, payload []byte) ([]NormalizedResult, error)
}

// MappingParser mengimpor berkas asosiasi nomor BIB dengan chip transponder.
type MappingParser interface {
    ProviderName() string
    ParseChipMappings(ctx context.Context, payload []byte) ([]ChipMappingRecord, error)
}
```

### C. Capability Registry Tanpa Giant Universal Adapter
Alih-alih membuat `generic_vendor.go` sebagai adapter raksasa, kita menyediakan:
* [`services/api/internal/modules/results/timing/registry.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/registry.go): Registri ringan yang menyimpan instans adapter berdasarkan tipe provider.
* [`services/api/internal/modules/results/timing/raceresult.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/raceresult.go): Khusus parser forwarding/exporter RACE RESULT 12.
* [`services/api/internal/modules/results/timing/csv.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/csv.go): Khusus parser CSV.
* [`services/api/internal/modules/results/timing/vendor_api.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/vendor_api.go): Khusus payload JSON kontrak terbuka IVY.

---

## 5. Final Wave & Checkpoint Rules

### A. Aturan Gelombang Start & Gun Time (Tanpa Fallback Diam-diam)
Audit pada [`processor.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go#L212-L219) menemukan fallback diam-diam: `else if chipTimeMs != nil { gunTimeMs = chipTimeMs }`. Ini dikoreksi secara ketat:
1. **Wajib Waktu Flag-off:** Gun time hanya dihitung jika `race_waves.start_at` tersedia dan valid (`finish_time > wave.start_at`).
2. **Tanpa Fallback Net Time:** Jika peserta ditugaskan ke wave tetapi `wave.start_at` belum di-set panitia atau kosong:
   * `gun_time_ms` **DIBIARKAN NULL** (tidak disintesis dari net time).
   * Status hasil ditandai dengan flag operasional `REVIEW_REQUIRED_MISSING_WAVE_START`.
3. **Mode Start Policy (`start_mode`):**
   * `CHIP_PREFERRED`: Net time dihitung dari matras START. Jika matras START tidak terbaca, gunakan `wave.start_at` sebagai start jika kebijakan event mengizinkan `allow_missing_start`.
   * `GUN_ONLY`: Waktu lomba dihitung murni dari `wave.start_at`.
   * `CHIP_MANDATORY`: Pelari yang tidak memiliki deteksi matras START otomatis berstatus `DNS`.

### B. Aturan Checkpoint & Pemetaan Alias
1. Setiap checkpoint memiliki `code` resmi (misal: `START`, `SPLIT_5K`, `FINISH`).
2. Pencocokan observasi matras decoder menggunakan hierarki:
   * Cocok langsung dengan `timing_checkpoints.code`.
   * Cocok dengan array `timing_checkpoints.aliases`.
   * Cocok dengan objek provider-spesifik `timing_checkpoints.provider_aliases -> provider` (misal: `{"RACE_RESULT": ["FinishLine"], "CSV": ["Mat_2"]}`).
3. Jika titik tidak dikenali, passing dicatat dengan status `checkpoint_unresolved` tanpa membuang baris dari `timing_passings`.

---

## 6. Final Files to Change (Daftar Berkas Perubahan)

| Komponen | Berkas yang Terlibat | Tindakan yang Akan Dijalankan |
| :--- | :--- | :--- |
| **Database Migration** | [`database/migrations/00063_extend_timing_multi_vendor.sql`](file:///root/ivyticketing/database/migrations/00063_extend_timing_multi_vendor.sql) | Dibuat (constraint provider, transport, policy, alias JSONB, status DSQ/OTL, guest nullable, consent audit). |
| **Database Queries** | [`database/queries/timing.sql`](file:///root/ivyticketing/database/queries/timing.sql), [`database/queries/orders.sql`](file:///root/ivyticketing/database/queries/orders.sql), [`database/queries/tickets.sql`](file:///root/ivyticketing/database/queries/tickets.sql) | Diperbarui untuk kolom baru & query klaim order. |
| **Generated DB Code** | [`services/api/internal/db/timing.sql.go`](file:///root/ivyticketing/services/api/internal/db/timing.sql.go), [`orders.sql.go`](file:///root/ivyticketing/services/api/internal/db/orders.sql.go) | Di-generate via `sqlc generate`. |
| **Timing Interfaces** | [`services/api/internal/modules/results/timing/provider.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/provider.go) | Diperbarui ke capability interfaces (`PassingParser`, `ParticipantExporter`, dll). |
| **Timing Registry** | [`services/api/internal/modules/results/timing/registry.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/registry.go) | Dibuat (pencarian runtime adapter per provider). |
| **Vendor API Adapter** | [`services/api/internal/modules/results/timing/vendor_api.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/vendor_api.go) | Dibuat (parser payload JSON terbuka). |
| **RACE RESULT Adapter** | [`services/api/internal/modules/results/timing/raceresult.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/raceresult.go) | Diperbarui sesuai format stream Exporter/Forwarding resmi. |
| **Scoring Processor** | [`services/api/internal/modules/results/timing/processor.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go) | Diperbarui (evaluasi alias JSONB, aturan ketat gun time wave, policy debounce dinamis). |
| **Timing Service & API** | [`services/api/internal/modules/results/timing_service.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing_service.go), [`timing_handler.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing_handler.go) | Diperbarui (dukungan sync_mode final results, DTO transport & policy). |
| **Ticket Issuer** | [`services/api/internal/modules/tickets/issuer.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/issuer.go) | Diperbarui agar aman menangani order guest tanpa user lookup. |
| **Order Service & Routes** | [`services/api/internal/modules/orders/service.go`](file:///root/ivyticketing/services/api/internal/modules/orders/service.go), [`handler.go`](file:///root/ivyticketing/services/api/internal/modules/orders/handler.go), [`routes.go`](file:///root/ivyticketing/services/api/internal/modules/orders/routes.go) | Diperbarui (checkout publik, validasi server-side waiver, API claim order). |
| **Admin Event Studio** | [`apps/web/src/pages/admin/events/edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro) | Diperbarui (Tab 10: Timing & Race RFID). |
| **Frontend Checkout** | [`apps/web/src/pages/events/[eventId]/checkout.astro`](file:///root/ivyticketing/apps/web/src/pages/events/[eventId]/checkout.astro), [`apps/web/src/lib/checkout.ts`](file:///root/ivyticketing/apps/web/src/lib/checkout.ts) | Diperbarui (input konfirmasi email, checkbox waiver/T&C nyata, pemanggilan API guest). |
| **Shared Results Client** | [`apps/web/src/lib/results.ts`](file:///root/ivyticketing/apps/web/src/lib/results.ts) | Diperbarui (TypeScript interface sinkron dengan DTO backend baru). |

---

## 7. Hal yang Sengaja Tidak Dibuat Karena Existing Code Sudah Cukup

1. **TIDAK membuat service BIB baru:** Seluruh operasi nomor dada tetap menggunakan [`AssignNextBib`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go#L95), [`SetBib`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go#L145), dan [`BulkAssignBib`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go#L205).
2. **TIDAK membuat tabel hasil lomba kedua:** Hasil resmi seluruh vendor (baik passing mentah maupun hasil akhir) tetap bermuara pada [`race_results`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql).
3. **TIDAK membuat engine ranking kedua:** Perhitungan peringkat overall, gender, dan kategori tetap menggunakan [`recomputeRanks`](file:///root/ivyticketing/services/api/internal/modules/results/service.go#L128-L142).
4. **TIDAK membuat antarmuka web Results terpisah:** Seluruh tampilan hasil publik dan operasional race-day tetap menggunakan [`results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro) dan rute peserta.
5. **TIDAK membuat Event Builder terpisah:** Konfigurasi timing ditempatkan sebagai Tab 10 di Admin Event Studio ([`apps/web/src/pages/admin/events/edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro)).
6. **TIDAK membuat endpoint API duplikat khusus Admin vs Organizer:** Keduanya memanggil endpoint backend yang sama (`/organizations/{orgId}/events/{eventId}/results/timing/*`), di mana Platform Admin secara otomatis memperoleh izin via bypass [`RequirePermission`](file:///root/ivyticketing/services/api/internal/platform/middleware/authz.go#L36).
7. **TIDAK membuat tabel akun pengguna terpisah:** Tidak ada tabel `guest_users` atau `organizer_users`. Seluruh akun terdaftar tetap menggunakan tabel [`users`](file:///root/ivyticketing/database/migrations/00002_create_users.sql).
