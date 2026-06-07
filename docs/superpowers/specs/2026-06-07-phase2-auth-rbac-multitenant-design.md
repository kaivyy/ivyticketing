# Spec — Phase 2: Auth, RBAC & Multi-Tenant Core

Date: 2026-06-07
Status: Approved (design)
Scope: Phase 2 dari masterplan.md — Auth + Multi-Tenant + RBAC
Depends on: Phase 1 (monorepo foundation) — selesai

## Tujuan

Membangun fondasi SaaS multi-organizer: autentikasi (hybrid token), multi-tenant
organization + membership, dan RBAC dengan custom role per organizer. Backend-only
(level API), teruji lewat unit + integration test.

## Keputusan yang sudah disepakati

| Topik | Keputusan |
|---|---|
| Cakupan | Auth + Multi-tenant + RBAC dalam satu spec. |
| Token | Hybrid: access JWT umur pendek (~15m) + refresh token opaque (~7h) di DB, revocable. |
| RBAC | Custom role per organizer + permission granular (`module.action`). Role bawaan di-seed sbg template, di-copy per org. |
| Frontend | Backend-only. UI auth menyusul bersama dashboard (fase lain). |
| User model | Satu tabel `users` global. Konteks (peserta vs staff) ditentukan saat otorisasi. |
| Peserta | Tidak wajib login untuk war/registrasi. Guest flow = fase Order/Queue, bukan Phase 2. |
| Audit log | Versi minimal masuk scope Phase 2 (aksi sensitif RBAC/member). |

## Non-Goals (YAGNI untuk Phase 2)

- Tidak ada UI/halaman login (backend-only).
- Tidak ada guest checkout/registrasi peserta (fase Order/Queue).
- Tidak ada OAuth/social login, phone/KYC verification, forgot-password email flow
  (struktur menyebut, tapi butuh email service — ditunda ke fase Notification).
- Tidak ada deny-list access token di Redis (revoke instan akses) — jendela 15m diterima.
- Tidak ada billing/subscription (Phase 17).

## Model Data (migrasi goose, di atas schema_health)

`users` global; tabel lain ber-`organization_id` mengikuti aturan multi-tenant.

```
users
├─ id (uuid, pk)
├─ email (citext, unique)
├─ password_hash (nullable — peserta guest/social bisa tanpa password)
├─ full_name, phone
├─ is_platform_admin (bool, default false)   ← super admin / platform owner
├─ email_verified_at (nullable), created_at, updated_at

organizations
├─ id (uuid, pk)
├─ name, slug (unique)
├─ created_at, updated_at

organization_members          ← user ↔ organization (staff)
├─ id (uuid, pk)
├─ organization_id (fk), user_id (fk)
├─ created_at
└─ UNIQUE(organization_id, user_id)

roles                         ← role bawaan (organization_id NULL = template) + custom per org
├─ id (uuid, pk)
├─ organization_id (fk, nullable)   ← NULL = role sistem/template global
├─ name, slug, is_system (bool)
└─ UNIQUE(organization_id, slug)

permissions                   ← katalog tetap, di-seed (module.action)
├─ id (uuid, pk)
├─ key (unique, e.g. "event.create")
├─ description

role_permissions              ← role ↔ permission (m2m)
├─ role_id (fk), permission_id (fk)
└─ PK(role_id, permission_id)

member_roles                  ← membership ↔ role (staff bisa punya >1 role)
├─ organization_member_id (fk), role_id (fk)
└─ PK(organization_member_id, role_id)

refresh_tokens                ← hybrid token (opaque, revocable)
├─ id (uuid, pk)
├─ user_id (fk)
├─ token_hash (unique)        ← SHA-256 hash, bukan token mentah
├─ expires_at, revoked_at (nullable), created_at

audit_logs                    ← versi minimal
├─ id (uuid, pk)
├─ organization_id (fk, nullable)
├─ actor_user_id (fk, nullable)
├─ action (text, e.g. "member.add")
├─ target_type, target_id (text, nullable)
├─ metadata (jsonb, nullable)
├─ created_at
```

Catatan:
- **Custom role per org**: `roles.organization_id` diisi → milik org. `NULL` + `is_system=true`
  → template bawaan; di-copy jadi role milik org saat org dibuat (org bisa ubah tanpa
  mempengaruhi org lain).
