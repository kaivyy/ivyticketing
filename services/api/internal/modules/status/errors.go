package status

import apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"

var (
	ErrIncidentNotFound = apperr.New(404, "INCIDENT_NOT_FOUND", "incident not found")
	ErrInvalidIncident  = apperr.New(400, "INVALID_INCIDENT", "invalid incident payload")
	ErrInvalidComponent = apperr.New(400, "INVALID_COMPONENT", "invalid status component payload")
	ErrInvalidID        = apperr.New(400, "INVALID_ID", "invalid id")
)
