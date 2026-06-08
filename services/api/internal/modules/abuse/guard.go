package abuse

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	"github.com/varin/ivyticketing/services/api/internal/platform/captcha"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// AbuseLogger records abuse events (implemented by Service).
type AbuseLogger interface {
	Log(ctx context.Context, e AbuseEvent)
}

// QueueCapChecker enforces the cross-event active queue cap (implemented by Service).
type QueueCapChecker interface {
	WithinQueueCap(ctx context.Context, userID uuid.UUID) (bool, error)
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
