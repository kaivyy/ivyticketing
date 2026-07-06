package abuse

// Subject types.
const (
	SubjectUser = "user"
	SubjectIP   = "ip"
)

// Setting keys (platform_settings).
const (
	SettingTurnstileEnabled        = "turnstile_enabled"
	SettingRateLimitEnabled        = "rate_limit_enabled"
	SettingIPReputationEnabled     = "ip_reputation_enabled"
	SettingBlocklistEnabled        = "blocklist_enabled"
	SettingCodeBruteForceBlock     = "code_brute_force_block"
	SettingCodeBruteForceWindow    = "code_brute_force_window"    // seconds
	SettingCodeBruteForceMaxTries  = "code_brute_force_max_tries"
	SettingCodeBruteForceBlockDur  = "code_brute_force_block_dur" // seconds
)

// Endpoint categories.
const (
	CategoryQueueJoin    = "queue_join"
	CategoryCheckout     = "checkout"
	CategoryAuthLogin    = "auth_login"
	CategoryAuthRegister = "auth_register"
	CategoryBallotApply  = "ballot_apply"
	CategoryAccessRedeem = "access_redeem"
	CategoryRacepackPickup = "racepack_pickup"
	CategoryRacepackProblem = "racepack_problem"
	CategoryDefault      = "default"
)

// abuse_log actions.
const (
	ActionRateLimited    = "RATE_LIMITED"
	ActionBlockedHit     = "BLOCKED_HIT"
	ActionCaptchaFail    = "CAPTCHA_FAIL"
	ActionDuplicateQueue = "DUPLICATE_QUEUE"
	ActionReputationDeny = "REPUTATION_DENY"
	ActionBlockSet              = "BLOCK_SET"
	ActionUnblock               = "UNBLOCK"
	ActionCodeBruteForceBlock   = "CODE_BRUTE_FORCE_BLOCK"
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
	case CategoryBallotApply:
		return RateLimit{PerIP: 10, PerUser: 3}
	case CategoryAccessRedeem:
		return RateLimit{PerIP: 20, PerUser: 5}
	default:
		return RateLimit{PerIP: 120, PerUser: 0}
	}
}
