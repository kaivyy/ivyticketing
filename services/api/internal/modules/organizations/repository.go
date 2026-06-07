package organizations

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
	CreateOrganization(ctx context.Context, arg db.CreateOrganizationParams) (db.Organization, error)
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (db.Organization, error)
	ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]db.Organization, error)
	GetMemberByOrgAndUser(ctx context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error)
	CreateMember(ctx context.Context, arg db.CreateMemberParams) (db.OrganizationMember, error)
	ListTemplateRoles(ctx context.Context) ([]db.Role, error)
	ListPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]db.Permission, error)
	CreateRole(ctx context.Context, arg db.CreateRoleParams) (db.Role, error)
	AddRolePermission(ctx context.Context, arg db.AddRolePermissionParams) error
	AddMemberRole(ctx context.Context, arg db.AddMemberRoleParams) error
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
	txRepo := &sqlcRepo{pool: r.pool, q: db.New(tx)}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *sqlcRepo) CreateOrganization(ctx context.Context, arg db.CreateOrganizationParams) (db.Organization, error) {
	return r.q.CreateOrganization(ctx, arg)
}
func (r *sqlcRepo) GetOrganizationByID(ctx context.Context, id uuid.UUID) (db.Organization, error) {
	return r.q.GetOrganizationByID(ctx, id)
}
func (r *sqlcRepo) ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]db.Organization, error) {
	return r.q.ListOrganizationsForUser(ctx, userID)
}
func (r *sqlcRepo) GetMemberByOrgAndUser(ctx context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error) {
	return r.q.GetMemberByOrgAndUser(ctx, arg)
}
func (r *sqlcRepo) CreateMember(ctx context.Context, arg db.CreateMemberParams) (db.OrganizationMember, error) {
	return r.q.CreateMember(ctx, arg)
}
func (r *sqlcRepo) ListTemplateRoles(ctx context.Context) ([]db.Role, error) {
	return r.q.ListTemplateRoles(ctx)
}
func (r *sqlcRepo) ListPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]db.Permission, error) {
	return r.q.ListPermissionsForRole(ctx, roleID)
}
func (r *sqlcRepo) CreateRole(ctx context.Context, arg db.CreateRoleParams) (db.Role, error) {
	return r.q.CreateRole(ctx, arg)
}
func (r *sqlcRepo) AddRolePermission(ctx context.Context, arg db.AddRolePermissionParams) error {
	return r.q.AddRolePermission(ctx, arg)
}
func (r *sqlcRepo) AddMemberRole(ctx context.Context, arg db.AddMemberRoleParams) error {
	return r.q.AddMemberRole(ctx, arg)
}
