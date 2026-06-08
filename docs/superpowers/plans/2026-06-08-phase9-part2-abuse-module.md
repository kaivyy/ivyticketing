# Phase 9 Plan — Part 2: Abuse Module

> Part of the Phase 9 implementation plan. Index: [2026-06-08-phase9-antibot-abuse.md](2026-06-08-phase9-antibot-abuse.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** New files + additive changes. Assumes Part 1 exists (config, migrations, sqlc, ratelimit, captcha, abuse model/clientip/fingerprint).

**Generated types (verify in `services/api/internal/db/abuse.sql.go` before coding):** `db.PlatformSetting{Key, Value string, UpdatedBy *uuid.UUID, UpdatedAt pgtype.Timestamptz}`; `db.BlockedSubject{ID uuid.UUID, SubjectType, SubjectValue string, Reason pgtype.Text, BlockedBy *uuid.UUID, CreatedAt pgtype.Timestamptz, ExpiresAt pgtype.Timestamptz}`; `db.IpRule{ID, Cidr, Rule string, Note pgtype.Text, ...}`; `db.AbuseLog{...}`; `db.IpReputation{SubjectType, SubjectValue string, Score int32, UpdatedAt pgtype.Timestamptz}`. Param structs: `UpsertPlatformSettingParams`, `GetBlockedSubjectParams{SubjectType, SubjectValue}`, `UpsertBlockedSubjectParams`, `DeleteBlockedSubjectParams`, `CreateIPRuleParams`, `InsertAbuseLogParams`, `ListAbuseLogParams{Limit, Offset}`, `GetReputationParams`, `BumpReputationParams{SubjectType, SubjectValue string, Score int32}`. Confirm exact names/types.

---

## Task 9: Abuse repository

**Files:**
- Create: `services/api/internal/modules/abuse/repository.go`
- Create: `services/api/internal/modules/abuse/errors.go`

- [ ] **Step 1: Implement errors.go**

```go
package abuse

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrUserBlocked       = apperr.New(http.StatusForbidden, "USER_BLOCKED", "access blocked")
	ErrRateLimited       = apperr.New(http.StatusTooManyRequests, "RATE_LIMITED", "too many requests, slow down")
	ErrCaptchaRequired   = apperr.New(http.StatusForbidden, "CAPTCHA_REQUIRED", "captcha verification required")
	ErrCaptchaInvalid    = apperr.New(http.StatusForbidden, "CAPTCHA_INVALID", "captcha verification failed")
	ErrReputationDenied  = apperr.New(http.StatusForbidden, "REPUTATION_DENIED", "request denied")
	ErrQueueCapExceeded  = apperr.New(http.StatusTooManyRequests, "QUEUE_ENTRY_CAP_EXCEEDED", "too many active queue entries")
	ErrInvalidSetting    = apperr.New(http.StatusBadRequest, "INVALID_SETTING", "invalid setting key or value")
)
```

- [ ] **Step 2: Implement repository.go**

Read `services/api/internal/db/abuse.sql.go` first for exact method names/params. Then:
```go
package abuse

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ListSettings(ctx context.Context) ([]db.PlatformSetting, error)
	UpsertSetting(ctx context.Context, arg db.UpsertPlatformSettingParams) (db.PlatformSetting, error)

	GetBlockedSubject(ctx context.Context, arg db.GetBlockedSubjectParams) (db.BlockedSubject, error)
	ListBlockedSubjects(ctx context.Context, arg db.ListBlockedSubjectsParams) ([]db.BlockedSubject, error)
	UpsertBlockedSubject(ctx context.Context, arg db.UpsertBlockedSubjectParams) (db.BlockedSubject, error)
	DeleteBlockedSubject(ctx context.Context, arg db.DeleteBlockedSubjectParams) error

	ListIPRules(ctx context.Context) ([]db.IpRule, error)
	CreateIPRule(ctx context.Context, arg db.CreateIPRuleParams) (db.IpRule, error)
	DeleteIPRule(ctx context.Context, id uuid.UUID) error

	InsertAbuseLog(ctx context.Context, arg db.InsertAbuseLogParams) error
	ListAbuseLog(ctx context.Context, arg db.ListAbuseLogParams) ([]db.AbuseLog, error)

	GetReputation(ctx context.Context, arg db.GetReputationParams) (db.IpReputation, error)
	BumpReputation(ctx context.Context, arg db.BumpReputationParams) (db.IpReputation, error)

	CountActiveQueueTokensByUser(ctx context.Context, participantID uuid.UUID) (int64, error)
}

type sqlcRepo struct {
	q *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

// thin pass-throughs calling r.q.<Generated method> for every interface method.
```

> Write each pass-through. Confirm generated names: `ListPlatformSettings`→`ListSettings` wrapper, `UpsertPlatformSetting`, `GetBlockedSubject`, `ListBlockedSubjects`, `UpsertBlockedSubject`, `DeleteBlockedSubject`, `ListIPRules`, `CreateIPRule`, `DeleteIPRule`, `InsertAbuseLog`, `ListAbuseLog`, `GetReputation`, `BumpReputation`, `CountActiveQueueTokensByUser`. Adjust interface method names if the generated signatures differ (e.g., a query taking a single arg vs a params struct).

- [ ] **Step 3: Build**

```bash
cd services/api && go build ./internal/modules/abuse/...; cd ../..
```
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/abuse/repository.go services/api/internal/modules/abuse/errors.go
git commit -m "feat(phase9): abuse repository + error codes"
```

---

## Task 10: Settings cache (runtime toggle)

**Files:**
- Create: `services/api/internal/modules/abuse/settings.go`
- Create: `services/api/internal/modules/abuse/tests/settings_test.go`

- [ ] **Step 1: Write the failing test** (fake repo)

Create `services/api/internal/modules/abuse/tests/settings_test.go`:
```go
package abuse_test

import (
	"context"
	"testing"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
)

type fakeSettingsRepo struct {
	rows []db.PlatformSetting
}

func (f *fakeSettingsRepo) ListSettings(ctx context.Context) ([]db.PlatformSetting, error) {
	return f.rows, nil
}

func TestSettings_IsEnabled(t *testing.T) {
	repo := &fakeSettingsRepo{rows: []db.PlatformSetting{
		{Key: "rate_limit_enabled", Value: "true"},
		{Key: "turnstile_enabled", Value: "false"},
	}}
	s := abuse.NewSettings(repo)
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !s.IsEnabled("rate_limit_enabled") {
		t.Error("rate_limit_enabled should be true")
	}
	if s.IsEnabled("turnstile_enabled") {
		t.Error("turnstile_enabled should be false")
	}
}

func TestSettings_FailSafeDefaults(t *testing.T) {
	// Empty cache (never refreshed): blocklist/ratelimit/reputation default ON,
	// turnstile default OFF.
	s := abuse.NewSettings(&fakeSettingsRepo{})
	if !s.IsEnabled("rate_limit_enabled") {
		t.Error("rate_limit default should be ON (fail-safe)")
	}
	if !s.IsEnabled("blocklist_enabled") {
		t.Error("blocklist default should be ON (fail-safe)")
	}
	if !s.IsEnabled("ip_reputation_enabled") {
		t.Error("ip_reputation default should be ON (fail-safe)")
	}
	if s.IsEnabled("turnstile_enabled") {
		t.Error("turnstile default should be OFF")
	}
}
```

The settings cache needs only `ListSettings` — define a narrow interface `SettingsReader` in settings.go so the fake is small.

- [ ] **Step 2: Run to verify fail**

```bash
cd services/api && go test ./internal/modules/abuse/tests/ -run TestSettings -v; cd ../..
```
Expected: FAIL — undefined.

- [ ] **Step 3: Implement settings.go**

```go
package abuse

import (
	"context"
	"sync"
	"time"
)

// SettingsReader is the minimal repo surface the settings cache needs.
type SettingsReader interface {
	ListSettings(ctx context.Context) ([]PlatformSettingRow, error)
}
```

> WAIT — the repo returns `[]db.PlatformSetting`, not a local type. To keep the fake small AND avoid a db import in the test's fake, define `SettingsReader` over `db.PlatformSetting`. Update the test's fake to match. Rewrite settings.go:

```go
package abuse

import (
	"context"
	"sync"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// SettingsReader is the minimal repo surface the settings cache needs.
type SettingsReader interface {
	ListSettings(ctx context.Context) ([]db.PlatformSetting, error)
}

// failSafeDefaults: protective features ON by default; turnstile OFF (won't block
// users when no secret configured).
var failSafeDefaults = map[string]bool{
	SettingRateLimitEnabled:    true,
	SettingBlocklistEnabled:    true,
	SettingIPReputationEnabled: true,
	SettingTurnstileEnabled:    false,
}

// Settings is an in-memory cache of platform_settings, refreshed periodically.
type Settings struct {
	repo SettingsReader
	mu   sync.RWMutex
	vals map[string]string
}

func NewSettings(repo SettingsReader) *Settings {
	return &Settings{repo: repo, vals: map[string]string{}}
}

// Refresh reloads all settings from the DB into the cache.
func (s *Settings) Refresh(ctx context.Context) error {
	rows, err := s.repo.ListSettings(ctx)
	if err != nil {
		return err
	}
	m := make(map[string]string, len(rows))
	for _, row := range rows {
		m[row.Key] = row.Value
	}
	s.mu.Lock()
	s.vals = m
	s.mu.Unlock()
	return nil
}

// IsEnabled returns whether a boolean feature toggle is on. Missing key →
// fail-safe default (protective features ON, turnstile OFF).
func (s *Settings) IsEnabled(key string) bool {
	s.mu.RLock()
	v, ok := s.vals[key]
	s.mu.RUnlock()
	if !ok {
		return failSafeDefaults[key]
	}
	return v == "true"
}

// Get returns the raw string value (empty if unset).
func (s *Settings) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vals[key]
}

// StartRefresh launches a background ticker refreshing the cache until ctx is done.
func (s *Settings) StartRefresh(ctx context.Context, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = s.Refresh(ctx)
			}
		}
	}()
}
```

Remove the first (wrong) settings.go draft — only the second version stands. Update the test's `fakeSettingsRepo` to return `[]db.PlatformSetting` (it already does).

- [ ] **Step 4: Run to verify pass**

```bash
cd services/api && go test ./internal/modules/abuse/tests/ -run TestSettings -v; cd ../..
```
Expected: PASS (both).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/abuse/settings.go services/api/internal/modules/abuse/tests/settings_test.go
git commit -m "feat(phase9): platform settings cache (runtime toggle, fail-safe defaults)"
```

