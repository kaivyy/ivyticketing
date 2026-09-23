# Model Data & Struktur Entitas Basis Data (Data Model & Schema)

Dokumen ini menjelaskan rancangan skema basis data relasional PostgreSQL pada IvyTicketing, integritas referensial antar-tabel, dan migrasi skema.

---

## 1. Diagram Relasi Entitas Utama (Entity Relationship)

```
+------------------+       +-------------------+       +-----------------------+
|  organizations   | 1---N |      events       | 1---N |   event_categories    |
+------------------+       +-------------------+       +-----------------------+
        | 1                         | 1                            | 1
        |                           |                              |
        | N                         | N                            | N
+------------------+       +-------------------+                   |
|   audit_logs     |       |      orders       |                   |
+------------------+       +-------------------+                   |
                                    | 1                            |
                                    |                              |
                                    | 1                            |
                           +-------------------+                   |
                           |      tickets      | <-----------------+
                           +-------------------+
                             (Source of Truth)
                              - bib_number
                              - form_answers
                              - status (VALID/USED/CANCELLED)
                              - wave_id
```

---

## 2. Rincian Entitas Inti

### A. Tabel `event_categories`
Menyimpan konfigurasi teknis setiap kategori lomba lari:
- `id` (uuid, PK)
- `event_id` (uuid, FK `events.id` ON DELETE CASCADE)
- `name` (text, e.g. "Full Marathon 42.195K")
- `price` (bigint, harga dasar dalam mata uang Rupiah)
- `quota` (integer, kuota maksimum peserta)
- `distance_km` (numeric(5,2), jarak resmi lintasan dalam kilometer)
- `cutoff_time` (text, batas waktu lomba, contoh: "07:00:00")
- `pricing_tiers` (jsonb, pengaturan harga bertingkat: Early Bird, Reguler, Late)

### B. Tabel `orders`
Menyimpan transaksi pembelian tiket oleh peserta:
- `id` (uuid, PK)
- `organization_id` (uuid, FK `organizations.id`)
- `event_id` (uuid, FK `events.id`)
- `customer_name` (text, nama pemesan)
- `customer_email` (citext, alamat email pemesan)
- `status` (text, status pesanan: PENDING, PAID, CANCELLED, REFUNDED)
- `form_answers` (jsonb, jawaban formulir registrasi kustom atlet)
- `refund_status` (text, status proses refund: NONE, REQUESTED, APPROVED, REJECTED)
- `refund_amount` (bigint, nominal dana yang dikembalikan)
- `refund_reason` (text, alasan pengajuan pengembalian dana)

### C. Tabel `tickets` (Single Source of Truth)
Pusat kebenaran (*Single Source of Truth*) untuk tiket, atlet, dan nomor BIB lomba:
- `id` (uuid, PK)
- `organization_id` (uuid, FK `organizations.id`)
- `event_id` (uuid, FK `events.id`)
- `category_id` (uuid, FK `event_categories.id`)
- `order_id` (uuid, FK `orders.id`, UNIQUE constraint)
- `participant_id` (uuid, FK `users.id` NULLABLE untuk pembelian tamu)
- `ticket_number` (text, nomor identifikasi tiket acak berkeamanan tinggi)
- `holder_name` (text, nama pelari terdaftar)
- `holder_email` (text, email pelari)
- `bib_number` (text NULLABLE, nomor dada pelari di lintasan)
- `bib_assigned_at` (timestamptz, waktu penomoran BIB)
- `bib_assigned_by` (uuid, staf yang menetapkan BIB)
- `wave_id` (uuid NULLABLE, gelombang start yang dialokasikan)
- `form_answers` (jsonb, salinan jawaban formulir atlet)
- `status` (text, status tiket: VALID, USED, CANCELLED)
- `issued_at` (timestamptz, waktu tiket diterbitkan)
- `used_at` (timestamptz NULLABLE, waktu saat pelari melewati pemindai gerbang start)

### D. Tabel Finansial: `org_payout_accounts` & `payout_requests`
Mengelola rekening bank penyelenggara dan permohonan pencairan omset tiket:
- **`org_payout_accounts`**:
  - `id` (uuid, PK)
  - `organization_id` (uuid, FK `organizations.id`)
  - `bank_name` (text, nama bank pencairan, misal: BCA, Mandiri, BRI)
  - `account_number` (text, nomor rekening)
  - `account_holder` (text, nama pemilik rekening sesuai buku tabungan)
  - `is_default` (boolean, penanda rekening utama)
- **`payout_requests`**:
  - `id` (uuid, PK)
  - `organization_id` (uuid, FK `organizations.id`)
  - `amount` (bigint, nominal dana yang diajukan)
  - `status` (text: PENDING, APPROVED, REJECTED, COMPLETED)
  - `account_id` (uuid, FK `org_payout_accounts.id`)
  - `requested_by` (uuid, FK `users.id`)
  - `notes` (text, catatan tambahan)

### E. Tabel `audit_logs`
Mencatat seluruh aksi operasional penting untuk kebutuhan audit forensik:
- `id` (uuid, PK)
- `organization_id` (uuid, FK `organizations.id`)
- `actor_user_id` (uuid, FK `users.id` NULLABLE)
- `action` (text, nama aksi: e.g. `BIB_ASSIGNED`, `BROADCAST_SENT`, `PAYOUT_REQUESTED`)
- `target_type` (text, tipe objek: e.g. `event`, `ticket`, `payout`)
- `target_id` (text, identitas objek)
- `metadata` (jsonb, payload data pendukung)
- `created_at` (timestamptz, waktu pencatatan log)

---

## 3. Garansi Integritas Data

1. **At-Most-Once Issuance**:
   Setiap order yang berhasil dibayar hanya dapat menerbitkan maksimal satu tiket melalui constraint `UNIQUE(order_id)` pada tabel `tickets`.
2. **Atomic BIB Allocation**:
   Penomoran BIB otomatis menggunakan fungsi agregat numerik `GetNextBibNumeric` dengan lock row untuk mencegah terjadinya duplikasi nomor dada pelari di lintasan yang sama.
3. **Idempotent Check-in**:
   Status tiket `VALID -> USED` pada saat pemindaian gerbang start diproteksi oleh kondisi atomic SQL `WHERE id = $1 AND status = 'VALID'` sehingga pemindaian berulang tidak menyebabkan perhitungan ganda.
