# Phase 11 Part 1: Access Engine Foundation — Full Pool Types + Access Pool Members

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the Phase 10 `access_pools` table with the remaining pool types and add `access_pool_members` and `corporate_accounts` tables. Wire the full `AccessPool` model (all 8 types), `EligibilityChecker` skeleton, and `CorporateAccount` domain — no participant-facing endpoints yet.

**Architecture:** Phase 10 already created `access_pools` with all pool_type values in the check constraint. Phase 11 Part 1 adds columns (`owner_account_id`, `is_visible_to_participants`, `eligibility_rule`) via ALTER, adds `access_pool_members` and `corporate_accounts` tables, and extends the `access` module with the full pool management service. All gated by `access_engine_enabled=false` platform setting.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose. Module: `github.com/varin/ivyticketing/services/api`.

---

### Task 1: Migrations 00041–00043

**Files:**
- Create: `database/migrations/00041_alter_access_pools_phase11.sql`
- Create: `database/migrations/00042_create_corporate_accounts.sql`
- Create: `database/migrations/00043_create_access_pool_members.sql`

- [ ] **Step 1: Write 00041_alter_access_pools_phase11.sql**

```sql
-- +goose Up
ALTER TABLE access_pools
    ADD COLUMN IF NOT EXISTS owner_account_id            uuid,
    ADD COLUMN IF NOT EXISTS is_visible_to_participants  boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS eligibility_rule            jsonb;

CREATE INDEX access_pools_owner_idx ON access_pools(owner_account_id)
    WHERE owner_account_id IS NOT NULL;

-- +goose Down
ALTER TABLE access_pools
    DROP COLUMN IF EXISTS owner_account_id,
    DROP COLUMN IF EXISTS is_visible_to_participants,
    DROP COLUMN IF EXISTS eligibility_rule;
DROP INDEX IF EXISTS access_pools_owner_idx;
```

- [ ] **Step 2: Write 00042_create_corporate_accounts.sql**

```sql
-- +goose Up
CREATE TABLE corporate_accounts (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name             text NOT NULL,
    billing_email    text NOT NULL,
    invoice_required boolean NOT NULL DEFAULT false,
    status           text NOT NULL DEFAULT 'PENDING'
                         CHECK (status IN ('PENDING','ACTIVE','SUSPENDED')),
    approved_at      timestamptz,
    approved_by      uuid REFERENCES users(id),
    created_by       uuid NOT NULL REFERENCES users(id),
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX corporate_accounts_org_idx ON corporate_accounts(organization_id, status);

-- +goose Down
DROP TABLE corporate_accounts;
```

- [ ] **Step 3: Write 00043_create_access_pool_members.sql**

```sql
-- +goose Up
CREATE TABLE access_pool_members (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id             uuid NOT NULL REFERENCES access_pools(id) ON DELETE CASCADE,
    user_id             uuid REFERENCES users(id),
    email               text NOT NULL,
    member_status       text NOT NULL DEFAULT 'PENDING'
                            CHECK (member_status IN ('PENDING','ACTIVE','REGISTERED','EXPIRED','REVOKED')),
    eligibility_meta    jsonb,
    access_grant_id     uuid REFERENCES access_grants(id),
    invited_at          timestamptz NOT NULL DEFAULT now(),
    registered_at       timestamptz,
    revoked_at          timestamptz,
    UNIQUE (pool_id, email)
);
CREATE INDEX access_pool_members_pool_status_idx ON access_pool_members(pool_id, member_status);
CREATE INDEX access_pool_members_user_idx ON access_pool_members(user_id) WHERE user_id IS NOT NULL;

-- +goose Down
DROP TABLE access_pool_members;
```

- [ ] **Step 4: Run migrations**

```bash
cd /Users/kaivy/Coding/ivyticketing
make migrate-up 2>&1
# Expected: migrated to version 43
make migrate-down 2>&1
make migrate-up 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add database/migrations/00041_alter_access_pools_phase11.sql \
        database/migrations/00042_create_corporate_accounts.sql \
        database/migrations/00043_create_access_pool_members.sql
git commit -m "feat(phase11): migrations 00041-00043 (pool columns, corporate accounts, pool members)"
```

---

### Task 2: sqlc Queries — Corporate + Pool Members

**Files:**
- Modify: `database/queries/access.sql` (add new queries)

- [ ] **Step 1: Append to access.sql**