---

## Task 11: Blocklist + IP rules

**Files:**
- Create: `services/api/internal/modules/abuse/blocklist.go`
- Create: `services/api/internal/modules/abuse/tests/blocklist_test.go`

- [ ] **Step 1: Write the failing test** (fake repo)

`blocklist_test.go`:
```go
package abuse_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
)

type fakeBlockRepo struct {
	blocked map[string]bool // "type:value" → blocked
	rules   []db.IpRule
}

func (f *fakeBlockRepo) GetBlockedSubject(ctx context.Context, arg db.GetBlockedSubjectParams) (db.BlockedSubject, error) {
	if f.blocked[arg.SubjectType+":"+arg.SubjectValue] {
		return db.BlockedSubject{SubjectType: arg.SubjectType, SubjectValue: arg.SubjectValue}, nil
	}
	return db.BlockedSubject{}, pgx.ErrNoRows
}
func (f *fakeBlockRepo) ListIPRules(ctx context.Context) ([]db.IpRule, error) { return f.rules, nil }

func TestIsBlocked_User(t *testing.T) {
	repo := &fakeBlockRepo{blocked: map[string]bool{"user:u1": true}}
	bl := abuse.NewBlocklist(repo)
	if !bl.IsBlocked(context.Background(), "u1", "1.2.3.4") {
		t.Fatal("u1 should be blocked")
	}
	if bl.IsBlocked(context.Background(), "u2", "1.2.3.4") {
		t.Fatal("u2 should not be blocked")
	}
}

func TestIPRule_DenyAndAllow(t *testing.T) {
	repo := &fakeBlockRepo{
		blocked: map[string]bool{},
		rules: []db.IpRule{
			{Cidr: "203.0.113.0/24", Rule: "deny"},
			{Cidr: "203.0.113.5/32", Rule: "allow"},
		},
	}
	bl := abuse.NewBlocklist(repo)
	// IP in deny range → blocked
	if !bl.IsBlocked(context.Background(), "", "203.0.113.9") {
		t.Fatal("203.0.113.9 should be denied by CIDR")
	}
	// IP with explicit allow → not blocked (allow wins)
	if bl.IsBlocked(context.Background(), "", "203.0.113.5") {
		t.Fatal("203.0.113.5 should be allowed (allow wins)")
	}
	_ = pgtype.Text{}
}
```

