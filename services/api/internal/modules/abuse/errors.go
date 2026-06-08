package abuse

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrUserBlocked      = apperr.New(http.StatusForbidden, "USER_BLOCKED", "access blocked")
	ErrRateLimited      = apperr.New(http.StatusTooManyRequests, "RATE_LIMITED", "too many requests, slow down")
	ErrCaptchaRequired  = apperr.New(http.StatusForbidden, "CAPTCHA_REQUIRED", "captcha verification required")
	ErrCaptchaInvalid   = apperr.New(http.StatusForbidden, "CAPTCHA_INVALID", "captcha verification failed")
	ErrReputationDenied = apperr.New(http.StatusForbidden, "REPUTATION_DENIED", "request denied")
	ErrQueueCapExceeded = apperr.New(http.StatusTooManyRequests, "QUEUE_ENTRY_CAP_EXCEEDED", "too many active queue entries")
	ErrInvalidSetting   = apperr.New(http.StatusBadRequest, "INVALID_SETTING", "invalid setting key or value")
)
