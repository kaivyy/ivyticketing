package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
)

func TestAuthn_RejectsMissingHeader(t *testing.T) {
	signer := security.NewJWTSigner("test-secret", time.Minute)
	h := Authn(signer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not be called")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthn_AcceptsValidTokenAndSetsIdentity(t *testing.T) {
	signer := security.NewJWTSigner("test-secret", time.Minute)
	uid := uuid.New()
	tok, _ := signer.Sign(uid, false)

	var gotID uuid.UUID
	h := Authn(signer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := authctx.FromContext(r.Context())
		if !ok {
			t.Fatal("expected identity in context")
		}
		gotID = id.UserID
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotID != uid {
		t.Errorf("UserID = %v, want %v", gotID, uid)
	}
}
