package middleware

import (
	"net/http"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// RequirePlatformAdmin allows only platform (super) admins. Requires Authn upstream.
func RequirePlatformAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := authctx.FromContext(r.Context())
			if !ok {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
				return
			}
			if !id.IsPlatformAdmin {
				apperr.WriteError(w, r, apperr.New(http.StatusForbidden, "FORBIDDEN", "platform admin only"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