- **Permission granular** `module.action` — katalog tetap; role merakit dari katalog.
- **Super admin** = `users.is_platform_admin=true` (di luar tenant), bukan role org.
- **Refresh token** disimpan ter-hash + `revoked_at` → revoke seketika.

## Struktur Modul Go

```
internal/modules/
├── auth/            register, login, refresh, logout, me
│   handler.go service.go repository.go dto.go routes.go errors.go + tests
├── organizations/   buat org, daftar org milik user
│   handler.go service.go repository.go dto.go routes.go errors.go + tests
├── members/         kelola staff, assign role
│   handler.go service.go repository.go dto.go routes.go errors.go + tests
└── roles/           CRUD custom role, assign permission, list permission catalog
    handler.go service.go repository.go dto.go routes.go errors.go + tests

internal/platform/
├── security/
│   ├── password.go  bcrypt hash & verify
│   ├── jwt.go       sign & verify access token (HS256, secret dari env)
│   └── token.go     generate + SHA-256 hash opaque refresh token
├── authctx/
│   └── authctx.go   simpan/ambil identity (user_id, is_platform_admin) di context
└── middleware/
    ├── authn.go     verifikasi access JWT → isi context (siapa kamu)
    └── authz.go     cek permission dalam konteks org (boleh ngapain)
```

Aturan: handler = HTTP only; service = logika (tak sentuh HTTP); repository = query sqlc.
authn = "siapa kamu"; authz = "boleh ngapain". Super admin lolos semua cek authz.

## Alur Token Hybrid & Endpoint Auth

Access token JWT (~15m) tiap request; refresh token opaque (~7h, ter-hash di DB) untuk
perpanjang. Revoke = set `revoked_at`.

```
POST /api/v1/auth/register
  { email, password, fullName, phone }
  → buat user (bcrypt), 201 + profil (tanpa token; login terpisah)

POST /api/v1/auth/login
  { email, password }
  → access JWT (body) + refresh token (HttpOnly cookie)
  resp: { accessToken, expiresIn, user }

POST /api/v1/auth/refresh   (cookie: refresh_token)
  → validasi (ada di DB, belum expired, belum revoked)
  → ROTASI: revoke lama, terbitkan refresh + access baru
  resp: { accessToken, expiresIn }

POST /api/v1/auth/logout    (cookie: refresh_token)
  → revoke refresh, hapus cookie → 204

GET  /api/v1/auth/me        (access token)
  → user + daftar membership/role/permission
```

Keamanan:
- **Rotasi refresh** tiap refresh (sekali pakai; deteksi pencurian).
- Refresh ter-hash (SHA-256) di DB; mentah hanya di cookie.
- Access JWT klaim minimal: `sub`, `exp`, `iat`, `is_platform_admin`. **Tanpa** permission
  di JWT — permission dimuat dari DB saat authz (hindari basi).
- Cookie refresh: `HttpOnly`, `Secure` (prod), `SameSite=Lax`, `Path=/api/v1/auth`.
- Error: `INVALID_CREDENTIALS`, `EMAIL_EXISTS`, `TOKEN_EXPIRED`, `TOKEN_REVOKED`,
  `UNAUTHENTICATED`, dibungkus `{ "error": { code, message, requestId } }`.

## Endpoint Multi-Tenant & RBAC

Semua perlu access token. Authz berbasis permission dalam konteks `orgId` dari URL.

```
POST   /api/v1/organizations                buat org (pembuat → Owner; seed role bawaan org)
GET    /api/v1/organizations                daftar org milik user
GET    /api/v1/organizations/{orgId}        detail (perlu membership)

# perlu permission member.manage
GET    /api/v1/organizations/{orgId}/members
POST   /api/v1/organizations/{orgId}/members            { email, roleIds: [] }
DELETE /api/v1/organizations/{orgId}/members/{memberId}
PUT    /api/v1/organizations/{orgId}/members/{memberId}/roles   { roleIds: [] }

# perlu permission role.manage
GET    /api/v1/organizations/{orgId}/roles
POST   /api/v1/organizations/{orgId}/roles              { name, permissionKeys: [] }
PUT    /api/v1/organizations/{orgId}/roles/{roleId}     { name?, permissionKeys? }
DELETE /api/v1/organizations/{orgId}/roles/{roleId}     (tolak jika is_system / masih dipakai)
GET    /api/v1/organizations/{orgId}/permissions        katalog (read-only)
```