- [ ] **Step 2: Run to verify fail**

```bash
cd services/api && go test ./internal/modules/abuse/tests/ -run 'TestIsBlocked|TestIPRule' -v; cd ../..
```
Expected: FAIL — undefined.

- [ ] **Step 3: Implement blocklist.go**

```go
package abuse

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// BlocklistReader is the minimal repo surface the blocklist needs.
type BlocklistReader interface {
	GetBlockedSubject(ctx context.Context, arg db.GetBlockedSubjectParams) (db.BlockedSubject, error)
	ListIPRules(ctx context.Context) ([]db.IpRule, error)
}

type Blocklist struct {
	repo BlocklistReader
}

func NewBlocklist(repo BlocklistReader) *Blocklist { return &Blocklist{repo: repo} }

// IsBlocked reports whether the user or IP is blocked. An explicit allow ip_rule
// wins over a deny rule and over reputation/rate (checked by caller). userID may
// be empty (unauthenticated).
func (b *Blocklist) IsBlocked(ctx context.Context, userID, ip string) bool {
	// ip_rules: allow wins
	rules, err := b.repo.ListIPRules(ctx)
	if err == nil {
		allowed, denied := matchIPRules(rules, ip)
		if allowed {
			return false
		}
		if denied {
			return true
		}
	}
	if userID != "" {
		if _, err := b.repo.GetBlockedSubject(ctx, db.GetBlockedSubjectParams{SubjectType: SubjectUser, SubjectValue: userID}); err == nil {
			return true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			// on error, fail-safe: do not block solely due to DB error
		}
	}
	if ip != "" {
		if _, err := b.repo.GetBlockedSubject(ctx, db.GetBlockedSubjectParams{SubjectType: SubjectIP, SubjectValue: ip}); err == nil {
			return true
		}
	}
	return false
}

// matchIPRules returns (allowed, denied) for the given IP against the rules.
func matchIPRules(rules []db.IpRule, ip string) (allowed, denied bool) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false, false
	}
	for _, rule := range rules {
		if !cidrContains(rule.Cidr, parsed) {
			continue
		}
		switch rule.Rule {
		case "allow":
			allowed = true
		case "deny":
			denied = true
		}
	}
	return allowed, denied
}

// cidrContains handles both bare IPs and CIDR notation.
func cidrContains(cidr string, ip net.IP) bool {
	if _, network, err := net.ParseCIDR(cidr); err == nil {
		return network.Contains(ip)
	}
	if single := net.ParseIP(cidr); single != nil {
		return single.Equal(ip)
	}
	return false
}
```

