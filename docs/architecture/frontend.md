# Frontend Architecture and User Interfaces

The IvyTicketing frontend ecosystem consists of two distinct client applications: the **Astro Web Portal** for participants and organizers, and the **Svelte Scanner PWA** for race expo and gate staff.

---

## 1. Web Portal Architecture (`apps/web`)

The primary web application is built with **Astro 5**, **TypeScript**, and **Tailwind CSS**.

### Architecture Principles

- **Server-First Hybrid Rendering**: Marketing, event discovery, and documentation pages are rendered statically or on the server for instant page loads and optimal SEO.
- **Interactive Islands**: Dynamic features (such as live queue polling bars, checkout timers, and interactive seating charts) are hydrated as lightweight client-side components using React or vanilla TypeScript.
- **Strict TypeScript**: The entire codebase enforces strict TypeScript with zero `any` usage.

### Portal Route Hierarchy

```
apps/web/src/pages/
├── index.astro                  # Marketing landing page
├── events/
│   ├── index.astro              # Public events directory with search & filters
│   └── [slug].astro             # Event details, categories, routes, and pricing
├── register/
│   └── [slug].astro             # Registration checkout island & war queue waiting room
├── auth/
│   ├── login.astro              # Participant & staff login
│   └── register.astro           # New account registration
├── dashboard/                   # Participant self-service portal
│   ├── index.astro              # Overview of upcoming races
│   ├── orders.astro             # Order history and payment links
│   ├── tickets.astro            # Issued HMAC QR tickets
│   └── profile.astro            # Athlete medical info, jersey sizes, and emergency contacts
├── organizations/[slug]/        # Organizer workspace
│   ├── dashboard.astro          # Event list and ticket sales charts
│   ├── events/                  # Event creation and category quota management
│   ├── queue.astro              # Real-time war queue controls and release rates
│   ├── ballots.astro            # Lottery draw execution and conversion analytics
│   ├── racepack.astro           # Expo slots, counter rosters, and problem desk
│   ├── results.astro            # Timing CSV imports and certificate designer
│   └── team.astro               # Staff invitations and RBAC role assignments
└── admin/                       # Platform administrator console
    ├── warroom.astro            # Real-time connection saturation and queue depths
    ├── organizations.astro      # Tenant provisioning and subscription plans
    ├── abuse.astro              # Bot rules, IP blocklists, and Turnstile settings
    └── billing.astro            # Platform fee ledger and invoice management
```

---

## 2. Scanner PWA Architecture (`apps/scanner`)

The gate and expo scanner is implemented as a standalone Progressive Web App (PWA) using **Svelte 5** and **Vite**.

### PWA Capabilities

- **Offline-First Resilience**: Service worker caches all application assets, allowing staff to launch and operate the scanner even in areas with zero cellular reception.
- **Hardware Camera Access**: Utilizes the native Barcode Detection API (with JavaScript fallback) to achieve sub-second QR code capture.
- **Local Cryptographic Verification**: The scanner verifies the HMAC-SHA256 signature locally against the event secret, authenticating tickets without requiring an online database query.
- **Offline Scan Synchronization**: Scans performed while disconnected are buffered in local IndexedDB storage and automatically synchronized to `POST /api/v1/scan/verify` once network connectivity is re-established.
