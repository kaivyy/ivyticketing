# Phase 2 Plan — Part 4: Wiring & Verification (Tasks 13-16)

> Part of the Phase 2 implementation plan. Index: [2026-06-07-phase2-auth-rbac-multitenant.md](2026-06-07-phase2-auth-rbac-multitenant.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

## Task 13: Handlers, routes, and full wiring in server + main

This task adds HTTP handlers for organizations/members/roles, then assembles every module into the router with the right authn/authz middleware, and updates `main.go` to build dependencies.

**Files:**
- Create: `services/api/internal/modules/organizations/handler.go`
- Create: `services/api/internal/modules/organizations/routes.go`
- Create: `services/api/internal/modules/members/handler.go`
- Create: `services/api/internal/modules/members/routes.go`
- Create: `services/api/internal/modules/roles/handler.go`
- Create: `services/api/internal/modules/roles/routes.go`
- Modify: `services/api/internal/app/server.go`
- Modify: `services/api/cmd/api/main.go`

- [ ] **Step 1: Organizations handler**

Create `services/api/internal/modules/organizations/handler.go`:
```go
package organizations

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	id, _ := authctx.FromContext(r.Context())
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name is required"))
		return
	}
	org, err := h.svc.Create(r.Context(), id.UserID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, org)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	id, _ := authctx.FromContext(r.Context())
	orgs, err := h.svc.ListForUser(r.Context(), id.UserID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, orgs)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, _ := authctx.FromContext(r.Context())
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return
	}
	org, err := h.svc.Get(r.Context(), orgID, id.UserID, id.IsPlatformAdmin)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, org)
}
```

- [ ] **Step 2: Organizations routes**

Create `services/api/internal/modules/organizations/routes.go`:
```go
package organizations

import "github.com/go-chi/chi/v5"

// RegisterRoutes mounts org-level endpoints. The parent router must already be
// behind authn. Member/role sub-routes are mounted by their own modules.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/organizations", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.Get("/{orgId}", h.Get)
	})
}
```

- [ ] **Step 3: Roles handler**

Create `services/api/internal/modules/roles/handler.go`:
```go
package roles

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) orgID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	cat, err := h.svc.ListPermissionCatalog(r.Context())
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, cat)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	roles, err := h.svc.List(r.Context(), orgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, roles)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name is required"))
		return
	}
	role, err := h.svc.Create(r.Context(), orgID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, role)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	roleID, err := uuid.Parse(chi.URLParam(r, "roleId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ROLE_ID", "invalid role id"))
		return
	}
	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed request body"))
		return
	}
	role, err := h.svc.Update(r.Context(), orgID, roleID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, role)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	roleID, err := uuid.Parse(chi.URLParam(r, "roleId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ROLE_ID", "invalid role id"))
		return
	}
	if err := h.svc.Delete(r.Context(), orgID, roleID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Roles routes**

Create `services/api/internal/modules/roles/routes.go`:
```go
package roles

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts role + permission endpoints under an existing
// /organizations/{orgId} router. The parent is already behind authn.
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "role.manage")).
		Get("/permissions", h.ListPermissions)
	r.Route("/roles", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "role.manage"))
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Put("/{roleId}", h.Update)
		r.Delete("/{roleId}", h.Delete)
	})
}

var _ = http.MethodGet
```
Note: drop the trailing `var _ = http.MethodGet` and the `net/http` import if the build flags them unused.

- [ ] **Step 5: Members handler**

Create `services/api/internal/modules/members/handler.go`:
```go
package members

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) orgID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.List(r.Context(), orgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "email is required"))
		return
	}
	m, err := h.svc.Add(r.Context(), orgID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, m)
}

