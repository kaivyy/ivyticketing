# Phase 2 Plan — Part 1: Foundation (Tasks 1-5)

> Part of the Phase 2 implementation plan. Index: [2026-06-07-phase2-auth-rbac-multitenant.md](2026-06-07-phase2-auth-rbac-multitenant.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

## Task 1: Add dependencies, config, and sqlc overrides

**Files:**
- Modify: `services/api/go.mod` (via `go get`)
- Modify: `services/api/sqlc.yaml`
- Modify: `services/api/internal/app/config.go`
- Modify: `services/api/internal/app/config_test.go`
- Modify: `services/api/.env.example`
- Modify: `.env.example` (root)

- [ ] **Step 1: Add Go dependencies**

Run:
```bash
cd services/api && \
  go get golang.org/x/crypto/bcrypt && \
  go get github.com/golang-jwt/jwt/v5 && \
  go get github.com/google/uuid && \
  cd ../..
```
Expected: adds the three modules to `go.mod`.

- [ ] **Step 2: Add sqlc type overrides**

Replace `services/api/sqlc.yaml` entirely:
```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "../../database/queries"
    schema: "../../database/migrations"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "uuid"
            nullable: true
            go_type:
              type: "uuid.UUID"
              pointer: true
          - db_type: "pg_catalog.timestamptz"
            go_type: "time.Time"
          - db_type: "pg_catalog.timestamptz"
            nullable: true
            go_type:
              type: "time.Time"
              pointer: true
          - db_type: "citext"
            go_type: "string"
```
Note: existing Phase 1 generated code (`system.sql.go`) will be regenerated; `HealthPing` now returns `time.Time` instead of `pgtype.Timestamptz`. Task 2 regenerates and Task 3 confirms the build still passes.

- [ ] **Step 3: Write failing test for config**

Add to `services/api/internal/app/config_test.go` (keep existing tests):
```go
func TestLoadConfig_AuthDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("ACCESS_TOKEN_TTL", "")
	t.Setenv("REFRESH_TOKEN_TTL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("AccessTokenTTL = %v, want 15m", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 168*time.Hour {
		t.Errorf("RefreshTokenTTL = %v, want 168h", cfg.RefreshTokenTTL)
	}
}

func TestLoadConfig_MissingJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for missing JWT_SECRET, got nil")
	}
}
```
Add `"time"` to the test file imports.

- [ ] **Step 4: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig -v; cd ../..
```
Expected: FAIL — `cfg.AccessTokenTTL` undefined / build error.

- [ ] **Step 5: Extend config**

Replace `services/api/internal/app/config.go` entirely:
```go
package app

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	AppEnv          string
	AppName         string
	APIPort         string
	DatabaseURL     string
	RedisURL        string
	WebOrigin       string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func LoadConfig() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "local"),
		AppName:     getEnv("APP_NAME", "ivyticketing"),
		APIPort:     getEnv("API_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		WebOrigin:   getEnv("WEB_ORIGIN", "http://localhost:4321"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("config: REDIS_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("config: JWT_SECRET is required")
	}

	accessTTL, err := getDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTTL, err := getDuration("REFRESH_TOKEN_TTL", 168*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.AccessTokenTTL = accessTTL
	cfg.RefreshTokenTTL = refreshTTL

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s invalid duration: %w", key, err)
	}
	return d, nil
}
```

- [ ] **Step 6: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig -v; cd ../..
```
Expected: PASS (all subtests, including the Phase 1 ones).

- [ ] **Step 7: Update env templates**

Append to `.env.example` (root) AND `services/api/.env.example`:
```bash
JWT_SECRET=
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
```

- [ ] **Step 8: Commit**

```bash
git add services/api/go.mod services/api/go.sum services/api/sqlc.yaml \
  services/api/internal/app/config.go services/api/internal/app/config_test.go \
  services/api/.env.example .env.example
git commit -m "feat(api): add auth config (jwt/ttl) and sqlc type overrides"
```

---

## Task 2: Database migrations + seed

**Files:**
- Create: `database/migrations/00002_create_users.sql`
- Create: `database/migrations/00003_create_organizations.sql`
- Create: `database/migrations/00004_create_rbac.sql`
- Create: `database/migrations/00005_create_refresh_tokens.sql`
- Create: `database/migrations/00006_create_audit_logs.sql`
- Create: `database/migrations/00007_seed_rbac_catalog.sql`

- [ ] **Step 1: Users migration**

Create `database/migrations/00002_create_users.sql`:
```sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email             citext NOT NULL UNIQUE,
    password_hash     text,
    full_name         text NOT NULL,
    phone             text,
    is_platform_admin boolean NOT NULL DEFAULT false,
    email_verified_at timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
```

