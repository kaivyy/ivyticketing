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
		key(orgID, userID): {"role.manage": true},
	}}
	mw := RequirePermission(loader, "member.manage")
	if code := serve(t, mw, orgID, authctx.Identity{UserID: userID}, true); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
}

func TestAuthz_DeniesNonMember(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()
	loader := &fakeLoader{perms: map[string]map[string]bool{}}
	mw := RequirePermission(loader, "member.manage")
	if code := serve(t, mw, orgID, authctx.Identity{UserID: userID}, true); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
}

func TestAuthz_PlatformAdminBypasses(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()
	loader := &fakeLoader{perms: map[string]map[string]bool{}}
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
