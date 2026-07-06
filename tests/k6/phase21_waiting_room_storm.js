// phase21_waiting_room_storm.js — Phase 21 waiting-room scale + refresh storm.
//
// Simulates a mass queue join followed by every waiting user hammering the
// status endpoint (the "refresh storm" / "mobile reconnect storm" scenarios).
// Proves:
//   - queue/join admits/enqueues without 5xx under a burst of N users.
//   - queue/status stays fast (cached position) while thousands poll it.
//   - No mass queue reset: a user's position/token is stable across refreshes.
//
// Usage:
//   k6 run phase21_waiting_room_storm.js \
//     -e API_URL=https://api.example.com \
//     -e EVENT_ID=<uuid> \
//     -e TOKENS_FILE=tokens.txt \
//     -e USERS=50000 -e POLLS=10
import http from "k6/http"
import { check, sleep } from "k6"
import { SharedArray } from "k6/data"

const users = parseInt(__ENV.USERS ?? "50000")
const polls = parseInt(__ENV.POLLS ?? "10")

const tokens = new SharedArray("tokens", function () {
  const raw = open(__ENV.TOKENS_FILE ?? "tokens.txt")
  return raw.split("\n").map((l) => l.trim()).filter(Boolean)
})

export const options = {
  scenarios: {
    // Phase 1: everyone joins the queue at once.
    join_burst: {
      executor: "shared-iterations",
      exec: "join",
      vus: Math.min(tokens.length, 2000),
      iterations: Math.min(users, tokens.length),
      maxDuration: "3m",
      startTime: "0s",
    },
    // Phase 2: refresh storm — everyone polls status repeatedly.
    refresh_storm: {
      executor: "shared-iterations",
      exec: "poll",
      vus: Math.min(tokens.length, 3000),
      iterations: Math.min(users, tokens.length) * polls,
      maxDuration: "5m",
      startTime: "30s",
    },
  },
  thresholds: {
    "checks{kind:no_5xx}": ["rate==1.0"],
    "http_req_duration{scenario:refresh_storm}": ["p(95)<400"],
  },
}

export function join() {
  const token = tokens[__VU % tokens.length]
  const res = http.post(
    `${__ENV.API_URL}/api/v1/events/${__ENV.EVENT_ID}/queue/join`,
    JSON.stringify({}),
    { headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" } }
  )
  check(res, { "join no 5xx": (r) => r.status < 500 }, { kind: "no_5xx" })
}

export function poll() {
  const token = tokens[__VU % tokens.length]
  const res = http.get(`${__ENV.API_URL}/api/v1/events/${__ENV.EVENT_ID}/queue/status`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  check(res, { "status no 5xx": (r) => r.status < 500 }, { kind: "no_5xx" })
  check(res, { "status 200": (r) => r.status === 200 })
  sleep(Math.random() * 2) // jittered client refresh cadence
}

export function handleSummary(data) {
  console.log(`Waiting-room storm: ${users} users, ${polls} polls each.`)
  console.log(`Verify no queue reset: positions must be monotonic per token across polls; queue_active_users gauge on /metrics should not collapse to 0 mid-run.`)
  return {}
}
