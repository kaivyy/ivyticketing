# Spec — Phase 3: Event & Category Management

Date: 2026-06-07
Status: Approved (design)
Scope: Phase 3 dari masterplan.md — Event + Category management
Depends on: Phase 1 (monorepo foundation), Phase 2 (auth/RBAC/multi-tenant) — selesai

## Tujuan

Organizer bisa membuat dan mengelola event lomba beserta kategorinya (nomor lomba:
42K/21K/10K/dst), lengkap dengan status lifecycle (draft/published/archived) dan
media (banner/logo) lewat storage pluggable. Backend-only (level API), teruji lewat
unit + integration test. Event `published` tampil di endpoint publik.

## Keputusan yang sudah disepakati

| Topik | Keputusan |
|---|---|
| Cakupan | Event + Category management dalam satu spec. Backend-only. |
| Media event | Masuk scope. Disimpan sebagai `object_key` (bukan URL). |
| Storage | Interface pluggable: driver `local` (default, tulis ke disk) penuh; cloud (R2/Tencent/S3) lewat presigned URL agar tak membebani server. |
| Upload | Per-backend: cloud → presigned PUT (direct-to-storage); local → multipart ke API → disk. Seragam lewat satu interface. |
| Status event | `draft` → `published` → `archived`. Unpublish = published→draft. Tanpa scheduled (butuh worker, ditunda). |
| Kategori | Field lengkap (price/capacity/registration window/bib_prefix/min_age/max_order) + CRUD + validasi. TANPA inventory/stok (Phase 5). |
| Public page | Endpoint publik read-only: hanya event `published`. |

## Non-Goals (YAGNI untuk Phase 3)

- Tidak ada inventory/stok real-time (atomic decrement, reservation, oversold prevention) — itu Phase 5. `capacity` hanya angka tersimpan.
- Tidak ada order/checkout, registration form builder (Phase 4), coupon/merchandise.
- Tidak ada scheduled publish (butuh worker; `services/worker` belum dibangun).
- Tidak ada driver R2/Tencent/S3 konkret penuh — interface + driver `local` saja; cloud di-stub dengan kontrak jelas, diisi saat kredensial tersedia.
- Tidak ada UI/halaman frontend (backend-only).
- Tidak ada image processing (resize/thumbnail) — disimpan apa adanya.

## Model Data (migrasi goose, di atas Phase 2)

Keduanya ber-`organization_id` mengikuti aturan multi-tenant.

```
events                                  ← migrasi 00008
├─ id (uuid, pk, default gen_random_uuid())
├─ organization_id (uuid, fk → organizations, ON DELETE CASCADE)
├─ name (text, not null)
├─ slug (text, not null)
├─ description (text, nullable)
├─ event_type (text, not null)          ← "marathon"/"trail"/"cycling"/"triathlon"/"funrun"/"expo"/"seminar"/"concert"/"other"
├─ status (text, not null, default 'draft')   ← draft | published | archived
├─ banner_object_key (text, nullable)   ← key di storage; URL dirakit saat baca
├─ logo_object_key (text, nullable)
├─ venue_name (text, nullable)
├─ venue_address (text, nullable)
├─ starts_at (timestamptz, nullable)
├─ ends_at (timestamptz, nullable)
├─ faq (text, nullable)
├─ terms (text, nullable)
├─ waiver (text, nullable)
├─ published_at (timestamptz, nullable)
├─ created_at (timestamptz, not null, default now())
├─ updated_at (timestamptz, not null, default now())
└─ UNIQUE(organization_id, slug)

event_categories                        ← migrasi 00009
├─ id (uuid, pk, default gen_random_uuid())
├─ organization_id (uuid, fk → organizations, ON DELETE CASCADE)   ← denormalized
├─ event_id (uuid, fk → events, ON DELETE CASCADE)
├─ name (text, not null)
├─ price (bigint, not null)             ← satuan terkecil (sen), CHECK >= 0
├─ capacity (integer, not null)         ← CHECK > 0; hanya angka tersimpan (tanpa inventory)
├─ registration_opens_at (timestamptz, not null)
├─ registration_closes_at (timestamptz, not null)
├─ bib_prefix (text, nullable)
├─ min_age (integer, nullable)          ← CHECK (min_age IS NULL OR min_age >= 0)
├─ max_order_per_user (integer, not null, default 1)   ← CHECK >= 1
├─ created_at (timestamptz, not null, default now())
├─ updated_at (timestamptz, not null, default now())
└─ UNIQUE(event_id, name)
CREATE INDEX idx_events_org ON events(organization_id);
CREATE INDEX idx_events_org_status ON events(organization_id, status);
CREATE INDEX idx_event_categories_event ON event_categories(event_id);
```

