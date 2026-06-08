# Phase 9 Plan — Part 1: Foundation (config, migrations, platform packages)

> Part of the Phase 9 implementation plan. Index: [2026-06-08-phase9-antibot-abuse.md](2026-06-08-phase9-antibot-abuse.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** New files + additive changes only. Do not alter Phase 1-8 behavior. Never guard the webhook port.

---

## Task 1: Config — anti-bot settings

**Files:**
- Modify: `services/api/internal/app/config.go`
- Modify: `services/api/internal/app/config_test.go`
- Modify: `services/api/.env.example`, `.env.example`

- [ ] **Step 1: Write the failing test**

Add to `config_test.go` (success-path tests must set all existing required secrets — include `TICKET_QR_SECRET`):
```go
func TestLoadConfig_AbuseDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("TICKET_QR_SECRET", "qr-secret")
	t.Setenv("MAX_ACTIVE_QUEUE_PER_USER", "")
	t.Setenv("REPUTATION_CHALLENGE_THRESHOLD", "")
	t.Setenv("REPUTATION_DENY_THRESHOLD", "")
	t.Setenv("ABUSE_SETTINGS_REFRESH", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxActiveQueuePerUser != 5 {
		t.Errorf("MaxActiveQueuePerUser = %d, want 5", cfg.MaxActiveQueuePerUser)
	}
	if cfg.ReputationChallengeThreshold != 10 {
		t.Errorf("ReputationChallengeThreshold = %d, want 10", cfg.ReputationChallengeThreshold)
	}
	if cfg.ReputationDenyThreshold != 25 {
		t.Errorf("ReputationDenyThreshold = %d, want 25", cfg.ReputationDenyThreshold)
	}
	if cfg.AbuseSettingsRefresh != 30*time.Second {
		t.Errorf("AbuseSettingsRefresh = %v, want 30s", cfg.AbuseSettingsRefresh)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig_AbuseDefaults -v; cd ../..
```
Expected: FAIL — fields undefined.

- [ ] **Step 3: Add fields + loading**

In `config.go`, add to `Config` struct:
```go
	TurnstileSecret              string
	TurnstileSiteKey             string
	MaxActiveQueuePerUser        int
	ReputationChallengeThreshold int
	ReputationDenyThreshold      int
	AbuseSettingsRefresh         time.Duration
```
In `LoadConfig`, before `return cfg, nil`:
```go
	cfg.TurnstileSecret = os.Getenv("TURNSTILE_SECRET")
	cfg.TurnstileSiteKey = os.Getenv("TURNSTILE_SITE_KEY")

	maxQueue, err := getInt64("MAX_ACTIVE_QUEUE_PER_USER", 5)
	if err != nil {
		return Config{}, err
	}
	repChallenge, err := getInt64("REPUTATION_CHALLENGE_THRESHOLD", 10)
	if err != nil {
		return Config{}, err
	}
	repDeny, err := getInt64("REPUTATION_DENY_THRESHOLD", 25)
	if err != nil {
		return Config{}, err
	}
	abuseRefresh, err := getDuration("ABUSE_SETTINGS_REFRESH", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxActiveQueuePerUser = int(maxQueue)
	cfg.ReputationChallengeThreshold = int(repChallenge)
	cfg.ReputationDenyThreshold = int(repDeny)
	cfg.AbuseSettingsRefresh = abuseRefresh
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig -v; cd ../..
```
Expected: PASS (all config tests).

- [ ] **Step 5: Update .env.example files**

Append to both `services/api/.env.example` and root `.env.example`:
```
# Anti-bot / abuse (Phase 9)
TURNSTILE_SECRET=
TURNSTILE_SITE_KEY=
MAX_ACTIVE_QUEUE_PER_USER=5
REPUTATION_CHALLENGE_THRESHOLD=10
REPUTATION_DENY_THRESHOLD=25
ABUSE_SETTINGS_REFRESH=30s
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/app/config.go services/api/internal/app/config_test.go services/api/.env.example .env.example
git commit -m "feat(phase9): add anti-bot config (turnstile, reputation thresholds, queue cap)"
```

---

## Task 2: RequirePlatformAdmin middleware

**Files:**
- Create: `services/api/internal/platform/middleware/platformadmin.go`
- Create: `services/api/internal/platform/middleware/platformadmin_test.go`

- [ ] **Step 1: Write the failing test**

Read `services/api/internal/platform/middleware/authz_test.go` first for the test harness (how identity is injected into context via `authctx`). Create `platformadmin_test.go`:
```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
)

func TestRequirePlatformAdmin_Allows(t *testing.T) {
	called := false
	h := RequirePlatformAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	ctx := authctx.WithIdentity(req.Context(), authctx.Identity{UserID: uuid.New(), IsPlatformAdmin: true})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("admin should pass: called=%v code=%d", called, rec.Code)
	}
}

func TestRequirePlatformAdmin_DeniesNonAdmin(t *testing.T) {
	h := RequirePlatformAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	}))
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	ctx := authctx.WithIdentity(req.Context(), authctx.Identity{UserID: uuid.New(), IsPlatformAdmin: false})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin should get 403, got %d", rec.Code)
	}
}

func TestRequirePlatformAdmin_DeniesUnauthenticated(t *testing.T) {
	h := RequirePlatformAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	}))
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req) // no identity
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated should get 401, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd services/api && go test ./internal/platform/middleware/ -run TestRequirePlatformAdmin -v; cd ../..
```
Expected: FAIL — `RequirePlatformAdmin` undefined.

- [ ] **Step 3: Implement platformadmin.go**

```go
package middleware

import (
	"net/http"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// RequirePlatformAdmin allows only platform (super) admins. Requires Authn upstream.
func RequirePlatformAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := authctx.FromContext(r.Context())
			if !ok {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
				return
			}
			if !id.IsPlatformAdmin {
				apperr.WriteError(w, r, apperr.New(http.StatusForbidden, "FORBIDDEN", "platform admin only"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd services/api && go test ./internal/platform/middleware/ -run TestRequirePlatformAdmin -v; cd ../..
```
Expected: PASS (all 3).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/platform/middleware/platformadmin.go services/api/internal/platform/middleware/platformadmin_test.go
git commit -m "feat(phase9): RequirePlatformAdmin middleware"
```

---

## Task 3: Migrations — abuse tables + settings seed

**Files:**
- Create: `database/migrations/00026_create_platform_settings.sql`
- Create: `database/migrations/00027_create_blocked_subjects.sql`
- Create: `database/migrations/00028_create_ip_rules.sql`
- Create: `database/migrations/00029_create_abuse_log.sql`
- Create: `database/migrations/00030_create_ip_reputation.sql`

- [ ] **Step 1: Write migrations**

`00026_create_platform_settings.sql`:
```sql
-- +goose Up
CREATE TABLE platform_settings (
    key        text PRIMARY KEY,
    value      text NOT NULL,
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO platform_settings (key, value) VALUES
    ('turnstile_enabled', 'false'),
    ('rate_limit_enabled', 'true'),
    ('ip_reputation_enabled', 'true'),
    ('blocklist_enabled', 'true');

-- +goose Down
DROP TABLE platform_settings;
```

`00027_create_blocked_subjects.sql`:
```sql
-- +goose Up
CREATE TABLE blocked_subjects (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_type  text NOT NULL,
    subject_value text NOT NULL,
    reason        text,
    blocked_by    uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz,
    CONSTRAINT bs_type_check CHECK (subject_type IN ('user','ip')),
    CONSTRAINT bs_unique UNIQUE (subject_type, subject_value)
);
CREATE INDEX idx_blocked_subjects_lookup ON blocked_subjects(subject_type, subject_value);

-- +goose Down
DROP TABLE blocked_subjects;
```

`00028_create_ip_rules.sql`:
```sql
-- +goose Up
CREATE TABLE ip_rules (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cidr       text NOT NULL,
    rule       text NOT NULL,
    note       text,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ir_rule_check CHECK (rule IN ('allow','deny')),
    CONSTRAINT ir_unique UNIQUE (cidr, rule)
);

-- +goose Down
DROP TABLE ip_rules;
```

`00029_create_abuse_log.sql`:
```sql
-- +goose Up
CREATE TABLE abuse_log (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_type  text,
    subject_value text,
    action        text NOT NULL,
    category      text,
    fingerprint   text,
    ip            text,
    user_id       uuid,
    detail        jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_abuse_log_created ON abuse_log(created_at DESC);
CREATE INDEX idx_abuse_log_subject ON abuse_log(subject_type, subject_value);

-- +goose Down
DROP TABLE abuse_log;
```

`00030_create_ip_reputation.sql`:
```sql
-- +goose Up
CREATE TABLE ip_reputation (
    subject_type  text NOT NULL,
    subject_value text NOT NULL,
    score         integer NOT NULL DEFAULT 0,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (subject_type, subject_value),
    CONSTRAINT rep_type_check CHECK (subject_type IN ('ip','user'))
);

-- +goose Down
DROP TABLE ip_reputation;
```

- [ ] **Step 2: Roundtrip**

```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: clean up/down/up for all five.

- [ ] **Step 3: Commit**

```bash
git add database/migrations/0002{6,7,8,9}_*.sql database/migrations/00030_create_ip_reputation.sql
git commit -m "feat(phase9): abuse migrations (settings, blocklist, ip_rules, abuse_log, reputation)"
```

---

## Task 4: sqlc queries for abuse

**Files:**
- Create: `database/queries/abuse.sql`
- Regenerate: `services/api/internal/db/*`

- [ ] **Step 1: Write queries**

`database/queries/abuse.sql`:
```sql
-- name: ListPlatformSettings :many
SELECT * FROM platform_settings;

-- name: UpsertPlatformSetting :one
INSERT INTO platform_settings (key, value, updated_by, updated_at)
VALUES ($1,$2,$3,now())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: GetBlockedSubject :one
SELECT * FROM blocked_subjects
WHERE subject_type = $1 AND subject_value = $2
  AND (expires_at IS NULL OR expires_at > now());

-- name: ListBlockedSubjects :many
SELECT * FROM blocked_subjects ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: UpsertBlockedSubject :one
INSERT INTO blocked_subjects (subject_type, subject_value, reason, blocked_by, expires_at)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (subject_type, subject_value) DO UPDATE SET reason = EXCLUDED.reason, blocked_by = EXCLUDED.blocked_by, expires_at = EXCLUDED.expires_at, created_at = now()
RETURNING *;

-- name: DeleteBlockedSubject :exec
DELETE FROM blocked_subjects WHERE subject_type = $1 AND subject_value = $2;

-- name: ListIPRules :many
SELECT * FROM ip_rules ORDER BY created_at DESC;

-- name: CreateIPRule :one
INSERT INTO ip_rules (cidr, rule, note, created_by) VALUES ($1,$2,$3,$4) RETURNING *;

-- name: DeleteIPRule :exec
DELETE FROM ip_rules WHERE id = $1;

-- name: InsertAbuseLog :exec
INSERT INTO abuse_log (subject_type, subject_value, action, category, fingerprint, ip, user_id, detail)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8);

-- name: ListAbuseLog :many
SELECT * FROM abuse_log ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: GetReputation :one
SELECT * FROM ip_reputation WHERE subject_type = $1 AND subject_value = $2;

-- name: BumpReputation :one
INSERT INTO ip_reputation (subject_type, subject_value, score, updated_at)
VALUES ($1,$2,$3,now())
ON CONFLICT (subject_type, subject_value) DO UPDATE SET score = ip_reputation.score + EXCLUDED.score, updated_at = now()
RETURNING *;

-- name: CountActiveQueueTokensByUser :one
SELECT count(*) FROM queue_tokens WHERE participant_id = $1 AND status IN ('WAITING','ALLOWED');
```

- [ ] **Step 2: Regenerate + build**

```bash
make sqlc && cd services/api && go build ./internal/db/...; cd ../..
```
Expected: `PlatformSetting`, `BlockedSubject`, `IpRule`, `AbuseLog`, `IpReputation` structs + methods; builds clean.

- [ ] **Step 3: Commit**

```bash
git add database/queries/abuse.sql services/api/internal/db
git commit -m "feat(phase9): abuse sqlc queries"
```

---

## Task 5: platform/ratelimit — Redis token bucket

**Files:**
- Create: `services/api/internal/platform/ratelimit/ratelimit.go`
- Create: `services/api/internal/platform/ratelimit/ratelimit_test.go`

- [ ] **Step 1: Write the failing test** (skips without REDIS_TEST_URL)

```go
package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testClient(t *testing.T) *redis.Client {
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL not set")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	c := redis.NewClient(opt)
	t.Cleanup(func() { c.Close() })
	return c
}

func TestAllow_WithinAndOverLimit(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	l := New(c)
	key := "rl-test-" + t.Name()
	c.Del(ctx, "ratelimit:"+key)

	for i := 0; i < 3; i++ {
		ok, err := l.Allow(ctx, key, 3, time.Minute)
		if err != nil || !ok {
			t.Fatalf("call %d should be allowed: ok=%v err=%v", i, ok, err)
		}
	}
	ok, err := l.Allow(ctx, key, 3, time.Minute)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatal("4th call over limit should be denied")
	}
	c.Del(ctx, "ratelimit:"+key)
}
```

- [ ] **Step 2: Run to verify fail**

```bash
cd services/api && go test ./internal/platform/ratelimit/ -v; cd ../..
```
Expected: FAIL — `New`/`Allow` undefined (or skip without Redis → must still compile-fail until implemented).

- [ ] **Step 3: Implement ratelimit.go**

```go
// Package ratelimit provides a Redis-backed fixed-window token bucket.
// Fail-open: on Redis error, Allow returns true (never block normal users on infra failure).
package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	c *redis.Client
}

func New(c *redis.Client) *Limiter { return &Limiter{c: c} }

// Allow increments the counter for key and returns false once it exceeds limit
// within the window. Fail-open on Redis errors.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if l == nil || l.c == nil {
		return true, nil
	}
	rk := "ratelimit:" + key
	n, err := l.c.Incr(ctx, rk).Result()
	if err != nil {
		return true, nil // fail-open
	}
	if n == 1 {
		_ = l.c.Expire(ctx, rk, window).Err()
	}
	return n <= int64(limit), nil
}
```

- [ ] **Step 4: Run to verify pass**

```bash
REDIS_TEST_URL=redis://localhost:6379 go test ./internal/platform/ratelimit/ -v   # from services/api
```
Expected: PASS if Redis available; SKIP otherwise.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/platform/ratelimit
git commit -m "feat(phase9): redis token-bucket rate limiter (fail-open)"
```

---

## Task 6: platform/captcha — Turnstile verifier + fake

**Files:**
- Create: `services/api/internal/platform/captcha/captcha.go`
- Create: `services/api/internal/platform/captcha/turnstile.go`
- Create: `services/api/internal/platform/captcha/fake.go`
- Create: `services/api/internal/platform/captcha/captcha_test.go`

- [ ] **Step 1: Write the failing test**

```go
package captcha

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFakeVerifier(t *testing.T) {
	if ok, _ := (FakeVerifier{Pass: true}).Verify(context.Background(), "tok", "1.2.3.4"); !ok {
		t.Fatal("fake pass should return true")
	}
	if ok, _ := (FakeVerifier{Pass: false}).Verify(context.Background(), "tok", "1.2.3.4"); ok {
		t.Fatal("fake fail should return false")
	}
}

func TestTurnstileVerify_SuccessAndFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("response") == "good" {
			w.Write([]byte(`{"success":true}`))
		} else {
			w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
		}
	}))
	defer srv.Close()

	v := &Turnstile{Secret: "s", Endpoint: srv.URL, HTTP: srv.Client()}
	if ok, err := v.Verify(context.Background(), "good", "1.2.3.4"); err != nil || !ok {
		t.Fatalf("good token should pass: ok=%v err=%v", ok, err)
	}
	if ok, _ := v.Verify(context.Background(), "bad", "1.2.3.4"); ok {
		t.Fatal("bad token should fail")
	}
}
```

- [ ] **Step 2: Run to verify fail**

```bash
cd services/api && go test ./internal/platform/captcha/ -v; cd ../..
```
Expected: FAIL — types undefined.

- [ ] **Step 3: Implement captcha.go**

```go
// Package captcha verifies CAPTCHA tokens (Cloudflare Turnstile).
package captcha

