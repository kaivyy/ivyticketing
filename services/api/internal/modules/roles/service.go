package roles

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) ListPermissionCatalog(ctx context.Context) ([]PermissionResponse, error) {
	perms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PermissionResponse, 0, len(perms))
	for _, p := range perms {
		out = append(out, PermissionResponse{Key: p.Key, Description: p.Description})
	}
	return out, nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]RoleResponse, error) {
	id := orgID
	roles, err := s.repo.ListRolesByOrg(ctx, &id)
	if err != nil {
		return nil, err
	}
	out := make([]RoleResponse, 0, len(roles))
	for _, r := range roles {
		keys, err := s.permKeysForRole(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, toRoleResponse(r, keys))
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, req CreateRoleRequest) (RoleResponse, error) {
	permIDs, err := s.resolvePermissions(ctx, req.PermissionKeys)
	if err != nil {
		return RoleResponse{}, err
	}
	slug := slugify(req.Name)
	if _, err := s.repo.GetRoleByOrgAndSlug(ctx, db.GetRoleByOrgAndSlugParams{OrganizationID: &orgID, Slug: slug}); err == nil {
		return RoleResponse{}, ErrSlugConflict
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return RoleResponse{}, err
	}

	var created db.Role
	err = s.repo.ExecTx(ctx, func(r Repository) error {
		oid := orgID
		role, err := r.CreateRole(ctx, db.CreateRoleParams{OrganizationID: &oid, Name: req.Name, Slug: slug, IsSystem: false})
		if err != nil {
			return err
		}
		created = role
		for _, pid := range permIDs {
			if err := r.AddRolePermission(ctx, db.AddRolePermissionParams{RoleID: role.ID, PermissionID: pid}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return RoleResponse{}, err
	}
	return toRoleResponse(created, req.PermissionKeys), nil
}

func (s *Service) Update(ctx context.Context, orgID, roleID uuid.UUID, req UpdateRoleRequest) (RoleResponse, error) {
	role, err := s.loadOrgRole(ctx, orgID, roleID)
	if err != nil {
		return RoleResponse{}, err
	}
	if role.IsSystem {
		return RoleResponse{}, ErrSystemRole
	}

	var permIDs []uuid.UUID
	if req.PermissionKeys != nil {
		permIDs, err = s.resolvePermissions(ctx, *req.PermissionKeys)
		if err != nil {
			return RoleResponse{}, err
		}
	}

	err = s.repo.ExecTx(ctx, func(r Repository) error {
		if req.Name != nil {
			if _, err := r.UpdateRoleName(ctx, db.UpdateRoleNameParams{ID: roleID, Name: *req.Name, OrganizationID: &orgID}); err != nil {
				return err
			}
		}
		if req.PermissionKeys != nil {
			if err := r.ClearRolePermissions(ctx, roleID); err != nil {
				return err
			}
			for _, pid := range permIDs {
				if err := r.AddRolePermission(ctx, db.AddRolePermissionParams{RoleID: roleID, PermissionID: pid}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return RoleResponse{}, err
	}
	return s.get(ctx, orgID, roleID)
}

func (s *Service) Delete(ctx context.Context, orgID, roleID uuid.UUID) error {
	role, err := s.loadOrgRole(ctx, orgID, roleID)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return ErrSystemRole
	}
	count, err := s.repo.CountMembersWithRole(ctx, roleID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrRoleInUse
	}
	return s.repo.DeleteRole(ctx, db.DeleteRoleParams{ID: roleID, OrganizationID: &orgID})
}

func (s *Service) get(ctx context.Context, orgID, roleID uuid.UUID) (RoleResponse, error) {
	role, err := s.loadOrgRole(ctx, orgID, roleID)
	if err != nil {
		return RoleResponse{}, err
	}
	keys, err := s.permKeysForRole(ctx, roleID)
	if err != nil {
		return RoleResponse{}, err
	}
	return toRoleResponse(role, keys), nil
}

// loadOrgRole fetches a role and confirms it belongs to the org (tenant isolation).
func (s *Service) loadOrgRole(ctx context.Context, orgID, roleID uuid.UUID) (db.Role, error) {
	role, err := s.repo.GetRoleByID(ctx, roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Role{}, ErrRoleNotFound
	} else if err != nil {
		return db.Role{}, err
	}
	if role.OrganizationID == nil || *role.OrganizationID != orgID {
		return db.Role{}, ErrRoleNotFound
	}
	return role, nil
}

func (s *Service) resolvePermissions(ctx context.Context, keys []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(keys))
	for _, k := range keys {
		p, err := s.repo.GetPermissionByKey(ctx, k)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnknownPerm
		} else if err != nil {
			return nil, err
		}
		ids = append(ids, p.ID)
	}
	return ids, nil
}

func (s *Service) permKeysForRole(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	perms, err := s.repo.ListPermissionsForRole(ctx, roleID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(perms))
	for _, p := range perms {
		keys = append(keys, p.Key)
	}
	return keys, nil
}

func toRoleResponse(r db.Role, keys []string) RoleResponse {
	if keys == nil {
		keys = []string{}
	}
	return RoleResponse{ID: r.ID, Name: r.Name, Slug: r.Slug, IsSystem: r.IsSystem, PermissionKeys: keys}
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
