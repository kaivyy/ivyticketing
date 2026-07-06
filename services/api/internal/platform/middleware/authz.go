package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// PermissionLoader returns the caller's permission set in an org. The bool is
// false when the user is not a member of the org.
type PermissionLoader interface {
	LoadPermissions(ctx context.Context, orgID, userID uuid.UUID) (perms map[string]bool, isMember bool, err error)
}

func RequirePermission(loader PermissionLoader, required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := authctx.FromContext(r.Context())
			if !ok {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
				return
			}

			orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
			if err != nil {
				apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
				return
			}

			if id.IsPlatformAdmin {
				next.ServeHTTP(w, r)
				return
			}

			perms, isMember, err := loader.LoadPermissions(r.Context(), orgID, id.UserID)
			if err != nil {
				apperr.WriteError(w, r, err)
				return
			}
			if !isMember {
				apperr.WriteError(w, r, apperr.New(http.StatusForbidden, "FORBIDDEN", "not a member of this organization"))
				return
			}
			if !perms[required] {
				apperr.WriteError(w, r, apperr.New(http.StatusForbidden, "FORBIDDEN", "missing permission: "+required))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission mirrors RequirePermission but passes when the caller
// holds ANY one of the required permissions (logical OR). It runs the same
// authentication check, orgId parse, platform-admin bypass, and org-membership
// check; the only difference is the permission gate. A caller that is
// authenticated and a member of the org but holds none of the required
// permissions receives a 403.
//
// This exists so an endpoint readable by more than one role (e.g. /scan/verify,
// readable by both racepack.execute pickup staff and checkin.execute gate
// staff) can be gated by a single middleware rather than mounting a copy per
// permission.
func RequireAnyPermission(loader PermissionLoader, required ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := authctx.FromContext(r.Context())
			if !ok {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
				return
			}

			orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
			if err != nil {
				apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
				return
			}

			if id.IsPlatformAdmin {
				next.ServeHTTP(w, r)
				return
			}

			perms, isMember, err := loader.LoadPermissions(r.Context(), orgID, id.UserID)
			if err != nil {
				apperr.WriteError(w, r, err)
				return
			}
			if !isMember {
				apperr.WriteError(w, r, apperr.New(http.StatusForbidden, "FORBIDDEN", "not a member of this organization"))
				return
			}
			for _, p := range required {
				if perms[p] {
					next.ServeHTTP(w, r)
					return
				}
			}
			apperr.WriteError(w, r, apperr.New(http.StatusForbidden, "FORBIDDEN", "missing any of required permissions: "+strings.Join(required, ", ")))
		})
	}
}
