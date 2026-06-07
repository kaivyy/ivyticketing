package roles

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
	ListPermissions(ctx context.Context) ([]db.Permission, error)
	GetPermissionByKey(ctx context.Context, key string) (db.Permission, error)
	ListRolesByOrg(ctx context.Context, orgID *uuid.UUID) ([]db.Role, error)
	ListPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]db.Permission, error)
	GetRoleByID(ctx context.Context, id uuid.UUID) (db.Role, error)
	GetRoleByOrgAndSlug(ctx context.Context, arg db.GetRoleByOrgAndSlugParams) (db.Role, error)
	CreateRole(ctx context.Context, arg db.CreateRoleParams) (db.Role, error)
	UpdateRoleName(ctx context.Context, arg db.UpdateRoleNameParams) (db.Role, error)
	DeleteRole(ctx context.Context, arg db.DeleteRoleParams) error
	AddRolePermission(ctx context.Context, arg db.AddRolePermissionParams) error
	ClearRolePermissions(ctx context.Context, roleID uuid.UUID) error
	CountMembersWithRole(ctx context.Context, roleID uuid.UUID) (int64, error)
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

func (r *sqlcRepo) ListPermissions(ctx context.Context) ([]db.Permission, error) {
	return r.q.ListPermissions(ctx)
}
func (r *sqlcRepo) GetPermissionByKey(ctx context.Context, key string) (db.Permission, error) {
	return r.q.GetPermissionByKey(ctx, key)
}
func (r *sqlcRepo) ListRolesByOrg(ctx context.Context, orgID *uuid.UUID) ([]db.Role, error) {
	return r.q.ListRolesByOrg(ctx, orgID)
}
func (r *sqlcRepo) ListPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]db.Permission, error) {
	return r.q.ListPermissionsForRole(ctx, roleID)
}
func (r *sqlcRepo) GetRoleByID(ctx context.Context, id uuid.UUID) (db.Role, error) {
	return r.q.GetRoleByID(ctx, id)
}
func (r *sqlcRepo) GetRoleByOrgAndSlug(ctx context.Context, arg db.GetRoleByOrgAndSlugParams) (db.Role, error) {
	return r.q.GetRoleByOrgAndSlug(ctx, arg)
}
func (r *sqlcRepo) CreateRole(ctx context.Context, arg db.CreateRoleParams) (db.Role, error) {
	return r.q.CreateRole(ctx, arg)
}
func (r *sqlcRepo) UpdateRoleName(ctx context.Context, arg db.UpdateRoleNameParams) (db.Role, error) {
	return r.q.UpdateRoleName(ctx, arg)
}
func (r *sqlcRepo) DeleteRole(ctx context.Context, arg db.DeleteRoleParams) error {
	return r.q.DeleteRole(ctx, arg)
}
func (r *sqlcRepo) AddRolePermission(ctx context.Context, arg db.AddRolePermissionParams) error {
	return r.q.AddRolePermission(ctx, arg)
}
func (r *sqlcRepo) ClearRolePermissions(ctx context.Context, roleID uuid.UUID) error {
	return r.q.ClearRolePermissions(ctx, roleID)
}
func (r *sqlcRepo) CountMembersWithRole(ctx context.Context, roleID uuid.UUID) (int64, error) {
	return r.q.CountMembersWithRole(ctx, roleID)
}
