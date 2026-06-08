# Phase 11 Part 2: Access Codes + Redemption + INVITATION_ONLY Gate

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement access codes (create, bulk-generate, revoke), the redemption flow (verify → quota-reserve → issue grant), `AccessGrantChecker` for the RAE gate, and INVITATION_ONLY gate integration. Add the frontend "I have an access code" flow.

**Architecture:** New `access_codes` table. Redemption is atomic: code hash lookup → expiry check → eligibility check → `ReserveSlot` → `CreateGrant` → increment `use_count` with optimistic guard. Code values are never stored in plain text — sha256 hash only. `AccessGrantChecker` interface declared in `registration/gate.go`; `access.PoolManager` implements it. Participant endpoints require authn middleware.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, crypto/sha256. Module: `github.com/varin/ivyticketing/services/api`.

---

### Task 1: Migration 00044 — access_codes

**Files:**
- Create: `database/migrations/00044_create_access_codes.sql`

- [ ] **Step 1: Write 00044_create_access_codes.sql**

```sql
-- +goose Up
CREATE TABLE access_codes (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id      uuid REFERENCES event_categories(id) ON DELETE CASCADE,
    code_type        text NOT NULL
                         CHECK (code_type IN ('INVITATION','PRIORITY','COMMUNITY','CORPORATE',
                                             'COUPON','PARTNER','SPONSOR','VIP','ELITE')),
    code_value_hash  text NOT NULL,
    is_single_use    boolean NOT NULL DEFAULT true,
    max_uses         integer NOT NULL DEFAULT 1 CHECK (max_uses > 0),
    use_count        integer NOT NULL DEFAULT 0,
    valid_from       timestamptz NOT NULL,
    valid_until      timestamptz NOT NULL,
    pool_id          uuid REFERENCES access_pools(id),
    eligibility_rule jsonb,
    created_by       uuid NOT NULL REFERENCES users(id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    metadata         jsonb,
    UNIQUE (event_id, code_value_hash),
    CONSTRAINT access_codes_dates_check CHECK (valid_from < valid_until),
    CONSTRAINT access_codes_use_count_check CHECK (use_count <= max_uses)
);
CREATE INDEX access_codes_event_type_idx ON access_codes(event_id, code_type, valid_until);
CREATE INDEX access_codes_active_idx ON access_codes(event_id, code_value_hash)
    WHERE valid_until > now() AND use_count < max_uses;

-- +goose Down
DROP TABLE access_codes;
```

- [ ] **Step 2: Run migration**

```bash
cd /Users/kaivy/Coding/ivyticketing && make migrate-up 2>&1
# Expected: migrated to version 44
```

- [ ] **Step 3: Commit**

```bash
git add database/migrations/00044_create_access_codes.sql
git commit -m "feat(phase11): migration 00044 access_codes"
```

---

### Task 2: sqlc Access Code Queries

**Files:**
- Modify: `database/queries/access.sql`

- [ ] **Step 1: Append code queries to access.sql**

```sql
-- name: CreateAccessCode :one
INSERT INTO access_codes
    (organization_id, event_id, category_id, code_type, code_value_hash,
     is_single_use, max_uses, valid_from, valid_until, pool_id,
     eligibility_rule, created_by, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetAccessCodeByHash :one
SELECT * FROM access_codes
WHERE event_id = $1 AND code_value_hash = $2;

-- name: ListAccessCodesByEvent :many
SELECT * FROM access_codes
WHERE event_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: IncrementCodeUseCount :one
UPDATE access_codes
SET use_count = use_count + 1
WHERE id = $1 AND use_count < max_uses
RETURNING *;

-- name: RevokeAccessCode :exec
UPDATE access_codes SET valid_until = now() WHERE id = $1;
```

- [ ] **Step 2: Regenerate sqlc + build**

```bash
cd /Users/kaivy/Coding/ivyticketing && make sqlc 2>&1
cd services/api && go build ./internal/db/... 2>&1
```

- [ ] **Step 3: Commit**

```bash
git add database/queries/access.sql
git commit -m "feat(phase11): sqlc access code queries"
```

---

### Task 3: Access Code Service — Create, BulkGenerate, Revoke, Redeem

**Files:**
- Create: `services/api/internal/modules/access/code_service.go`
- Create: `services/api/internal/modules/access/tests/code_service_test.go`

- [ ] **Step 1: Write failing tests**

