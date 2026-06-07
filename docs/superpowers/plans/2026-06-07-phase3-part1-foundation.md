# Phase 3 Plan — Part 1: Foundation (Tasks 1-4)

> Part of the Phase 3 implementation plan. Index: [2026-06-07-phase3-event-category-management.md](2026-06-07-phase3-event-category-management.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

---

## Task 1: Storage config

**Files:**
- Modify: `services/api/internal/app/config.go`
- Modify: `services/api/internal/app/config_test.go`
- Modify: `services/api/.env.example`
- Modify: `.env.example` (root)

- [ ] **Step 1: Write the failing test**

Add to `services/api/internal/app/config_test.go` (keep existing tests):
```go
func TestLoadConfig_StorageDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("STORAGE_DRIVER", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StorageDriver != "local" {
		t.Errorf("StorageDriver = %q, want local", cfg.StorageDriver)
	}
	if cfg.StorageLocalPath != "./var/media" {
		t.Errorf("StorageLocalPath = %q, want ./var/media", cfg.StorageLocalPath)
	}
	if cfg.StorageUploadMaxBytes != 5242880 {
		t.Errorf("StorageUploadMaxBytes = %d, want 5242880", cfg.StorageUploadMaxBytes)
	}
}

func TestLoadConfig_CloudDriverRequiresCredentials(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("STORAGE_DRIVER", "r2")
	t.Setenv("STORAGE_BUCKET", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for cloud driver without credentials, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig_Storage -v; cd ../..
```
Expected: FAIL — `cfg.StorageDriver` undefined.

- [ ] **Step 3: Extend config**

In `services/api/internal/app/config.go`, add fields to the `Config` struct (after `RefreshTokenTTL`):
```go
	StorageDriver         string
	StorageLocalPath      string
	StoragePublicBaseURL  string
	StorageUploadMaxBytes int64
	StorageBucket         string
	StorageEndpoint       string
	StorageAccessKey      string
	StorageSecretKey      string
	StorageRegion         string
```

In `LoadConfig`, after the TTL block and before `return cfg, nil`, add:
```go
	cfg.StorageDriver = getEnv("STORAGE_DRIVER", "local")
	cfg.StorageLocalPath = getEnv("STORAGE_LOCAL_PATH", "./var/media")
	cfg.StoragePublicBaseURL = getEnv("STORAGE_PUBLIC_BASE_URL", "http://localhost:8080")
	cfg.StorageBucket = os.Getenv("STORAGE_BUCKET")
	cfg.StorageEndpoint = os.Getenv("STORAGE_ENDPOINT")
	cfg.StorageAccessKey = os.Getenv("STORAGE_ACCESS_KEY")
	cfg.StorageSecretKey = os.Getenv("STORAGE_SECRET_KEY")
	cfg.StorageRegion = os.Getenv("STORAGE_REGION")

	maxBytes, err := getInt64("STORAGE_UPLOAD_MAX_BYTES", 5242880)
	if err != nil {
		return Config{}, err
	}
	cfg.StorageUploadMaxBytes = maxBytes

	if cfg.StorageDriver != "local" {
		if cfg.StorageBucket == "" || cfg.StorageAccessKey == "" || cfg.StorageSecretKey == "" {
			return Config{}, fmt.Errorf("config: STORAGE_BUCKET/ACCESS_KEY/SECRET_KEY required when STORAGE_DRIVER=%s", cfg.StorageDriver)
		}
	}
```

Add the `getInt64` helper at the bottom:
```go
func getInt64(key string, fallback int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s invalid int: %w", key, err)
	}
	return n, nil
}
```
Add `"strconv"` to the imports.

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/app/ -v; cd ../..
```
Expected: PASS (all config tests, including Phase 1/2 ones).

- [ ] **Step 5: Update env templates**

Append to BOTH `.env.example` (root) and `services/api/.env.example`:
```bash
STORAGE_DRIVER=local
STORAGE_LOCAL_PATH=./var/media
STORAGE_PUBLIC_BASE_URL=http://localhost:8080
STORAGE_UPLOAD_MAX_BYTES=5242880
STORAGE_BUCKET=
STORAGE_ENDPOINT=
STORAGE_ACCESS_KEY=
STORAGE_SECRET_KEY=
STORAGE_REGION=
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/app/config.go services/api/internal/app/config_test.go services/api/.env.example .env.example
git commit -m "feat(api): add storage configuration"
```

---

## Task 2: Database migrations

**Files:**
- Create: `database/migrations/00008_create_events.sql`
- Create: `database/migrations/00009_create_event_categories.sql`

- [ ] **Step 1: Events migration**

Create `database/migrations/00008_create_events.sql`:
```sql
-- +goose Up
CREATE TABLE events (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name              text NOT NULL,
    slug              text NOT NULL,
    description       text,
    event_type        text NOT NULL,
    status            text NOT NULL DEFAULT 'draft',
    banner_object_key text,
    logo_object_key   text,
    venue_name        text,
    venue_address     text,
    starts_at         timestamptz,
    ends_at           timestamptz,
    faq               text,
    terms             text,
    waiver            text,
    published_at      timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT events_status_check CHECK (status IN ('draft', 'published', 'archived')),
    UNIQUE (organization_id, slug)
);
CREATE INDEX idx_events_org ON events(organization_id);
CREATE INDEX idx_events_org_status ON events(organization_id, status);

-- +goose Down
DROP TABLE events;
```

- [ ] **Step 2: Event categories migration**

Create `database/migrations/00009_create_event_categories.sql`:
```sql
-- +goose Up
CREATE TABLE event_categories (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id              uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name                  text NOT NULL,
    price                 bigint NOT NULL,
    capacity              integer NOT NULL,
    registration_opens_at  timestamptz NOT NULL,
    registration_closes_at timestamptz NOT NULL,
    bib_prefix            text,
    min_age               integer,
    max_order_per_user    integer NOT NULL DEFAULT 1,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT event_categories_price_check CHECK (price >= 0),
    CONSTRAINT event_categories_capacity_check CHECK (capacity > 0),
    CONSTRAINT event_categories_min_age_check CHECK (min_age IS NULL OR min_age >= 0),
    CONSTRAINT event_categories_max_order_check CHECK (max_order_per_user >= 1),
    UNIQUE (event_id, name)
);
CREATE INDEX idx_event_categories_event ON event_categories(event_id);
CREATE INDEX idx_event_categories_org ON event_categories(organization_id);

-- +goose Down
DROP TABLE event_categories;
```

- [ ] **Step 3: Apply migrations and verify roundtrip**

Run:
```bash
make migrate-up
make migrate-down && make migrate-up
```
Expected: both migrations apply, roll back to a clean state, and re-apply with no errors.

- [ ] **Step 4: Commit**

```bash
git add database/migrations
git commit -m "feat(db): add events and event_categories migrations"
```

---

## Task 3: Storage interface + local driver + S3 stub

**Files:**
- Create: `services/api/internal/platform/storage/storage.go`
- Create: `services/api/internal/platform/storage/local.go`
- Create: `services/api/internal/platform/storage/s3.go`
- Test: `services/api/internal/platform/storage/local_test.go`

- [ ] **Step 1: Define the interface and factory**

Create `services/api/internal/platform/storage/storage.go`:
```go
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotConfigured = errors.New("storage: driver not configured")

// Storage abstracts object storage across local disk and S3-compatible clouds.
type Storage interface {
	// PresignUpload returns a direct-to-storage upload ticket if supported.
	// ok=false means the backend cannot presign (local) — use Put instead.
	PresignUpload(ctx context.Context, key, contentType string, ttl time.Duration) (PutTicket, bool, error)
	// Put writes bytes directly (local driver, or fallback).
	Put(ctx context.Context, key string, r io.Reader, contentType string) error
	// PublicURL builds the readable URL for a stored object.
	PublicURL(key string) string
	// Delete removes an object (best-effort).
	Delete(ctx context.Context, key string) error
}

type PutTicket struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Expires time.Time         `json:"expires"`
}

