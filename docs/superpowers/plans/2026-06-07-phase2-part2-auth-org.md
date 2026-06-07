# Phase 2 Plan — Part 2: Auth & Organizations (Tasks 6-8)

> Part of the Phase 2 implementation plan. Index: [2026-06-07-phase2-auth-rbac-multitenant.md](2026-06-07-phase2-auth-rbac-multitenant.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

## Task 6: Auth module — service + DTOs + typed errors

This task builds the auth service against a repository interface so it is unit-testable with a fake (no DB). Handler/routes come in Task 7.

**Files:**
- Create: `services/api/internal/modules/auth/errors.go`
- Create: `services/api/internal/modules/auth/dto.go`
- Create: `services/api/internal/modules/auth/repository.go`
- Create: `services/api/internal/modules/auth/service.go`
- Test: `services/api/internal/modules/auth/service_test.go`

- [ ] **Step 1: Typed errors**

Create `services/api/internal/modules/auth/errors.go`:
```go
package auth

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrEmailExists       = apperr.New(http.StatusConflict, "EMAIL_EXISTS", "email already registered")
	ErrInvalidCredential = apperr.New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
	ErrTokenExpired      = apperr.New(http.StatusUnauthorized, "TOKEN_EXPIRED", "refresh token expired")
	ErrTokenRevoked      = apperr.New(http.StatusUnauthorized, "TOKEN_REVOKED", "refresh token revoked")
	ErrTokenInvalid      = apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "invalid refresh token")
)
```

- [ ] **Step 2: DTOs**

Create `services/api/internal/modules/auth/dto.go`:
```go
package auth

import "github.com/google/uuid"

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
}

type UserResponse struct {
	ID       uuid.UUID `json:"id"`
	Email    string    `json:"email"`
	FullName string    `json:"fullName"`
	Phone    string    `json:"phone"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResult carries the access token for the body plus the raw refresh token
// for the handler to set as an HttpOnly cookie (never serialized to JSON).
type LoginResult struct {
	AccessToken  string       `json:"accessToken"`
	ExpiresIn    int          `json:"expiresIn"`
	User         UserResponse `json:"user"`
	RefreshToken string       `json:"-"`
	RefreshTTL   int          `json:"-"`
}

type RefreshResult struct {
	AccessToken  string `json:"accessToken"`
	ExpiresIn    int    `json:"expiresIn"`
	RefreshToken string `json:"-"`
	RefreshTTL   int    `json:"-"`
}
```

- [ ] **Step 3: Repository interface + adapter**

Create `services/api/internal/modules/auth/repository.go`:
```go
package auth

import (
	"context"
	"time"

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

var _ = time.Now // time imported for callers building params; keep import used if unreferenced
```
Note: if the `time` import ends up unused after `go build`, delete the import and the trailing `var _` line — it's only a guard for the params struct fields. Prefer removing it; the build in Step 6 will tell you.

- [ ] **Step 4: Write the failing service test**

Create `services/api/internal/modules/auth/service_test.go`:
```go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"

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
			now := time.Now()
			t.RevokedAt = &now
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
	// Old token now revoked → reusing it fails.
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
	tok.ExpiresAt = time.Now().Add(-time.Hour)
	repo.tokens[hash] = tok

	if _, err := svc.Refresh(ctx, login.RefreshToken); err != ErrTokenExpired {
		t.Fatalf("refresh err = %v, want ErrTokenExpired", err)
	}
}
```

- [ ] **Step 5: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/auth/ -v; cd ../..
```
Expected: FAIL — `undefined: NewService`, `undefined: Service`.

- [ ] **Step 6: Implement the service**

Create `services/api/internal/modules/auth/service.go`:
```go
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
)

type Service struct {
	repo       Repository
	signer     *security.JWTSigner
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewService(repo Repository, signer *security.JWTSigner, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, signer: signer, accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (UserResponse, error) {
	if _, err := s.repo.GetUserByEmail(ctx, req.Email); err == nil {
		return UserResponse{}, ErrEmailExists
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return UserResponse{}, err
	}

	hash, err := security.HashPassword(req.Password)
	if err != nil {
		return UserResponse{}, err
	}
	phash := hash
	u, err := s.repo.CreateUser(ctx, db.CreateUserParams{
		Email:        req.Email,
		PasswordHash: &phash,
		FullName:     req.FullName,
		Phone:        nullableStr(req.Phone),
	})
	if err != nil {
		return UserResponse{}, err
	}
	return toUserResponse(u), nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResult, error) {
	u, err := s.repo.GetUserByEmail(ctx, req.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginResult{}, ErrInvalidCredential
	} else if err != nil {
		return LoginResult{}, err
	}
	if u.PasswordHash == nil || !security.VerifyPassword(*u.PasswordHash, req.Password) {
		return LoginResult{}, ErrInvalidCredential
	}

	access, raw, err := s.issueTokens(ctx, u)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		AccessToken:  access,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		User:         toUserResponse(u),
		RefreshToken: raw,
		RefreshTTL:   int(s.refreshTTL.Seconds()),
	}, nil
}

func (s *Service) Refresh(ctx context.Context, rawToken string) (RefreshResult, error) {
	if rawToken == "" {
		return RefreshResult{}, ErrTokenInvalid
	}
	stored, err := s.repo.GetRefreshTokenByHash(ctx, security.HashToken(rawToken))
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshResult{}, ErrTokenInvalid
	} else if err != nil {
		return RefreshResult{}, err
	}
	if stored.RevokedAt != nil {
		return RefreshResult{}, ErrTokenRevoked
	}
	if time.Now().After(stored.ExpiresAt) {
		return RefreshResult{}, ErrTokenExpired
	}

	// Rotate: revoke old, issue new.
	if err := s.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return RefreshResult{}, err
	}
	u, err := s.repo.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return RefreshResult{}, err
	}
	access, raw, err := s.issueTokens(ctx, u)
	if err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{
		AccessToken:  access,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		RefreshToken: raw,
		RefreshTTL:   int(s.refreshTTL.Seconds()),
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	stored, err := s.repo.GetRefreshTokenByHash(ctx, security.HashToken(rawToken))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	return s.repo.RevokeRefreshToken(ctx, stored.ID)
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (db.User, error) {
	return s.repo.GetUserByID(ctx, id)
}

func (s *Service) issueTokens(ctx context.Context, u db.User) (access, raw string, err error) {
	access, err = s.signer.Sign(u.ID, u.IsPlatformAdmin)
	if err != nil {
		return "", "", err
	}
	raw, hash, err := security.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}
	if _, err = s.repo.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}); err != nil {
		return "", "", err
	}
	return access, raw, nil
}

func toUserResponse(u db.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, FullName: u.FullName, Phone: derefStr(u.Phone)}
}

func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```
Note: the exact generated field types (`PasswordHash *string`, `Phone *string`, `RevokedAt *time.Time`) come from the sqlc overrides in Task 1. If `go build` reports a type mismatch (e.g. `pgtype.Text` instead of `*string`), inspect `internal/db/models.go` and adjust the helpers — but with the configured overrides these should be pointers.

- [ ] **Step 7: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/auth/ -v; cd ../..
```
Expected: PASS (all four service tests). Remove the unused `time` guard in `repository.go` if the build flagged it.

- [ ] **Step 8: Commit**

```bash
git add services/api/internal/modules/auth
git commit -m "feat(auth): add auth service with register/login/refresh/logout"
```

---

## Task 7: Auth handler, routes, cookies + authn middleware

**Files:**
- Create: `services/api/internal/modules/auth/handler.go`
- Create: `services/api/internal/modules/auth/routes.go`
- Create: `services/api/internal/platform/middleware/authn.go`
- Test: `services/api/internal/platform/middleware/authn_test.go`

`/me` requires authentication, so it sits behind the authn middleware. Building authn here lets the handler and its route be wired together.

- [ ] **Step 1: Write failing test for authn middleware**

Create `services/api/internal/platform/middleware/authn_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/middleware/ -run TestAuthn -v; cd ../..
```
Expected: FAIL — `undefined: Authn`.

- [ ] **Step 3: Implement authn middleware**

Create `services/api/internal/platform/middleware/authn.go`:
```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
)

