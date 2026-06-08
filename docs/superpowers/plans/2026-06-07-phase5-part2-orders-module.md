# Phase 5 Plan — Part 2: Orders Module (Tasks 5-7)

> Part of the Phase 5 implementation plan. Index: [2026-06-07-phase5-orders-inventory-checkout.md](2026-06-07-phase5-orders-inventory-checkout.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **Depends on:** Part 1 (migrations, sqlc queries, inventory domain). Verify generated `db.Order`/`db.InventoryReservation` + `*Params` types before writing repository/service. **EXTEND, DON'T REWRITE.**

---

## Task 5: Orders errors, model, dto, order-number generator, repository

**Files:**
- Create: `services/api/internal/modules/orders/errors.go`
- Create: `services/api/internal/modules/orders/model.go`
- Create: `services/api/internal/modules/orders/dto.go`
- Create: `services/api/internal/modules/orders/ordernum.go`
- Test: `services/api/internal/modules/orders/ordernum_test.go`
- Create: `services/api/internal/modules/orders/repository.go`

- [ ] **Step 1: Typed errors**

Create `services/api/internal/modules/orders/errors.go`:
```go
package orders

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrOrderNotFound     = apperr.New(http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
	ErrCategoryNotFound  = apperr.New(http.StatusNotFound, "CATEGORY_NOT_FOUND", "category not found")
	ErrEventNotPublished = apperr.New(http.StatusConflict, "EVENT_NOT_PUBLISHED", "event is not published")
	ErrRegistrationClosed = apperr.New(http.StatusConflict, "REGISTRATION_CLOSED", "registration window is closed")
	ErrMaxOrderExceeded  = apperr.New(http.StatusConflict, "MAX_ORDER_EXCEEDED", "maximum orders per user reached for this category")
	ErrDuplicateActive   = apperr.New(http.StatusConflict, "DUPLICATE_ACTIVE_ORDER", "you already have an active order for this category")
	ErrInvalidState      = apperr.New(http.StatusConflict, "INVALID_ORDER_STATE", "order cannot transition from its current state")
	ErrOrderNumberGen    = apperr.New(http.StatusInternalServerError, "ORDER_NUMBER_GENERATION_FAILED", "could not generate a unique order number")
)
```

- [ ] **Step 2: Model (status constants)**

Create `services/api/internal/modules/orders/model.go`:
```go
package orders

const (
	StatusDraft          = "DRAFT"
	StatusPendingPayment = "PENDING_PAYMENT"
	StatusPaid           = "PAID"
	StatusExpired        = "EXPIRED"
	StatusCancelled      = "CANCELLED"
	StatusRefunded       = "REFUNDED"
)

// Reservation terminal statuses (mirror of inventory states used by orders).
const (
	ReservationActive    = "ACTIVE"
	ReservationReleased  = "RELEASED"
	ReservationExpired   = "EXPIRED"
	ReservationCompleted = "COMPLETED"
)
```

- [ ] **Step 3: DTOs**

Create `services/api/internal/modules/orders/dto.go`:
```go
package orders

import (
	"time"

	"github.com/google/uuid"
)

type OrderResponse struct {
	ID          uuid.UUID  `json:"id"`
	OrderNumber string     `json:"orderNumber"`
	EventID     uuid.UUID  `json:"eventId"`
	CategoryID  uuid.UUID  `json:"categoryId"`
	Status      string     `json:"status"`
	Subtotal    int64      `json:"subtotal"`
	Fee         int64      `json:"fee"`
	Discount    int64      `json:"discount"`
	Total       int64      `json:"total"`
	ExpiredAt   *time.Time `json:"expiredAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}
```

- [ ] **Step 4: Write failing test for order-number generator**

Create `services/api/internal/modules/orders/ordernum_test.go`:
```go
package orders

import (
	"regexp"
	"testing"
	"time"
)

func TestGenerateOrderNumber_Format(t *testing.T) {
	num, err := generateOrderNumber(time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	re := regexp.MustCompile(`^ORD-20260607-[A-Z0-9]{6}$`)
	if !re.MatchString(num) {
		t.Errorf("order number %q does not match format", num)
	}
}

func TestGenerateOrderNumber_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		num, err := generateOrderNumber(time.Now())
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[num] {
			t.Fatalf("collision at %d: %s", i, num)
		}
		seen[num] = true
	}
}
```

- [ ] **Step 5: Implement order-number generator**

Create `services/api/internal/modules/orders/ordernum.go`:
```go
package orders

