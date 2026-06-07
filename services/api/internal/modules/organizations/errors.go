package organizations

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrSlugTaken = apperr.New(http.StatusConflict, "SLUG_TAKEN", "organization slug already in use")
	ErrNotFound  = apperr.New(http.StatusNotFound, "ORG_NOT_FOUND", "organization not found")
	ErrForbidden = apperr.New(http.StatusForbidden, "FORBIDDEN", "not a member of this organization")
)