- [ ] **Step 2: Organizations migration**

Create `database/migrations/00003_create_organizations.sql`:
```sql
-- +goose Up
CREATE TABLE organizations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    slug       text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organization_members (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, user_id)
);
CREATE INDEX idx_org_members_user ON organization_members(user_id);

-- +goose Down
DROP TABLE organization_members;
DROP TABLE organizations;
```

- [ ] **Step 3: RBAC migration**

Create `database/migrations/00004_create_rbac.sql`:
```sql
-- +goose Up
CREATE TABLE roles (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE,
    name            text NOT NULL,
    slug            text NOT NULL,
    is_system       boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, slug)
);
-- Enforce uniqueness for template roles (organization_id IS NULL),
-- since UNIQUE treats NULLs as distinct.
CREATE UNIQUE INDEX idx_roles_template_slug
    ON roles(slug) WHERE organization_id IS NULL;

CREATE TABLE permissions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key         text NOT NULL UNIQUE,
    description text NOT NULL DEFAULT ''
);

CREATE TABLE role_permissions (
    role_id       uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE member_roles (
    organization_member_id uuid NOT NULL REFERENCES organization_members(id) ON DELETE CASCADE,
    role_id                uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (organization_member_id, role_id)
);

-- +goose Down
DROP TABLE member_roles;
DROP TABLE role_permissions;
DROP TABLE permissions;
DROP TABLE roles;
```

- [ ] **Step 4: Refresh tokens migration**

Create `database/migrations/00005_create_refresh_tokens.sql`:
```sql
-- +goose Up
CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);

-- +goose Down
DROP TABLE refresh_tokens;
```

- [ ] **Step 5: Audit logs migration**

Create `database/migrations/00006_create_audit_logs.sql`:
```sql
-- +goose Up
CREATE TABLE audit_logs (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid REFERENCES organizations(id) ON DELETE SET NULL,
    actor_user_id   uuid REFERENCES users(id) ON DELETE SET NULL,
    action          text NOT NULL,
    target_type     text,
    target_id       text,
    metadata        jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_org ON audit_logs(organization_id);

-- +goose Down
DROP TABLE audit_logs;
```

- [ ] **Step 6: Seed migration (permission catalog + role templates)**

Create `database/migrations/00007_seed_rbac_catalog.sql`. This is reference data, written idempotently so re-running up is safe.
```sql
-- +goose Up
INSERT INTO permissions (key, description) VALUES
    ('member.manage',       'Manage organization staff and their roles'),
    ('role.manage',         'Create and edit custom roles'),
    ('organization.manage', 'Edit organization settings'),
    ('event.create',        'Create events'),
    ('event.edit',          'Edit events'),
    ('event.publish',       'Publish events'),
    ('event.delete',        'Delete events'),
    ('category.manage',     'Manage event categories'),
    ('form.manage',         'Manage registration forms'),
    ('order.view',          'View orders'),
    ('order.refund',        'Refund orders'),
    ('payment.view',        'View payments'),
    ('payment.refund',      'Refund payments'),
    ('participant.view',    'View participants'),
    ('participant.export',  'Export participant data'),
    ('coupon.manage',       'Manage coupons'),
    ('bib.manage',          'Manage BIB numbers'),
    ('racepack.scan',       'Scan racepack pickups'),
    ('racepack.manage',     'Manage racepack pickup config'),
    ('report.view',         'View reports'),
    ('broadcast.send',      'Send broadcasts')
ON CONFLICT (key) DO NOTHING;

INSERT INTO roles (organization_id, name, slug, is_system) VALUES
    (NULL, 'Owner',            'owner',            true),
    (NULL, 'Manager',          'manager',          true),
    (NULL, 'Finance',          'finance',          true),
    (NULL, 'Customer Service', 'customer-service', true),
    (NULL, 'Racepack Staff',   'racepack-staff',   true)
ON CONFLICT DO NOTHING;

-- Owner: all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.organization_id IS NULL AND r.slug = 'owner'
ON CONFLICT DO NOTHING;

-- Manager
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'event.create','event.edit','event.publish','event.delete',
    'category.manage','form.manage','participant.view','order.view',
    'report.view','broadcast.send','coupon.manage','bib.manage')
WHERE r.organization_id IS NULL AND r.slug = 'manager'
ON CONFLICT DO NOTHING;

-- Finance
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'order.view','order.refund','payment.view','payment.refund','report.view')
WHERE r.organization_id IS NULL AND r.slug = 'finance'
ON CONFLICT DO NOTHING;

-- Customer Service
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'participant.view','order.view')
WHERE r.organization_id IS NULL AND r.slug = 'customer-service'
ON CONFLICT DO NOTHING;

-- Racepack Staff
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'racepack.scan','racepack.manage','participant.view')
WHERE r.organization_id IS NULL AND r.slug = 'racepack-staff'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE organization_id IS NULL);
DELETE FROM roles WHERE organization_id IS NULL;
DELETE FROM permissions;
```

