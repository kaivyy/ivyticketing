package abuse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	"github.com/varin/ivyticketing/services/api/internal/platform/captcha"
	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
)

// guardRepo satisfies BlocklistReader + ReputationStore.
type guardRepo struct {
	blocked map[string]bool // "type:value" → blocked
}

func (r *guardRepo) GetBlockedSubject(ctx context.Context, arg db.GetBlockedSubjectParams) (db.BlockedSubject, error) {
	if r.blocked[arg.SubjectType+":"+arg.SubjectValue] {
		return db.BlockedSubject{SubjectType: arg.SubjectType, SubjectValue: arg.SubjectValue}, nil
	}
	return db.BlockedSubject{}, pgx.ErrNoRows
}

func (r *guardRepo) ListIPRules(ctx context.Context) ([]db.IpRule, error) { return nil, nil }

func (r *guardRepo) GetReputation(ctx context.Context, arg db.GetReputationParams) (db.IpReputation, error) {
	return db.IpReputation{}, pgx.ErrNoRows
}

func (r *guardRepo) BumpReputation(ctx context.Context, arg db.BumpReputationParams) (db.IpReputation, error) {
	return db.IpReputation{}, nil
}

// guardSettingsRepo returns explicit toggle values for guard tests.
type guardSettingsRepo struct {
	rows []db.PlatformSetting
}

func (f *guardSettingsRepo) ListSettings(ctx context.Context) ([]db.PlatformSetting, error) {
	return f.rows, nil
}

func allDisabledSettings() []db.PlatformSetting {
	return []db.PlatformSetting{
		{Key: abuse.SettingBlocklistEnabled, Value: "false"},
		{Key: abuse.SettingRateLimitEnabled, Value: "false"},
		{Key: abuse.SettingIPReputationEnabled, Value: "false"},
		{Key: abuse.SettingTurnstileEnabled, Value: "false"},
	}
}

func blocklistOnSettings() []db.PlatformSetting {
	rows := allDisabledSettings()
	rows[0].Value = "true" // blocklist_enabled
	return rows
}

func turnstileOnSettings() []db.PlatformSetting {
	rows := allDisabledSettings()
	rows[3].Value = "true" // turnstile_enabled
	return rows
}

// noopLogger satisfies AbuseLogger.
type noopLogger struct{}

func (n *noopLogger) Log(ctx context.Context, e abuse.AbuseEvent) {}

// noopCap satisfies QueueCapChecker — always within cap.
type noopCap struct{}

func (n *noopCap) WithinQueueCap(ctx context.Context, userID uuid.UUID) (bool, error) {
	return true, nil
}

// buildGuard constructs a Guard from a settings row slice and a blocked-subjects map.
func buildGuard(t *testing.T, settingRows []db.PlatformSetting, blocked map[string]bool) *abuse.Guard {
	t.Helper()

	repo := &guardRepo{blocked: blocked}

	s := abuse.NewSettings(&guardSettingsRepo{rows: settingRows})
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("settings.Refresh: %v", err)
	}

	bl := abuse.NewBlocklist(repo)
	rate := abuse.NewRateChecker(ratelimit.New(nil)) // nil Redis → always allow
	rep := abuse.NewReputation(repo, 1000, 2000)     // high thresholds → never triggers
	cv := captcha.FakeVerifier{Pass: true}

	return abuse.NewGuard(s, bl, rate, rep, cv, &noopLogger{}, &noopCap{})
}

// withUserCtx injects an authenticated identity into the request context.
func withUserCtx(r *http.Request, uid uuid.UUID) *http.Request {
	ctx := authctx.WithIdentity(r.Context(), authctx.Identity{UserID: uid})
	return r.WithContext(ctx)
}

func TestGuard_CleanRequestPasses(t *testing.T) {
	g := buildGuard(t, allDisabledSettings(), map[string]bool{})
	uid := uuid.New()

	called := false
	h := g.Middleware(abuse.CategoryQueueJoin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := withUserCtx(httptest.NewRequest("POST", "/events/x/queue/join", nil), uid)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler was not called for a clean request")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGuard_BlockedUserReturns403(t *testing.T) {
	uid := uuid.New()
	g := buildGuard(t, blocklistOnSettings(), map[string]bool{"user:" + uid.String(): true})

	h := g.Middleware(abuse.CategoryQueueJoin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("blocked request must not reach handler")
	}))

	req := withUserCtx(httptest.NewRequest("POST", "/events/x/queue/join", nil), uid)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for blocked user, got %d", rec.Code)
	}
}

func TestGuard_BlocklistDisabled_BlockedUserPasses(t *testing.T) {
	uid := uuid.New()
	// blocklist toggle off — blocked entry in repo must be ignored
	g := buildGuard(t, allDisabledSettings(), map[string]bool{"user:" + uid.String(): true})

	called := false
	h := g.Middleware(abuse.CategoryDefault)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := withUserCtx(httptest.NewRequest("GET", "/", nil), uid)
	req.RemoteAddr = "5.6.7.8:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler should be called when blocklist is disabled")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGuard_TurnstileRequired_MissingToken(t *testing.T) {
	uid := uuid.New()
	// turnstile on, CategoryQueueJoin requires captcha by default
	g := buildGuard(t, turnstileOnSettings(), map[string]bool{})

	h := g.Middleware(abuse.CategoryQueueJoin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request without turnstile token must not reach handler")
	}))

	req := withUserCtx(httptest.NewRequest("POST", "/queue/join", nil), uid)
	req.RemoteAddr = "9.9.9.9:1234"
	// no X-Turnstile-Token header
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing turnstile token, got %d", rec.Code)
	}
}

func TestGuard_TurnstileRequired_ValidToken(t *testing.T) {
	uid := uuid.New()
	g := buildGuard(t, turnstileOnSettings(), map[string]bool{})

	called := false
	h := g.Middleware(abuse.CategoryQueueJoin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := withUserCtx(httptest.NewRequest("POST", "/queue/join", nil), uid)
	req.RemoteAddr = "9.9.9.9:1234"
	req.Header.Set("X-Turnstile-Token", "valid-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("valid turnstile token should allow request through")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
