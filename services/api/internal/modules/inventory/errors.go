package inventory

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrSoldOut  = apperr.New(http.StatusConflict, "SOLD_OUT", "no remaining capacity for this category")
	ErrCategory = apperr.New(http.StatusNotFound, "CATEGORY_NOT_FOUND", "category not found")
)
