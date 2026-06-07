package authctx

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestContextRoundTrip(t *testing.T) {
	uid := uuid.New()
	ctx := WithIdentity(context.Background(), Identity{UserID: uid, IsPlatformAdmin: true})

	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected identity in context")
	}
	if got.UserID != uid || !got.IsPlatformAdmin {
		t.Errorf("unexpected identity: %+v", got)
	}
}

func TestFromContext_Missing(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("expected no identity in empty context")
	}
}
