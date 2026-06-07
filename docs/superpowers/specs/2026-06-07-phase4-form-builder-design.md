# Spec — Phase 4: Custom Registration Form Builder

Date: 2026-06-07
Status: Approved (design)
Scope: Phase 4 dari masterplan.md — Custom Registration Form Builder
Depends on: Phase 1 (foundation), Phase 2 (auth/RBAC/multi-tenant), Phase 3 (event/category) — selesai

## Tujuan

Organizer bisa membuat form pendaftaran custom tanpa coding: menyusun field (text,
email, dropdown, dst), aturan validasi per field, conditional logic (tampil/sembunyi
berdasarkan jawaban field lain), dan membatasi field tertentu ke kategori tertentu.
Backend-only (level API), teruji lewat unit + integration test. **Builder saja** —
pengisian form oleh peserta (submission) ditunda ke Phase 5 karena terikat order.

## Keputusan yang sudah disepakati

| Topik | Keputusan |
|---|---|
| Cakupan | Form BUILDER saja. Submission peserta ditunda ke Phase 5 (terikat order). |
| Model skema | Relational: `form_schemas` (1/event) + `form_fields` (1 baris/field). |
| Binding | 1 form per event; field bisa dibatasi per kategori lewat `category_scope`. |
| Conditional | Multi-condition AND/OR bertingkat (pohon group + leaf), acyclic, batas kedalaman. |
| GET /form kosong | Auto-create form kosong (lazy init) lalu balas form + fields []. |
| Preview | Endpoint preview + dry-run validate untuk membuktikan conditional & validasi jalan tanpa submission. |

## Non-Goals (YAGNI untuk Phase 4)

- Tidak ada submission peserta persisten (Phase 5, terikat order). Tidak ada tabel `form_submissions`.
- Tidak ada penyimpanan jawaban file upload peserta (Phase 5). Definisi field bertipe `file` boleh dibuat, tapi value/upload jawaban menyusul.
- Tidak ada export submission (belum ada submission; Phase 5/16).
- Tidak ada versioning/riwayat revisi form.
- Tidak ada UI/halaman frontend (backend-only).
- Tidak ada conditional yang merujuk field sesudahnya (hanya acyclic, rujuk field sebelumnya).

## Model Data (migrasi goose, di atas Phase 3)

Keduanya ber-`organization_id` mengikuti aturan multi-tenant.

```
form_schemas                          ← migrasi 00010
├─ id (uuid, pk, default gen_random_uuid())
├─ organization_id (uuid, fk → organizations, ON DELETE CASCADE)
├─ event_id (uuid, fk → events, ON DELETE CASCADE)
├─ name (text, not null, default '')
├─ created_at (timestamptz, not null, default now())
├─ updated_at (timestamptz, not null, default now())
└─ UNIQUE(event_id)                   ← 1 form per event

form_fields                           ← migrasi 00011
├─ id (uuid, pk, default gen_random_uuid())
├─ organization_id (uuid, fk → organizations, ON DELETE CASCADE)   ← denormalized
├─ form_schema_id (uuid, fk → form_schemas, ON DELETE CASCADE)
├─ field_type (text, not null)        ← text|email|phone|number|date|dropdown|radio|checkbox|textarea|file
├─ label (text, not null)
├─ field_key (text, not null)         ← snake_case, kunci jawaban (Phase 5) & referensi conditional
├─ help_text (text, nullable)
├─ is_required (boolean, not null, default false)
├─ display_order (integer, not null)
├─ options (jsonb, nullable)          ← ["Pria","Wanita"] utk dropdown/radio/checkbox
├─ validation (jsonb, nullable)       ← {"minLength":3,"max":100,"pattern":"..."}
├─ conditional (jsonb, nullable)      ← pohon AND/OR (lihat §Conditional)
├─ category_scope (jsonb, nullable)   ← null = semua kategori; ["catId",...] = override per kategori
├─ created_at (timestamptz, not null, default now())
├─ updated_at (timestamptz, not null, default now())
└─ UNIQUE(form_schema_id, field_key)
CREATE INDEX idx_form_fields_schema ON form_fields(form_schema_id);
CREATE INDEX idx_form_schemas_event ON form_schemas(event_id);
CREATE INDEX idx_form_fields_org ON form_fields(organization_id);
```

Catatan:
- **`UNIQUE(event_id)`** menjamin satu form per event; upsert via service.
- **`category_scope`**: `null` → field muncul di semua kategori (field umum). Array id kategori → field hanya muncul di kategori tsb (override). Saat preview kategori X: filter `category_scope IS NULL OR X ∈ category_scope`. Itu satu-satunya aturan merge — satu pass, tanpa tabel form ganda.
- **`field_key`** unik per form, snake_case; jangan diubah sembarangan karena dirujuk conditional & (nanti) jawaban.
- jsonb (`options`/`validation`/`conditional`/`category_scope`) **selalu divalidasi** strukturnya di service (package `formschema`), tak pernah dipercaya mentah.

## Struktur Modul Go