func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	memberID, err := uuid.Parse(chi.URLParam(r, "memberId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_MEMBER_ID", "invalid member id"))
		return
	}
	if err := h.svc.Remove(r.Context(), orgID, memberID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateRoles(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	memberID, err := uuid.Parse(chi.URLParam(r, "memberId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_MEMBER_ID", "invalid member id"))
		return
	}
	var req UpdateRolesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed request body"))
		return
	}
	m, err := h.svc.UpdateRoles(r.Context(), orgID, memberID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, m)
}
```

- [ ] **Step 6: Members routes**

Create `services/api/internal/modules/members/routes.go`:
```go
package members

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts member endpoints under an existing
// /organizations/{orgId} router (already behind authn).
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/members", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "member.manage"))
		r.Get("/", h.List)
		r.Post("/", h.Add)
		r.Delete("/{memberId}", h.Remove)
		r.Put("/{memberId}/roles", h.UpdateRoles)
	})
}
```

- [ ] **Step 7: Rewrite server assembly**

Replace `services/api/internal/app/server.go` entirely:
```go
package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
	authmod "github.com/varin/ivyticketing/services/api/internal/modules/auth"
	membersmod "github.com/varin/ivyticketing/services/api/internal/modules/members"
	orgsmod "github.com/varin/ivyticketing/services/api/internal/modules/organizations"
	rolesmod "github.com/varin/ivyticketing/services/api/internal/modules/roles"
	"github.com/varin/ivyticketing/services/api/internal/modules/system"
	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
	appmw "github.com/varin/ivyticketing/services/api/internal/platform/middleware"
	"github.com/varin/ivyticketing/services/api/internal/platform/rbac"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
)

func NewRouter(cfg Config, log *slog.Logger, pool *pgxpool.Pool, pg, rdb system.Checker) http.Handler {
	r := chi.NewRouter()
	r.Use(appmw.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.WebOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		AllowCredentials: true,
	}))

	// System (Phase 1).
	system.NewHandler(pg, rdb).RegisterRoutes(r)

	// Shared deps.
	queries := db.New(pool)
	signer := security.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	loader := rbac.NewLoader(queries)
	secureCookie := cfg.AppEnv != "local"

	authHandler := authmod.NewHandler(
		authmod.NewService(authmod.NewRepository(queries), signer, cfg.AccessTokenTTL, cfg.RefreshTokenTTL),
		secureCookie,
	)
	orgHandler := orgsmod.NewHandler(orgsmod.NewService(orgsmod.NewRepository(pool)))
	memberHandler := membersmod.NewHandler(membersmod.NewService(membersmod.NewRepository(pool)))
	roleHandler := rolesmod.NewHandler(rolesmod.NewService(rolesmod.NewRepository(pool)))

	r.Route("/api/v1", func(r chi.Router) {
		// Auth (mixed public/protected; mounts its own /me behind authn).
		authHandler.RegisterRoutes(r, signer)

		// Everything else requires authentication.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authn(signer))

			orgHandler.RegisterRoutes(r)

			// Per-org sub-resources, authz enforced per route.
			r.Route("/organizations/{orgId}", func(r chi.Router) {
				memberHandler.RegisterRoutes(r, loader)
				roleHandler.RegisterRoutes(r, loader)
			})
		})
	})

	log.Info("router assembled", "web_origin", cfg.WebOrigin)
	return r
}

func StartServer(ctx context.Context, cfg Config, log *slog.Logger, handler http.Handler) error {
	srv := &http.Server{Addr: ":" + cfg.APIPort, Handler: handler}
	log.Info("api listening", "port", cfg.APIPort)
	return srv.ListenAndServe()
}
```
Note: the two `/organizations` routers coexist — `orgHandler.RegisterRoutes` defines `/organizations`, `/organizations/{orgId}` (GET/POST/list), and the separate `r.Route("/organizations/{orgId}", …)` mounts members/roles under the same prefix. Chi merges these. If chi complains about a route conflict on `/organizations/{orgId}`, move the `Get("/{orgId}")` registration into the `r.Route("/organizations/{orgId}", …)` block as `r.Get("/", h.Get)` instead. Verify in Step 9.

- [ ] **Step 8: Update main to pass the pool**

In `services/api/cmd/api/main.go`, the `database.Connect` returns `*database.Postgres` which holds `Pool`. Update the `NewRouter` call to pass the pool. Replace the router line:
```go
	handler := app.NewRouter(cfg, log, pg.Pool, pg, rdb)
```
(Leave the rest of `main.go` unchanged — `pg` still satisfies `system.Checker` via its `Ping` method.)

- [ ] **Step 9: Build and run full test suite**

Run:
```bash
cd services/api && go build ./... && go vet ./... && go test ./... ; cd ../..
```
Expected: build OK; vet clean; all unit tests PASS. If chi reports a route conflict, apply the fallback in Step 7's note and re-run.

- [ ] **Step 10: Commit**

```bash
git add services/api/internal/modules/organizations/handler.go services/api/internal/modules/organizations/routes.go \
  services/api/internal/modules/members/handler.go services/api/internal/modules/members/routes.go \
  services/api/internal/modules/roles/handler.go services/api/internal/modules/roles/routes.go \
  services/api/internal/app/server.go services/api/cmd/api/main.go
