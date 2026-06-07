# Phase 2 Plan — Part 3: RBAC — authz, roles, members (Tasks 9-12)

> Part of the Phase 2 implementation plan. Index: [2026-06-07-phase2-auth-rbac-multitenant.md](2026-06-07-phase2-auth-rbac-multitenant.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

## Task 9: authz middleware — membership + permission check, super-admin bypass

authz answers "boleh ngapain". It reads the `orgId` URL param and the identity from context (set by authn), then: platform admins bypass; otherwise the caller must be a member of the org AND hold the required permission. It depends on a small `PermissionLoader` interface so it can be tested with a fake.

**Files:**
- Create: `services/api/internal/platform/middleware/authz.go`
- Test: `services/api/internal/platform/middleware/authz_test.go`

- [ ] **Step 1: Write the failing test**

Create `services/api/internal/platform/middleware/authz_test.go`:
```go
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
)

// fakeLoader maps (orgID,userID) -> permission set; missing entry = not a member.
type fakeLoader struct {
	perms map[string]map[string]bool
}

func key(orgID, userID uuid.UUID) string { return orgID.String() + "|" + userID.String() }

func (f *fakeLoader) LoadPermissions(_ context.Context, orgID, userID uuid.UUID) (map[string]bool, bool, error) {
	p, ok := f.perms[key(orgID, userID)]
	return p, ok, nil
}

func serve(t *testing.T, mw func(http.Handler) http.Handler, orgID uuid.UUID, id authctx.Identity, hasID bool) int {
	t.Helper()
	r := chi.NewRouter()
	r.With(mw).Get("/organizations/{orgId}/members", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/organizations/"+orgID.String()+"/members", nil)
	if hasID {
		req = req.WithContext(authctx.WithIdentity(req.Context(), id))
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}

func TestAuthz_AllowsWithPermission(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()
	loader := &fakeLoader{perms: map[string]map[string]bool{
		key(orgID, userID): {"member.manage": true},
	}}
	mw := RequirePermission(loader, "member.manage")
	if code := serve(t, mw, orgID, authctx.Identity{UserID: userID}, true); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
}

func TestAuthz_DeniesWithoutPermission(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()
	loader := &fakeLoader{perms: map[string]map[string]bool{
		key(orgID, userID): {"role.manage": true}, // has a different perm
	}}
	mw := RequirePermission(loader, "member.manage")
	if code := serve(t, mw, orgID, authctx.Identity{UserID: userID}, true); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
}

func TestAuthz_DeniesNonMember(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()
	loader := &fakeLoader{perms: map[string]map[string]bool{}} // not a member
	mw := RequirePermission(loader, "member.manage")
	if code := serve(t, mw, orgID, authctx.Identity{UserID: userID}, true); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
}

func TestAuthz_PlatformAdminBypasses(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()
	loader := &fakeLoader{perms: map[string]map[string]bool{}} // not a member
	mw := RequirePermission(loader, "member.manage")
	id := authctx.Identity{UserID: userID, IsPlatformAdmin: true}
	if code := serve(t, mw, orgID, id, true); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
}

func TestAuthz_RejectsMissingIdentity(t *testing.T) {
	orgID := uuid.New()
	loader := &fakeLoader{perms: map[string]map[string]bool{}}
	mw := RequirePermission(loader, "member.manage")
	if code := serve(t, mw, orgID, authctx.Identity{}, false); code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/middleware/ -run TestAuthz -v; cd ../..
```
Expected: FAIL — `undefined: RequirePermission`.

- [ ] **Step 3: Implement authz middleware**

Create `services/api/internal/platform/middleware/authz.go`:
```go
package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// PermissionLoader returns the caller's permission set in an org. The bool is
// false when the user is not a member of the org.
type PermissionLoader interface {
	LoadPermissions(ctx context.Context, orgID, userID uuid.UUID) (perms map[string]bool, isMember bool, err error)
}

func RequirePermission(loader PermissionLoader, required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := authctx.FromContext(r.Context())
			if !ok {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
				return
			}

			orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
			if err != nil {
				apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
				return
			}

			if id.IsPlatformAdmin {
				next.ServeHTTP(w, r)
				return
			}

			perms, isMember, err := loader.LoadPermissions(r.Context(), orgID, id.UserID)
			if err != nil {
				apperr.WriteError(w, r, err)
				return
			}
			if !isMember {
				apperr.WriteError(w, r, apperr.New(http.StatusForbidden, "FORBIDDEN", "not a member of this organization"))
				return
			}
			if !perms[required] {
				apperr.WriteError(w, r, apperr.New(http.StatusForbidden, "FORBIDDEN", "missing permission: "+required))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/platform/middleware/ -run TestAuthz -v; cd ../..
```
Expected: PASS (all five tests).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/platform/middleware/authz.go services/api/internal/platform/middleware/authz_test.go
git commit -m "feat(api): add authz middleware with membership and permission checks"
```

---

## Task 10: RBAC permission loader + audit-log helper

This task implements the concrete `PermissionLoader` (used to wire authz) and a thin audit writer. Both are small DB-backed adapters with no business branching, so they have no unit tests; they're exercised by the integration tests in Task 13.

**Files:**
- Create: `services/api/internal/platform/rbac/loader.go`
- Create: `services/api/internal/platform/audit/audit.go`

- [ ] **Step 1: Implement the permission loader**

Create `services/api/internal/platform/rbac/loader.go`:
```go
package rbac

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Loader implements middleware.PermissionLoader against the database.
type Loader struct {
	q *db.Queries
}

func NewLoader(q *db.Queries) *Loader { return &Loader{q: q} }

func (l *Loader) LoadPermissions(ctx context.Context, orgID, userID uuid.UUID) (map[string]bool, bool, error) {
	member, err := l.q.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{OrganizationID: orgID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}

	keys, err := l.q.ListPermissionsForMember(ctx, member.ID)
	if err != nil {
		return nil, true, err
	}
	perms := make(map[string]bool, len(keys))
	for _, k := range keys {
		perms[k] = true
	}
	return perms, true, nil
}
```
Note: `ListPermissionsForMember` returns `[]string` (the query selects `p.key`). Confirm against generated `members.sql.go`; if sqlc named the return differently, adjust the loop accordingly.

- [ ] **Step 2: Implement the audit helper**

Create `services/api/internal/platform/audit/audit.go`:
```go
package audit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Logger writes audit entries. Failures are logged, never fatal — an audit
// write must not break the user-facing action.
type Logger struct {
	q   *db.Queries
	log *slog.Logger
}

func NewLogger(q *db.Queries, log *slog.Logger) *Logger {
	return &Logger{q: q, log: log}
}

type Entry struct {
	OrganizationID *uuid.UUID
	ActorUserID    *uuid.UUID
	Action         string
	TargetType     string
	TargetID       string
	Metadata       map[string]any
}

func (l *Logger) Record(ctx context.Context, e Entry) {
	var meta []byte
	if e.Metadata != nil {
		meta, _ = json.Marshal(e.Metadata)
	}
	err := l.q.CreateAuditLog(ctx, db.CreateAuditLogParams{
		OrganizationID: e.OrganizationID,
		ActorUserID:    e.ActorUserID,
		Action:         e.Action,
		TargetType:     nullable(e.TargetType),
		TargetID:       nullable(e.TargetID),
		Metadata:       meta,
	})
	if err != nil {
		l.log.Error("audit write failed", "action", e.Action, "error", err)
	}
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```
Note: `CreateAuditLogParams.Metadata` is `[]byte` for a `jsonb` column under pgx/v5 (sqlc maps `jsonb` to `[]byte` by default). If the generated type is `pgtype.JSONB` or similar, wrap accordingly; the `[]byte` mapping is the pgx/v5 default and what this code assumes. A `nil` slice stores SQL NULL.

- [ ] **Step 3: Verify build**

Run:
```bash
cd services/api && go build ./... && cd ../..
```
Expected: builds with no errors.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/platform/rbac services/api/internal/platform/audit
git commit -m "feat(api): add db-backed permission loader and audit-log helper"
```

---

## Task 11: Roles module — CRUD custom roles, assign permissions, permission catalog

**Files:**
- Create: `services/api/internal/modules/roles/errors.go`
- Create: `services/api/internal/modules/roles/dto.go`
- Create: `services/api/internal/modules/roles/repository.go`
- Create: `services/api/internal/modules/roles/service.go`
- Test: `services/api/internal/modules/roles/service_test.go`

- [ ] **Step 1: Typed errors**

Create `services/api/internal/modules/roles/errors.go`:
```go
package roles

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrRoleNotFound   = apperr.New(http.StatusNotFound, "ROLE_NOT_FOUND", "role not found")
	ErrSystemRole     = apperr.New(http.StatusForbidden, "ROLE_IS_SYSTEM", "cannot modify or delete a system role")
	ErrRoleInUse      = apperr.New(http.StatusConflict, "ROLE_IN_USE", "role is still assigned to members")
	ErrUnknownPerm    = apperr.New(http.StatusBadRequest, "UNKNOWN_PERMISSION", "unknown permission key")
	ErrLastOwner      = apperr.New(http.StatusConflict, "LAST_OWNER", "cannot remove the last owner")
	ErrSlugConflict   = apperr.New(http.StatusConflict, "ROLE_SLUG_TAKEN", "a role with this name already exists")
)
```

- [ ] **Step 2: DTOs**

Create `services/api/internal/modules/roles/dto.go`:
```go
package roles

import "github.com/google/uuid"

type PermissionResponse struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

type RoleResponse struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	IsSystem       bool      `json:"isSystem"`
	PermissionKeys []string  `json:"permissionKeys"`
}

type CreateRoleRequest struct {
	Name           string   `json:"name"`
	PermissionKeys []string `json:"permissionKeys"`
}

type UpdateRoleRequest struct {
	Name           *string   `json:"name"`
	PermissionKeys *[]string `json:"permissionKeys"`
}
```

- [ ] **Step 3: Repository interface + sqlc adapter**

Create `services/api/internal/modules/roles/repository.go`:
```go
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
```
Note: `ListRolesByOrg` takes `*uuid.UUID` because the column is nullable; pass `&orgID`. If sqlc generated a non-pointer `uuid.UUID` param (it shouldn't, given the override), pass `orgID` directly and adjust the interface.

- [ ] **Step 4: Write failing service tests**

Create `services/api/internal/modules/roles/service_test.go`:
```go
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
```
Note: this test treats org-created system roles (`IsSystem=true`) as undeletable. In Task 8 we copy templates with `IsSystem=false`, so in practice org roles are editable — `IsSystem=true` here is a synthetic guard test.

- [ ] **Step 5: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/roles/ -v; cd ../..
```
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 6: Implement the service**

Create `services/api/internal/modules/roles/service.go`:
```go
package roles

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) ListPermissionCatalog(ctx context.Context) ([]PermissionResponse, error) {
	perms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PermissionResponse, 0, len(perms))
	for _, p := range perms {
		out = append(out, PermissionResponse{Key: p.Key, Description: p.Description})
	}
	return out, nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]RoleResponse, error) {
	id := orgID
	roles, err := s.repo.ListRolesByOrg(ctx, &id)
	if err != nil {
		return nil, err
	}
	out := make([]RoleResponse, 0, len(roles))
	for _, r := range roles {
		keys, err := s.permKeysForRole(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, toRoleResponse(r, keys))
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, req CreateRoleRequest) (RoleResponse, error) {
	permIDs, err := s.resolvePermissions(ctx, req.PermissionKeys)
	if err != nil {
		return RoleResponse{}, err
	}
	slug := slugify(req.Name)
	if _, err := s.repo.GetRoleByOrgAndSlug(ctx, db.GetRoleByOrgAndSlugParams{OrganizationID: &orgID, Slug: slug}); err == nil {
		return RoleResponse{}, ErrSlugConflict
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return RoleResponse{}, err
	}

	var created db.Role
	err = s.repo.ExecTx(ctx, func(r Repository) error {
		oid := orgID
		role, err := r.CreateRole(ctx, db.CreateRoleParams{OrganizationID: &oid, Name: req.Name, Slug: slug, IsSystem: false})
		if err != nil {
			return err
		}
		created = role
		for _, pid := range permIDs {
			if err := r.AddRolePermission(ctx, db.AddRolePermissionParams{RoleID: role.ID, PermissionID: pid}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return RoleResponse{}, err
	}
	return toRoleResponse(created, req.PermissionKeys), nil
}

func (s *Service) Update(ctx context.Context, orgID, roleID uuid.UUID, req UpdateRoleRequest) (RoleResponse, error) {
	role, err := s.loadOrgRole(ctx, orgID, roleID)
	if err != nil {
		return RoleResponse{}, err
	}
	if role.IsSystem {
		return RoleResponse{}, ErrSystemRole
	}

	var permIDs []uuid.UUID
	if req.PermissionKeys != nil {
		permIDs, err = s.resolvePermissions(ctx, *req.PermissionKeys)
		if err != nil {
			return RoleResponse{}, err
		}
	}

	err = s.repo.ExecTx(ctx, func(r Repository) error {
		if req.Name != nil {
			if _, err := r.UpdateRoleName(ctx, db.UpdateRoleNameParams{ID: roleID, Name: *req.Name, OrganizationID: &orgID}); err != nil {
				return err
			}
		}
		if req.PermissionKeys != nil {
			if err := r.ClearRolePermissions(ctx, roleID); err != nil {
				return err
			}
			for _, pid := range permIDs {
				if err := r.AddRolePermission(ctx, db.AddRolePermissionParams{RoleID: roleID, PermissionID: pid}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return RoleResponse{}, err
	}
	return s.get(ctx, orgID, roleID)
}

func (s *Service) Delete(ctx context.Context, orgID, roleID uuid.UUID) error {
	role, err := s.loadOrgRole(ctx, orgID, roleID)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return ErrSystemRole
	}
	count, err := s.repo.CountMembersWithRole(ctx, roleID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrRoleInUse
	}
	return s.repo.DeleteRole(ctx, db.DeleteRoleParams{ID: roleID, OrganizationID: &orgID})
}

func (s *Service) get(ctx context.Context, orgID, roleID uuid.UUID) (RoleResponse, error) {
	role, err := s.loadOrgRole(ctx, orgID, roleID)
	if err != nil {
		return RoleResponse{}, err
	}
	keys, err := s.permKeysForRole(ctx, roleID)
	if err != nil {
		return RoleResponse{}, err
	}
	return toRoleResponse(role, keys), nil
}

// loadOrgRole fetches a role and confirms it belongs to the org (tenant isolation).
func (s *Service) loadOrgRole(ctx context.Context, orgID, roleID uuid.UUID) (db.Role, error) {
	role, err := s.repo.GetRoleByID(ctx, roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Role{}, ErrRoleNotFound
	} else if err != nil {
		return db.Role{}, err
	}
	if role.OrganizationID == nil || *role.OrganizationID != orgID {
		return db.Role{}, ErrRoleNotFound
	}
	return role, nil
}

func (s *Service) resolvePermissions(ctx context.Context, keys []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(keys))
	for _, k := range keys {
		p, err := s.repo.GetPermissionByKey(ctx, k)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnknownPerm
		} else if err != nil {
			return nil, err
		}
		ids = append(ids, p.ID)
	}
	return ids, nil
}

func (s *Service) permKeysForRole(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	perms, err := s.repo.ListPermissionsForRole(ctx, roleID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(perms))
	for _, p := range perms {
		keys = append(keys, p.Key)
	}
	return keys, nil
}

func toRoleResponse(r db.Role, keys []string) RoleResponse {
	if keys == nil {
		keys = []string{}
	}
	return RoleResponse{ID: r.ID, Name: r.Name, Slug: r.Slug, IsSystem: r.IsSystem, PermissionKeys: keys}
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
```
Note: the last-Owner guard (`ErrLastOwner`) belongs to the *members* flow (removing a member or changing their roles), implemented in Task 12 — not role deletion. `ErrLastOwner` is declared here in the shared `roles` errors file but consumed by the members service.

- [ ] **Step 7: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/roles/ -v; cd ../..
```
Expected: PASS (all role tests).

- [ ] **Step 8: Commit**

```bash
git add services/api/internal/modules/roles
git commit -m "feat(roles): add roles module with custom-role crud and permission catalog"
```

---

## Task 12: Members module — list, add, remove, update roles (with last-Owner guard)

The last-Owner guard: removing a member or replacing their roles must not drop the org to zero Owners. The service counts Owner-holders and, when the target currently holds Owner, rejects if it would become the last one (spec: "tolak hapus/turunkan Owner terakhir").

**Files:**
- Create: `services/api/internal/modules/members/errors.go`
- Create: `services/api/internal/modules/members/dto.go`
- Create: `services/api/internal/modules/members/repository.go`
- Create: `services/api/internal/modules/members/service.go`
- Test: `services/api/internal/modules/members/service_test.go`

- [ ] **Step 1: Typed errors**

Create `services/api/internal/modules/members/errors.go`:
```go
package members

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrUserNotFound   = apperr.New(http.StatusNotFound, "USER_NOT_FOUND", "no user with that email")
	ErrAlreadyMember  = apperr.New(http.StatusConflict, "ALREADY_MEMBER", "user is already a member")
	ErrMemberNotFound = apperr.New(http.StatusNotFound, "MEMBER_NOT_FOUND", "member not found")
	ErrRoleNotInOrg   = apperr.New(http.StatusBadRequest, "ROLE_NOT_IN_ORG", "one or more roles do not belong to this organization")
	ErrLastOwner      = apperr.New(http.StatusConflict, "LAST_OWNER", "cannot remove or demote the last owner")
)
```

- [ ] **Step 2: DTOs**

Create `services/api/internal/modules/members/dto.go`:
```go
package members

import "github.com/google/uuid"

type MemberResponse struct {
	ID       uuid.UUID   `json:"id"`
	UserID   uuid.UUID   `json:"userId"`
	Email    string      `json:"email"`
	FullName string      `json:"fullName"`
	RoleIDs  []uuid.UUID `json:"roleIds"`
}

type AddMemberRequest struct {
	Email   string      `json:"email"`
	RoleIDs []uuid.UUID `json:"roleIds"`
}

type UpdateRolesRequest struct {
	RoleIDs []uuid.UUID `json:"roleIds"`
}
```

- [ ] **Step 3: Repository interface + sqlc adapter**

Create `services/api/internal/modules/members/repository.go`:
```go
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
	CountOwnersInOrg(ctx context.Context, orgID uuid.UUID) (int64, error)
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
func (r *sqlcRepo) CountOwnersInOrg(ctx context.Context, orgID uuid.UUID) (int64, error) {
	return r.q.CountOwnersInOrg(ctx, orgID)
}
func (r *sqlcRepo) MemberHasRoleSlug(ctx context.Context, arg db.MemberHasRoleSlugParams) (bool, error) {
	return r.q.MemberHasRoleSlug(ctx, arg)
}
func (r *sqlcRepo) GetRoleByID(ctx context.Context, id uuid.UUID) (db.Role, error) {
	return r.q.GetRoleByID(ctx, id)
}
```
Note: `ListMembersByOrgRow` is the generated row type for the joined `ListMembersByOrg` query (selects member fields + `u.email`, `u.full_name`). Confirm the exact name and fields in the generated `members.sql.go` after Task 5 and adjust `toMemberResponse` if column names differ.

- [ ] **Step 4: Write failing service tests**

Create `services/api/internal/modules/members/service_test.go`:
```go
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
func (f *fakeRepo) CountOwnersInOrg(_ context.Context, orgID uuid.UUID) (int64, error) {
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
	svc := NewService(newFakeRepo())
	_, err := svc.Add(context.Background(), uuid.New(), AddMemberRequest{Email: "ghost@x.com"})
	if err != ErrUserNotFound {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestAdd_AssignsRoles(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
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
	svc := NewService(repo)
	orgID := uuid.New()
	otherOrg := uuid.New()
	repo.usersByEmail["staff@x.com"] = db.User{ID: uuid.New(), Email: "staff@x.com", FullName: "Staff"}
	foreignRole := repo.seedRole(otherOrg, "manager") // belongs to a different org

	_, err := svc.Add(context.Background(), orgID, AddMemberRequest{Email: "staff@x.com", RoleIDs: []uuid.UUID{foreignRole.ID}})
	if err != ErrRoleNotInOrg {
		t.Fatalf("err = %v, want ErrRoleNotInOrg", err)
	}
}

func TestRemove_RejectsLastOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	owner := repo.seedRole(orgID, "owner")
	member := db.OrganizationMember{ID: uuid.New(), OrganizationID: orgID, UserID: uuid.New()}
	repo.members[member.ID] = member
	repo.memberRoles[member.ID] = []uuid.UUID{owner.ID}
	repo.ownerCount = 1 // this is the only owner

	if err := svc.Remove(context.Background(), orgID, member.ID); err != ErrLastOwner {
		t.Fatalf("err = %v, want ErrLastOwner", err)
	}
}

func TestUpdateRoles_RejectsDemotingLastOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	owner := repo.seedRole(orgID, "owner")
	manager := repo.seedRole(orgID, "manager")
	member := db.OrganizationMember{ID: uuid.New(), OrganizationID: orgID, UserID: uuid.New()}
	repo.members[member.ID] = member
	repo.memberRoles[member.ID] = []uuid.UUID{owner.ID}
	repo.ownerCount = 1

	// Replacing owner with manager would remove the last owner.
	_, err := svc.UpdateRoles(context.Background(), orgID, member.ID, UpdateRolesRequest{RoleIDs: []uuid.UUID{manager.ID}})
	if err != ErrLastOwner {
		t.Fatalf("err = %v, want ErrLastOwner", err)
	}
}
```

- [ ] **Step 5: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/members/ -v; cd ../..
```
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 6: Implement the service**

Create `services/api/internal/modules/members/service.go`:
```go
package members

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

const ownerSlug = "owner"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

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
	return s.repo.DeleteMember(ctx, db.DeleteMemberParams{ID: memberID, OrganizationID: orgID})
}

func (s *Service) UpdateRoles(ctx context.Context, orgID, memberID uuid.UUID, req UpdateRolesRequest) (MemberResponse, error) {
	member, err := s.loadOrgMember(ctx, orgID, memberID)
	if err != nil {
		return MemberResponse{}, err
	}
	if err := s.assertRolesInOrg(ctx, orgID, req.RoleIDs); err != nil {
		return MemberResponse{}, err
	}

	// If the new role set would strip Owner from the last owner, reject.
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
	count, err := s.repo.CountOwnersInOrg(ctx, orgID)
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
```

- [ ] **Step 7: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/members/ -v; cd ../..
```
Expected: PASS (all member tests).

- [ ] **Step 8: Commit**

```bash
git add services/api/internal/modules/members
git commit -m "feat(members): add members module with role assignment and last-owner guard"
```

---
