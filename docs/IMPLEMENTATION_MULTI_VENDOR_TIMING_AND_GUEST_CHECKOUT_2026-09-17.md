# Dokumentasi Implementasi Multi-Vendor Race Timing & Guest Checkout

* Dokumen: `DOC-IMPLEMENTATION-TIMING-GUEST-CHECKOUT-2026-09-17`
* Tanggal: 2026-09-17
* Versi: 1.0.0
* Status: Terimplementasi & Terverifikasi (Production Ready)
* Modul Terkait: [`services/api/internal/modules/results/timing`](file:///root/ivyticketing/services/api/internal/modules/results/timing), [`services/api/internal/modules/orders`](file:///root/ivyticketing/services/api/internal/modules/orders), [`services/api/internal/modules/tickets`](file:///root/ivyticketing/services/api/internal/modules/tickets), [`apps/web`](file:///root/ivyticketing/apps/web)

---

## 1. Ikhtisar Arsitektur

IvyTicketing kini mendukung operasional race management dan ticketing terpadu dengan arsitektur timing agnostik vendor dan checkout tamu tanpa pembuatan akun palsu (fake user).

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
• No fake user     • Login/Register           • Tab 10: Timing     • Live telemetry
• Re-enter email   • Own tickets & certs      • Setup mats/aliases • Recompute trigger
• Real waiver      • Claim guest orders       • Token generation   • DSQ/OTL badges
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
```

### Prinsip Utama
1. **Pemisahan Provider vs Transport**: Jenis vendor timing (misal: RACE RESULT, Vendor API, CSV) terpisah dari protokol pengiriman data (HTTP Push, HTTP Pull, CSV Upload, Local Edge Agent, SFTP).
2. **Non-Destructive Raw Passings**: Seluruh deteksi matras chip dicatat di [`timing_passings`](file:///root/ivyticketing/database/migrations/00063_extend_timing_multi_vendor.sql) lengkap dengan `raw_chip_code`. Data mentah tidak pernah dimutasi saat proses scoring diulang.
3. **Strict Wave Gun Time**: Waktu tembakan start wave resmi digunakan jika matras start terlewati. Tidak ada fabrikasi net time palsu.
4. **Single BIB Engine**: Penomoran BIB tetap terpusat pada [`tickets.bib_number`](file:///root/ivyticketing/services/api/internal/modules/tickets/service.go) (`AssignNextBib`, `SetBib`, `BulkAssignBib`). Timing hanya bertugas memetakan BIB ke kode chip RFID.
5. **Guest Checkout Tanpa Akun Palsu**: Kolom `participant_id` bersifat opsional (nullable). Guest checkout tidak membuat baris pengguna dengan password kosong. Pengguna tamu dapat mengklaim pesanan mereka ke akun terdaftar kapan saja.

---

## 2. Perubahan Skema Database

Migration: [`database/migrations/00063_extend_timing_multi_vendor.sql`](file:///root/ivyticketing/database/migrations/00063_extend_timing_multi_vendor.sql)

### A. Tabel `timing_configs`
* `provider`: Diperluas dengan check constraint `CHECK (provider IN ('RACE_RESULT', 'GENERIC_CSV', 'VENDOR_API', 'NATIVE_RFID', 'MANUAL'))`.
* `transport`: Kolom protokol `VARCHAR(32) NOT NULL DEFAULT 'HTTP_PUSH'` dengan check constraint `CHECK (transport IN ('HTTP_PUSH', 'HTTP_PULL', 'CSV_UPLOAD', 'LOCAL_AGENT', 'SFTP'))`.
* `sync_mode`: Kolom mode sinkronisasi `VARCHAR(32) NOT NULL DEFAULT 'RAW_PASSINGS'` dengan check constraint `CHECK (sync_mode IN ('RAW_PASSINGS', 'FINAL_RESULTS'))`.
* `policy`: Kolom konfigurasi scoring `JSONB NOT NULL DEFAULT '{}'::jsonb`.

### B. Tabel `timing_checkpoints`
* `aliases`: Array alias nama pos hardware `TEXT[] NOT NULL DEFAULT '{}'`.
* `provider_aliases`: Pemetaan JSONB `JSONB NOT NULL DEFAULT '{}'::jsonb` untuk pemetaan spesifik vendor.

### C. Tabel `timing_passings`
* `raw_chip_code`: Kode chip asli yang diterima dari scanner hardware `VARCHAR(64)`.

### D. Tabel `race_results`
* `status`: Diperluas dengan check constraint `CHECK (status IN ('FINISHED', 'DNF', 'DNS', 'DSQ', 'OTL'))`.

### E. Tabel `orders` dan `tickets`
* `participant_id`: Diubah menjadi nullable (`DROP NOT NULL`).
* `guest_email`: Alamat email pembeli tamu `VARCHAR(255)`.
* `guest_name`: Nama lengkap pembeli tamu `VARCHAR(255)`.
* `guest_phone`: Nomor telepon pembeli tamu `VARCHAR(50)`.
* `terms_accepted`: Status persetujuan syarat dan ketentuan `BOOLEAN NOT NULL DEFAULT false`.
* `waiver_accepted`: Status persetujuan pelepasan tanggung jawab medis `BOOLEAN NOT NULL DEFAULT false`.
* `consent_version`: Versi dokumen syarat dan ketentuan yang disetujui `VARCHAR(50)`.
* `consented_at`: Waktu pencatatan persetujuan `TIMESTAMPTZ`.

---

## 3. Arsitektur Multi-Vendor Timing

Implementasi Go terletak pada direktori [`services/api/internal/modules/results/timing`](file:///root/ivyticketing/services/api/internal/modules/results/timing).

### A. Capability Interfaces ([provider.go](file:///root/ivyticketing/services/api/internal/modules/results/timing/provider.go))
Arsitektur menggunakan interface berbasis kapabilitas:
* [`PassingParser`](file:///root/ivyticketing/services/api/internal/modules/results/timing/provider.go): Mengurai passing mentah (`ParsePassings(raw []byte, options map[string]interface{}) ([]TimingPassing, error)`).
* [`ParticipantExporter`](file:///root/ivyticketing/services/api/internal/modules/results/timing/provider.go): Menyiapkan berkas ekspor peserta ke timing system (`ExportParticipants(participants []TimingParticipant) ([]byte, error)`).
* [`FinalResultParser`](file:///root/ivyticketing/services/api/internal/modules/results/timing/provider.go): Mengurai hasil akhir terhitung dari vendor pihak ketiga (`ParseFinalResults(raw []byte) ([]NormalizedResult, error)`).
* [`MappingParser`](file:///root/ivyticketing/services/api/internal/modules/results/timing/provider.go): Mengurai berkas pemetaan BIB dan chip transponder (`ParseMappings(raw []byte) ([]ChipMappingItem, error)`).

### B. Registry Pattern ([registry.go](file:///root/ivyticketing/services/api/internal/modules/results/timing/registry.go))
Pendaftaran adapter timing dilakukan secara modular melalui `DefaultRegistry()`:
* Provider `RACE_RESULT`: Mengimplementasikan `PassingParser`, `ParticipantExporter`, dan `MappingParser`.
* Provider `GENERIC_CSV`: Mengimplementasikan `PassingParser`, `FinalResultParser`, dan `MappingParser`.
* Provider `VENDOR_API`: Mengimplementasikan `PassingParser` dan `FinalResultParser`.

### C. Normalisasi & Generic Processor ([processor.go](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go))
Mesin kalkulasi hasil waktu memproses passing secara deterministik:
1. **Alias Matching**: Membangun index pemetaan alias pos matras ke kode pos checkpoint canonical (misal: `MAT_1` atau `RR_START` dipetakan ke `START`).
2. **Debounce Window**: Mencegah bouncing multi deteksi pada antena matras yang sama dalam rentang detik yang ditentukan di `policy.debounceWindowSeconds` (default: 15 detik).
3. **Gun Time & Net Time**:
   * Gun time dihitung dari selisih waktu passing finish terhadap waktu start wave kategori atlet.
   * Net time dihitung dari selisih waktu passing finish terhadap waktu passing start matras atlet.
   * Jika atlet tidak terdeteksi di matras start dan `policy.allowMissingStart` bernilai true, net time dibiarkan kosong (`nil`) dan waktu resmi mengacu ke gun time.
4. **Batas Cutoff (OTL)**: Jika net time atau gun time melebihi `policy.cutoffMinutes` (misal 360 menit untuk Marathon), atlet ditandai dengan status `OTL` (Over Time Limit).
5. **Ranking Dinamis**: Menghitung peringkat Overall, Gender, dan Kategori berdasarkan chip time (atau gun time jika chip time tidak tersedia) secara sekuensial.

---

## 4. Alur Guest Checkout & Account Linking

### A. Alur Checkout Tamu (Unauthenticated)
1. Atlet memilih kategori lomba pada [`checkout.astro`](file:///root/ivyticketing/apps/web/src/pages/events/[eventId]/checkout.astro).
2. Atlet mengisi formulir data diri, ukuran jersey, dan nama dada BIB.
3. Atlet memvalidasi email dengan mengetik ulang pada modal konfirmasi untuk mencegah salah ketik.
4. Atlet mencentang persetujuan Syarat & Ketentuan serta Pernyataan Pelepasan Tanggung Jawab Medis (Medical Waiver).
5. Klien mengirim permintaan ke endpoint `POST /api/v1/events/{eventId}/categories/{categoryId}/checkout`:
   * Validasi kuota tiket via sistem reservasi.
   * Pembuatan order dengan `participant_id = NULL`, menyimpan `guest_email`, `guest_name`, `guest_phone`, serta audit consent.
   * Pembuatan tiket dengan `participant_id = NULL` dan alokasi nomor BIB resmi otomatis.
6. Atlet diarahkan ke instruksi pembayaran (QRIS / Virtual Account).

### B. Alur Penautan Akun (Account Linking)
1. Setelah menyelesaikan pembayaran, layar konfirmasi menyediakan kartu opsi klaim e-tiket ke akun Ivy.
2. Saat atlet masuk (login) atau mendaftar akun baru dengan email yang sama, klien memanggil endpoint `POST /api/v1/orders/{orderId}/claim`:
   * Sistem memverifikasi kecocokan email order terhadap akun terautentikasi.
   * Mengupdate `orders.participant_id` dan seluruh `tickets.participant_id` terkait secara atomik.
   * Tidak ada tiket duplikat atau pesanan baru yang dibuat.

---

## 5. Dokumentasi Antarmuka Pengguna (UI)

### A. Admin Event Studio ([apps/web/src/pages/admin/events/edit.astro](file:///root/ivyticketing/apps/web/src/pages/admin/events/edit.astro))
Terletak pada **Tab 10 (Timing & Race RFID)**:
* **Pilihan Provider**: RACE RESULT, Generic CSV, Vendor API, Native RFID, Manual.
* **Pilihan Transport**: HTTP Push, HTTP Pull, CSV Upload, Local Edge Agent, SFTP.
* **Mode Sinkronisasi**: RAW PASSINGS vs FINAL RESULTS.
* **Kebijakan Scoring**:
  * Input Debounce Window (detik).
  * Input Cutoff Limit (menit untuk evaluasi OTL).
  * Checkbox Fallback Gun Time jika start matras terlewat.
* **Kredensial Ingestion**: Tombol pembuatan token rahasia sekali-tampil dengan tombol salin, dan tampilan URL endpoint ingestion.
* **Manajemen Checkpoint**: Tambah dan hapus pos matras (START, SPLIT, FINISH) beserta jarak meter dan alias vendor hardware.

### B. Organizer Results Console ([apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro))
* **Ringkasan Operasional**: Menampilkan status provider aktif, protokol transport, mode sinkronisasi, prefix token, dan observasi telemetri antrean passing.
* **Badge Status Lomba**:
  * `FINISHED`: Hijau.
  * `DNF`: Kuning / Amber.
  * `DNS`: Abu-abu / Slate.
  * `DSQ`: Merah mawar tebal (Did Not Qualify).
  * `OTL`: Ungu tebal (Over Time Limit).
* **Aksi Lapangan**: Hitung Ulang Timing Telemetri, Impor Mapping BIB ke Chip, dan Konfigurasi Ulang Provider.

---

## 6. Referensi Endpoint API

### Timing Configuration
```http
POST /api/v1/organizations/{orgId}/events/{eventId}/results/timing/config
Authorization: Bearer <token_penyelenggara>
Content-Type: application/json

{
  "provider": "RACE_RESULT",
  "transport": "HTTP_PUSH",
  "syncMode": "RAW_PASSINGS",
  "externalRaceId": "2026-BKK-MARATHON",
  "policy": {
    "debounceWindowSeconds": 15,
    "cutoffMinutes": 360,
    "allowMissingStart": true
  }
}
```

### Passing Ingestion (Push Real-Time)
```http
POST /api/v1/organizations/{orgId}/events/{eventId}/results/timing/passings
Authorization: Bearer <ingestion_token>
Content-Type: application/json

[
  {
    "bibNumber": "1001",
    "checkpointCode": "START_MAT",
    "detectionTime": "2026-09-17T05:00:02.120Z",
    "rawChipCode": "CHIP-A991"
  },
  {
    "bibNumber": "1001",
    "checkpointCode": "10K",
    "detectionTime": "2026-09-17T05:42:15.800Z",
    "rawChipCode": "CHIP-A991"
  }
]
```

### Public Guest Checkout
```http
POST /api/v1/events/{eventId}/categories/{categoryId}/checkout
Content-Type: application/json

{
  "guestEmail": "pelari@example.com",
  "guestName": "Budi Santoso",
  "guestPhone": "+628123456789",
  "termsAccepted": true,
  "waiverAccepted": true,
  "consentVersion": "2026-09-v1"
}
```

### Claim Guest Order
```http
POST /api/v1/orders/{orderId}/claim
Authorization: Bearer <user_jwt_token>
```

---

## 7. Verifikasi & Prosedur Uji

1. **Uji Kompilasi & Unit Test Backend**:
   ```bash
   cd services/api
   go test -v ./internal/modules/results/... ./internal/modules/orders/...
   go test ./...
   ```
2. **Uji Kompilasi Frontend**:
   ```bash
   pnpm --dir apps/web build
   ```
3. **Pemeriksaan Kepatuhan Aturan Gaya**:
   * Konfirmasi ketiadaan karakter em dash pada seluruh berkas yang dimodifikasi.