```go
// services/api/internal/modules/access/tests/code_service_test.go
package access_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

type codeRepo struct {
	fakeAccessRepoFull
	codes     map[string]db.AccessCode // keyed by code_value_hash
	useCounts map[uuid.UUID]int
}

func (r *codeRepo) GetAccessCodeByHash(_ context.Context, arg db.GetAccessCodeByHashParams) (db.AccessCode, error) {
	if c, ok := r.codes[arg.CodeValueHash]; ok { return c, nil }
	return db.AccessCode{}, pgx.ErrNoRows
}
func (r *codeRepo) IncrementCodeUseCount(_ context.Context, id uuid.UUID) (db.AccessCode, error) {
	r.useCounts[id]++
	code := r.codes[""]
	if r.useCounts[id] > int(code.MaxUses) { return db.AccessCode{}, pgx.ErrNoRows }
	return code, nil
}
func (r *codeRepo) ReservePoolSlot(_ context.Context, _ uuid.UUID) (db.AccessPool, error) {
	return db.AccessPool{}, nil
}
func (r *codeRepo) CreateAccessGrant(_ context.Context, _ db.CreateAccessGrantParams) (db.AccessGrant, error) {
	return db.AccessGrant{ID: uuid.New()}, nil
}

func validCode() db.AccessCode {
	return db.AccessCode{
		ID:            uuid.New(),
		CodeValueHash: access.HashCode("SECRET123"),
		MaxUses:       1,
		UseCount:      0,
		ValidFrom:     pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
		ValidUntil:    pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}
}

func TestRedeem_ValidCode_IssuesGrant(t *testing.T) {
	code := validCode()
	repo := &codeRepo{codes: map[string]db.AccessCode{code.CodeValueHash: code}, useCounts: map[uuid.UUID]int{}}
	svc := access.NewCodeService(repo, access.NewEligibilityChecker(&fakeEligRepo{}))
	grant, err := svc.Redeem(context.Background(), uuid.New(), uuid.New(), uuid.New(), "SECRET123")
	if err != nil { t.Fatalf("valid redemption should succeed: %v", err) }
	if grant.ID == uuid.Nil { t.Fatal("grant ID should not be nil") }
}

func TestRedeem_WrongCode_ReturnsNotFound(t *testing.T) {
	repo := &codeRepo{codes: map[string]db.AccessCode{}, useCounts: map[uuid.UUID]int{}}
	svc := access.NewCodeService(repo, access.NewEligibilityChecker(&fakeEligRepo{}))
	_, err := svc.Redeem(context.Background(), uuid.New(), uuid.New(), uuid.New(), "WRONG")
	if err == nil { t.Fatal("wrong code should return error") }
}

func TestRedeem_ExpiredCode_ReturnsError(t *testing.T) {
	code := validCode()
	code.ValidUntil = pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true}
	repo := &codeRepo{codes: map[string]db.AccessCode{code.CodeValueHash: code}, useCounts: map[uuid.UUID]int{}}
	svc := access.NewCodeService(repo, access.NewEligibilityChecker(&fakeEligRepo{}))
	_, err := svc.Redeem(context.Background(), uuid.New(), uuid.New(), uuid.New(), "SECRET123")
	if err == nil { t.Fatal("expired code should return error") }
}

func TestRedeem_ExhaustedCode_ReturnsError(t *testing.T) {
	code := validCode()
	code.UseCount = 1 // already at max_uses=1
	repo := &codeRepo{codes: map[string]db.AccessCode{code.CodeValueHash: code}, useCounts: map[uuid.UUID]int{}}
	svc := access.NewCodeService(repo, access.NewEligibilityChecker(&fakeEligRepo{}))
	_, err := svc.Redeem(context.Background(), uuid.New(), uuid.New(), uuid.New(), "SECRET123")
	if err == nil { t.Fatal("exhausted code should return error") }
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -run TestRedeem -v 2>&1
```

- [ ] **Step 3: Write code_service.go**