type Config struct {
	Driver        string
	LocalPath     string
	PublicBaseURL string
	Bucket        string
	Endpoint      string
	AccessKey     string
	SecretKey     string
	Region        string
}

// New builds a Storage from config.
func New(cfg Config) (Storage, error) {
	switch cfg.Driver {
	case "local", "":
		return NewLocal(cfg.LocalPath, cfg.PublicBaseURL)
	case "r2", "tencent", "s3":
		return NewS3(cfg)
	default:
		return nil, errors.New("storage: unknown driver " + cfg.Driver)
	}
}
```

- [ ] **Step 2: Write the failing test for local driver**

Create `services/api/internal/platform/storage/local_test.go`:
```go
package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocal_PutAndPublicURL(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocal(dir, "http://localhost:8080")
	if err != nil {
		t.Fatalf("new local: %v", err)
	}
	key := "org/o1/event/e1/banner/abc.png"
	if err := s.Put(context.Background(), key, strings.NewReader("imgdata"), "image/png"); err != nil {
		t.Fatalf("put: %v", err)
	}
	// File written under dir/key.
	b, err := os.ReadFile(filepath.Join(dir, key))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(b) != "imgdata" {
		t.Errorf("content = %q, want imgdata", string(b))
	}
	if got := s.PublicURL(key); got != "http://localhost:8080/media/"+key {
		t.Errorf("PublicURL = %q", got)
	}
}

