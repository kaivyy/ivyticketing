# Backend Architecture and Go Modules

The IvyTicketing backend is implemented in Go 1.22+ and organized into modular domain packages within [`services/api/internal/modules/`](file:///root/ivyticketing/services/api/internal/modules/).

---

## 1. Package Design Pattern

Each domain module adheres to a strict three-tier architecture:

```
module/
├── handler.go     # HTTP transport: Chi route registration, JSON decoding, status envelopes
├── service.go     # Domain logic: Business invariants, validations, cross-module orchestration
├── repository.go  # Persistence: SQL queries, pgxpool transactions, database mapping
└── (types.go)     # Domain entities, DTOs, and interface definitions
```

### Invariants

- **Handlers never touch the database**: Handlers only decode HTTP requests, validate input schemas, delegate to the service layer, and encode JSON response envelopes.
- **Services own transactions**: When a business operation spans multiple tables (e.g. checkout reserving inventory and consuming a grant), the service opens a `pgx.Tx` transaction and commits or rolls back atomically.
- **Repositories are mockable**: Repository methods are defined as Go interfaces, enabling fast, isolated unit and integration testing without database dependencies where needed.

---

## 2. Inventory of Domain Modules

| Module | Primary Responsibility | Key Files |
| :--- | :--- | :--- |
| `abuse` | Bot detection, Cloudflare Turnstile verification, IP reputation scores, and CIDR blocklists | [`guard.go`](file:///root/ivyticketing/services/api/internal/modules/abuse/guard.go) |
| `access` | Priority access codes, corporate quotas, eligibility rules, and single-use access grants | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/access/service.go) |
| `auth` | User account registration, bcrypt password hashing, and HMAC JWT token issuance | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/auth/service.go) |
| `ballot` | Lottery draws, CSPRNG winner randomization, 48h payment windows, and winner expiration | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/ballot/service.go) |
| `billing` | Platform subscription packages, monthly invoices, and per-order fee ledgers | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/billing/service.go) |
| `categories` | Ticket categories, pricing, hard quotas, and age-based eligibility constraints | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/categories/service.go) |
| `competitions` | Generic multi-sport tournament engine: stages, matches, heats, rosters, and scoring | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/competitions/service.go) |
| `enterprise` | Developer API keys, outbound webhook dispatching, and public read APIs | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/enterprise/service.go) |
| `events` | Event metadata, venues, dates, banner uploads, and publication state machines | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/events/service.go) |
| `forms` | Dynamic participant registration forms, custom fields, and validation logic | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/forms/service.go) |
| `members` | Organization team management, staff invitations, and role assignments | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/members/service.go) |
| `notifications` | Transactional email (SMTP) and SMS delivery, templates, and exponential retry queue | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/notifications/service.go) |
| `orders` | Cart initialization, checkout, 15-minute inventory holds, and order lifecycle | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/orders/service.go) |
| `payments` | Gateway driver registry (Duitku / Xendit), callback verification, and reconciliation | [`processor.go`](file:///root/ivyticketing/services/api/internal/modules/payments/processor.go) |
| `queue` | High-traffic war queue, Redis sorted set store, status caching, and rate limiters | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/queue/service.go) |
| `racepack` | Expo pickup slots, physical counter allocations, proxy authorization, and problem desk | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/racepack/service.go) |
| `registration` | Gate keeper enforcing the 8 registration modes and admission token requirements | [`gate.go`](file:///root/ivyticketing/services/api/internal/modules/registration/gate.go) |
| `reporting` | Financial summaries, registration velocity charts, and asynchronous CSV export jobs | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/reporting/service.go) |
| `results` | Official chip/gun timing CSV imports, category ranking calculations, and certificate rendering | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/results/service.go) |
| `roles` | RBAC role definitions and 38 platform permissions catalog | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/roles/service.go) |
| `scanner` | Mobile gate check-in, offline HMAC-SHA256 signature verification, and check-in audit | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/scanner/service.go) |
| `status` | Public status page, system component health, and incident updates | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/status/service.go) |
| `tickets` | Ticket issuance, cryptographic HMAC QR code signing, and sequential BIB allocations | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/service.go) |
| `waitlist` | Event standby pools, FIFO waitlist rankings, and automated promotion triggers | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/waitlist/service.go) |
| `whitelabel` | Custom branding, theme colors, logos, and DNS TXT custom domain verification | [`service.go`](file:///root/ivyticketing/services/api/internal/modules/whitelabel/service.go) |

---

## 3. Database Transaction Boundaries

All critical business transactions are executed using `pgxpool.Pool` transactions to ensure consistency:

```go
tx, err := pool.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx)

// 1. Validate admission grant
// 2. Lock inventory row: SELECT ... FOR UPDATE
// 3. Decrement available quota, increment reserved quota
// 4. Insert order with PENDING_PAYMENT
// 5. Consume access grant
// 6. Record audit log entry

if err := tx.Commit(ctx); err != nil {
    return err
}
```

This strict transaction boundary prevents partial state corruptions during network timeouts or concurrent race conditions.