- [ ] **Step 7: Apply migrations and verify roundtrip**

Run:
```bash
make migrate-up
make migrate-down && make migrate-up
```
Expected: all migrations apply, roll back, and re-apply cleanly with no errors.

- [ ] **Step 8: Verify seed landed**

Run:
```bash
PG="$(brew --prefix)/opt/postgresql@16/bin"
"$PG/psql" ivyticketing -c "SELECT count(*) FROM permissions;"
"$PG/psql" ivyticketing -c "SELECT slug FROM roles WHERE organization_id IS NULL ORDER BY slug;"
```
Expected: 21 permissions; 5 template roles (customer-service, finance, manager, owner, racepack-staff).

- [ ] **Step 9: Commit**

```bash
git add database/migrations
git commit -m "feat(db): add auth/org/rbac/refresh/audit migrations and rbac seed"
```

---

## Task 3: Security primitives — password, JWT, refresh token

**Files:**
- Create: `services/api/internal/platform/security/password.go`
- Create: `services/api/internal/platform/security/jwt.go`
- Create: `services/api/internal/platform/security/token.go`
- Test: `services/api/internal/platform/security/security_test.go`

- [ ] **Step 1: Write failing tests**

Create `services/api/internal/platform/security/security_test.go`:
```go
package security

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPassword_HashVerify(t *testing.T) {
	hash, err := HashPassword("s3cret-pw")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "s3cret-pw" {
		t.Fatal("hash must not equal plaintext")
	}
	if !VerifyPassword(hash, "s3cret-pw") {
		t.Error("correct password should verify")
	}
	if VerifyPassword(hash, "wrong-pw") {
		t.Error("wrong password should not verify")
	}
}

func TestJWT_SignVerify(t *testing.T) {
	signer := NewJWTSigner("test-secret", time.Minute)
	uid := uuid.New()

	tok, err := signer.Sign(uid, true)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := signer.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UserID != uid {
		t.Errorf("UserID = %v, want %v", claims.UserID, uid)
	}
	if !claims.IsPlatformAdmin {
		t.Error("IsPlatformAdmin = false, want true")
	}
}

func TestJWT_RejectExpired(t *testing.T) {
	signer := NewJWTSigner("test-secret", -time.Minute) // already expired
	tok, err := signer.Sign(uuid.New(), false)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := signer.Verify(tok); err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestJWT_RejectWrongSecret(t *testing.T) {
	tok, err := NewJWTSigner("secret-a", time.Minute).Sign(uuid.New(), false)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := NewJWTSigner("secret-b", time.Minute).Verify(tok); err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestRefreshToken_RawDiffersFromHash(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if raw == hash {
		t.Fatal("raw token must differ from its hash")
	}
	if HashToken(raw) != hash {
		t.Error("HashToken(raw) should equal returned hash")
	}
	if HashToken("other") == hash {
		t.Error("different input should not match hash")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/security/ -v; cd ../..
```
Expected: FAIL — `undefined: HashPassword`, `undefined: NewJWTSigner`, etc.

- [ ] **Step 3: Implement password**

Create `services/api/internal/platform/security/password.go`:
```go
package security

import "golang.org/x/crypto/bcrypt"

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
```

- [ ] **Step 4: Implement JWT**

Create `services/api/internal/platform/security/jwt.go`:
```go
package security

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID          uuid.UUID
	IsPlatformAdmin bool
}

type JWTSigner struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTSigner(secret string, ttl time.Duration) *JWTSigner {
	return &JWTSigner{secret: []byte(secret), ttl: ttl}
}

func (s *JWTSigner) Sign(userID uuid.UUID, isPlatformAdmin bool) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":               userID.String(),
		"is_platform_admin": isPlatformAdmin,
		"iat":               now.Unix(),
		"exp":               now.Add(s.ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *JWTSigner) Verify(tokenStr string) (Claims, error) {
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return Claims{}, fmt.Errorf("invalid token: %w", err)
	}
	mc, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, fmt.Errorf("invalid claims")
	}
	sub, _ := mc["sub"].(string)
	uid, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub claim: %w", err)
	}
	isAdmin, _ := mc["is_platform_admin"].(bool)
	return Claims{UserID: uid, IsPlatformAdmin: isAdmin}, nil
}
```