func TestLocal_PresignNotSupported(t *testing.T) {
	s, _ := NewLocal(t.TempDir(), "http://localhost:8080")
	_, ok, err := s.PresignUpload(context.Background(), "k", "image/png", time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if ok {
		t.Error("local should not support presign (ok=true)")
	}
}

func TestLocal_RejectsPathTraversal(t *testing.T) {
	s, _ := NewLocal(t.TempDir(), "http://localhost:8080")
	if err := s.Put(context.Background(), "../escape.png", strings.NewReader("x"), "image/png"); err == nil {
		t.Fatal("expected error for path traversal key, got nil")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/storage/ -v; cd ../..
```
Expected: FAIL — `undefined: NewLocal`.

- [ ] **Step 4: Implement the local driver**

Create `services/api/internal/platform/storage/local.go`:
```go
package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Local struct {
	root          string
	publicBaseURL string
}

func NewLocal(root, publicBaseURL string) (*Local, error) {
	if root == "" {
		return nil, errors.New("storage: local path is empty")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Local{root: root, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}, nil
}

// safePath resolves key under root and rejects traversal outside root.
func (l *Local) safePath(key string) (string, error) {
	clean := filepath.Clean("/" + key) // forces absolute, collapses ..
	full := filepath.Join(l.root, clean)
	rel, err := filepath.Rel(l.root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("storage: invalid key")
	}
	return full, nil
}

func (l *Local) PresignUpload(_ context.Context, _ , _ string, _ time.Duration) (PutTicket, bool, error) {
	return PutTicket{}, false, nil
}

func (l *Local) Put(_ context.Context, key string, r io.Reader, _ string) error {
	full, err := l.safePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (l *Local) PublicURL(key string) string {
	return l.publicBaseURL + "/media/" + key
}

func (l *Local) Delete(_ context.Context, key string) error {
	full, err := l.safePath(key)
	if err != nil {
		return err
	}
	err = os.Remove(full)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
```
Note: the `PresignUpload` signature has two unnamed `string` params (`key`, `contentType`) — write it as `func (l *Local) PresignUpload(_ context.Context, _, _ string, _ time.Duration)`. Adjust if `gofmt` complains.

- [ ] **Step 5: Implement the S3 stub**

Create `services/api/internal/platform/storage/s3.go`:
```go
package storage

import (
	"context"
	"io"
	"strings"
	"time"
)

// S3 is an S3-compatible driver (R2/Tencent/AWS). Phase 3 ships the contract;
// the implementation is filled in when cloud credentials are available.
// Until then every operation returns ErrNotConfigured.
type S3 struct {
	cfg Config
}

func NewS3(cfg Config) (*S3, error) {
	return &S3{cfg: cfg}, nil
}

// PresignUpload will issue a presigned PUT URL (direct-to-storage upload) so
// the API never proxies file bytes. Contract: ok=true on success.
func (s *S3) PresignUpload(_ context.Context, _, _ string, _ time.Duration) (PutTicket, bool, error) {
	return PutTicket{}, false, ErrNotConfigured
}

func (s *S3) Put(_ context.Context, _ string, _ io.Reader, _ string) error {
	return ErrNotConfigured
}

func (s *S3) PublicURL(key string) string {
	base := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	return base + "/" + key
}

func (s *S3) Delete(_ context.Context, _ string) error {
	return ErrNotConfigured
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/platform/storage/ -v && go build ./...; cd ../..
```
Expected: PASS (3 local tests); build OK.

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/platform/storage
git commit -m "feat(api): add pluggable storage interface, local driver, and s3 stub"
```

---

## Task 4: sqlc queries + regenerate

**Files:**
- Create: `database/queries/events.sql`
- Create: `database/queries/event_categories.sql`
- Regenerate: `services/api/internal/db/*`

- [ ] **Step 1: Events queries**

Create `database/queries/events.sql`:
```sql
-- name: CreateEvent :one
INSERT INTO events (organization_id, name, slug, description, event_type,
    venue_name, venue_address, starts_at, ends_at, faq, terms, waiver)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetEventByID :one
SELECT * FROM events WHERE id = $1;

-- name: GetEventByOrgAndSlug :one
SELECT * FROM events WHERE organization_id = $1 AND slug = $2;

-- name: ListEventsByOrg :many
SELECT * FROM events WHERE organization_id = $1 ORDER BY created_at DESC;

-- name: UpdateEvent :one
UPDATE events SET
    name = $2, description = $3, event_type = $4,
    venue_name = $5, venue_address = $6, starts_at = $7, ends_at = $8,
    faq = $9, terms = $10, waiver = $11, updated_at = now()
WHERE id = $1 AND organization_id = $12
RETURNING *;

-- name: UpdateEventStatus :one
UPDATE events SET status = $2, published_at = $3, updated_at = now()
WHERE id = $1 AND organization_id = $4
RETURNING *;

-- name: SetEventMediaKey :one
UPDATE events SET banner_object_key = COALESCE($2, banner_object_key),
    logo_object_key = COALESCE($3, logo_object_key), updated_at = now()
WHERE id = $1 AND organization_id = $4
RETURNING *;

-- name: DeleteEvent :exec
DELETE FROM events WHERE id = $1 AND organization_id = $2;

-- name: CountCategoriesForEvent :one
SELECT count(*) FROM event_categories WHERE event_id = $1;
```
Note: `SetEventMediaKey` uses `COALESCE($n, col)` so a NULL param leaves the column unchanged — the service passes only the relevant `kind`'s key as non-NULL. Params for nullable text are `pgtype.Text` after generation; check the generated signature.

- [ ] **Step 2: Event categories queries**

Create `database/queries/event_categories.sql`:
```sql
-- name: CreateCategory :one
INSERT INTO event_categories (organization_id, event_id, name, price, capacity,
    registration_opens_at, registration_closes_at, bib_prefix, min_age, max_order_per_user)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetCategoryByID :one
SELECT * FROM event_categories WHERE id = $1;

-- name: ListCategoriesByEvent :many
SELECT * FROM event_categories WHERE event_id = $1 ORDER BY created_at;

-- name: UpdateCategory :one
UPDATE event_categories SET
    name = $2, price = $3, capacity = $4,
    registration_opens_at = $5, registration_closes_at = $6,
    bib_prefix = $7, min_age = $8, max_order_per_user = $9, updated_at = now()
WHERE id = $1 AND event_id = $10
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM event_categories WHERE id = $1 AND event_id = $2;
```

- [ ] **Step 3: Public catalog queries**

Append to `database/queries/events.sql`:
```sql
-- name: ListPublishedEventsByOrgSlug :many
SELECT e.* FROM events e
JOIN organizations o ON o.id = e.organization_id
WHERE o.slug = $1 AND e.status = 'published'
ORDER BY e.starts_at NULLS LAST, e.created_at DESC;

-- name: GetPublishedEventByOrgAndSlug :one
SELECT e.* FROM events e
JOIN organizations o ON o.id = e.organization_id
WHERE o.slug = $1 AND e.slug = $2 AND e.status = 'published';
```

- [ ] **Step 4: Regenerate and verify build**

Run:
```bash
make sqlc
cd services/api && go build ./...; cd ../..
```
Expected: `sqlc generate` succeeds; new files `events.sql.go`, `event_categories.sql.go`, updated `models.go` with `Event` and `EventCategory` structs. Build passes.

- [ ] **Step 5: Inspect generated types**

Run:
```bash
sed -n '/type Event struct/,/^}/p;/type EventCategory struct/,/^}/p' services/api/internal/db/models.go
```
Note the field types (e.g. `Description pgtype.Text`, `StartsAt pgtype.Timestamptz`, `Price int64`, `Capacity int32`, `MinAge pgtype.Int4`). Parts 2-3 reference these; adjust DTO mapping helpers to match.

- [ ] **Step 6: Commit**

```bash
git add database/queries services/api/internal/db
git commit -m "feat(db): add events and categories sqlc queries and regenerate"
```

---
