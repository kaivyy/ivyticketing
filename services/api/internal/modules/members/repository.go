package members

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetMemberByOrgAndUser(ctx context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error)
	GetMemberByID(ctx context.Context, id uuid.UUID) (db.OrganizationMember, error)
	CreateMember(ctx context.Context, arg db.CreateMemberParams) (db.OrganizationMember, error)
	DeleteMember(ctx context.Context, arg db.DeleteMemberParams) error
	ListMembersByOrg(ctx context.Context, orgID uuid.UUID) ([]db.ListMembersByOrgRow, error)
	AddMemberRole(ctx context.Context, arg db.AddMemberRoleParams) error
	ClearMemberRoles(ctx context.Context, memberID uuid.UUID) error
	ListRolesForMember(ctx context.Context, memberID uuid.UUID) ([]db.Role, error)
	CountOwnersInOrg(ctx context.Context, orgID *uuid.UUID) (int64, error)
	MemberHasRoleSlug(ctx context.Context, arg db.MemberHasRoleSlugParams) (bool, error)
	GetRoleByID(ctx context.Context, id uuid.UUID) (db.Role, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(&sqlcRepo{pool: r.pool, q: db.New(tx)}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *sqlcRepo) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	return r.q.GetUserByEmail(ctx, email)
}
func (r *sqlcRepo) GetMemberByOrgAndUser(ctx context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error) {
	return r.q.GetMemberByOrgAndUser(ctx, arg)
}
func (r *sqlcRepo) GetMemberByID(ctx context.Context, id uuid.UUID) (db.OrganizationMember, error) {
	return r.q.GetMemberByID(ctx, id)
}
func (r *sqlcRepo) CreateMember(ctx context.Context, arg db.CreateMemberParams) (db.OrganizationMember, error) {
	return r.q.CreateMember(ctx, arg)
}
func (r *sqlcRepo) DeleteMember(ctx context.Context, arg db.DeleteMemberParams) error {
	return r.q.DeleteMember(ctx, arg)
}
func (r *sqlcRepo) ListMembersByOrg(ctx context.Context, orgID uuid.UUID) ([]db.ListMembersByOrgRow, error) {
	return r.q.ListMembersByOrg(ctx, orgID)
}
func (r *sqlcRepo) AddMemberRole(ctx context.Context, arg db.AddMemberRoleParams) error {
	return r.q.AddMemberRole(ctx, arg)
}
func (r *sqlcRepo) ClearMemberRoles(ctx context.Context, memberID uuid.UUID) error {
	return r.q.ClearMemberRoles(ctx, memberID)
}
func (r *sqlcRepo) ListRolesForMember(ctx context.Context, memberID uuid.UUID) ([]db.Role, error) {
	return r.q.ListRolesForMember(ctx, memberID)
}
func (r *sqlcRepo) CountOwnersInOrg(ctx context.Context, orgID *uuid.UUID) (int64, error) {
	return r.q.CountOwnersInOrg(ctx, orgID)
}
func (r *sqlcRepo) MemberHasRoleSlug(ctx context.Context, arg db.MemberHasRoleSlugParams) (bool, error) {
	return r.q.MemberHasRoleSlug(ctx, arg)
}
func (r *sqlcRepo) GetRoleByID(ctx context.Context, id uuid.UUID) (db.Role, error) {
	return r.q.GetRoleByID(ctx, id)
}
