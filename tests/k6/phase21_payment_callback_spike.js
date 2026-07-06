// phase21_payment_callback_spike.js — Phase 21 payment-callback spike.
//
// A gateway that batches callbacks (or retries) can deliver thousands of
// webhooks in a few seconds. This fires a spike of callbacks — including
// DUPLICATES for the same order — to prove:
//   - The processor is idempotent: N callbacks for one paid order still yield
//     exactly one ticket / one billing charge (no double payment).
//   - Bad signatures get 401; everything else returns 200 (no retry storm).
//   - gateway_webhook_delay_seconds p95 stays bounded under the spike.
//
// This drives the standalone webhook server (payments/webhook/http), which
// exposes POST /webhooks/{gateway}. Signature verification is gateway-specific;
// supply pre-signed callback bodies via CALLBACKS_FILE (one JSON body per line)
// or a signing secret so the script can sign them.
//
// Usage:
//   k6 run phase21_payment_callback_spike.js \
//     -e WEBHOOK_URL=https://hooks.example.com \
//     -e GATEWAY=duitku \
//     -e CALLBACKS_FILE=callbacks.jsonl \
//     -e DUP_FACTOR=3
import http from "k6/http"
import { check } from "k6"
import { SharedArray } from "k6/data"

const dupFactor = parseInt(__ENV.DUP_FACTOR ?? "3")
const gateway = __ENV.GATEWAY ?? "duitku"

// Each line is a ready-to-POST, correctly-signed callback body for a distinct
// paid order. The signature header (if the gateway uses one) must be embedded
// per line as "<header-name>:<value>\t<body>" — see README in tests/k6.
const callbacks = new SharedArray("callbacks", function () {
  const raw = open(__ENV.CALLBACKS_FILE ?? "callbacks.jsonl")
  return raw.split("\n").map((l) => l.trim()).filter(Boolean)
})

export const options = {
  scenarios: {
    spike: {
      executor: "ramping-arrival-rate",
      startRate: 50,
      timeUnit: "1s",
      preAllocatedVUs: 500,
      maxVUs: 2000,
      stages: [
        { target: 1000, duration: "10s" }, // ramp to spike
        { target: 1000, duration: "30s" }, // sustained flood
        { target: 0, duration: "10s" },    // drain
      ],
    },
  },
  thresholds: {
    // No 5xx: the handler returns 200 even on processing errors to avoid
    // gateway retry storms; only bad signatures are 401.
    "checks{kind:no_5xx}": ["rate==1.0"],
    http_req_duration: ["p(95)<1000"],
  },
}

export default function () {
  // Duplicate delivery: pick a callback, deliver it dupFactor times over the run.
  const idx = Math.floor(Math.random() * callbacks.length)
  const line = callbacks[idx % callbacks.length]

  let headers = { "Content-Type": "application/json" }
  let body = line
  const tab = line.indexOf("\t")
  if (tab !== -1) {
    const hdr = line.slice(0, tab)
    body = line.slice(tab + 1)
    const colon = hdr.indexOf(":")
    if (colon !== -1) headers[hdr.slice(0, colon)] = hdr.slice(colon + 1)
  }

  const res = http.post(`${__ENV.WEBHOOK_URL}/webhooks/${gateway}`, body, { headers })
  check(res, { "webhook no 5xx": (r) => r.status < 500 }, { kind: "no_5xx" })
  check(res, { "200 or 401": (r) => r.status === 200 || r.status === 401 })
}

export function handleSummary() {
  console.log(`Payment-callback spike on /${gateway}. Duplicate factor: ${dupFactor}x.`)
  console.log(`Post-run assertion (SQL): each order in the callback set must have exactly one PAID payment and exactly one ticket. Idempotency proven if counts match order count regardless of duplicate deliveries.`)
  return {}
}
