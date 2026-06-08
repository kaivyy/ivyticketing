# Phase 7 Plan — Part 6: Docs + final verification

> Part of the Phase 7 implementation plan. Index: [2026-06-08-phase7-participant-dashboard-ticket.md](2026-06-08-phase7-participant-dashboard-ticket.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** Docs + final verification only. Assumes Parts 1-5 complete.

Docs mirror the Phase 6 style (sequence diagrams in text, state machine, failure/recovery scenarios).

---

## Task 24: Write Phase 7 docs

**Files:**
- Create: `docs/TICKET_FLOW.md`
- Create: `docs/QR_TICKET.md`
- Create: `docs/PARTICIPANT_DASHBOARD.md`
- Create: `docs/PHASE7_DECISIONS.md`

- [ ] **Step 1: Write docs/TICKET_FLOW.md**

Create `docs/TICKET_FLOW.md` covering:
- End-to-end: order `PENDING_PAYMENT` → PAID callback (Phase 6) → `applyPaid` issues ticket in the **same transaction** → ticket `VALID`.
- Text sequence diagram:
  ```
  Gateway → webhook(8090): POST /webhooks/{gateway}
  webhook → payments.Processor.ProcessRaw: store raw, verify, parse
  Processor → ExecTx BEGIN
    MarkPaymentPaid → UpdateOrderStatus(PAID) → CompleteReservations(COMPLETED)
    TicketIssuer.IssueForOrder(txQuerier, order)  [CreateTicket ON CONFLICT DO NOTHING]
  ExecTx COMMIT  (or ROLLBACK on any error → nothing persisted)
  ```
- Atomicity guarantee: **PAID ⟺ ticket exists**. Issuer error → full rollback (order stays PENDING_PAYMENT, payment stays PENDING). State this is verified by `TestApplyPaid_IssuerError_RollsBack`.
- Idempotency: duplicate callbacks → one ticket (UNIQUE order_id + ON CONFLICT). Cross-references `WEBHOOK_PROCESSING.md` (Phase 6).
- Ticket state machine: `VALID → USED` (Phase 15 scan), `VALID → CANCELLED` (refund, future). Only `VALID` is produced in Phase 7.

- [ ] **Step 2: Write docs/QR_TICKET.md**

Create `docs/QR_TICKET.md` covering:
- Token format: `<version>.<base64url(payload)>.<base64url(hmac_sha256)>`.
- Payload: `{ "tid", "eid", "v" }` — only UUIDs + version, **no PII** (cites non-negotiable rule "No sensitive data inside QR").
- Signing: HMAC-SHA256 with `TICKET_QR_SECRET` (separate from `JWT_SECRET`); why separation matters (blast-radius isolation).
- Stateless verification: `qr.Verify` checks signature without DB; status (VALID/USED) lookup happens at scan time (Phase 15).
- Versioning: `qr_version` column + payload `v` enable secret/schema rotation without invalidating old tickets.
- Why no `exp`: ticket validity is event-bound and enforced by DB status at scan, not token expiry.
- Verify/scan endpoint is **out of scope for Phase 7** (Phase 15 Scanner PWA will consume `qr.Verify`).

- [ ] **Step 3: Write docs/PARTICIPANT_DASHBOARD.md**

Create `docs/PARTICIPANT_DASHBOARD.md` covering:
- Endpoint table (participant + organizer) from the spec.
- Ownership model: participant resources filtered by `participant_id = caller`; mismatch → 404 (not 403).
- Invoice gating: only `PAID` orders; `INVOICE_NOT_AVAILABLE` otherwise. JSON now, PDF deferred (print via browser).
- Order timeline derivation (no new table; from timestamps).
- Frontend: `apps/web` participant pages, minimal auth foundation (sessionStorage access token + HttpOnly refresh cookie + one-shot refresh-on-401), client-side QR rendering.

- [ ] **Step 4: Write docs/PHASE7_DECISIONS.md**

Create `docs/PHASE7_DECISIONS.md` (Phase 6 decisions style) covering, with Why/Tradeoff each:
- **Why synchronous issuance in the processor** (atomic PAID⟺ticket; vs async worker which risks PAID-without-ticket + needs reconcile).
- **Why the `TicketIssuer` interface in payments** (dependency inversion, payments doesn't import tickets; same pattern as `AuditRecorder`; the tx-querier seam via `Repository.Querier()` keeps it atomic).
- **Why signed HMAC QR vs opaque DB token** (stateless verify, no per-scan lookup, no PII; tradeoff: per-token revocation needs a status check at scan — acceptable since scan already hits DB in Phase 15).
- **Why separate `TICKET_QR_SECRET`** (blast-radius isolation from auth JWT).
- **Why JSON invoice, PDF deferred** (avoids heavy PDF deps/worker; browser print covers MVP).
- **Why verify/scan deferred to Phase 15** (Phase 7 scope is participant-facing; scan belongs with Scanner PWA).
- **Frontend access-token-in-sessionStorage tradeoff** (XSS exposure vs SSR-cookie complexity; refresh token stays HttpOnly).
- **`CANCELLED` status reserved** (forward-compat for refund, not used yet — mirrors Phase 6 `REFUNDED`).

- [ ] **Step 5: Commit**

```bash
git add docs/TICKET_FLOW.md docs/QR_TICKET.md docs/PARTICIPANT_DASHBOARD.md docs/PHASE7_DECISIONS.md
git commit -m "docs(phase7): ticket flow, QR, participant dashboard, decisions"
```

---

## Task 25: Update CHANGELOG

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add a Phase 7 entry**

Prepend a Phase 7 section to `CHANGELOG.md` following the existing format (check how Phase 6 was recorded and match it). Include:
- `tickets` module + `tickets/qr` HMAC signer.
- Atomic ticket issuance wired into payments `applyPaid` via `TicketIssuer` (PAID⟺ticket, rollback-safe).
- Migrations `00018_create_tickets`, `00019_seed_ticket_view`.
- Participant endpoints (tickets, ticket+qr, order ticket, invoice) + organizer event ticket list (`ticket.view`).
- `apps/web` participant dashboard + minimal auth foundation.
- New env `TICKET_QR_SECRET` (required).
- Deferred: QR verify/scan (Phase 15), PDF, refund.

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(phase7): update CHANGELOG"
```

---

## Task 26: Final verification — full suite + DoD checklist

**Files:** none (verification).

- [ ] **Step 1: Regenerate sqlc + vet + build**

Run:
```bash
make sqlc
cd services/api && go vet ./... && go build ./...; cd ../..
```
Expected: sqlc clean (no diff beyond committed), vet clean, build clean.

- [ ] **Step 2: Run the full Go test suite (unit + race)**

Run:
```bash
cd services/api && go test ./... -race; cd ../..
```
Expected: PASS (all packages). Confirms Phase 1-6 tests still green (no behavior change) + Phase 7 unit/processor tests.

- [ ] **Step 3: Run integration + concurrency (requires test DB)**

Run:
```bash
cd services/api && go test -tags=integration -race ./tests/integration/ -run TestPhase7 -v; cd ../..
```
Expected: PASS — full PAID→ticket, duplicate→one ticket, ownership 404, invoice gating, organizer perms, concurrency (one ticket under 50 concurrent PAID).

- [ ] **Step 4: Migration roundtrip**

Run:
```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: `00018`/`00019` up/down/up clean.

- [ ] **Step 5: Frontend build**

Run:
```bash
cd apps/web && npm run build; cd ../..
```
Expected: build succeeds.

- [ ] **Step 6: Walk the Definition of Done**

Verify each item from the spec's Definition of Done section is satisfied. Tick them off:
1. Migration `tickets` roundtrip + `ticket.view` seed idempotent. ✅/❌
2. Order→PAID issues ticket atomically (same tx); PAID⟺ticket; idempotent. ✅/❌
3. Rollback: issuer error → order & payment NOT PAID (tested). ✅/❌
4. QR signed HMAC; no PII; Sign/Verify tested; invalid/tampered/wrong-version rejected. ✅/❌
5. Participant endpoints with ownership 404. ✅/❌
6. Organizer event ticket list requires `ticket.view`; super admin passes. ✅/❌
7. Frontend dashboard + auth foundation; QR renders; invoice printable; browser-verified. ✅/❌
8. Audit `TICKET_ISSUED` recorded. ✅/❌
9. `go test ./... -race` + integration green. ✅/❌
10. `sqlc generate` clean; all queries via sqlc; `go vet` clean. ✅/❌
11. Fail-fast on missing `TICKET_QR_SECRET`; no secret hardcoded/logged. ✅/❌
12. No Phase 1-6 behavior/API change (extend-only); docs + CHANGELOG updated. ✅/❌

For any ❌, fix before declaring done.

- [ ] **Step 7: Finishing the branch**

Invoke the `superpowers:finishing-a-development-branch` skill to decide merge/PR/cleanup.

---

Part 6 complete. **Phase 7 done** when the DoD checklist is all green.
