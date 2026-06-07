package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
)

type fakeRepo struct {
	users    map[string]db.User // by email
	usersIID map[uuid.UUID]db.User
	tokens   map[string]db.RefreshToken // by hash
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:    map[string]db.User{},
		usersIID: map[uuid.UUID]db.User{},
		tokens:   map[string]db.RefreshToken{},
	}
}

func (f *fakeRepo) CreateUser(_ context.Context, arg db.CreateUserParams) (db.User, error) {
	u := db.User{ID: uuid.New(), Email: arg.Email, PasswordHash: arg.PasswordHash, FullName: arg.FullName, Phone: arg.Phone}
	f.users[arg.Email] = u
	f.usersIID[u.ID] = u
	return u, nil
}
func (f *fakeRepo) GetUserByEmail(_ context.Context, email string) (db.User, error) {
	u, ok := f.users[email]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}
func (f *fakeRepo) GetUserByID(_ context.Context, id uuid.UUID) (db.User, error) {
	u, ok := f.usersIID[id]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}
func (f *fakeRepo) CreateRefreshToken(_ context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error) {
	t := db.RefreshToken{ID: uuid.New(), UserID: arg.UserID, TokenHash: arg.TokenHash, ExpiresAt: arg.ExpiresAt}
	f.tokens[arg.TokenHash] = t
	return t, nil
}
func (f *fakeRepo) GetRefreshTokenByHash(_ context.Context, hash string) (db.RefreshToken, error) {
	t, ok := f.tokens[hash]
	if !ok {
		return db.RefreshToken{}, pgx.ErrNoRows
	}
	return t, nil
}
func (f *fakeRepo) RevokeRefreshToken(_ context.Context, id uuid.UUID) error {
	for h, t := range f.tokens {
		if t.ID == id {
			now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
			t.RevokedAt = now
			f.tokens[h] = t
		}
	}
	return nil
}

func newTestService(repo Repository) *Service {
	return NewService(repo, security.NewJWTSigner("test-secret", time.Minute), 15*time.Minute, time.Hour)
}

func TestRegister_RejectsDuplicateEmail(t *testing.T) {
	svc := newTestService(newFakeRepo())
	ctx := context.Background()
	req := RegisterRequest{Email: "a@b.com", Password: "pw123456", FullName: "A"}

	if _, err := svc.Register(ctx, req); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := svc.Register(ctx, req); err != ErrEmailExists {
		t.Fatalf("second register err = %v, want ErrEmailExists", err)
	}
}

func TestLogin_RejectsBadCredentials(t *testing.T) {
	svc := newTestService(newFakeRepo())
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegisterRequest{Email: "a@b.com", Password: "pw123456", FullName: "A"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := svc.Login(ctx, LoginRequest{Email: "a@b.com", Password: "wrong"}); err != ErrInvalidCredential {
		t.Fatalf("login err = %v, want ErrInvalidCredential", err)
	}
}

func TestRefresh_RotatesAndRevokesOld(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegisterRequest{Email: "a@b.com", Password: "pw123456", FullName: "A"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := svc.Login(ctx, LoginRequest{Email: "a@b.com", Password: "pw123456"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	refreshed, err := svc.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Error("refresh should rotate the token")
	}
	// Old token now revoked -> reusing it fails.
	if _, err := svc.Refresh(ctx, login.RefreshToken); err != ErrTokenRevoked {
		t.Fatalf("reuse old token err = %v, want ErrTokenRevoked", err)
	}
}

func TestRefresh_RejectsExpired(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegisterRequest{Email: "a@b.com", Password: "pw123456", FullName: "A"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := svc.Login(ctx, LoginRequest{Email: "a@b.com", Password: "pw123456"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	// Force the stored token to be expired.
	hash := security.HashToken(login.RefreshToken)
	tok := repo.tokens[hash]
	tok.ExpiresAt = pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true}
	repo.tokens[hash] = tok

	if _, err := svc.Refresh(ctx, login.RefreshToken); err != ErrTokenExpired {
		t.Fatalf("refresh err = %v, want ErrTokenExpired", err)
	}
}
