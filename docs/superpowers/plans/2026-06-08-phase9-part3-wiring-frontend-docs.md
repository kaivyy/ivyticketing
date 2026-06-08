# Phase 9 Plan — Part 3: Endpoints, Wiring, Frontend, Docs

> Part of the Phase 9 implementation plan. Index: [2026-06-08-phase9-antibot-abuse.md](2026-06-08-phase9-antibot-abuse.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** Assumes Parts 1-2 exist. This part exposes super-admin endpoints, wires the guard into server.go (queue-join/auth/checkout), adds the frontend Turnstile widget, and finishes with tests + docs. Never guard the webhook port.

---

## Task 16: Super-admin handler + routes + /security/config

**Files:**
- Create: `services/api/internal/modules/abuse/handler.go`
- Create: `services/api/internal/modules/abuse/routes.go`
- Create: `services/api/internal/modules/abuse/securityconfig.go`

- [ ] **Step 1: Implement handler.go**

Read `services/api/internal/modules/payments/handler.go` and `tickets/handler.go` for the apperr/authctx/chi patterns. Then:
```go
package abuse

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func actor(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func pageParams(r *http.Request) (int32, int32) {
	limit := int32(50)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	return limit, offset
}

func (h *Handler) ListSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListSettings(r.Context())
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) SetSetting(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	var req SettingDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.SetSetting(r.Context(), uid, req.Key, req.Value); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Block(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	var req BlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.Block(r.Context(), uid, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Unblock(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	var req UnblockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.Unblock(r.Context(), uid, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListBlocked(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	out, err := h.svc.ListBlocked(r.Context(), limit, offset)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) ListLog(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	out, err := h.svc.ListAbuseLog(r.Context(), limit, offset)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) ListIPRules(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListIPRules(r.Context())
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) AddIPRule(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	var req IPRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.AddIPRule(r.Context(), uid, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteIPRule(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "ruleId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_RULE_ID", "invalid rule id"))
		return
	}
	if err := h.svc.DeleteIPRule(r.Context(), uid, id); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Implement securityconfig.go** (public endpoint, no auth)

```go
package abuse

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// SecurityConfigHandler serves the public client config (turnstile on/off + site key).
type SecurityConfigHandler struct{ svc *Service }

func NewSecurityConfigHandler(svc *Service) *SecurityConfigHandler {
	return &SecurityConfigHandler{svc: svc}
}

func (h *SecurityConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	apperr.WriteJSON(w, http.StatusOK, h.svc.SecurityConfig())
}
```

- [ ] **Step 3: Implement routes.go**

```go
package abuse

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterAdminRoutes mounts super-admin abuse endpoints (RequirePlatformAdmin applied upstream).
func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Route("/abuse", func(r chi.Router) {
		r.Get("/settings", h.ListSettings)
		r.Put("/settings", h.SetSetting)
		r.Post("/block", h.Block)
		r.Post("/unblock", h.Unblock)
		r.Get("/blocked", h.ListBlocked)
		r.Get("/log", h.ListLog)
		r.Get("/ip-rules", h.ListIPRules)
		r.Post("/ip-rules", h.AddIPRule)
		r.Delete("/ip-rules/{ruleId}", h.DeleteIPRule)
	})
}

var _ = middleware.RequirePlatformAdmin // applied by server.go
```

> Remove the `var _ =` line; it's just a reminder that the platform-admin guard is applied in server.go where the route group is mounted.

- [ ] **Step 4: Build**

```bash
cd services/api && go build ./internal/modules/abuse/...; cd ../..
```
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/abuse/handler.go services/api/internal/modules/abuse/routes.go services/api/internal/modules/abuse/securityconfig.go
git commit -m "feat(phase9): abuse admin handler/routes + security config endpoint"
```

---

## Task 17: Wire abuse into server.go

**Files:**
- Modify: `services/api/internal/app/server.go`
- Modify: `services/api/internal/modules/queue/routes.go`
- Modify: `services/api/internal/modules/auth/routes.go`

- [ ] **Step 1: Build the abuse stack in server.go**