import "context"

// Verifier validates a CAPTCHA response token.
type Verifier interface {
	Verify(ctx context.Context, token, remoteIP string) (bool, error)
}
```

- [ ] **Step 4: Implement fake.go**

```go
package captcha

import "context"

// FakeVerifier is a test/dev verifier with a fixed outcome.
type FakeVerifier struct{ Pass bool }

func (f FakeVerifier) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	return f.Pass, nil
}
```

- [ ] **Step 5: Implement turnstile.go**

```go
package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTurnstileEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Turnstile is the Cloudflare Turnstile adapter.
type Turnstile struct {
	Secret   string
	Endpoint string       // defaults to Cloudflare siteverify
	HTTP     *http.Client // defaults to a 5s-timeout client
}

func NewTurnstile(secret string) *Turnstile {
	return &Turnstile{Secret: secret, Endpoint: defaultTurnstileEndpoint, HTTP: &http.Client{Timeout: 5 * time.Second}}
}

type siteverifyResp struct {
	Success bool `json:"success"`
}

func (t *Turnstile) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	endpoint := t.Endpoint
	if endpoint == "" {
		endpoint = defaultTurnstileEndpoint
	}
	client := t.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	form := url.Values{}
	form.Set("secret", t.Secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var sv siteverifyResp
	if err := json.NewDecoder(resp.Body).Decode(&sv); err != nil {
		return false, err
	}
	return sv.Success, nil
}
```

- [ ] **Step 6: Run to verify pass**

```bash
cd services/api && go test ./internal/platform/captcha/ -v; cd ../..
```
Expected: PASS (all).

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/platform/captcha
git commit -m "feat(phase9): captcha verifier interface + Turnstile adapter + fake"
```

