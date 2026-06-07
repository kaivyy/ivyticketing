# Phase 5: Architecture Decisions & Tradeoffs

This document records the key design decisions made during Phase 5 (Orders, Inventory &
Checkout) and the reasoning behind each choice.

---

## 1. `SELECT ... FOR UPDATE` on the Category Row

**Decision:** Use a pessimistic row-level lock on `event_categories` as the serialization
point for concurrent checkouts.

**Alternatives considered:**
- **Optimistic concurrency** (compare-and-swap on a version column): Requires retry logic
  in the application layer. Under high contention (e.g. flash sale), most transactions would
  fail and retry, creating thundering herd.
- **Application-level counter in memory/Redis**: Fast reads, but requires a separate Redis
  instance, introduces cache invalidation complexity, and means the database is no longer
  the single source of truth for inventory.
- **Advisory locks**: More flexible but harder to reason about; doesn't tie naturally to the
  row being protected.

**Why `FOR UPDATE`:**
- PostgreSQL row locks are cheap, well-understood, and automatically released on
  commit/rollback.
- The lock scope is exactly right: one category at a time. Concurrent checkouts on
  *different* categories do not block each other.
- No extra infrastructure required.
- The critical section is small (a few microseconds of DB work), so lock contention is
  short-lived.

**Tradeoff:** Under extreme load (thousands of concurrent checkouts for the same category),
`FOR UPDATE` can create a queue. This is acceptable for Phase 5 volumes; if needed, Phase 8+
can add a Redis-based pre-filter to shed obviously-rejected requests before they reach the
database.

---

## 2. No Redis for Inventory in Phase 5

**Decision:** PostgreSQL is the sole source of truth for inventory counts. Redis is not used.

**Reasoning:**
- Redis was already in the stack for Phase 1's health check, but using it for inventory
  requires careful cache invalidation: every cancel, expiry, and payment would need to
  update the cache atomically with the DB write.
- Incorrect cache values would cause overselling or false "sold out" errors — both
  unacceptable.
- The `FOR UPDATE` approach (see above) is correct-by-construction with no cache.
- Redis is reserved for Phase 8's job queue (background task processing). Mixing
  inventory counting into Redis now would complicate Phase 8's design.

**Tradeoff:** At very high read volume (millions of `/public/events/:slug` requests showing
remaining capacity), a Redis cache for the *read path* (not the write path) could reduce DB
load. This can be added later as a pure read optimization without changing the write path.

---

## 3. Participant = Logged-In User Only

**Decision:** Checkout requires authentication. Guest checkout (no account) is not supported
in Phase 5.

**Reasoning:**
- Guest checkout requires associating an order with an email address, handling order lookup
  without a user account, and re-authentication for cancellation.
- The additional complexity was out of scope for Phase 5.
- The `user_id` foreign key on `orders` provides a clean ownership model with zero ambiguity.

**Tradeoff:** Some participants may prefer not to create an account. Guest checkout can be
added in a future phase by making `user_id` nullable and adding an `email` column to `orders`.

---

## 4. One Slot Per Checkout (No Multi-Item Cart)

**Decision:** Each checkout creates exactly one order for one slot in one category. There
is no cart or multi-item order.

**Reasoning:**
- A participant buying multiple slots (e.g. a family registration) is a distinct UX pattern
  (group registration) that requires its own design.
- Keeping orders 1:1 with reservations simplifies the inventory formula, the expiration
  worker, and the state machine.
- The schema supports multi-item orders in Phase 6+ by adding an `order_items` table without
  breaking Phase 5 orders.

**Tradeoff:** A participant who wants 2 slots must complete checkout twice. This is a known
limitation documented in the PRD.

---

## 5. Worker as a Separate Binary

**Decision:** The expiration worker (`services/api/cmd/worker`) is a separate process from
the API (`services/api/cmd/api`).

**Reasoning:**
- **Clean separation of concerns** — the API handles synchronous request/response; the
  worker handles background time-based processing. Mixing them would require goroutine
  lifecycle management inside the API server.
- **Independent scaling** — in production, you run one (or a few) worker instances and
  N API instances. Running the worker inside the API would either run N copies of the
  worker (wasteful, causes contention) or require leader election.
- **Independent deployment** — the worker can be updated or restarted without touching
  the API process.
- **Crash isolation** — a panic in the worker does not affect the API.

**Idempotency design:**
- `FOR UPDATE SKIP LOCKED` ensures that if two worker instances run concurrently (e.g.
  during a rolling restart), they process disjoint sets of orders.
- The `AND status = 'ACTIVE'` guard on reservation updates means a double-run is a no-op.

---

## 6. Order Number Format and Collision Retry

**Decision:** `ORD-YYYYMMDD-XXXXXX` where XXXXXX is 6 uppercase base-36 characters
generated with `crypto/rand`.

**Reasoning:**
- Human-readable prefix (`ORD`) makes support queries easy.
- Date component (`YYYYMMDD`) allows natural partitioning and approximate age estimation.
- 6 base-36 chars = 36^6 ≈ 2.1 billion combinations per day. At Phase 5 scale, collision
  probability per order ≈ 1 / 2,176,782,336 — effectively zero.
- `crypto/rand` (not `math/rand`) ensures no two concurrent goroutines produce identical
  sequences even without a mutex.
- The `UNIQUE` database constraint is the final guard. On collision, the generator retries
  up to 5 times before returning an error.

**Tradeoff:** The random component means order numbers are not sequential. A sequential
approach (DB sequence) would be simpler but would expose volume information to participants.

---

## 7. Extend-Don't-Rewrite: What Phase 5 Added

**Decision:** Phase 5 added new modules (`orders`, `inventory`) and configuration without
modifying any existing Phase 1–4 module files.

**What was added:**
- `services/api/internal/modules/orders/` — new module
- `services/api/cmd/worker/` — new binary
- `services/api/internal/app/config.go` — two new fields (`OrderExpiration`, `WorkerInterval`) appended
- `services/api/internal/app/server.go` — `ordersHandler` wired in via `RegisterRoutes` and `RegisterEventRoutes`
- `database/migrations/00012-00014` — new tables (`orders`, `inventory_reservations`) and permissions
- `packages/ui/` — new workspace package

**What was NOT changed:**
- `internal/modules/auth/`
- `internal/modules/events/`
- `internal/modules/categories/`
- `internal/modules/forms/`
- `internal/modules/members/`
- `internal/modules/roles/`
- `internal/modules/organizations/`
- `internal/modules/publiccatalog/`

This extend-only policy ensures Phase 1–4 functionality is unchanged and reduces the risk of
regression. All Phase 1–4 integration tests continue to pass unchanged.
