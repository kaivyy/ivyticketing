package registration

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrInvalidMode = apperr.New(http.StatusBadRequest, "INVALID_REGISTRATION_MODE", "unknown registration mode")
)
