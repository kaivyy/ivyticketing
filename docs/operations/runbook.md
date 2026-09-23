# Operator Runbook and War-Day Procedures

This runbook defines the standard operating procedures (SOPs) for site reliability engineers (SREs), system operators, and event directors during high-traffic ticket release windows ("War-Day").

---

## 1. Pre-Launch Readiness Checklist

### T-minus 72 Hours: Infrastructure Validation
- [ ] **Database Connection Limits**: Verify PostgreSQL `max_connections` (minimum 200 recommended) and pgxpool pool size.
- [ ] **Redis Memory & Eviction Policy**: Ensure Redis `maxmemory-policy` is set to `noeviction` so waiting queues are never silently dropped under memory pressure.
- [ ] **SSL & CDN**: Confirm Cloudflare SSL termination and Turnstile keys are verified.
- [ ] **Payment Gateways**: Test end-to-end sandbox or live penny transactions with Duitku and Xendit.

### T-minus 1 Hour: Pre-War Room Setup
- [ ] **Telemetry Dashboard**: Open `/admin/warroom` or Prometheus Grafana dashboard.
- [ ] **Worker Verification**: Confirm the worker daemon (`cmd/worker`) is running and logging:
  - `queue_release`
  - `queue_admission_expiry`
  - `expire_orders`
- [ ] **Queue Verification**: Confirm target event `registration_mode` is set to `WAR_QUEUE` or `RANDOMIZED_QUEUE`.
- [ ] **Initial Release Rate**: Set default rate to `100` participants per 10 seconds.

### T-minus 10 Minutes: Final Checks
- [ ] Confirm database acquired connection count is low (<10).
- [ ] Check Redis latency (`redis-cli --latency`).
- [ ] Verify organizer operational staff are on standby.

---

## 2. Active Sale Launch Procedures (T-Zero)

1. **Monitor Inbound Rate**: Observe Redis waiting room depth (`queue:{eventId}:waiting`).
2. **Observe Checkout Conversion**: Track orders transitioning to `PENDING_PAYMENT` and `PAID`.
3. **Database Health**:
   - Inspect active vs idle database connections. Acquired connections should remain below 80% of pool capacity.
   - If acquired connections exceed 90%, throttle the queue release rate immediately.

---

## 3. Dynamic War-Room Rate Throttling

If upstream payment gateways experience elevated response times (>5s) or database contention increases:

### Throttling Down
Reduce release rate to relieve pressure on payment partners:
```bash
curl -X POST https://api.ivyticketing.com/api/v1/organizations/{orgId}/events/{eventId}/queue/release-rate \
  -H "Authorization: Bearer $ORGANIZER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"rate": 50}'
```

### Boosting Up
If checkout latency is low (<150ms) and database saturation is negligible (<25%):
```bash
curl -X POST https://api.ivyticketing.com/api/v1/organizations/{orgId}/events/{eventId}/queue/release-rate \
  -H "Authorization: Bearer $ORGANIZER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"rate": 200}'
```

---

## 4. Emergency Procedures

### Scenario A: Payment Gateway Total Outage
If Duitku or Xendit reports an outage during active ticket sales:
1. **Pause the Queue Immediately**:
   ```bash
   curl -X POST https://api.ivyticketing.com/api/v1/organizations/{orgId}/events/{eventId}/queue/pause \
     -H "Authorization: Bearer $ORGANIZER_TOKEN"
   ```
2. **Post Public Incident Notice**:
   Navigate to `/admin` -> **Status Page** and mark Payment Gateway component as `DOWN`.
3. **Protect In-Flight Checkouts**: Do not cancel existing `PENDING_PAYMENT` orders immediately; grant extra time for gateway reconnection.

### Scenario B: Accidental Queue Jam / Stuck Tokens
If participant admission becomes stuck due to an unhandled exception:
1. Clear the event's allowed set and rebuild the waiting pool:
   ```bash
   # Inspect Redis allowed set size
   redis-cli ZCARD queue:{eventId}:allowed
   ```
2. Re-trigger the queue release worker.
