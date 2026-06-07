# Master Development Plan — Race Registration Platform

## Prinsip Utama

Platform dibangun bertahap:

1. Jangan langsung microservice.
2. Mulai dari modular monolith Go.
3. Payment dan inventory harus benar dulu.
4. Queue/war dibuat setelah order locking stabil.
5. Racepack dibuat setelah ticket/BIB stabil.
6. Enterprise/white-label dibuat setelah core SaaS matang.

---

# Phase 0 — Product & Technical Foundation

## Tujuan

Menentukan fondasi produk, arsitektur, standar coding, dan batas MVP.

## Output

* PRD final
* System Design Document
* Database draft
* UI flow utama
* API draft
* Risk register
* Technical decision record

## Modul yang dirancang

* Multi-tenant organization
* Event management
* Registration
* Payment
* Queue
* Ballot
* BIB
* Racepack
* Dashboard
* Notification
* Reporting

## Acceptance Criteria

* Semua role jelas
* Semua flow utama jelas
* MVP scope tidak melebar
* Stack final disetujui
* Risiko war/payment/racepack sudah dipetakan

---

# Phase 1 — Monorepo & Dev Foundation

## Tujuan

Membuat struktur project yang rapi dan siap dikembangkan jangka panjang.

## Deliverables

* Monorepo
* Astro apps
* Go API
* Go worker
* PostgreSQL
* Redis/DragonflyDB
* Docker Compose
* Makefile
* CI basic
* Logging basic
* Env management

## Struktur awal

```txt
apps/web
apps/organizer-dashboard
apps/admin-dashboard
apps/scanner

services/api
services/worker

database
infra
docs
tests
```

## Acceptance Criteria

* Semua service bisa jalan lokal
* Database migration jalan
* API health check tersedia
* Frontend bisa call API
* README setup lengkap
* Tidak ada secret hardcoded

---

# Phase 2 — Auth, RBAC & Multi-Tenant Core

## Tujuan

Membuat dasar SaaS multi-organizer.

## Fitur

### Auth

* Login
* Register
* Forgot password
* Session management
* JWT/session token

### Multi-Tenant

* Organization
* Organization member
* Role
* Permission

### Roles

Super Admin:

* Platform owner

Organizer:

* Owner
* Manager
* Finance
* Customer Service
* Racepack Staff

Participant:

* Peserta

## Acceptance Criteria

* User bisa login
* Organizer hanya melihat data miliknya
* Super admin bisa melihat semua organizer
* Staff permission dibatasi
* Semua admin action masuk audit log

---

# Phase 3 — Event & Category Management

## Tujuan

Organizer bisa membuat event dan kategori lomba.

## Fitur

### Event

* Create event
* Edit event
* Publish/unpublish
* Event banner
* Venue
* Description
* Schedule
* FAQ
* Terms
* Waiver

### Category

* 42K
* 21K
* 10K
* 5K
* Kids Dash

Per category:

* Price
* Capacity
* Registration window
* BIB prefix
* Minimum age
* Max order per user

## Acceptance Criteria

* EO bisa membuat event
* EO bisa membuat kategori
* Event bisa tampil di public page
* Capacity tersimpan benar
* Draft event tidak muncul publik

---

# Phase 4 — Custom Registration Form Builder

## Tujuan

Organizer bisa membuat form custom tanpa coding.

## Field Types

* Text
* Email
* Phone
* Number
* Date
* Dropdown
* Radio
* Checkbox
* Textarea
* File upload

## Contoh Field

* Nama lengkap
* No HP
* Email
* Gender
* Tanggal lahir
* Golongan darah
* Kontak darurat
* Nomor kontak darurat
* Komunitas lari
* Ukuran jersey
* Riwayat penyakit
* Link Strava
* Passport number

## Advanced

* Required/optional
* Conditional logic
* Field per category
* Validation per field
* Form preview

## Acceptance Criteria

* EO bisa membuat form
* Peserta bisa mengisi form
* Conditional field berjalan
* Data tersimpan sebagai submission
* Form bisa di-export

---

# Phase 5 — Inventory, Order & Checkout Core

