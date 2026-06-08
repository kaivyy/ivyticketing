# Phase 7: Participant Dashboard & Ticket Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When an order becomes PAID, atomically issue a QR-bearing ticket in the same transaction; expose participant dashboard endpoints (My Orders/Tickets, ticket+QR, invoice) and an organizer ticket list; build a minimal participant dashboard in `apps/web` — all extending the Phase 1-6 production baseline without changing existing behavior.

**Architecture:** New `tickets` module (HTTP + service + repo + `ticketnum`) plus a `tickets/qr` sub-package (HMAC-SHA256 signed token, no PII). Ticket issuance is wired into the existing payments `Processor.applyPaid` via a small `TicketIssuer` interface declared in `payments` (dependency inversion, same pattern as `AuditRecorder`). The issuer runs on the **same tx-bound querier** as the order→PAID transition (exposed via a new `Querier()` method on the payments `Repository`), so PAID ⟺ ticket exists, and any issue error rolls back the whole transaction. Frontend is Astro in `apps/web` with a minimal auth foundation.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, `crypto/hmac`+`crypto/sha256`, `github.com/google/uuid`. Frontend: Astro 4, Tailwind, TypeScript, client-side `qrcode` lib.

**Reference spec:** `docs/superpowers/specs/2026-06-08-phase7-participant-dashboard-ticket-design.md`

**EXTEND, DON'T REWRITE.** Phase 1-6 is production baseline. Do not change their API, auth, order/payment flow, behavior, or move/rename existing files. Phase 7 = new migrations + new `tickets` module + `tickets/qr` + additive payments seam (interface + one call in `applyPaid` + ctor arg) + additive `server.go` wiring + new `apps/web` pages only.

**Conventions (verified against the existing codebase):**
- Module path: `github.com/varin/ivyticketing/services/api`.
- Module layout mirrors `internal/modules/payments` & `orders`: `Handler`/`NewHandler`, `Service`/`NewService`, `Repository` interface + `sqlcRepo` adapter with `ExecTx(ctx, func(Repository) error)`, `NewRepository(pool *pgxpool.Pool)`.
- Error envelope: `apperr.New(status, code, message)` + `apperr.WriteError`/`apperr.WriteJSON` from `internal/platform/errors`.
- Identity: `authctx.FromContext(ctx) (Identity, bool)`, `Identity{UserID uuid.UUID, IsPlatformAdmin bool}` from `internal/platform/authctx`.
- authz: `middleware.RequirePermission(loader, "ticket.view")` for organizer routes; participant routes use authn only + ownership filter (foreign id mismatch → 404, never 403).
- Audit: `audit.Logger.Record(ctx, audit.Entry{OrganizationID *uuid.UUID, ActorUserID *uuid.UUID, Action, TargetType, TargetID string, Metadata map[string]any})`.
- sqlc generated nullable types use `pgtype.*` (NOT pointers); nullable uuid FK → `*uuid.UUID`; `bigint`→`int64`; `integer`→`int32`; `timestamptz`→`pgtype.Timestamptz`; nullable text→`pgtype.Text`. **Always check generated `internal/db/*.go` before writing service/repo code.**
- Migrations: `database/migrations/NNNNN_*.sql` (goose up/down). Phase 6 ended at `00017`. Phase 7 continues `00018`.
- sqlc queries: `database/queries/*.sql`, one file per module. Run sqlc: `make sqlc` (from repo root).
- `users` table: `id uuid`, `email citext NOT NULL`, `full_name text NOT NULL`. Query `GetUserByID :one SELECT * FROM users WHERE id=$1` exists.
- `orders` has `organization_id, event_id, category_id, participant_id, order_number, status, subtotal, fee, discount, total, expired_at, created_at`. Status values: `DRAFT|PENDING_PAYMENT|PAID|EXPIRED|CANCELLED|REFUNDED`.
- Role template slugs (org_id NULL, is_system): `owner`, `manager`, `finance`, `customer-service`, `racepack-staff`.
- Payments `Processor`: `NewProcessor(repo Repository, recorder AuditRecorder)`; PAID transition in `applyPaid(ctx, tx Repository, webhookID, payment, res)` (`processor.go:174`). Reconcile path calls the same `apply`/`applyPaid` via `Apply`.
- Single test pkg: `cd services/api && go test ./internal/<path>/ -run <Name> -v`. Race: add `-race`. Integration DB: `ivyticketing_test`, truncate per test (see existing `services/api/tests/integration`).