```
internal/modules/
└── forms/           CRUD form schema + fields, reorder, preview/validate
    handler.go service.go repository.go dto.go routes.go errors.go + tests

internal/platform/
└── formschema/      tipe + validator definisi + evaluator conditional (murni, tanpa DB)
    field.go         tipe Field, FieldType allowlist, struktur Validation/Conditional
    validate.go      ValidateFields([]Field) error  (key unik, type, options, validation, conditional acyclic)
    conditional.go   tipe pohon AND/OR + parse/validate + Evaluate(cond, answers) bool
    answers.go       ValidateAnswers(fields, answers, categoryID) []FieldError  (untuk preview)
    *_test.go
```

Aturan tetap: handler = HTTP only; service = logika + orkestrasi repo + panggil `formschema`;
repository = query sqlc. `formschema` murni (tanpa DB/HTTP) agar mudah dites unit.

## Conditional Logic (package `formschema`)

Pohon AND/OR bertingkat. Dua jenis node:
- **Group**: `{ "op": "and"|"or", "rules": [ ...node... ] }` — boleh bersarang.
- **Leaf**: `{ "field": "<field_key>", "op": "<operator>", "value": <any> }`.

Operator leaf: `equals`, `notEquals`, `in`, `notIn`, `gt`, `gte`, `lt`, `lte`.

Contoh:
```json
{
  "op": "and",
  "rules": [
    { "field": "wna", "op": "equals", "value": "Ya" },
    { "op": "or", "rules": [
        { "field": "umur", "op": "gte", "value": 17 },
        { "field": "punya_wali", "op": "equals", "value": "Ya" }
    ]}
  ]
}
```

Batas (cegah pohon patologis): kedalaman bersarang ≤ 3, total leaf per field ≤ 20.

**Validasi conditional** (`ValidateFields`):
- Setiap `field` yang dirujuk leaf harus ada di form.
- Hanya boleh merujuk field ber-`display_order` lebih kecil (acyclic; cegah loop & forward-ref).
- `op` leaf ∈ allowlist; group `op` ∈ {and, or} dengan `rules` non-kosong.
- Operator numerik (`gt/gte/lt/lte`) hanya valid jika field rujukan bertipe number/date.
- Operator `in/notIn` butuh `value` berupa array.

**Evaluasi** (`Evaluate(cond, answers map[string]any) bool`): group `and` → semua rule true;
`or` → minimal satu true; leaf → bandingkan `answers[field]` dengan `value` per operator.
Field tak terjawab → leaf dianggap false.

## Validasi Definisi Field (`formschema.ValidateFields([]Field) error`)

Dipanggil service sebelum commit create/update field, atas **seluruh set field** form
(termasuk perubahan):
- `field_key`: non-kosong, snake_case (`^[a-z][a-z0-9_]*$`), unik dalam form.
- `field_type` ∈ allowlist.
- `options`: wajib & non-kosong untuk `dropdown/radio/checkbox`; harus kosong/null untuk tipe lain.
- `validation`: hanya rule yang cocok dengan tipe — `minLength`/`maxLength`/`pattern` untuk `text/textarea/email/phone`; `min`/`max` untuk `number`; `min`/`max` (tanggal ISO) untuk `date`. Rule asing → error.
- `conditional`: lolos validasi pohon di atas.
- `category_scope`: null atau array string non-kosong; tiap id (divalidasi sebagai kategori milik event di service layer, bukan di package murni).

## Validasi Jawaban untuk Preview (`formschema.ValidateAnswers`)

`ValidateAnswers(fields []Field, answers map[string]any, categoryID *uuid.UUID) []FieldError`:
1. Filter field per kategori (`category_scope`).
2. Evaluasi `conditional` tiap field → tentukan field yang **tampil**.
3. Untuk field tampil: cek `is_required` (kosong → error) dan rule `validation`.
4. Field tersembunyi → dilewati (tak wajib, tak divalidasi).
Mengembalikan daftar `{FieldKey, Message}`. Dipakai endpoint preview/dry-run; **tidak**
menyimpan apa pun (submission Phase 5).

## Endpoint

Semua perlu access token (authn Phase 2) + authz `form.manage` dalam konteks `orgId`.

```
# Form schema (per event)
GET    /api/v1/organizations/{orgId}/events/{eventId}/form
       → form event + fields terurut; auto-create form kosong bila belum ada
PUT    /api/v1/organizations/{orgId}/events/{eventId}/form
       { name }                              → upsert metadata form

# Fields
GET    /api/v1/organizations/{orgId}/events/{eventId}/form/fields
POST   /api/v1/organizations/{orgId}/events/{eventId}/form/fields
       { fieldType, label, fieldKey, helpText?, isRequired?, options?, validation?, conditional?, categoryScope? }
PUT    /api/v1/organizations/{orgId}/events/{eventId}/form/fields/{fieldId}   (body sama)
DELETE /api/v1/organizations/{orgId}/events/{eventId}/form/fields/{fieldId}
PUT    /api/v1/organizations/{orgId}/events/{eventId}/form/fields/reorder
       { fieldIds: [...] }                   → set display_order sesuai urutan

# Preview / dry-run
GET    /api/v1/organizations/{orgId}/events/{eventId}/form/preview?categoryId={catId}
       → field efektif tampil untuk kategori (sudah difilter category_scope)
POST   /api/v1/organizations/{orgId}/events/{eventId}/form/preview/validate?categoryId={catId}
       { answers: { field_key: value } }
       → { valid, errors: [{fieldKey, message}], visibleFields: [field_key,...] }
```

