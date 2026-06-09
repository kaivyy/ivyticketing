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
)
