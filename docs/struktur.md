Repository Structure — Race Registration Platform
Recommended Stack
Frontend:
Astro
TypeScript
Tailwind CSS
PWA support for scanner
Backend:
Go
Chi / stdlib router
PostgreSQL
Redis / DragonflyDB
SQLC
Goose / Atlas migrations
Infra:
Docker
Cloudflare
R2
Nginx optional
Prometheus / Grafana / Loki / Sentry

1. Root Structure
race-platform/
├── apps/
│   ├── web/                    # Public website + participant UI, Astro
│   ├── organizer-dashboard/    # EO dashboard, Astro
│   ├── admin-dashboard/        # Super admin dashboard, Astro
│   └── scanner/                # Racepack/check-in scanner PWA
│
├── services/
│   ├── api/                    # Main Go API
│   ├── worker/                 # Background jobs
│   ├── queue/                  # Queue service / Durable Object adapter
│   └── webhook/                # Payment webhook receiver
│
├── packages/
│   ├── ui/                     # Shared UI components
│   ├── config/                 # Shared config
│   ├── types/                  # Shared TS types
│   ├── validators/             # Shared validation schemas
│   └── sdk/                    # Client SDK for frontend
│
├── database/
│   ├── migrations/
│   ├── queries/
│   ├── seeds/
│   └── schema/
│
├── infra/
│   ├── docker/
│   ├── compose/
│   ├── cloudflare/
│   ├── k8s/
│   ├── terraform/
│   └── monitoring/
│
├── docs/
│   ├── product/
│   ├── architecture/
│   ├── api/
│   ├── database/
│   ├── runbooks/
│   ├── security/
│   ├── payment/
│   ├── queue/
│   ├── racepack/
│   └── decisions/
│
├── scripts/
│   ├── dev/
│   ├── db/
│   ├── deploy/
│   ├── loadtest/
│   └── maintenance/
│
├── tests/
│   ├── e2e/
│   ├── load/
│   ├── contract/
│   └── security/
│
├── .github/
│   └── workflows/
│
├── .env.example
├── docker-compose.yml
├── Makefile
├── README.md
├── CONTRIBUTING.md
├── SECURITY.md
└── CHANGELOG.md


2. Frontend Apps
apps/web
Untuk:
Landing event
Registration
Queue page
Ballot
Checkout
Participant dashboard
apps/web/
├── public/
├── src/
│   ├── assets/
│   ├── components/
│   │   ├── event/
│   │   ├── queue/
│   │   ├── checkout/
│   │   ├── ticket/
│   │   └── common/
│   │
│   ├── layouts/
│   │   ├── PublicLayout.astro
│   │   ├── EventLayout.astro
│   │   └── ParticipantLayout.astro
│   │
│   ├── pages/
│   │   ├── index.astro
│   │   ├── events/
│   │   │   └── [slug]/
│   │   │       ├── index.astro
│   │   │       ├── register.astro
│   │   │       ├── queue.astro
│   │   │       ├── checkout.astro
│   │   │       └── success.astro
│   │   ├── participant/
│   │   │   ├── dashboard.astro
│   │   │   ├── orders.astro
│   │   │   └── tickets.astro
│   │   └── status.astro
│   │
│   ├── lib/
│   │   ├── api.ts
│   │   ├── auth.ts
│   │   ├── queue.ts
│   │   ├── payment.ts
│   │   └── error.ts
│   │
│   ├── stores/
│   ├── styles/
│   └── middleware.ts
│
├── astro.config.mjs
├── package.json
└── README.md


apps/organizer-dashboard
Untuk EO.
apps/organizer-dashboard/
├── src/
│   ├── components/
│   │   ├── dashboard/
│   │   ├── events/
│   │   ├── forms/
│   │   ├── participants/
│   │   ├── payments/
│   │   ├── queue/
│   │   ├── ballot/
│   │   ├── racepack/
│   │   └── reports/
│   │
│   ├── layouts/
│   │   └── DashboardLayout.astro
│   │
│   ├── pages/
│   │   ├── dashboard.astro
│   │   ├── events/
│   │   ├── participants/
│   │   ├── orders/
│   │   ├── payments/
│   │   ├── forms/
│   │   ├── queue/
│   │   ├── ballot/
│   │   ├── racepack/
│   │   ├── broadcast/
│   │   ├── reports/
│   │   └── settings/
│   │
│   ├── lib/
│   └── stores/
│
└── package.json


