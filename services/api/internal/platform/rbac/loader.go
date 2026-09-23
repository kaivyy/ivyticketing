package rbac

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Loader implements middleware.PermissionLoader against the database.
type Loader struct {
	q *db.Queries
}

func NewLoader(q *db.Queries) *Loader { return &Loader{q: q} }

func (l *Loader) LoadPermissions(ctx context.Context, orgID, userID uuid.UUID) (map[string]bool, bool, error) {
	member, err := l.q.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{OrganizationID: orgID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}

	keys, err := l.q.ListPermissionsForMember(ctx, member.ID)
	if err != nil {
		return nil, true, err
	}
	perms := make(map[string]bool, len(keys))
	for _, k := range keys {
		perms[k] = true
	}
	return perms, true, nil
}

// ResolveOrgSlug resolves an organization slug or ID string to its UUID.
func (l *Loader) ResolveOrgSlug(ctx context.Context, identifier string) (uuid.UUID, error) {
	if parsed, err := uuid.Parse(identifier); err == nil {
		return parsed, nil
	}
	org, err := l.q.GetOrganizationBySlug(ctx, identifier)
	if err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

// ResolveEventSlug resolves an event slug or ID string to its UUID and its canonical organization UUID.
func (l *Loader) ResolveEventSlug(ctx context.Context, orgID uuid.UUID, identifier string) (uuid.UUID, uuid.UUID, error) {
	if parsed, err := uuid.Parse(identifier); err == nil {
		ev, err := l.q.GetEventByID(ctx, parsed)
		if err == nil {
			return ev.ID, ev.OrganizationID, nil
		}
		return parsed, orgID, nil
	}
	if orgID != uuid.Nil {
		ev, err := l.q.GetEventByOrgAndSlug(ctx, db.GetEventByOrgAndSlugParams{
			OrganizationID: orgID,
			Slug:           identifier,
		})
		if err == nil {
			return ev.ID, ev.OrganizationID, nil
		}
	}
	evGlobal, err := l.q.GetEventBySlug(ctx, identifier)
	if err == nil {
		return evGlobal.ID, evGlobal.OrganizationID, nil
	}
	return uuid.Nil, uuid.Nil, err
}