---

## Task 7: abuse model + clientip + fingerprint

**Files:**
- Create: `services/api/internal/modules/abuse/model.go`
- Create: `services/api/internal/modules/abuse/clientip.go`
- Create: `services/api/internal/modules/abuse/fingerprint.go`
- Create: `services/api/internal/modules/abuse/fingerprint_test.go`

- [ ] **Step 1: Write the failing test**

`fingerprint_test.go`:
```go
package abuse

import (
	"net/http/httptest"
	"testing"
)

func TestFingerprint_Stable(t *testing.T) {
	r1 := httptest.NewRequest("GET", "/", nil)
	r1.Header.Set("User-Agent", "UA/1.0")
	r1.Header.Set("Accept-Language", "en")
	r1.RemoteAddr = "1.2.3.4:5555"

	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("User-Agent", "UA/1.0")
	r2.Header.Set("Accept-Language", "en")
	r2.RemoteAddr = "1.2.3.4:6666" // different port, same IP

	if Fingerprint(r1) != Fingerprint(r2) {
		t.Fatal("fingerprint should be stable across ports (same IP+UA+lang)")
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	r.RemoteAddr = "10.0.0.1:1234"
	if got := ClientIP(r); got != "203.0.113.7" {
		t.Fatalf("ClientIP = %q, want 203.0.113.7", got)
	}
}

func TestClientIP_RemoteAddrFallback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.5:9999"
	if got := ClientIP(r); got != "192.0.2.5" {
		t.Fatalf("ClientIP = %q, want 192.0.2.5", got)
	}
}
```