Read server.go fully first. After the queue wiring (queueSvc exists) and using the existing `redisClient`, `pool`, `auditLog`, `cfg`, add:
```go
	// Anti-bot / abuse (Phase 9)
	abuseRepo := abusemod.NewRepository(pool)
	abuseSettings := abusemod.NewSettings(abuseRepo)
	_ = abuseSettings.Refresh(context.Background()) // initial load; ignore error (fail-safe defaults)
	abuseSettings.StartRefresh(context.Background(), cfg.AbuseSettingsRefresh)
	rateLimiter := ratelimit.New(redisClient)
	abuseRate := abusemod.NewRateChecker(rateLimiter)
	abuseBlocklist := abusemod.NewBlocklist(abuseRepo)
	abuseReputation := abusemod.NewReputation(abuseRepo, cfg.ReputationChallengeThreshold, cfg.ReputationDenyThreshold)
	var captchaVerifier captcha.Verifier = captcha.NewTurnstile(cfg.TurnstileSecret)
	abuseSvc := abusemod.NewService(abuseRepo, abuseSettings, auditLog, cfg.MaxActiveQueuePerUser, cfg.TurnstileSiteKey)
	abuseGuard := abusemod.NewGuard(abuseSettings, abuseBlocklist, abuseRate, abuseReputation, captchaVerifier, abuseSvc, abuseSvc)
	abuseHandler := abusemod.NewHandler(abuseSvc)
	securityConfigHandler := abusemod.NewSecurityConfigHandler(abuseSvc)
```
Add imports:
```go
	"context"
	abusemod "github.com/varin/ivyticketing/services/api/internal/modules/abuse"
	"github.com/varin/ivyticketing/services/api/internal/platform/captcha"
	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
```
(`context` may already be imported — check.)

- [ ] **Step 2: Mount /security/config (public, no auth)**

In the `/api/v1` route group, alongside the public read-only routes (where `publicHandler.RegisterRoutes(r)` is), add:
```go
		r.Get("/security/config", securityConfigHandler.Get)
```

- [ ] **Step 3: Mount super-admin abuse routes (authn + RequirePlatformAdmin)**

Inside the authn group (`r.Group(func(r chi.Router) { r.Use(middleware.Authn(signer)) ...`), add an admin subgroup:
```go
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequirePlatformAdmin())
				r.Route("/admin", func(r chi.Router) {
					abuseHandler.RegisterAdminRoutes(r)
				})
			})
```
Final paths: `/api/v1/admin/abuse/*`.

- [ ] **Step 4: Guard queue-join, checkout, auth login/register**

**Queue join:** the queue routes currently apply `EntryGuard`. Change `queue/routes.go` `RegisterRoutes` to accept a join middleware:
```go
func (h *Handler) RegisterRoutes(r chi.Router, joinGuard func(http.Handler) http.Handler) {
	r.With(joinGuard).Post("/events/{eventId}/queue/join", h.Join)
	r.Get("/events/{eventId}/queue/status", h.Status)
}
```
Add `"net/http"` import; remove the `EntryGuard` usage (the no-op `guard.go` can stay unused or be deleted — delete its usage here). In server.go where `queueHandler.RegisterRoutes(r)` is called, pass the guard:
```go
			queueHandler.RegisterRoutes(r, abuseGuard.Middleware(abusemod.CategoryQueueJoin))
```

**Checkout:** the checkout route is registered via `ordersHandler.RegisterEventRoutes` (within the org/event group). Wrap just the checkout POST. Simplest: in server.go, the orders event routes are mounted in a group; add the abuse middleware to the checkout route specifically. Since orders registers its own routes, pass an optional guard: modify `ordersHandler.RegisterEventRoutes` to accept a `checkoutGuard func(http.Handler) http.Handler` and apply it to the checkout POST only. Read orders/routes.go to find the checkout registration and wrap it:
```go
// orders/routes.go RegisterEventRoutes(..., checkoutGuard func(http.Handler) http.Handler)
r.With(checkoutGuard).Post("/categories/{categoryId}/checkout", h.Checkout)
```
In server.go pass `abuseGuard.Middleware(abusemod.CategoryCheckout)`.