func Authn(signer *security.JWTSigner) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "missing or malformed Authorization header"))
				return
			}
			claims, err := signer.Verify(parts[1])
			if err != nil {
				apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "invalid access token"))
				return
			}
			ctx := authctx.WithIdentity(r.Context(), authctx.Identity{
				UserID:          claims.UserID,
				IsPlatformAdmin: claims.IsPlatformAdmin,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/platform/middleware/ -run TestAuthn -v; cd ../..
```
Expected: PASS (both tests).

- [ ] **Step 5: Implement the auth handler**

Create `services/api/internal/modules/auth/handler.go`:
```go
package auth

import (
	"encoding/json"
	"net/http"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

const refreshCookieName = "refresh_token"
const refreshCookiePath = "/api/v1/auth"

type Handler struct {
	svc    *Service
	secure bool // Secure cookie flag (true in non-local env)
}

func NewHandler(svc *Service, secure bool) *Handler {
	return &Handler{svc: svc, secure: secure}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed request body"))
		return
	}
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "email, password, and fullName are required"))
		return
	}
	user, err := h.svc.Register(r.Context(), req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed request body"))
		return
	}
	res, err := h.svc.Login(r.Context(), req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	h.setRefreshCookie(w, res.RefreshToken, res.RefreshTTL)
	apperr.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	raw := readRefreshCookie(r)
	res, err := h.svc.Refresh(r.Context(), raw)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	h.setRefreshCookie(w, res.RefreshToken, res.RefreshTTL)
	apperr.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	raw := readRefreshCookie(r)
	if err := h.svc.Logout(r.Context(), raw); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	u, err := h.svc.GetUser(r.Context(), id.UserID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toUserResponse(u))
}

