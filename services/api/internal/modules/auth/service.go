package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
)

type Service struct {
	repo       Repository
	signer     *security.JWTSigner
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewService(repo Repository, signer *security.JWTSigner, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, signer: signer, accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (UserResponse, error) {
	if _, err := s.repo.GetUserByEmail(ctx, req.Email); err == nil {
		return UserResponse{}, ErrEmailExists
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return UserResponse{}, err
	}

	hash, err := security.HashPassword(req.Password)
	if err != nil {
		return UserResponse{}, err
	}
	u, err := s.repo.CreateUser(ctx, db.CreateUserParams{
		Email:        req.Email,
		PasswordHash: pgtype.Text{String: hash, Valid: true},
		FullName:     req.FullName,
		Phone:        nullablePgText(req.Phone),
	})
	if err != nil {
		return UserResponse{}, err
	}
	return toUserResponse(u), nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResult, error) {
	u, err := s.repo.GetUserByEmail(ctx, req.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginResult{}, ErrInvalidCredential
	} else if err != nil {
		return LoginResult{}, err
	}
	if !u.PasswordHash.Valid || !security.VerifyPassword(u.PasswordHash.String, req.Password) {
		return LoginResult{}, ErrInvalidCredential
	}

	access, raw, err := s.issueTokens(ctx, u)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		AccessToken:  access,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		User:         toUserResponse(u),
		RefreshToken: raw,
		RefreshTTL:   int(s.refreshTTL.Seconds()),
	}, nil
}

func (s *Service) Refresh(ctx context.Context, rawToken string) (RefreshResult, error) {
	if rawToken == "" {
		return RefreshResult{}, ErrTokenInvalid
	}
	stored, err := s.repo.GetRefreshTokenByHash(ctx, security.HashToken(rawToken))
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshResult{}, ErrTokenInvalid
	} else if err != nil {
		return RefreshResult{}, err
	}
	if stored.RevokedAt.Valid {
		return RefreshResult{}, ErrTokenRevoked
	}
	if time.Now().After(stored.ExpiresAt.Time) {
		return RefreshResult{}, ErrTokenExpired
	}

	// Rotate: revoke old, issue new.
	if err := s.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return RefreshResult{}, err
	}
	u, err := s.repo.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return RefreshResult{}, err
	}
	access, raw, err := s.issueTokens(ctx, u)
	if err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{
		AccessToken:  access,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		RefreshToken: raw,
		RefreshTTL:   int(s.refreshTTL.Seconds()),
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	stored, err := s.repo.GetRefreshTokenByHash(ctx, security.HashToken(rawToken))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	return s.repo.RevokeRefreshToken(ctx, stored.ID)
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (db.User, error) {
	return s.repo.GetUserByID(ctx, id)
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (MeResponse, error) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return MeResponse{}, err
	}
	orgs, err := s.repo.ListOrganizationsForUser(ctx, userID)
	if err != nil {
		return MeResponse{}, err
	}
	memberships := make([]MembershipResponse, 0, len(orgs))
	for _, org := range orgs {
		member, err := s.repo.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{OrganizationID: org.ID, UserID: userID})
		if err != nil {
			return MeResponse{}, err
		}
		roles, err := s.repo.ListRolesForMember(ctx, member.ID)
		if err != nil {
			return MeResponse{}, err
		}
		perms, err := s.repo.ListPermissionsForMember(ctx, member.ID)
		if err != nil {
			return MeResponse{}, err
		}
		slugs := make([]string, 0, len(roles))
		for _, r := range roles {
			slugs = append(slugs, r.Slug)
		}
		if perms == nil {
			perms = []string{}
		}
		memberships = append(memberships, MembershipResponse{
			OrganizationID: org.ID,
			MemberID:       member.ID,
			RoleSlugs:      slugs,
			Permissions:    perms,
		})
	}
	return MeResponse{User: toUserResponse(u), Memberships: memberships}, nil
}

func (s *Service) issueTokens(ctx context.Context, u db.User) (access, raw string, err error) {
	access, err = s.signer.Sign(u.ID, u.IsPlatformAdmin)
	if err != nil {
		return "", "", err
	}
	raw, hash, err := security.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}
	if _, err = s.repo.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(s.refreshTTL), Valid: true},
	}); err != nil {
		return "", "", err
	}
	return access, raw, nil
}

func toUserResponse(u db.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, FullName: u.FullName, Phone: u.Phone.String}
}

func nullablePgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}