Aturan otorisasi:
- **Isolasi tenant**: query repository difilter `organization_id`. Cek membership org dulu,
  baru permission. Member org A tidak bisa sentuh org B (403).
- **Super admin** (`is_platform_admin`): lolos semua, lihat semua org.
- **Owner terakhir**: tolak hapus/turunkan Owner terakhir.
- **Audit**: aksi sensitif (member add/remove, role change, permission change) → `audit_logs`.

## Seeding & Katalog Permission

Permission catalog + role template + role_permissions template → via **migrasi goose**
(data referensi, idempoten).

Katalog permission (`module.action`):
```
# aktif Phase 2
member.manage  role.manage  organization.manage
# katalog untuk fase depan (belum ada endpoint)
event.create event.edit event.publish event.delete
category.manage  form.manage
order.view order.refund  payment.view payment.refund
participant.view participant.export
coupon.manage  bib.manage  racepack.scan racepack.manage
report.view  broadcast.send
```

Role template bawaan (`organization_id NULL`, `is_system=true`):
```
Owner            semua permission
Manager          event.*, category.manage, form.manage, participant.view,
                 order.view, report.view, broadcast.send, coupon.manage, bib.manage
Finance          order.view, order.refund, payment.view, payment.refund, report.view
Customer Service participant.view, order.view
Racepack Staff   racepack.scan, racepack.manage, participant.view
```

Mekanisme:
- Catalog + template via migrasi.
- `POST /organizations`: service meng-copy semua role template (+ permission) jadi role
  milik org baru, lalu pembuat di-assign role "Owner" org tsb (logika aplikasi).
- Org bebas tambah/ubah/hapus role-nya tanpa mempengaruhi template/org lain.

## Testing

Unit (tanpa DB, fake repository):
- `security/password`: hash↔verify; password salah ditolak.
- `security/jwt`: sign↔verify; expired/tanda tangan salah ditolak.
- `security/token`: mentah ≠ hash; verify hash cocok.
- `auth/service`: register (email dup ditolak); login (kredensial salah ditolak);
  refresh (revoked/expired ditolak + rotasi terjadi).
- `authz`: punya permission lolos; tidak punya ditolak; super admin selalu lolos;
  bukan member ditolak.
- `roles/service`: copy template saat org dibuat; tolak hapus is_system; tolak
  turunkan Owner terakhir.

Integration (Postgres test DB `ivyticketing_test`, di-migrate, dibersihkan per test):
- Alur penuh: register → login → /me → buat org → tambah member → assign role →
  akses endpoint terproteksi.
- Isolasi tenant: member org A → 403 di org B.
- Seed: katalog permission & role template ada setelah migrate up.

## Definition of Done

1. Migrasi roundtrip (up/down): users, organizations, organization_members, roles,
   permissions, role_permissions, member_roles, refresh_tokens, audit_logs.
2. Seed permission catalog & role template ada setelah migrate.
3. Register + login → access JWT + refresh cookie; `/me` benar.
4. Refresh merotasi token; lama invalid; logout merevoke.
5. Buat org → role bawaan ter-copy, pembuat jadi Owner.
6. RBAC: endpoint terproteksi tolak tanpa permission (403), lolos dengan permission.
7. Isolasi tenant terbukti: member org A tak bisa akses data org B.
8. Custom role: bisa dibuat, di-assign, permission-nya berlaku.
9. Aksi sensitif tercatat di `audit_logs`.
10. Super admin bisa akses lintas org.
11. `go test ./...` hijau (unit + integration); tanpa secret hardcoded.
12. `sqlc generate` bersih; semua query lewat sqlc.

## Env Baru

```
JWT_SECRET=            # WAJIB, tanpa default — API gagal start bila kosong
ACCESS_TOKEN_TTL=15m   # default
REFRESH_TOKEN_TTL=168h # default (7 hari)
```

## Setelah Phase 2

Phase 3 — Event & Category Management, di atas fondasi org/RBAC ini.