apps/admin-dashboard
Untuk kamu sebagai platform owner.
apps/admin-dashboard/
├── src/
│   ├── components/
│   │   ├── organizers/
│   │   ├── events/
│   │   ├── billing/
│   │   ├── revenue/
│   │   ├── system/
│   │   └── support/
│   │
│   ├── pages/
│   │   ├── dashboard.astro
│   │   ├── organizers/
│   │   ├── events/
│   │   ├── transactions/
│   │   ├── subscriptions/
│   │   ├── invoices/
│   │   ├── support/
│   │   ├── audit-logs/
│   │   └── system-health/
│   │
│   └── lib/
│
└── package.json


apps/scanner
Untuk racepack dan check-in.
apps/scanner/
├── src/
│   ├── components/
│   │   ├── ScannerCamera.tsx
│   │   ├── ParticipantCard.tsx
│   │   ├── PickupConfirm.tsx
│   │   └── OfflineSyncStatus.tsx
│   │
│   ├── pages/
│   │   ├── login.astro
│   │   ├── scanner.astro
│   │   ├── racepack.astro
│   │   ├── checkin.astro
│   │   └── offline.astro
│   │
│   ├── lib/
│   │   ├── scanner.ts
│   │   ├── offline-db.ts
│   │   ├── sync.ts
│   │   └── qr.ts
│   │
│   └── pwa/
│
└── package.json


3. Backend Go API
services/api
services/api/
├── cmd/
│   └── api/
│       └── main.go
│
├── internal/
│   ├── app/
│   │   ├── bootstrap.go
│   │   ├── config.go
│   │   └── server.go
│   │
│   ├── modules/
│   │   ├── auth/
│   │   ├── organizations/
│   │   ├── users/
│   │   ├── events/
│   │   ├── categories/
│   │   ├── registration/
│   │   ├── forms/
│   │   ├── queue/
│   │   ├── ballot/
│   │   ├── orders/
│   │   ├── inventory/
│   │   ├── payments/
│   │   ├── coupons/
│   │   ├── merchandise/
│   │   ├── bibs/
│   │   ├── tickets/
│   │   ├── racepack/
│   │   ├── scanner/
│   │   ├── notifications/
│   │   ├── reports/
│   │   ├── billing/
│   │   ├── audit/
│   │   └── system/
│   │
│   ├── platform/
│   │   ├── database/
│   │   ├── redis/
│   │   ├── logger/
│   │   ├── middleware/
│   │   ├── validator/
│   │   ├── security/
│   │   ├── idempotency/
│   │   ├── pagination/
│   │   ├── errors/
│   │   └── telemetry/
│   │
│   └── shared/
│       ├── constants/
│       ├── enums/
│       ├── dto/
│       └── utils/
│
├── api/
│   ├── openapi.yaml
│   └── postman.json
│
├── tests/
│   ├── integration/
│   └── unit/
│
├── go.mod
├── go.sum
└── README.md


4. Go Module Pattern
Setiap module backend pakai struktur sama.
Contoh:
internal/modules/orders/
├── handler.go
├── service.go
├── repository.go
├── model.go
├── dto.go
├── validator.go
├── routes.go
├── errors.go
├── events.go
└── tests/
    ├── service_test.go
    └── repository_test.go

Aturan:
handler.go:
HTTP request/response only
service.go:
business logic
repository.go:
database operation
model.go:
domain model
dto.go:
request/response structs
validator.go:
validation
routes.go:
route registration
events.go:
domain events
errors.go:
typed errors

5. Critical Backend Modules
auth
auth/
├── handler.go
├── service.go
├── session.go
├── password.go
├── jwt.go
├── oauth.go
├── rbac.go
└── middleware.go


queue
queue/
├── handler.go
├── service.go
├── repository.go
├── allocator.go
├── token.go
├── reconnect.go
├── throttle.go
├── pause.go
├── status.go
└── events.go

Responsibilities:
Create queue token
Maintain position
Handle reconnect
Release participants gradually
Pause/resume queue
Prevent duplicate queue entries

inventory
inventory/
├── service.go
├── repository.go
├── lock.go
├── stock.go
├── reservation.go
├── expiration.go
└── tests/

Responsibilities:
Atomic stock decrement
Reservation
Release expired slot
Prevent overselling

payments
payments/
├── handler.go
├── service.go
├── repository.go
├── gateway.go
├── duitku.go
├── xendit.go
├── midtrans.go
├── webhook.go
├── reconcile.go
├── refund.go
├── signature.go
├── idempotency.go
└── tests/

Responsibilities:
Create payment
Verify callback
Retry webhook
Payment reconciliation
Fallback gateway

