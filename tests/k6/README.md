# Load & Reliability Tests (k6)

Load scripts that prove the ivyticketing API holds its correctness guarantees
under war-day traffic. They complement the Go integration tests: the integration
tests prove the invariants (no oversell, no double payment) at the unit/service
level; these scripts prove they hold **end-to-end through the router, guards, and
connection pool** at scale.

Install [k6](https://k6.io/docs/get-started/installation/), then run any script
with `k6 run <script>.js -e KEY=value ...`.

## Scripts

| Script | Scenario (masterplan Phase 21) | Proves |
| --- | --- | --- |
| `phase21_checkout_race.js` | Checkout race condition | At most `CAPACITY` × 201; excess 409; zero 5xx; no oversell |
| `phase21_waiting_room_storm.js` | 500k waiting room · refresh storm · mobile reconnect storm | Join without 5xx; status stays fast under poll flood; no mass queue reset |
| `phase21_payment_callback_spike.js` | Payment callback spike | Idempotent under duplicate delivery (one ticket / one charge per order); bad sig → 401; no retry storm |
| `phase10_ballot_apply.js` | Ballot apply load (Phase 10) | — |
| `phase11_quota_exhaustion.js` | Access-code quota exhaustion (Phase 11) | Exactly `QUOTA` grants; excess 409 |
| `phase11_redemption_load.js` | Redemption throughput (Phase 11) | — |

## Data files

Several scripts need real, distinct participants — reusing one token would mask
oversell (a user holds only one active reservation per category).

### `tokens.txt` — one bearer token per line

Seed N participant accounts, log each in, and write the JWT per line:

```
eyJhbGci...userA
eyJhbGci...userB
...
```

Generate with a short script that hits `POST /api/v1/auth/register` then
`POST /api/v1/auth/login` for N synthetic users against a **staging** database.
Provide the path via `-e TOKENS_FILE=tokens.txt`. Supply at least as many tokens
as the scenario's `vus`.

### `callbacks.jsonl` — one pre-signed gateway callback per line

For `phase21_payment_callback_spike.js`. Each line is the raw callback body for a
distinct paid order. If the gateway signs via a header, prefix the line with
`<header-name>:<value>\t` (tab-separated) — the script splits on the first tab
and sets that header:

```
X-Callback-Signature:abc123\t{"merchantOrderId":"...","resultCode":"00",...}
{"reference":"...","status":"PAID",...}
```

Bodies must be signed with the **staging** gateway secret so
`processor.ProcessRaw` accepts them. Never use production `TICKET_QR_SECRET` or
gateway secrets in load fixtures.

## Interpreting results

Every script prints post-run SQL assertions in `handleSummary`. The load test
**passes** only when both the k6 thresholds are green *and* the SQL invariant
holds — e.g. `count(active reservations) <= capacity` and `one PAID payment +
one ticket per order`. Watch `/api/v1/admin/warroom` and the Grafana war-day
dashboard live during the run.

## Targets (Phase 21 acceptance)

- No oversold inventory.
- No double payment.
- No mass queue reset.
- API p95 within target under load (checkout p95 < 1.5s; queue status p95 < 400ms).
- Incident runbook valid — see [`docs/INCIDENT_RUNBOOK.md`](../../docs/INCIDENT_RUNBOOK.md).
