# Phase 7 Plan — Part 2: Tickets module (issuer + service)

> Part of the Phase 7 implementation plan. Index: [2026-06-08-phase7-participant-dashboard-ticket.md](2026-06-08-phase7-participant-dashboard-ticket.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** New files + additive changes only. Assumes Part 1 code exists.

**Generated types (verify in `services/api/internal/db/tickets.sql.go` before coding):** `db.Ticket` has `ID, OrganizationID, EventID, CategoryID, OrderID, ParticipantID uuid.UUID`; `TicketNumber, Status, HolderName, HolderEmail, EventTitle, CategoryName string`; `QrVersion int32`; `IssuedAt, CreatedAt, UpdatedAt pgtype.Timestamptz`; `UsedAt pgtype.Timestamptz` (nullable). `db.GetUserByID` returns `db.User{ID uuid.UUID, Email string, FullName string, ...}`. `db.GetEventByID` returns event with `Name string`. `db.GetCategoryByID` returns category with `Name string`.

---

## Task 6: Ticket model, DTOs, errors, ticket number

**Files:**
- Create: `services/api/internal/modules/tickets/model.go`
- Create: `services/api/internal/modules/tickets/errors.go`
- Create: `services/api/internal/modules/tickets/ticketnum.go`
- Create: `services/api/internal/modules/tickets/ticketnum_test.go`
- Create: `services/api/internal/modules/tickets/dto.go`

- [ ] **Step 1: Write the failing test (ticket number format)**

Create `services/api/internal/modules/tickets/ticketnum_test.go`:
```go
package tickets

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateTicketNumber_Format(t *testing.T) {
	now := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	n, err := generateTicketNumber(now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(n, "TIX-20260608-") {
		t.Fatalf("prefix wrong: %q", n)
	}
	if len(n) != len("TIX-20260608-")+6 {
		t.Fatalf("length wrong: %q", n)
	}
}

func TestGenerateTicketNumber_Unique(t *testing.T) {
	now := time.Now()
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		n, err := generateTicketNumber(now)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if seen[n] {
			t.Fatalf("collision: %q", n)
		}
		seen[n] = true
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/tickets/ -run TestGenerateTicketNumber -v; cd ../..
```
Expected: FAIL — package/`generateTicketNumber` not defined.

- [ ] **Step 3: Implement ticketnum.go** (mirrors `orders/ordernum.go`)

Create `services/api/internal/modules/tickets/ticketnum.go`:
```go
package tickets

import (
	"crypto/rand"
	"time"
)

const ticketNumAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateTicketNumber(now time.Time) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	suffix := make([]byte, 6)
	for i := range b {
		suffix[i] = ticketNumAlphabet[int(b[i])%len(ticketNumAlphabet)]
	}
	return "TIX-" + now.Format("20060102") + "-" + string(suffix), nil
}
```

- [ ] **Step 4: Implement model.go**

Create `services/api/internal/modules/tickets/model.go`:
```go
package tickets

// Ticket statuses.
const (
	StatusValid     = "VALID"
	StatusUsed      = "USED"
	StatusCancelled = "CANCELLED"
)

// Order status constants needed for invoice gating (mirror orders module values).
const orderStatusPaid = "PAID"
```

- [ ] **Step 5: Implement errors.go** (typed errors → codes, envelope Phase 2)

Create `services/api/internal/modules/tickets/errors.go`:
```go
package tickets

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrTicketNotFound     = apperr.New(http.StatusNotFound, "TICKET_NOT_FOUND", "Ticket not found.")
	ErrTicketNotAvailable = apperr.New(http.StatusConflict, "TICKET_NOT_AVAILABLE", "Ticket is not available for this order yet.")
	ErrInvoiceNotAvailable = apperr.New(http.StatusConflict, "INVOICE_NOT_AVAILABLE", "Invoice is only available for paid orders.")
)
```

- [ ] **Step 6: Implement dto.go**

Create `services/api/internal/modules/tickets/dto.go`:
```go
package tickets

import "time"

// TicketResponse is the participant/organizer-facing ticket view.
type TicketResponse struct {
	ID           string     `json:"id"`
	TicketNumber string     `json:"ticketNumber"`
	Status       string     `json:"status"`
	OrderID      string     `json:"orderId"`
	EventID      string     `json:"eventId"`
	CategoryID   string     `json:"categoryId"`
	HolderName   string     `json:"holderName"`
	HolderEmail  string     `json:"holderEmail"`
	EventTitle   string     `json:"eventTitle"`
	CategoryName string     `json:"categoryName"`
	IssuedAt     time.Time  `json:"issuedAt"`
	UsedAt       *time.Time `json:"usedAt,omitempty"`
}

// TicketWithQR adds the signed QR token to a ticket view.
type TicketWithQR struct {
	TicketResponse
	QRToken string `json:"qrToken"`
}

// QRResponse is the QR-only endpoint payload.
type QRResponse struct {
	QRToken string `json:"qrToken"`
}

// InvoiceResponse is the JSON invoice for a paid order.
type InvoiceResponse struct {
	OrderID      string    `json:"orderId"`
	OrderNumber  string    `json:"orderNumber"`
	Status       string    `json:"status"`
	EventTitle   string    `json:"eventTitle"`
	CategoryName string    `json:"categoryName"`
	HolderName   string    `json:"holderName"`
	HolderEmail  string    `json:"holderEmail"`
	Subtotal     int64     `json:"subtotal"`
	Fee          int64     `json:"fee"`
	Discount     int64     `json:"discount"`
	Total        int64     `json:"total"`
	Currency     string    `json:"currency"`
	IssuedAt     time.Time `json:"issuedAt"`
}
```

- [ ] **Step 7: Run tests to verify they pass + build**

Run:
```bash
cd services/api && go test ./internal/modules/tickets/ -run TestGenerateTicketNumber -v && go build ./internal/modules/tickets/...; cd ../..
```
Expected: PASS + builds.

- [ ] **Step 8: Commit**

```bash
git add services/api/internal/modules/tickets/model.go services/api/internal/modules/tickets/errors.go services/api/internal/modules/tickets/ticketnum.go services/api/internal/modules/tickets/ticketnum_test.go services/api/internal/modules/tickets/dto.go
git commit -m "feat(phase7): tickets model, dto, errors, ticket number"
```

---

## Task 7: Tickets repository

**Files:**
- Create: `services/api/internal/modules/tickets/repository.go`

- [ ] **Step 1: Implement repository.go** (pool-backed + tx-aware via ExecTx, mirrors payments)

Create `services/api/internal/modules/tickets/repository.go`:
```go
package tickets

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository defines data access for tickets.
type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error

	CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error)
	GetTicketByID(ctx context.Context, id uuid.UUID) (db.Ticket, error)
	GetTicketByOrderID(ctx context.Context, orderID uuid.UUID) (db.Ticket, error)
	ListTicketsByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Ticket, error)
	ListTicketsByEvent(ctx context.Context, arg db.ListTicketsByEventParams) ([]db.Ticket, error)

	// Lookups for snapshotting + invoice (reuse existing queries).
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error)
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
	if err := fn(&sqlcRepo{pool: r.pool, q: db.New(tx)}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *sqlcRepo) CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error) {
	return r.q.CreateTicket(ctx, arg)
}
func (r *sqlcRepo) GetTicketByID(ctx context.Context, id uuid.UUID) (db.Ticket, error) {
	return r.q.GetTicketByID(ctx, id)
}
func (r *sqlcRepo) GetTicketByOrderID(ctx context.Context, orderID uuid.UUID) (db.Ticket, error) {
	return r.q.GetTicketByOrderID(ctx, orderID)
}
func (r *sqlcRepo) ListTicketsByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Ticket, error) {
	return r.q.ListTicketsByParticipant(ctx, participantID)
}
func (r *sqlcRepo) ListTicketsByEvent(ctx context.Context, arg db.ListTicketsByEventParams) ([]db.Ticket, error) {
	return r.q.ListTicketsByEvent(ctx, arg)
}
func (r *sqlcRepo) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return r.q.GetUserByID(ctx, id)
}
func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}
func (r *sqlcRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return r.q.GetCategoryByID(ctx, id)
}
func (r *sqlcRepo) GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error) {
	return r.q.GetOrderByID(ctx, id)
}
```

> Before committing, confirm the generated method names match (`db.CreateTicketParams`, `db.ListTicketsByEventParams`, `GetUserByID`, `GetEventByID`, `GetCategoryByID`, `GetOrderByID`). VERIFIED present: `GetUserByID`, `GetEventByID`, `GetCategoryByID`, and `GetOrderByID` (`-- name: GetOrderByID :one` in `database/queries/orders.sql:7`) already exist — reuse them, no new query needed. Drop `database/queries/orders.sql` from the Step 3 commit if you didn't modify it.

- [ ] **Step 2: Build**

Run:
```bash
cd services/api && go build ./internal/modules/tickets/...; cd ../..
```
Expected: builds clean.

- [ ] **Step 3: Commit**

```bash
git add services/api/internal/modules/tickets/repository.go database/queries/orders.sql services/api/internal/db
git commit -m "feat(phase7): tickets repository"
```

---

## Task 8: Ticket issuer (idempotent, tx-bound)

**Files:**
- Create: `services/api/internal/modules/tickets/issuer.go`
- Create: `services/api/internal/modules/tickets/tests/issuer_test.go`

The issuer is called by the payments processor inside the PAID transaction. It receives the **tx-bound `*db.Queries`** so the INSERT shares the transaction. It is idempotent via `CreateTicket`'s `ON CONFLICT (order_id) DO NOTHING`.

- [ ] **Step 1: Write the failing test** (fake querier-less: test the issuer against an in-memory fake implementing the small surface it needs)

Create `services/api/internal/modules/tickets/tests/issuer_test.go`:
```go
package tickets_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets"
)

// fakeQ implements tickets.IssuerQuerier.
type fakeQ struct {
	user     db.User
	event    db.Event
	category db.EventCategory
	created  []db.CreateTicketParams
	conflict bool // when true, CreateTicket returns no rows (duplicate)
}

func (f *fakeQ) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return f.user, nil
}
func (f *fakeQ) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return f.event, nil
}
func (f *fakeQ) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return f.category, nil
}
func (f *fakeQ) CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error) {
	f.created = append(f.created, arg)
	if f.conflict {
		return db.Ticket{}, db.ErrNoRows // sentinel; see note
	}
	return db.Ticket{ID: uuid.New(), OrderID: arg.OrderID}, nil
}

func sampleOrder() db.Order {
	return db.Order{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		EventID:        uuid.New(),
		CategoryID:     uuid.New(),
		ParticipantID:  uuid.New(),
		OrderNumber:    "ORD-20260608-ABCDEF",
		Status:         "PAID",
	}
}

func TestIssueForOrder_Snapshots(t *testing.T) {
	fq := &fakeQ{
		user:     db.User{Email: "p@example.com", FullName: "Pelari Satu"},
		event:    db.Event{Name: "Jakarta Run 2026"},
		category: db.EventCategory{Name: "10K"},
	}
	iss := tickets.NewIssuer(nil) // audit recorder optional; nil tolerated
	order := sampleOrder()

	if err := iss.IssueWith(context.Background(), fq, order); err != nil {
		t.Fatalf("issue: %v", err)
	}
	if len(fq.created) != 1 {
		t.Fatalf("expected 1 create, got %d", len(fq.created))
	}
	got := fq.created[0]
	if got.HolderName != "Pelari Satu" || got.HolderEmail != "p@example.com" {
		t.Errorf("holder snapshot wrong: %+v", got)
	}
	if got.EventTitle != "Jakarta Run 2026" || got.CategoryName != "10K" {
		t.Errorf("event/category snapshot wrong: %+v", got)
	}
	if got.OrderID != order.ID {
		t.Errorf("order id mismatch")
	}
}

func TestIssueForOrder_DuplicateNoError(t *testing.T) {
	fq := &fakeQ{
		user:     db.User{Email: "p@example.com", FullName: "Pelari Satu"},
		event:    db.Event{Name: "Jakarta Run 2026"},
		category: db.EventCategory{Name: "10K"},
		conflict: true,
	}
	iss := tickets.NewIssuer(nil)
	if err := iss.IssueWith(context.Background(), fq, sampleOrder()); err != nil {
		t.Fatalf("duplicate issue should be no-op, got: %v", err)
	}
}
```

> NOTE on `db.ErrNoRows`: sqlc/pgx returns `pgx.ErrNoRows`. If `db` has no re-export, import `github.com/jackc/pgx/v5` in the test and use `pgx.ErrNoRows` instead of `db.ErrNoRows`, and have the issuer compare with `errors.Is(err, pgx.ErrNoRows)`.

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/tickets/tests/ -run TestIssueForOrder -v; cd ../..
```
Expected: FAIL — `tickets.NewIssuer`/`IssueWith`/`IssuerQuerier` undefined.

- [ ] **Step 3: Implement issuer.go**

Create `services/api/internal/modules/tickets/issuer.go`:
```go
package tickets

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// IssuerQuerier is the minimal tx-bound query surface the issuer needs.
// *db.Queries satisfies it.
type IssuerQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error)
}

