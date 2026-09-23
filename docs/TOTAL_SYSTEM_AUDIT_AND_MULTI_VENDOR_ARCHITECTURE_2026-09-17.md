# Total System Audit & Arsitektur Integrasi Multi-Vendor Race Timing

* Document ID: `DOC-TOTAL-AUDIT-MULTI-VENDOR-2026-09-17`
* Date: 2026-09-17
* Version: 2.1.0
* Status: Complete Pre-Implementation System Audit & Architecture Blueprint
* Target Modules: [`auth`](file:///root/ivyticketing/services/api/internal/modules/auth), [`users`](file:///root/ivyticketing/database/migrations/00002_create_users.sql), [`organizations`](file:///root/ivyticketing/services/api/internal/modules/organizations), [`orders`](file:///root/ivyticketing/services/api/internal/modules/orders), [`tickets`](file:///root/ivyticketing/services/api/internal/modules/tickets), [`results`](file:///root/ivyticketing/services/api/internal/modules/results), [`apps/web`](file:///root/ivyticketing/apps/web)

---

## 1. Executive Summary & Vision

IvyTicketing diposisikan sebagai platform terpadu untuk ajang olahraga dan lomba lari (road race, trail, marathon, triathlon, cycling, fun run). Sistem ini menggabungkan modul registrasi, tiket, alokasi nomor dada (BIB), pengambilan paket lomba (racepack), dan integrasi pencatatan waktu lomba fisik (RFID/chip timing) dalam satu arsitektur modular monolith tanpa dependensi eksternal yang kaku.

```text
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

### Prinsip Inti Perancangan:
1. **Zero Rewrite & Strict Reuse:** Memanfaatkan seluruh entitas database, query SQLC, dan service domain yang telah terbukti stabil.
2. **Identitas Pengguna Tunggal:** Satu akun `users` dapat bertindak sebagai Participant, Organizer Staff di berbagai organisasi, maupun Platform Admin.
3. **Guest Checkout Tanpa Hambatan:** Pengguna dapat membeli tiket lomba tanpa wajib login atau membuat password di awal alur pendaftaran.
4. **Pemisahan Tegas Antara Provider, Transport, dan Sync Mode:** Format data decoder terpisah dari kanal transmisi jaringan.
5. **Non-Destructive Ingestion:** Data observasi antena mentah disimpan utuh di `timing_passings` untuk memastikan kemampuan audit forensik dan hitung ulang (*reprocessing*).
6. **Provider-Agnostic Scoring Engine:** Logika pemeringkatan dan penilaian waktu (*net time*, *gun time*, *split pace*) bersifat netral vendor.

---

## 2. What Already Exists (Audit Komponen Existing)

### A. Backend & Platform
* **Identitas & Autentikasi:**
  * Tabel [`users`](file:///root/ivyticketing/database/migrations/00002_create_users.sql) dengan kolom `password_hash text NULL` (mendukung pengguna tanpa password).
  * Penanda [`is_platform_admin boolean DEFAULT false`](file:///root/ivyticketing/database/migrations/00002_create_users.sql#L11).
  * JWT signer dan verifier di [`services/api/internal/platform/security/jwt.go`](file:///root/ivyticketing/services/api/internal/platform/security/jwt.go).
  * Token refresh dan sesi di [`database/migrations/00005_create_refresh_tokens.sql`](file:///root/ivyticketing/database/migrations/00005_create_refresh_tokens.sql).
* **Organisasi & RBAC:**
  * Tabel [`organizations`](file:///root/ivyticketing/database/migrations/00003_create_organizations.sql) dan [`organization_members`](file:///root/ivyticketing/database/migrations/00003_create_organizations.sql#L10-L16).
  * Tabel [`roles`](file:///root/ivyticketing/database/migrations/00004_create_rbac.sql#L2-L10), [`permissions`](file:///root/ivyticketing/database/migrations/00004_create_rbac.sql#L16-L20), dan [`member_roles`](file:///root/ivyticketing/database/migrations/00004_create_rbac.sql#L28-L32).
  * Seed role sistem (`owner`, `manager`, `finance`, `customer-service`, `racepack-staff`) di [`00007_seed_rbac_catalog.sql`](file:///root/ivyticketing/database/migrations/00007_seed_rbac_catalog.sql).
  * Permission hasil lomba `results.manage` di [`00061_create_race_results.sql`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql#L71-L81).
  * Middleware otorisasi [`RequirePermission`](file:///root/ivyticketing/services/api/internal/platform/middleware/authz.go#L21-L57) dengan bypass otomatis untuk Platform Admin.
* **Tiket & Penomoran BIB:**
  * Tabel [`tickets`](file:///root/ivyticketing/database/migrations/00018_create_tickets.sql) dengan konstrain unik relasi order `tickets_order_unique`.
  * Kolom BIB lomba di [`database/migrations/00049_add_tickets_bib_columns.sql`](file:///root/ivyticketing/database/migrations/00049_add_tickets_bib_columns.sql) dengan indeks parsial unik `uniq_tickets_event_bib`.
  * Service nomor BIB di [`services/api/internal/modules/tickets/bib_service.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go) (`AssignNextBib`, `SetBib`, `BulkAssignBib`, `StreamTicketsForBibExport`).