Catatan:
- **`organization_id` didenormalisasi** di `event_categories` agar filter tenant/authz tak selalu join ke `events`. Service tetap memvalidasi `category.event_id` milik event yang `organization_id`-nya cocok (konsistensi dijaga di service layer).
- **Media = `object_key`**, bukan URL → ganti storage backend tak ubah data. URL publik dirakit storage adapter saat response.
- **`price` bigint dalam sen** — hindari bug floating point (aturan engineering masterplan).
- **`registration_opens_at`/`closes_at` not null** — kategori wajib punya window; validasi `opens < closes` di service.

## Struktur Modul Go

```
internal/modules/
├── events/          CRUD event, status lifecycle, media upload flow
│   handler.go service.go repository.go dto.go routes.go errors.go + tests
├── categories/      CRUD kategori per event, validasi
│   handler.go service.go repository.go dto.go routes.go errors.go + tests
└── publiccatalog/   endpoint publik read-only (event published + kategori)
    handler.go service.go repository.go dto.go routes.go + tests

internal/platform/
└── storage/
    ├── storage.go   interface Storage + PutTicket + driver factory dari config
    ├── local.go     driver lokal: Put ke disk, PublicURL, PresignUpload→ok=false
    └── s3.go        driver S3-compatible (R2/Tencent) — presigned PUT; STUB berkontrak, diisi saat kredensial ada
```

Aturan tetap: handler = HTTP only; service = logika; repository = query sqlc.
Authz Phase 2 dipakai apa adanya (permission `event.*`, `category.manage` sudah di-seed).

## Storage Abstraction

```go
type Storage interface {
    // PresignUpload returns a direct-to-storage upload ticket if the backend
    // supports it (R2/Tencent/S3). ok=false → backend can't presign (local).
    PresignUpload(ctx context.Context, key, contentType string, ttl time.Duration) (PutTicket, bool, error)
    // Put writes bytes directly (local backend, or fallback).
    Put(ctx context.Context, key string, r io.Reader, contentType string) error
    // PublicURL builds the readable URL for a stored object.
    PublicURL(key string) string
    // Delete removes an object (best-effort).
    Delete(ctx context.Context, key string) error
}

type PutTicket struct {
    URL     string            `json:"url"`
    Method  string            `json:"method"`   // "PUT"
    Headers map[string]string `json:"headers"`  // headers client must send
    Expires time.Time         `json:"expires"`
}
```

**Driver:**
- `local` (default dev): `PresignUpload` → `ok=false`. Client upload multipart ke API, API `Put` ke disk di `STORAGE_LOCAL_PATH`. `PublicURL(key)` → `{STORAGE_PUBLIC_BASE_URL}/media/{key}`, di-serve route static read-only.
- `r2`/`tencent`/`s3` (prod): `PresignUpload` → `ok=true`, terbitkan presigned PUT URL (AWS SDK v2, S3-compatible). Client PUT **langsung** ke object storage — server tak proses byte. `PublicURL` → CDN/public bucket URL. **Phase 3: stub berkontrak** — signature & error final, body diisi saat kredensial cloud tersedia.

**Object key format:** `org/{orgId}/event/{eventId}/{kind}/{uuid}.{ext}` (kind = banner|logo).
Namespaced per tenant, tak bisa ditebak/tabrakan. Ekstensi divalidasi terhadap allowlist (`jpg/jpeg/png/webp`), `contentType` dicek terhadap allowlist gambar.

**Alur upload (seragam, ringan):**
```
1. POST .../events/{eventId}/media/{kind}        body: { contentType, fileName }
   service → storage.PresignUpload(key, contentType, ttl)
   - ok (cloud):  200 { mode:"presigned", objectKey, upload:{url,method,headers,expires} }
                  → client PUT langsung ke storage
   - !ok (local): 200 { mode:"direct", objectKey, uploadUrl:".../media/{kind}/upload" }
                  → client POST multipart ke uploadUrl → API stream ke disk

2. POST .../events/{eventId}/media/{kind}/upload   (LOCAL only) multipart sink → storage.Put

3. PUT  .../events/{eventId}/media/{kind}/confirm   body: { objectKey }
   → validasi objectKey berformat & berprefix milik event ini (anti-tamper)
   → set events.banner_object_key / logo_object_key = objectKey
```

