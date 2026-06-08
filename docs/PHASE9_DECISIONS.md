# Phase 9 Architecture Decisions

---

## Decision 1: Application-layer scope, not WAF-only

**Choice:** Implement the abuse guard chain inside the Go application rather than relying solely on a WAF.

**Rationale:** WAF is an edge-layer configuration tied to a specific deployment provider (Cloudflare). The application guard catches what the WAF misses — requests that pass edge inspection but exhibit abuse patterns visible only after authentication or business-logic context (e.g., a logged-in user joining 50 queues). It is also portable: the guard works identically in local dev, staging, and any cloud provider.

WAF rules remain a recommended deployment-layer complement (see `docs/ABUSE_OPERATIONS.md`), but the application is not dependent on them.

---

## Decision 2: Runtime DB toggle vs env-var flags

**Choice:** Feature flags for the guard chain are stored in the `platform_settings` table and refreshed on a ticker, not read from environment variables at startup.

**Rationale:** During a live incident (e.g., a flash sale with unexpected bot traffic), operators need to disable or re-enable individual guard steps within seconds. An env-var change requires a redeploy, which takes minutes and causes request drops. A DB write takes effect across all instances within one refresh interval (default 30s) with no service interruption.

The tradeoff is a dependency on DB availability for settings reads, mitigated by in-process caching with safe defaults (all guards on).

---

## Decision 3: Fail-open rate limit and captcha vs fail-safe blocklist and reputation

**Choice:** Redis errors skip the rate limit check (fail-open). Cloudflare API errors treat the captcha token as valid (fail-open). DB errors on blocklist and reputation score reads block the request (fail-safe).

**Rationale:** Rate limiting and captcha are friction controls — their purpose is to slow abuse, not to be an absolute gate. A Redis outage during a high-traffic sale should not prevent legitimate users from buying tickets. The cost of being fail-open here is accepting some extra traffic during an infra incident.

Blocklist and reputation are explicit deny controls. A user or IP that has been manually blocked must not slip through because the DB had a momentary hiccup. The cost of being fail-safe is that a DB outage temporarily blocks all traffic at those steps, which is the safer failure mode for a security control.

---

## Decision 4: Behavior-score + manual list, no external feed

**Choice:** IP reputation is computed from in-application signals (rate limit hits, captcha failures, block events) and stored in `ip_reputation`. No external IP threat feed is integrated.

**Rationale:** External feeds (e.g., AbuseIPDB, Cloudflare threat intel API) introduce an external dependency, a per-lookup latency budget, and a vendor contract. For an event ticketing platform, the most relevant signals are behavior on this platform specifically — a generic threat feed would flag many false positives (shared NAT IPs, university networks, etc.) and miss platform-specific patterns.

Manual blocklist via `blocked_subjects` covers confirmed bad actors. Behavior-score covers emerging patterns. This is simpler to operate and reason about.

External feeds remain a future option if the manual + behavior model proves insufficient at scale.

---

## Decision 5: Server-side fingerprint only

**Choice:** Bot fingerprinting uses a server-side hash of `User-Agent + IP + Accept-Language`. No client-side JavaScript fingerprinting (canvas, WebGL, font enumeration, etc.).

**Rationale:** Client-side JS fingerprinting adds frontend complexity, requires a script load on every page, and is increasingly blocked by privacy-focused browsers and extensions. It also introduces a dependency on the client executing JS correctly, which is not guaranteed in all environments.

The server-side hash is lightweight, requires no client cooperation, and is sufficient to correlate requests from the same source across a session. It is not a strong anti-bot signal on its own — it is used as a correlation key, not a trust signal.

---

## Decision 6: Guard injected via middleware params, not imported by modules

**Choice:** The abuse guard is a middleware function passed into the queue, orders, and auth route constructors. Those modules do not import the `abuse` package directly.

**Rationale:** Dependency inversion keeps the business modules (queue, orders, auth) free of abuse-system knowledge. The guard interface is a simple function signature. This makes the modules testable without a real abuse stack and allows the guard to be swapped (e.g., replaced with a no-op in tests) without modifying module code.

This follows the same pattern used for `RegistrationGate` in Phase 8.

---

## Decision 7: Webhook binary isolation

**Choice:** The payment webhook binary on port `:8090` has no abuse guard middleware. It is a separate binary from the main API.

**Rationale:** Payment callbacks from Duitku and Xendit are server-to-server calls that must always be processed. Rate limiting or captcha-gating these would cause missed payment confirmations and incorrect order states. The webhook binary is network-isolated at the infrastructure layer (not publicly routable) — the combination of network isolation and signature verification (per Phase 6) is the security model for that surface. Application-layer abuse guards are inappropriate there.

---

## Deferred

- **Cloudflare WAF edge config** — deployment-layer concern, out of application code.
- **External IP reputation feed** — behavior-score model is sufficient for initial scale.
- **Client-side JS fingerprinting** — server-side hash covers current needs; revisit if bot sophistication increases.
