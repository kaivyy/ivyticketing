package organizations

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// fakeRepo holds in-memory state and ignores transactions.
type fakeRepo struct {
	orgs        map[uuid.UUID]db.Organization
	slugs       map[string]bool
	members     []db.OrganizationMember
	templates   []db.Role         // organization_id IS NULL
	tmplPerms   map[uuid.UUID][]db.Permission
	orgRoles    []db.Role         // created copies
	rolePerms   map[uuid.UUID][]uuid.UUID
	memberRoles map[uuid.UUID][]uuid.UUID // memberID -> roleIDs
}

func newFakeRepo() *fakeRepo {
	owner := db.Role{ID: uuid.New(), Name: "Owner", Slug: "owner", IsSystem: true}
	mgr := db.Role{ID: uuid.New(), Name: "Manager", Slug: "manager", IsSystem: true}
	pView := db.Permission{ID: uuid.New(), Key: "member.manage"}
	pRole := db.Permission{ID: uuid.New(), Key: "role.manage"}
	return &fakeRepo{
		orgs:      map[uuid.UUID]db.Organization{},
		slugs:     map[string]bool{},
		templates: []db.Role{owner, mgr},
		tmplPerms: map[uuid.UUID][]db.Permission{
			owner.ID: {pView, pRole},
			mgr.ID:   {pView},
		},
		rolePerms:   map[uuid.UUID][]uuid.UUID{},
		memberRoles: map[uuid.UUID][]uuid.UUID{},
	}
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(Repository) error) error { return fn(f) }

func (f *fakeRepo) CreateOrganization(_ context.Context, arg db.CreateOrganizationParams) (db.Organization, error) {
	if f.slugs[arg.Slug] {
		return db.Organization{}, ErrSlugTaken
	}
	o := db.Organization{ID: uuid.New(), Name: arg.Name, Slug: arg.Slug}
	f.orgs[o.ID] = o
	f.slugs[arg.Slug] = true
	return o, nil
}
func (f *fakeRepo) GetOrganizationByID(_ context.Context, id uuid.UUID) (db.Organization, error) {
	o, ok := f.orgs[id]
	if !ok {
		return db.Organization{}, pgx.ErrNoRows
	}
	return o, nil
}
func (f *fakeRepo) ListOrganizationsForUser(_ context.Context, userID uuid.UUID) ([]db.Organization, error) {
	var out []db.Organization
	for _, m := range f.members {
		if m.UserID == userID {
			out = append(out, f.orgs[m.OrganizationID])
		}
	}
	return out, nil
}
func (f *fakeRepo) GetMemberByOrgAndUser(_ context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error) {
	for _, m := range f.members {
		if m.OrganizationID == arg.OrganizationID && m.UserID == arg.UserID {
			return m, nil
		}
	}
	return db.OrganizationMember{}, pgx.ErrNoRows
}
func (f *fakeRepo) CreateMember(_ context.Context, arg db.CreateMemberParams) (db.OrganizationMember, error) {
	m := db.OrganizationMember{ID: uuid.New(), OrganizationID: arg.OrganizationID, UserID: arg.UserID}
	f.members = append(f.members, m)
	return m, nil
}
func (f *fakeRepo) ListTemplateRoles(_ context.Context) ([]db.Role, error) { return f.templates, nil }
func (f *fakeRepo) ListPermissionsForRole(_ context.Context, roleID uuid.UUID) ([]db.Permission, error) {
	if p, ok := f.tmplPerms[roleID]; ok {
		return p, nil
	}
	return nil, nil
}
func (f *fakeRepo) CreateRole(_ context.Context, arg db.CreateRoleParams) (db.Role, error) {
	r := db.Role{ID: uuid.New(), OrganizationID: arg.OrganizationID, Name: arg.Name, Slug: arg.Slug, IsSystem: arg.IsSystem}
	f.orgRoles = append(f.orgRoles, r)
	return r, nil
}
func (f *fakeRepo) AddRolePermission(_ context.Context, arg db.AddRolePermissionParams) error {
	f.rolePerms[arg.RoleID] = append(f.rolePerms[arg.RoleID], arg.PermissionID)
	return nil
}
func (f *fakeRepo) AddMemberRole(_ context.Context, arg db.AddMemberRoleParams) error {
	f.memberRoles[arg.OrganizationMemberID] = append(f.memberRoles[arg.OrganizationMemberID], arg.RoleID)
	return nil
}

func TestCreate_CopiesTemplatesAndAssignsOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()
	creator := uuid.New()

	org, err := svc.Create(ctx, creator, CreateRequest{Name: "Jakarta Marathon"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if org.Slug != "jakarta-marathon" {
		t.Errorf("slug = %q, want jakarta-marathon", org.Slug)
	}

	// All templates copied as org-owned, is_system=false.
	if len(repo.orgRoles) != len(repo.templates) {
		t.Fatalf("copied %d roles, want %d", len(repo.orgRoles), len(repo.templates))
	}
	for _, r := range repo.orgRoles {
		if r.OrganizationID == nil || *r.OrganizationID != org.ID {
			t.Errorf("copied role %q not owned by org", r.Slug)
		}
		if r.IsSystem {
			t.Errorf("copied role %q should have is_system=false", r.Slug)
		}
	}

	// Creator is a member with the org's Owner role.
	if len(repo.members) != 1 || repo.members[0].UserID != creator {
		t.Fatalf("expected creator to be the sole member")
	}
	memberID := repo.members[0].ID
	var ownerRoleID uuid.UUID
	for _, r := range repo.orgRoles {
		if r.Slug == "owner" {
			ownerRoleID = r.ID
		}
	}
	assigned := repo.memberRoles[memberID]
	if len(assigned) != 1 || assigned[0] != ownerRoleID {
		t.Errorf("creator should be assigned exactly the Owner role")
	}
}

func TestCreate_RejectsDuplicateSlug(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()
	if _, err := svc.Create(ctx, uuid.New(), CreateRequest{Name: "Repeat"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := svc.Create(ctx, uuid.New(), CreateRequest{Name: "Repeat"}); err != ErrSlugTaken {
		t.Fatalf("second create err = %v, want ErrSlugTaken", err)
	}
}