func (h *Handler) setRefreshCookie(w http.ResponseWriter, raw string, ttlSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    raw,
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   ttlSeconds,
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func readRefreshCookie(r *http.Request) string {
	c, err := r.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
```
Note: `/me` returns just the user here. The spec's "user + membership/role/permission" payload is enriched in Task 10 once the members repository exists; this keeps Task 7 self-contained and compiling.

- [ ] **Step 6: Implement routes**

Create `services/api/internal/modules/auth/routes.go`:
```go
package auth

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
)

// RegisterRoutes mounts auth endpoints under /api/v1/auth.
func (h *Handler) RegisterRoutes(r chi.Router, signer *security.JWTSigner) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
		r.With(middleware.Authn(signer)).Get("/me", h.Me)
	})
}
```

- [ ] **Step 7: Verify build and tests**

Run:
```bash
cd services/api && go build ./... && go test ./internal/modules/auth/ ./internal/platform/middleware/ -v; cd ../..
```
Expected: build OK; tests PASS.

- [ ] **Step 8: Commit**

```bash
git add services/api/internal/modules/auth/handler.go services/api/internal/modules/auth/routes.go \
  services/api/internal/platform/middleware/authn.go services/api/internal/platform/middleware/authn_test.go
git commit -m "feat(auth): add auth handler, routes, refresh cookie, and authn middleware"
```

---

## Task 8: Organizations module — create (copy templates + assign Owner), list, get

**Design decision (locked here):** When an org is created, every system role *template* (`organization_id IS NULL`) is copied into an org-owned row with `is_system = false`, so the org has full control to edit/delete its roles (spec: "Org bebas tambah/ubah/hapus role-nya tanpa mempengaruhi template/org lain"). Deletion safety comes from two guards enforced later (Task 11): reject deleting a role still in use, and reject removing the last Owner. The `is_system = false` filter on `DeleteRole` still defensively prevents deleting any global template via an org-scoped query.

The whole create flow (org + member + role copies + permission copies + owner assignment) runs in one transaction. The repository exposes `ExecTx` so the service orchestrates inside a tx; the fake repo runs the callback inline.

**Files:**
- Create: `services/api/internal/modules/organizations/errors.go`
- Create: `services/api/internal/modules/organizations/dto.go`
- Create: `services/api/internal/modules/organizations/slug.go`
- Test: `services/api/internal/modules/organizations/slug_test.go`
- Create: `services/api/internal/modules/organizations/repository.go`
- Create: `services/api/internal/modules/organizations/service.go`
- Test: `services/api/internal/modules/organizations/service_test.go`

- [ ] **Step 1: Typed errors**

Create `services/api/internal/modules/organizations/errors.go`:
```go
package organizations

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrSlugTaken = apperr.New(http.StatusConflict, "SLUG_TAKEN", "organization slug already in use")
	ErrNotFound  = apperr.New(http.StatusNotFound, "ORG_NOT_FOUND", "organization not found")
	ErrForbidden = apperr.New(http.StatusForbidden, "FORBIDDEN", "not a member of this organization")
)
```

- [ ] **Step 2: DTOs**

Create `services/api/internal/modules/organizations/dto.go`:
```go
package organizations

