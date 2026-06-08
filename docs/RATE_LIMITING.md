# Rate Limiting

## Per-Category Limits

| Category | Per-IP / min | Per-User / min |
|---|---|---|
| `queue_join` | 10 | 5 |
| `checkout` | 20 | 10 |
| `auth_login` | 10 | 5 |
| `auth_register` | 5 | — |
| `default` | 120 | — |

Per-user limits apply only when the caller is authenticated. Unauthenticated requests are checked against per-IP limits only.

---

## Algorithm

Redis fixed-window token bucket using `INCR` + `EXPIRE`:

1. Compute the key (see format below).
2. `INCR key` — atomically increment the counter.
3. If the returned value is `1` (first hit in the window), call `EXPIRE key 60` to start the 60-second window.
4. If the returned value exceeds the configured limit, return `429 Too Many Requests`.

The window resets hard at the 60-second boundary. There is no sliding window or token replenishment within a window.

---

## Key Format

```
ratelimit:{category}:ip:{ip}
ratelimit:{category}:user:{userID}
```

Examples:

```
ratelimit:queue_join:ip:203.0.113.42
ratelimit:queue_join:user:01234567-89ab-cdef-0123-456789abcdef
ratelimit:auth_login:ip:203.0.113.42
ratelimit:default:ip:198.51.100.7
```

Keys are scoped per category so limits are independent across entry points.

---

## Fail-Open Behavior

If the Redis call fails (connection error, timeout, etc.) the rate limit check is skipped and the request is allowed through. This ensures a Redis outage during a high-traffic event does not lock out legitimate users. The failure is logged at `WARN` level for observability.

---

## Webhook Exclusion

The payment webhook binary on port `:8090` does not pass through any rate limit middleware. Payment callbacks from Duitku and Xendit must always be processed regardless of traffic volume.
