package abuse

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	"github.com/varin/ivyticketing/services/api/internal/platform/captcha"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
)

// AbuseLogger records abuse events (implemented by Service).
type AbuseLogger interface {
	Log(ctx context.Context, e AbuseEvent)
}

// QueueCapChecker enforces the cross-event active queue cap (implemented by Service).
type QueueCapChecker interface {
	WithinQueueCap(ctx context.Context, userID uuid.UUID) (bool, error)
}

// ipBlocker is the narrow repo surface Guard needs to write a temporary IP block.
type ipBlocker interface {
	UpsertBlockedSubject(ctx context.Context, arg db.UpsertBlockedSubjectParams) (db.BlockedSubject, error)
}

// AbuseEvent carries context for a single abuse log entry.
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
	lim       *ratelimit.Limiter // Redis INCR/EXPIRE for brute-force tracking
	blockRepo ipBlocker          // writes temporary IP blocks to DB
}

// NewGuard constructs a Guard with all dependencies injected.
func NewGuard(
	settings *Settings,
	blocklist *Blocklist,
	rate *RateChecker,
	rep *Reputation,
	captchaVerifier captcha.Verifier,
	logger AbuseLogger,
	qcap QueueCapChecker,
) *Guard {
	return &Guard{
		settings:  settings,
		blocklist: blocklist,
		rate:      rate,
		rep:       rep,
		captcha:   captchaVerifier,
		logger:    logger,
		cap:       qcap,
	}
}

// WithBruteForce attaches a Redis limiter and blocklist writer used for
// code brute-force detection and temporary IP blocking.
func (g *Guard) WithBruteForce(lim *ratelimit.Limiter, repo ipBlocker) *Guard {
	g.lim = lim
	g.blockRepo = repo
	return g
}

// Middleware returns the guard chain for an endpoint category.
// Chain: blocklist → rate limit → reputation → turnstile → queue cap.
// Each step is skipped when its toggle is disabled.
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

			// 1. Blocklist — deny explicitly blocked users / IPs.
			if g.settings.IsEnabled(SettingBlocklistEnabled) {
				if g.blocklist.IsBlocked(ctx, userID, ip) {
					g.logEvent(ctx, ActionBlockedHit, category, fp, ip, userPtr)
					apperr.WriteError(w, r, ErrUserBlocked)
					return
				}
			}

			// 2. Rate limit — per-IP and per-user fixed-window counters.
			if g.settings.IsEnabled(SettingRateLimitEnabled) {
				okIP, _ := g.rate.AllowIP(ctx, category, ip)
				okUser, _ := g.rate.AllowUser(ctx, category, userID)
				if !okIP || !okUser {
					g.bump(ctx, ip, BumpRateViolation, "rate")
					g.logEvent(ctx, ActionRateLimited, category, fp, ip, userPtr)
					w.Header().Set("Retry-After", "60")
					apperr.WriteError(w, r, ErrRateLimited)
					return
				}
			}

			// 3. Reputation gate — deny high-score IPs; escalate to challenge.
			challengeRequired := categoryNeedsCaptcha(category)
			if g.settings.IsEnabled(SettingIPReputationEnabled) {
				if g.rep.ShouldDeny(ctx, SubjectIP, ip) {
					g.logEvent(ctx, ActionReputationDeny, category, fp, ip, userPtr)
					apperr.WriteError(w, r, ErrReputationDenied)
					return
				}
				if g.rep.ShouldChallenge(ctx, SubjectIP, ip) {
					challengeRequired = true
				}
			}

			// 4. Turnstile — verify captcha when enabled and required.
			if g.settings.IsEnabled(SettingTurnstileEnabled) && challengeRequired {
				token := r.Header.Get("X-Turnstile-Token")
				if token == "" {
					apperr.WriteError(w, r, ErrCaptchaRequired)
					return
				}
				ok, err := g.captcha.Verify(ctx, token, ip)
				if err != nil {
					// fail-open on verifier error: let request through
				} else if !ok {
					g.bump(ctx, ip, BumpCaptchaFail, "captcha")
					g.logEvent(ctx, ActionCaptchaFail, category, fp, ip, userPtr)
					apperr.WriteError(w, r, ErrCaptchaInvalid)
					return
				}
			}

			// 5. Queue entry cap — enforce per-user active entry limit.
			if category == CategoryQueueJoin && g.cap != nil && userPtr != nil {
				within, err := g.cap.WithinQueueCap(ctx, *userPtr)
				if err == nil && !within {
					g.logEvent(ctx, ActionDuplicateQueue, category, fp, ip, userPtr)
					apperr.WriteError(w, r, ErrQueueCapExceeded)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TrackCodeFailure increments the brute-force counter for ip. When the count
// reaches CodeBruteForceMaxTries within the window, the IP is auto-blocked.
// Fail-open: no-ops when Redis or blockRepo is unavailable.
func (g *Guard) TrackCodeFailure(ctx context.Context, ip string) {
	if !g.settings.IsEnabled(SettingCodeBruteForceBlock) {
		return
	}
	if g.lim == nil {
		return
	}
	window := time.Duration(g.settings.GetInt(SettingCodeBruteForceWindow)) * time.Second
	maxTries := g.settings.GetInt(SettingCodeBruteForceMaxTries)
	blockDur := time.Duration(g.settings.GetInt(SettingCodeBruteForceBlockDur)) * time.Second

	count, err := g.lim.IncrExpire(ctx, "code_fail:"+ip, window)
	if err != nil {
		return // fail-open
	}
	if int(count) >= maxTries && g.blockRepo != nil {
		g.blockIPTemporary(ctx, ip, blockDur)
		g.logEvent(ctx, ActionCodeBruteForceBlock, CategoryAccessRedeem, "", ip, nil)
	}
}

// BumpReputation increments the IP reputation score by delta when reputation is enabled.
func (g *Guard) BumpReputation(ctx context.Context, ip string, delta int) {
	g.bump(ctx, ip, delta, "code_fail")
}

// blockIPTemporary writes a time-limited block entry for the IP into the DB.
func (g *Guard) blockIPTemporary(ctx context.Context, ip string, dur time.Duration) {
	if g.blockRepo == nil {
		return
	}
	expiresAt := pgtype.Timestamptz{Time: timeNow().Add(dur), Valid: true}
	_, _ = g.blockRepo.UpsertBlockedSubject(ctx, db.UpsertBlockedSubjectParams{
		SubjectType:  SubjectIP,
		SubjectValue: ip,
		Reason:       pgtype.Text{String: "code_brute_force", Valid: true},
		ExpiresAt:    expiresAt,
	})
}

// logEvent emits an AbuseEvent, preferring user subject over IP when available.
func (g *Guard) logEvent(ctx context.Context, action, category, fp, ip string, userPtr *uuid.UUID) {
	if g.logger == nil {
		return
	}
	st, sv := SubjectIP, ip
	if userPtr != nil {
		st, sv = SubjectUser, userPtr.String()
	}
	g.logger.Log(ctx, AbuseEvent{
		SubjectType:  st,
		SubjectValue: sv,
		Action:       action,
		Category:     category,
		Fingerprint:  fp,
		IP:           ip,
		UserID:       userPtr,
	})
}

// bump increments the IP reputation score when reputation is enabled.
func (g *Guard) bump(ctx context.Context, ip string, delta int, reason string) {
	if g.settings.IsEnabled(SettingIPReputationEnabled) {
		g.rep.Bump(ctx, SubjectIP, ip, delta, reason)
	}
}

// timeNow is a package-level variable to allow testing.
var timeNow = func() time.Time { return time.Now() }
