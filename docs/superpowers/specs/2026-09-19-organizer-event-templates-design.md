# Desain Fitur: Pemilihan Template Event untuk Portal Organizer

## Ringkasan Eksekutif
Portal Organizer saat ini memiliki formulir pembuatan event manual di `/org/[orgId]/events/new`. Fitur ini menambahkan galeri template event olahraga siap pakai (8 desain teruji: Summarecon Bandung, Tokyo, Berlin, Borobudur, Sydney, Bangkok, IRONMAN, Boston) di bagian atas formulir pembuatan event. Ketika organizer memilih salah satu template, seluruh field penting (nama event, disiplin olahraga, venue, mekanisme pendaftaran War/Ballot/Normal, kategori tiket awal, kuota, harga standar, deskripsi rute, dan banner) terisi otomatis (autofill). Organizer tetap memiliki kontrol penuh untuk menyesuaikan data sebelum disimpan.

## Perbedaan Peran (Superadmin vs Event Organizer)
1. **Superadmin (`/admin/events`):**
   - Kontrol tingkat platform: mengubah alignment hero public site, varian tampilan split/centered/cinematic, override styling CSS global, reset database platform, integrasi timing chip mentah.
2. **Event Organizer (`/org/[orgId]/events/new`):**
   - Kontrol tingkat operasional event: identitas lomba, tanggal & waktu cut-off, venue lokal, kuota atlet, biaya pendaftaran kategori, waiver persetujuan atlet.
   - Menggunakan template sebagai starter kit (fondasi cepat), bukan konfigurasi global sistem.

## Arsitektur & Aliran Data
1. **Sumber Data Template:**
   - Memanfaatkan preset `MARATHON_TEMPLATES` dari `apps/web/src/lib/events-store.ts`.
   - 8 template:
     - `template-summarecon` (Urban Fest & Neon 10K/5K, Mode War Ticket)
     - `template-tokyo` (World Major Marathon & 10.7K, Mode Ballot)
     - `template-berlin` (Speed Course Flat PB, Mode War Ticket)
     - `template-borobudur` (Heritage Cultural Run, Mode War Ticket)
     - `template-sydney` (Ocean & Harbour Bridge Course, Mode Ballot)
     - `template-bangkok` (Midnight Illumination Run, Mode Normal)
     - `template-ironman` (Triathlon Swim-Bike-Run 70.3, Mode War Ticket)
     - `template-baa` (Boston Historic Course, Mode Priority/War)
2. **Komponen Visual UI:**
   - Bagian Carousel Template di atas form dengan scroll horizontal mobile (`overscroll-x-contain` dan `scrollbar-none`).
   - Setiap kartu template menampilkan foto banner, badge jenis lomba, nama template, kategori bawaan, dan tombol "Gunakan Template".
   - Indikator template aktif (border oranye, icon ceklis).
   - Tombol "Mulai dari Kosong (Reset Form)" untuk mengembalikan form ke status bersih.
3. **Mekanisme Pengisian (Autofill):**
   - Mengisi input `name` dengan `${tpl.name} 2026`.
   - Mengisi select `eventType` (`MARATHON` atau `TRIATHLON`).
   - Mengisi input `venueName` dan `venueAddress`.
   - Mengisi textarea `description` dengan sorotan rute dan sertifikasi.
   - Memilih radio button `primaryMode` (`WAR_QUEUE`, `BALLOT`, `RANDOMIZED_QUEUE`, atau `NORMAL`) dan memicu pembaruan styling radio.
   - Mengisi input `catName`, `catPrice`, `catCapacity`, dan `catBib` dari `sampleCategories[0]` template.
   - Menyimpan preferensi banner dan tema warna ke penyimpanan event publik.
4. **Prinsip Antislop & Mobile Responsiveness:**
   - Target sentuh minimum 44px (`min-h-[44px]`).
   - Kungkungan horizontal ketat (`w-full min-w-0 overscroll-x-contain`).
   - Kontras warna teks memenuhi WCAG.
   - Tipografi terstruktur, bebas jargon klise AI.