- [ ] **Step 4: Run to verify pass**

```bash
cd services/api && go test ./internal/modules/abuse/tests/ -run 'TestIsBlocked|TestIPRule' -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/abuse/blocklist.go services/api/internal/modules/abuse/tests/blocklist_test.go
git commit -m "feat(phase9): blocklist + ip allow/deny rules (allow wins)"
```

---

## Task 12: Reputation

**Files:**
- Create: `services/api/internal/modules/abuse/reputation.go`
- Create: `services/api/internal/modules/abuse/tests/reputation_test.go`

- [ ] **Step 1: Write the failing test** (fake repo)

`reputation_test.go`:
```go
package abuse_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
)

type fakeRepRepo struct {
	scores map[string]int32
}

func (f *fakeRepRepo) GetReputation(ctx context.Context, arg db.GetReputationParams) (db.IpReputation, error) {
	if v, ok := f.scores[arg.SubjectType+":"+arg.SubjectValue]; ok {
		return db.IpReputation{SubjectType: arg.SubjectType, SubjectValue: arg.SubjectValue, Score: v}, nil
	}
	return db.IpReputation{}, pgx.ErrNoRows
}
func (f *fakeRepRepo) BumpReputation(ctx context.Context, arg db.BumpReputationParams) (db.IpReputation, error) {
	if f.scores == nil {
		f.scores = map[string]int32{}
	}
	f.scores[arg.SubjectType+":"+arg.SubjectValue] += arg.Score
	return db.IpReputation{Score: f.scores[arg.SubjectType+":"+arg.SubjectValue]}, nil
}

func TestReputation_BumpAndThresholds(t *testing.T) {
	repo := &fakeRepRepo{scores: map[string]int32{}}
	rep := abuse.NewReputation(repo, 10, 25) // challenge=10, deny=25

	rep.Bump(context.Background(), abuse.SubjectIP, "1.2.3.4", abuse.BumpBlockedHit, "blocked")
	if got := rep.Score(context.Background(), abuse.SubjectIP, "1.2.3.4"); got != 5 {
		t.Fatalf("score = %d, want 5", got)
	}
	if rep.ShouldChallenge(context.Background(), abuse.SubjectIP, "1.2.3.4") {
		t.Fatal("score 5 should not trigger challenge (threshold 10)")
	}
	// bump to 11 → challenge
	rep.Bump(context.Background(), abuse.SubjectIP, "1.2.3.4", 6, "x")
	if !rep.ShouldChallenge(context.Background(), abuse.SubjectIP, "1.2.3.4") {
		t.Fatal("score 11 should trigger challenge")
	}
	if rep.ShouldDeny(context.Background(), abuse.SubjectIP, "1.2.3.4") {
		t.Fatal("score 11 should not deny (threshold 25)")
	}
}
```

- [ ] **Step 2: Run to verify fail**

```bash
cd services/api && go test ./internal/modules/abuse/tests/ -run TestReputation -v; cd ../..
```
Expected: FAIL — undefined.

- [ ] **Step 3: Implement reputation.go**

```go
package abuse

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// ReputationStore is the minimal repo surface reputation needs.
type ReputationStore interface {
	GetReputation(ctx context.Context, arg db.GetReputationParams) (db.IpReputation, error)
	BumpReputation(ctx context.Context, arg db.BumpReputationParams) (db.IpReputation, error)
}

type Reputation struct {
	repo               ReputationStore
	challengeThreshold int
	denyThreshold      int
}

func NewReputation(repo ReputationStore, challenge, deny int) *Reputation {
	return &Reputation{repo: repo, challengeThreshold: challenge, denyThreshold: deny}
}

func (r *Reputation) Score(ctx context.Context, subjectType, subjectValue string) int {
	rec, err := r.repo.GetReputation(ctx, db.GetReputationParams{SubjectType: subjectType, SubjectValue: subjectValue})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0
		}
		return 0 // fail-open on read error
	}
	return int(rec.Score)
}

func (r *Reputation) Bump(ctx context.Context, subjectType, subjectValue string, delta int, reason string) {
	_, _ = r.repo.BumpReputation(ctx, db.BumpReputationParams{
		SubjectType:  subjectType,
		SubjectValue: subjectValue,
		Score:        int32(delta),
	})
}

func (r *Reputation) ShouldChallenge(ctx context.Context, subjectType, subjectValue string) bool {
	return r.Score(ctx, subjectType, subjectValue) >= r.challengeThreshold
}

func (r *Reputation) ShouldDeny(ctx context.Context, subjectType, subjectValue string) bool {
	return r.Score(ctx, subjectType, subjectValue) >= r.denyThreshold
}
```

> Confirm `BumpReputationParams.Score` is `int32` (matches the `integer` column). If the generated upsert adds to the existing score (per the `ON CONFLICT ... score = ip_reputation.score + EXCLUDED.score` query in Part 1), passing the delta as `Score` is correct.

- [ ] **Step 4: Run to verify pass**