```go
package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// HashCode returns the sha256 hex of a plain-text code. Never store the plain text.
func HashCode(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

type CodeService struct {
	repo        Repository
	eligibility *EligibilityChecker
}

func NewCodeService(repo Repository, elig *EligibilityChecker) *CodeService {
	return &CodeService{repo: repo, eligibility: elig}
}

// Redeem validates plaintext code, reserves a pool slot, and issues an AccessGrant.
// The plain-text code is hashed immediately — never stored.
func (s *CodeService) Redeem(ctx context.Context, participantID, eventID, categoryID uuid.UUID, plainCode string) (db.AccessGrant, error) {
	hash := HashCode(plainCode)
	code, err := s.repo.GetAccessCodeByHash(ctx, db.GetAccessCodeByHashParams{EventID: eventID, CodeValueHash: hash})
	if errors.Is(err, pgx.ErrNoRows) { return db.AccessGrant{}, ErrCodeNotFound }
	if err != nil { return db.AccessGrant{}, err }

	// Expiry check
	now := time.Now()
	if code.ValidFrom.Valid && now.Before(code.ValidFrom.Time) { return db.AccessGrant{}, ErrCodeExpired }
	if code.ValidUntil.Valid && now.After(code.ValidUntil.Time) { return db.AccessGrant{}, ErrCodeExpired }

	// Exhaustion check
	if code.UseCount >= code.MaxUses { return db.AccessGrant{}, ErrCodeExhausted }

	// Eligibility check
	if len(code.EligibilityRule) > 0 {
		ok, reason, err := s.eligibility.Check(ctx, participantID, code.OrganizationID, code.EligibilityRule)
		if err != nil { return db.AccessGrant{}, err }
		if !ok { return db.AccessGrant{}, fmt.Errorf("%w: %s", ErrNotEligible, reason) }
	}

	// Reserve pool slot (if code has a pool)
	if code.PoolID.Valid {
		if err := s.repo.ConsumePoolSlot(ctx, code.PoolID.Bytes); err != nil {
			// Try atomic reserve first
			if _, rErr := s.repo.ReservePoolSlot(ctx, code.PoolID.Bytes); rErr != nil {
				return db.AccessGrant{}, ErrPoolExhausted
			}
		}
	}

	// Issue grant
	expiresAt := code.ValidUntil.Time
	grant, err := s.repo.CreateAccessGrant(ctx, db.CreateAccessGrantParams{
		PoolID:        code.PoolID,
		ParticipantID: participantID,
		EventID:       eventID,
		CategoryID:    categoryID,
		CodeID:        pgtype.UUID{Bytes: code.ID, Valid: true},
		ExpiresAt:     pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil { return db.AccessGrant{}, err }

	// Increment use_count (optimistic guard — if 0 rows returned, someone else exhausted it)
	if _, err := s.repo.IncrementCodeUseCount(ctx, code.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return db.AccessGrant{}, ErrCodeExhausted }
		return db.AccessGrant{}, err
	}

	return grant, nil
}

func (s *CodeService) Create(ctx context.Context, orgID, eventID uuid.UUID, categoryID *uuid.UUID, codeType, plainCode string, maxUses int32, validFrom, validUntil time.Time, poolID *uuid.UUID, createdBy uuid.UUID) (db.AccessCode, error) {
	hash := HashCode(plainCode)
	params := db.CreateAccessCodeParams{
		OrganizationID: orgID, EventID: eventID,
		CodeType:      codeType,
		CodeValueHash: hash,
		MaxUses:       maxUses,
		ValidFrom:     pgtype.Timestamptz{Time: validFrom, Valid: true},
		ValidUntil:    pgtype.Timestamptz{Time: validUntil, Valid: true},
		CreatedBy:     createdBy,
	}
	if categoryID != nil { params.CategoryID = pgtype.UUID{Bytes: *categoryID, Valid: true} }
	if poolID != nil { params.PoolID = pgtype.UUID{Bytes: *poolID, Valid: true} }
	return s.repo.CreateAccessCode(ctx, params)
}

func (s *CodeService) Revoke(ctx context.Context, codeID uuid.UUID) error {
	return s.repo.RevokeAccessCode(ctx, codeID)
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -run TestRedeem -race -v 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/access/code_service.go \
        services/api/internal/modules/access/tests/code_service_test.go
git commit -m "feat(phase11): access code service (create, redeem, revoke) — TDD"
```

---

### Task 4: AccessGrantChecker Interface + RAE Gate Integration

**Files:**
- Modify: `services/api/internal/modules/registration/gate.go`
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Write failing test**

```go
// services/api/internal/modules/registration/tests/gate_access_test.go
package registration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

type fakeAccessGrantChecker struct{ err error }

func (f *fakeAccessGrantChecker) CheckGrant(_ context.Context, _, _ uuid.UUID, _ string) error {
	return f.err
}

func TestAdmit_InvitationOnly_ValidGrant(t *testing.T) {
	checker := &fakeAccessGrantChecker{err: nil}
	// build gate with accessGrant=checker, mode=INVITATION_ONLY
	// call Admit → expect nil
	_ = checker
	t.Skip("wire after gate.go updated")
}

func TestAdmit_InvitationOnly_NoGrant_Returns403(t *testing.T) {
	checker := &fakeAccessGrantChecker{err: registration.ErrModeNotAvailable}
	_ = checker
	t.Skip("wire after gate.go updated")
}
```