git commit -m "feat(api): add org/member/role handlers and wire all modules into router"
```

---

## Task 14: Enriched `/me` + audit logging on sensitive actions

The spec requires `/me` to return "user + daftar membership/role/permission" (§Endpoint Auth) and sensitive RBAC/member actions to be recorded in `audit_logs` (§Aturan otorisasi, DoD #9). Both were deferred from earlier tasks to keep them self-contained; this task closes them.

**Files:**
- Modify: `services/api/internal/modules/auth/dto.go`
- Modify: `services/api/internal/modules/auth/repository.go`
- Modify: `services/api/internal/modules/auth/service.go`
- Modify: `services/api/internal/modules/auth/handler.go`
- Modify: `services/api/internal/modules/members/service.go`
- Modify: `services/api/internal/modules/members/repository.go` (add audit dependency)
- Modify: `services/api/internal/app/server.go` (inject audit logger)
- Test: extend `services/api/internal/modules/auth/service_test.go`

- [ ] **Step 1: Add membership DTOs to auth**

Append to `services/api/internal/modules/auth/dto.go`:
```go
type MembershipResponse struct {
	OrganizationID uuid.UUID `json:"organizationId"`
	MemberID       uuid.UUID `json:"memberId"`
	RoleSlugs      []string  `json:"roleSlugs"`
	Permissions    []string  `json:"permissions"`
}

type MeResponse struct {
	User        UserResponse         `json:"user"`
	Memberships []MembershipResponse `json:"memberships"`
}
```

- [ ] **Step 2: Extend the auth repository interface**

Add to the `Repository` interface in `services/api/internal/modules/auth/repository.go` (and the `sqlcRepo` implementation):
```go
	ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]db.Organization, error)
	GetMemberByOrgAndUser(ctx context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error)
	ListRolesForMember(ctx context.Context, memberID uuid.UUID) ([]db.Role, error)
	ListPermissionsForMember(ctx context.Context, memberID uuid.UUID) ([]string, error)
