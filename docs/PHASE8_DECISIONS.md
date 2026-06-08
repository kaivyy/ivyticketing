# Phase 8 Design Decisions

## 1. Hybrid Store: Postgres (Durable) + Redis (Real-Time)

**Decision:** Queue state is durably persisted in Postgres with Redis sorted sets as
a rebuildable real-time index for position display and counts.

**Why:** At 100k concurrent participants, position lookup via Postgres `COUNT(*)`
or `ROW_NUMBER()` would add latency to every status poll (every 4 seconds per
participant). Redis sorted sets deliver O(log N) rank in sub-millisecond time
for this scale. But Redis alone is insufficient -- it is an in-memory store, and
a crash or restart loses all queue state.

Postgres is the durable source of truth. Every mutation hits Postgres first
(inside transactions). Redis is populated as a best-effort side effect. If
Redis is down, position display degrades gracefully (shows position 0) but
the checkout admission check and all queue mechanics work exclusively from
Postgres.

Redis sorted sets are rebuildable: iterate `WHERE status = 'WAITING' ORDER BY
pool DESC, score ASC` and `ZADD` them back. A rebuild script can be added in
a future phase.

---

## 2. Foundation-First: Registration Module Shared Across Phases 9-11

**Decision:** The registration module (9 modes, resolver, settings endpoints) was
built in Phase 8 even though only NORMAL, CLOSED, and the three queue modes are
active. BALLOT, INVITATION_ONLY, PRIORITY_ACCESS, and WAITLIST_ONLY are in the
enum and can be saved as settings, but the gate returns
`REGISTRATION_MODE_NOT_AVAILABLE` for deferred modes.

**Why:** The `RegistrationGate` seam in the orders module is the integration
point between registration decisions and checkout admission. If we deferred the
full mode set to later phases, each new mode would require:
1. Updating the mode enum (migration).
2. Updating the gate logic (new error code, new case).
3. Updating settings endpoints (new field in request/response).
4. Updating the frontend (new mode option in dropdowns).
5. Regression-testing existing modes.

Building the complete enum and resolver once in Phase 8 makes Phase 9-11
additions pure capability work (implement the actual mode logic) without
touching the gate seam or the orders module at all.

**Tradeoff:** Six modes are defined that do nothing useful in Phase 8. This is a
deliberate forward investment. The maintenance cost is minimal -- the resolver is a
pure function with a test, the gate has a one-line `default` case, and the deferred
modes are documented.

---

## 3. Three Queue Modes in Phase 8 (WAR / RANDOMIZED / HYBRID)

**Decision:** Phase 8 implements three queue modes, not one. These cover the three
most common real-world patterns:

- **WAR_QUEUE**: Pure FIFO. The traditional "war ticket" model where quickest
  fingers win. Used for concerts, festival passes, limited-edition releases.
- **RANDOMIZED_QUEUE**: Presale random pool + post-sale FIFO. The
  "virtual waiting room" model. Used for high-demand events where fairness
  matters more than speed.
- **HYBRID_QUEUE**: Identical ordering to RANDOMIZED_QUEUE in this phase.
  Structurally the same but treated as a separate mode so future extensions
  can differentiate (e.g., HYBRID might later incorporate priority tiers that
  RANDOMIZED does not).

**Why:** The three modes cover approximately 90% of the use cases organizers
ask for. Organizers running a 5K charity run want WAR_QUEUE. A major conference
with 50k+ attendees wants RANDOMIZED_QUEUE. A marathon with sponsor early access
wants HYBRID_QUEUE. Shipping one mode would limit the feature's applicability.

**Tradeoff:** HYBRID_QUEUE is functionally identical to RANDOMIZED_QUEUE in
Phase 8. This creates a minor "leet" complexity (two enums, one implementation)
but the naming is intentionally separate to allow future divergence without
renaming or migrating existing event settings.

---

## 4. Anti-Bot Stub (Phase 9 Fills)

**Decision:** The join endpoint runs through an `EntryGuard` middleware that is
currently a no-op pass-through. Rate limiting, Turnstile verification, and
duplicate detection are deferred to Phase 9.

**Why:** Anti-bot is important but separable. There is no point building
sophisticated bot detection before the queue itself is proven. Conversely,
bot detection added after the queue is already in production requires no
architectural changes -- the guard is already positioned at the join entry
point. Phase 9 implements the guard body; Phase 8 wires the placeholder.

**Tradeoff:** In Phase 8, there is no protection against a script that hammers
`POST /queue/join` with many accounts. The `UNIQUE (event_id, participant_id)`
constraint prevents one account from creating multiple tokens, but it does
not prevent one operator with many accounts from joining all of them.
This is a known gap accepted for Phase 8 delivery velocity.

---

## 5. Per-Event Scope

**Decision:** Queue configuration (mode, release rate, schedule, seed) is
per-event, not per-organization. Each event has its own `queue_control` row
and its own `event_registration_settings` row.

**Why:** Different events within the same organization have different needs:
- Event A (marathon main race): WAR_QUEUE, high release rate, large capacity.
- Event B (kids dash): NORMAL, no queue needed.
- Event C (VIP gala dinner): INVITATION_ONLY (future).

A per-organization default would be a convenience convenience feature (apply
this mode to all new events) rather than a primary setting. The org-wide
default can be added later as a special case without changing per-event
behavior.