forms
forms/
├── handler.go
├── service.go
├── repository.go
├── schema.go
├── field.go
├── conditional.go
├── validation.go
└── renderer.go

Responsibilities:
Custom form builder
Field validation
Conditional logic
Per-event form schema

bibs
bibs/
├── handler.go
├── service.go
├── repository.go
├── generator.go
├── allocator.go
├── import.go
└── export.go

Responsibilities:
Auto BIB assignment
Manual BIB upload
Prefix per category
Duplicate prevention

racepack
racepack/
├── handler.go
├── service.go
├── repository.go
├── slots.go
├── pickup.go
├── proxy.go
├── scanner.go
├── offline_sync.go
└── audit.go

Responsibilities:
Pickup schedule
Slot quota
QR verification
Proxy pickup
Duplicate pickup prevention

6. Worker Service
services/worker/
├── cmd/
│   └── worker/
│       └── main.go
│
├── internal/
│   ├── jobs/
│   │   ├── expire_orders.go
│   │   ├── release_inventory.go
│   │   ├── retry_webhooks.go
│   │   ├── payment_reconcile.go
│   │   ├── send_email.go
│   │   ├── send_whatsapp.go
│   │   ├── generate_ticket.go
│   │   ├── assign_bib.go
│   │   ├── export_report.go
│   │   └── cleanup_sessions.go
│   │
│   ├── scheduler/
│   ├── queue/
│   └── platform/
│
├── go.mod
└── README.md

Worker jobs:
Expire unpaid orders
Release locked inventory
Retry failed payment callbacks
Reconcile payment
Send notification
Generate PDF ticket
Assign BIB
Export reports

7. Webhook Service
Optional, tapi bagus untuk isolation.
services/webhook/
├── cmd/
│   └── webhook/
│       └── main.go
│
├── internal/
│   ├── duitku/
│   ├── xendit/
│   ├── midtrans/
│   ├── verifier/
│   ├── processor/
│   ├── idempotency/
│   └── audit/
│
└── README.md

Reason:
Payment callback harus ringan, cepat, aman, dan tidak terganggu API utama.

8. Queue Service
Optional jika war scale besar.
services/queue/
├── cmd/
│   └── queue/
│       └── main.go
│
├── internal/
│   ├── waitingroom/
│   ├── durableobject/
│   ├── token/
│   ├── allocator/
│   ├── throttler/
│   ├── fairness/
│   └── metrics/
│
└── README.md


9. Database Folder
database/
├── migrations/
│   ├── 000001_create_organizations.sql
│   ├── 000002_create_users.sql
│   ├── 000003_create_events.sql
│   ├── 000004_create_categories.sql
│   ├── 000005_create_forms.sql
│   ├── 000006_create_orders.sql
│   ├── 000007_create_payments.sql
│   ├── 000008_create_queue_entries.sql
│   ├── 000009_create_bibs.sql
│   ├── 000010_create_tickets.sql
│   ├── 000011_create_racepack.sql
│   └── 000012_create_audit_logs.sql
│
├── queries/
│   ├── organizations.sql
│   ├── users.sql
│   ├── events.sql
│   ├── categories.sql
│   ├── orders.sql
│   ├── payments.sql
│   ├── inventory.sql
│   ├── queue.sql
│   ├── bibs.sql
│   └── racepack.sql
│
├── seeds/
│   ├── local.sql
│   ├── demo_event.sql
│   └── demo_organizer.sql
│
└── schema/
    ├── erd.md
    ├── tables.md
    └── indexes.md


10. Important Database Tables
Core:
organizations
organization_members
users
roles
permissions

events
event_categories
event_settings
event_branding

form_schemas
form_fields
form_submissions

queue_entries
queue_tokens
queue_sessions

orders
order_items
inventory_reservations

payments
payment_attempts
payment_webhooks
refunds

participants
participant_profiles

bibs
tickets
ticket_qr_tokens

racepack_slots
racepack_pickups
racepack_proxy_pickups

coupons
coupon_redemptions

merchandise_items
merchandise_inventory

notifications
notification_templates
broadcasts

audit_logs
system_events


