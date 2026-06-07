package middleware

import (
	"net/http"
	"strings"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
)

func Authn(signer *security.JWTSigner) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "missing or malformed Authorization header"))
				return
			}
			claims, err := signer.Verify(parts[1])
			if err != nil {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "invalid access token"))
				return
			}
			ctx := authctx.WithIdentity(r.Context(), authctx.Identity{
				UserID:          claims.UserID,
				IsPlatformAdmin: claims.IsPlatformAdmin,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