## Aturan Otorisasi & Validasi

- **Isolasi tenant**: form/field difilter `organization_id`; event harus milik `orgId`. Mismatch → **404** (pola Phase 3, tak bocorkan keberadaan). Super admin lolos.
- **Upsert form**: `GET /form` & `PUT /form` membuat `form_schemas` bila belum ada (lazy init / upsert). `UNIQUE(event_id)` menjaga satu form.
- **Create/Update field**: muat seluruh field form + terapkan perubahan → `formschema.ValidateFields` → commit bila lolos; gagal → 400 dengan kode spesifik. `display_order` field baru = max+1.
- **category_scope**: tiap id divalidasi sebagai kategori milik event tsb (service cek ke repo categories/events). Id asing → `CATEGORY_NOT_IN_EVENT`.
- **Reorder**: `fieldIds` harus persis himpunan field form (tak ada hilang/asing) → else `INVALID_REORDER_SET`. Update `display_order` dalam transaksi.
- **Delete field**: bila `field_key`-nya dirujuk `conditional` field lain → tolak `FIELD_REFERENCED` (hapus/ubah rujukan dulu).
- **Preview**: `categoryId` opsional; jika ada harus kategori milik event → else `CATEGORY_NOT_IN_EVENT`.

## Error Codes (tambahan, dibungkus envelope Phase 2)

`FORM_NOT_FOUND`, `FIELD_NOT_FOUND`, `DUPLICATE_FIELD_KEY`, `INVALID_FIELD_TYPE`,
`INVALID_FIELD_KEY`, `OPTIONS_REQUIRED`, `OPTIONS_NOT_ALLOWED`, `INVALID_VALIDATION_RULE`,
`INVALID_CONDITIONAL`, `CONDITIONAL_CYCLE`, `CONDITIONAL_UNKNOWN_FIELD`, `FIELD_REFERENCED`,
`INVALID_REORDER_SET`, `CATEGORY_NOT_IN_EVENT`.

## Testing

**Unit (tanpa DB):**
- `formschema/validate`: key unik/snake_case; type allowlist; options wajib utk dropdown/radio/checkbox & terlarang lainnya; validation cocok tipe; rule asing ditolak.
- `formschema/conditional`: siklus (A→B→A) ditolak; forward-ref (rujuk display_order lebih besar) ditolak; field tak dikenal ditolak; batas kedalaman/jumlah; operator numerik pada field non-number ditolak; `in` tanpa array ditolak.
- `formschema/evaluate`: AND/OR bersarang benar; tiap operator; field tak terjawab → false; field tersembunyi tak tampil.
- `formschema/validateAnswers`: required pada field tampil ditegakkan; field tersembunyi dilewati; minLength/min/max/pattern ditegakkan; filter kategori benar.
- `forms/service`: upsert form per event; create field memanggil ValidateFields; reorder set tak valid ditolak; delete field dirujuk ditolak; tenant mismatch → not found.

**Integration (Postgres `ivyticketing_test`, truncate per test, reuse harness Phase 2/3):**
- Alur penuh: login → create org → create event (+kategori) → GET /form (auto-create) → tambah field text & dropdown → reorder → preview.
- Conditional end-to-end: field B `showIf` A=Ya → validate dengan A=Ya menampilkan B (required ditegakkan); A=Tidak menyembunyikan B (tak required).
- Override kategori: field `category_scope=[42K.id]` muncul di preview 42K, tidak di 10K.
- Isolasi tenant: form event org A → 404 dari org B.
- Validasi: create field dengan conditional siklik → 400 `CONDITIONAL_CYCLE`; field_key dup → 400 `DUPLICATE_FIELD_KEY`.

## Definition of Done

1. Migrasi roundtrip (up/down): `form_schemas`, `form_fields`.
2. CRUD field + reorder + upsert form berfungsi & teruji.
3. Package `formschema`: ValidateFields, conditional acyclic, Evaluate AND/OR, ValidateAnswers — teruji unit.
4. Conditional logic terbukti jalan via endpoint preview/validate.
5. Field per kategori (`category_scope`) terbukti via preview.
6. Form preview menampilkan field efektif per kategori.
7. Isolasi tenant terbukti (form org A → 404 dari org B).
8. Validasi definisi menolak input cacat (siklus, key dup, options salah) dengan kode spesifik.
9. `go test ./...` hijau (unit) + integration hijau.
10. `sqlc generate` bersih; semua query lewat sqlc.
11. Tidak ada secret hardcoded.
12. CHANGELOG.md diperbarui dengan entry Phase 4.

## Setelah Phase 4

Phase 5 — Inventory, Order & Checkout Core. Submission peserta (mengisi form ini)
dibangun di sana, terikat order, memakai `formschema.ValidateAnswers` yang sudah ada.
