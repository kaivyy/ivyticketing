// phase11_redemption_load.js — Phase 11 redemption burst load test.
// Ramps to 10k concurrent VUs hammering the redeem endpoint.
//
// Usage:
//   k6 run phase11_redemption_load.js \
//     -e API_URL=https://api.example.com \
//     -e EVENT_ID=<uuid> \
//     -e CATEGORY_ID=<uuid> \
//     -e TOKEN=<bearer-token> \
//     -e ACCESS_CODE=<code-with-quota-1000>
import http from "k6/http"
import { check, sleep } from "k6"

export const options = {
  stages: [
    { duration: "30s", target: 500 },
    { duration: "60s", target: 10000 },
    { duration: "30s", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(99)<1000"],
    http_req_failed: ["rate<0.05"],
  },
}

export default function () {
  const eventId = __ENV.EVENT_ID
  const token = __ENV.TOKEN
  const code = __ENV.ACCESS_CODE // a code with quota=1000, max_uses=1000

  const res = http.post(
    `${__ENV.API_URL}/api/v1/events/${eventId}/access/redeem`,
    JSON.stringify({ code, categoryId: __ENV.CATEGORY_ID }),
    {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
    }
  )

  check(res, {
    "200 (grant issued) or 409 (exhausted/already)": (r) =>
      r.status === 200 || r.status === 409,
    "not 500": (r) => r.status !== 500,
  })
  sleep(0.1)
}
