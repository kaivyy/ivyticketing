package enterprise

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
)

// APIContext carries the authenticated API key's identity through the request
// context so downstream handlers can scope reads to the owning org and enforce
// per-key scopes without re-reading the DB.
type APIContext struct {
	KeyID  uuid.UUID
	OrgID  uuid.UUID
	Scopes map[string]bool
}

type apiCtxKey struct{}

func withAPIContext(ctx context.Context, ac APIContext) context.Context {
	return context.WithValue(ctx, apiCtxKey{}, ac)
}

// APIContextFrom returns the API key identity set by Authenticator, if present.
func APIContextFrom(ctx context.Context) (APIContext, bool) {
	ac, ok := ctx.Value(apiCtxKey{}).(APIContext)
	return ac, ok
}

// Authenticator builds middleware that authenticates public-API requests by
// API key and enforces a per-key fixed-window rate limit. The key is read from
// the "X-API-Key" header, falling back to a "Bearer" Authorization header.
//
// On success it injects an APIContext (org + scopes) so scoped handlers can
// authorize reads. Rate limiting is fail-open (Redis outage never blocks a
// legitimate integrator) but the per-key ceiling caps abuse when Redis is up.
func (s *Service) Authenticator(limiter *ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := extractAPIKey(r)
			if raw == "" {
				apperr.WriteError(w, r, ErrInvalidAPIKey)
				return
			}
			key, err := s.Authenticate(r.Context(), raw)
			if err != nil {
				apperr.WriteError(w, r, ErrInvalidAPIKey)
				return
			}

			// Per-key fixed-window rate limit (limit is per minute).
			allowed, _ := limiter.Allow(r.Context(),
				"apikey:"+key.ID.String(),
				int(key.RateLimitPerMin),
				time.Minute)
			if !allowed {
				apperr.WriteError(w, r, ErrRateLimited)
				return
			}

			ctx := withAPIContext(r.Context(), APIContext{
				KeyID:  key.ID,
				OrgID:  key.OrganizationID,
				Scopes: scopeSet(key),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope builds middleware that rejects a request whose API key lacks the
// named scope. Must be mounted after Authenticator.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := APIContextFrom(r.Context())
			if !ok {
				apperr.WriteError(w, r, ErrInvalidAPIKey)
				return
			}
			if !ac.Scopes[scope] {
				apperr.WriteError(w, r, ErrForbiddenScope)
				return
			}
			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}

// extractAPIKey pulls the raw key from X-API-Key or a Bearer Authorization header.
func extractAPIKey(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-API-Key")); v != "" {
		return v
	}
	header := r.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// scopeSet decodes a key's stored scopes into a lookup set.
func scopeSet(k db.ApiKey) map[string]bool {
	set := map[string]bool{}
	for _, sc := range decodeScopes(k.Scopes) {
		set[sc] = true
	}
	return set
}
