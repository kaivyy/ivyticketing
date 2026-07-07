package enterprise

import apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"

var (
	ErrInvalidOrgID      = apperr.New(400, "INVALID_ORG_ID", "invalid organization id")
	ErrInvalidPayload    = apperr.New(400, "INVALID_PAYLOAD", "invalid request payload")
	ErrAPIKeyNotFound    = apperr.New(404, "API_KEY_NOT_FOUND", "api key not found")
	ErrWebhookNotFound   = apperr.New(404, "WEBHOOK_NOT_FOUND", "webhook endpoint not found")
	ErrInvalidAPIKey     = apperr.New(401, "INVALID_API_KEY", "missing or invalid API key")
	ErrForbiddenScope    = apperr.New(403, "FORBIDDEN_SCOPE", "API key lacks the required scope")
	ErrRateLimited       = apperr.New(429, "RATE_LIMITED", "API key rate limit exceeded")
	ErrResourceNotFound  = apperr.New(404, "RESOURCE_NOT_FOUND", "resource not found")
	ErrInvalidWebhookURL = apperr.New(400, "INVALID_WEBHOOK_URL", "webhook url must be a valid https URL")
)
