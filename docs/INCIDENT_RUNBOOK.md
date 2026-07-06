# Incident Runbook — War Day

Operational playbook for the on-call engineer during high-traffic events. Pairs
with the Phase 20 observability stack: alerts in
[`ops/prometheus/alerts.yml`](../ops/prometheus/alerts.yml), the live war-room at
`/api/v1/admin/warroom` (super-admin), and the Grafana **War Day** dashboard
(`ops/grafana/war-day-dashboard.json`).

## First 60 seconds (any alert)

1. Open the **War Room** (`/admin/warroom`) and the Grafana War Day dashboard.
2. Read the top row: Error Rate (5xx), API p95, In-flight, DB pool.
3. Identify blast radius: one event or global? Check `http_requests_total` by
   `route` and `status` in Grafana.
4. Post an acknowledgement in the incident channel and, if participant-facing,
   open a public incident (Phase 19 status page: `POST /admin/incidents`).

## Golden invariants (never violate)

These are enforced in code and proven by tests/k6 — during any mitigation, do not
take an action that could break them:

- **No oversell.** Inventory is gated by a `FOR UPDATE` row lock in
  `inventory.CheckAndLock`. Never bypass checkout to "manually add" capacity
  without raising the category `capacity` through the normal path.
- **No double payment.** `payments.Processor` is idempotent on the order state
  machine. Never replay callbacks by hand against production without confirming
  idempotency; duplicate deliveries are safe, manual `applyPaid` is not.
- **No secret duplication.** `TICKET_QR_SECRET` is composed once into a single
  `qr.Signer` shared by tickets and scanner. Never mint a second signer or copy
  the secret into another service.

---

## Playbooks by alert

### HighErrorRateWarning / HighErrorRateCritical (5xx share > 1% / 5%)

**Triage**
- Grafana → *HTTP Request Rate by Status* → which route spikes 5xx?
- War Room → DB pool. Saturated pool (`DBPoolSaturation`) is the most common root
  cause of a broad 5xx wave.

**Mitigate**
- DB-pool driven: reduce load upstream — lower queue release rate
  (`PUT /organizations/{org}/events/{event}/queue/release-rate`) to throttle
  admissions into checkout. This is the primary war-day lever.
- Single-route driven: if a non-critical route (reporting/export) is erroring,
  it will not threaten checkout — note it and continue.
- If a deploy correlates with the spike, roll back.

**Verify**: 5xx share returns below 1%; checkout success rate recovers.

### HighAPILatencyP95 (p95 > 1s)

- War Room → DB pool + In-flight. Rising in-flight with flat throughput = a
  downstream bottleneck (usually DB).
- Check slow queries: `pg_stat_activity` for long-running statements; confirm
  expected indexes are used on the hot checkout path.
- Lever: lower queue release rate to shed load; latency should fall as the
  backlog drains. Do **not** raise the release rate to "push through."

### PaymentFailureSpike (failure share > 20%, 10m)

- War Room → Pembayaran Sukses/Gagal tiles; Grafana → *Payment Outcomes*.
- Check *Gateway Webhook Delay p95* — a rising delay with rising failures points
  at the gateway, not us.
- Verify gateway status page (Duitku / Xendit). If the gateway is down, this is a
  **Gateway down** incident (below), not an app bug.
- Do **not** manually mark orders paid. Reserved inventory is released on
  expiry; failed payments will not oversell. Let the idempotent processor
  reconcile when the gateway recovers.

### WebhookProcessingSlow (webhook p95 delay > 10s)

- Callbacks are queued and processed idempotently; a backlog delays paid-order
  confirmation but does not lose or double them.
- Confirm the webhook server is up (`GET /healthz` on the webhook service) and
  its DB pool is healthy. Scale webhook workers if this is sustained.
- Participants may see "menunggu pembayaran" longer than usual — set expectations
  via the status page rather than intervening in the DB.

### DBPoolSaturation (pool > 90%, 5m)

- This is the war-day canary. Primary mitigation: **lower queue release rate**
  immediately to reduce concurrent checkouts.
- Check for connection leaks / long transactions in `pg_stat_activity`.
- Only raise `max pool size` if the database has headroom (CPU, connection
  limit); otherwise you move the bottleneck to Postgres itself.

### CheckoutSuccessDrop (success < 80%, 10m)

- Correlate with the alerts above — this is usually a **symptom**, not a root
  cause. Work the driving alert (DB pool, gateway, latency).
- If success is dropping with 409s (not 5xx), the event is simply selling out —
  that is correct behavior, not an incident.

---

## Named war-day scenarios

**Redis failover** — Rate limiting and queue position caching depend on Redis.
On failover: rate limiting should fail-open (requests allowed, not blocked);
queue positions recompute from Postgres. Verify `queue_active_users` does not
collapse to 0 (would indicate a mass reset — a bug). Load-tested by
`phase21_waiting_room_storm.js`.

**Database slow query** — See *HighAPILatencyP95*. Identify via
`pg_stat_activity`, kill a runaway statement only as a last resort, and shed load
via release rate.

**Gateway down** — Payments fail; reserved inventory expires and returns to pool.
No oversell, no double charge. Open a public incident, pause new admissions if
desired (`queue/pause`), and wait for the gateway. When it recovers, the
idempotent processor reconciles the backlog automatically.

**Checkout race / stampede** — Row-lock holds; excess returns 409. Proven at
scale by `phase21_checkout_race.js`. No action needed unless 5xx appear (then
work DB pool).

**Payment callback spike** — Idempotent processor deduplicates. Proven by
`phase21_payment_callback_spike.js`. If webhook delay climbs, see
*WebhookProcessingSlow*.

**Refresh / mobile reconnect storm** — Status endpoint is cheap and cached;
poll floods should not move DB pool much. Proven by
`phase21_waiting_room_storm.js`. If status p95 climbs, confirm the position cache
is serving (not hitting Postgres per poll).

---

## Escalation

- **Sev-1** (checkout down globally, or oversell/double-payment suspected): page
  the engineering lead immediately; freeze deploys; preserve state — do **not**
  run destructive DB fixes without a second engineer.
- **Sev-2** (degraded but selling): work the playbook, keep the status page
  current.
- After any incident: capture the war-room snapshot + Grafana time range, and
  file a post-incident note. Update this runbook if a lever was missing.