Confirm eksplisit diperlukan karena upload cloud tak lewat API — API hanya percaya key yang lolos validasi prefix `org/{orgId}/event/{eventId}/{kind}/`.

## Env Baru

```
STORAGE_DRIVER=local                       # local | r2 | tencent | s3
STORAGE_LOCAL_PATH=./var/media             # dir untuk driver local
STORAGE_PUBLIC_BASE_URL=http://localhost:8080
STORAGE_UPLOAD_MAX_BYTES=5242880           # 5MB default, batas multipart local
# cloud (diisi saat fasenya; kosong utk local):
STORAGE_BUCKET=
STORAGE_ENDPOINT=
STORAGE_ACCESS_KEY=
STORAGE_SECRET_KEY=
STORAGE_REGION=
```

`STORAGE_DRIVER=local` adalah default; tak ada secret wajib di Phase 3. Saat driver cloud dipilih tapi kredensial kosong → API gagal start dengan pesan jelas (pola Phase 2 untuk var wajib bersyarat).

## Endpoint

Semua endpoint organizer perlu access token (authn Phase 2) + authz permission dalam
konteks `orgId`. Public tanpa auth.

### Events (authz per aksi)
```
# event.create
POST   /api/v1/organizations/{orgId}/events
       { name, eventType, description?, venueName?, venueAddress?, startsAt?, endsAt?, faq?, terms?, waiver? }
       → 201 event (status=draft, slug ter-generate)

# event.edit
GET    /api/v1/organizations/{orgId}/events                  daftar (semua status, milik org)
GET    /api/v1/organizations/{orgId}/events/{eventId}        detail + kategori
PUT    /api/v1/organizations/{orgId}/events/{eventId}        edit field

# event.publish
POST   /api/v1/organizations/{orgId}/events/{eventId}/publish      draft→published (tolak jika 0 kategori)
POST   /api/v1/organizations/{orgId}/events/{eventId}/unpublish    published→draft
POST   /api/v1/organizations/{orgId}/events/{eventId}/archive      →archived

# event.delete
DELETE /api/v1/organizations/{orgId}/events/{eventId}        hapus (cascade kategori; hapus media best-effort)

# media (event.edit)
POST   /api/v1/organizations/{orgId}/events/{eventId}/media/{kind}          kind=banner|logo → upload ticket
POST   /api/v1/organizations/{orgId}/events/{eventId}/media/{kind}/upload   (local only) multipart sink
PUT    /api/v1/organizations/{orgId}/events/{eventId}/media/{kind}/confirm  { objectKey }
```

### Categories (authz: category.manage)
```
GET    /api/v1/organizations/{orgId}/events/{eventId}/categories
POST   /api/v1/organizations/{orgId}/events/{eventId}/categories
       { name, price, capacity, registrationOpensAt, registrationClosesAt, bibPrefix?, minAge?, maxOrderPerUser? }
GET    /api/v1/organizations/{orgId}/events/{eventId}/categories/{categoryId}
PUT    /api/v1/organizations/{orgId}/events/{eventId}/categories/{categoryId}
DELETE /api/v1/organizations/{orgId}/events/{eventId}/categories/{categoryId}
```

### Public (tanpa auth, read-only)
```
GET    /api/v1/public/organizations/{orgSlug}/events                  hanya status=published
GET    /api/v1/public/organizations/{orgSlug}/events/{eventSlug}      detail + kategori (hanya jika published)
GET    /media/{key...}                                                serve file (driver local saja)
```

## Aturan Otorisasi & Validasi