- [ ] **Step 5: Implement refresh token**

Create `services/api/internal/platform/security/token.go`:
```go
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateRefreshToken returns (raw, hash). The raw value goes to the client
// cookie; only the hash is stored in the DB.
func GenerateRefreshToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	return raw, HashToken(raw), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/platform/security/ -v; cd ../..
```
Expected: PASS (all four tests).

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/platform/security services/api/go.mod services/api/go.sum
git commit -m "feat(api): add password, jwt, and refresh-token security primitives"
```

---

## Task 4: Shared error envelope + auth context

**Files:**
- Create: `services/api/internal/platform/errors/errors.go`
- Test: `services/api/internal/platform/errors/errors_test.go`
- Create: `services/api/internal/platform/authctx/authctx.go`
- Test: `services/api/internal/platform/authctx/authctx_test.go`

The error envelope matches struktur.md §19: `{ "error": { code, message, requestId } }`. `requestId` is read from the `X-Request-Id` header set by the Phase 1 `RequestID` middleware.

- [ ] **Step 1: Write failing test for errors**

Create `services/api/internal/platform/errors/errors_test.go`:
```go
package errors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError_ShapeAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "req_123")

	WriteError(rec, req, New(http.StatusForbidden, "FORBIDDEN", "no access"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "FORBIDDEN" || body.Error.Message != "no access" {
		t.Errorf("unexpected body: %+v", body.Error)
	}
	if body.Error.RequestID != "req_123" {
		t.Errorf("requestId = %q, want req_123", body.Error.RequestID)
	}
}

func TestWriteError_NonAPIErrorBecomes500(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	WriteError(rec, req, errString("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "INTERNAL" {
		t.Errorf("code = %q, want INTERNAL", body.Error.Code)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/errors/ -v; cd ../..
```
Expected: FAIL — `undefined: WriteError`, `undefined: New`.

- [ ] **Step 3: Implement errors**

Create `services/api/internal/platform/errors/errors.go`:
```go
package errors

import (
	"encoding/json"
	"errors"
	"net/http"
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func New(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

type envelope struct {
	Error body `json:"error"`
}

type body struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

// WriteError renders err as the standard JSON envelope. Non-APIErrors are
// masked as a generic 500 (struktur.md §19: never leak internal errors).
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := &APIError{Status: http.StatusInternalServerError, Code: "INTERNAL", Message: "internal server error"}
	var ae *APIError
	if errors.As(err, &ae) {
		apiErr = ae
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.Status)
	json.NewEncoder(w).Encode(envelope{Error: body{
		Code:      apiErr.Code,
		Message:   apiErr.Message,
		RequestID: r.Header.Get("X-Request-Id"),
	}})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/platform/errors/ -v; cd ../..
```
Expected: PASS (both tests).

- [ ] **Step 5: Write failing test for authctx**

Create `services/api/internal/platform/authctx/authctx_test.go`:
```go
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
```

- [ ] **Step 6: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/authctx/ -v; cd ../..
```
Expected: FAIL — `undefined: WithIdentity`, `undefined: Identity`.

- [ ] **Step 7: Implement authctx**

Create `services/api/internal/platform/authctx/authctx.go`:
```go
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
```

- [ ] **Step 8: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/platform/authctx/ -v; cd ../..
```
Expected: PASS (both tests).

- [ ] **Step 9: Commit**

```bash
git add services/api/internal/platform/errors services/api/internal/platform/authctx
git commit -m "feat(api): add shared error envelope and auth identity context"
```

---

## Task 5: sqlc queries + regenerate

**Files:**
- Create: `database/queries/users.sql`
- Create: `database/queries/organizations.sql`
- Create: `database/queries/members.sql`
- Create: `database/queries/roles.sql`
- Create: `database/queries/refresh_tokens.sql`
- Create: `database/queries/audit_logs.sql`
- Regenerate: `services/api/internal/db/*` (via `sqlc generate`)

- [ ] **Step 1: Users queries**

Create `database/queries/users.sql`:
```sql
-- name: CreateUser :one
INSERT INTO users (email, password_hash, full_name, phone)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;
```

- [ ] **Step 2: Organizations queries**

Create `database/queries/organizations.sql`:
```sql
-- name: CreateOrganization :one
INSERT INTO organizations (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: ListOrganizationsForUser :many
SELECT o.* FROM organizations o
JOIN organization_members m ON m.organization_id = o.id
WHERE m.user_id = $1
ORDER BY o.created_at;
```

- [ ] **Step 3: Members queries**

Create `database/queries/members.sql`:
```sql
-- name: CreateMember :one
INSERT INTO organization_members (organization_id, user_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetMemberByOrgAndUser :one
SELECT * FROM organization_members
WHERE organization_id = $1 AND user_id = $2;

-- name: GetMemberByID :one
SELECT * FROM organization_members WHERE id = $1;

-- name: ListMembersByOrg :many
SELECT m.id, m.user_id, m.created_at, u.email, u.full_name
FROM organization_members m
JOIN users u ON u.id = m.user_id
WHERE m.organization_id = $1
ORDER BY m.created_at;

-- name: DeleteMember :exec
DELETE FROM organization_members WHERE id = $1 AND organization_id = $2;

-- name: AddMemberRole :exec
INSERT INTO member_roles (organization_member_id, role_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ClearMemberRoles :exec
DELETE FROM member_roles WHERE organization_member_id = $1;

-- name: ListRolesForMember :many
SELECT r.* FROM roles r
JOIN member_roles mr ON mr.role_id = r.id
WHERE mr.organization_member_id = $1
ORDER BY r.name;

-- name: ListPermissionsForMember :many
SELECT DISTINCT p.key
FROM member_roles mr
JOIN role_permissions rp ON rp.role_id = mr.role_id
JOIN permissions p ON p.id = rp.permission_id
WHERE mr.organization_member_id = $1;

-- name: CountOwnersInOrg :one
SELECT count(DISTINCT mr.organization_member_id)
FROM member_roles mr
JOIN roles r ON r.id = mr.role_id
WHERE r.organization_id = $1 AND r.slug = 'owner';

-- name: MemberHasRoleSlug :one
SELECT EXISTS (
    SELECT 1 FROM member_roles mr
    JOIN roles r ON r.id = mr.role_id
    WHERE mr.organization_member_id = $1 AND r.slug = $2
);
```

- [ ] **Step 4: Roles queries**

Create `database/queries/roles.sql`:
```sql
-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY key;

-- name: GetPermissionByKey :one
SELECT * FROM permissions WHERE key = $1;

-- name: ListTemplateRoles :many
SELECT * FROM roles WHERE organization_id IS NULL ORDER BY name;

-- name: ListPermissionsForRole :many
SELECT p.* FROM permissions p
JOIN role_permissions rp ON rp.permission_id = p.id
WHERE rp.role_id = $1
ORDER BY p.key;

-- name: CreateRole :one
INSERT INTO roles (organization_id, name, slug, is_system)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRoleByID :one
SELECT * FROM roles WHERE id = $1;

-- name: GetRoleByOrgAndSlug :one
SELECT * FROM roles WHERE organization_id = $1 AND slug = $2;

-- name: ListRolesByOrg :many
SELECT * FROM roles WHERE organization_id = $1 ORDER BY name;

-- name: UpdateRoleName :one
UPDATE roles SET name = $2 WHERE id = $1 AND organization_id = $3
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1 AND organization_id = $2 AND is_system = false;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ClearRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1;

-- name: CountMembersWithRole :one
SELECT count(*) FROM member_roles WHERE role_id = $1;
```

- [ ] **Step 5: Refresh-token queries**

Create `database/queries/refresh_tokens.sql`:
```sql
-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;
```

- [ ] **Step 6: Audit-log queries**

Create `database/queries/audit_logs.sql`:
```sql
-- name: CreateAuditLog :exec
INSERT INTO audit_logs (organization_id, actor_user_id, action, target_type, target_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListAuditLogsByOrg :many
SELECT * FROM audit_logs WHERE organization_id = $1 ORDER BY created_at DESC LIMIT $2;
```

- [ ] **Step 7: Regenerate sqlc code and verify build**

Run:
```bash
make sqlc
cd services/api && go build ./... && cd ../..
```
Expected: `sqlc generate` succeeds with no errors; new files appear under `services/api/internal/db/` (`users.sql.go`, `organizations.sql.go`, `members.sql.go`, `roles.sql.go`, `refresh_tokens.sql.go`, `audit_logs.sql.go`, updated `models.go`). Build passes. The Phase 1 `system.sql.go` `HealthPing` now returns `time.Time` — `system` handler doesn't use the return value, so it still builds.

- [ ] **Step 8: Commit**

```bash
git add database/queries services/api/internal/db
git commit -m "feat(db): add sqlc queries for auth/org/rbac and regenerate code"
```

---
