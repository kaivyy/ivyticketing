import http from "k6/http";
import { check, sleep } from "k6";

const BASE = __ENV.API_URL || "http://localhost:8080";
const EVENT_ID = __ENV.EVENT_ID;
const TOKEN = __ENV.ACCESS_TOKEN;

export const options = {
  stages: [
    { duration: "1m", target: 10000 },
    { duration: "2m", target: 50000 },
    { duration: "2m", target: 100000 },
    { duration: "1m", target: 0 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<2000"],
  },
};

export default function () {
  const headers = {
    Authorization: `Bearer ${TOKEN}`,
    "Content-Type": "application/json",
  };

  // Join queue (idempotent — safe to call repeatedly)
  http.post(`${BASE}/api/v1/events/${EVENT_ID}/queue/join`, null, { headers });

  // Poll status
  const res = http.get(`${BASE}/api/v1/events/${EVENT_ID}/queue/status`, { headers });
  check(res, { "status 200": (r) => r.status === 200 });

  sleep(4);
}
