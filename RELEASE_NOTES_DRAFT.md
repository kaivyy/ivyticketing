# IvyTicketing <next-version>

> **Note**: Version tag placeholder `<next-version>` requires human decision based on whether this release is tagged as `v0.2.0`, `v1.0.0`, or following the project's semantic versioning roadmap (advancing from the Phase 27 / v0.1.0 baseline).

## Highlights

- **Full Multi-Sport Competition Engine**: Extension from endurance road races into a generic tournament engine supporting stages, matches, heats, rosters, round-robin pools, knockout brackets, and BWF badminton rally scoring resolvers.
- **Multi-Vendor Timing & Results Integration**: Unified timing bridge supporting RaceResult, Native RFID transponder passings, and CSV uploads with automatic split calculations and dynamic finisher certificates.
- **Critical Security & Concurrency Remediation**: Resolved P0 ballot winner false lapse edge case, enforced transactional atomic access grant consumption, and hardened the high-traffic queue status endpoint with 2-second Redis caching and sliding-window rate limiting.
- **Production-Ready Documentation System**: Complete 36-file documentation suite under `docs/` covering getting started, core concepts, user guides, architecture, workflows, API reference, runbooks, and developer onboarding.

---

## Added

### Multi-Sport Tournament Engine
- Database migrations `00066_create_generic_sports_schema.sql`, `00067_multi_sport_integrity_and_performance.sql`, and `00068_add_bye_walkover_match_status.sql` introducing normalized tournament entities.
- Tournament stage formats: `single_race`, `group_round_robin`, `single_elimination`, `double_elimination`, `heats`, and `peloton_stage`.
- Competition match statuses: `SCHEDULED`, `LIVE`, `COMPLETED`, `SUSPENDED`, `CANCELLED`, `FORFEIT`, `WALKOVER`, and `BYE`.
- Official sport scoring resolvers: Badminton World Federation (BWF) 21-point rally scoring with deuce and road cycling bunch finish time gap rules.
- Multi-participant formation support: `INDIVIDUAL`, `TEAM`, `PAIR`, `RELAY`, and `SQUAD`.

### Timing & Results Integration
- Timing provider adapter interface supporting RaceResult, Native RFID transponders, and Generic CSV imports.
- Mat checkpoint passings, intermediate split time tracking (5K, 10K, Halfway, 30K), and automatic category/gender ranking calculations.
- Dynamic finisher certificate renderer with cryptographic QR verification stamp.

### Comprehensive Documentation System
- Root `README.md` redesigned as a polished open-source landing page.
- Master index in `docs/README.md` with role-based navigation.
- 36 source-verified guides across `getting-started/`, `concepts/`, `user-guides/`, `architecture/`, `workflows/`, `reference/`, `operations/`, `development/`, and `security/`.

---

## Fixed

### Critical P0 Ballot Winner False Lapse
- Resolved race condition where paid ballot winners were incorrectly lapsed by the expiration worker.
- Updated `ballot_entries.status` to `CONVERTED` atomically within the order checkout and payment transactions.
- Added expirer order cross-check preventing lapsing any entrant with an active or paid order.

### Access Grant Single-Use Enforcement & Atomicity
- Enforced single-use consumption of `access_grants` atomically inside the `orders.Checkout` PostgreSQL transaction before commit.
- Added participant identity and category validation on all access grant redemptions.

### High-Traffic War Queue Hardening
- Implemented 2-second Redis status caching (`queue:status:{eventId}:{participantId}`) to protect PostgreSQL connection pools during mass polling.
- Added sliding-window rate limiting on queue status polling returning `429 TOO_MANY_REQUESTS` on excessive polling.

### Multi-Tenant RBAC & BOLA Defense
- Hardened `middleware/authz.go` with canonical slug-to-UUID rewriting and strict event-to-organization database ownership verification, preventing cross-tenant access even in platform admin mode.

---

## Security

- **Row-Level Inventory Locking**: Enforced `SELECT ... FOR UPDATE` in PostgreSQL to guarantee zero overselling under high concurrency.
- **Cryptographic Ticket Authenticity**: HMAC-SHA256 signed QR codes verified offline with no master secret leakage.
- **Webhook Idempotency**: SHA-256 payload hashing in `payment_webhooks` guaranteeing at-most-once processing of payment notifications.
- **Bot Mitigation**: Cloudflare Turnstile integration, IP reputation scoring, and sliding-window rate limiters.

---

## Performance

- **Sub-Millisecond Queue Polling**: Redis caching reduces database read queries to zero during active waiting-room polling.
- **Optimized Composite Indexes**: Added indexes on `(event_id, status)` and `(organization_id, id)` across all high-throughput tables.

---

## Testing & Reliability

- All 35+ backend module test suites passing (`go test ./internal/modules/...`).
- Verified zero overselling with 50-thread concurrent adversarial checkout benchmarks.
- Verified zero double-allocation in payment vs winner-expiration race conditions.
- Strict TypeScript verification with zero `any` types.

---

## Upgrade Notes

- Run Goose migrations up to `00068`:
  ```bash
  goose -dir database/migrations postgres "$DATABASE_URL" up
  ```
- Ensure environment configuration includes `TICKET_QR_SECRET`, `JWT_SECRET`, `DATABASE_URL`, and `REDIS_URL`.
- Recommended Redis memory policy: `maxmemory-policy noeviction`.
