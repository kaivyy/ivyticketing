# Organizer Event Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan katalog template marathon dan triathlon siap pakai di halaman pembuatan event organizer (`/org/[orgId]/events/new`) dengan fitur 1-klik autofill dan desain antislop mobile-first.

**Architecture:** Menyisipkan komponen template selector responsif berbasis `MARATHON_TEMPLATES` dari `events-store.ts` di atas formulir pembuatan event di `apps/web/src/pages/org/[orgId]/events/new.astro`. Mengikat event listener untuk melakukan autofill cerdas pada field formulir dan menyinkronkan tema visual template ke data event yang dibuat.

**Tech Stack:** Astro, Tailwind CSS, TypeScript, Playwright untuk verifikasi otomatis.

## Global Constraints
- Zero em dash characters (`\u2014` or `—`). Must use standard hyphens (-) or colons (:).
- All file references in responses must use `file://` scheme.
- Antislop UI and mobile layout rules applied: min 44px touch targets, zero horizontal scroll leak, WCAG contrast.
- Run `graphify update .` after code modifications.

---

### Task 1: Integrasi Komponen Template Carousel di Form Organizer
- **Files to Modify:**
  - `apps/web/src/pages/org/[orgId]/events/new.astro`
- **Langkah-langkah:**
  - [ ] Tambahkan container carousel template di bawah deskripsi judul dan di atas form utama dengan `w-full min-w-0 overscroll-x-contain`.
  - [ ] Render 8 kartu template dari `MARATHON_TEMPLATES` (Summarecon, Tokyo, Berlin, Borobudur, Sydney, Bangkok, IRONMAN, Boston) lengkap dengan foto banner, badge jenis lomba, nama template, dan tag kategori.
  - [ ] Tambahkan tombol "Gunakan Template" dan status badge aktif saat salah satu template terpilih.
  - [ ] Tambahkan tombol "Reset / Mulai dari Form Kosong".

### Task 2: Logika Autofill Interaktif & Sinkronisasi Form
- **Files to Modify:**
  - `apps/web/src/pages/org/[orgId]/events/new.astro`
- **Langkah-langkah:**
  - [ ] Buat fungsi JavaScript client-side untuk menangani klik tombol template.
  - [ ] Isi field `name`, `eventType`, `venueName`, `venueAddress`, `description`, serta sesuaikan radio pendaftaran `primaryMode` (WAR_QUEUE / BALLOT / NORMAL).
  - [ ] Isi field kategori perdana (`catName`, `catPrice`, `catCapacity`, `catBib`) sesuai data template.
  - [ ] Tampilkan visual toast / highlight bahwa template berhasil diterapkan.
  - [ ] Simpan metadata template (bannerUrl, themeColor, layoutConfig) saat form disubmit sehingga event publik langsung tampil konsisten.

### Task 3: Verifikasi Playwright & Audit Mobile Responsiveness
- **Files to Create:**
  - `scratch/test_organizer_templates.py`
- **Langkah-langkah:**
  - [ ] Buat skrip audit Playwright untuk menguji interaksi pemilihan template di layar mobile (390px) dan desktop (1440px).
  - [ ] Uji autofill saat tombol template diklik dan pastikan nilai input form terisi dengan tepat.
  - [ ] Verifikasi ketiadaan horizontal overflow (`scrollWidth === clientWidth`) pada layar mobile.
  - [ ] Ambil screenshot hasil untuk verifikasi visual artifact.

### Task 4: Sinkronisasi Knowledge Graph & Finalisasi
- **Langkah-langkah:**
  - [ ] Jalankan `pnpm --dir apps/web build` untuk memastikan tidak ada error TypeScript/Astro.
  - [ ] Jalankan `graphify update .` untuk memperbarui graph ketergantungan.
  - [ ] Pastikan tidak ada karakter em dash (`—`) pada seluruh teks laporan.