## Tujuan

Membuat sistem order yang aman dari oversold.

## Fitur

### Inventory

* Capacity per category
* Remaining slot
* Reserved slot
* Paid slot
* Expired slot

### Order

Status:

* Draft
* Pending
* Paid
* Expired
* Cancelled
* Refunded

### Reservation

* Slot dikunci saat checkout
* Payment timeout
* Slot dilepas jika expired

## Non-Negotiable

Stock harus atomic.

Tidak boleh:

```txt
check stock -> create order
```

Harus:

```txt
atomic decrement / transaction lock
```

## Acceptance Criteria

* Tidak bisa oversold
* Order dobel dicegah
* Refresh tidak membuat order baru
* Expired order melepas slot
* Load test checkout lolos

---

# Phase 6 — Payment Gateway V1

## Tujuan

Integrasi payment gateway pertama.

## Prioritas Gateway

1. Duitku
2. Xendit
3. Midtrans

## Fitur

* Create payment
* QRIS
* VA
* Payment status
* Callback/webhook
* Signature validation
* Idempotency
* Payment log
* Retry processing
* Manual reconcile

## Acceptance Criteria

* Pembayaran sukses mengubah order ke PAID
* Callback dobel tidak membuat efek dobel
* Callback invalid ditolak
* Payment expired aman
* Semua callback tersimpan di log

---

# Phase 7 — Participant Dashboard & Ticket

## Tujuan

Peserta bisa melihat order, tiket, dan status event.

## Fitur

* My Events
* My Orders
* Payment status
* Ticket status
* QR ticket
* Invoice
* Timeline order

## Ticket

QR token berisi:

```txt
ticket_id
event_id
signature
```

Tidak boleh menyimpan data sensitif di QR.

## Acceptance Criteria

* Peserta bisa melihat tiket setelah paid
* QR valid dan signed
* QR invalid ditolak
* Ticket bisa di-download
* Order timeline jelas

---

# Phase 8 — Queue / War Ticket System

## Tujuan

Membuat sistem antrean yang adil, transparan, dan tidak reset.

## Fitur Peserta

* Join queue
* Queue token permanen
* Nomor antrean
* Estimasi waktu
* User di depan
* Status sistem
* Auto reconnect
* Refresh safe
* Mobile sleep safe

## Fitur Admin

* Start queue
* Pause queue
* Resume queue
* Throttle release
* Set release rate
* View active users
* View error rate
* Queue incident banner

## Queue Status

* Waiting
* Allowed to checkout
* Expired
* Completed
* Blocked

## Acceptance Criteria

* Refresh tidak reset antrean
* Reconnect tidak reset antrean
* User tidak bisa punya banyak antrean untuk event sama
* Admin bisa pause/resume
* Load test minimal 100.000 simulated users
* Error message manusiawi

---

# Phase 9 — Anti-Bot & Abuse Protection

## Tujuan

Mengurangi bot tanpa menyiksa user normal.

## Fitur

* Cloudflare WAF
* Turnstile
* Rate limit
* IP reputation
* Device/session fingerprint ringan
* Queue duplicate detection
* Suspicious behavior flag
* Max order per user
* Max queue entry per user

## Acceptance Criteria

* Bot spam join queue bisa ditahan
* User normal tidak kena captcha berulang
* Abuse log tersedia
* Admin bisa block/unblock user
* Rate limit tidak merusak payment callback

---

# Phase 10 — Ballot / Lottery System

## Tujuan

Mendukung event dengan sistem undian.

## Flow

```txt
Open ballot
↓
User submit registration
↓
Ballot closes
↓
Draw
↓
Winner gets payment window
↓
Unpaid winner expired
↓
Waitlist promoted
```

## Fitur

* Ballot period
* Eligibility rule
* Random draw
* Winner announcement
* Waitlist
* Payment deadline
* Audit draw

## Acceptance Criteria

* Draw bisa diaudit
* Winner dapat payment link
* Payment expired pindah ke waitlist
* EO bisa export ballot data
* Peserta bisa cek status ballot

---

