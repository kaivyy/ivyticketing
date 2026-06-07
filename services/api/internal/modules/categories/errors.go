package categories

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrEventNotFound   = apperr.New(http.StatusNotFound, "EVENT_NOT_FOUND", "event not found")
	ErrNotFound        = apperr.New(http.StatusNotFound, "CATEGORY_NOT_FOUND", "category not found")
	ErrNameTaken       = apperr.New(http.StatusConflict, "CATEGORY_NAME_TAKEN", "a category with this name already exists for the event")
	ErrInvalidPrice    = apperr.New(http.StatusBadRequest, "INVALID_PRICE", "price must be >= 0")
	ErrInvalidCapacity = apperr.New(http.StatusBadRequest, "INVALID_CAPACITY", "capacity must be > 0")
	ErrInvalidWindow   = apperr.New(http.StatusBadRequest, "INVALID_REGISTRATION_WINDOW", "registration opens must be before closes")
	ErrInvalidAge      = apperr.New(http.StatusBadRequest, "INVALID_AGE", "min age must be >= 0")
	ErrInvalidMaxOrder = apperr.New(http.StatusBadRequest, "INVALID_MAX_ORDER", "max order per user must be >= 1")
)
