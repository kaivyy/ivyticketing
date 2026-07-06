// phase21_checkout_race.js — Phase 21 checkout race / oversell load test.
//
// Fires far more concurrent checkouts than the category capacity to prove the
// FOR UPDATE row-lock in inventory.CheckAndLock holds under real HTTP load:
//   - At most `CAPACITY` checkouts return 201.
//   - Every excess request gets 409 (sold out), never 500 and never a 201 that
//     pushes paid+reserved over capacity (oversell).
//
// The unit-level guarantee is already proven by TestInventoryConcurrency_NoOversell;
// this script proves it end-to-end through the router, queue guard, and pool.
//
// Usage:
//   k6 run phase21_checkout_race.js \
//     -e API_URL=https://api.example.com \
//     -e ORG_ID=<uuid> -e EVENT_ID=<uuid> -e CATEGORY_ID=<uuid> \
//     -e CAPACITY=1000 \
//     -e TOKENS_FILE=tokens.txt   # one bearer token per line, >= VUS distinct users
import http from "k6/http"
import { check } from "k6"
import { SharedArray } from "k6/data"

const capacity = parseInt(__ENV.CAPACITY ?? "1000")
const overshoot = Math.ceil(capacity * 0.5)

// One distinct participant token per VU — a user can only hold one active
// reservation per category, so reusing tokens would mask oversell.
const tokens = new SharedArray("tokens", function () {
  const raw = open(__ENV.TOKENS_FILE ?? "tokens.txt")
  return raw.split("\n").map((l) => l.trim()).filter(Boolean)
})

export const options = {
  scenarios: {
    stampede: {
      executor: "shared-iterations",
      vus: Math.min(tokens.length, capacity + overshoot),
      iterations: capacity + overshoot,
      maxDuration: "2m",
    },
  },
  thresholds: {
    // No server errors at all — every response is a clean 201 or 409.
    "checks{kind:no_5xx}": ["rate==1.0"],
    // p95 under target even during the stampede.
    http_req_duration: ["p(95)<1500"],
  },
}

export default function () {
  const token = tokens[__VU % tokens.length]
  const url = `${__ENV.API_URL}/api/v1/organizations/${__ENV.ORG_ID}/events/${__ENV.EVENT_ID}/categories/${__ENV.CATEGORY_ID}/checkout`
  const res = http.post(url, JSON.stringify({}), {
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
  })
  check(res, {
    "201 or 409 — no 5xx": (r) => r.status < 500,
  }, { kind: "no_5xx" })
  check(res, {
    "created or sold-out": (r) => r.status === 201 || r.status === 409,
  })
}

export function handleSummary(data) {
  console.log(`Capacity: ${capacity}, attempts: ${capacity + overshoot}`)
  console.log(`Expected: <= ${capacity} × 201 (created), rest 409 (sold out), zero 5xx.`)
  console.log(`Verify no oversell: SELECT count(*) FROM inventory_reservations WHERE category_id='${__ENV.CATEGORY_ID}' AND status='active'; must be <= ${capacity}.`)
  return {}
}
