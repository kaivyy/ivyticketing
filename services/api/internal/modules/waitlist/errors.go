package waitlist

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrWaitlistNotFound  = apperr.New(http.StatusNotFound, "WAITLIST_NOT_FOUND", "waitlist not found")
	ErrAlreadyOnWaitlist = apperr.New(http.StatusConflict, "ALREADY_ON_WAITLIST", "already on this waitlist")
	ErrNotOnWaitlist     = apperr.New(http.StatusNotFound, "NOT_ON_WAITLIST", "not on this waitlist")
	ErrWaitlistClosed    = apperr.New(http.StatusConflict, "WAITLIST_CLOSED", "waitlist is closed")
)
