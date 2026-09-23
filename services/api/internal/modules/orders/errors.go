package orders

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrOrderNotFound      = apperr.New(http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
	ErrCategoryNotFound   = apperr.New(http.StatusNotFound, "CATEGORY_NOT_FOUND", "category not found")
	ErrEventNotPublished  = apperr.New(http.StatusConflict, "EVENT_NOT_PUBLISHED", "event is not published")
	ErrRegistrationClosed = apperr.New(http.StatusConflict, "REGISTRATION_CLOSED", "registration window is closed")
	ErrMaxOrderExceeded   = apperr.New(http.StatusConflict, "MAX_ORDER_EXCEEDED", "maximum orders per user reached for this category")
	ErrDuplicateActive    = apperr.New(http.StatusConflict, "DUPLICATE_ACTIVE_ORDER", "you already have an active order for this category")
	ErrInvalidState        = apperr.New(http.StatusConflict, "INVALID_ORDER_STATE", "order cannot transition from its current state")
	ErrOrderNumberGen      = apperr.New(http.StatusInternalServerError, "ORDER_NUMBER_GENERATION_FAILED", "could not generate a unique order number")
	ErrTermsRequired       = apperr.New(http.StatusBadRequest, "TERMS_REQUIRED", "terms and conditions must be accepted")
	ErrGuestEmailRequired  = apperr.New(http.StatusBadRequest, "GUEST_EMAIL_REQUIRED", "valid guest email is required")
	ErrOrderAlreadyClaimed = apperr.New(http.StatusConflict, "ORDER_ALREADY_CLAIMED", "order has already been claimed by an account")
	ErrGrantAlreadyConsumed = apperr.New(http.StatusConflict, "GRANT_ALREADY_CONSUMED", "access grant already used")
	ErrGrantNotFound        = apperr.New(http.StatusNotFound, "GRANT_NOT_FOUND", "access grant not found")
	ErrGrantExpired         = apperr.New(http.StatusForbidden, "GRANT_EXPIRED", "access grant has expired")
)