// AuditRecorder is satisfied by *audit.Logger.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

// Issuer creates a ticket for a just-PAID order, on the SAME tx as the caller.
type Issuer struct {
	audit AuditRecorder
}

func NewIssuer(recorder AuditRecorder) *Issuer {
	return &Issuer{audit: recorder}
}

// IssueForOrder satisfies payments.TicketIssuer. q MUST be the tx-bound querier
// from the payments transaction so the INSERT commits/rolls back atomically.
func (i *Issuer) IssueForOrder(ctx context.Context, q *db.Queries, order db.Order) error {
	return i.IssueWith(ctx, q, order)
}

// IssueWith is the testable core; q is any IssuerQuerier.
func (i *Issuer) IssueWith(ctx context.Context, q IssuerQuerier, order db.Order) error {
	user, err := q.GetUserByID(ctx, order.ParticipantID)
	if err != nil {
		return err
	}
	event, err := q.GetEventByID(ctx, order.EventID)
	if err != nil {
		return err
	}
	category, err := q.GetCategoryByID(ctx, order.CategoryID)
	if err != nil {
		return err
	}

	num, err := generateTicketNumber(time.Now())
	if err != nil {
		return err
	}

	created, err := q.CreateTicket(ctx, db.CreateTicketParams{
		OrganizationID: order.OrganizationID,
		EventID:        order.EventID,
		CategoryID:     order.CategoryID,
		OrderID:        order.ID,
		ParticipantID:  order.ParticipantID,
		TicketNumber:   num,
		HolderName:     user.FullName,
		HolderEmail:    user.Email,
		EventTitle:     event.Name,
		CategoryName:   category.Name,
		QrVersion:      1,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Ticket already issued for this order (ON CONFLICT) → idempotent no-op.
		return nil
	}
	if err != nil {
		return err
	}

	if i.audit != nil {
		orgID := order.OrganizationID
		actor := order.ParticipantID
		i.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &actor,
			Action:         "TICKET_ISSUED",
			TargetType:     "ticket",
			TargetID:       created.ID.String(),
			Metadata:       map[string]any{"orderId": order.ID.String(), "ticketNumber": num},
		})
	}
	return nil
}
```

> `*db.Queries` has methods `GetUserByID/GetEventByID/GetCategoryByID/CreateTicket` with these exact signatures (sqlc-generated), so it satisfies `IssuerQuerier` — that is what makes `IssueForOrder(ctx, *db.Queries, order)` work with the tx querier in Part 3.

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/modules/tickets/tests/ -run TestIssueForOrder -v; cd ../..
```
Expected: PASS (both). If `db.ErrNoRows` doesn't exist, apply the test NOTE (use `pgx.ErrNoRows`).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/tickets/issuer.go services/api/internal/modules/tickets/tests/issuer_test.go
git commit -m "feat(phase7): idempotent ticket issuer (tx-bound, snapshots)"
```

---

## Task 9: Tickets service (get, list, QR, invoice)

**Files:**
- Create: `services/api/internal/modules/tickets/service.go`
- Create: `services/api/internal/modules/tickets/tests/service_test.go`

- [ ] **Step 1: Write the failing tests** (ownership + invoice gating, fake repo)

Create `services/api/internal/modules/tickets/tests/service_test.go`:
```go
package tickets_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets"
)

