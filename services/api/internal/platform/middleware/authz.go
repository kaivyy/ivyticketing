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

// OrgSlugResolver resolves an organization slug or ID string to a canonical UUID.
type OrgSlugResolver interface {
	ResolveOrgSlug(ctx context.Context, identifier string) (uuid.UUID, error)
}

// EventSlugResolver resolves an event slug or ID string to a canonical event UUID and org UUID.
type EventSlugResolver interface {
	ResolveEventSlug(ctx context.Context, orgID uuid.UUID, identifier string) (eventID uuid.UUID, canonicalOrgID uuid.UUID, err error)
}

func resolveOrgParam(r *http.Request, loader PermissionLoader) (uuid.UUID, error) {
	raw := chi.URLParam(r, "orgId")
	if orgID, err := uuid.Parse(raw); err == nil {
		return orgID, nil
	}
	if resolver, ok := loader.(OrgSlugResolver); ok {
		orgID, err := resolver.ResolveOrgSlug(r.Context(), raw)
		if err == nil {
			// Rewrite route param so downstream handlers receive the canonical UUID
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				for i, k := range rctx.URLParams.Keys {
					if k == "orgId" {
						rctx.URLParams.Values[i] = orgID.String()
						break
					}
				}
			}
			return orgID, nil
		}
	}
	return uuid.Nil, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id")
}

func resolveEventAndOrgParams(r *http.Request, loader PermissionLoader, orgID uuid.UUID, isPlatformAdmin bool) (uuid.UUID, uuid.UUID, error) {
	rawEvent := chi.URLParam(r, "eventId")
	if rawEvent == "" {
		return orgID, uuid.Nil, nil
	}

	resolver, ok := loader.(EventSlugResolver)
	if !ok {
		parsedEvent, err := uuid.Parse(rawEvent)
		if err != nil {
			return orgID, uuid.Nil, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id")
		}
		return orgID, parsedEvent, nil
	}

	eventID, canonicalOrgID, err := resolver.ResolveEventSlug(r.Context(), orgID, rawEvent)
	if err != nil {
		return orgID, uuid.Nil, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id")
	}

	effectiveOrgID := orgID
	if isPlatformAdmin && canonicalOrgID != uuid.Nil && canonicalOrgID != orgID {
		effectiveOrgID = canonicalOrgID
	}

	// Rewrite route params so downstream handlers receive canonical UUIDs
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		for i, k := range rctx.URLParams.Keys {
			if k == "eventId" {
				rctx.URLParams.Values[i] = eventID.String()
			}
			if isPlatformAdmin && canonicalOrgID != uuid.Nil && k == "orgId" {
				rctx.URLParams.Values[i] = canonicalOrgID.String()
			}
		}
	}

	return effectiveOrgID, eventID, nil
}

func RequirePermission(loader PermissionLoader, required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := authctx.FromContext(r.Context())
			if !ok {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
				return
			}

			orgID, err := resolveOrgParam(r, loader)
			if err != nil {
				apperr.WriteError(w, r, err)
				return
			}

			orgID, _, err = resolveEventAndOrgParams(r, loader, orgID, id.IsPlatformAdmin)
			if err != nil {
				apperr.WriteError(w, r, err)
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

			orgID, err := resolveOrgParam(r, loader)
			if err != nil {
				apperr.WriteError(w, r, err)
				return
			}

			orgID, _, err = resolveEventAndOrgParams(r, loader, orgID, id.IsPlatformAdmin)
			if err != nil {
				apperr.WriteError(w, r, err)
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