import (
	"crypto/rand"
	"time"
)

const orderNumAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// generateOrderNumber builds ORD-YYYYMMDD-XXXXXX with a crypto-random suffix.
func generateOrderNumber(now time.Time) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	suffix := make([]byte, 6)
	for i := range b {
		suffix[i] = orderNumAlphabet[int(b[i])%len(orderNumAlphabet)]
	}
	return "ORD-" + now.Format("20060102") + "-" + string(suffix), nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/modules/orders/ -run TestGenerateOrderNumber -v; cd ../..
```
Expected: PASS.

- [ ] **Step 7: Repository interface + adapter (with ExecTx)**

Create `services/api/internal/modules/orders/repository.go`:
```go
package orders

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error

	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error)
	GetOrderByNumber(ctx context.Context, number string) (db.Order, error)
	CreateOrder(ctx context.Context, arg db.CreateOrderParams) (db.Order, error)
	UpdateOrderStatus(ctx context.Context, arg db.UpdateOrderStatusParams) (db.Order, error)
	ListOrdersByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Order, error)
	ListOrdersByOrgEvent(ctx context.Context, arg db.ListOrdersByOrgEventParams) ([]db.Order, error)
	CountActiveOrdersForUserCategory(ctx context.Context, arg db.CountActiveOrdersForUserCategoryParams) (int64, error)

	// Inventory ops share the same tx. Inventory() returns an inventory.Repository
	// bound to this repo's queries (pool or tx).
	Inventory() inv.Repository
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

func (r *sqlcRepo) Inventory() inv.Repository { return inv.NewRepository(r.q) }

func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}
func (r *sqlcRepo) GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error) {
	return r.q.GetOrderByID(ctx, id)
}
func (r *sqlcRepo) GetOrderByNumber(ctx context.Context, number string) (db.Order, error) {
	return r.q.GetOrderByNumber(ctx, number)
}
func (r *sqlcRepo) CreateOrder(ctx context.Context, arg db.CreateOrderParams) (db.Order, error) {
	return r.q.CreateOrder(ctx, arg)
}
func (r *sqlcRepo) UpdateOrderStatus(ctx context.Context, arg db.UpdateOrderStatusParams) (db.Order, error) {
	return r.q.UpdateOrderStatus(ctx, arg)
}
func (r *sqlcRepo) ListOrdersByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Order, error) {
	return r.q.ListOrdersByParticipant(ctx, participantID)
}
func (r *sqlcRepo) ListOrdersByOrgEvent(ctx context.Context, arg db.ListOrdersByOrgEventParams) ([]db.Order, error) {
	return r.q.ListOrdersByOrgEvent(ctx, arg)
}
func (r *sqlcRepo) CountActiveOrdersForUserCategory(ctx context.Context, arg db.CountActiveOrdersForUserCategoryParams) (int64, error) {
	return r.q.CountActiveOrdersForUserCategory(ctx, arg)
}
```
Note: `Inventory()` returns an `inventory.Repository` bound to the SAME `*db.Queries` (`r.q`) — so when called within `ExecTx`, both order and reservation writes use the same tx. Confirm `inv.NewRepository(*db.Queries)` signature matches Part 1.

- [ ] **Step 8: Build**

Run:
```bash
cd services/api && go build ./... && go test ./internal/modules/orders/ -v; cd ../..
```
Expected: build OK; ordernum tests PASS.

- [ ] **Step 9: Commit**

```bash
git add services/api/internal/modules/orders/errors.go services/api/internal/modules/orders/model.go \
  services/api/internal/modules/orders/dto.go services/api/internal/modules/orders/ordernum.go \
  services/api/internal/modules/orders/ordernum_test.go services/api/internal/modules/orders/repository.go
git commit -m "feat(orders): add errors, model, dto, order-number generator, repository"
```

---

## Task 6: Orders service (atomic checkout, cancel, list)

**Files:**
- Create: `services/api/internal/modules/orders/validator.go`
- Create: `services/api/internal/modules/orders/service.go`
- Test: `services/api/internal/modules/orders/service_test.go`

- [ ] **Step 1: Validator (event/category checkout eligibility)**

Create `services/api/internal/modules/orders/validator.go`:
```go
package orders