```bash
cd services/api && go test ./internal/modules/abuse/tests/ -run TestReputation -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/abuse/reputation.go services/api/internal/modules/abuse/tests/reputation_test.go
git commit -m "feat(phase9): ip reputation score (bump + challenge/deny thresholds)"
```

---

## Task 13: Rate-limit wrapper

**Files:**
- Create: `services/api/internal/modules/abuse/ratelimit.go`

- [ ] **Step 1: Implement ratelimit.go** (thin wrapper over platform/ratelimit with per-category keys)

```go
package abuse

import (
	"context"
	"time"

	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
)

// RateChecker checks per-category, per-subject limits.
type RateChecker struct {
	lim *ratelimit.Limiter
}

func NewRateChecker(lim *ratelimit.Limiter) *RateChecker { return &RateChecker{lim: lim} }

// AllowIP / AllowUser return false when the per-minute limit for the category is
// exceeded. A zero limit means "no limit for this dimension" → always allowed.
func (rc *RateChecker) AllowIP(ctx context.Context, category, ip string) (bool, error) {
	limit := categoryLimits(category).PerIP
	if limit <= 0 || ip == "" {
		return true, nil
	}
	return rc.lim.Allow(ctx, category+":ip:"+ip, limit, time.Minute)
}

func (rc *RateChecker) AllowUser(ctx context.Context, category, userID string) (bool, error) {
	limit := categoryLimits(category).PerUser
	if limit <= 0 || userID == "" {
		return true, nil
	}
	return rc.lim.Allow(ctx, category+":user:"+userID, limit, time.Minute)
}
```

- [ ] **Step 2: Build**

```bash
cd services/api && go build ./internal/modules/abuse/...; cd ../..
```
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add services/api/internal/modules/abuse/ratelimit.go
git commit -m "feat(phase9): per-category rate-limit wrapper"
```

---

## Task 14: Guard middleware chain

**Files:**
- Create: `services/api/internal/modules/abuse/guard.go`
- Create: `services/api/internal/modules/abuse/tests/guard_test.go`

- [ ] **Step 1: Write the failing test**

`guard_test.go` builds a `Guard` from fakes (settings, blocklist via fake repo, rate checker over a fake limiter, fake captcha) and asserts middleware outcomes. Keep it focused:

```go
package abuse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	"github.com/varin/ivyticketing/services/api/internal/platform/captcha"
	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
)

// Build a guard with all deps that pass-through (clean request → 200).
// Use real Settings with all-OFF so steps are skipped, plus a no-op logger.
// NOTE: construct via abuse.NewGuard(...) — match the actual constructor signature
// finalized in this task. The test below assumes:
//   NewGuard(settings *Settings, blocklist *Blocklist, rate *RateChecker,
//            rep *Reputation, captcha captcha.Verifier, logger AbuseLogger,
//            cap *QueueCap) *Guard
// Adjust to the real signature once written.

func TestGuard_CleanRequestPasses(t *testing.T) {
	g := newTestGuard(t, guardOpts{}) // helper: everything disabled/pass
	called := false
	h := g.Middleware(abuse.CategoryQueueJoin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := withUser(httptest.NewRequest("POST", "/events/x/queue/join", nil), uuid.New())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("clean request should pass: called=%v code=%d", called, rec.Code)
	}
}

func TestGuard_BlockedReturns403(t *testing.T) {
	g := newTestGuard(t, guardOpts{blockedUser: true, blocklistEnabled: true})
	h := g.Middleware(abuse.CategoryQueueJoin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("blocked request must not reach handler")
	}))
	req := withUser(httptest.NewRequest("POST", "/x", nil), uuid.New())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("blocked → 403, got %d", rec.Code)
	}
}

func withUser(r *http.Request, uid uuid.UUID) *http.Request {
	ctx := authctx.WithIdentity(r.Context(), authctx.Identity{UserID: uid})
	return r.WithContext(ctx)
}

var _ = captcha.FakeVerifier{}
var _ = context.Background
```

> Implement `newTestGuard`/`guardOpts` helper in the test file constructing a `Guard` with fakes. The exact fakes depend on the constructor — write the constructor in Step 3 first, then finalize the helper. The two behaviors to prove: clean→200, blocked→403. Keep other steps (rate/captcha/reputation) covered by their own unit tests (Tasks 10-13); the guard test proves wiring + order + toggle-skip for the two highest-value paths.

- [ ] **Step 2: Run to verify fail**

```bash
cd services/api && go test ./internal/modules/abuse/tests/ -run TestGuard -v; cd ../..
```
Expected: FAIL — undefined.

- [ ] **Step 3: Implement guard.go**

```go
package abuse

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	"github.com/varin/ivyticketing/services/api/internal/platform/captcha"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// AbuseLogger records abuse events (implemented by Service / repo-backed logger).
type AbuseLogger interface {
	Log(ctx context.Context, e AbuseEvent)
}

// QueueCapChecker enforces the cross-event active queue cap (implemented by Service).
type QueueCapChecker interface {
	WithinQueueCap(ctx context.Context, userID uuid.UUID) (bool, error)
}

