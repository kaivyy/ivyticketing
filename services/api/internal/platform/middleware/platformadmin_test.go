package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
)

func TestRequirePlatformAdmin_Allows(t *testing.T) {
	called := false
	h := RequirePlatformAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	ctx := authctx.WithIdentity(req.Context(), authctx.Identity{UserID: uuid.New(), IsPlatformAdmin: true})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("admin should pass: called=%v code=%d", called, rec.Code)
	}
}

func TestRequirePlatformAdmin_DeniesNonAdmin(t *testing.T) {
	h := RequirePlatformAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	}))
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	ctx := authctx.WithIdentity(req.Context(), authctx.Identity{UserID: uuid.New(), IsPlatformAdmin: false})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin should get 403, got %d", rec.Code)
	}
}

func TestRequirePlatformAdmin_DeniesUnauthenticated(t *testing.T) {
	h := RequirePlatformAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	}))
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated should get 401, got %d", rec.Code)
	}
}
