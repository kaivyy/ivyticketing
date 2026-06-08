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

// AllowIP returns false when the per-minute IP limit for the category is exceeded.
// Zero limit means no limit → always allowed.
func (rc *RateChecker) AllowIP(ctx context.Context, category, ip string) (bool, error) {
	limit := categoryLimits(category).PerIP
	if limit <= 0 || ip == "" {
		return true, nil
	}
	return rc.lim.Allow(ctx, category+":ip:"+ip, limit, time.Minute)
}

// AllowUser returns false when the per-minute user limit for the category is exceeded.
func (rc *RateChecker) AllowUser(ctx context.Context, category, userID string) (bool, error) {
	limit := categoryLimits(category).PerUser
	if limit <= 0 || userID == "" {
		return true, nil
	}
	return rc.lim.Allow(ctx, category+":user:"+userID, limit, time.Minute)
}