import (
	"time"

	"github.com/google/uuid"
)

type CreateRequest struct {
	Name string `json:"name"`
}

type Response struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"createdAt"`
}
```

- [ ] **Step 3: Write failing test for slug**

Create `services/api/internal/modules/organizations/slug_test.go`:
```go
package organizations

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Jakarta Marathon":   "jakarta-marathon",
		"  Trail   Run 2026 ": "trail-run-2026",
		"Bali!!! Run":        "bali-run",
		"ALL CAPS":           "all-caps",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/organizations/ -run TestSlugify -v; cd ../..
```
Expected: FAIL — `undefined: slugify`.

- [ ] **Step 5: Implement slug**

Create `services/api/internal/modules/organizations/slug.go`:
```go
package organizations

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
```

- [ ] **Step 6: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/organizations/ -run TestSlugify -v; cd ../..
```
Expected: PASS.

- [ ] **Step 7: Repository interface + sqlc adapter (with ExecTx)**

Create `services/api/internal/modules/organizations/repository.go`:
```go
package organizations

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
	CreateOrganization(ctx context.Context, arg db.CreateOrganizationParams) (db.Organization, error)
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (db.Organization, error)
	ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]db.Organization, error)
	GetMemberByOrgAndUser(ctx context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error)
	CreateMember(ctx context.Context, arg db.CreateMemberParams) (db.OrganizationMember, error)
	ListTemplateRoles(ctx context.Context) ([]db.Role, error)
	ListPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]db.Permission, error)
	CreateRole(ctx context.Context, arg db.CreateRoleParams) (db.Role, error)
	AddRolePermission(ctx context.Context, arg db.AddRolePermissionParams) error
	AddMemberRole(ctx context.Context, arg db.AddMemberRoleParams) error
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
	txRepo := &sqlcRepo{pool: r.pool, q: db.New(tx)}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *sqlcRepo) CreateOrganization(ctx context.Context, arg db.CreateOrganizationParams) (db.Organization, error) {
	return r.q.CreateOrganization(ctx, arg)
}
func (r *sqlcRepo) GetOrganizationByID(ctx context.Context, id uuid.UUID) (db.Organization, error) {
	return r.q.GetOrganizationByID(ctx, id)
}
func (r *sqlcRepo) ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]db.Organization, error) {
	return r.q.ListOrganizationsForUser(ctx, userID)
}
func (r *sqlcRepo) GetMemberByOrgAndUser(ctx context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error) {
	return r.q.GetMemberByOrgAndUser(ctx, arg)
}
func (r *sqlcRepo) CreateMember(ctx context.Context, arg db.CreateMemberParams) (db.OrganizationMember, error) {
	return r.q.CreateMember(ctx, arg)
}
func (r *sqlcRepo) ListTemplateRoles(ctx context.Context) ([]db.Role, error) {
	return r.q.ListTemplateRoles(ctx)
}
func (r *sqlcRepo) ListPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]db.Permission, error) {
	return r.q.ListPermissionsForRole(ctx, roleID)
}
func (r *sqlcRepo) CreateRole(ctx context.Context, arg db.CreateRoleParams) (db.Role, error) {
	return r.q.CreateRole(ctx, arg)
}
func (r *sqlcRepo) AddRolePermission(ctx context.Context, arg db.AddRolePermissionParams) error {
	return r.q.AddRolePermission(ctx, arg)
}
func (r *sqlcRepo) AddMemberRole(ctx context.Context, arg db.AddMemberRoleParams) error {
	return r.q.AddMemberRole(ctx, arg)
}

var _ = pgx.ErrNoRows
```
Note: remove the trailing `var _ = pgx.ErrNoRows` and the `pgx` import if `go build` flags them as unused — they're only present if a later edit references `pgx` here.

- [ ] **Step 8: Write failing service test**

Create `services/api/internal/modules/organizations/service_test.go`:
```go
package organizations

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// fakeRepo holds in-memory state and ignores transactions.
type fakeRepo struct {
	orgs        map[uuid.UUID]db.Organization
	slugs       map[string]bool
	members     []db.OrganizationMember
	templates   []db.Role          // organization_id IS NULL
	tmplPerms   map[uuid.UUID][]db.Permission
	orgRoles    []db.Role          // created copies
	rolePerms   map[uuid.UUID][]uuid.UUID
	memberRoles map[uuid.UUID][]uuid.UUID // memberID -> roleIDs
}

