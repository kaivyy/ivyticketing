package forms

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrEventNotFound    = apperr.New(http.StatusNotFound, "EVENT_NOT_FOUND", "event not found")
	ErrFormNotFound     = apperr.New(http.StatusNotFound, "FORM_NOT_FOUND", "form not found")
	ErrFieldNotFound    = apperr.New(http.StatusNotFound, "FIELD_NOT_FOUND", "field not found")
	ErrFieldReferenced  = apperr.New(http.StatusConflict, "FIELD_REFERENCED", "field is referenced by another field's conditional")
	ErrInvalidReorder   = apperr.New(http.StatusBadRequest, "INVALID_REORDER_SET", "reorder set must match exactly the form's fields")
	ErrCategoryNotInEvt = apperr.New(http.StatusBadRequest, "CATEGORY_NOT_IN_EVENT", "category does not belong to this event")
)