```
And the matching adapter methods:
```go
func (r *sqlcRepo) ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]db.Organization, error) {
	return r.q.ListOrganizationsForUser(ctx, userID)
}
func (r *sqlcRepo) GetMemberByOrgAndUser(ctx context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error) {
	return r.q.GetMemberByOrgAndUser(ctx, arg)
}
func (r *sqlcRepo) ListRolesForMember(ctx context.Context, memberID uuid.UUID) ([]db.Role, error) {
	return r.q.ListRolesForMember(ctx, memberID)
}
func (r *sqlcRepo) ListPermissionsForMember(ctx context.Context, memberID uuid.UUID) ([]string, error) {
	return r.q.ListPermissionsForMember(ctx, memberID)
}
```
Update the `fakeRepo` in `service_test.go` to implement these four new methods (return empty slices is fine for existing tests):
```go
func (f *fakeRepo) ListOrganizationsForUser(context.Context, uuid.UUID) ([]db.Organization, error) {
	return nil, nil
}
func (f *fakeRepo) GetMemberByOrgAndUser(context.Context, db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error) {
	return db.OrganizationMember{}, pgx.ErrNoRows
}
func (f *fakeRepo) ListRolesForMember(context.Context, uuid.UUID) ([]db.Role, error) { return nil, nil }
func (f *fakeRepo) ListPermissionsForMember(context.Context, uuid.UUID) ([]string, error) {
	return nil, nil
}
```

- [ ] **Step 3: Add a `Me` method to the auth service**

Add to `services/api/internal/modules/auth/service.go`:
```go
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
```

- [ ] **Step 4: Use it in the handler**

In `services/api/internal/modules/auth/handler.go`, replace the body of `Me` to call the new service method:
```go
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	me, err := h.svc.Me(r.Context(), id.UserID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, me)
}
```

- [ ] **Step 5: Add `/me` enrichment test**

Add to `services/api/internal/modules/auth/service_test.go` a test that seeds one org membership in the fake and asserts `Me` returns it. Extend `fakeRepo` with settable fields:
```go
func TestMe_IncludesMemberships(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	u, err := svc.Register(ctx, RegisterRequest{Email: "a@b.com", Password: "pw123456", FullName: "A"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Override the four membership methods for this test via a wrapper repo.
	orgID := uuid.New()
	memberID := uuid.New()
	repo.meOrgs = []db.Organization{{ID: orgID, Name: "Org", Slug: "org"}}
	repo.meMember = db.OrganizationMember{ID: memberID, OrganizationID: orgID, UserID: u.ID}
	repo.meRoles = []db.Role{{ID: uuid.New(), Slug: "owner"}}
	repo.mePerms = []string{"member.manage", "role.manage"}

	me, err := svc.Me(ctx, u.ID)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if len(me.Memberships) != 1 {
		t.Fatalf("memberships = %d, want 1", len(me.Memberships))
	}
	m := me.Memberships[0]
	if m.OrganizationID != orgID || len(m.RoleSlugs) != 1 || m.RoleSlugs[0] != "owner" {
		t.Errorf("unexpected membership: %+v", m)
	}
	if len(m.Permissions) != 2 {
		t.Errorf("permissions = %v, want 2", m.Permissions)
	}
}
```
And change the four membership methods on `fakeRepo` (from Step 2) to return these new fields instead of empty:
```go
// add fields to fakeRepo:
//   meOrgs   []db.Organization
//   meMember db.OrganizationMember
//   meRoles  []db.Role
//   mePerms  []string
func (f *fakeRepo) ListOrganizationsForUser(context.Context, uuid.UUID) ([]db.Organization, error) {
	return f.meOrgs, nil
}
func (f *fakeRepo) GetMemberByOrgAndUser(context.Context, db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error) {
	return f.meMember, nil
}
func (f *fakeRepo) ListRolesForMember(context.Context, uuid.UUID) ([]db.Role, error) {
	return f.meRoles, nil
}
func (f *fakeRepo) ListPermissionsForMember(context.Context, uuid.UUID) ([]string, error) {
	return f.mePerms, nil
}
```

- [ ] **Step 6: Wire audit logging into the members service**

Add an audit dependency to the members service. Modify `NewService` to accept an audit recorder via a small interface (so tests pass `nil`-safe). In `services/api/internal/modules/members/service.go`:
```go
// AuditRecorder records sensitive actions. Implemented by platform/audit.Logger.
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
```
Add `import "github.com/varin/ivyticketing/services/api/internal/platform/audit"`. After a successful `Add`, `Remove`, and `UpdateRoles`, call (guard for nil so unit tests can pass `nil`):
```go
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			Action:         "member.add", // or "member.remove", "member.roles.update"
			TargetType:     "member",
			TargetID:       member.ID.String(),
		})
	}
```
Update the members `service_test.go` `NewService(repo)` calls to `NewService(repo, nil)`.

- [ ] **Step 7: Inject the audit logger in server assembly**

In `services/api/internal/app/server.go`, build the audit logger and pass it to the members service:
```go
	auditLog := audit.NewLogger(queries, log)
	memberHandler := membersmod.NewHandler(membersmod.NewService(membersmod.NewRepository(pool), auditLog))
```
Add `import "github.com/varin/ivyticketing/services/api/internal/platform/audit"`.

- [ ] **Step 8: Build and test**

Run:
```bash
cd services/api && go build ./... && go test ./... ; cd ../..
```
Expected: build OK; all unit tests PASS (including the new `/me` test).

- [ ] **Step 9: Commit**

```bash
git add services/api/internal/modules/auth services/api/internal/modules/members services/api/internal/app/server.go
git commit -m "feat(api): enrich /me with memberships and audit sensitive member actions"
```

---

## Task 15: Integration tests (real Postgres)

These run against the `ivyticketing_test` database, migrated fresh and truncated per test. They prove the full flow and tenant isolation (spec §Testing, DoD #3-#10). Gated behind a build tag so `go test ./...` stays fast unless integration is requested.

**Files:**
- Create: `services/api/tests/integration/helpers_test.go`
- Create: `services/api/tests/integration/auth_flow_test.go`
- Create: `services/api/tests/integration/tenant_isolation_test.go`
- Create: `services/api/tests/integration/seed_test.go`
- Modify: `Makefile` (add `test-integration` target + test DB setup)

- [ ] **Step 1: Create the test database and migrate it**

Run (one-time / idempotent):
```bash
PG="$(brew --prefix)/opt/postgresql@16/bin"
"$PG/createdb" ivyticketing_test 2>/dev/null || echo "test db exists"
"$(go env GOPATH)/bin/goose" -dir database/migrations postgres \
  "postgres://localhost:5432/ivyticketing_test?sslmode=disable" up
