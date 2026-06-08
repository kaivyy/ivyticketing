# Phase 8 Plan — Part 1: Registration Mode Foundation

> Part of the Phase 8 implementation plan. Index: [2026-06-08-phase8-queue-war-system.md](2026-06-08-phase8-queue-war-system.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** New files + additive changes only. NORMAL mode must stay identical to current Phase 5 checkout behavior.

This part builds the shared foundation reused by Phase 9-11: registration mode enum, per-event/category settings, a pure resolver, and a `RegistrationGate` seam in orders. No queue logic yet — queue modes return "not available" until Part 3 fills them.

---

## Task 1: Config — QUEUE_* settings

**Files:**
- Modify: `services/api/internal/app/config.go`
- Modify: `services/api/internal/app/config_test.go`
- Modify: `services/api/.env.example`, `.env.example`

- [ ] **Step 1: Write the failing test**

Add to `services/api/internal/app/config_test.go` (keep existing; remember every success-path test must already set `TICKET_QR_SECRET` from Phase 7 — set it here too):
```go
func TestLoadConfig_QueueDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("TICKET_QR_SECRET", "qr-secret")
	t.Setenv("QUEUE_RELEASE_INTERVAL", "")
	t.Setenv("QUEUE_CHECKOUT_WINDOW", "")
	t.Setenv("QUEUE_DEFAULT_RELEASE_RATE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.QueueReleaseInterval != 10*time.Second {
		t.Errorf("QueueReleaseInterval = %v, want 10s", cfg.QueueReleaseInterval)
	}
	if cfg.QueueCheckoutWindow != 5*time.Minute {
		t.Errorf("QueueCheckoutWindow = %v, want 5m", cfg.QueueCheckoutWindow)
	}
	if cfg.QueueDefaultReleaseRate != 100 {
		t.Errorf("QueueDefaultReleaseRate = %d, want 100", cfg.QueueDefaultReleaseRate)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig_QueueDefaults -v; cd ../..
```
Expected: FAIL — `cfg.QueueReleaseInterval` undefined.

- [ ] **Step 3: Add fields + loading**

In `config.go`, add to `Config` struct (near other durations):
```go
	QueueReleaseInterval    time.Duration
	QueueCheckoutWindow     time.Duration
	QueueDefaultReleaseRate int
```
In `LoadConfig`, before `return cfg, nil`:
```go
	qInterval, err := getDuration("QUEUE_RELEASE_INTERVAL", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	qWindow, err := getDuration("QUEUE_CHECKOUT_WINDOW", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	qRate, err := getInt("QUEUE_DEFAULT_RELEASE_RATE", 100)
	if err != nil {
		return Config{}, err
	}
	cfg.QueueReleaseInterval = qInterval
	cfg.QueueCheckoutWindow = qWindow
	cfg.QueueDefaultReleaseRate = qRate
```

> Check `config.go` for the existing int helper name. Phase 5 used `getInt64`/`getDuration`. If only `getInt64` exists, use it and make `QueueDefaultReleaseRate` an `int` via conversion, or add a small `getInt` helper mirroring `getInt64`. Verify before writing.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig -v; cd ../..
```
Expected: PASS (all config tests).

- [ ] **Step 5: Update .env.example files**

Append to both `services/api/.env.example` and root `.env.example`:
```
# Queue / war (Phase 8)
QUEUE_RELEASE_INTERVAL=10s
QUEUE_DEFAULT_RELEASE_RATE=100
QUEUE_CHECKOUT_WINDOW=5m
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/app/config.go services/api/internal/app/config_test.go services/api/.env.example .env.example
git commit -m "feat(phase8): add queue config (release interval, rate, checkout window)"
```

---

## Task 2: Migration — registration settings tables

**Files:**
- Create: `database/migrations/00020_create_registration_settings.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
CREATE TABLE event_registration_settings (
    event_id         uuid PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
    default_mode     text NOT NULL DEFAULT 'NORMAL',
    queue_enabled    boolean NOT NULL DEFAULT false,
    ballot_enabled   boolean NOT NULL DEFAULT false,
    priority_enabled boolean NOT NULL DEFAULT false,
    waitlist_enabled boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ers_mode_check CHECK (default_mode IN
        ('NORMAL','WAR_QUEUE','RANDOMIZED_QUEUE','HYBRID_QUEUE','BALLOT','INVITATION_ONLY','PRIORITY_ACCESS','WAITLIST_ONLY','CLOSED'))
);

CREATE TABLE category_registration_settings (
    category_id      uuid PRIMARY KEY REFERENCES event_categories(id) ON DELETE CASCADE,
    registration_mode text,
    override_enabled boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT crs_mode_check CHECK (registration_mode IS NULL OR registration_mode IN
        ('NORMAL','WAR_QUEUE','RANDOMIZED_QUEUE','HYBRID_QUEUE','BALLOT','INVITATION_ONLY','PRIORITY_ACCESS','WAITLIST_ONLY','CLOSED'))
);

-- +goose Down
DROP TABLE category_registration_settings;
DROP TABLE event_registration_settings;
```

- [ ] **Step 2: Roundtrip**

```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: clean up/down/up.

- [ ] **Step 3: Commit**

```bash
git add database/migrations/00020_create_registration_settings.sql
git commit -m "feat(phase8): registration settings migration"
```

---

## Task 3: sqlc queries for registration settings

**Files:**
- Create: `database/queries/registration.sql`
- Regenerate: `services/api/internal/db/*`

- [ ] **Step 1: Write queries**

Create `database/queries/registration.sql`:
```sql
-- name: GetEventRegistrationSettings :one
SELECT * FROM event_registration_settings WHERE event_id = $1;

-- name: UpsertEventRegistrationSettings :one
INSERT INTO event_registration_settings (event_id, default_mode, queue_enabled, ballot_enabled, priority_enabled, waitlist_enabled)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (event_id) DO UPDATE SET
    default_mode = EXCLUDED.default_mode,
    queue_enabled = EXCLUDED.queue_enabled,
    ballot_enabled = EXCLUDED.ballot_enabled,
    priority_enabled = EXCLUDED.priority_enabled,
    waitlist_enabled = EXCLUDED.waitlist_enabled,
    updated_at = now()
RETURNING *;

-- name: GetCategoryRegistrationSettings :one
SELECT * FROM category_registration_settings WHERE category_id = $1;

-- name: UpsertCategoryRegistrationSettings :one
INSERT INTO category_registration_settings (category_id, registration_mode, override_enabled)
VALUES ($1,$2,$3)
ON CONFLICT (category_id) DO UPDATE SET
    registration_mode = EXCLUDED.registration_mode,
    override_enabled = EXCLUDED.override_enabled,
    updated_at = now()
RETURNING *;

-- name: ListCategoryRegistrationSettingsByEvent :many
SELECT crs.* FROM category_registration_settings crs
JOIN event_categories ec ON ec.id = crs.category_id
WHERE ec.event_id = $1;
```

- [ ] **Step 2: Regenerate + build**

```bash
make sqlc && cd services/api && go build ./internal/db/...; cd ../..
```
Expected: `EventRegistrationSetting` + `CategoryRegistrationSetting` structs + methods generated; builds clean.

- [ ] **Step 3: Commit**

```bash
git add database/queries/registration.sql services/api/internal/db
git commit -m "feat(phase8): registration settings sqlc queries"
```

---

## Task 4: Registration model + resolver (pure)

**Files:**
- Create: `services/api/internal/modules/registration/model.go`
- Create: `services/api/internal/modules/registration/resolver.go`
- Create: `services/api/internal/modules/registration/resolver_test.go`

- [ ] **Step 1: Write the failing test**

Create `services/api/internal/modules/registration/resolver_test.go`:
```go
package registration

import "testing"

func TestResolveMode_DefaultNormal(t *testing.T) {
	// no event settings, no category settings → NORMAL
	if got := ResolveMode(ModeInput{}); got != ModeNormal {
		t.Fatalf("got %q, want NORMAL", got)
	}
}

func TestResolveMode_EventLevel(t *testing.T) {
	in := ModeInput{EventModeSet: true, EventMode: ModeWarQueue}
	if got := ResolveMode(in); got != ModeWarQueue {
		t.Fatalf("got %q, want WAR_QUEUE", got)
	}
}

func TestResolveMode_CategoryOverride(t *testing.T) {
	in := ModeInput{
		EventModeSet: true, EventMode: ModeWarQueue,
		CategoryOverride: true, CategoryModeSet: true, CategoryMode: ModeBallot,
	}
	if got := ResolveMode(in); got != ModeBallot {
		t.Fatalf("got %q, want BALLOT (category override)", got)
	}
}

func TestResolveMode_CategoryOverrideDisabled_UsesEvent(t *testing.T) {
	in := ModeInput{
		EventModeSet: true, EventMode: ModeWarQueue,
		CategoryOverride: false, CategoryModeSet: true, CategoryMode: ModeBallot,
	}
	if got := ResolveMode(in); got != ModeWarQueue {
		t.Fatalf("got %q, want WAR_QUEUE (override disabled)", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd services/api && go test ./internal/modules/registration/ -run TestResolveMode -v; cd ../..
```
Expected: FAIL — package/`ResolveMode` undefined.

- [ ] **Step 3: Implement model.go**

```go
package registration

// Mode is a registration access mode.
type Mode string

const (
	ModeNormal          Mode = "NORMAL"
	ModeWarQueue        Mode = "WAR_QUEUE"
	ModeRandomizedQueue Mode = "RANDOMIZED_QUEUE"
	ModeHybridQueue     Mode = "HYBRID_QUEUE"
	ModeBallot          Mode = "BALLOT"
	ModeInvitationOnly  Mode = "INVITATION_ONLY"
	ModePriorityAccess  Mode = "PRIORITY_ACCESS"
	ModeWaitlistOnly    Mode = "WAITLIST_ONLY"
	ModeClosed          Mode = "CLOSED"
)

// IsQueueMode reports whether the mode is queue-backed (Phase 8).
func IsQueueMode(m Mode) bool {
	return m == ModeWarQueue || m == ModeRandomizedQueue || m == ModeHybridQueue
}

// Valid reports whether m is a known mode.
func Valid(m Mode) bool {
	switch m {
	case ModeNormal, ModeWarQueue, ModeRandomizedQueue, ModeHybridQueue,
		ModeBallot, ModeInvitationOnly, ModePriorityAccess, ModeWaitlistOnly, ModeClosed:
		return true
	}
	return false
}
```

- [ ] **Step 4: Implement resolver.go**

```go
package registration

// ModeInput carries resolved settings flags for a single (event, category).
type ModeInput struct {
	EventModeSet     bool
	EventMode        Mode
	CategoryOverride bool
	CategoryModeSet  bool
	CategoryMode     Mode
}

// ResolveMode applies the override rule: category overrides event when
// override_enabled and a category mode is set; otherwise event mode; default NORMAL.
func ResolveMode(in ModeInput) Mode {
	if in.CategoryOverride && in.CategoryModeSet && in.CategoryMode != "" {
		return in.CategoryMode
	}
	if in.EventModeSet && in.EventMode != "" {
		return in.EventMode
	}
	return ModeNormal
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd services/api && go test ./internal/modules/registration/ -run TestResolveMode -v; cd ../..
```
Expected: PASS (all 4).

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/registration/model.go services/api/internal/modules/registration/resolver.go services/api/internal/modules/registration/resolver_test.go
git commit -m "feat(phase8): registration mode enum + resolver"
```

---

## Task 5: Registration repository + settings service

**Files:**
- Create: `services/api/internal/modules/registration/repository.go`
- Create: `services/api/internal/modules/registration/service.go`
- Create: `services/api/internal/modules/registration/dto.go`
- Create: `services/api/internal/modules/registration/errors.go`

- [ ] **Step 1: Implement repository.go**

```go
package registration

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	GetEventSettings(ctx context.Context, eventID uuid.UUID) (db.EventRegistrationSetting, error)
	UpsertEventSettings(ctx context.Context, arg db.UpsertEventRegistrationSettingsParams) (db.EventRegistrationSetting, error)
	GetCategorySettings(ctx context.Context, categoryID uuid.UUID) (db.CategoryRegistrationSetting, error)
	UpsertCategorySettings(ctx context.Context, arg db.UpsertCategoryRegistrationSettingsParams) (db.CategoryRegistrationSetting, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) GetEventSettings(ctx context.Context, eventID uuid.UUID) (db.EventRegistrationSetting, error) {
	return r.q.GetEventRegistrationSettings(ctx, eventID)
}
func (r *sqlcRepo) UpsertEventSettings(ctx context.Context, arg db.UpsertEventRegistrationSettingsParams) (db.EventRegistrationSetting, error) {
	return r.q.UpsertEventRegistrationSettings(ctx, arg)
}
func (r *sqlcRepo) GetCategorySettings(ctx context.Context, categoryID uuid.UUID) (db.CategoryRegistrationSetting, error) {
	return r.q.GetCategoryRegistrationSettings(ctx, categoryID)
}
func (r *sqlcRepo) UpsertCategorySettings(ctx context.Context, arg db.UpsertCategoryRegistrationSettingsParams) (db.CategoryRegistrationSetting, error) {
	return r.q.UpsertCategoryRegistrationSettings(ctx, arg)
}
```

> Verify the exact generated param struct field names/types in `services/api/internal/db/registration.sql.go` before finalizing (especially nullable `registration_mode` → `pgtype.Text`).

- [ ] **Step 2: Implement errors.go**

```go
package registration

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrInvalidMode = apperr.New(http.StatusBadRequest, "INVALID_REGISTRATION_MODE", "unknown registration mode")
)
```

- [ ] **Step 3: Implement dto.go**

```go
package registration

