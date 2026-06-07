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