11. Docs Folder
docs/
├── product/
│   ├── PRD.md
│   ├── ROADMAP.md
│   ├── USER_ROLES.md
│   └── FEATURE_MATRIX.md
│
├── architecture/
│   ├── OVERVIEW.md
│   ├── SYSTEM_DESIGN.md
│   ├── MULTI_TENANCY.md
│   ├── SERVICE_BOUNDARIES.md
│   ├── DATA_FLOW.md
│   └── SCALING_STRATEGY.md
│
├── api/
│   ├── OPENAPI.md
│   ├── AUTH.md
│   ├── ERROR_CODES.md
│   ├── IDEMPOTENCY.md
│   └── WEBHOOKS.md
│
├── database/
│   ├── ERD.md
│   ├── MIGRATIONS.md
│   ├── INDEXING.md
│   ├── BACKUP_RESTORE.md
│   └── DATA_RETENTION.md
│
├── queue/
│   ├── WAITING_ROOM.md
│   ├── QUEUE_FAIRNESS.md
│   ├── TOKEN_DESIGN.md
│   ├── RECONNECT.md
│   └── INCIDENT_PLAYBOOK.md
│
├── payment/
│   ├── DUITKU.md
│   ├── XENDIT.md
│   ├── MIDTRANS.md
│   ├── CALLBACK_SECURITY.md
│   ├── RECONCILIATION.md
│   └── REFUND.md
│
├── racepack/
│   ├── BIB_SYSTEM.md
│   ├── QR_TICKET.md
│   ├── PICKUP_SLOTS.md
│   ├── SCANNER.md
│   ├── OFFLINE_MODE.md
│   └── PROXY_PICKUP.md
│
├── security/
│   ├── THREAT_MODEL.md
│   ├── RBAC.md
│   ├── RATE_LIMIT.md
│   ├── BOT_PROTECTION.md
│   ├── PII.md
│   └── AUDIT_LOG.md
│
├── runbooks/
│   ├── WAR_DAY.md
│   ├── PAYMENT_DOWN.md
│   ├── QUEUE_PAUSE.md
│   ├── OVERSOLD_PREVENTION.md
│   ├── DATABASE_FAILOVER.md
│   └── RACEPACK_DAY.md
│
└── decisions/
    ├── ADR-0001-why-go.md
    ├── ADR-0002-why-astro.md
    ├── ADR-0003-why-postgres.md
    ├── ADR-0004-queue-design.md
    └── ADR-0005-payment-isolation.md


12. Infra Folder
infra/
├── docker/
│   ├── api.Dockerfile
│   ├── worker.Dockerfile
│   ├── webhook.Dockerfile
│   └── frontend.Dockerfile
│
├── compose/
│   ├── docker-compose.local.yml
│   ├── docker-compose.staging.yml
│   └── docker-compose.prod.yml
│
├── cloudflare/
│   ├── waf-rules.md
│   ├── turnstile.md
│   ├── waiting-room.md
│   ├── r2.md
│   └── pages.md
│
├── monitoring/
│   ├── prometheus.yml
│   ├── grafana-dashboards/
│   ├── loki.yml
│   └── alert-rules.yml
│
├── k8s/
│   ├── api-deployment.yml
│   ├── worker-deployment.yml
│   ├── webhook-deployment.yml
│   ├── ingress.yml
│   └── hpa.yml
│
└── terraform/
    ├── environments/
    │   ├── staging/
    │   └── production/
    └── modules/


13. Scripts
scripts/
├── dev/
│   ├── start-local.sh
│   ├── reset-local.sh
│   └── seed-demo.sh
│
├── db/
│   ├── migrate-up.sh
│   ├── migrate-down.sh
│   ├── backup.sh
│   └── restore.sh
│
├── deploy/
│   ├── deploy-staging.sh
│   ├── deploy-production.sh
│   └── rollback.sh
│
├── loadtest/
│   ├── queue-war.js
│   ├── checkout-flow.js
│   ├── payment-webhook.js
│   └── racepack-scan.js
│
└── maintenance/
    ├── cleanup-expired-orders.sh
    ├── reconcile-payments.sh
    └── generate-reports.sh


14. Tests
tests/
├── e2e/
│   ├── participant-register.spec.ts
│   ├── queue-war.spec.ts
│   ├── checkout-payment.spec.ts
│   ├── ballot.spec.ts
│   └── racepack-pickup.spec.ts
│
├── load/
│   ├── 100k-waiting-room.js
│   ├── payment-spike.js
│   └── scanner-venue.js
│
├── contract/
│   ├── duitku-webhook.test.ts
│   ├── xendit-webhook.test.ts
│   └── midtrans-webhook.test.ts
│
└── security/
    ├── rate-limit.test.ts
    ├── webhook-signature.test.ts
    └── rbac.test.ts


15. Makefile
dev:
	docker compose -f infra/compose/docker-compose.local.yml up

api:
	cd services/api && go run ./cmd/api

worker:
	cd services/worker && go run ./cmd/worker

web:
	cd apps/web && pnpm dev