import (
	"time"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// checkoutEligible validates an event is published and a category's registration
// window is open at `now`. Returns a typed error or nil.
func checkoutEligible(event db.Event, cat db.EventCategory, now time.Time) error {
	if event.Status != "published" {
		return ErrEventNotPublished
	}
	if cat.RegistrationOpensAt.Valid && now.Before(cat.RegistrationOpensAt.Time) {
		return ErrRegistrationClosed
	}
	if cat.RegistrationClosesAt.Valid && now.After(cat.RegistrationClosesAt.Time) {
		return ErrRegistrationClosed
	}
	return nil
}
```

- [ ] **Step 2: Write the failing service tests**

Create `services/api/internal/modules/orders/service_test.go`:
```go
package orders

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
)

// fakeRepo implements orders.Repository AND inventory.Repository (so Inventory()
// can return itself). It models a single category's capacity/reservations.
type fakeRepo struct {
	events       map[uuid.UUID]db.Event
	categories   map[uuid.UUID]db.EventCategory
	orders       map[uuid.UUID]db.Order
	reservations map[uuid.UUID]db.InventoryReservation // by order_id
	orderNumbers map[string]bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		events:       map[uuid.UUID]db.Event{},
		categories:   map[uuid.UUID]db.EventCategory{},
		orders:       map[uuid.UUID]db.Order{},
		reservations: map[uuid.UUID]db.InventoryReservation{},
		orderNumbers: map[string]bool{},
	}
}

func (f *fakeRepo) seed(orgID uuid.UUID, capacity, maxOrder int32) (db.Event, db.EventCategory) {
	ev := db.Event{ID: uuid.New(), OrganizationID: orgID, Status: "published"}
	cat := db.EventCategory{
		ID: uuid.New(), OrganizationID: orgID, EventID: ev.ID, Price: 100000,
		Capacity: capacity, MaxOrderPerUser: maxOrder,
		RegistrationOpensAt:  pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
		RegistrationClosesAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}
	f.events[ev.ID] = ev
	f.categories[cat.ID] = cat
	return ev, cat
}

// --- orders.Repository ---
func (f *fakeRepo) ExecTx(ctx context.Context, fn func(Repository) error) error { return fn(f) }
func (f *fakeRepo) Inventory() inv.Repository                                     { return f }
func (f *fakeRepo) GetEventByID(_ context.Context, id uuid.UUID) (db.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return db.Event{}, pgx.ErrNoRows
	}
	return e, nil
}
func (f *fakeRepo) GetOrderByID(_ context.Context, id uuid.UUID) (db.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return db.Order{}, pgx.ErrNoRows
	}
	return o, nil
}
func (f *fakeRepo) GetOrderByNumber(_ context.Context, n string) (db.Order, error) {
	if f.orderNumbers[n] {
		return db.Order{OrderNumber: n}, nil
	}
	return db.Order{}, pgx.ErrNoRows
}
func (f *fakeRepo) CreateOrder(_ context.Context, arg db.CreateOrderParams) (db.Order, error) {
	o := db.Order{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID,
		CategoryID: arg.CategoryID, ParticipantID: arg.ParticipantID, OrderNumber: arg.OrderNumber,
		Status: arg.Status, Subtotal: arg.Subtotal, Fee: arg.Fee, Discount: arg.Discount,
		Total: arg.Total, ExpiredAt: arg.ExpiredAt,
	}
	f.orders[o.ID] = o
	f.orderNumbers[arg.OrderNumber] = true
	return o, nil
}
func (f *fakeRepo) UpdateOrderStatus(_ context.Context, arg db.UpdateOrderStatusParams) (db.Order, error) {
	o, ok := f.orders[arg.ID]
	if !ok || o.Status != arg.Status_2 {
		return db.Order{}, pgx.ErrNoRows
	}
	o.Status = arg.Status
	f.orders[arg.ID] = o
	return o, nil
}
func (f *fakeRepo) ListOrdersByParticipant(_ context.Context, pid uuid.UUID) ([]db.Order, error) {
	var out []db.Order
	for _, o := range f.orders {
		if o.ParticipantID == pid {
			out = append(out, o)
		}
	}
	return out, nil
}
func (f *fakeRepo) ListOrdersByOrgEvent(_ context.Context, arg db.ListOrdersByOrgEventParams) ([]db.Order, error) {
	var out []db.Order
	for _, o := range f.orders {
		if o.OrganizationID == arg.OrganizationID && o.EventID == arg.EventID {
			out = append(out, o)
		}
	}
	return out, nil
}
func (f *fakeRepo) CountActiveOrdersForUserCategory(_ context.Context, arg db.CountActiveOrdersForUserCategoryParams) (int64, error) {
	var n int64
	for _, o := range f.orders {
		if o.CategoryID == arg.CategoryID && o.ParticipantID == arg.ParticipantID &&
			(o.Status == StatusPendingPayment || o.Status == StatusPaid) {
			n++
		}
	}
	return n, nil
}