type EventSettingsRequest struct {
	DefaultMode     string `json:"defaultMode"`
	QueueEnabled    bool   `json:"queueEnabled"`
	BallotEnabled   bool   `json:"ballotEnabled"`
	PriorityEnabled bool   `json:"priorityEnabled"`
	WaitlistEnabled bool   `json:"waitlistEnabled"`
}

type CategorySettingsRequest struct {
	CategoryID       string  `json:"categoryId"`
	RegistrationMode *string `json:"registrationMode"`
	OverrideEnabled  bool    `json:"overrideEnabled"`
}

type SettingsResponse struct {
	EventID         string                     `json:"eventId"`
	DefaultMode     string                     `json:"defaultMode"`
	QueueEnabled    bool                       `json:"queueEnabled"`
	BallotEnabled   bool                       `json:"ballotEnabled"`
	PriorityEnabled bool                       `json:"priorityEnabled"`
	WaitlistEnabled bool                       `json:"waitlistEnabled"`
	Categories      []CategorySettingsResponse `json:"categories"`
}

type CategorySettingsResponse struct {
	CategoryID       string  `json:"categoryId"`
	RegistrationMode *string `json:"registrationMode"`
	OverrideEnabled  bool    `json:"overrideEnabled"`
}
```

- [ ] **Step 4: Implement service.go**

```go
package registration

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ResolveForCheckout loads settings for (event, category) and returns the resolved mode.
// Missing rows → NORMAL (regression-safe).
func (s *Service) ResolveForCheckout(ctx context.Context, eventID, categoryID uuid.UUID) (Mode, error) {
	in := ModeInput{}
	ev, err := s.repo.GetEventSettings(ctx, eventID)
	if err == nil {
		in.EventModeSet = true
		in.EventMode = Mode(ev.DefaultMode)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ModeNormal, err
	}
	cat, err := s.repo.GetCategorySettings(ctx, categoryID)
	if err == nil {
		in.CategoryOverride = cat.OverrideEnabled
		if cat.RegistrationMode.Valid {
			in.CategoryModeSet = true
			in.CategoryMode = Mode(cat.RegistrationMode.String)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ModeNormal, err
	}
	return ResolveMode(in), nil
}

func (s *Service) SetEventSettings(ctx context.Context, eventID uuid.UUID, req EventSettingsRequest) error {
	if !Valid(Mode(req.DefaultMode)) {
		return ErrInvalidMode
	}
	_, err := s.repo.UpsertEventSettings(ctx, db.UpsertEventRegistrationSettingsParams{
		EventID: eventID, DefaultMode: req.DefaultMode,
		QueueEnabled: req.QueueEnabled, BallotEnabled: req.BallotEnabled,
		PriorityEnabled: req.PriorityEnabled, WaitlistEnabled: req.WaitlistEnabled,
	})
	return err
}

func (s *Service) SetCategorySettings(ctx context.Context, categoryID uuid.UUID, req CategorySettingsRequest) error {
	var mode pgtype.Text
	if req.RegistrationMode != nil {
		if !Valid(Mode(*req.RegistrationMode)) {
			return ErrInvalidMode
		}
		mode = pgtype.Text{String: *req.RegistrationMode, Valid: true}
	}
	_, err := s.repo.UpsertCategorySettings(ctx, db.UpsertCategoryRegistrationSettingsParams{
		CategoryID: categoryID, RegistrationMode: mode, OverrideEnabled: req.OverrideEnabled,
	})
	return err
}
```

> Verify generated param struct names match (`UpsertEventRegistrationSettingsParams`, `UpsertCategoryRegistrationSettingsParams`) and nullable mode field type (`pgtype.Text`). Adjust to actual generated code.

- [ ] **Step 5: Build**

```bash
cd services/api && go build ./internal/modules/registration/...; cd ../..
```
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/registration/repository.go services/api/internal/modules/registration/service.go services/api/internal/modules/registration/dto.go services/api/internal/modules/registration/errors.go
git commit -m "feat(phase8): registration settings repository + service"
```

---

## Task 6: RegistrationGate interface in orders + gate wiring

**Files:**
- Create: `services/api/internal/modules/orders/gate.go`
- Modify: `services/api/internal/modules/orders/service.go`
- Modify: `services/api/internal/modules/orders/validator.go`
- Create: `services/api/internal/modules/registration/gate.go`

This is the seam. NORMAL/CLOSED handled now; queue modes return a typed "admission required" that Part 3 fills.

- [ ] **Step 1: Declare the gate interface in orders**

Create `services/api/internal/modules/orders/gate.go`:
```go
package orders

import (
	"context"

	"github.com/google/uuid"
)

// RegistrationGate decides whether a participant may proceed to checkout for a
// given event/category, given an optional admission token (queue modes).
// Implemented by the registration module (dependency inversion; orders does not
// import registration/queue).
type RegistrationGate interface {
	Admit(ctx context.Context, participantID, eventID, categoryID uuid.UUID, admissionToken string) error
}

// noopGate permits everything — used when no gate is wired (preserves NORMAL behavior in tests).
type noopGate struct{}

func (noopGate) Admit(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) error { return nil }
```

- [ ] **Step 2: Wire gate into Service**

In `service.go`, add `gate RegistrationGate` to `Service` struct and constructor. Update `NewService`:
```go
type Service struct {
	repo  Repository
	audit AuditRecorder
	ttl   time.Duration
	gate  RegistrationGate
}

func NewService(repo Repository, recorder AuditRecorder, ttl time.Duration, gate RegistrationGate) *Service {
	if gate == nil {
		gate = noopGate{}
	}
	return &Service{repo: repo, audit: recorder, ttl: ttl, gate: gate}
}
```
Update `Checkout` signature to accept the admission token and call the gate after `checkoutEligible`:
```go
func (s *Service) Checkout(ctx context.Context, participantID, eventID, categoryID uuid.UUID, admissionToken string) (OrderResponse, error) {
```
Inside the `ExecTx`, right after the existing `checkoutEligible(event, cat, now)` block succeeds, add:
```go
		if err := s.gate.Admit(ctx, participantID, eventID, categoryID, admissionToken); err != nil {
			return err
		}
```

> The gate runs inside the tx but performs its own reads (queue admission lookup) — that is acceptable; it does not need the orders tx. It must NOT call back into orders. Keep it a plain method call.

- [ ] **Step 3: Update all Checkout callers**

```bash
cd services/api && grep -rn "\.Checkout(" . | grep -v _test; cd ../..
```
Update `orders/handler.go` `Checkout` to read the header and pass it:
```go
	admissionToken := r.Header.Get("X-Queue-Token")
	order, err := h.svc.Checkout(r.Context(), userID, eventID, categoryID, admissionToken)
```
Update `cmd/worker/main.go` if it constructs orders Service (it does — `NewService(...)` for `ExpireJob`). Pass `nil` gate there (worker never checks out):
```go
	svc := ordersmod.NewService(ordersmod.NewRepository(pg.Pool), auditLog, cfg.OrderExpiration, nil)
```
Update any orders tests calling `NewService` (add `nil`) and `Checkout` (add `""`).

- [ ] **Step 4: Implement registration gate**

Create `services/api/internal/modules/registration/gate.go`:
```go
package registration

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// QueueAdmitter is the queue module's admission check (filled in Part 3).
// Declared here so registration depends on an interface, not the concrete queue package.
type QueueAdmitter interface {
	CheckAdmission(ctx context.Context, participantID, eventID uuid.UUID, admissionToken string) error
}

// Gate implements orders.RegistrationGate.
type Gate struct {
	svc   *Service
	queue QueueAdmitter // may be nil until Part 3
}

func NewGate(svc *Service, queue QueueAdmitter) *Gate {
	return &Gate{svc: svc, queue: queue}
}

var (
	ErrModeNotAvailable = apperr.New(http.StatusConflict, "REGISTRATION_MODE_NOT_AVAILABLE", "this registration mode is not available yet")
	ErrClosed           = apperr.New(http.StatusConflict, "REGISTRATION_CLOSED", "registration is closed")
	ErrAdmissionReq     = apperr.New(http.StatusForbidden, "ADMISSION_REQUIRED", "queue admission required for checkout")
)

func (g *Gate) Admit(ctx context.Context, participantID, eventID, categoryID uuid.UUID, admissionToken string) error {
	mode, err := g.svc.ResolveForCheckout(ctx, eventID, categoryID)
	if err != nil {
		return err
	}
	switch mode {
	case ModeNormal:
		return nil // window already checked by orders.checkoutEligible
	case ModeClosed:
		return ErrClosed
	case ModeWarQueue, ModeRandomizedQueue, ModeHybridQueue:
		if g.queue == nil {
			return ErrModeNotAvailable
		}
		return g.queue.CheckAdmission(ctx, participantID, eventID, admissionToken)
	default:
		// BALLOT / INVITATION_ONLY / PRIORITY_ACCESS / WAITLIST_ONLY — Phase 10-11
		return ErrModeNotAvailable
	}
}
```

- [ ] **Step 5: Build**

```bash
cd services/api && go build ./...; cd ../..
```
Expected: clean (gate wired, queue nil for now).

- [ ] **Step 6: Run orders tests (regression)**

```bash
cd services/api && go test ./internal/modules/orders/... -race; cd ../..
```
Expected: PASS — NORMAL checkout unchanged (no settings rows → NORMAL → gate returns nil).

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/modules/orders services/api/internal/modules/registration/gate.go services/api/cmd/worker/main.go
git commit -m "feat(phase8): RegistrationGate seam in orders + registration gate (NORMAL/CLOSED)"
```

---

## Task 7: Registration handler/routes + server wiring + seed permissions

**Files:**
- Create: `services/api/internal/modules/registration/handler.go`, `routes.go`
- Create: `database/migrations/00021_seed_registration_permissions.sql`
- Modify: `services/api/internal/app/server.go`

**Migration numbering (authoritative across all Phase 8 parts — do NOT deviate):**
- `00020` settings (Part 1 Task 2)
- `00021` seed `registration.manage` (Part 1 Task 7, THIS task — `registration.manage` ONLY)
- `00022` queue_tokens, `00023` queue_admissions, `00024` queue_control (Part 2 Task 8)
- `00025` seed `queue.manage` (Part 3 Task 18)

This task seeds `registration.manage` ONLY. `queue.manage` is seeded separately in Part 3.

- [ ] **Step 1: Seed migration**

Create `database/migrations/00021_seed_registration_permissions.sql` (mirrors `00019` pattern):
```sql
-- +goose Up
INSERT INTO permissions (key, description) VALUES
    ('registration.manage', 'Manage registration mode & settings')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key = 'registration.manage'
WHERE r.organization_id IS NULL AND r.is_system = true AND r.slug IN ('owner','manager')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE key = 'registration.manage');
DELETE FROM permissions WHERE key = 'registration.manage';
```
Run roundtrip: `make migrate-up && make migrate-down && make migrate-up`.

- [ ] **Step 2: Implement handler.go**

```go
package registration

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) SetEventSettings(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	var req EventSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.SetEventSettings(r.Context(), eventID, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetCategorySettings(w http.ResponseWriter, r *http.Request) {
	var req CategorySettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	catID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	if err := h.svc.SetCategorySettings(r.Context(), catID, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Implement routes.go**

```go
package registration

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterEventRoutes mounts under /organizations/{orgId}/events/{eventId}.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "registration.manage")).
		Put("/registration", h.SetEventSettings)
	r.With(middleware.RequirePermission(loader, "registration.manage")).
		Put("/registration/category", h.SetCategorySettings)
}
```

> A GET endpoint to read settings is in the spec. Add `GetSettings` handler + `Get("/registration", h.GetSettings)` returning `SettingsResponse` (event + categories). Implement it mirroring the existing read patterns; reuse `ListCategoryRegistrationSettingsByEvent`. Include it in this task.

- [ ] **Step 4: Wire in server.go**

In `server.go`, build registration service+gate+handler and inject the gate into orders. Replace the orders construction line:
```go
	ordersHandler := ordersmod.NewHandler(ordersmod.NewService(ordersmod.NewRepository(pool), auditLog, cfg.OrderExpiration))
```
with:
```go
	registrationSvc := registrationmod.NewService(registrationmod.NewRepository(pool))
	registrationGate := registrationmod.NewGate(registrationSvc, nil) // queue admitter wired in Part 3
	registrationHandler := registrationmod.NewHandler(registrationSvc)
	ordersHandler := ordersmod.NewHandler(ordersmod.NewService(ordersmod.NewRepository(pool), auditLog, cfg.OrderExpiration, registrationGate))
```
Add import `registrationmod "github.com/varin/ivyticketing/services/api/internal/modules/registration"`. Mount routes inside the events route group (next to `ordersHandler.RegisterEventRoutes`):
```go
					registrationHandler.RegisterEventRoutes(r, loader)
```

- [ ] **Step 5: Build + full test**

```bash
cd services/api && go build ./... && go test ./internal/modules/registration/... ./internal/modules/orders/... -race; cd ../..
```
Expected: clean + green.

- [ ] **Step 6: Commit**

```bash
git add database/migrations/ services/api/internal/modules/registration services/api/internal/app/server.go
git commit -m "feat(phase8): registration settings endpoints + seed registration.manage + wiring"
```

---

Part 1 complete. NORMAL/CLOSED modes work; queue modes return REGISTRATION_MODE_NOT_AVAILABLE until Part 3. Next: [Part 2 — Queue Core + Waiting Room](2026-06-08-phase8-part2-queue-core.md).