type AbuseEvent struct {
	SubjectType  string
	SubjectValue string
	Action       string
	Category     string
	Fingerprint  string
	IP           string
	UserID       *uuid.UUID
}

// Guard builds anti-bot middleware chains. Each protection step is gated by a
// platform_settings toggle; a disabled step is a pass-through.
type Guard struct {
	settings  *Settings
	blocklist *Blocklist
	rate      *RateChecker
	rep       *Reputation
	captcha   captcha.Verifier
	logger    AbuseLogger
	cap       QueueCapChecker
}

func NewGuard(settings *Settings, blocklist *Blocklist, rate *RateChecker, rep *Reputation, cap_ captcha.Verifier, logger AbuseLogger, qcap QueueCapChecker) *Guard {
	return &Guard{settings: settings, blocklist: blocklist, rate: rate, rep: rep, captcha: cap_, logger: logger, cap: qcap}
}

// Middleware returns the guard chain for an endpoint category.
func (g *Guard) Middleware(category string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ip := ClientIP(r)
			fp := Fingerprint(r)
			var userID string
			var userPtr *uuid.UUID
			if id, ok := authctx.FromContext(ctx); ok {
				userID = id.UserID.String()
				u := id.UserID
				userPtr = &u
			}

			// 1. Blocklist
			if g.settings.IsEnabled(SettingBlocklistEnabled) {
				if g.blocklist.IsBlocked(ctx, userID, ip) {
					g.log(ctx, ActionBlockedHit, category, fp, ip, userPtr)
					apperr.WriteError(w, r, ErrUserBlocked)
					return
				}
			}

			// 2. Rate limit (per IP + per user)
			if g.settings.IsEnabled(SettingRateLimitEnabled) {
				okIP, _ := g.rate.AllowIP(ctx, category, ip)
				okUser, _ := g.rate.AllowUser(ctx, category, userID)
				if !okIP || !okUser {
					g.bump(ctx, ip, BumpRateViolation, "rate")
					g.log(ctx, ActionRateLimited, category, fp, ip, userPtr)
					w.Header().Set("Retry-After", "60")
					apperr.WriteError(w, r, ErrRateLimited)
					return
				}
			}

			// 3. Reputation gate (deny / force challenge)
			challengeRequired := categoryNeedsCaptcha(category)
			if g.settings.IsEnabled(SettingIPReputationEnabled) {
				if g.rep.ShouldDeny(ctx, SubjectIP, ip) {
					g.log(ctx, ActionReputationDeny, category, fp, ip, userPtr)
					apperr.WriteError(w, r, ErrReputationDenied)
					return
				}
				if g.rep.ShouldChallenge(ctx, SubjectIP, ip) {
					challengeRequired = true
				}
			}

			// 4. Turnstile (when enabled AND the category/reputation requires it)
			if g.settings.IsEnabled(SettingTurnstileEnabled) && challengeRequired {
				token := r.Header.Get("X-Turnstile-Token")
				if token == "" {
					apperr.WriteError(w, r, ErrCaptchaRequired)
					return
				}
				ok, err := g.captcha.Verify(ctx, token, ip)
				if err != nil {
					// fail-open on verifier error (e.g., no secret/transport) — don't block users
				} else if !ok {
					g.bump(ctx, ip, BumpCaptchaFail, "captcha")
					g.log(ctx, ActionCaptchaFail, category, fp, ip, userPtr)
					apperr.WriteError(w, r, ErrCaptchaInvalid)
					return
				}
			}

			// 5. Queue entry cap (queue_join only)
			if category == CategoryQueueJoin && g.cap != nil && userPtr != nil {
				within, err := g.cap.WithinQueueCap(ctx, *userPtr)
				if err == nil && !within {
					g.log(ctx, ActionDuplicateQueue, category, fp, ip, userPtr)
					apperr.WriteError(w, r, ErrQueueCapExceeded)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (g *Guard) log(ctx context.Context, action, category, fp, ip string, userPtr *uuid.UUID) {
	if g.logger == nil {
		return
	}
	st, sv := SubjectIP, ip
	if userPtr != nil {
		st, sv = SubjectUser, userPtr.String()
	}
	g.logger.Log(ctx, AbuseEvent{SubjectType: st, SubjectValue: sv, Action: action, Category: category, Fingerprint: fp, IP: ip, UserID: userPtr})
}

func (g *Guard) bump(ctx context.Context, ip string, delta int, reason string) {
	if g.settings.IsEnabled(SettingIPReputationEnabled) {
		g.rep.Bump(ctx, SubjectIP, ip, delta, reason)
	}
}

var _ = strconv.Itoa // keep strconv if used elsewhere; remove if unused
```

> Remove the `var _ = strconv.Itoa` line and the `strconv` import if unused after writing.

- [ ] **Step 4: Run to verify pass**

```bash
cd services/api && go test ./internal/modules/abuse/tests/ -run TestGuard -v -race; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/abuse/guard.go services/api/internal/modules/abuse/tests/guard_test.go
git commit -m "feat(phase9): abuse guard middleware chain (toggle-gated)"
```

---

## Task 15: Service (admin ops, queue cap, abuse logger)

**Files:**
- Create: `services/api/internal/modules/abuse/service.go`
- Create: `services/api/internal/modules/abuse/dto.go`

- [ ] **Step 1: Implement dto.go**

```go
package abuse

import "time"

type SettingDTO struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type BlockRequest struct {
	SubjectType  string  `json:"subjectType"`
	SubjectValue string  `json:"subjectValue"`
	Reason       string  `json:"reason"`
	ExpiresAt    *string `json:"expiresAt"`
}

type UnblockRequest struct {
	SubjectType  string `json:"subjectType"`
	SubjectValue string `json:"subjectValue"`
}

type BlockedDTO struct {
	SubjectType  string     `json:"subjectType"`
	SubjectValue string     `json:"subjectValue"`
	Reason       string     `json:"reason,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
}

type IPRuleRequest struct {
	CIDR string `json:"cidr"`
	Rule string `json:"rule"`
	Note string `json:"note"`
}

type AbuseLogDTO struct {
	Action       string    `json:"action"`
	Category     string    `json:"category,omitempty"`
	SubjectType  string    `json:"subjectType,omitempty"`
	SubjectValue string    `json:"subjectValue,omitempty"`
	IP           string    `json:"ip,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type SecurityConfigDTO struct {
	TurnstileEnabled bool   `json:"turnstileEnabled"`
	SiteKey          string `json:"siteKey,omitempty"`
}
```

- [ ] **Step 2: Implement service.go**

The service implements `AbuseLogger` (`Log`), `QueueCapChecker` (`WithinQueueCap`), and admin ops. It needs the repo, settings, audit logger, max-cap config, and turnstile site key (for /security/config).

```go
package abuse

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type Service struct {
	repo        Repository
	settings    *Settings
	audit       AuditRecorder
	maxQueueCap int
	siteKey     string
}

func NewService(repo Repository, settings *Settings, recorder AuditRecorder, maxQueueCap int, siteKey string) *Service {
	return &Service{repo: repo, settings: settings, audit: recorder, maxQueueCap: maxQueueCap, siteKey: siteKey}
}

// Log implements AbuseLogger (called by the guard, fire-and-forget).
func (s *Service) Log(ctx context.Context, e AbuseEvent) {
	var detail []byte
	var uid uuid.NullUUID
	if e.UserID != nil {
		uid = uuid.NullUUID{UUID: *e.UserID, Valid: true}
	}
	_ = s.repo.InsertAbuseLog(ctx, db.InsertAbuseLogParams{
		SubjectType:  pgtype.Text{String: e.SubjectType, Valid: e.SubjectType != ""},
		SubjectValue: pgtype.Text{String: e.SubjectValue, Valid: e.SubjectValue != ""},
		Action:       e.Action,
		Category:     pgtype.Text{String: e.Category, Valid: e.Category != ""},
		Fingerprint:  pgtype.Text{String: e.Fingerprint, Valid: e.Fingerprint != ""},
		Ip:           pgtype.Text{String: e.IP, Valid: e.IP != ""},
		UserID:       toUUIDPtr(uid),
		Detail:       detail,
	})
}

// WithinQueueCap implements QueueCapChecker.
func (s *Service) WithinQueueCap(ctx context.Context, userID uuid.UUID) (bool, error) {
	if s.maxQueueCap <= 0 {
		return true, nil
	}
	n, err := s.repo.CountActiveQueueTokensByUser(ctx, userID)
	if err != nil {
		return true, nil // fail-open
	}
	return n < int64(s.maxQueueCap), nil
}

func (s *Service) SecurityConfig() SecurityConfigDTO {
	return SecurityConfigDTO{
		TurnstileEnabled: s.settings.IsEnabled(SettingTurnstileEnabled),
		SiteKey:          s.siteKey,
	}
}

func (s *Service) SetSetting(ctx context.Context, actor uuid.UUID, key, value string) error {
	if !validSettingKey(key) {
		return ErrInvalidSetting
	}
	_, err := s.repo.UpsertSetting(ctx, db.UpsertPlatformSettingParams{
		Key: key, Value: value, UpdatedBy: &actor,
	})
	if err != nil {
		return err
	}
	_ = s.settings.Refresh(ctx) // write-through
	s.record(ctx, &actor, "ABUSE_SETTING_CHANGED", "platform_setting", key, map[string]any{"value": value})
	return nil
}

func (s *Service) Block(ctx context.Context, actor uuid.UUID, req BlockRequest) error {
	if req.SubjectType != SubjectUser && req.SubjectType != SubjectIP {
		return ErrInvalidSetting
	}
	var exp pgtype.Timestamptz
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return ErrInvalidSetting
		}
		exp = pgtype.Timestamptz{Time: t, Valid: true}
	}
	_, err := s.repo.UpsertBlockedSubject(ctx, db.UpsertBlockedSubjectParams{
		SubjectType:  req.SubjectType,
		SubjectValue: req.SubjectValue,
		Reason:       pgtype.Text{String: req.Reason, Valid: req.Reason != ""},
		BlockedBy:    &actor,
		ExpiresAt:    exp,
	})
	if err != nil {
		return err
	}
	s.record(ctx, &actor, "ABUSE_BLOCK_SET", req.SubjectType, req.SubjectValue, map[string]any{"reason": req.Reason})
	return nil
}

func (s *Service) Unblock(ctx context.Context, actor uuid.UUID, req UnblockRequest) error {
	if err := s.repo.DeleteBlockedSubject(ctx, db.DeleteBlockedSubjectParams{
		SubjectType: req.SubjectType, SubjectValue: req.SubjectValue,
	}); err != nil {
		return err
	}
	s.record(ctx, &actor, "ABUSE_UNBLOCK", req.SubjectType, req.SubjectValue, nil)
	return nil
}

func (s *Service) ListSettings(ctx context.Context) ([]SettingDTO, error) {
	rows, err := s.repo.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SettingDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, SettingDTO{Key: r.Key, Value: r.Value})
	}
	return out, nil
}