// --- inventory.Repository ---
func (f *fakeRepo) LockCategoryForUpdate(_ context.Context, id uuid.UUID) (db.EventCategory, error) {
	c, ok := f.categories[id]
	if !ok {
		return db.EventCategory{}, pgx.ErrNoRows
	}
	return c, nil
}
func (f *fakeRepo) CountActiveReservationsByCategory(_ context.Context, categoryID uuid.UUID) (int64, error) {
	var n int64
	for _, rsv := range f.reservations {
		if rsv.CategoryID == categoryID && rsv.Status == ReservationActive {
			n++
		}
	}
	return n, nil
}
func (f *fakeRepo) CountPaidByCategory(_ context.Context, categoryID uuid.UUID) (int64, error) {
	var n int64
	for _, o := range f.orders {
		if o.CategoryID == categoryID && o.Status == StatusPaid {
			n++
		}
	}
	return n, nil
}
func (f *fakeRepo) CreateReservation(_ context.Context, arg db.CreateReservationParams) (db.InventoryReservation, error) {
	rsv := db.InventoryReservation{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID,
		CategoryID: arg.CategoryID, OrderID: arg.OrderID, ParticipantID: arg.ParticipantID,
		Status: ReservationActive, ExpiresAt: arg.ExpiresAt,
	}
	f.reservations[arg.OrderID] = rsv
	return rsv, nil
}
func (f *fakeRepo) ExpireReservationsForOrder(_ context.Context, orderID uuid.UUID) error {
	if rsv, ok := f.reservations[orderID]; ok && rsv.Status == ReservationActive {
		rsv.Status = ReservationExpired
		f.reservations[orderID] = rsv
	}
	return nil
}
func (f *fakeRepo) UpdateReservationStatusByOrder(_ context.Context, arg db.UpdateReservationStatusByOrderParams) error {
	if rsv, ok := f.reservations[arg.OrderID]; ok && rsv.Status == ReservationActive {
		rsv.Status = arg.Status
		f.reservations[arg.OrderID] = rsv
	}
	return nil
}

// no-op audit
type nopAudit struct{}

func (nopAudit) Record(context.Context, auditEntry) {}

func newSvc(repo Repository) *Service {
	return NewService(repo, nil, 15*time.Minute)
}

func TestCheckout_Success(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, cat := repo.seed(orgID, 100, 2)
	userID := uuid.New()

	order, err := svc.Checkout(context.Background(), userID, ev.ID, cat.ID)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if order.Status != StatusPendingPayment {
		t.Errorf("status = %q, want PENDING_PAYMENT", order.Status)
	}
	if order.Total != 100000 {
		t.Errorf("total = %d, want 100000", order.Total)
	}
	if _, ok := repo.reservations[order.ID]; !ok {
		t.Error("expected reservation created")
	}
}

func TestCheckout_SoldOut(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, cat := repo.seed(orgID, 1, 5)
	// first checkout takes the only slot
	if _, err := svc.Checkout(context.Background(), uuid.New(), ev.ID, cat.ID); err != nil {
		t.Fatalf("first checkout: %v", err)
	}
	// second → sold out
	if _, err := svc.Checkout(context.Background(), uuid.New(), ev.ID, cat.ID); err != inv.ErrSoldOut {
		t.Fatalf("err = %v, want ErrSoldOut", err)
	}
}

func TestCheckout_MaxOrderExceeded(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, cat := repo.seed(orgID, 100, 1)
	userID := uuid.New()
	if _, err := svc.Checkout(context.Background(), userID, ev.ID, cat.ID); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := svc.Checkout(context.Background(), userID, ev.ID, cat.ID); err != ErrMaxOrderExceeded {
		t.Fatalf("err = %v, want ErrMaxOrderExceeded", err)
	}
}

func TestCheckout_EventNotPublished(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, cat := repo.seed(orgID, 100, 5)
	ev.Status = "draft"
	repo.events[ev.ID] = ev
	if _, err := svc.Checkout(context.Background(), uuid.New(), ev.ID, cat.ID); err != ErrEventNotPublished {
		t.Fatalf("err = %v, want ErrEventNotPublished", err)
	}
}

