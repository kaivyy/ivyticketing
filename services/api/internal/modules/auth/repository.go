package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, hash string) (db.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
}

// sqlcRepo adapts *db.Queries to Repository.
type sqlcRepo struct{ q *db.Queries }

func NewRepository(q *db.Queries) Repository { return &sqlcRepo{q: q} }

func (r *sqlcRepo) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	return r.q.CreateUser(ctx, arg)
}
func (r *sqlcRepo) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	return r.q.GetUserByEmail(ctx, email)
}
func (r *sqlcRepo) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return r.q.GetUserByID(ctx, id)
}
func (r *sqlcRepo) CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error) {
	return r.q.CreateRefreshToken(ctx, arg)
}
func (r *sqlcRepo) GetRefreshTokenByHash(ctx context.Context, hash string) (db.RefreshToken, error) {
	return r.q.GetRefreshTokenByHash(ctx, hash)
}
func (r *sqlcRepo) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	return r.q.RevokeRefreshToken(ctx, id)
}