type fakeRepo struct {
	ticket db.Ticket
	order  db.Order
	getErr error
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(tickets.Repository) error) error { return fn(f) }
func (f *fakeRepo) CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error) {
	return f.ticket, nil
}
func (f *fakeRepo) GetTicketByID(ctx context.Context, id uuid.UUID) (db.Ticket, error) {
	if f.getErr != nil {
		return db.Ticket{}, f.getErr
	}
	return f.ticket, nil
}
func (f *fakeRepo) GetTicketByOrderID(ctx context.Context, orderID uuid.UUID) (db.Ticket, error) {
	if f.getErr != nil {
		return db.Ticket{}, f.getErr
	}
	return f.ticket, nil
}
func (f *fakeRepo) ListTicketsByParticipant(ctx context.Context, pid uuid.UUID) ([]db.Ticket, error) {
	return []db.Ticket{f.ticket}, nil
}
func (f *fakeRepo) ListTicketsByEvent(ctx context.Context, arg db.ListTicketsByEventParams) ([]db.Ticket, error) {
	return []db.Ticket{f.ticket}, nil
}
func (f *fakeRepo) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) { return db.User{}, nil }
func (f *fakeRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) { return db.Event{}, nil }
func (f *fakeRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return db.EventCategory{}, nil
}
func (f *fakeRepo) GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error) { return f.order, nil }