func TestCancel_ReleasesReservation(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, cat := repo.seed(orgID, 100, 5)
	userID := uuid.New()
	order, _ := svc.Checkout(context.Background(), userID, ev.ID, cat.ID)

	if err := svc.Cancel(context.Background(), userID, order.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if o := repo.orders[order.ID]; o.Status != StatusCancelled {
		t.Errorf("status = %q, want CANCELLED", o.Status)
	}
	if rsv := repo.reservations[order.ID]; rsv.Status != ReservationReleased {
		t.Errorf("reservation = %q, want RELEASED", rsv.Status)
	}
}

func TestCancel_NotOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, cat := repo.seed(orgID, 100, 5)
	order, _ := svc.Checkout(context.Background(), uuid.New(), ev.ID, cat.ID)
	if err := svc.Cancel(context.Background(), uuid.New(), order.ID); err != ErrOrderNotFound {
		t.Fatalf("err = %v, want ErrOrderNotFound", err)
	}
}

func TestGet_NotOwnerIsNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, cat := repo.seed(orgID, 100, 5)
	order, _ := svc.Checkout(context.Background(), uuid.New(), ev.ID, cat.ID)
	if _, err := svc.GetForParticipant(context.Background(), uuid.New(), order.ID); err != ErrOrderNotFound {
		t.Fatalf("err = %v, want ErrOrderNotFound", err)
	}
}
```
Note: the test references `auditEntry` and a `Record(ctx, auditEntry)` interface. Define the audit seam in the service (Step 3) as `type AuditRecorder interface { Record(ctx, audit.Entry) }` and pass `nil` in tests (the service guards nil). **Adjust the test's `nopAudit`** — actually simplest: pass `nil` for the recorder, delete the `nopAudit`/`auditEntry` types from the test. Use `NewService(repo, nil, 15*time.Minute)`. Remove the `nopAudit` struct and `auditEntry` references when writing the file.

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/orders/ -run TestCheckout -v; cd ../..
```
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 4: Implement the service**