func newFakeRepo() *fakeRepo {
	owner := db.Role{ID: uuid.New(), Name: "Owner", Slug: "owner", IsSystem: true}
	mgr := db.Role{ID: uuid.New(), Name: "Manager", Slug: "manager", IsSystem: true}
	pView := db.Permission{ID: uuid.New(), Key: "member.manage"}
	pRole := db.Permission{ID: uuid.New(), Key: "role.manage"}
	return &fakeRepo{
		orgs:      map[uuid.UUID]db.Organization{},
		slugs:     map[string]bool{},
		templates: []db.Role{owner, mgr},
		tmplPerms: map[uuid.UUID][]db.Permission{
			owner.ID: {pView, pRole},
			mgr.ID:   {pView},
		},
		rolePerms:   map[uuid.UUID][]uuid.UUID{},
		memberRoles: map[uuid.UUID][]uuid.UUID{},
	}
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(Repository) error) error { return fn(f) }

func (f *fakeRepo) CreateOrganization(_ context.Context, arg db.CreateOrganizationParams) (db.Organization, error) {
	if f.slugs[arg.Slug] {
		return db.Organization{}, ErrSlugTaken
	}
	o := db.Organization{ID: uuid.New(), Name: arg.Name, Slug: arg.Slug}
	f.orgs[o.ID] = o
	f.slugs[arg.Slug] = true
	return o, nil
}
func (f *fakeRepo) GetOrganizationByID(_ context.Context, id uuid.UUID) (db.Organization, error) {
	o, ok := f.orgs[id]
	if !ok {
		return db.Organization{}, pgx.ErrNoRows
	}
	return o, nil
}
func (f *fakeRepo) ListOrganizationsForUser(_ context.Context, userID uuid.UUID) ([]db.Organization, error) {
	var out []db.Organization
	for _, m := range f.members {
		if m.UserID == userID {
			out = append(out, f.orgs[m.OrganizationID])
		}
	}
	return out, nil
}
func (f *fakeRepo) GetMemberByOrgAndUser(_ context.Context, arg db.GetMemberByOrgAndUserParams) (db.OrganizationMember, error) {
	for _, m := range f.members {
		if m.OrganizationID == arg.OrganizationID && m.UserID == arg.UserID {
			return m, nil
		}
	}
	return db.OrganizationMember{}, pgx.ErrNoRows
}
func (f *fakeRepo) CreateMember(_ context.Context, arg db.CreateMemberParams) (db.OrganizationMember, error) {
	m := db.OrganizationMember{ID: uuid.New(), OrganizationID: arg.OrganizationID, UserID: arg.UserID}
	f.members = append(f.members, m)
	return m, nil
}
func (f *fakeRepo) ListTemplateRoles(_ context.Context) ([]db.Role, error) { return f.templates, nil }
func (f *fakeRepo) ListPermissionsForRole(_ context.Context, roleID uuid.UUID) ([]db.Permission, error) {
	if p, ok := f.tmplPerms[roleID]; ok {
		return p, nil
	}
	return nil, nil
}
func (f *fakeRepo) CreateRole(_ context.Context, arg db.CreateRoleParams) (db.Role, error) {
	r := db.Role{ID: uuid.New(), OrganizationID: arg.OrganizationID, Name: arg.Name, Slug: arg.Slug, IsSystem: arg.IsSystem}
	f.orgRoles = append(f.orgRoles, r)
	return r, nil
}
func (f *fakeRepo) AddRolePermission(_ context.Context, arg db.AddRolePermissionParams) error {
	f.rolePerms[arg.RoleID] = append(f.rolePerms[arg.RoleID], arg.PermissionID)
	return nil
}
func (f *fakeRepo) AddMemberRole(_ context.Context, arg db.AddMemberRoleParams) error {
	f.memberRoles[arg.OrganizationMemberID] = append(f.memberRoles[arg.OrganizationMemberID], arg.RoleID)
	return nil
}

func TestCreate_CopiesTemplatesAndAssignsOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()
	creator := uuid.New()

	org, err := svc.Create(ctx, creator, CreateRequest{Name: "Jakarta Marathon"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if org.Slug != "jakarta-marathon" {
		t.Errorf("slug = %q, want jakarta-marathon", org.Slug)
	}

	// All templates copied as org-owned, is_system=false.
	if len(repo.orgRoles) != len(repo.templates) {
		t.Fatalf("copied %d roles, want %d", len(repo.orgRoles), len(repo.templates))
	}
	for _, r := range repo.orgRoles {
		if r.OrganizationID == nil || *r.OrganizationID != org.ID {
			t.Errorf("copied role %q not owned by org", r.Slug)
		}
		if r.IsSystem {
			t.Errorf("copied role %q should have is_system=false", r.Slug)
		}
	}

	// Creator is a member with the org's Owner role.
	if len(repo.members) != 1 || repo.members[0].UserID != creator {
		t.Fatalf("expected creator to be the sole member")
	}
	memberID := repo.members[0].ID
	var ownerRoleID uuid.UUID
	for _, r := range repo.orgRoles {
		if r.Slug == "owner" {
			ownerRoleID = r.ID
		}
	}
	assigned := repo.memberRoles[memberID]
	if len(assigned) != 1 || assigned[0] != ownerRoleID {
		t.Errorf("creator should be assigned exactly the Owner role")
	}
}

func TestCreate_RejectsDuplicateSlug(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()
	if _, err := svc.Create(ctx, uuid.New(), CreateRequest{Name: "Repeat"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := svc.Create(ctx, uuid.New(), CreateRequest{Name: "Repeat"}); err != ErrSlugTaken {
		t.Fatalf("second create err = %v, want ErrSlugTaken", err)
	}
}
```

- [ ] **Step 9: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/organizations/ -run TestCreate -v; cd ../..
```
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 10: Implement the service**

Create `services/api/internal/modules/organizations/service.go`:
```go
package organizations

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, creatorID uuid.UUID, req CreateRequest) (Response, error) {
	slug := slugify(req.Name)
	var created db.Organization

	err := s.repo.ExecTx(ctx, func(r Repository) error {
		org, err := r.CreateOrganization(ctx, db.CreateOrganizationParams{Name: req.Name, Slug: slug})
		if err != nil {
			return err
		}
		created = org

		member, err := r.CreateMember(ctx, db.CreateMemberParams{OrganizationID: org.ID, UserID: creatorID})
		if err != nil {
			return err
		}

		templates, err := r.ListTemplateRoles(ctx)
		if err != nil {
			return err
		}

		orgID := org.ID
		var ownerRoleID uuid.UUID
		for _, tmpl := range templates {
			perms, err := r.ListPermissionsForRole(ctx, tmpl.ID)
			if err != nil {
				return err
			}
			copied, err := r.CreateRole(ctx, db.CreateRoleParams{
				OrganizationID: &orgID,
				Name:           tmpl.Name,
				Slug:           tmpl.Slug,
				IsSystem:       false,
			})
			if err != nil {
				return err
			}
			for _, p := range perms {
				if err := r.AddRolePermission(ctx, db.AddRolePermissionParams{RoleID: copied.ID, PermissionID: p.ID}); err != nil {
					return err
				}
			}
			if copied.Slug == "owner" {
				ownerRoleID = copied.ID
			}
		}

		return r.AddMemberRole(ctx, db.AddMemberRoleParams{OrganizationMemberID: member.ID, RoleID: ownerRoleID})
	})
	if err != nil {
		return Response{}, err
	}
	return toResponse(created), nil
}

func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]Response, error) {
	orgs, err := s.repo.ListOrganizationsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Response, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, toResponse(o))
	}
	return out, nil
}

// Get returns the org if the caller is a member or a platform admin.
func (s *Service) Get(ctx context.Context, orgID, userID uuid.UUID, isPlatformAdmin bool) (Response, error) {
	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Response{}, ErrNotFound
	} else if err != nil {
		return Response{}, err
	}
	if !isPlatformAdmin {
		if _, err := s.repo.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{OrganizationID: orgID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
			return Response{}, ErrForbidden
		} else if err != nil {
			return Response{}, err
		}
	}
	return toResponse(org), nil
}

func toResponse(o db.Organization) Response {
	return Response{ID: o.ID, Name: o.Name, Slug: o.Slug, CreatedAt: o.CreatedAt}
}
```

- [ ] **Step 11: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/organizations/ -v; cd ../..
```
Expected: PASS (slug + both create tests). Clean up the `pgx` guard in `repository.go` if the build flagged it.

- [ ] **Step 12: Commit**

```bash
git add services/api/internal/modules/organizations
git commit -m "feat(org): add organizations module with template-copy on create"
```

---
