package organizations

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, creatorID uuid.UUID, req CreateRequest) (Response, error) {
	slug := slugify(req.Name)
	var created db.Organization

	err := s.repo.ExecTx(ctx, func(r Repository) error {
		org, err := r.CreateOrganization(ctx, db.CreateOrganizationParams{Name: req.Name, Slug: slug})
		if err != nil {
			return err
		}
		created = org

		member, err := r.CreateMember(ctx, db.CreateMemberParams{OrganizationID: org.ID, UserID: creatorID})
		if err != nil {
			return err
		}

		templates, err := r.ListTemplateRoles(ctx)
		if err != nil {
			return err
		}

		orgID := org.ID
		var ownerRoleID uuid.UUID
		for _, tmpl := range templates {
			perms, err := r.ListPermissionsForRole(ctx, tmpl.ID)
			if err != nil {
				return err
			}
			copied, err := r.CreateRole(ctx, db.CreateRoleParams{
				OrganizationID: &orgID,
				Name:           tmpl.Name,
				Slug:           tmpl.Slug,
				IsSystem:       false,
			})
			if err != nil {
				return err
			}
			for _, p := range perms {
				if err := r.AddRolePermission(ctx, db.AddRolePermissionParams{RoleID: copied.ID, PermissionID: p.ID}); err != nil {
					return err
				}
			}
			if copied.Slug == "owner" {
				ownerRoleID = copied.ID
			}
		}

		return r.AddMemberRole(ctx, db.AddMemberRoleParams{OrganizationMemberID: member.ID, RoleID: ownerRoleID})
	})
	if err != nil {
		return Response{}, err
	}
	return toResponse(created), nil
}

func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]Response, error) {
	orgs, err := s.repo.ListOrganizationsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Response, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, toResponse(o))
	}
	return out, nil
}

// Get returns the org if the caller is a member or a platform admin.
func (s *Service) Get(ctx context.Context, orgID, userID uuid.UUID, isPlatformAdmin bool) (Response, error) {
	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Response{}, ErrNotFound
	} else if err != nil {
		return Response{}, err
	}
	if !isPlatformAdmin {
		if _, err := s.repo.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{OrganizationID: orgID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
			return Response{}, ErrForbidden
		} else if err != nil {
			return Response{}, err
		}
	}
	return toResponse(org), nil
}

func toResponse(o db.Organization) Response {
	return Response{ID: o.ID, Name: o.Name, Slug: o.Slug, CreatedAt: o.CreatedAt.Time}
}
