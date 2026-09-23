# IvyTicketing

High-throughput, multi-tenant registration, ticketing, and competition management platform engineered for endurance races (marathons, triathlons, cycling gran fondos) and multi-sport tournaments.

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Astro](https://img.shields.io/badge/Astro-5.0-FF5D01?logo=astro&logoColor=white)](https://astro.build/)
[![Svelte](https://img.shields.io/badge/Svelte-5.0-FF3E00?logo=svelte&logoColor=white)](https://svelte.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white)](https://redis.io/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## What is IvyTicketing?

IvyTicketing is a competition and event management platform built to solve the hardest problem in sports ticketing: extreme "war-day" traffic surges when thousands of competitors hit registration in the exact same second.

Unlike simple e-commerce shopping carts or generic ticketing software, IvyTicketing unifies the entire competitive sports lifecycle: from pre-sale waiting rooms and randomized ballot draws to row-locked inventory reservations, physical race bib assignment, expo racepack pickup, timing mat RFID integrations, and multi-sport tournament bracket adjudication.

### Who is it for?

- **Athletes & Participants**: Enjoy fair waiting rooms, transparent ballot draws, instant HMAC-signed QR tickets, proxy pickup authorization, and official finisher certificates.
- **Race Organizers**: Control traffic flow, configure 8 registration modes, allocate sequential BIB numbers, staff expo pickup desks, upload chip timing CSVs, and manage tournament stages.
- **Timing & Gate Operators**: Scan tickets offline with a dedicated PWA scanner, ingest transponder passings, recalculate category ranks, and print finisher certificates.
- **Platform Administrators**: Supervise multi-tenant organizations, inspect live war-room database saturation, tune anti-bot challenge thresholds, and track platform fee ledgers.

---

## The End-to-End Competition Pipeline

IvyTicketing separates admission control from inventory allocation and connects every stage of race operations:

```
Event Management
      ↓
Registration Policy (8 Supported Modes: Normal, War Queue, Ballot, etc.)
      ↓
Admission Control (Redis Waiting Room / Lottery Winner Grant / Priority Pass)
      ↓
Inventory Allocation (PostgreSQL Row-Level Lock: 15-Minute Hold)
      ↓
Orders & Commerce (Cart Validation, Dynamic Form Fields, Coupon Engine)
      ↓
Payments (Duitku & Xendit Gateways with Idempotent Webhook Verification)
      ↓
Tickets & BIB Assignment (HMAC-SHA256 Signed QR Tickets & Sequential BIB Numbering)
      ↓
Racepack Expo & Gate Entry (Time Slot Booking, Proxy Verification, Offline PWA Scanner)
      ↓
Competition Engine (Stages, Knockout Brackets, Round-Robin Pools, Heats, BWF Scoring)
      ↓
Results & Rankings (RFID Mat Timing Passings, Net Chip Times, Finisher Certificates)
```

---

## Key Capabilities

### 1. Registration & Traffic Control
- **8 Registration Modes**: `NORMAL`, `WAR_QUEUE`, `RANDOMIZED_QUEUE`, `HYBRID_QUEUE`, `BALLOT`, `INVITATION_ONLY`, `WAITLIST_ONLY`, and `PRIORITY_ACCESS`.
- **Admission vs Allocation Separation**: Queue position or ballot winning grants admission to checkout; only database row locks reserve physical capacity.
- **High-Traffic War Queue**: In-memory Redis sorted sets (`ZSET`) absorb surges; status queries are cached in Redis for 2 seconds to protect database connection pools.
- **Ballot & Lottery Engine**: Cryptographic random draw, 48-hour payment windows, and automated waitlist promotion cascade on winner lapse.

### 2. Commerce & Zero-Overselling Guarantee
- **Database Row Locking**: Quota decrements execute via `SELECT ... FOR UPDATE` inside PostgreSQL transactions. Overselling is mathematically impossible.
- **15-Minute Reservation Holds**: Uncompleted checkouts hold inventory for 15 minutes before the `expire_orders` worker automatically returns capacity to the pool.
- **Payment Gateway Integrations**: Direct integration with Duitku and Xendit supporting QRIS, Virtual Accounts, E-Wallets, and Credit Cards.
- **Strict Webhook Idempotency**: Inbound payment callbacks are SHA-256 hashed and deduplicated; retried callbacks never issue duplicate tickets or fees.

### 3. Event Logistics & Ticketing
- **Cryptographic HMAC QR Tickets**: Tamper-proof tickets signed using HMAC-SHA256 with offline validation capability.
- **BIB Allocation Engine**: Automated sequential numbering per category range (`AUTO`), manual VIP assignments (`MANUAL`), and on-site swaps (`OVERRIDE`).
- **RFID Transponder Mapping**: Bridges physical race bib numbers to electronic timing chips.
- **Racepack Expo Logistics**: Capacity-constrained pickup slot booking, proxy collection authorization with ID upload, and Problem Desk dispute tracking.
- **Offline PWA Scanner**: Standalone Svelte 5 application capable of scanning and verifying tickets with zero internet connectivity.

### 4. Multi-Sport Competition & Timing Engine
- **Endurance Race Timing**: Ingests raw RFID mat passings, computes split times (5K, 10K, Halfway, 30K), and calculates net chip times and gross gun times.
- **Generic Tournament Hierarchy**: Normalized schema for Sports, Disciplines, Events, Categories, Stages, Matches, and Rosters.
- **Tournament Stage Formats**: Single race, round-robin pools, single elimination, double elimination, heats, and multi-day peloton stages.
- **Sport-Specific Scoring Resolvers**: Implements official BWF badminton rally point rules and road cycling peloton time gap rules.
- **Automated Finisher Certificates**: Dynamic SVG/PDF rendering with athlete name, category, net time, gun time, and category rank.

### 5. Multi-Tenant Enterprise Platform
- **Role-Based Access Control**: 38 granular permissions mapped to 6 pre-configured system roles (`Owner`, `Manager`, `Finance`, `Customer Service`, `Racepack Staff`, `Timing Operator`).
- **Strict BOLA Defense**: Scoped URL parameters and database ownership constraints prevent cross-tenant object access.
- **Anti-Bot & AbuseGuard**: Cloudflare Turnstile integration, IP reputation scoring, sliding-window rate limiters, and CIDR blocklists.
- **White-Labeling & Custom Domains**: Custom branding themes and cryptographic DNS TXT verification for custom organizer subdomains.
- **Real-Time War Room Telemetry**: Live dashboard reporting active database pool connections, queue depths, release rates, and gateway callback latency.

---

## High-Level Architecture

```mermaid
graph TD
    subgraph Clients["Client Applications"]
        Browser[Participant Web Portal<br/>Astro 5 + Tailwind]
        Scanner[Gate & Expo Scanner<br/>Svelte 5 Offline PWA]
        ExtApp[Enterprise Integrations<br/>REST API]
    end

    subgraph Edge["Edge Layer"]
        CF[Cloudflare CDN & Turnstile Protection]
    end

    subgraph Application["Application Layer"]
        Router[Chi HTTP REST API :8080<br/>services/api/cmd/api]
        Abuse[AbuseGuard Middleware]
        Modules[Domain Logic Modules]
        Workers[Background Worker Daemon<br/>services/api/cmd/worker]
    end

    subgraph Persistence["Storage & Cache"]
        PG[(PostgreSQL 16 Engine<br/>Authoritative State)]
        Redis[(Redis 7 Engine<br/>Queues, Caching, Rate Limits)]
        Media[(Local Media / S3 Storage)]
    end

    subgraph External["External Services"]
        Duitku[Duitku Gateway]
        Xendit[Xendit Gateway]
        SMTP[SMTP Email Server]
    end

    Clients --> CF
    CF --> Router
    Router --> Abuse
    Abuse --> Modules
    Modules --> PG
    Modules --> Redis
    Modules --> Media

    Workers --> PG
    Workers --> Redis
    Workers --> SMTP

    Modules --> Duitku
    Modules --> Xendit
    Duitku -->|Webhook| Router
    Xendit -->|Webhook| Router
```

---

## Verified Technology Stack

| Layer | Component | Version / Specification | Purpose |
| :--- | :--- | :--- | :--- |
| **Backend** | Go | `1.25.5` | High-concurrency modular monolith |
| **HTTP Router** | `go-chi/chi/v5` | `v5.3.0` | Zero-allocation HTTP request routing |
| **Database** | PostgreSQL | `16` | Authoritative transactional state |
| **DB Driver** | `jackc/pgx/v5` | `v5.10.0` | Native PostgreSQL connection pooling |
| **Queries** | `sqlc` | `v1.28.0` | Type-safe SQL query generation |
| **Migrations** | Goose | `v3` | 68 sequential database migrations |
| **Cache & Queue**| Redis | `7` | Sorted sets for queue, rate limiting, status cache |
| **Web Portal** | Astro | `4.15.0` (Astro 5 compatible) | Server-rendered public portal & interactive islands |
| **Styling** | Tailwind CSS | `3.4.0` | Responsive utility-first design system |
| **Scanner** | Svelte / Vite | `Svelte 5` / `Vite` | Offline-first Progressive Web App (PWA) |
| **Payments** | Duitku & Xendit | REST & HMAC Webhook | Indonesian QRIS, VA, E-Wallets, Cards |
| **Telemetry** | Prometheus | `v1.23.2` | Metrics endpoint (`/metrics`) and war-room stats |

---

## Documentation Navigation

Comprehensive, source-verified documentation is available in the [`docs/`](docs/README.md) directory:

| Section | Target Audience | Key Documents |
| :--- | :--- | :--- |
| **Getting Started** | New Users | [Participant Onboarding](docs/getting-started/user.md) · [Organizer Quickstart](docs/getting-started/organizer.md) · [Admin Onboarding](docs/getting-started/admin.md) |
| **Core Concepts** | All Roles | [Platform Overview](docs/concepts/platform-overview.md) · [Registration Modes](docs/concepts/registration.md) · [Queue Architecture](docs/concepts/queue.md) · [Ballot Engine](docs/concepts/ballot.md) · [Orders & Payments](docs/concepts/orders-and-payments.md) · [HMAC Tickets](docs/concepts/tickets.md) · [BIB Allocation](docs/concepts/bib.md) · [Racepack Expo](docs/concepts/racepack.md) · [Competition Engine](docs/concepts/competition-engine.md) |
| **User Guides** | End-to-End Journeys | [Participant Guide](docs/user-guides/participant.md) · [Organizer Operations](docs/user-guides/organizer.md) · [Admin Operations](docs/user-guides/admin.md) |
| **Architecture** | Engineers & Architects | [System Overview](docs/architecture/overview.md) · [Backend Modules](docs/architecture/backend.md) · [Frontend & Scanner](docs/architecture/frontend.md) · [Database Schema](docs/architecture/database.md) · [Redis Keyspaces](docs/architecture/redis.md) · [Worker Daemons](docs/architecture/workers.md) |
| **Workflows** | Technical Integrators | [Direct Registration](docs/workflows/direct-registration.md) · [Queue Flow](docs/workflows/queue-registration.md) · [Ballot Flow](docs/workflows/ballot-registration.md) · [Checkout Flow](docs/workflows/checkout.md) · [Payment Callback](docs/workflows/payment-webhook.md) · [BIB Allocation Flow](docs/workflows/bib.md) · [Results Workflow](docs/workflows/results.md) |
| **Reference** | Developers | [Permissions Matrix](docs/reference/permissions.md) · [State Machines](docs/reference/state-machines.md) · [REST API Reference](docs/reference/api.md) · [Error Codes Catalog](docs/reference/errors.md) · [Configuration Reference](docs/reference/configuration.md) |
| **Operations** | SREs & Operators | [War-Day Runbook](docs/operations/runbook.md) · [Troubleshooting Guide](docs/operations/troubleshooting.md) · [Backups & Recovery](docs/operations/backups.md) · [Database Migrations](docs/operations/migrations.md) |
| **Development** | Contributors | [Local Setup](docs/development/setup.md) · [Testing & Verification](docs/development/testing.md) · [Contributing Guide](docs/development/contributing.md) · [Debugging & Tracing](docs/development/debugging.md) |
| **Security** | Security Auditors | [Security Model & Threat Mitigation](docs/security/security-model.md) |

---

## Quick Start (Developer Setup)

Follow these steps to launch a local development environment. For detailed requirements, see the [Local Development Setup Guide](docs/development/setup.md).

### 1. Clone Repository & Start Infrastructure
```bash
git clone https://github.com/kaivyy/ivyticketing.git
cd ivyticketing

# Start PostgreSQL 16 and Redis 7 containers
docker compose up -d postgres redis
```

### 2. Apply Database Migrations
```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/ivyticketing?sslmode=disable"
goose -dir database/migrations postgres "$DATABASE_URL" up
```

### 3. Configure Environment
Copy `.env.example` to `.env` and provide the required cryptographic secrets:
```bash
cp .env.example .env
# Required: JWT_SECRET, TICKET_QR_SECRET, DATABASE_URL, REDIS_URL
```

### 4. Run Services
In separate terminal sessions:
```bash
# Terminal 1: Go REST API Server (:8080)
cd services/api && go run cmd/api/main.go

# Terminal 2: Background Worker Daemon
cd services/api && go run cmd/worker/main.go

# Terminal 3: Astro Web Portal (:4321)
cd apps/web && pnpm install && pnpm dev

# Terminal 4: Svelte Scanner PWA (:5173)
cd apps/scanner && pnpm install && pnpm dev
```

---

## Project Structure

```
ivyticketing/
├── services/api/              Go modular monolith backend
│   ├── cmd/api/               Main Chi HTTP REST server (:8080)
│   ├── cmd/worker/            Background sweep & queue release daemon
│   ├── internal/app/          Server assembly, router configuration, config loader
│   ├── internal/modules/      25 domain modules (auth, orders, queue, ballot, etc.)
│   └── internal/platform/     Platform middleware, security, rate limiters, storage
├── apps/web/                  Astro 5 web application (public site, participant, organizer)
├── apps/scanner/              Svelte 5 offline-first PWA scanner for race expo & entry gates
├── database/
│   ├── migrations/            68 sequential Goose SQL migrations
│   └── queries/               sqlc query definition files
├── docs/                      Complete, source-verified documentation system
├── ops/                       k6 load testing scenarios & observability alerts
└── scripts/                   Local setup and verification scripts
```

---

## Testing & Quality Assurance

IvyTicketing maintains extensive automated test suites across all domain modules:

```bash
# Run all backend unit and integration tests (35+ test suites)
cd services/api && go test ./internal/modules/...

# Run with Go race detector
cd services/api && go test -race ./internal/modules/...

# Run high-concurrency overselling prevention test
cd services/api && go test -v -run TestConcurrentCheckoutOversellProtection ./internal/modules/orders/...

# Run payment vs winner-expiration race test
cd services/api && go test -v -run TestPaymentVsWinnerExpirationRace ./internal/modules/ballot/...

# Run TypeScript typechecks (Zero 'any' allowed)
cd apps/web && pnpm check
cd apps/scanner && pnpm check
```

For detailed test scenarios and benchmarks, see the [Testing and Verification Guide](docs/development/testing.md).

---

## Security Model

IvyTicketing enforces defense-in-depth across every layer:
- **Authentication**: HMAC-SHA256 JWT access tokens paired with secure `HttpOnly` refresh cookies.
- **Authorization & BOLA Defense**: Strict multi-tenant RBAC with 38 permissions; route parameter rewrites and database ownership checks guarantee tenant isolation.
- **Cryptographic Tickets**: HMAC-SHA256 signed QR payloads verified offline without leaking master secrets.
- **Payment Verification**: Dual checksum verification (Duitku MD5/SHA256 and Xendit token headers) with SHA-256 payload deduplication.
- **Anti-Bot Engine**: Cloudflare Turnstile integration, IP reputation penalty scoring, and sliding-window rate limiters.
- **Auditability**: Append-only PostgreSQL `audit_logs` table records all sensitive administrative transactions.

For complete architectural details, see the [Security Architecture Document](docs/security/security-model.md).

---

## Release Information & Changelog

All notable changes, security remediations, and architectural evolutions are documented in [CHANGELOG.md](CHANGELOG.md).

- **Current Repository Status**: Feature complete across all planned masterplan phases, including the generic multi-sport competition engine, race results timing bridge, security/transaction remediations, and comprehensive documentation system.
- **Release Versioning**: Semantic Versioning (SemVer) guidelines.

---

## License

This project is licensed under the terms of the [MIT License](LICENSE).