migrate-up:
	goose -dir database/migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir database/migrations postgres "$$DATABASE_URL" down

test:
	go test ./...

lint:
	golangci-lint run ./...

loadtest-queue:
	k6 run tests/load/100k-waiting-room.js


16. Naming Convention
Use:
snake_case untuk database
camelCase untuk JSON
PascalCase untuk Go exported struct
kebab-case untuk folder frontend route

Example:
Database:
organization_id
created_at
payment_status

JSON:
{
  "organizationId": "...",
  "paymentStatus": "paid"
}


17. Environment Files
.env.example
.env.local
.env.staging
.env.production

Example:
APP_ENV=local
APP_NAME=RacePlatform

DATABASE_URL=postgres://user:pass@localhost:5432/race_platform
REDIS_URL=redis://localhost:6379

JWT_SECRET=
SESSION_SECRET=

R2_BUCKET=
R2_ACCESS_KEY=
R2_SECRET_KEY=

DUITKU_MERCHANT_CODE=
DUITKU_API_KEY=

XENDIT_SECRET_KEY=
MIDTRANS_SERVER_KEY=

SENTRY_DSN=

Never commit real env.

18. API Versioning
Use:
/api/v1

Example:
GET    /api/v1/events
POST   /api/v1/events/:id/register
POST   /api/v1/queue/join
GET    /api/v1/queue/status
POST   /api/v1/orders
POST   /api/v1/payments
POST   /api/v1/webhooks/duitku
POST   /api/v1/webhooks/xendit
POST   /api/v1/webhooks/midtrans


19. Error Response Standard
{
  "error": {
    "code": "QUEUE_TOKEN_EXPIRED",
    "message": "Queue session expired.",
    "requestId": "req_abc123"
  }
}

Do not leak internal errors.
Bad:
panic: nil pointer
SQL timeout
redis connection refused

Good:
Sistem sedang padat. Posisi antrean kamu tetap aman.


20. Logging Standard
Every log must include:
request_id
organization_id
event_id
user_id
module
action
status
duration_ms

Example:
{
  "requestId": "req_123",
  "eventId": "evt_123",
  "module": "payments",
  "action": "callback_received",
  "status": "success"
}


21. Audit Log
Audit:
Login
Create event
Change price
Change capacity
Pause queue
Resume queue
Manual payment update
Refund
Racepack pickup
Staff permission change

22. Branching Strategy
main        production
staging     staging
develop     active development
feature/*   new features
fix/*       bug fixes
hotfix/*    emergency production fixes


23. Commit Convention
feat(queue): add persistent queue token
fix(payment): prevent duplicate webhook processing
docs(racepack): add scanner offline mode
refactor(api): split order service
test(inventory): add oversell prevention test


24. MVP Development Order
Phase 1 — Foundation
Monorepo
Auth
Organization
Event
Category
Basic registration
PostgreSQL
Redis
Admin dashboard skeleton
Phase 2 — Payment
Orders
Inventory reservation
Duitku integration
Webhook idempotency
Payment expiry worker
Phase 3 — Queue
Queue join
Queue status
Queue token
Queue release
Admin pause/resume
Load test
Phase 4 — Custom Form
Form builder
Form renderer
Conditional fields
Validation
Phase 5 — Racepack
BIB assignment
QR ticket
Pickup slot
Scanner PWA
Duplicate prevention
Phase 6 — SaaS
Organizer billing
White label
Custom domain
Multi gateway
Reporting

25. Non-Negotiable Engineering Rules
No stock update without transaction.
No payment callback without idempotency.
No queue token stored only in browser.
No sensitive data inside QR.
No admin action without audit log.
No raw error shown to user.
No production console noise.
No manual database edit without runbook.
No payment gateway change without test webhook.
No war launch without load test.

26. Ideal First MVP Repo Shape
For first build, do not over-split too early.
Start with:
apps/web
apps/organizer-dashboard
apps/admin-dashboard
apps/scanner

services/api
services/worker

database
docs
infra
tests

Keep backend as modular monolith first.
Split into services later only after traffic proves the need.
Recommended first backend:
Go modular monolith
PostgreSQL
Redis
Cloudflare
Duitku first
Xendit/Midtrans later


27. Final Recommendation
Best maintainable structure:
Astro multi-app frontend
Go modular monolith backend
Separate worker
PostgreSQL as source of truth
Redis/DragonflyDB for queue/cache
Cloudflare for edge/war protection
Docs-first engineering culture

This structure is suitable for:
MVP
SaaS
Enterprise event
100k+ queue traffic
Multi organizer
White label
Long-term maintainability
