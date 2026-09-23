# Desain Spesifikasi: Mobile Bottom Navbar, Opsi Tipe Navigasi Mobile & Gaya Sudut Elemen

Tanggal: 18 September 2026
Status: Disetujui dengan Pembaruan Kustomisasi Admin

---

## 1. Latar Belakang & Tujuan

1. **Mobile Bottom Navbar**: Navigasi mobile dipindahkan ke bagian bawah layar (bottom navigation bar) dengan 4 menu utama ergonomis, dilengkapi header atas minimalis (Logo IVY + status) untuk identitas brand.
2. **Kustomisasi Tipe Navigasi Mobile di Admin**:
   - Admin dapat memilih tipe navigasi mobile: **Mobile Bottom Navbar (Bawaan/Default)** atau **Top Navbar Klasik (Standard Header)**.
   - Nilai bawaan sistem: **Mobile Bottom Navbar**.
3. **Pengecualian Halaman Event ID**: Fitur Mobile Bottom Navbar aktif di semua halaman platform publik (Beranda `/`, Katalog Event `/events`, dll.) kecuali halaman detail lomba `/events/[eventId]/*` yang memiliki navigasi balap mandiri.
4. **Gaya Sudut Elemen (Kotak Tegas / Sharp Boxy) yang Dapat Dikustomisasi di Admin**:
   - Pengguna menginginkan gaya "full kotak" (sharp 90-degree corners, tanpa lengkungan/rounded-none) untuk kartu dan tombol.
   - Gaya sudut ini dapat diatur melalui Admin Studio (`/admin/events`) sehingga admin dapat memilih antara gaya **Kotak Tegas (Square)**, **Melengkung Atletik (Rounded)**, atau **Pill Lengkung (Pill)**.
   - Nilai bawaan sistem (default) ditetapkan ke **Kotak Tegas (Square)**.

---

## 2. Arsitektur & Struktur Data

### 2.1 Ekstensi Tipe Data `HomepageConfig` di `events-store.ts`

```typescript
export type ElementCornerStyle = "square" | "rounded" | "pill";
export type MobileNavType = "bottom_nav" | "top_bar";

export interface CornerConfig {
  style: ElementCornerStyle; // "square" (default) | "rounded" | "pill"
}

export interface NavbarConfig {
  style: NavbarStyleType; // "full_width" | "liquid_glass" | "floating_compact"
  mobileNavType?: MobileNavType; // "bottom_nav" (default) | "top_bar"
  opacity: number;
  blur: "none" | "sm" | "md" | "lg";
  showBorder: boolean;
  glassBlur?: number;
  glassSaturate?: number;
  glassTint?: string;
  glassRadius?: number;
}

export interface HomepageConfig {
  brand: BrandConfig;
  navbar: NavbarConfig;
  corners?: CornerConfig;
  theme: WebsiteThemeConfig;
  features: FeaturesConfig;
  hero: HeroSectionConfig;
}
```

### 2.2 Nilai Bawaan (`DEFAULT_HOMEPAGE_CONFIG`)
- `navbar.style`: `"full_width"`
- `navbar.mobileNavType`: `"bottom_nav"`
- `corners.style`: `"square"`

---

## 3. Komponen Antarmuka Pengguna (UI)

### 3.1 Mobile Bottom Navbar (`#mobile-bottom-nav`)
- **Penempatan**: `fixed bottom-0 left-0 right-0 z-50 w-full backdrop-blur-xl border-t`
- **Kondisi Tampil**: Hanya muncul jika `window.innerWidth < 1024` DAN `navbar.mobileNavType !== "top_bar"`.
- **Daftar 4 Tab Menu Utama**:
  1. **Beranda**: Ikon rumah + teks "Beranda" (`/`)
  2. **Katalog Event**: Ikon kalender/grid + teks "Event" (`/events`)
  3. **War & Ballot**: Ikon tiket/petir + teks "War Tiket" (`/#section-system`)
  4. **Akun Saya**: Ikon pengguna + teks "Akun" (`/participant/dashboard` atau `/login`)
- **Aksesibilitas (Sesuai Antislop)**:
  - Tinggi tombol minimal 48px (standar tap target >= 44px).
  - Teks dan ikon memiliki kontras tinggi dengan indikator aktif beraksen oranye.
  - Halaman diberi bantalan bawah `pb-24` (padding-bottom) agar konten tidak tertutup navigasi bawah.

### 3.2 Header Atas Mobile (`#main-nav`)
- Jika `mobileNavType === "bottom_nav"`:
  - Tampilan atas menjadi minimalis: Logo IVY di kiri dan tombol tema/status di kanan. Tombol hamburger menu disembunyikan karena navigasi beralih ke bawah.
- Jika `mobileNavType === "top_bar"`:
  - Tampilan atas menampilkan hamburger menu klasik dengan dropdown menu mobile.

### 3.3 Penyesuaian Gaya Sudut Kartu & Tombol (Sharp Boxy)
- Menggunakan CSS variable `--element-radius` dan kelas styling terpusat:
  - Saat `corners.style === "square"`: border radius diset ke `0px` (`rounded-none`).
  - Saat `corners.style === "rounded"`: border radius diset ke radius standar (`rounded-xl` pada kartu, `rounded-lg` pada tombol).
  - Saat `corners.style === "pill"`: border radius diset ke lengkung penuh (`rounded-2xl` pada kartu, `rounded-full` pada tombol).

---

## 4. Kustomisasi di Halaman Admin (`/admin/events`)

Di dalam Studio Kustomisasi Admin:
1. **Pengaturan Tipe Navigasi Mobile**:
   - Radio 1: **Mobile Bottom Navbar (Bawaan)**: Navigasi bawah modern 4 tombol jempol.
   - Radio 2: **Top Navbar Klasik**: Header atas dengan hamburger menu drawer.
2. **Pengaturan Gaya Sudut Elemen (Corner Style)**:
   - Radio 1: **Kotak Tegas / Sharp Boxy (Bawaan)**: Sudut siku-siku 90 derajat tanpa lengkungan.
   - Radio 2: **Melengkung Atletik / Modern Rounded**: Sudut melengkung elegan standar.
   - Radio 3: **Pill Lengkung / Smooth Curved**: Sudut melengkung penuh lembut.
3. Simulator pratinjau live di admin langsung merespons kedua pengaturan ini.
4. Perubahan tersimpan secara otomatis ke `localStorage` dan tersinkronisasi ke `/platform-config.json` untuk semua perangkat.

---

## 5. Rencana Verifikasi & Pengujian

1. **Pengujian Playwright Mobile Viewport (390x844)**:
   - Verifikasi Mobile Bottom Navbar muncul saat default di Beranda (`/`) dan Katalog Event (`/events`).
   - Verifikasi Mobile Bottom Navbar TIDAK muncul di halaman event ID (`/events/[eventId]`, queue, checkout).
   - Verifikasi pengubahan ke `top_bar` di Admin memulihkan hamburger menu atas di mobile.
   - Verifikasi pengubahan gaya sudut di Admin (`square` vs `rounded`) merubah border radius kartu secara real-time.
2. **Kepatuhan Antislop**:
   - Zero em dash (`\u2014`).
   - Build aplikasi web berjalan sukses tanpa kesalahan kompilasi.
