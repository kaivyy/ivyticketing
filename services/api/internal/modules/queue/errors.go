package queue

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrNotEnabled        = apperr.New(http.StatusConflict, "QUEUE_NOT_ENABLED", "queue is not enabled for this event")
	ErrTokenNotFound     = apperr.New(http.StatusNotFound, "QUEUE_TOKEN_NOT_FOUND", "queue token not found")
	ErrNotAllowed        = apperr.New(http.StatusForbidden, "QUEUE_NOT_ALLOWED", "not yet released to checkout")
	ErrAdmissionRequired = apperr.New(http.StatusForbidden, "ADMISSION_REQUIRED", "queue admission required")
	ErrAdmissionExpired  = apperr.New(http.StatusForbidden, "ADMISSION_EXPIRED", "checkout window expired")
)
