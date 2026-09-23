# Testing and Verification Guide

IvyTicketing utilizes a rigorous multi-tier testing strategy encompassing unit tests, database integration tests, race condition detection, and high-concurrency adversarial benchmarks.

---

## 1. Running Backend Test Suites

All backend tests are located alongside domain packages or in `tests/`:

### Run All Unit and Integration Tests
```bash
cd services/api
go test -v ./internal/modules/...
```

### Run with Race Detector
To detect data races and concurrent memory access anomalies:
```bash
cd services/api
go test -race ./internal/modules/...
```

### Run a Specific Module Suite
```bash
# Test order checkout & inventory reservation
go test -v ./internal/modules/orders/...

# Test high-traffic queue and status caching
go test -v ./internal/modules/queue/...

# Test ballot draws and winner expiration
go test -v ./internal/modules/ballot/...
```

---

## 2. High-Concurrency Adversarial Tests

IvyTicketing includes integration tests designed to simulate "war-day" load:

### Overselling Prevention Test
Spawns 50 concurrent goroutines attempting to purchase the last 5 available category slots.
```bash
cd services/api
go test -v -run TestConcurrentCheckoutOversellProtection ./internal/modules/orders/...
```
**Verification Requirement**: Exactly 5 orders succeed with `201 Created`; 45 orders fail with `409 POOL_EXHAUSTED`. Database `sold_quota` must equal 5 and `available_quota` must equal 0.

### Payment vs. Winner Expiration Race Test
Simulates a ballot winner submitting payment at the exact second the `ballot_winner_expirer` daemon sweeps the database.
```bash
cd services/api
go test -v -run TestPaymentVsWinnerExpirationRace ./internal/modules/ballot/...
```
**Verification Requirement**: The order transitions to `PAID` and the ballot entry transitions to `CONVERTED`. The entry is never lapsed or reallocated to the waitlist.

---

## 3. Frontend Typechecking and Linting

Verify TypeScript integrity across the Astro portal and Svelte scanner:

```bash
# Typecheck Web Portal (Zero 'any' allowed)
cd apps/web
pnpm check

# Typecheck Scanner PWA
cd apps/scanner
pnpm check
```
