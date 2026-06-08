# Abuse Operations Runbook

All endpoints in this runbook require the `RequirePlatformAdmin` middleware. Super-admin status is a platform-level flag on the `users` table, not an org role.

---

## Toggle Features Live

Use this during an active incident to disable a guard step without redeploying.

```
PUT /api/v1/admin/abuse/settings
Content-Type: application/json

{
  "key": "ratelimit_enabled",
  "value": "false"
}
```

Valid keys: `blocklist_enabled`, `ratelimit_enabled`, `reputation_enabled`, `captcha_enabled`, `queue_cap_enabled`.

Changes are stored in `platform_settings` and picked up by all API instances within one `ABUSE_SETTINGS_REFRESH` interval (default 30s). No redeploy required.

To read current settings:

```
GET /api/v1/admin/abuse/settings
```

---

## Block / Unblock a Subject

Block a user ID or IP address:

```
POST /api/v1/admin/abuse/block
Content-Type: application/json

{
  "subject_type": "user",   // "user" or "ip"
  "subject_id": "01234567-89ab-cdef-0123-456789abcdef",
  "reason": "scalper bot confirmed"
}
```

Unblock:

```
POST /api/v1/admin/abuse/unblock
Content-Type: application/json

{
  "subject_type": "ip",
  "subject_id": "203.0.113.42"
}
```

Blocked entries are written to `blocked_subjects`. The blocklist check is fail-safe: if the table is unreachable, the request is blocked.

---

## Add / Remove IP Allow and Deny Rules

IP rules provide coarser control than the per-subject blocklist. **Allow rules win** over deny rules for the same IP or CIDR.

Add a rule:

```
POST /api/v1/admin/abuse/ip-rules
Content-Type: application/json

{
  "cidr": "203.0.113.0/24",
  "rule_type": "deny",        // "allow" or "deny"
  "note": "known scraper ASN"
}
```

Remove a rule:

```
DELETE /api/v1/admin/abuse/ip-rules/{ruleId}
```

List rules:

```
GET /api/v1/admin/abuse/ip-rules
```

Rules are stored in `ip_rules`. Precedence: explicit allow > explicit deny > default (pass through to rest of guard chain).

---

## Read Abuse Log

```
GET /api/v1/admin/abuse/log?limit=100&offset=0&subject_type=ip&subject_id=203.0.113.42
```

Query parameters (all optional):

| Param | Description |
|---|---|
| `limit` | Max rows returned (default 50, max 500) |
| `offset` | Pagination offset |
| `subject_type` | Filter by `user` or `ip` |
| `subject_id` | Filter by specific subject |
| `action` | Filter by action type (e.g. `block`, `rate_limit_hit`, `captcha_fail`) |
| `since` | RFC3339 timestamp lower bound |

Abuse log entries are append-only writes to `abuse_log`. They are never deleted by application code.

---

## Reputation Thresholds

Reputation scores are tracked per IP in `ip_reputation`. Scores increase on abuse signals (failed captcha, rate limit hits, blocked attempts) and decay over time.

| Threshold | Env var | Default | Effect |
|---|---|---|---|
| Challenge | `REPUTATION_CHALLENGE_THRESHOLD` | `10` | Score ≥ threshold → require Turnstile captcha |
| Deny | `REPUTATION_DENY_THRESHOLD` | `25` | Score ≥ threshold → block request (403) |

To manually reset a reputation score, delete or update the row in `ip_reputation` directly, or use:

```
POST /api/v1/admin/abuse/reputation/reset
Content-Type: application/json

{ "ip": "203.0.113.42" }
```

---

## Cloudflare WAF Notes

Cloudflare WAF is an edge-layer complement to the application guard chain — it is not part of the application code. The following are deployment-layer configurations managed outside this repository:

- **WAF rules**: block known bad user agents, enforce HTTP method allow-list per path prefix.
- **IP reputation feed**: Cloudflare's managed IP threat intelligence blocks known malicious IPs before they reach the origin.
- **Bot Fight Mode**: challenges automated traffic at the edge based on Cloudflare's bot scoring.

These controls reduce load on the application guard chain but are not required for the application to function correctly. The app-layer guard is designed to operate safely without any edge WAF in place.