* **Hasil Lomba & Sertifikat:**
  * Tabel [`race_results`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql#L10-L38) dan [`certificate_templates`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql#L50-L69).
  * Algoritma perankingan otomatis [`recomputeRanks`](file:///root/ivyticketing/services/api/internal/modules/results/service.go#L128-L142).
* **Sistem Timing Lomba (Migrasi 62):**
  * Tabel konfigurasi [`timing_configs`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql#L5-L16).
  * Tabel gelombang start [`race_waves`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql#L25-L38).
  * Tabel matras deteksi [`timing_checkpoints`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql#L45-L64).
  * Tabel relasi transponder [`bib_transponder_mappings`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql#L71-L90).
  * Tabel observasi mentah [`timing_passings`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql#L97-L125).
  * Tabel split per titik [`race_split_times`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql#L127-L140).

### B. Frontend
* **Admin Event Studio:** [`apps/web/src/pages/admin/events/edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro) dengan 9 tab (Info, Tata Letak, Judul, Blok Bebas, Kategori, Rute, Racepack, Jadwal, Formulir).
* **Organizer Results Dashboard:** [`apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro) (Telemetri live, unmapped chips, perankingan, publikasi sertifikat).
* **Participant Results & Sertifikat:** [`apps/web/src/pages/participant/certificate/[ticketId].astro`](file:///root/ivyticketing/apps/web/src/pages/participant/certificate/[ticketId].astro).

---

## 3. What Should Be Reused (Komponen yang Wajib Digunakan Kembali)

| Komponen | Lokasi File | Peran & Alasan Penggunaan Kembali |
| :--- | :--- | :--- |
| **Identitas Pengguna** | [`database/migrations/00002_create_users.sql`](file:///root/ivyticketing/database/migrations/00002_create_users.sql) | Sumber tunggal data user (Participant, Organizer, Admin). |
| **RBAC Scoped Organisasi** | [`services/api/internal/platform/middleware/authz.go`](file:///root/ivyticketing/services/api/internal/platform/middleware/authz.go) | Otentikasi dan otorisasi multi-tenant dengan isolasi data antar organisasi. |
| **Penerbit Tiket** | [`services/api/internal/modules/tickets/issuer.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/issuer.go) | Menjamin penerbitan tiket dan pembuatan nomor tiket yang konsisten setelah pembayaran selesai. |
| **Alokasi BIB Lomba** | [`services/api/internal/modules/tickets/bib_service.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go) | Sumber tunggal penomoran dada peserta (`AUTO`, `MANUAL`, `BULK`). |
| **Tabel Hasil Resmi** | [`database/migrations/00061_create_race_results.sql`](file:///root/ivyticketing/database/migrations/00061_create_race_results.sql) | Muara akhir seluruh hasil lomba resmi dan dasar pembuatan sertifikat. |
| **Mesin Pemeringkat** | [`services/api/internal/modules/results/service.go`](file:///root/ivyticketing/services/api/internal/modules/results/service.go) | Perhitungan peringkat overall, gender, dan kategori secara server-side. |
| **Penyimpanan Observasi** | [`timing_passings`](file:///root/ivyticketing/database/migrations/00062_create_timing_system.sql#L97) | Buffer data mentah dari decoder antena dengan indeks idempotensi ganda. |
| **Shared API Client** | [`apps/web/src/lib/results.ts`](file:///root/ivyticketing/apps/web/src/lib/results.ts) | Menghubungkan antarmuka Admin dan Organizer ke endpoint backend yang sama. |

---

## 4. What Must Be Extended (Komponen yang Perlu Diperluas)

1. **Migrasi Database `00063_extend_timing_multi_vendor.sql`:**
   * `timing_configs`: Menambahkan kolom `transport text NOT NULL DEFAULT 'HTTP_PUSH'`, `sync_mode text NOT NULL DEFAULT 'RAW_PASSINGS'`, dan `policy jsonb NOT NULL DEFAULT '{}'::jsonb`.
   * `timing_checkpoints`: Menambahkan kolom `aliases text[] NOT NULL DEFAULT '{}'` untuk alias nama matras decoder vendor.
   * `timing_passings`: Menambahkan kolom `raw_chip_code text` untuk kode asli hardware sebelum normalisasi.
   * `race_results`: Memperluas constraint status balapan agar mencakup `'DSQ'` (Disqualified) dan `'OTL'` (Over Time Limit).
   * `orders`: Menambahkan audit persetujuan syarat ketentuan dan dokumen waiver (`waiver_accepted_at timestamptz`, `terms_accepted boolean DEFAULT true`).
2. **Pemisahan Interface Capability Provider:**
   * Mengganti interface tunggal dengan capability terisolasi: `PassingParser`, `ParticipantExporter`, `FinalResultParser`, `MappingParser`.
3. **Registry Provider Dinamis:**
   * Registri runtime untuk memetakan nama vendor ke kumpulan kemampuan adapter masing-masing.
4. **Adapter Vendor Generic:**
   * Adapter JSON terbuka untuk menerima passing stream atau rekap hasil akhir dari vendor pihak ketiga mana pun.
5. **Kebijakan Scoring Dinamis:**
   * Pengaturan per event via JSONB: `debounce_window_seconds`, `cutoff_minutes`, `start_mode` (`CHIP_PREFERRED`, `GUN_ONLY`, `CHIP_MANDATORY`), dan `allow_missing_start`.
6. **Dukungan Guest Checkout pada Backend:**
   * Endpoint publik pemesanan tiket yang secara transparan membuat akun user guest tanpa password di tabel `users`.
7. **Admin Event Studio:**
   * Menambahkan Tab 10: Timing & Race RFID (`c-tab-timing`) pada [`edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro).

---

## 5. What Is Missing (Komponen yang Belum Ada)

1. Endpoint checkout publik tanpa kewajiban autentikasi Bearer token.
2. Form verifikasi ulang email (*email re-entry*) dan waiver consent pada antarmuka checkout publik.
3. Parser langsung untuk vendor yang hanya mengunggah rekap hasil akhir (*final results without raw passings*).
4. Mekanisme pengaitan pesanan guest ke akun peserta (*claim / link guest orders*).
5. Tab 10 di Admin Event Studio untuk konfigurasi provider, gelombang, matras, dan token ingesti.

---

## 6. What Must NOT Be Created (Larangan Keras)

1. **DILARANG membuat tabel akun terpisah:** Tidak boleh membuat tabel `organizer_users`, `admin_accounts`, atau `guest_participants`.
2. **DILARANG membuat generator BIB kedua:** Nomor BIB wajib selalu bersumber dari [`tickets.bib_number`](file:///root/ivyticketing/database/migrations/00049_add_tickets_bib_columns.sql).
3. **DILARANG membuat portal web Results kedua:** Seluruh tampilan hasil publik dan operasional menggunakan halaman yang sudah ada.
4. **DILARANG meletakkan logika scoring pada adapter vendor:** Adapter hanya bertugas menerjemahkan format data luar ke model internal.
5. **DILARANG membuat Event Builder kedua:** Semua kustomisasi event berada di Admin Event Studio ([`apps/web/src/pages/admin/events/edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro)).
6. **DILARANG membuat endpoint API duplikat khusus Admin vs Organizer:** Keduanya memanggil endpoint yang sama dengan otorisasi berbasis context.
7. **DILARANG membuang data observasi mentah sebelum disimpan:** Semua passing valid teknis wajib masuk ke `timing_passings`.

---

## 7. Collision & Duplication Risks

| Potensi Bentrok | Dampak Buruk | Solusi Mitigasi Aman |
| :--- | :--- | :--- |
| **Menjadikan `orders.participant_id` bernilai NULL** | Query SQLC join eksisting akan menghasilkan error atau data tidak konsisten. | **Guest Identity Pattern:** Buat record user baru di tabel `users` dengan `password_hash = NULL`. Integritas foreign key tetap utuh 100%. |
| **Tumpang Tindih Mapping Transponder** | Chip yang sama terdaftar pada dua pelari berbeda dalam satu event. | Konstrain unik parsial `uniq_bib_chip_active ON (event_id, chip_code) WHERE is_active`. |
| **Kebocoran Kredensial Ingesti** | Token rahasia timing terlihat pada inspect network browser. | Token disimpan dalam bentuk SHA-256 hash (`auth_token_hash`). API read hanya mengembalikan token bertopeng (masking). |
| **Passing Terkirim Berulang Kali** | Data passing ganda memicu perhitungan waktu berantakan. | Indeks unik idempotensi `uniq_timing_passings_external` dan filter debounce non-destruktif pada scoring processor. |

---

## 8. Arsitektur Guest Checkout & Akun Peserta Opsional

```text
Public Event Page
       ↓
Pilih Kategori Tiket
       ↓
Isi Formulir Pelari (Form Fields Kustom)
       ↓
Halaman Checkout
  ├── Konfirmasi / Re-enter Email
  ├── Persetujuan Syarat & Ketentuan
  └── Dokumen Waiver Pelepasan Tanggung Jawab
       ↓
POST /api/v1/public/.../checkout
  ├── Backend membuat / menemukan record di tabel `users` (password_hash = NULL)
  ├── Membuat record di tabel `orders` (status = PENDING_PAYMENT)
  └── Mereservasi kuota di tabel `inventory_reservations`
       ↓
Pembayaran Gateway (QRIS / VA / E-Wallet)
       ↓
Webhook Pembayaran Berhasil (PAID)
       ↓
Ticket Issuer menerbitkan Tiket (Status = VALID)
       ↓
Alokasi Nomor Dada Otomatis (AUTO BIB)
       ↓
Pengiriman Email Notifikasi (Tiket, QR Code, Panduan Racepack)
```

### Mekanisme Pengaitan Akun (Account Linking):
Jika pembeli guest di kemudian hari memutuskan untuk membuat akun atau login:
1. Pengguna mendaftar dengan alamat email yang sama.
2. Sistem mendeteksi akun guest (`password_hash IS NULL`) dan mengirimkan tautan verifikasi/OTP.
3. Setelah diverifikasi, `password_hash` diisi dan status akun menjadi penuh.
4. Seluruh riwayat order, tiket, nomor BIB, racepack, dan sertifikat masa lalu tetap terikat pada `user_id` yang sama tanpa pembuatan record baru.

---

## 9. Sistem Identitas, Organisasi & RBAC Multi-Tenant

Sistem menerapkan prinsip **Satu Identitas Pengguna, Beragam Konteks Peran**:

```text
                             User (Budi)
                                  │
          ┌───────────────────────┼───────────────────────┐
          ▼                       ▼                       ▼
  Participant Role        Organization A          Organization B
  • Lihat Tiket Saya      • Role: OWNER           • Role: TIMING_OPERATOR
  • Unduh Sertifikat      • Akses Penuh Event     • Kelola Matras & Chip
  • Riwayat Pesanan       • Akses Billing         • Pantau Telemetri Live
                          • Kelola Tim            (Ditolak dari Billing)
```

### Matriks Otorisasi Berdasarkan Peran:
| Role Organisasi | Kelola Event | Registrasi & BIB | Operasi Timing | Hasil & Sertifikat | Kelola Anggota & Billing |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **OWNER** | Ya | Ya | Ya | Ya | Ya |
| **ADMIN / MANAGER** | Ya | Ya | Ya | Ya | Tidak |
| **TIMING_OPERATOR** | Tidak | Tidak | Ya | Lihat Saja | Tidak |
| **RESULTS_MANAGER** | Tidak | Tidak | Proses | Ya (Publikasi) | Tidak |
| **CHECKIN_STAFF** | Tidak | Tidak | Tidak | Tidak | Tidak |
| **PLATFORM_ADMIN** | Global Bypass | Global Bypass | Global Bypass | Global Bypass | Global Bypass |

---

## 10. Admin Event Studio Integration (Tab 10: Timing & Race RFID)

Di dalam [`apps/web/src/pages/admin/events/edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro), Tab 10 ditambahkan secara harmonis melengkapi 9 tab yang ada:

```text
┌────────────────────────────────────────────────────────────────────────┐
│ Tab 10: TIMING & RACE RFID                                             │
├────────────────────────────────────────────────────────────────────────┤
│ [x] Aktifkan Integrasi Timing Lomba                                    │
│                                                                        │
│ Pilihan Provider:             Metode Transport:                        │
│ (•) RACE RESULT 12            (•) HTTP Push (Real-time Mat Stream)     │
│ ( ) Vendor Timing Pihak Ke-3  ( ) CSV Upload (File Hasil Akhir)        │
│ ( ) Generic CSV               ( ) Local Agent (TCP/LLRP Antena)        │
│                                                                        │
│ URL Target Ingesti:                                                    │
│ https://api.ivy.run/api/v1/organizations/{orgId}/events/{eventId}/...  │
│ Token Kredensial Ingesti:                                              │
│ [ ivy_live_7a8f9c2d1e0b... ]  [ Generate Ulang Token ]                 │
│                                                                        │
│ Pengaturan Gelombang Start (Race Waves):                               │
│ • Wave 1 (Elite): 05:30 WIB                                            │
│ • Wave 2 (Corral A): 05:40 WIB                                         │
│ • Wave 3 (Corral B): 05:50 WIB                                         │
│                                                                        │
│ Titik Matras & Alias Sensor:                                           │
│ • START      Alias: ["StartLine", "GunMat", "Mat_0"]                   │
│ • CP_5K      Alias: ["Split5K", "Km5", "Mat_1"]                        │
│ • FINISH     Alias: ["FinishLine", "FinishMat", "Mat_2"]               │
│                                                                        │
│ Kebijakan Scoring Lomba:                                               │
│ • Jendela Debounce: [ 15 ] detik                                       │
│ • Mode Start: [ Chip Preferred (Fallback Wave Gun Time) ]              │
│ • Batas Waktu Cutoff: [ 420 ] menit                                    │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 11. Multi-Provider Domain Design

Sistem membedakan secara tegas antara vendor sistem dengan metode pengiriman paket data:

### 1. Provider (Format Konversi Data)
* `RACE_RESULT`: Format Exporter / Passing Stream dari software RACE RESULT 12.
* `GENERIC_CSV`: Format spreadsheet standar (`bib,checkpoint,time` atau `chip,checkpoint,time`).
* `API_VENDOR`: Vendor pihak ketiga yang mengirimkan payload JSON terstruktur sesuai spesifikasi API terbuka IVY.
* `NATIVE_RFID`: Antena pembaca UHF RFID (LLRP / TCP reader box) yang mengirim deteksi EPC mentah.
* `MANUAL`: Pencatatan waktu manual oleh petugas lapangan.

### 2. Transport (Kanal Pengiriman Data)
* `HTTP_PUSH`: Vendor / Exporter mendorong data real-time via HTTP POST ke endpoint IVY.
* `HTTP_PULL`: Worker IVY menarik data berkala dari server vendor via REST API.
* `CSV_UPLOAD`: Panitia mengunggah file passing/hasil secara manual melalui web browser.
* `SFTP`: File log diletakkan pada folder SFTP dan diproses oleh background worker.
* `LOCAL_AGENT`: Perangkat lunak perantara lokal di lokasi lomba yang mem-forward sinyal ke cloud.

### 3. Sync Mode (Target Alur Pemrosesan)
* `RAW_PASSINGS`: Data yang masuk adalah deteksi antena mentah -> disimpan ke `timing_passings` -> diproses oleh generic scoring processor -> `race_results`.
* `FINAL_RESULTS`: Data yang masuk adalah hasil akhir resmi dari vendor timing eksternal -> langsung di-upsert ke `race_results` -> menjalankan `recomputeRanks`.

---

## 12. Capability-Based Provider Design (Go Interfaces)

Pemisahan interface berukuran kecil dan idiomatik Go mencegah terbentuknya *interface monster*:

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

// ParticipantExporter diimplementasikan jika vendor membutuhkan daftar pelari untuk konsol timing.
type ParticipantExporter interface {
    ProviderName() string
    ExportParticipants(ctx context.Context, participants []ParticipantRecord) ([]byte, error)
}

// FinalResultParser diimplementasikan jika vendor hanya mengunggah hasil akhir balapan.
type FinalResultParser interface {
    ProviderName() string
    ParseFinalResults(ctx context.Context, payload []byte) ([]NormalizedResult, error)
}

// MappingParser diimplementasikan jika vendor menyediakan file asosiasi nomor BIB ke chip RFID.
type MappingParser interface {
    ProviderName() string
    ParseChipMappings(ctx context.Context, payload []byte) ([]ChipMappingRecord, error)
}
```

### Matriks Kemampuan Vendor:
| Provider | PassingParser | ParticipantExporter | FinalResultParser | MappingParser |
| :--- | :---: | :---: | :---: | :---: |
| **RACE_RESULT** | Ya | Ya (CSV RR12) | Tidak | Ya |
| **GENERIC_CSV** | Ya | Ya (CSV Umum) | Ya | Ya |
| **API_VENDOR** | Ya (JSON) | Ya (JSON) | Ya (JSON) | Ya |
| **NATIVE_RFID** | Ya (LLRP Stream) | Tidak | Tidak | Tidak |
| **MANUAL** | Tidak | Tidak | Ya | Tidak |

---

## 13. Normalized Timing Contract

Semua adapter menerjemahkan payload eksternal ke dalam format Go typed berikut:

```go
package timing

import (
    "time"
)

type TimingPassing struct {
    ExternalID      string           `json:"externalId,omitempty"`
    CheckpointCode  string           `json:"checkpointCode"`
    ChipCode        string           `json:"chipCode"`
    RawChipCode     string           `json:"rawChipCode,omitempty"`
    BibNumber       *string          `json:"bibNumber,omitempty"`
    ObservedAt      time.Time        `json:"observedAt"`
    ReceivedAt      time.Time        `json:"receivedAt"`
    SourceProvider  string           `json:"sourceProvider"`
    TransportMethod string           `json:"transportMethod"`
    RawPayload      string           `json:"rawPayload,omitempty"`
    Telemetry       PassingTelemetry `json:"telemetry,omitempty"`
}

type PassingTelemetry struct {
    Hits     int    `json:"hits,omitempty"`
    RSSI     int    `json:"rssi,omitempty"`
    Antenna  int    `json:"antenna,omitempty"`
    DeviceID string `json:"deviceId,omitempty"`
}

type NormalizedResult struct {
    BibNumber       string     `json:"bibNumber"`
    ParticipantName string     `json:"participantName"`
    Gender          string     `json:"gender"`
    Age             *int       `json:"age,omitempty"`
    AgeGroup        string     `json:"ageGroup,omitempty"`
    Status          string     `json:"status"` // FINISHED, DNF, DNS, DSQ, OTL
    ChipTimeMs      *int64     `json:"chipTimeMs,omitempty"`
    GunTimeMs       *int64     `json:"gunTimeMs,omitempty"`
    FinishedAt      *time.Time `json:"finishedAt,omitempty"`
}
```

---

## 14. Tiga Model Integrasi BIB & Chip di Lapangan

1. **Model A (IVY Menentukan BIB):**
   * IVY mengalokasikan nomor dada otomatis (`tickets.bib_number`).
   * Panitia mengekspor data peserta via `ParticipantExporter`.
   * Vendor menempelkan transponder fisik sesuai daftar dan mengunggah kembali file pemetaan chip.
2. **Model B (Vendor Menentukan Chip On-Demand di RPC):**
   * Nomor dada telah ada di tiket.
   * Saat pelari mengambil racepack di RPC, petugas memindai barcode BIB fisik dan chip transponder acak.
   * Hubungan tersimpan instan via API `POST /timing/mappings`.
3. **Model C (Vendor Menyediakan Paket Pre-Mapped):**
   * Vendor menyiapkan nomor dada yang telah ditempeli chip dengan kode tetap.
   * Panitia mengimpor file asosiasi sebelum lomba dimulai.
   * Sistem mengaktifkan mapping secara massal.

---

## 15. Skema Migrasi Database Final (`00063_extend_timing_multi_vendor.sql`)

```sql
-- +goose Up
-- 1. Tambah kolom transport, sync_mode, policy ke timing_configs
ALTER TABLE timing_configs
    ADD COLUMN IF NOT EXISTS transport text NOT NULL DEFAULT 'HTTP_PUSH',
    ADD COLUMN IF NOT EXISTS sync_mode text NOT NULL DEFAULT 'RAW_PASSINGS',
    ADD COLUMN IF NOT EXISTS policy jsonb NOT NULL DEFAULT '{}'::jsonb;

-- 2. Tambah kolom aliases ke timing_checkpoints
ALTER TABLE timing_checkpoints
    ADD COLUMN IF NOT EXISTS aliases text[] NOT NULL DEFAULT '{}';

-- 3. Tambah kolom raw_chip_code ke timing_passings
ALTER TABLE timing_passings
    ADD COLUMN IF NOT EXISTS raw_chip_code text;

-- 4. Perluas status pada race_results untuk DSQ dan OTL
ALTER TABLE race_results
    DROP CONSTRAINT IF EXISTS race_results_status_check;

ALTER TABLE race_results
    ADD CONSTRAINT race_results_status_check
    CHECK (status IN ('FINISHED', 'DNF', 'DNS', 'DSQ', 'OTL'));

-- 5. Tambah audit waiver & T&C consent pada orders
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS terms_accepted boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS waiver_accepted_at timestamptz;

-- +goose Down
ALTER TABLE orders
    DROP COLUMN IF EXISTS waiver_accepted_at,
    DROP COLUMN IF EXISTS terms_accepted;

ALTER TABLE race_results
    DROP CONSTRAINT IF EXISTS race_results_status_check;

ALTER TABLE race_results
    ADD CONSTRAINT race_results_status_check
    CHECK (status IN ('FINISHED', 'DNF', 'DNS'));

ALTER TABLE timing_passings
    DROP COLUMN IF EXISTS raw_chip_code;

ALTER TABLE timing_checkpoints
    DROP COLUMN IF EXISTS aliases;

ALTER TABLE timing_configs
    DROP COLUMN IF EXISTS policy,
    DROP COLUMN IF EXISTS sync_mode,
    DROP COLUMN IF EXISTS transport;
```

---

## 16. Roadmap Implementasi Bertahap (Implementation Steps)

1. **Langkah 1: Database Schema & SQLC (`00063`)**
   * Buat dan jalankan migrasi `00063_extend_timing_multi_vendor.sql`.
   * Perbarui query [`database/queries/timing.sql`](file:///root/ivyticketing/database/queries/timing.sql) dan jalankan `sqlc generate`.
2. **Langkah 2: Capability Interfaces & Registry Engine**
   * Refactor [`provider.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/provider.go) dan buat [`registry.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/registry.go).
   * Implementasikan [`generic_vendor.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/generic_vendor.go).
3. **Langkah 3: Generic Processor Scoring Policy**
   * Sesuaikan [`processor.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go) untuk evaluasi alias matras, dynamic debounce, dan status `DSQ`/`OTL`.
4. **Langkah 4: Service & Handler Timing Update**
   * Lengkapi [`timing_service.go`](file:///root/ivyticketing/services/api/internal/modules/results/timing_service.go) untuk mendukung `sync_mode == 'FINAL_RESULTS'`.
5. **Langkah 5: Admin Event Studio UI (Tab 10)**
   * Tambahkan Tab 10 di [`apps/web/src/pages/admin/events/edit.astro`](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro) beserta event listener dan integrasi API [`results.ts`](file:///root/ivyticketing/apps/web/src/lib/results.ts).
6. **Langkah 6: Guest Checkout API & UI**
   * Buat rute guest checkout publik dan perbarui [`checkout.ts`](file:///root/ivyticketing/apps/web/src/lib/checkout.ts) dan [`checkout.astro`](file:///root/ivyticketing/apps/web/src/pages/events/[eventId]/checkout.astro).
7. **Langkah 7: Pengujian & Verifikasi Menyeluruh**
   * Jalankan contract test, unit test (`go test ./...`), build frontend (`pnpm build`), dan pastikan zero regression.