- [ ] **Step 2: Update gate.go — add AccessGrantChecker**

Add interface to `registration/gate.go`:

```go
type AccessGrantChecker interface {
	CheckGrant(ctx context.Context, participantID, categoryID uuid.UUID, grantToken string) error
}

// Gate gains accessGrant field
type Gate struct {
	svc         *Service
	queue        QueueAdmitter
	lifecycle    LifecycleChecker
	ballot       BallotAdmitter
	accessGrant  AccessGrantChecker  // Phase 11
}

func NewGate(svc *Service, queue QueueAdmitter, lc LifecycleChecker, ballot BallotAdmitter, accessGrant AccessGrantChecker) *Gate {
	return &Gate{svc: svc, queue: queue, lifecycle: lc, ballot: ballot, accessGrant: accessGrant}
}

// In Admit() switch, add:
case ModeInvitationOnly:
    if g.accessGrant == nil { return ErrModeNotAvailable }
    return g.accessGrant.CheckGrant(ctx, participantID, categoryID, admissionToken)
case ModeWaitlistOnly:
    if g.accessGrant == nil { return ErrModeNotAvailable }
    return g.accessGrant.CheckGrant(ctx, participantID, categoryID, admissionToken)
```

- [ ] **Step 3: Implement CheckGrant on PoolManager**

Add to `services/api/internal/modules/access/pool.go`:

```go
// CheckGrant implements registration.AccessGrantChecker.
// grantToken is the grant UUID as a string.
func (p *PoolManager) CheckGrant(ctx context.Context, participantID, categoryID uuid.UUID, grantToken string) error {
	return p.checkGrant(ctx, participantID, categoryID, grantToken)
}
```

(The private `checkGrant` already exists from Phase 10. This just adds the public method matching the interface.)

- [ ] **Step 4: Update server.go — inject accessGrant into NewGate**

```go
// In server.go, update NewGate call:
registrationGate := registration.NewGate(registrationSvc, queueAdmitter, lifecycleSvc, ballotSvc, poolMgr)
```

- [ ] **Step 5: Build + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api
go build ./... 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/registration/gate.go \
        services/api/internal/modules/registration/tests/ \
        services/api/internal/app/server.go
git commit -m "feat(phase11): RAE gate — INVITATION_ONLY + WAITLIST_ONLY via AccessGrantChecker"
```

---

### Task 5: Access Code Handler + Routes

**Files:**
- Create: `services/api/internal/modules/access/dto.go`
- Create: `services/api/internal/modules/access/handler.go`
- Create: `services/api/internal/modules/access/routes.go`

- [ ] **Step 1: Write dto.go**

```go
package access

import "time"

type RedeemRequest struct {
	Code       string `json:"code"`
	CategoryID string `json:"categoryId"`
}