- [ ] **Step 2: Run to verify fail**

```bash
cd services/api && go test ./internal/modules/abuse/ -v; cd ../..
```
Expected: FAIL — undefined.

- [ ] **Step 3: Implement model.go**

```go
package abuse

// Subject types.
const (
	SubjectUser = "user"
	SubjectIP   = "ip"
)

// Setting keys (platform_settings).
const (
	SettingTurnstileEnabled    = "turnstile_enabled"
	SettingRateLimitEnabled    = "rate_limit_enabled"
	SettingIPReputationEnabled = "ip_reputation_enabled"
	SettingBlocklistEnabled    = "blocklist_enabled"
)

// Endpoint categories.
const (
	CategoryQueueJoin    = "queue_join"
	CategoryCheckout     = "checkout"
	CategoryAuthLogin    = "auth_login"
	CategoryAuthRegister = "auth_register"
	CategoryDefault      = "default"
)

// abuse_log actions.
const (
	ActionRateLimited    = "RATE_LIMITED"
	ActionBlockedHit     = "BLOCKED_HIT"
	ActionCaptchaFail    = "CAPTCHA_FAIL"
	ActionDuplicateQueue = "DUPLICATE_QUEUE"
	ActionReputationDeny = "REPUTATION_DENY"
	ActionBlockSet       = "BLOCK_SET"
	ActionUnblock        = "UNBLOCK"
)

// Reputation bump deltas.
const (
	BumpRateViolation = 2
	BumpCaptchaFail   = 3
	BumpBlockedHit    = 5
	BumpDuplicate     = 1
)

// categoryNeedsCaptcha reports whether a category requires Turnstile by default.
func categoryNeedsCaptcha(category string) bool {
	return category == CategoryQueueJoin
}

// RateLimit holds per-category limits (per minute).
type RateLimit struct {
	PerIP   int
	PerUser int
}

// categoryLimits returns the per-category rate limits.
func categoryLimits(category string) RateLimit {
	switch category {
	case CategoryQueueJoin:
		return RateLimit{PerIP: 10, PerUser: 5}
	case CategoryCheckout:
		return RateLimit{PerIP: 20, PerUser: 10}
	case CategoryAuthLogin:
		return RateLimit{PerIP: 10, PerUser: 5}
	case CategoryAuthRegister:
		return RateLimit{PerIP: 5, PerUser: 0}
	default:
		return RateLimit{PerIP: 120, PerUser: 0}
	}
}
```