```sql
-- name: CreateCorporateAccount :one
INSERT INTO corporate_accounts (organization_id, name, billing_email, invoice_required, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCorporateAccount :one
SELECT * FROM corporate_accounts WHERE id = $1;

-- name: ListCorporateAccounts :many
SELECT * FROM corporate_accounts WHERE organization_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: ApproveCorporateAccount :one
UPDATE corporate_accounts
SET status = 'ACTIVE', approved_at = now(), approved_by = $2
WHERE id = $1 AND status = 'PENDING'
RETURNING *;

-- name: AddPoolMember :one
INSERT INTO access_pool_members (pool_id, user_id, email, eligibility_meta)
VALUES ($1, $2, $3, $4)
ON CONFLICT (pool_id, email) DO NOTHING
RETURNING *;

-- name: BulkAddPoolMembers :copyfrom
INSERT INTO access_pool_members (pool_id, email, member_status)
VALUES (@pool_id, @email, 'PENDING');

-- name: ListPoolMembers :many
SELECT * FROM access_pool_members
WHERE pool_id = $1 AND member_status != 'REVOKED'
ORDER BY invited_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPoolMemberByEmail :one
SELECT * FROM access_pool_members WHERE pool_id = $1 AND email = $2;

-- name: UpdatePoolMemberStatus :one
UPDATE access_pool_members
SET member_status = $2,
    registered_at = CASE WHEN $2 = 'REGISTERED' THEN now() ELSE registered_at END,
    revoked_at    = CASE WHEN $2 = 'REVOKED' THEN now() ELSE revoked_at END,
    access_grant_id = COALESCE($3, access_grant_id)
WHERE id = $1
RETURNING *;

-- name: UpdateAccessPoolColumns :one
UPDATE access_pools
SET is_visible_to_participants = COALESCE($2, is_visible_to_participants),
    eligibility_rule = COALESCE($3, eligibility_rule),
    owner_account_id = COALESCE($4, owner_account_id)
WHERE id = $1
RETURNING *;

-- name: ListVisiblePoolsByCategory :many
SELECT * FROM access_pools
WHERE event_id = $1 AND category_id = $2
  AND is_visible_to_participants = true
  AND (valid_until IS NULL OR valid_until > now());

-- name: TransferPoolSlots :one
UPDATE access_pools SET total_slots = total_slots + $2 WHERE id = $1
  AND $2 > 0 OR (total_slots + $2 >= reserved_slots + used_slots)
RETURNING *;
```

- [ ] **Step 2: Run sqlc + build**

```bash
cd /Users/kaivy/Coding/ivyticketing && make sqlc 2>&1
cd services/api && go build ./internal/db/... 2>&1
```

- [ ] **Step 3: Commit**

```bash
git add database/queries/access.sql
git commit -m "feat(phase11): sqlc queries for corporate accounts + pool members"
```

---

### Task 3: Extend access/repository.go + access/model.go

**Files:**
- Modify: `services/api/internal/modules/access/repository.go`
- Modify: `services/api/internal/modules/access/model.go`
- Modify: `services/api/internal/modules/access/errors.go`

- [ ] **Step 1: Add to model.go**

```go
// Corporate account statuses
const (
	CorporateStatusPending   = "PENDING"
	CorporateStatusActive    = "ACTIVE"
	CorporateStatusSuspended = "SUSPENDED"
)

// Pool member statuses
const (
	MemberStatusPending    = "PENDING"
	MemberStatusActive     = "ACTIVE"
	MemberStatusRegistered = "REGISTERED"
	MemberStatusExpired    = "EXPIRED"
	MemberStatusRevoked    = "REVOKED"
)
```

- [ ] **Step 2: Add to errors.go**

```go
var (
	ErrCorporateNotFound    = apperr.New(http.StatusNotFound, "CORPORATE_NOT_FOUND", "corporate account not found")
	ErrCorporateNotApproved = apperr.New(http.StatusForbidden, "CORPORATE_NOT_APPROVED", "corporate account not yet approved")
	ErrMemberNotInPool      = apperr.New(http.StatusForbidden, "MEMBER_NOT_IN_POOL", "not a member of this access pool")
	ErrPoolTransferInsufficient = apperr.New(http.StatusConflict, "POOL_TRANSFER_INSUFFICIENT", "source pool has insufficient available slots")
)
```

