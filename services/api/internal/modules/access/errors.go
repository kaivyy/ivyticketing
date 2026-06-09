package access

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrPoolExhausted        = apperr.New(http.StatusConflict, "POOL_EXHAUSTED", "no available slots in pool")
	ErrGrantNotFound        = apperr.New(http.StatusNotFound, "GRANT_NOT_FOUND", "access grant not found")
	ErrGrantExpired         = apperr.New(http.StatusForbidden, "GRANT_EXPIRED", "access grant has expired")
	ErrGrantAlreadyConsumed = apperr.New(http.StatusConflict, "GRANT_ALREADY_CONSUMED", "access grant already used")

	ErrCorporateNotFound        = apperr.New(http.StatusNotFound, "CORPORATE_NOT_FOUND", "corporate account not found")
	ErrCorporateNotApproved     = apperr.New(http.StatusForbidden, "CORPORATE_NOT_APPROVED", "corporate account not yet approved")
	ErrMemberNotInPool          = apperr.New(http.StatusForbidden, "MEMBER_NOT_IN_POOL", "not a member of this access pool")
	ErrPoolTransferInsufficient = apperr.New(http.StatusConflict, "POOL_TRANSFER_INSUFFICIENT", "source pool has insufficient available slots")

	// Access code errors
	ErrCodeNotFound  = apperr.New(http.StatusNotFound, "CODE_NOT_FOUND", "access code not found")
	ErrCodeExpired   = apperr.New(http.StatusForbidden, "CODE_EXPIRED", "access code has expired")
	ErrCodeExhausted = apperr.New(http.StatusConflict, "CODE_EXHAUSTED", "access code has reached its maximum uses")
	ErrNotEligible   = apperr.New(http.StatusForbidden, "NOT_ELIGIBLE", "you are not eligible to use this access code")
)
