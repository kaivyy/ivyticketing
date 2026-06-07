package auth

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrEmailExists       = apperr.New(http.StatusConflict, "EMAIL_EXISTS", "email already registered")
	ErrInvalidCredential = apperr.New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
	ErrTokenExpired      = apperr.New(http.StatusUnauthorized, "TOKEN_EXPIRED", "refresh token expired")
	ErrTokenRevoked      = apperr.New(http.StatusUnauthorized, "TOKEN_REVOKED", "refresh token revoked")
	ErrTokenInvalid      = apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "invalid refresh token")
)
