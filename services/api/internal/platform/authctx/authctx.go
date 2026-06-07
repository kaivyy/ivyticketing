package authctx

import (
	"context"

	"github.com/google/uuid"
)

type Identity struct {
	UserID          uuid.UUID
	IsPlatformAdmin bool
}

type ctxKey struct{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}
