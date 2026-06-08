# Phase 7 Design Decisions

## 1. Synchronous Ticket Issuance in the Payment Processor

**Decision:** `IssueForOrder` is called inside `applyPaid`'s database transaction,
not in a separate async step or background worker.

**Why:** The core invariant is PAID ⟺ ticket exists. Issuing synchronously inside
the same transaction makes this atomic — either both the payment-PAID transition and
the ticket row commit, or neither does. There is no window where an order is PAID
but has no ticket.

**Tradeoff:** An async worker (e.g., poll for PAID orders, issue tickets separately)
would decouple the two steps but introduce a gap: the participant's order is PAID but
their ticket does not exist yet. During that gap, `GET /tickets` returns nothing, the
dashboard is confusing, and any worker failure leaves permanently ticketless orders
that require manual remediation. The synchronous approach eliminates the gap entirely
at the cost of slightly longer callback processing time, which is acceptable given
that callbacks are backend-to-backend with no human waiting on the response.

---

## 2. TicketIssuer Interface in the Payments Package

**Decision:** `payments.Processor` depends on a `TicketIssuer` interface rather than
importing the `tickets` package directly.

**Why:** A direct import would create a package dependency cycle (`payments` → `tickets`
→ potentially `payments`). The interface inverts the dependency: `payments` defines
what it needs, `tickets` satisfies it, and `main.go` wires the concrete type. This is
the same pattern used by `AuditRecorder` in Phase 6.

The key design detail is that `IssueForOrder` accepts a `db.Querier` parameter (the
transaction-scoped querier from `ExecTx`) rather than opening its own connection. This
is the seam that keeps the ticket `INSERT` inside the enclosing transaction without
`payments` needing to know anything about how `tickets` accesses the database.

**Tradeoff:** The interface is defined in `payments`, which means `payments` owns the
contract. If `tickets` needs to evolve the signature, it is a cross-package change.
This is a minor friction point accepted in exchange for a clean dependency graph.

---

## 3. Signed HMAC QR Token vs. Opaque Database Token

**Decision:** QR tokens are HMAC-SHA256-signed bearer tokens (`v.payload.sig`) rather
than opaque random strings looked up in a database table.

**Why:**

- **Stateless verification.** The scanner can verify a token's authenticity with a
  pure CPU operation (recompute HMAC, compare). No database read is required to
  confirm the token is genuine. The DB is only consulted to check live status
  (`VALID` / `USED` / `CANCELLED`) after the signature is confirmed valid.

- **No PII in the token.** The payload contains only UUIDs and a version. A
  photographed or leaked QR code reveals nothing about the ticket holder.

- **No per-token storage.** There is no `qr_tokens` table to maintain, expire, or
  rotate. The token is derived from the ticket row on demand.

**Tradeoff:** Per-token revocation (invalidating a specific token without cancelling
the ticket) is not possible — the token is valid as long as the HMAC verifies. Status
enforcement happens at scan time via the DB `status` column (Phase 15). This is
sufficient for the event-entry use case: a ticket is either VALID, USED, or
CANCELLED, and the scanner always checks DB status after verifying the signature.

---

## 4. Separate TICKET_QR_SECRET

**Decision:** QR tokens are signed with `TICKET_QR_SECRET`, a separate environment
variable from the auth `JWT_SECRET`.

**Why:** Blast-radius isolation. A compromised `JWT_SECRET` lets an attacker forge
session tokens (impersonate any user). A compromised `TICKET_QR_SECRET` lets an
attacker forge QR tokens (create fake entry credentials for events). These are
different threat surfaces with different consequences and different remediation
procedures. Separate keys mean a breach of one does not automatically compromise the
other.

**Tradeoff:** One more required environment variable to manage. This is a deliberate
operational cost accepted for the security benefit. Both secrets are required at
startup; the application fails fast if either is missing.

---

## 5. JSON Invoice, PDF Deferred

**Decision:** `GET /orders/{orderId}/invoice` returns structured JSON. PDF generation
is not implemented. The browser's `@media print` CSS handles print formatting.

**Why:** PDF generation in Go requires either a heavy CGo dependency (wkhtmltopdf,
chromium headless) or a pure-Go library with limited layout fidelity. Either option
adds significant build complexity, binary size, and operational overhead (headless
browser process, memory spikes under load) for an MVP feature.

Browser print is a simpler and universally available alternative. The invoice page
is styled for both screen and print via CSS. The participant opens the page and
presses `Ctrl+P` — no server-side work required.

**Tradeoff:** The printed output depends on the browser's print renderer, which
varies slightly across browsers. A pixel-perfect PDF (e.g., for official tax
receipts) will require server-side generation in a future phase. The JSON response
format is already the right shape for that future renderer.

---

## 6. QR Verify/Scan Deferred to Phase 15

**Decision:** Phase 7 generates and displays QR tokens. The scan/verify endpoint
is not implemented until Phase 15.

**Why:** Phase 7 is participant-facing: it gives ticket holders access to their
credentials. Scanning is an organizer/staff operation performed at the event venue,
associated with a Scanner PWA and a separate operational workflow. Implementing the
scan endpoint in Phase 7 would mean building and testing an organizer-facing surface
with no frontend or operational context, creating dead code that sits untested in
production.

**Tradeoff:** Tickets cannot be scanned until Phase 15. This is acceptable because
Phase 7 events are not yet in the "day of event" operational window. The `USED`
status is reserved in the schema so no migration is needed when Phase 15 lands.

---

## 7. Frontend Access Token in sessionStorage

**Decision:** The `apps/web` frontend stores the access token in `sessionStorage`
rather than a cookie or `localStorage`.

**Why:** `sessionStorage` is scoped to the browser tab and is cleared when the tab
closes, limiting the exposure window compared to `localStorage` (persists
indefinitely). The refresh token, which has a 7-day TTL and higher sensitivity,
stays in an HttpOnly cookie where JavaScript cannot read it at all.

**Tradeoff:** `sessionStorage` is still readable by JavaScript running in the page,
which means an XSS vulnerability can exfiltrate the access token. The alternative —
storing the access token in a second HttpOnly cookie — would require the API to
accept cookie-based auth in addition to Bearer tokens, adding complexity to the auth
middleware and the CSRF threat model. For the MVP participant dashboard, the
sessionStorage approach is a pragmatic tradeoff that avoids SSR complexity while
keeping the high-value refresh token out of JavaScript reach.

---

## 8. CANCELLED Status Reserved

**Decision:** The `tickets` table includes a `CANCELLED` status in the enum even
though no cancellation or refund flow is implemented in Phase 7.

**Why:** Forward compatibility. When ticket cancellation is implemented (as part of
a refund flow in a future phase), existing rows can transition to `CANCELLED` without
a schema migration. Code that switches on ticket status will hit the default case
rather than failing to compile. This mirrors the pattern used in Phase 6 for the
`REFUNDED` payment status.

**Tradeoff:** The status exists in the schema but is unreachable through the API in
Phase 7. This is an intentional reservation, not dead code — it signals to future
implementors that the transition path is expected and the enum was planned with it
in mind.
