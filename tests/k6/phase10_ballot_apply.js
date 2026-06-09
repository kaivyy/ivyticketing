/**
 * Phase 10 ballot load test — simulates a high-concurrency ballot application burst.
 *
 * Required env vars:
 *   API_URL      base URL, e.g. http://localhost:8080
 *   EVENT_ID     UUID of a seeded test event
 *   CATEGORY_ID  UUID of a ballot-mode category in that event
 *   DRAW_ID      UUID of an OPEN draw for that category
 *   TOKEN        valid participant JWT
 *
 * Expected outcomes:
 *   201  first apply for a VU — success
 *   409  duplicate apply (ErrAlreadyApplied) — expected, not an error
 */

import http from "k6/http"
import { check, sleep } from "k6"
import { Counter } from "k6/metrics"

const duplicates = new Counter("ballot_duplicates")

export const options = {
  stages: [
    { duration: "15s", target: 200 },
    { duration: "30s", target: 2000 },
    { duration: "15s", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(99)<800"],
    http_req_failed: ["rate<0.02"],
  },
}

const BASE = __ENV.API_URL ?? "http://localhost:8080"

export default function () {
  const res = http.post(
    `${BASE}/api/v1/events/${__ENV.EVENT_ID}/categories/${__ENV.CATEGORY_ID}/ballot/apply`,
    JSON.stringify({ draw_id: __ENV.DRAW_ID }),
    {
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${__ENV.TOKEN}`,
      },
      tags: { name: "ballot_apply" },
    }
  )

  check(res, {
    "201 or 409 (already applied)": (r) => r.status === 201 || r.status === 409,
    "not 500": (r) => r.status !== 500,
  })

  if (res.status === 409) duplicates.add(1)

  sleep(0.05)
}
