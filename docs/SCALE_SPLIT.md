# Scale Split — From Modular Monolith to Services (Phase 25)

This is a **decision guide**, not a migration order. The API today is a modular
monolith (`services/api`) with clean module boundaries. That is deliberate and
correct for our traffic. Do **not** split a module into its own service until a
concrete, measured trigger below fires. Premature splitting buys you distributed
systems problems (network partitions, partial failure, cross-service
transactions, deploy coordination) in exchange for scaling headroom you do not
yet need.

> Golden rule: **split for an observed bottleneck, never for architecture
> aesthetics.** If you cannot point at a graph, you are not ready to split.

## What is already out-of-process

Two workloads already run as separate binaries — they are the proof that our
boundaries are extraction-ready, not a signal that more splits are due:

| Binary | Path | Why it is separate |
| --- | --- | --- |
| Webhook receiver | `services/api/cmd/webhook` | Payment callbacks must stay up and fast even if the main API is saturated. Isolated so a checkout spike cannot drop a gateway callback. |
| Async worker | `services/api/cmd/worker` | Notification dispatch, retries, expirations. Background work must not compete with request-path latency. |

Both share the same module code and the same database. That is the model every
future split should follow: **extract the deployment unit, keep the module code
and its contract intact.**

## Split candidates and their triggers

Each module below *could* become its own service. The **Trigger** column is the
only reason to actually do it. Until then, leave it in the monolith.

| Candidate | Module(s) | Trigger to extract | Signal source |
| --- | --- | --- | --- |
| Webhook service | `payments/webhook` | Payment callback latency degrades API p95 (already extracted as a binary — next step is its own DB read replica / deploy). | Grafana War Day: API p95 rises in lockstep with `payment_callback` route volume. |
| Queue service | `queue` | Queue join/poll traffic dominates request volume and Redis ops saturate. | `http_requests_total{route=~"/queue.*"}` > 60% of total; Redis CPU sustained > 70%. |
| Payment service | `payments`, `orders` (checkout hot path) | Checkout throughput needs independent horizontal scaling from read traffic. | Order-create p95 breaches SLO while read routes are healthy. |
| Notification service | `notifications` | Notification volume (email/WA) causes worker backlog that delays other jobs. | Worker queue depth alert; `notification_dispatch` lag sustained. |
| Report service | `reporting`, `results` | Large CSV/report exports lock DB rows or blow connection pool. | DB pool exhaustion alert correlated with `reports/export` or `results/import`. |

## Extraction pattern (how, when a trigger fires)

Follow the webhook/worker precedent. The goal is **no big rewrite** — the
acceptance criterion from the masterplan.

1. **Confirm the trigger with data.** Screenshot the Grafana panel. Attach it to
   the extraction ticket. No graph, no split.
2. **Freeze the module contract.** The module already exposes a `Service`
   interface and a repository behind `NewRepository(pool)`. That interface *is*
   the service contract. Do not change its signatures during extraction.
3. **Stand up a new `cmd/<name>` binary** that wires only that module's routes
   (mirror `cmd/webhook`): its own `chi.Router`, `/healthz` + `/readyz`, its own
   `/metrics` registry.
4. **Decide the data boundary.**
   - *Shared DB (default first step):* point the new binary at the same Postgres.
     Zero data migration. This alone gives independent deploy + scale.
   - *Dedicated DB (only if the DB itself is the bottleneck):* give the service
     its own database and define an explicit sync/event contract. This is a
     real project — do not do it to "be clean."
5. **Define the API contract explicitly.** If other modules called it in-process,
   replace those calls with an HTTP/gRPC client behind the *same Go interface*,
   so callers do not change. Document the contract in this file's appendix.
6. **Route at the edge.** Point the reverse proxy / ingress at the new service
   for its path prefix. The monolith keeps serving everything else.
7. **Cut over behind a flag, verify, then delete** the in-monolith route.

## Contracts must stay explicit

For any split service, record in an appendix here:

- **Owned paths** (e.g. `/api/v1/queue/*`).
- **Inbound contract**: request/response shapes (link the DTO structs).
- **Outbound dependencies**: which other services/DBs it calls.
- **Idempotency + retry semantics** (critical for payment/webhook/notification).
- **Golden invariants it must not break** (see below).

## Invariants that survive every split

These are enforced in code today and must hold no matter how the system is
carved up:

- **No oversell.** Inventory is gated by a `FOR UPDATE` row lock in
  `inventory.CheckAndLock`. A split payment/order service still takes that lock
  against the authoritative inventory rows.
- **No double payment.** `payments.Processor` is idempotent on the order state
  machine. A standalone payment service keeps that idempotency; duplicate
  callbacks stay safe.
- **No secret duplication.** `TICKET_QR_SECRET` composes exactly one
  `qr.Signer`, shared by tickets and scanner. A split scanner/ticket service
  must receive that single signer's secret — it must **never mint a second
  signer or copy the secret into a new service's own config path.**

## Per-service monitoring (acceptance criterion)

Every extracted service ships with, from day one:

- `/healthz` (liveness) and `/readyz` (readiness — checks DB/Redis it depends on).
- Its own `/metrics` Prometheus endpoint with request rate, p95 latency, and
  error rate labelled by route.
- A Grafana row (clone the War Day panels) scoped to the service.
- Alerts mirroring `ops/prometheus/alerts.yml` (error rate, p95, saturation),
  scoped to the new service's job label.

Without these, the split has *reduced* observability and must not ship.

## Acceptance checklist

- [ ] Trigger confirmed with a Grafana screenshot attached to the ticket.
- [ ] Module contract (Service interface) unchanged through the extraction.
- [ ] New `cmd/<name>` binary with `/healthz`, `/readyz`, `/metrics`.
- [ ] Data boundary decided and documented (shared DB unless DB is the bottleneck).
- [ ] API contract documented in the appendix.
- [ ] Golden invariants verified still hold post-split.
- [ ] Per-service Grafana row + alerts live before cutover.
- [ ] Rollback = re-enable the monolith route (kept behind a flag until stable).