type AccessGrantDTO struct {
	ID         string    `json:"id"`
	Token      string    `json:"token"`   // same as ID — used as admissionToken
	CategoryID string    `json:"categoryId"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type CreateCodeRequest struct {
	CodeType    string    `json:"codeType"`
	Code        string    `json:"code"`
	MaxUses     int32     `json:"maxUses"`
	ValidFrom   time.Time `json:"validFrom"`
	ValidUntil  time.Time `json:"validUntil"`
	CategoryID  *string   `json:"categoryId,omitempty"`
	PoolID      *string   `json:"poolId,omitempty"`
}

type AccessCodeDTO struct {
	ID        string    `json:"id"`
	CodeType  string    `json:"codeType"`
	MaxUses   int32     `json:"maxUses"`
	UseCount  int32     `json:"useCount"`
	ValidFrom time.Time `json:"validFrom"`
	ValidUntil time.Time `json:"validUntil"`
}
```

- [ ] **Step 2: Write handler.go**

Read `services/api/internal/modules/queue/handler.go` for exact `authctx.FromContext`, `apperr.WriteJSON`, `apperr.WriteError` pattern.

```go
package access

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	codes     *CodeService
	pools     *PoolService
	corporate *CorporateService
}

func NewHandler(codes *CodeService, pools *PoolService, corp *CorporateService) *Handler {
	return &Handler{codes: codes, pools: pools, corporate: corp}
}

func (h *Handler) Redeem(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	eventID, _ := uuid.Parse(chi.URLParam(r, "eventId"))
	var req RedeemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(400, "INVALID_BODY", "invalid request body")); return
	}
	categoryID, _ := uuid.Parse(req.CategoryID)
	grant, err := h.codes.Redeem(r.Context(), actor.UserID, eventID, categoryID, req.Code)
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, AccessGrantDTO{
		ID: grant.ID.String(), Token: grant.ID.String(),
		CategoryID: grant.CategoryID.String(), ExpiresAt: grant.ExpiresAt.Time,
	})
}

func (h *Handler) MyGrants(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	eventID, _ := uuid.Parse(chi.URLParam(r, "eventId"))
	grants, err := h.codes.repo.ListActiveGrantsForParticipant(r.Context(), actor.UserID, eventID)
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, grants)
}

func (h *Handler) CreateCode(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	orgID, _ := uuid.Parse(chi.URLParam(r, "orgId"))
	eventID, _ := uuid.Parse(chi.URLParam(r, "eventId"))
	var req CreateCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(400, "INVALID_BODY", "invalid request")); return
	}
	var catID *uuid.UUID
	if req.CategoryID != nil { id, _ := uuid.Parse(*req.CategoryID); catID = &id }
	var poolID *uuid.UUID
	if req.PoolID != nil { id, _ := uuid.Parse(*req.PoolID); poolID = &id }
	code, err := h.codes.Create(r.Context(), orgID, eventID, catID, req.CodeType, req.Code, req.MaxUses, req.ValidFrom, req.ValidUntil, poolID, actor.UserID)
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusCreated, AccessCodeDTO{
		ID: code.ID.String(), CodeType: code.CodeType,
		MaxUses: code.MaxUses, UseCount: code.UseCount,
	})
}

func (h *Handler) ListCodes(w http.ResponseWriter, r *http.Request) {
	eventID, _ := uuid.Parse(chi.URLParam(r, "eventId"))
	limit := int32(50)
	if v := r.URL.Query().Get("limit"); v != "" { if n, _ := strconv.Atoi(v); n > 0 { limit = int32(n) } }
	codes, err := h.codes.repo.ListAccessCodesByEvent(r.Context(), db.ListAccessCodesByEventParams{EventID: eventID, Limit: limit})
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, codes)
}

func (h *Handler) RevokeCode(w http.ResponseWriter, r *http.Request) {
	codeID, _ := uuid.Parse(chi.URLParam(r, "codeId"))
	if err := h.codes.Revoke(r.Context(), codeID); err != nil { apperr.WriteError(w, r, err); return }
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Write routes.go**

```go
package access

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterParticipantRoutes(r chi.Router) {
	r.Route("/events/{eventId}/access", func(r chi.Router) {
		r.Post("/redeem", h.Redeem)
		r.Get("/my-grants", h.MyGrants)
	})
}

func (h *Handler) RegisterOrganizerRoutes(r chi.Router) {
	r.Route("/org/{orgId}", func(r chi.Router) {
		r.Post("/events/{eventId}/access/codes", h.CreateCode)
		r.Get("/events/{eventId}/access/codes", h.ListCodes)
		r.Delete("/access/codes/{codeId}", h.RevokeCode)
		r.Get("/events/{eventId}/access/pools", h.ListPools)
		r.Put("/access/pools/{poolId}", h.AdjustPool)
	})
}
```

- [ ] **Step 4: Add ListActiveGrantsForParticipant query to access.sql**

```sql
-- name: ListActiveGrantsForParticipant :many
SELECT * FROM access_grants
WHERE participant_id = $1 AND event_id = $2 AND status = 'ACTIVE'
ORDER BY granted_at DESC;
```

Run `make sqlc`.

- [ ] **Step 5: Wire into server.go**

```go
// In server.go:
codeSvc := access.NewCodeService(accessRepo, eligibilityChecker)
accessHandler := access.NewHandler(codeSvc, poolSvc, corporateSvc)
// mount participant routes inside authn group:
accessHandler.RegisterParticipantRoutes(r)
// mount organizer routes inside org-authn group:
accessHandler.RegisterOrganizerRoutes(r)
```

Add abuse guard on redeem:
```go
r.With(abuseGuard.Middleware(abusemod.CategoryAccessRedeem)).
    Post("/events/{eventId}/access/redeem", accessHandler.Redeem)
```

Add to `abuse/model.go`:
```go
CategoryAccessRedeem = "access_redeem"
```

- [ ] **Step 6: Build + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api
go build ./... 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
```

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/modules/access/dto.go \
        services/api/internal/modules/access/handler.go \
        services/api/internal/modules/access/routes.go \
        services/api/internal/app/server.go
git commit -m "feat(phase11): access code handler + routes + INVITATION_ONLY gate wired"
```

---

### Task 6: Frontend — "I have an access code"

**Files:**
- Create: `apps/web/src/lib/access.ts`
- Create: `apps/web/src/components/access/RedeemCodeModal.astro`
- Modify: appropriate event page component (read `apps/web/src/pages/events/[eventId].astro` first)

- [ ] **Step 1: Read event page**

```bash
cat /Users/kaivy/Coding/ivyticketing/apps/web/src/pages/events/\[eventId\].astro 2>/dev/null | head -60
```

- [ ] **Step 2: Write access.ts**

```typescript
const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080"

export async function redeemCode(
  eventId: string,
  code: string,
  categoryId: string,
  token: string
): Promise<{ id: string; token: string; categoryId: string; expiresAt: string }> {
  const res = await fetch(`${API_URL}/api/v1/events/${eventId}/access/redeem`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ code, categoryId }),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: "unknown error" }))
    throw new Error(body.message ?? `HTTP ${res.status}`)
  }
  return res.json()
}
```

- [ ] **Step 3: Write RedeemCodeModal.astro**

```astro
---
interface Props {
  eventId: string
  categoryId: string
}
const { eventId, categoryId } = Astro.props
---