Create `services/api/internal/modules/orders/service.go`:
```go
package orders

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
)

// AuditRecorder records sensitive order actions (Phase 2 audit.Logger satisfies it).
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type Service struct {
	repo  Repository
	audit AuditRecorder
	ttl   time.Duration
}

func NewService(repo Repository, recorder AuditRecorder, ttl time.Duration) *Service {
	return &Service{repo: repo, audit: recorder, ttl: ttl}
}

// Checkout validates eligibility, reserves a slot, and creates a PENDING_PAYMENT
// order — all in one transaction with the category row locked FOR UPDATE.
func (s *Service) Checkout(ctx context.Context, participantID, eventID, categoryID uuid.UUID) (OrderResponse, error) {
	var created db.Order
	err := s.repo.ExecTx(ctx, func(tx Repository) error {
		event, err := tx.GetEventByID(ctx, eventID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCategoryNotFound
		} else if err != nil {
			return err
		}

		// Lock category + capacity check.
		check, err := inv.CheckAndLock(ctx, tx.Inventory(), categoryID)
		if errors.Is(err, inv.ErrCategory) {
			return ErrCategoryNotFound
		} else if err != nil {
			return err // includes inv.ErrSoldOut
		}
		cat := check.Category
		if cat.EventID != eventID {
			return ErrCategoryNotFound
		}

		now := time.Now()
		if err := checkoutEligible(event, cat, now); err != nil {
			return err
		}

		// max_order_per_user.
		activeCount, err := tx.CountActiveOrdersForUserCategory(ctx, db.CountActiveOrdersForUserCategoryParams{
			CategoryID: categoryID, ParticipantID: participantID,
		})
		if err != nil {
			return err
		}
		if activeCount >= int64(cat.MaxOrderPerUser) {
			return ErrMaxOrderExceeded
		}

		// Generate a unique order number (retry on collision).
		number, err := s.uniqueOrderNumber(ctx, tx, now)
		if err != nil {
			return err
		}

		expiresAt := now.Add(s.ttl)
		order, err := tx.CreateOrder(ctx, db.CreateOrderParams{
			OrganizationID: cat.OrganizationID, EventID: eventID, CategoryID: categoryID,
			ParticipantID: participantID, OrderNumber: number, Status: StatusPendingPayment,
			Subtotal: cat.Price, Fee: 0, Discount: 0, Total: cat.Price,
			ExpiredAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		})
		if err != nil {
			return err
		}
		created = order

		if _, err := inv.Reserve(ctx, tx.Inventory(), db.CreateReservationParams{
			OrganizationID: cat.OrganizationID, EventID: eventID, CategoryID: categoryID,
			OrderID: order.ID, ParticipantID: participantID,
			ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return OrderResponse{}, err
	}

	s.record(ctx, created, "ORDER_CREATED")
	s.recordReservation(ctx, created, "RESERVATION_CREATED")
	return toResponse(created), nil
}

func (s *Service) Cancel(ctx context.Context, participantID, orderID uuid.UUID) error {
	var cancelled db.Order
	err := s.repo.ExecTx(ctx, func(tx Repository) error {
		order, err := tx.GetOrderByID(ctx, orderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		} else if err != nil {
			return err
		}
		if order.ParticipantID != participantID {
			return ErrOrderNotFound // ownership: don't leak existence
		}
		if order.Status != StatusPendingPayment {
			return ErrInvalidState
		}
		updated, err := tx.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID: orderID, Status: StatusCancelled, Status_2: StatusPendingPayment,
		})
		if err != nil {
			return err
		}
		cancelled = updated
		return inv.Release(ctx, tx.Inventory(), orderID, ReservationReleased)
	})
	if err != nil {
		return err
	}
	s.record(ctx, cancelled, "ORDER_CANCELLED")
	return nil
}

func (s *Service) GetForParticipant(ctx context.Context, participantID, orderID uuid.UUID) (OrderResponse, error) {
	order, err := s.repo.GetOrderByID(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderResponse{}, ErrOrderNotFound
	} else if err != nil {
		return OrderResponse{}, err
	}
	if order.ParticipantID != participantID {
		return OrderResponse{}, ErrOrderNotFound
	}
	return toResponse(order), nil
}

func (s *Service) ListForParticipant(ctx context.Context, participantID uuid.UUID) ([]OrderResponse, error) {
	rows, err := s.repo.ListOrdersByParticipant(ctx, participantID)
	if err != nil {
		return nil, err
	}
	return toResponses(rows), nil
}

func (s *Service) ListForOrgEvent(ctx context.Context, orgID, eventID uuid.UUID) ([]OrderResponse, error) {
	rows, err := s.repo.ListOrdersByOrgEvent(ctx, db.ListOrdersByOrgEventParams{
		OrganizationID: orgID, EventID: eventID,
	})
	if err != nil {
		return nil, err
	}
	return toResponses(rows), nil
}

func (s *Service) uniqueOrderNumber(ctx context.Context, tx Repository, now time.Time) (string, error) {
	for i := 0; i < 5; i++ {
		num, err := generateOrderNumber(now)
		if err != nil {
			return "", ErrOrderNumberGen
		}
		_, err = tx.GetOrderByNumber(ctx, num)
		if errors.Is(err, pgx.ErrNoRows) {
			return num, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", ErrOrderNumberGen
}

func (s *Service) record(ctx context.Context, order db.Order, action string) {
	if s.audit == nil {
		return
	}
	oid := order.OrganizationID
	uid := order.ParticipantID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid, ActorUserID: &uid, Action: action,
		TargetType: "order", TargetID: order.ID.String(),
	})
}

func (s *Service) recordReservation(ctx context.Context, order db.Order, action string) {
	if s.audit == nil {
		return
	}
	oid := order.OrganizationID
	uid := order.ParticipantID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid, ActorUserID: &uid, Action: action,
		TargetType: "reservation", TargetID: order.ID.String(),
	})
}

func toResponse(o db.Order) OrderResponse {
	r := OrderResponse{
		ID: o.ID, OrderNumber: o.OrderNumber, EventID: o.EventID, CategoryID: o.CategoryID,
		Status: o.Status, Subtotal: o.Subtotal, Fee: o.Fee, Discount: o.Discount, Total: o.Total,
		CreatedAt: o.CreatedAt.Time,
	}
	if o.ExpiredAt.Valid {
		v := o.ExpiredAt.Time
		r.ExpiredAt = &v
	}
	return r
}

func toResponses(rows []db.Order) []OrderResponse {
	out := make([]OrderResponse, 0, len(rows))
	for _, o := range rows {
		out = append(out, toResponse(o))
	}
	return out
}
```
Note: `UpdateOrderStatusParams.Status_2` is the `status = $3` guard param (sqlc names the second `status` reference `Status_2`). **Verify the exact generated field name** in `orders.sql.go` — it may be `Status_2`. Adjust the fake and service consistently.

