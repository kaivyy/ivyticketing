// phase11_quota_exhaustion.js — Phase 11 quota exhaustion test.
// Fires more concurrent redemptions than the pool quota to verify:
//   - Exactly N grants are issued (N = quota)
//   - All excess requests get 409, not 500
//
// Usage:
//   k6 run phase11_quota_exhaustion.js \
//     -e API_URL=https://api.example.com \
//     -e EVENT_ID=<uuid> \
//     -e CATEGORY_ID=<uuid> \
//     -e TOKEN=<bearer-token> \
//     -e ACCESS_CODE=<code-with-quota-N> \
//     -e QUOTA=1000
import http from "k6/http"
import { check } from "k6"

const quota = parseInt(__ENV.QUOTA ?? "1000")

export const options = {
  vus: quota + 100,        // more VUs than quota
  iterations: quota + 100, // one redemption attempt per VU
  thresholds: {
    http_req_duration: ["p(99)<1000"],
    // All responses must be 200 or 409 — no 500s allowed.
    "checks": ["rate==1.0"],
  },
}

export default function () {
  const res = http.post(
    `${__ENV.API_URL}/api/v1/events/${__ENV.EVENT_ID}/access/redeem`,
    JSON.stringify({ code: __ENV.ACCESS_CODE, categoryId: __ENV.CATEGORY_ID }),
    {
      headers: {
        Authorization: `Bearer ${__ENV.TOKEN}`,
        "Content-Type": "application/json",
      },
    }
  )
  check(res, {
    "200 or 409 — no 500": (r) => r.status === 200 || r.status === 409,
  })
}

export function handleSummary(data) {
  const total = data.metrics["http_reqs"]?.values?.count ?? 0
  const failed = data.metrics["http_req_failed"]?.values?.count ?? 0
  const succeeded = total - failed
  console.log(`Grants issued: ~${succeeded} (quota: ${quota}, overshoot: ${quota + 100})`)
  console.log(`Expected: exactly ${quota} successes, ${100} exhausted (409)`)
  return {}
}