- [ ] **Step 3: Add new methods to Repository interface and sqlcRepo**

Add to the `Repository` interface and implement in `sqlcRepo`:
```go
// Corporate
CreateCorporateAccount(ctx context.Context, arg db.CreateCorporateAccountParams) (db.CorporateAccount, error)
GetCorporateAccount(ctx context.Context, id uuid.UUID) (db.CorporateAccount, error)
ListCorporateAccounts(ctx context.Context, arg db.ListCorporateAccountsParams) ([]db.CorporateAccount, error)
ApproveCorporateAccount(ctx context.Context, arg db.ApproveCorporateAccountParams) (db.CorporateAccount, error)

// Pool members
AddPoolMember(ctx context.Context, arg db.AddPoolMemberParams) (db.AccessPoolMember, error)
ListPoolMembers(ctx context.Context, arg db.ListPoolMembersParams) ([]db.AccessPoolMember, error)
GetPoolMemberByEmail(ctx context.Context, arg db.GetPoolMemberByEmailParams) (db.AccessPoolMember, error)
UpdatePoolMemberStatus(ctx context.Context, arg db.UpdatePoolMemberStatusParams) (db.AccessPoolMember, error)
UpdateAccessPoolColumns(ctx context.Context, arg db.UpdateAccessPoolColumnsParams) (db.AccessPool, error)
ListVisiblePoolsByCategory(ctx context.Context, arg db.ListVisiblePoolsByCategoryParams) ([]db.AccessPool, error)
```

- [ ] **Step 4: Build**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./internal/modules/access/... 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/access/
git commit -m "feat(phase11): access module extended (corporate, pool members, pool columns)"
```

---

### Task 4: EligibilityChecker

**Files:**
- Create: `services/api/internal/modules/access/eligibility.go`
- Create: `services/api/internal/modules/access/tests/eligibility_test.go`

- [ ] **Step 1: Write failing tests**

```go
// services/api/internal/modules/access/tests/eligibility_test.go
package access_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

type fakeEligRepo struct {
	orderCount   int64
	membershipID string
}

func (r *fakeEligRepo) CountPaidOrdersByUserInOrg(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return r.orderCount, nil
}
func (r *fakeEligRepo) GetUserMembershipID(_ context.Context, _ uuid.UUID) (string, error) {
	return r.membershipID, nil
}
func (r *fakeEligRepo) HasPaidOrderForEvent(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return r.orderCount > 0, nil
}

func rule(t *testing.T, v map[string]any) json.RawMessage {
	t.Helper()
	b, _ := json.Marshal(v)
	return b
}

func TestEligibility_ReturningRunner(t *testing.T) {
	checker := access.NewEligibilityChecker(&fakeEligRepo{orderCount: 1})
	ok, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"returning_runner": true}))
	if !ok { t.Fatal("user with 1 paid order should pass returning_runner") }

	checker2 := access.NewEligibilityChecker(&fakeEligRepo{orderCount: 0})
	ok2, _, _ := checker2.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"returning_runner": true}))
	if ok2 { t.Fatal("user with 0 orders should fail returning_runner") }
}

func TestEligibility_MinCompletions(t *testing.T) {
	checker := access.NewEligibilityChecker(&fakeEligRepo{orderCount: 3})
	ok, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"min_completions": 3}))
	if !ok { t.Fatal("3 completions should pass min_completions=3") }

	ok2, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"min_completions": 4}))
	if ok2 { t.Fatal("3 completions should fail min_completions=4") }
}

func TestEligibility_MembershipIDPrefix(t *testing.T) {
	checker := access.NewEligibilityChecker(&fakeEligRepo{membershipID: "MEM-12345"})
	ok, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"membership_id_prefix": "MEM"}))
	if !ok { t.Fatal("MEM-12345 should pass prefix MEM") }

	ok2, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"membership_id_prefix": "VIP"}))
	if ok2 { t.Fatal("MEM-12345 should fail prefix VIP") }
}