func TestGetTicketForUser_OwnershipMismatch_404(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	repo := &fakeRepo{ticket: db.Ticket{ID: uuid.New(), ParticipantID: owner}}
	svc := tickets.NewService(repo, tickets.NewQRSigner("secret"), nil)

	_, err := svc.GetTicketForUser(context.Background(), repo, other, repo.ticket.ID)
	if !errors.Is(err, tickets.ErrTicketNotFound) {
		t.Fatalf("want ErrTicketNotFound, got %v", err)
	}
}

func TestGetTicketForUser_NotFound_404(t *testing.T) {
	repo := &fakeRepo{getErr: pgx.ErrNoRows}
	svc := tickets.NewService(repo, tickets.NewQRSigner("secret"), nil)
	_, err := svc.GetTicketForUser(context.Background(), repo, uuid.New(), uuid.New())
	if !errors.Is(err, tickets.ErrTicketNotFound) {
		t.Fatalf("want ErrTicketNotFound, got %v", err)
	}
}

func TestGetInvoice_OrderNotPaid_Conflict(t *testing.T) {
	uid := uuid.New()
	repo := &fakeRepo{order: db.Order{ID: uuid.New(), ParticipantID: uid, Status: "PENDING_PAYMENT"}}
	svc := tickets.NewService(repo, tickets.NewQRSigner("secret"), nil)
	_, err := svc.GetInvoiceForUser(context.Background(), uid, repo.order.ID)
	if !errors.Is(err, tickets.ErrInvoiceNotAvailable) {
		t.Fatalf("want ErrInvoiceNotAvailable, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
cd services/api && go test ./internal/modules/tickets/tests/ -run 'TestGetTicket|TestGetInvoice' -v; cd ../..
```
Expected: FAIL — `tickets.NewService`/`NewQRSigner`/methods undefined.

- [ ] **Step 3: Implement service.go**

Create `services/api/internal/modules/tickets/service.go`:
```go
package tickets

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets/qr"
)

// QRSigner is the signing surface the service needs (qr.Signer satisfies it).
type QRSigner interface {
	Sign(ticketID, eventID uuid.UUID) (string, error)
}

// NewQRSigner is a thin constructor re-export for callers/tests.
func NewQRSigner(secret string) *qr.Signer { return qr.NewSigner(secret) }

type Service struct {
	repo   Repository
	signer QRSigner
	audit  AuditRecorder
}

func NewService(repo Repository, signer QRSigner, recorder AuditRecorder) *Service {
	return &Service{repo: repo, signer: signer, audit: recorder}
}

func toResponse(t db.Ticket) TicketResponse {
	r := TicketResponse{
		ID:           t.ID.String(),
		TicketNumber: t.TicketNumber,
		Status:       t.Status,
		OrderID:      t.OrderID.String(),
		EventID:      t.EventID.String(),
		CategoryID:   t.CategoryID.String(),
		HolderName:   t.HolderName,
		HolderEmail:  t.HolderEmail,
		EventTitle:   t.EventTitle,
		CategoryName: t.CategoryName,
		IssuedAt:     t.IssuedAt.Time,
	}
	if t.UsedAt.Valid {
		u := t.UsedAt.Time
		r.UsedAt = &u
	}
	return r
}

// GetTicketForUser returns a ticket owned by userID (else ErrTicketNotFound).
// repo param allows passing a tx repo; pass s.repo for plain reads.
func (s *Service) GetTicketForUser(ctx context.Context, repo Repository, userID, ticketID uuid.UUID) (TicketWithQR, error) {
	t, err := repo.GetTicketByID(ctx, ticketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TicketWithQR{}, ErrTicketNotFound
	}
	if err != nil {
		return TicketWithQR{}, err
	}
	if t.ParticipantID != userID {
		return TicketWithQR{}, ErrTicketNotFound
	}
	token, err := s.signer.Sign(t.ID, t.EventID)
	if err != nil {
		return TicketWithQR{}, err
	}
	return TicketWithQR{TicketResponse: toResponse(t), QRToken: token}, nil
}

func (s *Service) GetTicketByOrderForUser(ctx context.Context, userID, orderID uuid.UUID) (TicketWithQR, error) {
	t, err := s.repo.GetTicketByOrderID(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TicketWithQR{}, ErrTicketNotFound
	}
	if err != nil {
		return TicketWithQR{}, err
	}
	if t.ParticipantID != userID {
		return TicketWithQR{}, ErrTicketNotFound
	}
	token, err := s.signer.Sign(t.ID, t.EventID)
	if err != nil {
		return TicketWithQR{}, err
	}
	return TicketWithQR{TicketResponse: toResponse(t), QRToken: token}, nil
}

func (s *Service) ListMyTickets(ctx context.Context, userID uuid.UUID) ([]TicketResponse, error) {
	rows, err := s.repo.ListTicketsByParticipant(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]TicketResponse, 0, len(rows))
	for _, t := range rows {
		out = append(out, toResponse(t))
	}
	return out, nil
}

func (s *Service) GetQRForUser(ctx context.Context, userID, ticketID uuid.UUID) (string, error) {
	tw, err := s.GetTicketForUser(ctx, s.repo, userID, ticketID)
	if err != nil {
		return "", err
	}
	return tw.QRToken, nil
}

func (s *Service) ListEventTickets(ctx context.Context, orgID, eventID uuid.UUID) ([]TicketResponse, error) {
	rows, err := s.repo.ListTicketsByEvent(ctx, db.ListTicketsByEventParams{OrganizationID: orgID, EventID: eventID})
	if err != nil {
		return nil, err
	}
	out := make([]TicketResponse, 0, len(rows))
	for _, t := range rows {
		out = append(out, toResponse(t))
	}
	return out, nil
}

// GetInvoiceForUser returns a JSON invoice for a PAID order owned by userID.
func (s *Service) GetInvoiceForUser(ctx context.Context, userID, orderID uuid.UUID) (InvoiceResponse, error) {
	order, err := s.repo.GetOrderByID(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return InvoiceResponse{}, ErrTicketNotFound
	}
	if err != nil {
		return InvoiceResponse{}, err
	}
	if order.ParticipantID != userID {
		return InvoiceResponse{}, ErrTicketNotFound
	}
	if order.Status != orderStatusPaid {
		return InvoiceResponse{}, ErrInvoiceNotAvailable
	}
	t, err := s.repo.GetTicketByOrderID(ctx, orderID)
	if err != nil {
		return InvoiceResponse{}, err
	}
	return InvoiceResponse{
		OrderID:      order.ID.String(),
		OrderNumber:  order.OrderNumber,
		Status:       order.Status,
		EventTitle:   t.EventTitle,
		CategoryName: t.CategoryName,
		HolderName:   t.HolderName,
		HolderEmail:  t.HolderEmail,
		Subtotal:     order.Subtotal,
		Fee:          order.Fee,
		Discount:     order.Discount,
		Total:        order.Total,
		Currency:     "IDR",
		IssuedAt:     t.IssuedAt.Time,
	}, nil
}

var _ = time.Now // keep time import if unused after edits
```

> Remove the `var _ = time.Now` line and the `time` import if the compiler reports `time` unused.

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/modules/tickets/tests/ -run 'TestGetTicket|TestGetInvoice' -v; cd ../..
```
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/tickets/service.go services/api/internal/modules/tickets/tests/service_test.go
git commit -m "feat(phase7): tickets service (get/list/qr/invoice, ownership 404)"
```

---

## Task 10: Run full tickets package tests

- [ ] **Step 1: Run all tickets tests**

Run:
```bash
cd services/api && go test ./internal/modules/tickets/... -v && go vet ./internal/modules/tickets/...; cd ../..
```
Expected: PASS + vet clean.

- [ ] **Step 2: Commit (if any fixups)**

```bash
git add -A services/api/internal/modules/tickets
git commit -m "test(phase7): tickets package green" || echo "nothing to commit"
```

---

Part 2 complete. Next: [Part 3 — Payments seam (atomic issuance)](2026-06-08-phase7-part3-payments-seam.md).