# Phase 11 — Coupon, Invitation & Community Slot

## Tujuan

Mendukung promo, presale, community slot, dan corporate registration.

## Fitur

* Coupon fixed amount
* Coupon percentage
* Invitation code
* Community quota
* Corporate bulk registration
* Per-category restriction
* Usage limit
* Expiry date

## Acceptance Criteria

* Coupon tidak bisa dipakai melebihi quota
* Invitation code bisa lock kategori tertentu
* Bulk order tidak merusak inventory
* Semua redemption tercatat

---

# Phase 12 — Notification System

## Tujuan

Mengirim komunikasi otomatis dan broadcast.

## Channel

* Email
* WhatsApp
* Optional SMS

## Template

* Registration created
* Payment pending
* Payment success
* Payment expired
* Queue allowed
* Ballot winner
* Ballot not selected
* Racepack reminder
* Event reminder
* Result available

## Acceptance Criteria

* Template bisa dicustom EO
* Notification retry tersedia
* Failed notification tercatat
* Broadcast bisa ditarget berdasarkan event/category/status

---

# Phase 13 — BIB Management

## Tujuan

Membuat sistem nomor BIB yang rapi dan aman.

## Mode

* Auto sequential
* Randomized
* Manual upload

## Format

```txt
42K = A0001
21K = B0001
10K = C0001
5K = D0001
```

## Rules

* BIB final setelah PAID
* BIB unique per event
* Prefix custom per category
* Duplicate prevention

## Acceptance Criteria

* Paid participant otomatis dapat BIB
* Pending participant belum dapat BIB final
* BIB tidak duplikat
* EO bisa export BIB
* EO bisa override dengan permission khusus

---

# Phase 14 — Racepack Pickup System

## Tujuan

Menghindari antrean fisik saat pengambilan racepack.

## Fitur

* Pickup slot scheduling
* Slot quota
* Counter assignment
* QR racepack pass
* Pickup status
* Problem desk
* Proxy pickup

## Pickup Status

* Not ready
* Ready
* Picked up
* Picked up by proxy
* Cancelled

## Acceptance Criteria

* Peserta bisa pilih slot
* Slot penuh tidak bisa dipilih
* QR hanya valid untuk peserta terkait
* Pickup tidak bisa dobel
* Audit pickup lengkap

---

# Phase 15 — Scanner PWA

## Tujuan

Membuat scanner cepat untuk racepack dan check-in.

## Fitur

* Login staff
* Scan QR
* Show participant
* Confirm pickup
* Duplicate warning
* Offline mode
* Local queue
* Sync when online

## Target

* 5–10 detik per peserta
* Bisa jalan di HP biasa
* Bisa PWA install

## Acceptance Criteria

* QR valid bisa diproses
* QR sudah diambil muncul warning
* Offline scan tersimpan lokal
* Sync tidak membuat duplikat
* Staff hanya bisa akses event yang diizinkan

---

# Phase 16 — Reporting & Export

## Tujuan

EO bisa mengambil data operasional dan keuangan.

## Reports

* Participant report
* Sales report
* Payment report
* Coupon report
* Queue report
* Ballot report
* Racepack report
* Revenue report

## Export

* CSV
* Excel
* PDF summary

## Acceptance Criteria

* Export besar tidak membuat API lambat
* Export diproses worker
* EO bisa download hasil export
* Super admin bisa lihat aggregate semua EO

---

# Phase 17 — Super Admin Platform Billing

## Tujuan

Monetisasi platform.

## Fitur

* Organizer subscription
* Platform fee
* Invoice
* Payment to platform
* Revenue dashboard
* Package limit

## Paket

Starter:

* 1–3 event
* Basic registration
* Basic payment

Professional:

* Queue
* Ballot
* Racepack
* Custom branding

Enterprise:

* White label
* Custom domain
* Dedicated support
* Custom payment
* Dedicated queue

## Acceptance Criteria

* Paket membatasi fitur
* Billing tercatat
* Platform fee terlihat
* Invoice bisa di-generate
* Organizer bisa upgrade

---

# Phase 18 — White Label & Custom Domain