func (s *Service) ListBlocked(ctx context.Context, limit, offset int32) ([]BlockedDTO, error) {
	rows, err := s.repo.ListBlockedSubjects(ctx, db.ListBlockedSubjectsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	out := make([]BlockedDTO, 0, len(rows))
	for _, r := range rows {
		d := BlockedDTO{SubjectType: r.SubjectType, SubjectValue: r.SubjectValue, CreatedAt: r.CreatedAt.Time}
		if r.Reason.Valid {
			d.Reason = r.Reason.String
		}
		if r.ExpiresAt.Valid {
			e := r.ExpiresAt.Time
			d.ExpiresAt = &e
		}
		out = append(out, d)
	}
	return out, nil
}

func (s *Service) ListAbuseLog(ctx context.Context, limit, offset int32) ([]AbuseLogDTO, error) {
	rows, err := s.repo.ListAbuseLog(ctx, db.ListAbuseLogParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	out := make([]AbuseLogDTO, 0, len(rows))
	for _, r := range rows {
		d := AbuseLogDTO{Action: r.Action, CreatedAt: r.CreatedAt.Time}
		if r.Category.Valid {
			d.Category = r.Category.String
		}
		if r.SubjectType.Valid {
			d.SubjectType = r.SubjectType.String
		}
		if r.SubjectValue.Valid {
			d.SubjectValue = r.SubjectValue.String
		}
		if r.Ip.Valid {
			d.IP = r.Ip.String
		}
		out = append(out, d)
	}
	return out, nil
}

func (s *Service) ListIPRules(ctx context.Context) ([]db.IpRule, error) { return s.repo.ListIPRules(ctx) }

func (s *Service) AddIPRule(ctx context.Context, actor uuid.UUID, req IPRuleRequest) error {
	if req.Rule != "allow" && req.Rule != "deny" {
		return ErrInvalidSetting
	}
	_, err := s.repo.CreateIPRule(ctx, db.CreateIPRuleParams{
		Cidr: req.CIDR, Rule: req.Rule,
		Note:      pgtype.Text{String: req.Note, Valid: req.Note != ""},
		CreatedBy: &actor,
	})
	if err != nil {
		return err
	}
	s.record(ctx, &actor, "ABUSE_IP_RULE_ADDED", "ip_rule", req.CIDR, map[string]any{"rule": req.Rule})
	return nil
}

func (s *Service) DeleteIPRule(ctx context.Context, actor, id uuid.UUID) error {
	if err := s.repo.DeleteIPRule(ctx, id); err != nil {
		return err
	}
	s.record(ctx, &actor, "ABUSE_IP_RULE_REMOVED", "ip_rule", id.String(), nil)
	return nil
}

func (s *Service) record(ctx context.Context, actor *uuid.UUID, action, targetType, targetID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, audit.Entry{ActorUserID: actor, Action: action, TargetType: targetType, TargetID: targetID, Metadata: meta})
}

func validSettingKey(key string) bool {
	switch key {
	case SettingTurnstileEnabled, SettingRateLimitEnabled, SettingIPReputationEnabled, SettingBlocklistEnabled:
		return true
	}
	return false
}

func toUUIDPtr(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	u := n.UUID
	return &u
}

var _ = json.Marshal // remove if unused
var _ = errors.Is    // remove if unused
```

> Remove unused `json`/`errors` imports and the `var _ =` lines if the compiler flags them. Verify generated param/field names: `InsertAbuseLogParams` fields (`Ip` vs `IP`, `UserID *uuid.UUID` vs `pgtype`), `UpsertBlockedSubjectParams`, `ListBlockedSubjectsParams{Limit, Offset int32}`, `CreateIPRuleParams`, `UpsertPlatformSettingParams{Key, Value string, UpdatedBy *uuid.UUID}`. Adjust pgtype/pointer usage to actual generated types.

- [ ] **Step 3: Build + test**

```bash
cd services/api && go build ./internal/modules/abuse/... && go test ./internal/modules/abuse/... -race; cd ../..
```
Expected: clean + green.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/abuse/service.go services/api/internal/modules/abuse/dto.go
git commit -m "feat(phase9): abuse service (admin ops, queue cap, abuse logger)"
```

---

Part 2 complete. Next: [Part 3 — Endpoints, wiring, frontend, docs](2026-06-08-phase9-part3-wiring-frontend-docs.md).
