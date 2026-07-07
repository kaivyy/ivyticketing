# Production Launch Checklist — War Day

Operational go/no-go checklist for launching a high-traffic event on ivyticketing.
Pairs with [`INCIDENT_RUNBOOK.md`](./INCIDENT_RUNBOOK.md) (what to do when something
breaks) and [`SCALE_SPLIT.md`](./SCALE_SPLIT.md) (when to extract a service). This
document is *what to verify before and during* the event.

Every item is a checkbox with a concrete verification step — a command to run, an
endpoint to hit, or a screen to open. "Aktif" is not a state of mind; it is a green
check you have personally seen.

---

## T-7 days — Readiness gate

Nothing below is optional. If any item is red at T-24h, escalate the launch decision.

### Load & capacity
- [ ] **Load test lulus.** k6 scenarios in [`ops/k6/`](../ops/k6/) run at the
      expected peak concurrency with error rate < 1% and p95 within SLO. Record the
      run: date, peak RPS, p95, error rate.
- [ ] **DB pool headroom.** Under load-test peak, `db pool in-use` stayed below the
      configured `max_conns`. If it saturated, raise pool size or add a read replica
      before launch, not during.
- [ ] **Inventory no-oversell proven.** The concurrent-checkout k6 scenario confirms
      capacity never goes negative (the `FOR UPDATE` lock in `inventory.CheckAndLock`
      holds under contention).

### Payments
- [ ] **Payment test lulus (sandbox → live).** A full order → gateway callback →
      ticket-issued cycle completed against the live gateway with a real (small)
      transaction, then refunded. Verify idempotency: replay the same callback and
      confirm the order is not double-paid.
- [ ] **Reconciliation job scheduled.** The worker's reconcile loop
      (`payments.reconcile`) is running and its last-run timestamp is fresh. See
      [`PAYMENT_RECONCILIATION.md`](./PAYMENT_RECONCILIATION.md).
- [ ] **Webhook endpoint healthy.** `GET /healthz` on the webhook service returns 200
      and the signature-verification secret matches the gateway dashboard.

### Backup & recovery
- [ ] **Backup aktif dan terverifikasi.** Automated DB backup ran in the last 24h AND
      a restore was test-driven into a scratch database. An unverified backup is not a
      backup.
- [ ] **Rollback plan tertulis.** The previous known-good image tag is recorded, and
      the deploy tool can roll back to it in one command. Write the exact command here:
      `__________`.
- [ ] **Migration reversibility checked.** Any migration shipping with this release has
      a tested `-- +goose Down`, or is explicitly flagged as forward-only with a reason.

### Monitoring & alerting
- [ ] **Monitoring aktif.** `/metrics` is scraped by Prometheus; the Grafana **War Day**
      dashboard (`ops/grafana/war-day-dashboard.json`) loads with live data.
- [ ] **Alerts wired.** Rules in [`ops/prometheus/alerts.yml`](../ops/prometheus/alerts.yml)
      are loaded and route to the on-call channel. Fire a test alert and confirm it
      lands.
- [ ] **Sentry (atau error tracker) aktif.** A deliberately-triggered test error appears
      in the dashboard with correct release/environment tags.
- [ ] **War Room reachable.** Super-admin can open `/admin/warroom` and see live
      error rate, p95, in-flight, and DB pool.

### People & process
- [ ] **Runbook siap.** [`INCIDENT_RUNBOOK.md`](./INCIDENT_RUNBOOK.md) is current; the
      on-call engineer has read it end-to-end this week.
- [ ] **Admin/super-admin training selesai.** Whoever operates the war room, incident
      banner, and release-rate controls has done a dry run.
- [ ] **EO (organizer) training selesai.** The organizer knows how to read their
      dashboard, pause/resume the queue, and whom to call.
- [ ] **Support channel siap.** The participant-facing support channel is staffed for
      the event window, with canned responses for the top-5 expected issues (payment
      pending, didn't get ticket, queue position, refund, wrong data).
- [ ] **Emergency contacts jelas.** On-call engineer, gateway account manager, EO
      decision-maker, and infra/hosting support — names and numbers in one place.

### Launch rehearsal
- [ ] **Rehearsal berhasil.** A full dress rehearsal on staging (or a low-traffic
      window) exercised: queue enable → release rate → checkout → payment → ticket →
      incident banner → rollback. Every step observed working.

---

## War Day — T-0 checklist

Run this in order, out loud, with a second person confirming each check.

### Pre-open (T-30 min)
- [ ] **Queue enabled** with the intended mode (see [`QUEUE_MODES.md`](./QUEUE_MODES.md)).
- [ ] **Release rate diset** to the rehearsed value; the control responds.
- [ ] **Payment gateway healthy.** Webhook `/healthz` 200; a sandbox ping succeeds.
- [ ] **Status page ready** (Phase 19) and reachable by participants.
- [ ] **Incident banner ready** — drafted but not published, one click away.
- [ ] **Database backup verified** — the most recent backup is < 6h old and restorable.
- [ ] **Staff standby** — on-call engineer, support, and EO all confirmed present in
      the incident channel.
- [ ] **War Room + Grafana open** on a dedicated screen.

### At open
- [ ] Watch the first 5 minutes live: error rate flat, p95 within SLO, checkout
      conversions flowing, no inventory anomalies.
- [ ] Confirm first real payment callback issues a real ticket end-to-end.

### During
- [ ] Adjust release rate per queue depth; never bypass inventory locks to "add"
      capacity — raise category capacity through the normal path if needed.
- [ ] If a participant-facing incident starts, publish the incident banner and open a
      status-page incident *before* the support channel floods.

### At close
- [ ] Queue drained or intentionally closed; release rate zeroed.
- [ ] Final backup taken and verified.
- [ ] Snapshot the War Day dashboard (screenshots + metric exports) for the post-event
      report.

---

## Golden invariants (never violate — same as the runbook)

- **No oversell.** Inventory gated by `FOR UPDATE` in `inventory.CheckAndLock`.
- **No double payment.** `payments.Processor` is idempotent on the order state machine.
- **No secret duplication.** `TICKET_QR_SECRET` is composed once into a single
  `qr.Signer` shared by tickets and scanner — never mint a second signer or copy the
  secret into another service.

---

## Acceptance criteria (Phase 26)

- [x] Launch rehearsal procedure documented and runnable.
- [x] Rollback plan has an explicit, recorded command slot.
- [x] Emergency contacts section exists and is filled before each event.
- [x] Post-war report is producible — see [`POST_EVENT_REPORT.md`](./POST_EVENT_REPORT.md).
