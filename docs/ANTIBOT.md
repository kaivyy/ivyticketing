# Anti-Bot System

## Guard Chain

Every protected entry point runs the following chain in order. A rejection at any step short-circuits the rest.

```
Request
  └── 1. Blocklist check       (fail-safe)
  └── 2. Rate limit            (fail-open)
  └── 3. Reputation score      (fail-safe)
  └── 4. Turnstile captcha     (fail-open)
  └── 5. Queue cap             (fail-safe)
  └── Handler
```

### Step details

1. **Blocklist** — checks `blocked_subjects` for the caller's IP and user ID. A match returns 403 immediately. Fail-safe: if the DB is unreachable, the request is blocked.

2. **Rate limit** — token-bucket check via Redis. Enforces per-IP and per-user limits by category. Fail-open: a Redis error lets the request through rather than blocking legitimate users during an infra incident. See `docs/RATE_LIMITING.md` for limit table.

3. **Reputation score** — looks up `ip_reputation.score` for the caller's IP. Score ≥ `REPUTATION_CHALLENGE_THRESHOLD` (default 10) triggers a captcha challenge. Score ≥ `REPUTATION_DENY_THRESHOLD` (default 25) returns 403. Fail-safe: if the score cannot be read, the request is blocked.

4. **Turnstile captcha** — verifies the `CF-Turnstile-Response` token against Cloudflare's siteverify API. Only invoked when reputation score is in the challenge band or the platform setting `captcha_enabled` is true globally. Fail-open: if the Cloudflare API is unreachable, the token is treated as valid.

5. **Queue cap** — enforces `MAX_ACTIVE_QUEUE_PER_USER` (default 3). A user already holding that many active queue tokens cannot join another queue. Fail-safe: if the count cannot be read, the request is blocked.

---

## Enforcement Points

| Entry point | Guard applied |
|---|---|
| `POST /events/{eventId}/queue/join` | Full chain (all 5 steps) |
| `POST /api/v1/auth/login` | Steps 1–3 (no queue cap) |
| `POST /api/v1/auth/register` | Steps 1–3 (no queue cap) |
| `POST …/categories/{categoryId}/checkout` | Steps 1–3 + queue cap |

---

## Toggle Mechanism

All feature flags are stored in the `platform_settings` table (key/value rows). The abuse module reads these at startup and refreshes them on a background ticker whose interval is controlled by the `ABUSE_SETTINGS_REFRESH` environment variable (default `30s`).

Supported keys:

| Key | Type | Default | Effect |
|---|---|---|---|
| `blocklist_enabled` | bool | `true` | Enable/disable blocklist step |
| `ratelimit_enabled` | bool | `true` | Enable/disable rate limit step |
| `reputation_enabled` | bool | `true` | Enable/disable reputation step |
| `captcha_enabled` | bool | `true` | Enable/disable Turnstile step |
| `queue_cap_enabled` | bool | `true` | Enable/disable queue cap step |

Changes written via `PUT /api/v1/admin/abuse/settings` take effect within one refresh interval without a redeploy.

---

## Fail-Open vs Fail-Safe

| Guard step | Behavior on infra failure | Rationale |
|---|---|---|
| Blocklist | **Fail-safe** (block) | A missed DB hit should not accidentally unblock a banned actor |
| Rate limit | **Fail-open** (allow) | Redis downtime during a traffic spike must not lock out legitimate users |
| Reputation | **Fail-safe** (block) | Same as blocklist — unknown score is treated as high risk |
| Turnstile | **Fail-open** (allow) | Cloudflare outage should not prevent ticket purchases |
| Queue cap | **Fail-safe** (block) | Cannot safely allow an unknown number of queue slots to open |

---

## Webhook Exclusion

The payment webhook binary runs on port `:8090` (`services/api/cmd/webhook`). It has **no abuse guard middleware** applied. Payment callbacks from Duitku/Xendit must never be rate-limited or captcha-gated. The webhook binary is network-isolated at the infrastructure layer (not exposed to the public internet).
