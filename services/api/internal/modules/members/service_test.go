package members

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type fakeRepo struct {
	usersByEmail map[string]db.User
	members      map[uuid.UUID]db.OrganizationMember
	memberRoles  map[uuid.UUID][]uuid.UUID // memberID -> roleIDs
	roles        map[uuid.UUID]db.Role
	ownerCount   int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		usersByEmail: map[string]db.User{},
		members:      map[uuid.UUID]db.OrganizationMember{},
		memberRoles:  map[uuid.UUID][]uuid.UUID{},
		roles:        map[uuid.UUID]db.Role{},
	}
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(Repository) error) error { return fn(f) }
func (f *fakeRepo) GetUserByEmail(_ context.Context, email string) (db.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}
func (f *fakeRepo) GetMemberByOrgAndUser(_ context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error) {
	for _, m := range f.members {
		if m.OrganizationID == arg.OrganizationID && m.UserID == arg.UserID {
			return m, nil
		}
	}
	return db.OrganizationMember{}, pgx.ErrNoRows
}
func (f *fakeRepo) GetMemberByID(_ context.Context, id uuid.UUID) (db.OrganizationMember, error) {
	m, ok := f.members[id]
	if !ok {
		return db.OrganizationMember{}, pgx.ErrNoRows
	}
	return m, nil
}
func (f *fakeRepo) CreateMember(_ context.Context, arg db.CreateMemberParams) (db.OrganizationMember, error) {
	m := db.OrganizationMember{ID: uuid.New(), OrganizationID: arg.OrganizationID, UserID: arg.UserID}
	f.members[m.ID] = m
	return m, nil
}
func (f *fakeRepo) DeleteMember(_ context.Context, arg db.DeleteMemberParams) error {
	delete(f.members, arg.ID)
	delete(f.memberRoles, arg.ID)
	return nil
}
func (f *fakeRepo) ListMembersByOrg(_ context.Context, orgID uuid.UUID) ([]db.ListMembersByOrgRow, error) {
	return nil, nil
}
func (f *fakeRepo) AddMemberRole(_ context.Context, arg db.AddMemberRoleParams) error {
	f.memberRoles[arg.OrganizationMemberID] = append(f.memberRoles[arg.OrganizationMemberID], arg.RoleID)
	return nil
}
func (f *fakeRepo) ClearMemberRoles(_ context.Context, memberID uuid.UUID) error {
	delete(f.memberRoles, memberID)
	return nil
}
func (f *fakeRepo) ListRolesForMember(_ context.Context, memberID uuid.UUID) ([]db.Role, error) {
	var out []db.Role
	for _, rid := range f.memberRoles[memberID] {
		out = append(out, f.roles[rid])
	}
	return out, nil
}
func (f *fakeRepo) CountOwnersInOrg(_ context.Context, orgID *uuid.UUID) (int64, error) {
	return f.ownerCount, nil
}
func (f *fakeRepo) MemberHasRoleSlug(_ context.Context, arg db.MemberHasRoleSlugParams) (bool, error) {
	for _, rid := range f.memberRoles[arg.OrganizationMemberID] {
		if r, ok := f.roles[rid]; ok && r.Slug == arg.Slug {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeRepo) GetRoleByID(_ context.Context, id uuid.UUID) (db.Role, error) {
	r, ok := f.roles[id]
	if !ok {
		return db.Role{}, pgx.ErrNoRows
	}
	return r, nil
}

// helper: seed a role belonging to an org
func (f *fakeRepo) seedRole(orgID uuid.UUID, slug string) db.Role {
	r := db.Role{ID: uuid.New(), OrganizationID: &orgID, Name: slug, Slug: slug}
	f.roles[r.ID] = r
	return r
}

func TestAdd_RejectsUnknownEmail(t *testing.T) {
	svc := NewService(newFakeRepo(), nil)
	_, err := svc.Add(context.Background(), uuid.New(), AddMemberRequest{Email: "ghost@x.com"})
	if err != ErrUserNotFound {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestAdd_AssignsRoles(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)
	orgID := uuid.New()
	repo.usersByEmail["staff@x.com"] = db.User{ID: uuid.New(), Email: "staff@x.com", FullName: "Staff"}
	role := repo.seedRole(orgID, "manager")

	m, err := svc.Add(context.Background(), orgID, AddMemberRequest{Email: "staff@x.com", RoleIDs: []uuid.UUID{role.ID}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(m.RoleIDs) != 1 || m.RoleIDs[0] != role.ID {
		t.Errorf("roleIds = %v, want [%v]", m.RoleIDs, role.ID)
	}
}

func TestAdd_RejectsRoleFromOtherOrg(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)
	orgID := uuid.New()
	otherOrg := uuid.New()
	repo.usersByEmail["staff@x.com"] = db.User{ID: uuid.New(), Email: "staff@x.com", FullName: "Staff"}
	foreignRole := repo.seedRole(otherOrg, "manager")

	_, err := svc.Add(context.Background(), orgID, AddMemberRequest{Email: "staff@x.com", RoleIDs: []uuid.UUID{foreignRole.ID}})
	if err != ErrRoleNotInOrg {
		t.Fatalf("err = %v, want ErrRoleNotInOrg", err)
	}
}

func TestRemove_RejectsLastOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)
	orgID := uuid.New()
	owner := repo.seedRole(orgID, "owner")
	member := db.OrganizationMember{ID: uuid.New(), OrganizationID: orgID, UserID: uuid.New()}
	repo.members[member.ID] = member
	repo.memberRoles[member.ID] = []uuid.UUID{owner.ID}
	repo.ownerCount = 1

	if err := svc.Remove(context.Background(), orgID, member.ID); err != ErrLastOwner {
		t.Fatalf("err = %v, want ErrLastOwner", err)
	}
}

func TestUpdateRoles_RejectsDemotingLastOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)
	orgID := uuid.New()
	owner := repo.seedRole(orgID, "owner")
	manager := repo.seedRole(orgID, "manager")
	member := db.OrganizationMember{ID: uuid.New(), OrganizationID: orgID, UserID: uuid.New()}
	repo.members[member.ID] = member
	repo.memberRoles[member.ID] = []uuid.UUID{owner.ID}
	repo.ownerCount = 1

	_, err := svc.UpdateRoles(context.Background(), orgID, member.ID, UpdateRolesRequest{RoleIDs: []uuid.UUID{manager.ID}})
	if err != ErrLastOwner {
		t.Fatalf("err = %v, want ErrLastOwner", err)
	}
}