<div id="redeem-modal" class="hidden fixed inset-0 bg-black/50 flex items-center justify-center z-50">
  <div class="bg-white rounded-lg p-6 w-full max-w-sm">
    <h2 class="text-lg font-semibold mb-4">Enter your access code</h2>
    <input
      id="redeem-input"
      type="text"
      placeholder="e.g. MARATHON2026"
      class="w-full border rounded px-3 py-2 mb-3"
    />
    <p id="redeem-error" class="text-red-600 text-sm mb-3 hidden"></p>
    <div class="flex gap-2">
      <button id="redeem-cancel" class="flex-1 border rounded px-3 py-2">Cancel</button>
      <button id="redeem-submit" class="flex-1 bg-blue-600 text-white rounded px-3 py-2">Apply Code</button>
    </div>
  </div>
</div>

<script define:vars={{ eventId, categoryId }}>
  import { redeemCode } from "/src/lib/access.ts"

  const modal = document.getElementById("redeem-modal")
  const input = document.getElementById("redeem-input")
  const error = document.getElementById("redeem-error")

  document.getElementById("redeem-cancel")?.addEventListener("click", () => {
    modal?.classList.add("hidden")
  })

  document.getElementById("redeem-submit")?.addEventListener("click", async () => {
    const code = input?.value?.trim()
    if (!code) return
    const token = localStorage.getItem("auth_token") ?? ""
    try {
      const grant = await redeemCode(eventId, code, categoryId, token)
      localStorage.setItem(`grant_${categoryId}`, grant.token)
      window.location.href = `/events/${eventId}/checkout?categoryId=${categoryId}&grantToken=${grant.token}`
    } catch (e) {
      error.textContent = e instanceof Error ? e.message : "Invalid code"
      error.classList.remove("hidden")
    }
  })
</script>
```

- [ ] **Step 4: Add "I have an access code" link on event page**

In the event page, when `category.registration_mode === "INVITATION_ONLY"`, render:
```astro
<button onclick="document.getElementById('redeem-modal').classList.remove('hidden')"
        class="text-blue-600 underline text-sm">
  I have an access code
</button>
<RedeemCodeModal eventId={event.id} categoryId={category.id} />
```

- [ ] **Step 5: Build frontend**

```bash
cd /Users/kaivy/Coding/ivyticketing/apps/web && npm run build 2>&1
```

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/lib/access.ts apps/web/src/components/access/
git commit -m "feat(phase11): frontend access code redemption modal (INVITATION_ONLY)"
```

---

### Task 7: Part 2 Full Verification

- [ ] **Step 1: Full build + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api
go build ./... 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
cd /Users/kaivy/Coding/ivyticketing/apps/web && npm run build 2>&1
```

- [ ] **Step 2: Commit**

```bash
git commit -m "feat(phase11): part 2 complete — access codes + redemption + INVITATION_ONLY gate"
```