**Tradeoff:** Organizers with many similar events must configure the queue
per event. A "copy settings from event" feature could be added in a future
phase to reduce repetition.

---

## 6. Admission via X-Queue-Token Header

**Decision:** After promotion to ALLOWED, the participant's admission token
(an opaque UUID) is returned in the status response. The participant sends it
back as the `X-Queue-Token` HTTP header on the checkout request.

**Why:**
- **Stateless verification.** The server reads the header, looks up the
  admission row by `(event_id, participant_id)`, and verifies the token matches
  and is not expired. No server-side session or cookie is required.
- **REST-compatible.** Standard HTTP header, no custom auth scheme, works
  with any HTTP client. The frontend sends it as a regular fetch header.
- **No URL leakage.** The token is never in query parameters or path segments,
  avoiding accidental sharing via copy-paste, screenshots, or server access
  logs.
- **CSRF-safe.** Custom headers are not sent by simple HTML forms; they
  require JavaScript. Combined with the existing auth token, this provides
  a defense-in-depth against CSRF attacks.

**Tradeoff:** The admission token is a bearer credential. Anyone who obtains
it (e.g., via XSS) can complete checkout on behalf of the participant. The
token is short-lived (QUEUE_CHECKOUT_WINDOW, default 5m), limiting the
exposure window. Longer windows should be paired with stricter CSP and
auditing.

---

## 7. Frontend Included in Phase 8

**Decision:** Phase 8 includes frontend pages (Astro components): a waiting
room with auto-polling, position display, ETA, and ALLOWED redirect; and an
organizer queue controls page with pause/resume, rate adjustment, and stats.

**Why:** The waiting room is user-facing. A backend-only queue would not be
shippable -- the participant must see their position, know they are waiting,
and be notified when they are allowed to checkout. Without the frontend, the
queue is an API that no one can use.

The organizer controls page similarly is essential to the feature. A war-day
operator cannot be expected to use curl or Postman to pause a queue under
load.

**Tradeoff:** Frontend work in Phase 8 adds scope but produces a complete,
demonstrable feature. The frontend is intentionally simple (poll-based, no
WebSockets) to keep the implementation cost proportional to the backend.

---

## 8. Position and ETA in Status Response

**Decision:** `GET /queue/status` returns `position` (0-based rank in the
waiting line) and `estimatedWaitSeconds` (position / release rate).

**Why:** User trust. A queue without position is a black box -- participants
have no idea if they are 5th or 50,000th in line, no sense of progress,
and no basis for deciding whether to wait or give up. Position provides
transparency.

`estimatedWaitSeconds` is a coarse approximation (`position / rate`), not a
guaranteed ETA. It is labeled as an "estimate" in the UI. The actual wait
depends on:
- How many users ahead complete checkout vs. let their admission expire.
- Whether the release rate is changed live.
- Whether the queue is paused.

**Tradeoff:** Position accuracy depends on Redis being up. When Redis is
down, position degrades to 0. A future improvement could compute position
from Postgres as a fallback (slower but accurate), or use WebSockets for
push-based position updates (avoiding the polling overhead entirely).

---

## 9. Pure Release Rate (No Inventory Pre-Check)

**Decision:** The release engine promotes WAITING tokens at the configured rate
without checking whether inventory is available. The inventory lock during
checkout (`SELECT ... FOR UPDATE` on the category row) is the oversold
backstop.

**Why:** Pre-checking inventory before admission creates a TOCTOU (time-of-check
to time-of-use) gap: inventory may be available when the user is admitted,
depleted before they reach checkout. This means either:
1. Pre-check is a lie (user is admitted but cannot checkout) -- broken UX.
2. Admission must reserve inventory at promote time -- couples release to
   reservation, adds cross-module complexity for no net gain.

The pure-rate approach is simpler: users are admitted at a steady rate, and
if inventory is exhausted when they reach checkout, the checkout transaction
fails with the appropriate error (inventory depleted). The admission simply
expires and they are requeued -- they were never going to get a ticket anyway,
and no stale reservation needs cleanup.

**Tradeoff:** During the final moments of inventory depletion, more users may
be in ALLOWED state than there are remaining tickets. Some will reach
checkout only to find inventory gone. This is a known and accepted UX
tradeoff -- better than the alternative of over-committing and needing a
complex inventory-reservation-undone flow.

---

## 10. Expired Admission -- Requeue, Don't Eject

**Decision:** When an admission expires (user did not complete checkout within
the window), the token is requeued to the back of the WAITING line with a new
FIFO score. The user is not removed from the queue.

**Why:** Better UX for a common failure mode. A user's admission may expire
because:
- They stepped away from their device.
- Network interruption (mobile data drop, WiFi hiccup).
- Browser tab was backgrounded and the redirect didn't fire.
- Payment was slow or failed, they need to retry.

Permanently ejecting the user means a support ticket filed, an organizer
manually intervening, and an angry participant. Requeuing to the back gives
them another chance automatically. If they miss the window again, they
requeue again -- eventually they either complete checkout or inventory runs
out.

**Tradeoff:** A user with expired admission jumps ahead in line -- they would
have been at position 0, and now they are at position N (back of the line),
inserted ahead of users who joined after them. This is a fairness compromise
in favor of recovery and reduced support burden.