func TestEligibility_UnknownKeyIgnored(t *testing.T) {
	checker := access.NewEligibilityChecker(&fakeEligRepo{})
	ok, _, _ := checker.Check(context.Background(), uuid.New(), uuid.Nil, rule(t, map[string]any{"future_rule_v2": true}))
	if !ok { t.Fatal("unknown rule keys should be ignored (pass)") }
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -run TestEligibility -v 2>&1
```

- [ ] **Step 3: Write eligibility.go**

```go
package access

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

type EligibilityRepo interface {
	CountPaidOrdersByUserInOrg(ctx context.Context, userID, orgID uuid.UUID) (int64, error)
	GetUserMembershipID(ctx context.Context, userID uuid.UUID) (string, error)
	HasPaidOrderForEvent(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
}

type EligibilityChecker struct{ repo EligibilityRepo }

func NewEligibilityChecker(repo EligibilityRepo) *EligibilityChecker {
	return &EligibilityChecker{repo: repo}
}

// Check evaluates rule (jsonb) for userID. orgID used for org-scoped queries.
// Returns eligible bool, reason string, error.
// Unknown rule keys are ignored (forward-compatible).
func (e *EligibilityChecker) Check(ctx context.Context, userID, orgID uuid.UUID, rule json.RawMessage) (bool, string, error) {
	if len(rule) == 0 {
		return true, "", nil
	}
	var r map[string]any
	if err := json.Unmarshal(rule, &r); err != nil {
		return false, "invalid_rule", nil
	}
	for key, val := range r {
		switch key {
		case "returning_runner":
			if b, _ := val.(bool); b {
				n, err := e.repo.CountPaidOrdersByUserInOrg(ctx, userID, orgID)
				if err != nil { return false, "db_error", err }
				if n < 1 { return false, "not_returning_runner", nil }
			}
		case "min_completions":
			min := int64(0)
			switch v := val.(type) {
			case float64: min = int64(v)
			case int64:   min = v
			}
			n, err := e.repo.CountPaidOrdersByUserInOrg(ctx, userID, orgID)
			if err != nil { return false, "db_error", err }
			if n < min { return false, "insufficient_completions", nil }
		case "membership_id_prefix":
			prefix, _ := val.(string)
			mid, err := e.repo.GetUserMembershipID(ctx, userID)
			if err != nil { return false, "db_error", err }
			if !strings.HasPrefix(mid, prefix) { return false, "membership_id_mismatch", nil }
		case "event_completed":
			eventIDStr, _ := val.(string)
			eventID, err := uuid.Parse(eventIDStr)
			if err != nil { return false, "invalid_event_id", nil }
			ok, err := e.repo.HasPaidOrderForEvent(ctx, userID, eventID)
			if err != nil { return false, "db_error", err }
			if !ok { return false, "event_not_completed", nil }
		// unknown keys: ignored
		}
	}
	return true, "", nil
}
```

- [ ] **Step 4: Add eligibility queries to access.sql + regenerate sqlc**

```sql
-- name: CountPaidOrdersByUserInOrg :one
SELECT count(*) FROM orders
WHERE participant_id = $1 AND organization_id = $2 AND status = 'PAID';

-- name: GetUserMembershipID :one
SELECT COALESCE(membership_id, '') FROM users WHERE id = $1;

-- name: HasPaidOrderForEvent :one
SELECT EXISTS(
    SELECT 1 FROM orders WHERE participant_id = $1 AND event_id = $2 AND status = 'PAID'
) AS exists;
```

**Note:** Read `services/api/internal/db/` to confirm `users` table has `membership_id` column. If not, add migration `00044_add_users_membership_id.sql`:
```sql
-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS membership_id text;
-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS membership_id;
```

Then run `make sqlc`.

- [ ] **Step 5: Run tests — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -run TestEligibility -race -v 2>&1
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/access/eligibility.go \
        services/api/internal/modules/access/tests/eligibility_test.go
git commit -m "feat(phase11): EligibilityChecker (returning_runner, min_completions, membership_id_prefix, event_completed)"
```

---

### Task 5: Pool Management Service

**Files:**
- Create: `services/api/internal/modules/access/pool_service.go`
- Create: `services/api/internal/modules/access/tests/pool_service_test.go`

- [ ] **Step 1: Write failing test**

```go
// services/api/internal/modules/access/tests/pool_service_test.go
package access_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

func TestPoolService_CreateAndGetPool(t *testing.T) {
	repo := &fakeAccessRepoFull{}
	svc := access.NewPoolService(repo)
	poolID, err := svc.CreatePool(context.Background(),
		uuid.New(), uuid.New(), uuid.New(),
		access.PoolTypeReserved, "Test Pool", 100, uuid.New())
	if err != nil { t.Fatal(err) }
	if poolID == uuid.Nil { t.Fatal("poolID should not be nil") }
}

// fakeAccessRepoFull: a thin fake that returns sane defaults
type fakeAccessRepoFull struct{}
// implement all Repository methods as no-ops returning empty values + nil errors
// (required to satisfy interface — one line per method)
```

- [ ] **Step 2: Write pool_service.go**

```go
package access

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type PoolService struct {
	repo  Repository
	audit AuditRecorder
}

func NewPoolService(repo Repository) *PoolService { return &PoolService{repo: repo} }

func (s *PoolService) CreatePool(ctx context.Context, orgID, eventID, categoryID uuid.UUID, poolType, name string, totalSlots int, createdBy uuid.UUID) (uuid.UUID, error) {
	pool, err := s.repo.CreateAccessPool(ctx, db.CreateAccessPoolParams{
		OrganizationID: orgID,
		EventID:        eventID,
		CategoryID:     categoryID,
		PoolType:       poolType,
		Name:           name,
		TotalSlots:     int32(totalSlots),
		CreatedBy:      createdBy,
	})
	if err != nil { return uuid.Nil, err }
	return pool.ID, nil
}

func (s *PoolService) SetVisible(ctx context.Context, poolID uuid.UUID, visible bool) error {
	v := visible
	_, err := s.repo.UpdateAccessPoolColumns(ctx, db.UpdateAccessPoolColumnsParams{
		ID:                       poolID,
		IsVisibleToParticipants:  pgtype.Bool{Bool: v, Valid: true},
	})
	return err
}

func (s *PoolService) SetEligibilityRule(ctx context.Context, poolID uuid.UUID, rule []byte) error {
	_, err := s.repo.UpdateAccessPoolColumns(ctx, db.UpdateAccessPoolColumnsParams{
		ID:              poolID,
		EligibilityRule: rule,
	})
	return err
}

func (s *PoolService) AdjustTotalSlots(ctx context.Context, poolID uuid.UUID, delta int) error {
	if delta == 0 { return nil }
	_, err := s.repo.TransferPoolSlots(ctx, db.TransferPoolSlotsParams{ID: poolID, Column2: int32(delta)})
	return err
}
```

- [ ] **Step 3: Run test — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -race -v 2>&1
```

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/access/pool_service.go \
        services/api/internal/modules/access/tests/pool_service_test.go
git commit -m "feat(phase11): pool management service (create, visible, eligibility, adjust slots)"
```

---

### Task 6: Corporate Service

**Files:**
- Create: `services/api/internal/modules/access/corporate.go`
- Create: `services/api/internal/modules/access/tests/corporate_test.go`

- [ ] **Step 1: Write failing test**

```go
// services/api/internal/modules/access/tests/corporate_test.go
package access_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

func TestCorporate_BulkUpload_ParsesCSV(t *testing.T) {
	csv := "email,name\nfoo@example.com,Foo\nbar@example.com,Bar\n"
	repo := &fakeAccessRepoFull{}
	svc := access.NewCorporateService(repo)
	result, err := svc.BulkUploadMembers(context.Background(), uuid.New(), uuid.New(), strings.NewReader(csv))
	if err != nil { t.Fatal(err) }
	if result.Imported != 2 { t.Fatalf("want 2 imported, got %d", result.Imported) }
}

func TestCorporate_BulkUpload_SkipsDuplicateEmails(t *testing.T) {
	csv := "email\nfoo@example.com\nfoo@example.com\nbar@example.com\n"
	repo := &fakeAccessRepoFull{}
	svc := access.NewCorporateService(repo)
	result, err := svc.BulkUploadMembers(context.Background(), uuid.New(), uuid.New(), strings.NewReader(csv))
	if err != nil { t.Fatal(err) }
	if result.Imported != 2 { t.Fatalf("want 2 (deduped), got %d", result.Imported) }
	if result.Skipped != 1 { t.Fatalf("want 1 skipped duplicate, got %d", result.Skipped) }
}

func TestCorporate_BulkUpload_RejectsIfExceedsQuota(t *testing.T) {
	csv := "email\na@x.com\nb@x.com\nc@x.com\n"
	// pool only has 2 available slots
	repo := &fakeAccessRepoFull{poolAvailable: 2}
	svc := access.NewCorporateService(repo)
	_, err := svc.BulkUploadMembers(context.Background(), uuid.New(), uuid.New(), strings.NewReader(csv))
	if err == nil { t.Fatal("should reject upload that exceeds pool quota") }
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -run TestCorporate -v 2>&1
```

- [ ] **Step 3: Write corporate.go**

```go
package access

import (
	"context"
	"encoding/csv"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type BulkUploadResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

type CorporateService struct{ repo Repository }

func NewCorporateService(repo Repository) *CorporateService { return &CorporateService{repo: repo} }

func (s *CorporateService) Create(ctx context.Context, orgID uuid.UUID, name, billingEmail string, invoiceRequired bool, createdBy uuid.UUID) (db.CorporateAccount, error) {
	return s.repo.CreateCorporateAccount(ctx, db.CreateCorporateAccountParams{
		OrganizationID:  orgID,
		Name:            name,
		BillingEmail:    billingEmail,
		InvoiceRequired: invoiceRequired,
		CreatedBy:       createdBy,
	})
}

func (s *CorporateService) Approve(ctx context.Context, accountID, approvedBy uuid.UUID) error {
	_, err := s.repo.ApproveCorporateAccount(ctx, db.ApproveCorporateAccountParams{
		ID: accountID, ApprovedBy: pgtype.UUID{Bytes: approvedBy, Valid: true},
	})
	return err
}

// BulkUploadMembers parses a CSV (header row: email[,name]) and creates AccessPoolMember rows.
// Rejects entire upload if row count exceeds pool available slots.
func (s *CorporateService) BulkUploadMembers(ctx context.Context, poolID, actorID uuid.UUID, r io.Reader) (BulkUploadResult, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil { return BulkUploadResult{}, err }
	emailIdx := -1
	for i, h := range header {
		if strings.EqualFold(h, "email") { emailIdx = i; break }
	}
	if emailIdx < 0 { return BulkUploadResult{}, ErrPoolExhausted } // reuse sentinel or define ErrInvalidCSV

	seen := map[string]bool{}
	var emails []string
	skipped := 0
	rows, _ := cr.ReadAll()
	for _, row := range rows {
		if emailIdx >= len(row) { continue }
		email := strings.TrimSpace(strings.ToLower(row[emailIdx]))
		if email == "" { continue }
		if seen[email] { skipped++; continue }
		seen[email] = true
		emails = append(emails, email)
	}

	// Quota check
	pool, err := s.repo.GetAccessPool(ctx, poolID)
	if err != nil { return BulkUploadResult{}, err }
	available := int(pool.TotalSlots - pool.ReservedSlots - pool.UsedSlots)
	if len(emails) > available {
		return BulkUploadResult{}, ErrPoolExhausted
	}

	imported := 0
	for _, email := range emails {
		_, err := s.repo.AddPoolMember(ctx, db.AddPoolMemberParams{
			PoolID: poolID, Email: email,
		})
		if err != nil { skipped++; continue }
		imported++
	}
	return BulkUploadResult{Imported: imported, Skipped: skipped}, nil
}

func (s *CorporateService) GenerateInvoice(ctx context.Context, accountID, eventID uuid.UUID, unitPrice int64) (map[string]any, error) {
	account, err := s.repo.GetCorporateAccount(ctx, accountID)
	if err != nil { return nil, err }
	members, err := s.repo.ListPoolMembers(ctx, db.ListPoolMembersParams{PoolID: uuid.Nil, Limit: 10000})
	if err != nil { return nil, err }
	n := int64(len(members))
	return map[string]any{
		"account":    map[string]any{"name": account.Name, "billing_email": account.BillingEmail},
		"line_items": []map[string]any{{"description": "Corporate slots", "quantity": n, "unit_price": unitPrice, "total": n * unitPrice}},
		"total":      n * unitPrice,
		"currency":   "IDR",
	}, nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go test ./internal/modules/access/tests/ -run TestCorporate -race -v 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/access/corporate.go \
        services/api/internal/modules/access/tests/corporate_test.go
git commit -m "feat(phase11): corporate service (create, approve, bulk upload CSV, invoice)"
```

---

### Task 7: Part 1 Full Verification

- [ ] **Step 1: Full build + vet + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api
go build ./... 2>&1
go vet ./... 2>&1
go test ./internal/modules/access/... -race -v 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
# Expected: all ok
```

- [ ] **Step 2: Commit**

```bash
git commit -m "test(phase11): part 1 foundation green — full pool types, eligibility, corporate"
```