## Tujuan

EO besar bisa menggunakan domain dan branding sendiri.

## Fitur

* Custom domain
* Custom logo
* Custom theme
* Custom email sender
* Custom terms
* Custom footer
* White-label mode

## Acceptance Criteria

* Domain custom bisa diverifikasi
* Branding tampil konsisten
* SSL aktif
* EO tidak melihat branding platform jika white-label aktif

---

# Phase 19 — Public Status & Incident System

## Tujuan

Mengurangi panik saat war dan meningkatkan trust.

## Fitur

Public status page:

* Queue normal/degraded
* Payment normal/degraded
* Registration normal/degraded
* Incident banner
* Last updated

Admin incident:

* Create incident
* Update incident
* Resolve incident

## Acceptance Criteria

* Status page bisa diakses publik
* Admin bisa update status cepat
* Banner muncul di queue/checkout
* Incident history tersimpan

---

# Phase 20 — Observability & War Room Dashboard

## Tujuan

Mendeteksi masalah sebelum viral.

## Metrics

* Active queue users
* Queue release rate
* Checkout success rate
* Payment success rate
* API p95 latency
* Error rate
* DB connection usage
* Redis latency
* Gateway webhook delay
* Racepack scan rate

## Tools

* Prometheus
* Grafana
* Loki
* Sentry

## Acceptance Criteria

* Dashboard war day tersedia
* Alert error rate aktif
* Request ID bisa dilacak dari frontend sampai backend
* Payment incident bisa dianalisis

---

# Phase 21 — Load Testing & Reliability Hardening

## Tujuan

Membuktikan sistem kuat sebelum dipakai event besar.

## Test Scenarios

* 10.000 users
* 50.000 users
* 100.000 users
* 500.000 waiting room users
* Payment callback spike
* Checkout race condition
* Redis failover
* Database slow query
* Gateway down
* User refresh storm
* Mobile reconnect storm

## Acceptance Criteria

* Tidak oversold
* Tidak double payment
* Tidak queue reset massal
* API p95 sesuai target
* Incident runbook valid

---

# Phase 22 — Security Hardening

## Tujuan

Melindungi data peserta dan transaksi.

## Scope

* RBAC review
* SQL injection test
* XSS test
* CSRF protection
* Webhook signature
* QR signature
* PII encryption where needed
* Audit log immutability
* Admin action approval for sensitive changes

## Acceptance Criteria

* No critical vulnerability
* Webhook palsu ditolak
* QR palsu ditolak
* Staff tidak bisa akses event lain
* Sensitive data tidak muncul di logs

---

# Phase 23 — Enterprise API & Integration

## Tujuan

Membuka integrasi untuk EO besar.

## API

* Event API
* Participant API
* Order API
* Payment status API
* Racepack API
* Result import API
* Webhook outbound

## Acceptance Criteria

* API key per organization
* Rate limit per API key
* API docs lengkap
* Sandbox tersedia
* Webhook outbound idempotent

---

# Phase 24 — Result, Certificate & Timing Integration

## Tujuan

Menambah fitur pasca lomba.

## Fitur

* Import results CSV
* Timing vendor API
* Ranking
* Gender rank
* Age group rank
* Certificate generator
* Result page
* DNF/DNS support

## Acceptance Criteria

* EO bisa import hasil
* Peserta bisa download sertifikat
* Ranking bisa difilter
* Certificate template custom

---

# Phase 25 — Enterprise Scale Split

## Tujuan

Memecah service hanya jika diperlukan.

## Dari Modular Monolith ke Service

Pisahkan jika traffic sudah besar:

* Webhook service
* Queue service
* Payment service
* Notification service
* Report service

## Jangan split terlalu cepat.

Trigger split:

* Payment callback mengganggu API utama
* Queue traffic sangat tinggi
* Report export membebani DB
* Notification volume besar

## Acceptance Criteria

* Service split tanpa rewrite besar
* Contract API jelas
* Monitoring per service tersedia

---

# Phase 26 — Production Launch Checklist

## Before Launch

