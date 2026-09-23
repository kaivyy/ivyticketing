package registration

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrInvalidMode      = apperr.New(http.StatusBadRequest, "INVALID_REGISTRATION_MODE", "unknown registration mode")
	ErrEventNotFound    = apperr.New(http.StatusNotFound, "EVENT_NOT_FOUND", "event not found")
	ErrCategoryNotFound = apperr.New(http.StatusNotFound, "CATEGORY_NOT_FOUND", "category not found")
)