```
Expected: test DB exists and is fully migrated (incl. the RBAC seed).

- [ ] **Step 2: Add Makefile target**

Add to `Makefile`:
```make
TEST_DATABASE_URL ?= postgres://localhost:5432/ivyticketing_test?sslmode=disable

test-db-setup:
	$(GOPATH_BIN)/goose -dir database/migrations postgres "$(TEST_DATABASE_URL)" up

test-integration:
	cd services/api && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration ./tests/integration/... -v
```

- [ ] **Step 3: Test helpers**

Create `services/api/tests/integration/helpers_test.go`:
```go
//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/app"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// truncate clears all tenant data but keeps the seeded catalog/templates.
func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`TRUNCATE refresh_tokens, member_roles, organization_members,
		 audit_logs, role_permissions, organizations, users RESTART IDENTITY CASCADE;
		 DELETE FROM roles WHERE organization_id IS NOT NULL;`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// newTestServer builds the real router against the test pool.
type stubChecker struct{}

func (stubChecker) Ping(context.Context) error { return nil }

func newTestServer(t *testing.T, pool *pgxpool.Pool) *httptest.Server {
	t.Helper()
	cfg := app.Config{
		AppEnv:          "test",
		APIPort:         "0",
		WebOrigin:       "http://localhost:4321",
		JWTSecret:       "integration-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 168 * time.Hour,
	}
	logger := newNopLogger()
	h := app.NewRouter(cfg, logger, pool, stubChecker{}, stubChecker{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// helper to extract refresh cookie from a response
func refreshCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}
	return nil
}
```
Create a tiny logger helper in the same package (or inline) — add to `helpers_test.go`:
```go
import "log/slog"

func newNopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}
```
(Consolidate the imports — `log/slog`, `os`, etc. — into the single import block when writing the file.)

- [ ] **Step 4: Full auth + RBAC flow test**

Create `services/api/tests/integration/auth_flow_test.go`:
```go
//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func postJSON(t *testing.T, client *http.Client, url string, body any, bearer string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestFullFlow_RegisterLoginCreateOrgAddMember(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	// Register the owner.
	resp := postJSON(t, client, srv.URL+"/api/v1/auth/register",
		map[string]string{"email": "owner@x.com", "password": "pw123456", "fullName": "Owner"}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Register a second user (future staff).
	resp = postJSON(t, client, srv.URL+"/api/v1/auth/register",
		map[string]string{"email": "staff@x.com", "password": "pw123456", "fullName": "Staff"}, "")
	resp.Body.Close()

	// Login owner.
	resp = postJSON(t, client, srv.URL+"/api/v1/auth/login",
		map[string]string{"email": "owner@x.com", "password": "pw123456"}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", resp.StatusCode)
	}
	var login struct {
		AccessToken string `json:"accessToken"`
	}
	json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()
	if login.AccessToken == "" {
		t.Fatal("expected access token")
	}

	// Create org.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations",
		map[string]string{"name": "Jakarta Marathon"}, login.AccessToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create org status = %d, want 201", resp.StatusCode)
	}
	var org struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&org)
	resp.Body.Close()

	// Owner can list members (has member.manage via Owner role).
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+org.ID+"/members", nil)
	req.Header.Set("Authorization", "Bearer "+login.AccessToken)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list members status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Add staff member without roles.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+org.ID+"/members",
		map[string]any{"email": "staff@x.com", "roleIds": []string{}}, login.AccessToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add member status = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()
}
```

- [ ] **Step 5: Tenant isolation test**

Create `services/api/tests/integration/tenant_isolation_test.go`:
```go
//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func loginAndCreateOrg(t *testing.T, client *http.Client, baseURL, email, orgName string) (token, orgID string) {
	t.Helper()
	postJSON(t, client, baseURL+"/api/v1/auth/register",
		map[string]string{"email": email, "password": "pw123456", "fullName": email}, "").Body.Close()
	resp := postJSON(t, client, baseURL+"/api/v1/auth/login",
		map[string]string{"email": email, "password": "pw123456"}, "")
	var login struct {
		AccessToken string `json:"accessToken"`
	}
	json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()

	resp = postJSON(t, client, baseURL+"/api/v1/organizations",
		map[string]string{"name": orgName}, login.AccessToken)
	var org struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&org)
	resp.Body.Close()
	return login.AccessToken, org.ID
}