**Env added:**
```
TICKET_QR_SECRET=         # HMAC secret for ticket QR tokens; required (fail-fast), separate from JWT_SECRET
```
Frontend reuses existing `PUBLIC_API_URL` in `apps/web`.

---

## File Structure

```txt
database/
├── migrations/
│   ├── 00018_create_tickets.sql
│   └── 00019_seed_ticket_view.sql
└── queries/
    └── tickets.sql

services/api/
├── internal/
│   ├── app/
│   │   ├── config.go          # MODIFY: add TicketQRSecret (required)
│   │   ├── config_test.go     # MODIFY: add TicketQRSecret tests
│   │   └── server.go          # MODIFY: build qr signer + tickets issuer/handler; inject issuer into NewProcessor; mount routes
│   ├── db/                     # sqlc-regenerated (tickets queries)
│   └── modules/
│       ├── payments/
│       │   ├── repository.go   # MODIFY: add Querier() *db.Queries to interface + sqlcRepo
│       │   └── processor.go    # MODIFY: add TicketIssuer field + ctor arg; call issuer in applyPaid
│       └── tickets/            # NEW module
│           ├── model.go  dto.go  errors.go  ticketnum.go
│           ├── repository.go  issuer.go  service.go  handler.go  routes.go
│           ├── qr/qr.go  qr/qr_test.go
│           └── tests/ issuer_test.go service_test.go
└── tests/integration/          # phase7_ticket_test.go (+ concurrency)

apps/web/src/
├── lib/ auth.ts (NEW)  api.ts (MODIFY)  tickets.ts (NEW)  invoice.ts (NEW)
├── middleware.ts (NEW)
├── layouts/ ParticipantLayout.astro (NEW)
├── pages/ login.astro (NEW)
│   └── participant/ dashboard.astro orders.astro orders/[orderId].astro tickets.astro tickets/[ticketId].astro
└── components/ticket/ TicketCard.astro QrDisplay.astro OrderTimeline.astro InvoiceView.astro

docs/
├── TICKET_FLOW.md  QR_TICKET.md  PARTICIPANT_DASHBOARD.md  PHASE7_DECISIONS.md
└── CHANGELOG.md (MODIFY)
```

---

## Plan Parts

Execute in order — each part assumes the previous parts' code exists. Parts 1-4 are backend (sequential). Part 5 (frontend) depends on Part 4 endpoints. Part 6 is docs + final verification.

1. **[Part 1: Foundation — config, migrations, QR package](2026-06-08-phase7-part1-foundation-qr.md)** (Tasks 1-5)
   Config `TICKET_QR_SECRET`; `tickets` migration + `ticket.view` seed; sqlc queries; `tickets/qr` Sign/Verify.

2. **[Part 2: Tickets module — issuer + service](2026-06-08-phase7-part2-tickets-module.md)** (Tasks 6-10)
   `model/dto/errors/ticketnum`, `repository`, `issuer.IssueForOrder` (idempotent), `service` (get/list/invoice).

3. **[Part 3: Payments seam — atomic issuance](2026-06-08-phase7-part3-payments-seam.md)** (Tasks 11-13)
   Add `Querier()` to payments repo; add `TicketIssuer` interface + ctor arg; call in `applyPaid`; processor tests (idempotency + rollback closes the gap).

4. **[Part 4: HTTP + integration](2026-06-08-phase7-part4-http-integration.md)** (Tasks 14-17)
   `handler` + `routes` (participant + organizer); `server.go` wiring; integration + concurrency tests.

5. **[Part 5: Frontend participant dashboard](2026-06-08-phase7-part5-frontend.md)** (Tasks 18-23)
   Auth foundation (`lib/auth.ts`, `middleware.ts`, login), authed `api.ts`, dashboard/orders/tickets/invoice pages + components, browser verification.

6. **[Part 6: Docs + final verification](2026-06-08-phase7-part6-docs-verify.md)** (Tasks 24-26)
   `TICKET_FLOW/QR_TICKET/PARTICIPANT_DASHBOARD/PHASE7_DECISIONS`; CHANGELOG; full `go test ./... -race` + sqlc/vet green; DoD checklist.