- **Isolasi tenant**: query difilter `organization_id`. Event/kategori milik org lain → **404** (bukan 403, agar tak bocorkan keberadaan resource lintas tenant). Super admin (`is_platform_admin`) lolos & lihat semua (pola Phase 2).
- **Kategori milik event**: service memvalidasi `category.event_id` cocok dengan `{eventId}` di URL DAN `event.organization_id` cocok `{orgId}`. Mismatch → 404.
- **Publish**: ditolak jika event tak punya kategori → `EVENT_NO_CATEGORIES` (409). Hanya `draft`→`published`. Set `published_at`.
- **Unpublish**: hanya `published`→`draft`. **Archive**: dari `draft`/`published`→`archived`; transisi lain ditolak `INVALID_STATUS_TRANSITION` (409).
- **Validasi kategori**: `price >= 0`, `capacity > 0`, `registration_opens_at < registration_closes_at`, `min_age >= 0` (jika ada), `max_order_per_user >= 1`. Gagal → 400 dengan kode spesifik.
- **Slug event**: auto dari `name` (reuse pola `slugify` Phase 2), unik per org. Tabrakan → `SLUG_TAKEN` (409).
- **event_type**: divalidasi terhadap allowlist enum; tak dikenal → 400.
- **Media**: `contentType` & ekstensi terhadap allowlist gambar; ukuran ≤ `STORAGE_UPLOAD_MAX_BYTES` (local). Confirm menolak `objectKey` yang tak berprefix `org/{orgId}/event/{eventId}/{kind}/` → `INVALID_OBJECT_KEY` (400).
- **Audit**: aksi sensitif (publish/unpublish/archive/delete event) → `audit_logs` (helper Phase 2).
- **Public**: hanya membaca `published`; `draft`/`archived` → 404. Tak membocorkan field internal (mis. `object_key` mentah; kirim `PublicURL`).

## Error Codes (tambahan, dibungkus envelope Phase 2)

`EVENT_NOT_FOUND`, `EVENT_NO_CATEGORIES`, `INVALID_STATUS_TRANSITION`, `SLUG_TAKEN`,
`INVALID_EVENT_TYPE`, `CATEGORY_NOT_FOUND`, `CATEGORY_NAME_TAKEN`, `INVALID_PRICE`,
`INVALID_CAPACITY`, `INVALID_REGISTRATION_WINDOW`, `INVALID_AGE`, `INVALID_MAX_ORDER`,
`INVALID_OBJECT_KEY`, `INVALID_CONTENT_TYPE`, `FILE_TOO_LARGE`.

## Testing

**Unit (tanpa DB, fake repository):**
- `events/service`: create (slug ter-generate, default draft); publish ditolak tanpa kategori; transisi publish/unpublish/archive valid & invalid; tenant mismatch → not found.
- `categories/service`: validasi (price<0, capacity<=0, opens>=closes, min_age<0, max_order<1 ditolak); kategori harus milik event di org yang sama.
- `storage/local`: `Put` tulis file & `PublicURL` benar; `PresignUpload`→ok=false; key namespacing & ekstensi tervalidasi.
- `events` media: confirm menolak objectKey yang bukan milik event (anti-tamper).

**Integration (Postgres `ivyticketing_test`, truncate per test, helper reuse Phase 2):**
- Alur penuh: login → create org (Phase 2) → create event → add kategori → publish → muncul di endpoint public.
- Publish tanpa kategori → 409.
- Isolasi tenant: member org A → 404 saat akses event org B.
- Public: draft tak muncul; published muncul; archived hilang dari public.
- Upload local end-to-end: ticket(direct) → multipart upload → confirm → `banner_object_key` terisi → file ter-serve di `/media/{key}`.

## Definition of Done

1. Migrasi roundtrip (up/down): `events`, `event_categories`.
2. CRUD event + status lifecycle (draft/published/archived) berfungsi & teruji.
3. CRUD kategori + validasi field berfungsi & teruji.
4. Storage interface dengan driver `local` penuh; presigned-path (cloud) ter-stub berkontrak jelas.
5. Upload media local end-to-end (ticket → upload → confirm → URL ter-serve).
6. Public endpoint hanya menampilkan event `published`.
7. Isolasi tenant terbukti (event org A tak terlihat org B → 404).
8. Publish tanpa kategori ditolak (409).
9. Aksi sensitif tercatat di `audit_logs`.
10. `go test ./...` hijau (unit) + integration hijau.
11. `sqlc generate` bersih; semua query lewat sqlc.
12. Tidak ada secret hardcoded; storage config via env.
13. CHANGELOG.md diperbarui dengan entry Phase 3.

## Setelah Phase 3

Phase 4 — Custom Registration Form Builder, di atas event/category ini.