**Auth login/register:** `auth.RegisterRoutes(r, signer)` self-mounts. Add optional guards:
```go
// auth/routes.go
func (h *Handler) RegisterRoutes(r chi.Router, signer *security.JWTSigner, loginGuard, registerGuard func(http.Handler) http.Handler) {
	r.Route("/auth", func(r chi.Router) {
		r.With(registerGuard).Post("/register", h.Register)
		r.With(loginGuard).Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
		r.With(middleware.Authn(signer)).Get("/me", h.Me)
	})
}
```
In server.go: `authHandler.RegisterRoutes(r, signer, abuseGuard.Middleware(abusemod.CategoryAuthLogin), abuseGuard.Middleware(abusemod.CategoryAuthRegister))`.

> Auth login/register run BEFORE authn (no user in context) — the guard uses IP-only (userID empty). That's expected and handled (guard reads userID from authctx only if present).

> NOTE: passing `func(http.Handler) http.Handler` guards as params keeps modules decoupled (queue/orders/auth don't import abuse). A nil guard would panic on `r.With(nil)`; server.go always passes a real guard, so no nil-guard path. If you want nil-safety, wrap: `if g == nil { g = passthrough }` — optional.

- [ ] **Step 5: Confirm webhook untouched**

Verify `cmd/webhook/main.go` builds its own router WITHOUT any abuse guard. Do not modify it. (Acceptance: rate limit must not affect payment callbacks.)

- [ ] **Step 6: Build + test**

```bash
cd services/api && go build ./... && go test ./... -race 2>&1 | tail -15; cd ../..
```
Expected: clean + all green (existing tests still pass; callers of the changed RegisterRoutes signatures updated).

> The signature changes to `queue.RegisterRoutes`, `auth.RegisterRoutes`, `orders.RegisterEventRoutes` may break integration test helpers that call them directly. Grep and fix:
> ```bash
> grep -rn "RegisterRoutes\|RegisterEventRoutes" services/api/tests/ services/api/internal/app/
> ```
> Integration `newTestServer` builds via `app.NewRouter`, so it shouldn't call these directly — but verify.

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/app/server.go services/api/internal/modules/queue/routes.go services/api/internal/modules/auth/routes.go services/api/internal/modules/orders/routes.go
git commit -m "feat(phase9): wire abuse guard (queue/auth/checkout) + admin routes + security config"
```

---

## Task 18: Retire queue EntryGuard stub

**Files:**
- Modify/Delete: `services/api/internal/modules/queue/guard.go`

- [ ] **Step 1: Remove the now-unused EntryGuard**

The Phase 8 `EntryGuard` no-op is replaced by the abuse guard passed into `RegisterRoutes`. Delete `services/api/internal/modules/queue/guard.go` (or empty it). Verify nothing else references `queue.EntryGuard`:
```bash
grep -rn "EntryGuard" services/api/
```
Expected: no references after deletion.

- [ ] **Step 2: Build**

```bash
cd services/api && go build ./...; cd ../..
```
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add -A services/api/internal/modules/queue
git commit -m "refactor(phase9): retire queue EntryGuard stub (replaced by abuse guard)"
```

---

## Task 19: Integration tests

**Files:**
- Create: `services/api/tests/integration/phase9_abuse_test.go`

> Requires `TEST_DATABASE_URL` + Redis. Skip cleanly otherwise. Use existing helpers (`testPool`, `truncate`, `newTestServer`, `registerAndLogin`, `loginCreateOrg`, `postJSON`). Read `helpers_test.go` and a recent `phase8_queue_test.go` for patterns. The super-admin tests need a platform-admin user — check how `IsPlatformAdmin` is set (likely a column on users or a seed; read auth/users to find how to create one, or set it via direct DB update in the test).

- [ ] **Step 1: Write the integration tests**

Cover (build tag `//go:build integration`, package `integration`):
- **TestPhase9_BlockUserViaAdmin** — make a platform-admin token; `POST /api/v1/admin/abuse/block` {subjectType:"user", subjectValue:<uid>}; the blocked user's `POST /events/{id}/queue/join` → 403 USER_BLOCKED; an `abuse_log` row exists.
- **TestPhase9_RateLimitToggle** — ensure `rate_limit_enabled=true`; hammer queue-join past the per-IP limit → 429 RATE_LIMITED; set `rate_limit_enabled=false` via `PUT /admin/abuse/settings`; after settings refresh (call refresh or wait) requests pass.
- **TestPhase9_TurnstileGate** — set `turnstile_enabled=true`; queue-join without `X-Turnstile-Token` → 403 CAPTCHA_REQUIRED. (Use a fake verifier injected via a test-only NewRouter variant, OR assert CAPTCHA_REQUIRED which doesn't need the verifier since missing token short-circuits.)
- **TestPhase9_IPRuleDeny** — `POST /admin/abuse/ip-rules` {cidr:"<test-ip>/32", rule:"deny"}; request from that IP (set `X-Forwarded-For`) → 403.
- **TestPhase9_SuperAdminOnly** — non-admin user hits `/api/v1/admin/abuse/settings` → 403.
- **TestPhase9_QueueCapExceeded** — set `MAX_ACTIVE_QUEUE_PER_USER` low (via test config); user joins N events; (N+1)th join → 429 QUEUE_ENTRY_CAP_EXCEEDED.
- **TestPhase9_WebhookNotRateLimited** — confirm the abuse guard is not on the webhook path: the main API router has no `/webhooks/*` under abuse guard; assert a representative non-guarded behavior (e.g., the webhook binary router is separate — this can be a documented assertion that abuse middleware count on webhook routes is zero, or simply verify `GET /security/config` works without auth and webhook routes are absent from the main router).

> The `newTestServer` helper may need a way to inject a fake captcha verifier or set `MAX_ACTIVE_QUEUE_PER_USER`. If `newTestServer` builds via `app.NewRouter` with a fixed config, add a `newTestServerWithConfig(t, pool, cfgOverrides)` variant OR set platform_settings + config env in the test. Keep changes additive to helpers_test.go.

- [ ] **Step 2: Run**

```bash
cd services/api && go test -tags=integration ./tests/integration/ -run TestPhase9 -v -timeout 90s; cd ../..
```
Expected: PASS (or documented SKIP without DB/Redis).

- [ ] **Step 3: Commit**

```bash
git add services/api/tests/integration/phase9_abuse_test.go services/api/tests/integration/helpers_test.go
git commit -m "test(phase9): integration — block, rate limit toggle, turnstile, ip rule, super-admin, queue cap"
```

---

## Task 20: Concurrency test — rate limiter

**Files:**
- Create: `services/api/tests/integration/phase9_concurrency_test.go`

- [ ] **Step 1: Write the concurrency test**

`//go:build integration`, `-race`. Fire N concurrent requests through `ratelimit.Allow` (or the guarded endpoint) for the same key with limit M; assert exactly M allowed, N-M denied. Use the real Redis limiter (atomic INCR).

```go
//go:build integration

package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
)

func TestPhase9_RateLimiter_Concurrent(t *testing.T) {
	rc := newTestRedis(t) // helper returning *redis.Client or skip
	lim := ratelimit.New(rc)
	ctx := context.Background()
	key := "conc-test-" + t.Name()
	rc.Del(ctx, "ratelimit:"+key)

	const N = 50
	const limit = 10
	var allowed int64
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			ok, _ := lim.Allow(ctx, key, limit, time.Minute)
			if ok {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}
	wg.Wait()
	if allowed != limit {
		t.Fatalf("allowed = %d, want exactly %d", allowed, limit)
	}
	rc.Del(ctx, "ratelimit:"+key)
}
```

> Add `newTestRedis(t)` helper to helpers_test.go if not present (builds `*redis.Client` from `REDIS_TEST_URL`, skips otherwise).

- [ ] **Step 2: Run with -race**

```bash
cd services/api && go test -tags=integration -race ./tests/integration/ -run TestPhase9_RateLimiter -v; cd ../..
```
Expected: PASS — exactly `limit` allowed.

- [ ] **Step 3: Commit**

```bash
git add services/api/tests/integration/phase9_concurrency_test.go services/api/tests/integration/helpers_test.go
git commit -m "test(phase9): concurrent rate limiter exact-limit (-race)"
```

---

## Task 21: Frontend — Turnstile widget

**Files:**
- Create: `apps/web/src/lib/security.ts`
- Create: `apps/web/src/components/security/Turnstile.astro`
- Modify: `apps/web/src/components/queue/WaitingRoom.astro`

- [ ] **Step 1: Implement lib/security.ts**

```ts
const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";

export interface SecurityConfig {
  turnstileEnabled: boolean;
  siteKey?: string;
}

export async function getSecurityConfig(): Promise<SecurityConfig> {
  try {
    const res = await fetch(`${API_URL}/api/v1/security/config`);
    if (!res.ok) return { turnstileEnabled: false };
    return (await res.json()) as SecurityConfig;
  } catch {
    return { turnstileEnabled: false };
  }
}
```

- [ ] **Step 2: Implement Turnstile.astro**

```astro
---
// Renders Cloudflare Turnstile when enabled. Stores the token in a hidden input
// and on window for the join flow to read as X-Turnstile-Token.
---
<div id="turnstile-holder"></div>
<script>
  import { getSecurityConfig } from "../../lib/security";
  const cfg = await getSecurityConfig();
  if (cfg.turnstileEnabled && cfg.siteKey) {
    const s = document.createElement("script");
    s.src = "https://challenges.cloudflare.com/turnstile/v0/api.js";
    s.async = true;
    document.head.appendChild(s);
    const holder = document.getElementById("turnstile-holder")!;
    const widget = document.createElement("div");
    widget.className = "cf-turnstile";
    widget.dataset.sitekey = cfg.siteKey;
    widget.dataset.callback = "ivyTurnstileCb";
    holder.appendChild(widget);
    (window as any).ivyTurnstileCb = (token: string) => {
      (window as any).__ivyTurnstileToken = token;
    };
  }
</script>
```

- [ ] **Step 3: Wire token into WaitingRoom join**

In `WaitingRoom.astro`, the `joinQueue` call must send the token if present. Update `lib/queue.ts` `joinQueue` to read `window.__ivyTurnstileToken` and pass it as a header. Read the current `joinQueue` and `authedFetch`; extend `authedFetch` to accept optional extra headers, OR add the header in `joinQueue`:
```ts
// lib/queue.ts joinQueue — add header
export function joinQueue(eventId: string): Promise<JoinResponse> {
  const token = (window as any).__ivyTurnstileToken as string | undefined;
  return authedFetch<JoinResponse>(`/events/${eventId}/queue/join`, {
    method: "POST",
    headers: token ? { "X-Turnstile-Token": token } : undefined,
  });
}
```
Ensure `authedFetch` merges an optional `headers` option (extend its signature additively; keep existing callers working). Render `<Turnstile />` inside the queue page near the waiting room.

- [ ] **Step 4: Build**

```bash
cd apps/web && npm run build 2>&1 | tail -10; cd ../..
```
Expected: succeeds.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/lib/security.ts apps/web/src/components/security/Turnstile.astro apps/web/src/components/queue/WaitingRoom.astro apps/web/src/lib/queue.ts
git commit -m "feat(phase9): frontend Turnstile widget gated by security config"
```

---

## Task 22: Docs

**Files:**
- Create: `docs/ANTIBOT.md`, `docs/RATE_LIMITING.md`, `docs/ABUSE_OPERATIONS.md`, `docs/PHASE9_DECISIONS.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Write ANTIBOT.md** — guard chain order (blocklist→ratelimit→reputation→turnstile→queue-cap), enforcement points (queue-join/auth/checkout), toggle mechanism (platform_settings runtime), fail-open (rate/captcha/reputation reads) vs fail-safe (blocklist defaults ON), webhook exclusion.

- [ ] **Step 2: Write RATE_LIMITING.md** — per-category limits table (queue_join 10/5, checkout 20/10, auth_login 10/5, auth_register 5/-, default 120/-), Redis fixed-window token bucket, per-IP + per-user keys, fail-open on Redis error, webhook :8090 not rate-limited.

- [ ] **Step 3: Write ABUSE_OPERATIONS.md** — super-admin runbook: list/set settings (toggle features live during incident), block/unblock user/IP, add/remove ip allow-deny rules, read abuse_log, reputation thresholds; **Cloudflare WAF deployment notes** (edge config — out of application code: WAF rules, IP reputation feed, bot fight mode — describe as deployment-layer complement).

- [ ] **Step 4: Write PHASE9_DECISIONS.md** — decisions + tradeoffs: app-layer scope (WAF=edge docs), runtime DB toggle vs env, fail-open rate-limit vs fail-safe blocklist, behavior-score + manual list (no external feed), server-side fingerprint (no client JS), guard via middleware params (modules don't import abuse), webhook isolation.

- [ ] **Step 5: Update CHANGELOG.md** — prepend Phase 9 section (match Phase 7/8 format): abuse module, RequirePlatformAdmin, rate limiter, captcha/Turnstile, settings runtime toggle, blocklist + ip-rules, reputation, guard on queue/auth/checkout, super-admin endpoints, /security/config, frontend Turnstile, migrations 00026-00030, env vars, deferred (Cloudflare WAF edge config, external IP feed, client fingerprint).

- [ ] **Step 6: Commit**

```bash
git add docs/ANTIBOT.md docs/RATE_LIMITING.md docs/ABUSE_OPERATIONS.md docs/PHASE9_DECISIONS.md CHANGELOG.md
git commit -m "docs(phase9): anti-bot, rate limiting, abuse operations, decisions + CHANGELOG"
```

---

## Task 23: Final verification + DoD checklist

**Files:** none (verification).

- [ ] **Step 1: sqlc + vet + build**

```bash
make sqlc && cd services/api && go vet ./... && go build ./...; cd ../..
```
Expected: clean, no diff.

- [ ] **Step 2: Full test suite (race)**

```bash
cd services/api && go test ./... -race 2>&1 | grep -E "^(ok|FAIL|---)" | tail -30; cd ../..
```
Expected: all `ok`, no `FAIL`.

- [ ] **Step 3: Integration (if DB+Redis available)**

```bash
cd services/api && go test -tags=integration -race ./tests/integration/ -run TestPhase9 -v -timeout 120s; cd ../..
```
Expected: PASS or documented SKIP.

- [ ] **Step 4: Migration roundtrip**

```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: 00026-00030 clean.

- [ ] **Step 5: Frontend build**

```bash
cd apps/web && npm run build 2>&1 | tail -10; cd ../..
```
Expected: succeeds.

- [ ] **Step 6: Webhook isolation check**

```bash
grep -rn "abuse\|Guard\|ratelimit" services/api/cmd/webhook/
```
Expected: NO matches — webhook binary has no abuse guard (acceptance criterion).

- [ ] **Step 7: Walk the DoD checklist** (from spec §Definition of Done). Verify each ✅/❌; fix any ❌:
1. Migrations 00026-00030 roundtrip + settings seed.
2. ratelimit Redis token bucket, fail-open, per-category.
3. captcha Verifier + Turnstile + fake; fail-open if enabled-but-no-secret.
4. Settings runtime toggle (refresh + write-through, fail-safe defaults).
5. Guard chain (blocklist→ratelimit→reputation→turnstile→cap), toggle-gated.
6. EntryGuard replaced; queue-join + auth + checkout guarded.
7. Block/unblock + ip-rules + abuse log + reputation via super-admin endpoints.
8. Max active queue entry per user enforced.
9. Blocked user enforcement (403 + abuse_log).
10. Webhook :8090 NOT guarded (Step 6).
11. Frontend Turnstile gated by /security/config; 429/403 handled.
12. Audit (block/unblock/setting/ip-rule) + abuse_log.
13. `go test ./... -race` + integration green; sqlc/vet clean.
14. No Phase 1-8 behavior change; docs + CHANGELOG updated.

- [ ] **Step 8: Finishing the branch**

Invoke `superpowers:finishing-a-development-branch` to decide merge/cleanup.

---

Part 3 complete. **Phase 9 done** when the DoD checklist is all green. Next: Phase 10 (Ballot) reuses the registration foundation + abuse guard on ballot submission; Phase 11 (Invitation/Priority/Community/Corporate + WAITLIST_ONLY) adds gate variants protected by the abuse guard.
