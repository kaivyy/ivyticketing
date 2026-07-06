package billing

import apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"

var (
	ErrPackageNotFound      = apperr.New(404, "PACKAGE_NOT_FOUND", "subscription package not found")
	ErrSubscriptionNotFound = apperr.New(404, "SUBSCRIPTION_NOT_FOUND", "organization has no subscription")
	ErrInvoiceNotFound      = apperr.New(404, "INVOICE_NOT_FOUND", "invoice not found")
	ErrInvalidPackage       = apperr.New(400, "INVALID_PACKAGE", "invalid package payload")
	ErrInvalidOrgID         = apperr.New(400, "INVALID_ORG_ID", "invalid organization id")
	ErrEventLimitReached    = apperr.New(403, "EVENT_LIMIT_REACHED", "event limit for the current package has been reached")
)
