package roles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type fakeRepo struct {
	perms       []db.Permission
	roles       map[uuid.UUID]db.Role
	rolePerms   map[uuid.UUID][]db.Permission
	memberCount map[uuid.UUID]int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		perms: []db.Permission{
			{ID: uuid.New(), Key: "member.manage"},
			{ID: uuid.New(), Key: "role.manage"},
		},
		roles:       map[uuid.UUID]db.Role{},
		rolePerms:   map[uuid.UUID][]db.Permission{},
		memberCount: map[uuid.UUID]int64{},
	}
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(Repository) error) error { return fn(f) }
func (f *fakeRepo) ListPermissions(context.Context) ([]db.Permission, error)     { return f.perms, nil }
func (f *fakeRepo) GetPermissionByKey(_ context.Context, key string) (db.Permission, error) {
	for _, p := range f.perms {
		if p.Key == key {
			return p, nil
		}
	}
	return db.Permission{}, pgx.ErrNoRows
}
func (f *fakeRepo) ListRolesByOrg(_ context.Context, orgID *uuid.UUID) ([]db.Role, error) {
	var out []db.Role
	for _, r := range f.roles {
		out = append(out, r)
	}
	return out, nil
}
func (f *fakeRepo) ListPermissionsForRole(_ context.Context, roleID uuid.UUID) ([]db.Permission, error) {
	return f.rolePerms[roleID], nil
}
func (f *fakeRepo) GetRoleByID(_ context.Context, id uuid.UUID) (db.Role, error) {
	r, ok := f.roles[id]
	if !ok {
		return db.Role{}, pgx.ErrNoRows
	}
	return r, nil
}
func (f *fakeRepo) GetRoleByOrgAndSlug(_ context.Context, arg db.GetRoleByOrgAndSlugParams) (db.Role, error) {
	for _, r := range f.roles {
		if r.Slug == arg.Slug {
			return r, nil
		}
	}
	return db.Role{}, pgx.ErrNoRows
}
func (f *fakeRepo) CreateRole(_ context.Context, arg db.CreateRoleParams) (db.Role, error) {
	r := db.Role{ID: uuid.New(), OrganizationID: arg.OrganizationID, Name: arg.Name, Slug: arg.Slug, IsSystem: arg.IsSystem}
	f.roles[r.ID] = r
	return r, nil
}
func (f *fakeRepo) UpdateRoleName(_ context.Context, arg db.UpdateRoleNameParams) (db.Role, error) {
	r := f.roles[arg.ID]
	r.Name = arg.Name
	f.roles[arg.ID] = r
	return r, nil
}
func (f *fakeRepo) DeleteRole(_ context.Context, arg db.DeleteRoleParams) error {
	delete(f.roles, arg.ID)
	return nil
}
func (f *fakeRepo) AddRolePermission(_ context.Context, arg db.AddRolePermissionParams) error {
	for _, p := range f.perms {
		if p.ID == arg.PermissionID {
			f.rolePerms[arg.RoleID] = append(f.rolePerms[arg.RoleID], p)
		}
	}
	return nil
}
func (f *fakeRepo) ClearRolePermissions(_ context.Context, roleID uuid.UUID) error {
	delete(f.rolePerms, roleID)
	return nil
}
func (f *fakeRepo) CountMembersWithRole(_ context.Context, roleID uuid.UUID) (int64, error) {
	return f.memberCount[roleID], nil
}

func TestCreateRole_RejectsUnknownPermission(t *testing.T) {
	svc := NewService(newFakeRepo())
	orgID := uuid.New()
	_, err := svc.Create(context.Background(), orgID, CreateRoleRequest{Name: "Volunteer", PermissionKeys: []string{"does.not.exist"}})
	if err != ErrUnknownPerm {
		t.Fatalf("err = %v, want ErrUnknownPerm", err)
	}
}

func TestCreateRole_AssignsPermissions(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	role, err := svc.Create(context.Background(), orgID, CreateRoleRequest{Name: "Volunteer", PermissionKeys: []string{"member.manage"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(role.PermissionKeys) != 1 || role.PermissionKeys[0] != "member.manage" {
		t.Errorf("permissionKeys = %v, want [member.manage]", role.PermissionKeys)
	}
	if role.Slug != "volunteer" {
		t.Errorf("slug = %q, want volunteer", role.Slug)
	}
}

func TestDeleteRole_RejectsInUse(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	role, _ := svc.Create(context.Background(), orgID, CreateRoleRequest{Name: "Volunteer", PermissionKeys: nil})
	repo.memberCount[role.ID] = 2 // still assigned

	if err := svc.Delete(context.Background(), orgID, role.ID); err != ErrRoleInUse {
		t.Fatalf("err = %v, want ErrRoleInUse", err)
	}
}

func TestDeleteRole_RejectsSystem(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	sysRole := db.Role{ID: uuid.New(), OrganizationID: &orgID, Name: "Owner", Slug: "owner", IsSystem: true}
	repo.roles[sysRole.ID] = sysRole

	if err := svc.Delete(context.Background(), orgID, sysRole.ID); err != ErrSystemRole {
		t.Fatalf("err = %v, want ErrSystemRole", err)
	}
}

func TestDeleteRole_NotFound(t *testing.T) {
	svc := NewService(newFakeRepo())
	if err := svc.Delete(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("err = %v, want ErrRoleNotFound", err)
	}
}