func TestTenantIsolation_MemberOfACannotAccessB(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	_, orgA := loginAndCreateOrg(t, client, srv.URL, "owner-a@x.com", "Org A")
	tokenB, _ := loginAndCreateOrg(t, client, srv.URL, "owner-b@x.com", "Org B")

	// Owner B tries to list members of Org A → 403.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgA+"/members", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-tenant access status = %d, want 403", resp.StatusCode)
	}
}
```

- [ ] **Step 6: Seed presence test**

Create `services/api/tests/integration/seed_test.go`:
```go
//go:build integration

package integration

import (
	"context"
	"testing"
)

func TestSeed_CatalogAndTemplatesPresent(t *testing.T) {
	pool := testPool(t)

	var permCount int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM permissions").Scan(&permCount); err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if permCount != 21 {
		t.Errorf("permissions = %d, want 21", permCount)
	}

	var tmplCount int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM roles WHERE organization_id IS NULL").Scan(&tmplCount); err != nil {
		t.Fatalf("count templates: %v", err)
	}
	if tmplCount != 5 {
		t.Errorf("template roles = %d, want 5", tmplCount)
	}
}
```

- [ ] **Step 7: Run integration tests**

Run:
```bash
make test-db-setup
make test-integration
```
Expected: all integration tests PASS — full flow (201/200/201/200/201), tenant isolation (403), seed presence (21 perms, 5 templates).

- [ ] **Step 8: Commit**

```bash
git add services/api/tests/integration Makefile
git commit -m "test(api): add integration tests for auth flow, tenant isolation, and seed"
```

---

## Task 16: README update + full Definition-of-Done verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Document Phase 2 in the README**

Add a "Phase 2 — Auth, RBAC & Multi-Tenant" section to `README.md` covering: new env vars (`JWT_SECRET`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`), the auth endpoints, and how to run integration tests (`make test-db-setup && make test-integration`). Add a curl smoke-test block:
```markdown
## Phase 2 — Auth, RBAC & Multi-Tenant

Backend auth (hybrid token), multi-tenant orgs, and custom-role RBAC.

### New env

```bash
JWT_SECRET=change-me        # REQUIRED — API won't start without it
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
```

### Smoke test

```bash
# register + login
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'content-type: application/json' \
  -d '{"email":"a@b.com","password":"pw123456","fullName":"A"}'

curl -s -c cookies.txt -X POST localhost:8080/api/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"email":"a@b.com","password":"pw123456"}'
# → { "accessToken": "...", "expiresIn": 900, "user": {...} }

# create org (use the accessToken from login)
curl -s -X POST localhost:8080/api/v1/organizations \
  -H "authorization: Bearer <accessToken>" \
  -H 'content-type: application/json' \
  -d '{"name":"Jakarta Marathon"}'
```

### Integration tests

```bash
make test-db-setup       # create + migrate ivyticketing_test
make test-integration    # run -tags=integration suite
```
```

- [ ] **Step 2: Verify the full Definition of Done**

Run the complete gate:
```bash
# DoD #1 migrations roundtrip
make migrate-down && make migrate-up
# DoD #11/#12 unit tests + sqlc clean
make sqlc && cd services/api && go build ./... && go test ./... && cd ../..
# DoD #3-#10 integration
make test-db-setup && make test-integration
# DoD #11 no hardcoded secrets
grep -rn "JWT_SECRET" services/api/internal | grep -v "os.Getenv\|cfg.JWTSecret\|JWTSecret" || echo "no hardcoded secret"
```
Expected: every command succeeds. This maps to the spec's Definition of Done:
- #1 migrations up/down → `make migrate-down/up`
- #2 seed present → `seed_test.go`
- #3 register/login/me → `auth_flow_test.go` + enriched `/me` (Task 14)
- #4 refresh rotation/logout revoke → `auth/service_test.go` (Task 6)
- #5 org create copies roles + Owner → `organizations/service_test.go` (Task 8) + integration
- #6 RBAC 403/allow → `authz_test.go` (Task 9) + tenant isolation
- #7 tenant isolation → `tenant_isolation_test.go`
- #8 custom role create/assign → `roles/service_test.go` + members assign
- #9 audit logging → Task 14 wiring
- #10 super admin cross-org → `authz` bypass (Task 9)
- #11 `go test ./...` green, no hardcoded secrets → final gate
- #12 `sqlc generate` clean → `make sqlc`

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document phase 2 auth/rbac setup and smoke tests"
```

---