* Load test selesai
* Payment test selesai
* Backup aktif
* Monitoring aktif
* Sentry aktif
* Runbook siap
* Admin training selesai
* EO training selesai
* Support channel siap

## War Day Checklist

* Queue enabled
* Payment gateway healthy
* Release rate diset
* Staff standby
* Status page ready
* Incident banner ready
* Database backup verified

## Acceptance Criteria

* Launch rehearsal berhasil
* Rollback plan tersedia
* Emergency contacts jelas
* Post-war report bisa dibuat

---

# Phase 27 — Continuous Improvement

## Setelah Event

Analisis:

* Queue complaints
* Payment failed reason
* Checkout drop-off
* Racepack bottleneck
* Support ticket
* Error logs
* Organizer feedback
* Participant feedback

## Output

* Post-event report
* Bug list
* Improvement backlog
* Performance tuning
* Pricing review

---

# MVP Scope Recommendation

## MVP Wajib

* Auth
* Multi-tenant organization
* Event management
* Category management
* Custom form basic
* Order
* Inventory lock
* Duitku payment
* Payment webhook
* Participant ticket
* Organizer dashboard
* Super admin dashboard basic
* BIB basic
* Racepack scanner basic

## MVP Plus

* Queue war mode
* Pickup slot
* Notification email/WA
* Reports
* Coupon

## Jangan Masuk MVP Awal

* Full white-label
* Native mobile app
* Timing integration
* Certificate generator
* Marketplace sponsor
* Hotel/travel integration
* Complex affiliate system

---

# Recommended Build Order

## Month 1

* Repo foundation
* Auth
* Organization
* Event
* Category
* Form basic

## Month 2

* Order
* Inventory
* Duitku
* Webhook
* Participant ticket

## Month 3

* Organizer dashboard
* Super admin dashboard
* BIB
* Racepack scanner basic
* Reports basic

## Month 4

* Queue war mode
* Anti-bot
* Load testing
* Status page
* Notification

## Month 5

* Ballot
* Xendit
* Midtrans
* Pickup slot
* Proxy pickup

## Month 6

* Billing platform
* White label basic
* Enterprise hardening
* Production launch preparation

---

# Release Version Plan

## v0.1 — Internal Prototype

* Basic event
* Basic registration
* Basic payment sandbox

## v0.2 — MVP Alpha

* Real payment
* Inventory lock
* Organizer dashboard
* Participant ticket

## v0.3 — Private Beta

* First small EO
* 500–2.000 participants
* Racepack scanner

## v0.4 — War Beta

* Queue enabled
* 10.000 simulated users
* Status page

## v1.0 — Public Launch

* Normal sale
* War sale
* Payment
* BIB
* Racepack
* Reports
* SaaS organizer dashboard

## v1.5 — Growth

* Ballot
* Multi gateway
* Pickup slot
* Notification
* Coupon

## v2.0 — Enterprise

* White label
* Custom domain
* Dedicated queue
* API access
* Advanced reporting

---

# Critical Risk List

## Risk 1 — Overselling

Mitigation:

* Atomic stock update
* Transaction lock
* Inventory reservation
* Load test

## Risk 2 — Queue Reset

Mitigation:

* Server-side queue token
* Reconnect handling
* Durable queue state

## Risk 3 — Double Payment

Mitigation:

* Idempotency key
* Payment attempt table
* Webhook deduplication

## Risk 4 — Payment Gateway Down

Mitigation:

* Status page
* Queue pause
* Fallback gateway
* Reconciliation

## Risk 5 — Racepack Chaos

Mitigation:

* Pickup slot
* Counter split
* Scanner PWA
* Problem desk

## Risk 6 — Bot Attack

Mitigation:

* Cloudflare
* Turnstile
* Rate limit
* Fingerprint
* Account limit

---

# Final Priority Rule

Urutan prioritas engineering:

1. Correctness
2. Stability
3. Security
4. Transparency
5. Speed
6. UI polish

Untuk war tiket, sistem yang “agak lambat tapi jelas dan tidak rusak” lebih baik daripada sistem yang “terlihat cepat tapi error, oversold, atau antrian reset.”