- [ ] **Step 4: Implement clientip.go**

```go
package abuse

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the best-effort client IP: first hop of X-Forwarded-For,
// else the RemoteAddr host.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

- [ ] **Step 5: Implement fingerprint.go**

```go
package abuse

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

// Fingerprint returns a lightweight server-side hash of UA + client IP +
// Accept-Language. No client-side JS fingerprinting; no PII stored in clear.
func Fingerprint(r *http.Request) string {
	h := sha256.New()
	h.Write([]byte(r.Header.Get("User-Agent")))
	h.Write([]byte("|"))
	h.Write([]byte(ClientIP(r)))
	h.Write([]byte("|"))
	h.Write([]byte(r.Header.Get("Accept-Language")))
	return hex.EncodeToString(h.Sum(nil))
}
```

- [ ] **Step 6: Run to verify pass**

```bash
cd services/api && go test ./internal/modules/abuse/ -v; cd ../..
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/modules/abuse/model.go services/api/internal/modules/abuse/clientip.go services/api/internal/modules/abuse/fingerprint.go services/api/internal/modules/abuse/fingerprint_test.go
git commit -m "feat(phase9): abuse model, client IP, lightweight fingerprint"
```

---

## Task 8: Run Part 1 package tests + vet

- [ ] **Step 1: Build + vet + test new packages**

```bash
cd services/api && go build ./... && go vet ./internal/platform/ratelimit/... ./internal/platform/captcha/... ./internal/platform/middleware/... ./internal/modules/abuse/... && go test ./internal/platform/captcha/... ./internal/platform/middleware/... ./internal/modules/abuse/... -race; cd ../..
```
Expected: clean + green (ratelimit skips without Redis).

- [ ] **Step 2: Commit (if fixups)**

```bash
git add -A services/api
git commit -m "test(phase9): part 1 foundation green" || echo "nothing to commit"
```

---

Part 1 complete. Next: [Part 2 — Abuse module](2026-08-phase9-part2-abuse-module.md).