- [ ] **Step 5: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/modules/orders/ -v; cd ../..
```
Expected: PASS (all orders service tests). Remove `nopAudit`/`auditEntry` from the test file (use `nil` recorder).

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/orders/validator.go services/api/internal/modules/orders/service.go services/api/internal/modules/orders/service_test.go
git commit -m "feat(orders): add service with atomic checkout, cancel, and listing"
```

---

## Task 7: Orders handler, routes, and wiring

**Files:**
- Create: `services/api/internal/modules/orders/handler.go`
- Create: `services/api/internal/modules/orders/routes.go`
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Handler**

Create `services/api/internal/modules/orders/handler.go`:
```go
package orders

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func caller(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

// Checkout: POST /api/v1/events/{eventId}/categories/{categoryId}/checkout
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	categoryID, err := uuid.Parse(chi.URLParam(r, "categoryId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	order, err := h.svc.Checkout(r.Context(), userID, eventID, categoryID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, order)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(w, r)
	if !ok {
		return
	}
	orders, err := h.svc.ListForParticipant(r.Context(), userID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, orders)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(w, r)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	order, err := h.svc.GetForParticipant(r.Context(), userID, orderID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, order)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(w, r)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	if err := h.svc.Cancel(r.Context(), userID, orderID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// OrgList: GET /api/v1/organizations/{orgId}/events/{eventId}/orders
func (h *Handler) OrgList(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	orders, err := h.svc.ListForOrgEvent(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, orders)
}
```

- [ ] **Step 2: Routes**

Create `services/api/internal/modules/orders/routes.go`:
```go
package orders

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterParticipantRoutes mounts participant order endpoints (authn only, ownership
// enforced in service). Mount under the authenticated /api/v1 group.
func (h *Handler) RegisterParticipantRoutes(r chi.Router) {
	r.Post("/events/{eventId}/categories/{categoryId}/checkout", h.Checkout)
	r.Route("/orders", func(r chi.Router) {
		r.Get("/", h.List)
		r.Get("/{orderId}", h.Get)
		r.Delete("/{orderId}", h.Cancel)
	})
}

// RegisterOrgRoutes mounts the organizer order-list endpoint under
// /organizations/{orgId}/events/{eventId}, requiring order.view.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "order.view")).Get("/orders", h.OrgList)
}
```

- [ ] **Step 3: Wire into server.go**

In `services/api/internal/app/server.go` (additive only):

Add import:
```go
	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
```
Build the handler near the others:
```go
	orderHandler := ordersmod.NewHandler(ordersmod.NewService(ordersmod.NewRepository(pool), auditLog, cfg.OrderExpiration))
```
Inside the authenticated group (`r.Group(func(r chi.Router){ r.Use(middleware.Authn(signer)) ...})`):
- Add participant routes at the `/api/v1` authenticated level (NOT under `/organizations`). Place alongside `orgHandler.RegisterRoutes(r)`:
```go
			orderHandler.RegisterParticipantRoutes(r)
```
- Add the org list inside the existing per-org/event composition. The events module mounts sub-routes via `mountSubRoutes`; add `orderHandler.RegisterOrgRoutes` there:
```go
				eventHandler.RegisterRoutes(r, loader, func(r chi.Router) {
					categoryHandler.RegisterRoutes(r, loader)
					formHandler.RegisterRoutes(r, loader)
					orderHandler.RegisterOrgRoutes(r, loader)
				})
```
Note: `RegisterParticipantRoutes` mounts `/events/{eventId}/categories/{categoryId}/checkout` at the authenticated `/api/v1` level — this is a DIFFERENT path namespace from the organizer `/organizations/{orgId}/events/...`, so no conflict with Phase 3 event routes (those are under `/organizations`). Verify no route collision at boot.

- [ ] **Step 4: Build and full test**

Run:
```bash
cd services/api && go build ./... && go vet ./... && go test ./...; cd ../..
```
Expected: build OK; vet clean; all unit tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/orders/handler.go services/api/internal/modules/orders/routes.go services/api/internal/app/server.go
git commit -m "feat(orders): add handler, routes, and wire participant + org endpoints"
```

---
