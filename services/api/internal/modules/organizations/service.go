package organizations

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
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
	return s.GetByIdentifier(ctx, orgID.String(), userID, isPlatformAdmin)
}

// GetByIdentifier resolves an organization by either its UUID string or its unique slug.
func (s *Service) GetByIdentifier(ctx context.Context, identifier string, userID uuid.UUID, isPlatformAdmin bool) (Response, error) {
	var org db.Organization
	var err error
	if orgID, parseErr := uuid.Parse(identifier); parseErr == nil {
		org, err = s.repo.GetOrganizationByID(ctx, orgID)
	} else {
		org, err = s.repo.GetOrganizationBySlug(ctx, identifier)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Response{}, ErrNotFound
	} else if err != nil {
		return Response{}, err
	}
	if !isPlatformAdmin {
		if _, err := s.repo.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{OrganizationID: org.ID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
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

func (s *Service) ListAuditLogs(ctx context.Context, orgID uuid.UUID, limit int32) ([]AuditLogResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.repo.ListAuditLogs(ctx, orgID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]AuditLogResponse, 0, len(rows))
	for _, r := range rows {
		var meta map[string]any
		if len(r.Metadata) > 0 {
			_ = json.Unmarshal(r.Metadata, &meta)
		}
		var tt, ti string
		if r.TargetType.Valid {
			tt = r.TargetType.String
		}
		if r.TargetID.Valid {
			ti = r.TargetID.String
		}
		out = append(out, AuditLogResponse{
			ID:             r.ID,
			OrganizationID: r.OrganizationID,
			ActorUserID:    r.ActorUserID,
			ActorEmail:     r.ActorEmail,
			ActorName:      r.ActorName,
			Action:         r.Action,
			TargetType:     tt,
			TargetID:       ti,
			Metadata:       meta,
			CreatedAt:      r.CreatedAt.Time,
		})
	}
	return out, nil
}

func (s *Service) ListAllWithStats(ctx context.Context) ([]OrganizationWithStatsResponse, error) {
	rows, err := s.repo.ListAllOrganizationsWithStats(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationWithStatsResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, OrganizationWithStatsResponse{
			ID:          r.ID,
			Name:        r.Name,
			Slug:        r.Slug,
			CreatedAt:   r.CreatedAt.Time,
			EventCount:  r.EventCount,
			MemberCount: r.MemberCount,
			LeadEmail:   r.LeadEmail,
			LeadName:    r.LeadName,
		})
	}
	return out, nil
}

func (s *Service) AdminCreateOrganizer(ctx context.Context, req AdminCreateOrganizerRequest) (OrganizationWithStatsResponse, error) {
	if req.Name == "" {
		return OrganizationWithStatsResponse{}, errors.New("organization name is required")
	}
	if req.LeadEmail == "" {
		return OrganizationWithStatsResponse{}, errors.New("lead email is required")
	}
	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Name)
	}

	pass := req.LeadPassword
	if pass == "" {
		pass = "Organizer123!"
	}
	hashedPass, err := security.HashPassword(pass)
	if err != nil {
		return OrganizationWithStatsResponse{}, err
	}

	var createdOrg db.Organization
	var leadUser db.User

	err = s.repo.ExecTx(ctx, func(r Repository) error {
		// 1. Get or create user
		u, err := r.GetUserByEmail(ctx, req.LeadEmail)
		if errors.Is(err, pgx.ErrNoRows) {
			fullName := req.LeadFullName
			if fullName == "" {
				fullName = req.Name + " Admin"
			}
			phoneStr := req.LeadPhone
			u, err = r.CreateUser(ctx, db.CreateUserParams{
				Email:        req.LeadEmail,
				PasswordHash: pgtype.Text{String: hashedPass, Valid: true},
				FullName:     fullName,
				Phone:        pgtype.Text{String: phoneStr, Valid: phoneStr != ""},
			})
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		leadUser = u

		// 2. Create organization
		org, err := r.CreateOrganization(ctx, db.CreateOrganizationParams{
			Name: req.Name,
			Slug: slug,
		})
		if err != nil {
			return err
		}
		createdOrg = org

		// 3. Create organization member
		member, err := r.CreateMember(ctx, db.CreateMemberParams{
			OrganizationID: org.ID,
			UserID:         leadUser.ID,
		})
		if err != nil {
			return err
		}

		// 4. Copy template roles and assign owner role
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

		if ownerRoleID != uuid.Nil {
			if err := r.AddMemberRole(ctx, db.AddMemberRoleParams{
				OrganizationMemberID: member.ID,
				RoleID:               ownerRoleID,
			}); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return OrganizationWithStatsResponse{}, err
	}

	return OrganizationWithStatsResponse{
		ID:          createdOrg.ID,
		Name:        createdOrg.Name,
		Slug:        createdOrg.Slug,
		CreatedAt:   createdOrg.CreatedAt.Time,
		EventCount:  0,
		MemberCount: 1,
		LeadEmail:   leadUser.Email,
		LeadName:    leadUser.FullName,
	}, nil
}
