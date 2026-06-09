package lifecycle

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrLifecycleNotFound = apperr.New(http.StatusNotFound, "LIFECYCLE_NOT_FOUND", "lifecycle not found")
	ErrLifecyclePaused   = apperr.New(http.StatusConflict, "LIFECYCLE_PAUSED", "registration is paused")
	ErrPhaseNotActive    = apperr.New(http.StatusConflict, "LIFECYCLE_PHASE_NOT_ACTIVE", "registration phase not active")
	ErrInvalidTransition = apperr.New(http.StatusConflict, "LIFECYCLE_INVALID_TRANSITION", "invalid lifecycle status transition")
	ErrAlreadyActive     = apperr.New(http.StatusConflict, "LIFECYCLE_ALREADY_ACTIVE", "only one active lifecycle per category")
)
