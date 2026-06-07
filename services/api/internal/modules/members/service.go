package members

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

const ownerSlug = "owner"

// AuditRecorder records sensitive actions.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type Service struct {
	repo  Repository
	audit AuditRecorder
}

func NewService(repo Repository, recorder AuditRecorder) *Service {
	return &Service{repo: repo, audit: recorder}
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]MemberResponse, error) {
	rows, err := s.repo.ListMembersByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberResponse, 0, len(rows))
	for _, row := range rows {
		roles, err := s.repo.ListRolesForMember(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, MemberResponse{
			ID:       row.ID,
			UserID:   row.UserID,
			Email:    row.Email,
			FullName: row.FullName,
			RoleIDs:  roleIDs(roles),
		})
	}
	return out, nil
}

func (s *Service) Add(ctx context.Context, orgID uuid.UUID, req AddMemberRequest) (MemberResponse, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return MemberResponse{}, ErrUserNotFound
	} else if err != nil {
		return MemberResponse{}, err
	}

	if _, err := s.repo.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{OrganizationID: orgID, UserID: user.ID}); err == nil {
		return MemberResponse{}, ErrAlreadyMember
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return MemberResponse{}, err
	}

	if err := s.assertRolesInOrg(ctx, orgID, req.RoleIDs); err != nil {
		return MemberResponse{}, err
	}

	var member db.OrganizationMember
	err = s.repo.ExecTx(ctx, func(r Repository) error {
		m, err := r.CreateMember(ctx, db.CreateMemberParams{OrganizationID: orgID, UserID: user.ID})
		if err != nil {
			return err
		}
		member = m
		for _, rid := range req.RoleIDs {
			if err := r.AddMemberRole(ctx, db.AddMemberRoleParams{OrganizationMemberID: m.ID, RoleID: rid}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return MemberResponse{}, err
	}
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			Action:         "member.add",
			TargetType:     "member",
			TargetID:       member.ID.String(),
		})
	}
	return MemberResponse{ID: member.ID, UserID: user.ID, Email: user.Email, FullName: user.FullName, RoleIDs: req.RoleIDs}, nil
}

func (s *Service) Remove(ctx context.Context, orgID, memberID uuid.UUID) error {
	member, err := s.loadOrgMember(ctx, orgID, memberID)
	if err != nil {
		return err
	}
	if err := s.guardLastOwner(ctx, orgID, member.ID, false); err != nil {
		return err
	}
	if err := s.repo.DeleteMember(ctx, db.DeleteMemberParams{ID: memberID, OrganizationID: orgID}); err != nil {
		return err
	}
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			Action:         "member.remove",
			TargetType:     "member",
			TargetID:       memberID.String(),
		})
	}
	return nil
}

func (s *Service) UpdateRoles(ctx context.Context, orgID, memberID uuid.UUID, req UpdateRolesRequest) (MemberResponse, error) {
	member, err := s.loadOrgMember(ctx, orgID, memberID)
	if err != nil {
		return MemberResponse{}, err
	}
	if err := s.assertRolesInOrg(ctx, orgID, req.RoleIDs); err != nil {
		return MemberResponse{}, err
	}

	// If the new role set retains Owner, the last-owner guard is satisfied.
	newlyOwner := false
	for _, rid := range req.RoleIDs {
		role, err := s.repo.GetRoleByID(ctx, rid)
		if err != nil {
			return MemberResponse{}, err
		}
		if role.Slug == ownerSlug {
			newlyOwner = true
			break
		}
	}
	if !newlyOwner {
		if err := s.guardLastOwner(ctx, orgID, member.ID, false); err != nil {
			return MemberResponse{}, err
		}
	}

	err = s.repo.ExecTx(ctx, func(r Repository) error {
		if err := r.ClearMemberRoles(ctx, member.ID); err != nil {
			return err
		}
		for _, rid := range req.RoleIDs {
			if err := r.AddMemberRole(ctx, db.AddMemberRoleParams{OrganizationMemberID: member.ID, RoleID: rid}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return MemberResponse{}, err
	}
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			Action:         "member.roles.update",
			TargetType:     "member",
			TargetID:       memberID.String(),
		})
	}
	return MemberResponse{ID: member.ID, UserID: member.UserID, RoleIDs: req.RoleIDs}, nil
}

// guardLastOwner rejects an operation that would leave the org with zero owners,
// when the target member currently holds the Owner role and is the only one.
func (s *Service) guardLastOwner(ctx context.Context, orgID, memberID uuid.UUID, targetWillKeepOwner bool) error {
	if targetWillKeepOwner {
		return nil
	}
	isOwner, err := s.repo.MemberHasRoleSlug(ctx, db.MemberHasRoleSlugParams{OrganizationMemberID: memberID, Slug: ownerSlug})
	if err != nil {
		return err
	}
	if !isOwner {
		return nil
	}
	oid := orgID
	count, err := s.repo.CountOwnersInOrg(ctx, &oid)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastOwner
	}
	return nil
}

func (s *Service) assertRolesInOrg(ctx context.Context, orgID uuid.UUID, roleIDs []uuid.UUID) error {
	for _, rid := range roleIDs {
		role, err := s.repo.GetRoleByID(ctx, rid)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotInOrg
		} else if err != nil {
			return err
		}
		if role.OrganizationID == nil || *role.OrganizationID != orgID {
			return ErrRoleNotInOrg
		}
	}
	return nil
}

func (s *Service) loadOrgMember(ctx context.Context, orgID, memberID uuid.UUID) (db.OrganizationMember, error) {
	m, err := s.repo.GetMemberByID(ctx, memberID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.OrganizationMember{}, ErrMemberNotFound
	} else if err != nil {
		return db.OrganizationMember{}, err
	}
	if m.OrganizationID != orgID {
		return db.OrganizationMember{}, ErrMemberNotFound
	}
	return m, nil
}

func roleIDs(roles []db.Role) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	return ids
}
