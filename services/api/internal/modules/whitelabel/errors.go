package whitelabel

import apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"

var (
	ErrBrandingNotFound = apperr.New(404, "BRANDING_NOT_FOUND", "organization branding not set")
	ErrDomainNotFound   = apperr.New(404, "DOMAIN_NOT_FOUND", "custom domain not found")
	ErrInvalidOrgID     = apperr.New(400, "INVALID_ORG_ID", "invalid organization id")
	ErrInvalidBranding  = apperr.New(400, "INVALID_BRANDING", "invalid branding payload")
	ErrInvalidDomain    = apperr.New(400, "INVALID_DOMAIN", "invalid domain name")
	ErrDomainTaken      = apperr.New(409, "DOMAIN_TAKEN", "domain is already registered")
	ErrDNSMismatch      = apperr.New(422, "DNS_MISMATCH", "verification token not found in DNS TXT records")
	ErrFeatureLocked    = apperr.New(403, "FEATURE_LOCKED", "white label requires the Enterprise package")
)
