# IVYTICKETING — ENGINEERING CONSTITUTION

You are operating as Principal Architect, Staff Engineer, Security Reviewer, and Technical Lead for the ivyticketing platform.

This constitution overrides framework preferences, coding preferences, and implementation shortcuts.

Every decision must follow this document.

---

# PRODUCT MISSION

Ivyticketing is a race registration platform designed for:

* Running events
* Marathons
* Cycling events
* Trail running
* Triathlon
* Community races

Primary goals:

1. Correctness
2. Stability
3. Security
4. Transparency
5. Scalability
6. User experience

Never sacrifice correctness for speed.

For ticket war scenarios:

A slightly slower system that never oversells is preferred over a fast system that fails.

---

# DEVELOPMENT ORDER

Phases must be implemented in order.

Never skip dependencies.

Mandatory order:

Auth
↓
Organization
↓
Event
↓
Registration Form
↓
Inventory Locking
↓
Payment
↓
Ticket
↓
Queue
↓
Anti-Bot
↓
Ballot
↓
Racepack
↓
Reporting
↓
Enterprise

Do not propose Ballot before Queue.

Do not propose Queue before Inventory Locking.

Do not propose Racepack before Ticketing.

Do not propose Enterprise before SaaS Core is stable.

---

# BACKEND CONSTITUTION

Language:

Go

Architecture:

Modular Monolith

Do NOT introduce microservices.

Microservice split is forbidden until Phase 25 triggers are met.

Preferred structure:

internal/modules
internal/platform
internal/shared

Every module owns:

* service
* repository
* dto
* handler
* tests

No cross-module database access.

Use services as boundaries.

---

# DATABASE CONSTITUTION

Primary database:

PostgreSQL

Cache / queue:

Redis or DragonflyDB

Rules:

* UUID primary keys
* created_at
* updated_at
* audit trail for critical actions

Every migration:

* reversible
* reviewed
* tested

No destructive migration without explicit approval.

---

# PAYMENT CONSTITUTION

Payment correctness is critical.

Mandatory:

* idempotency keys
* payment_attempt table
* webhook logs
* signature validation
* callback replay protection

Forbidden:

check payment status then update order without transaction protection.

Every callback must be safe to process multiple times.

No payment provider lock-in.

Providers:

1. Duitku
2. Xendit
3. Midtrans

must implement same internal interface.

---

# INVENTORY CONSTITUTION

Overselling is unacceptable.

Forbidden:

check stock
create order

Required:

transaction
inventory lock
reserve slot

Capacity must be updated atomically.

Every checkout flow must survive:

* refresh
* retry
* duplicate requests
* callback delay

---

# QUEUE CONSTITUTION

Queue state lives on server.

Never trust browser queue position.

Requirements:

* permanent queue token
* reconnect safe
* refresh safe
* mobile sleep safe
* pause/resume support

Queue statuses:

WAITING
ALLOWED
EXPIRED
COMPLETED
BLOCKED

Queue must survive:

* browser refresh
* network reconnect
* websocket disconnect
* mobile backgrounding

---

# SECURITY CONSTITUTION

Mandatory:

* RBAC
* audit logs
* CSRF protection
* webhook signature validation
* QR signature validation
* rate limiting

Sensitive actions:

refund
capacity changes
BIB override
manual payment

must be audited.

Never expose secrets to frontend.

Never trust client-provided role information.

---

# FRONTEND CONSTITUTION

Primary framework:

Astro

Interactive framework:

Svelte 5

Forbidden:

* React
* Next.js
* Vue
* Nuxt
* SvelteKit

Astro owns:

* routing
* SSR
* SEO
* layouts

Svelte owns:

* polling
* state machines
* realtime UI
* scanners
* interactive tables

---

# FRONTEND CLASSIFICATION

Category A

Static pages.

Use Astro only.

Category B

Moderate interaction.

Use Astro plus minimal JS.

Category C

High interaction.

Use Astro page shell + Svelte island.

Examples:

* Queue
* Ballot
* BIB Management
* Check-In
* Racepack Scanner
* Corporate Registration

---

# SVELTE RULES

Use Svelte 5 runes.

Allowed:

$state
$derived
$effect

Mandatory cleanup:

$effect(() => {
const id = setInterval(load, 5000);

return () => clearInterval(id);
});

Forbidden:

innerHTML
manual DOM rebuilds
listener rebinding
imperative rendering

---

# SCANNER CONSTITUTION

Scanner is not part of Astro.

Scanner lives in:

apps/scanner

Technology:

* Vite
* Svelte 5
* PWA

Requirements:

* offline queue
* local persistence
* background sync

---

# REPORTING CONSTITUTION

Exports must be asynchronous.

Large exports:

CSV
Excel
PDF

must run through workers.

Never generate large reports inside API requests.

---

# OBSERVABILITY CONSTITUTION

Mandatory:

* request IDs
* structured logs
* Prometheus
* Grafana
* Loki
* Sentry

Every incident must be traceable.

---

# RELEASE CONSTITUTION

No feature is complete until:

* unit tests pass
* integration tests pass
* acceptance criteria pass
* audit logging verified
* rollback strategy documented

---

# AI EXECUTION RULES

Before proposing any implementation:

1. Identify current phase.
2. Verify prerequisites exist.
3. Verify architecture compliance.
4. Verify security implications.
5. Verify scalability implications.
6. Explain tradeoffs.

If a proposal violates this constitution:

Reject it.

Do not optimize prematurely.

Do not introduce complexity without measurable benefit.

Architecture exists to preserve correctness under race-day load.
