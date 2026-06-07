# Phase 3 Plan — Part 3: Categories, Media, Public Catalog (Tasks 8-11)

> Part of the Phase 3 implementation plan. Index: [2026-06-07-phase3-event-category-management.md](2026-06-07-phase3-event-category-management.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **Depends on:** Parts 1-2 (storage, events module wired). Verify generated `db.EventCategory` types before writing repository/service.

---

## Task 8: Categories module (errors, dto, repository, service, handler, routes)

**Files:**
- Create: `services/api/internal/modules/categories/errors.go`
- Create: `services/api/internal/modules/categories/dto.go`
- Create: `services/api/internal/modules/categories/repository.go`
- Create: `services/api/internal/modules/categories/service.go`
- Test: `services/api/internal/modules/categories/service_test.go`
- Create: `services/api/internal/modules/categories/handler.go`
- Create: `services/api/internal/modules/categories/routes.go`
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Typed errors**

Create `services/api/internal/modules/categories/errors.go`:
```go
package categories

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrEventNotFound   = apperr.New(http.StatusNotFound, "EVENT_NOT_FOUND", "event not found")
	ErrNotFound        = apperr.New(http.StatusNotFound, "CATEGORY_NOT_FOUND", "category not found")
	ErrNameTaken       = apperr.New(http.StatusConflict, "CATEGORY_NAME_TAKEN", "a category with this name already exists for the event")
	ErrInvalidPrice    = apperr.New(http.StatusBadRequest, "INVALID_PRICE", "price must be >= 0")
	ErrInvalidCapacity = apperr.New(http.StatusBadRequest, "INVALID_CAPACITY", "capacity must be > 0")
	ErrInvalidWindow   = apperr.New(http.StatusBadRequest, "INVALID_REGISTRATION_WINDOW", "registration opens must be before closes")
	ErrInvalidAge      = apperr.New(http.StatusBadRequest, "INVALID_AGE", "min age must be >= 0")
	ErrInvalidMaxOrder = apperr.New(http.StatusBadRequest, "INVALID_MAX_ORDER", "max order per user must be >= 1")
)
```

- [ ] **Step 2: DTOs**

Create `services/api/internal/modules/categories/dto.go`:
```go
package categories

import (
	"time"

	"github.com/google/uuid"
)

type WriteRequest struct {
	Name                 string    `json:"name"`
	Price                int64     `json:"price"`
	Capacity             int32     `json:"capacity"`
	RegistrationOpensAt  time.Time `json:"registrationOpensAt"`
	RegistrationClosesAt time.Time `json:"registrationClosesAt"`
	BibPrefix            string    `json:"bibPrefix"`
	MinAge               *int32    `json:"minAge"`
	MaxOrderPerUser      int32     `json:"maxOrderPerUser"`
}

type Response struct {
	ID                   uuid.UUID `json:"id"`
	EventID              uuid.UUID `json:"eventId"`
	Name                 string    `json:"name"`
	Price                int64     `json:"price"`
	Capacity             int32     `json:"capacity"`
	RegistrationOpensAt  time.Time `json:"registrationOpensAt"`
	RegistrationClosesAt time.Time `json:"registrationClosesAt"`
	BibPrefix            string    `json:"bibPrefix"`
	MinAge               *int32    `json:"minAge"`
	MaxOrderPerUser      int32     `json:"maxOrderPerUser"`
	CreatedAt            time.Time `json:"createdAt"`
}
```

- [ ] **Step 3: Repository interface + adapter**

Create `services/api/internal/modules/categories/repository.go`:
```go
package categories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	CreateCategory(ctx context.Context, arg db.CreateCategoryParams) (db.EventCategory, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	ListCategoriesByEvent(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error)
	UpdateCategory(ctx context.Context, arg db.UpdateCategoryParams) (db.EventCategory, error)
	DeleteCategory(ctx context.Context, arg db.DeleteCategoryParams) error
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}
func (r *sqlcRepo) CreateCategory(ctx context.Context, arg db.CreateCategoryParams) (db.EventCategory, error) {
	return r.q.CreateCategory(ctx, arg)
}
func (r *sqlcRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return r.q.GetCategoryByID(ctx, id)
}
func (r *sqlcRepo) ListCategoriesByEvent(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error) {
	return r.q.ListCategoriesByEvent(ctx, eventID)
}
func (r *sqlcRepo) UpdateCategory(ctx context.Context, arg db.UpdateCategoryParams) (db.EventCategory, error) {
	return r.q.UpdateCategory(ctx, arg)
}
func (r *sqlcRepo) DeleteCategory(ctx context.Context, arg db.DeleteCategoryParams) error {
	return r.q.DeleteCategory(ctx, arg)
}
```

- [ ] **Step 4: Write failing service tests**

Create `services/api/internal/modules/categories/service_test.go`:
```go
package categories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type fakeRepo struct {
	events     map[uuid.UUID]db.Event
	categories map[uuid.UUID]db.EventCategory
	names      map[string]bool // eventID|name
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		events:     map[uuid.UUID]db.Event{},
		categories: map[uuid.UUID]db.EventCategory{},
		names:      map[string]bool{},
	}
}

func (f *fakeRepo) seedEvent(orgID uuid.UUID) db.Event {
	e := db.Event{ID: uuid.New(), OrganizationID: orgID, Name: "E", Slug: "e", Status: "draft"}
	f.events[e.ID] = e
	return e
}

func (f *fakeRepo) GetEventByID(_ context.Context, id uuid.UUID) (db.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return db.Event{}, pgx.ErrNoRows
	}
	return e, nil
}
func (f *fakeRepo) CreateCategory(_ context.Context, arg db.CreateCategoryParams) (db.EventCategory, error) {
	c := db.EventCategory{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID,
		Name: arg.Name, Price: arg.Price, Capacity: arg.Capacity,
		RegistrationOpensAt: arg.RegistrationOpensAt, RegistrationClosesAt: arg.RegistrationClosesAt,
		MaxOrderPerUser: arg.MaxOrderPerUser,
	}
	f.categories[c.ID] = c
	f.names[arg.EventID.String()+"|"+arg.Name] = true
	return c, nil
}
func (f *fakeRepo) GetCategoryByID(_ context.Context, id uuid.UUID) (db.EventCategory, error) {
	c, ok := f.categories[id]
	if !ok {
		return db.EventCategory{}, pgx.ErrNoRows
	}
	return c, nil
}
func (f *fakeRepo) ListCategoriesByEvent(_ context.Context, eventID uuid.UUID) ([]db.EventCategory, error) {
	var out []db.EventCategory
	for _, c := range f.categories {
		if c.EventID == eventID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeRepo) UpdateCategory(_ context.Context, arg db.UpdateCategoryParams) (db.EventCategory, error) {
	c := f.categories[arg.ID]
	c.Name = arg.Name
	c.Price = arg.Price
	c.Capacity = arg.Capacity
	f.categories[arg.ID] = c
	return c, nil
}
func (f *fakeRepo) DeleteCategory(_ context.Context, arg db.DeleteCategoryParams) error {
	delete(f.categories, arg.ID)
	return nil
}

func validReq() WriteRequest {
	return WriteRequest{
		Name: "42K", Price: 350000, Capacity: 2000,
		RegistrationOpensAt:  time.Now(),
		RegistrationClosesAt: time.Now().Add(720 * time.Hour),
		MaxOrderPerUser:      1,
	}
}

func TestCreate_Valid(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	got, err := svc.Create(context.Background(), orgID, ev.ID, validReq())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.Name != "42K" || got.Price != 350000 {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestCreate_RejectsBadPrice(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	req := validReq()
	req.Price = -1
	if _, err := svc.Create(context.Background(), orgID, ev.ID, req); err != ErrInvalidPrice {
		t.Fatalf("err = %v, want ErrInvalidPrice", err)
	}
}

func TestCreate_RejectsBadCapacity(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	req := validReq()
	req.Capacity = 0
	if _, err := svc.Create(context.Background(), orgID, ev.ID, req); err != ErrInvalidCapacity {
		t.Fatalf("err = %v, want ErrInvalidCapacity", err)
	}
}

func TestCreate_RejectsBadWindow(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	req := validReq()
	req.RegistrationClosesAt = req.RegistrationOpensAt.Add(-time.Hour)
	if _, err := svc.Create(context.Background(), orgID, ev.ID, req); err != ErrInvalidWindow {
		t.Fatalf("err = %v, want ErrInvalidWindow", err)
	}
}

func TestCreate_RejectsBadMaxOrder(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	req := validReq()
	req.MaxOrderPerUser = 0
	if _, err := svc.Create(context.Background(), orgID, ev.ID, req); err != ErrInvalidMaxOrder {
		t.Fatalf("err = %v, want ErrInvalidMaxOrder", err)
	}
}

func TestCreate_TenantMismatchEventNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgA := uuid.New()
	ev := repo.seedEvent(orgA)
	orgB := uuid.New()
	if _, err := svc.Create(context.Background(), orgB, ev.ID, validReq()); err != ErrEventNotFound {
		t.Fatalf("err = %v, want ErrEventNotFound", err)
	}
}

func TestDelete_TenantMismatch(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgA := uuid.New()
	ev := repo.seedEvent(orgA)
	cat, _ := svc.Create(context.Background(), orgA, ev.ID, validReq())
	orgB := uuid.New()
	if err := svc.Delete(context.Background(), orgB, ev.ID, cat.ID); err != ErrEventNotFound {
		t.Fatalf("err = %v, want ErrEventNotFound", err)
	}
}

var _ = pgtype.Timestamptz{}
```

- [ ] **Step 5: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/categories/ -v; cd ../..
```
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 6: Implement the service**

Create `services/api/internal/modules/categories/service.go`:
```go
package categories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func validate(req WriteRequest) error {
	if req.Price < 0 {
		return ErrInvalidPrice
	}
	if req.Capacity <= 0 {
		return ErrInvalidCapacity
	}
	if !req.RegistrationOpensAt.Before(req.RegistrationClosesAt) {
		return ErrInvalidWindow
	}
	if req.MinAge != nil && *req.MinAge < 0 {
		return ErrInvalidAge
	}
	if req.MaxOrderPerUser < 1 {
		return ErrInvalidMaxOrder
	}
	return nil
}

// assertEvent confirms the event exists and belongs to orgID (tenant guard).
func (s *Service) assertEvent(ctx context.Context, orgID, eventID uuid.UUID) error {
	e, err := s.repo.GetEventByID(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEventNotFound
	} else if err != nil {
		return err
	}
	if e.OrganizationID != orgID {
		return ErrEventNotFound
	}
	return nil
}

func (s *Service) Create(ctx context.Context, orgID, eventID uuid.UUID, req WriteRequest) (Response, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return Response{}, err
	}
	if err := validate(req); err != nil {
		return Response{}, err
	}
	c, err := s.repo.CreateCategory(ctx, db.CreateCategoryParams{
		OrganizationID:       orgID,
		EventID:              eventID,
		Name:                 req.Name,
		Price:                req.Price,
		Capacity:             req.Capacity,
		RegistrationOpensAt:  pgtype.Timestamptz{Time: req.RegistrationOpensAt, Valid: true},
		RegistrationClosesAt: pgtype.Timestamptz{Time: req.RegistrationClosesAt, Valid: true},
		BibPrefix:            nullText(req.BibPrefix),
		MinAge:               nullInt4(req.MinAge),
		MaxOrderPerUser:      req.MaxOrderPerUser,
	})
	if err != nil {
		return Response{}, err
	}
	return toResponse(c), nil
}

func (s *Service) List(ctx context.Context, orgID, eventID uuid.UUID) ([]Response, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListCategoriesByEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]Response, 0, len(rows))
	for _, c := range rows {
		out = append(out, toResponse(c))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, orgID, eventID, categoryID uuid.UUID) (Response, error) {
	c, err := s.loadCategory(ctx, orgID, eventID, categoryID)
	if err != nil {
		return Response{}, err
	}
	return toResponse(c), nil
}

func (s *Service) Update(ctx context.Context, orgID, eventID, categoryID uuid.UUID, req WriteRequest) (Response, error) {
	if _, err := s.loadCategory(ctx, orgID, eventID, categoryID); err != nil {
		return Response{}, err
	}
	if err := validate(req); err != nil {
		return Response{}, err
	}
	c, err := s.repo.UpdateCategory(ctx, db.UpdateCategoryParams{
		ID:                   categoryID,
		Name:                 req.Name,
		Price:                req.Price,
		Capacity:             req.Capacity,
		RegistrationOpensAt:  pgtype.Timestamptz{Time: req.RegistrationOpensAt, Valid: true},
		RegistrationClosesAt: pgtype.Timestamptz{Time: req.RegistrationClosesAt, Valid: true},
		BibPrefix:            nullText(req.BibPrefix),
		MinAge:               nullInt4(req.MinAge),
		MaxOrderPerUser:      req.MaxOrderPerUser,
		EventID:              eventID,
	})
	if err != nil {
		return Response{}, err
	}
	return toResponse(c), nil
}

func (s *Service) Delete(ctx context.Context, orgID, eventID, categoryID uuid.UUID) error {
	if _, err := s.loadCategory(ctx, orgID, eventID, categoryID); err != nil {
		return err
	}
	return s.repo.DeleteCategory(ctx, db.DeleteCategoryParams{ID: categoryID, EventID: eventID})
}

// loadCategory confirms the category belongs to the event AND the event to the org.
func (s *Service) loadCategory(ctx context.Context, orgID, eventID, categoryID uuid.UUID) (db.EventCategory, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return db.EventCategory{}, err
	}
	c, err := s.repo.GetCategoryByID(ctx, categoryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.EventCategory{}, ErrNotFound
	} else if err != nil {
		return db.EventCategory{}, err
	}
	if c.EventID != eventID || c.OrganizationID != orgID {
		return db.EventCategory{}, ErrNotFound
	}
	return c, nil
}

func toResponse(c db.EventCategory) Response {
	r := Response{
		ID: c.ID, EventID: c.EventID, Name: c.Name, Price: c.Price, Capacity: c.Capacity,
		RegistrationOpensAt:  c.RegistrationOpensAt.Time,
		RegistrationClosesAt: c.RegistrationClosesAt.Time,
		BibPrefix:            c.BibPrefix.String,
		MaxOrderPerUser:      c.MaxOrderPerUser,
		CreatedAt:            c.CreatedAt.Time,
	}
	if c.MinAge.Valid {
		v := c.MinAge.Int32
		r.MinAge = &v
	}
	return r
}

func nullText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func nullInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

var _ = time.Now
```
Note: confirm `db.EventCategory` field types: `Price int64`, `Capacity int32`, `MaxOrderPerUser int32`, `MinAge pgtype.Int4`, `BibPrefix pgtype.Text`, `RegistrationOpensAt pgtype.Timestamptz`. If sqlc generated `int` instead of `int32` for `integer` columns, adjust DTO + helpers. Remove `var _ = time.Now` if unused.

- [ ] **Step 7: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/modules/categories/ -v; cd ../..
```
Expected: PASS (all category service tests).

- [ ] **Step 8: Handler**

Create `services/api/internal/modules/categories/handler.go`:
```go
package categories

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ids(w http.ResponseWriter, r *http.Request) (orgID, eventID uuid.UUID, ok bool) {
	var err error
	orgID, err = uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return uuid.Nil, uuid.Nil, false
	}
	eventID, err = uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, eventID, true
}

func (h *Handler) categoryID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "categoryId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	out, err := h.svc.List(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name is required"))
		return
	}
	c, err := h.svc.Create(r.Context(), orgID, eventID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, c)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	categoryID, ok := h.categoryID(w, r)
	if !ok {
		return
	}
	c, err := h.svc.Get(r.Context(), orgID, eventID, categoryID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	categoryID, ok := h.categoryID(w, r)
	if !ok {
		return
	}
	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name is required"))
		return
	}
	c, err := h.svc.Update(r.Context(), orgID, eventID, categoryID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	categoryID, ok := h.categoryID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), orgID, eventID, categoryID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 9: Routes**

Create `services/api/internal/modules/categories/routes.go`:
```go
package categories

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts category endpoints under an existing
// /organizations/{orgId}/events/{eventId} router (already behind authn).
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/categories", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "category.manage"))
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{categoryId}", h.Get)
		r.Put("/{categoryId}", h.Update)
		r.Delete("/{categoryId}", h.Delete)
	})
}
```

- [ ] **Step 10: Wire into server.go**

In `services/api/internal/app/server.go`, add import `categoriesmod "github.com/varin/ivyticketing/services/api/internal/modules/categories"`, build the handler:
```go
	categoryHandler := categoriesmod.NewHandler(categoriesmod.NewService(categoriesmod.NewRepository(pool)))
```
Mount under the event sub-router. **Restructure** the per-org block so categories nest under `/events/{eventId}`:
```go
			r.Route("/organizations/{orgId}", func(r chi.Router) {
				memberHandler.RegisterRoutes(r, loader)
				roleHandler.RegisterRoutes(r, loader)
				eventHandler.RegisterRoutes(r, loader)
				r.Route("/events/{eventId}", func(r chi.Router) {
					categoryHandler.RegisterRoutes(r, loader)
				})
			})
```
Note: `eventHandler.RegisterRoutes` already defines `/events` and `/events/{eventId}` sub-paths; chi merges this additional `/events/{eventId}` group for categories. If chi reports a conflict, move ALL event sub-routes (including categories + media) into a single `r.Route("/events", ...)` composition — but the merge usually works. Verify in Step 11.

- [ ] **Step 11: Build and test**

Run:
```bash
cd services/api && go build ./... && go test ./...; cd ../..
```
Expected: build OK; all tests PASS.

- [ ] **Step 12: Commit**

```bash
git add services/api/internal/modules/categories services/api/internal/app/server.go
git commit -m "feat(categories): add categories module with validation and wire routes"
```

---

## Task 9: Event media upload flow

**Files:**
- Create: `services/api/internal/modules/events/media.go`
- Test: `services/api/internal/modules/events/media_test.go`
- Modify: `services/api/internal/modules/events/routes.go`

The flow: request ticket → (cloud: client PUTs directly; local: client POSTs multipart to API) → confirm sets the object key. Confirm validates the key prefix to prevent tampering.

- [ ] **Step 1: Write the failing test for key validation**

Create `services/api/internal/modules/events/media_test.go`:
```go
package events

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateObjectKey(t *testing.T) {
	orgID := uuid.New()
	eventID := uuid.New()
	good := mediaKeyPrefix(orgID, eventID, "banner") + "abc.png"
	if err := validateObjectKey(good, orgID, eventID, "banner"); err != nil {
		t.Errorf("good key rejected: %v", err)
	}
	// wrong event
	otherEvent := uuid.New()
	bad := mediaKeyPrefix(orgID, otherEvent, "banner") + "abc.png"
	if err := validateObjectKey(bad, orgID, eventID, "banner"); err == nil {
		t.Error("key for another event should be rejected")
	}
	// traversal attempt
	if err := validateObjectKey("../../etc/passwd", orgID, eventID, "banner"); err == nil {
		t.Error("traversal key should be rejected")
	}
}

func TestValidExtension(t *testing.T) {
	if !validImageContentType("image/png") || !validImageContentType("image/jpeg") || !validImageContentType("image/webp") {
		t.Error("standard image types should be allowed")
	}
	if validImageContentType("application/pdf") {
		t.Error("pdf should be rejected")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/events/ -run 'TestValidateObjectKey|TestValidExtension' -v; cd ../..
```
Expected: FAIL — `undefined: mediaKeyPrefix`.

- [ ] **Step 3: Implement media helpers + handlers**

Create `services/api/internal/modules/events/media.go`:
```go
package events

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

const presignTTL = 10 * time.Minute

var imageExt = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

func validImageContentType(ct string) bool {
	_, ok := imageExt[ct]
	return ok
}

func mediaKeyPrefix(orgID, eventID uuid.UUID, kind string) string {
	return "org/" + orgID.String() + "/event/" + eventID.String() + "/" + kind + "/"
}

func validateObjectKey(key string, orgID, eventID uuid.UUID, kind string) error {
	if strings.Contains(key, "..") {
		return ErrInvalidObjectKey
	}
	if !strings.HasPrefix(key, mediaKeyPrefix(orgID, eventID, kind)) {
		return ErrInvalidObjectKey
	}
	return nil
}

type ticketRequest struct {
	ContentType string `json:"contentType"`
	FileName    string `json:"fileName"`
}

type confirmRequest struct {
	ObjectKey string `json:"objectKey"`
}

// RequestTicket issues an upload ticket (presigned for cloud, direct for local).
func (h *Handler) RequestTicket(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	kind := chi.URLParam(r, "kind")
	if kind != "banner" && kind != "logo" {
		apperr.WriteError(w, r, ErrInvalidMediaKind)
		return
	}
	var req ticketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	ext, ok := imageExt[req.ContentType]
	if !ok {
		apperr.WriteError(w, r, ErrInvalidContent)
		return
	}
	if _, err := h.svc.Get(r.Context(), orgID, eventID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}

	objectKey := mediaKeyPrefix(orgID, eventID, kind) + uuid.NewString() + "." + ext
	ticket, presigned, err := h.svc.store.PresignUpload(r.Context(), objectKey, req.ContentType, presignTTL)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	if presigned {
		apperr.WriteJSON(w, http.StatusOK, map[string]any{
			"mode": "presigned", "objectKey": objectKey, "upload": ticket,
		})
		return
	}
	// local: client POSTs multipart to the upload URL
	apperr.WriteJSON(w, http.StatusOK, map[string]any{
		"mode":      "direct",
		"objectKey": objectKey,
		"uploadUrl": "/api/v1/organizations/" + orgID.String() + "/events/" + eventID.String() + "/media/" + kind + "/upload?key=" + objectKey,
	})
}

// UploadDirect is the local-only multipart sink.
func (h *Handler) UploadDirect(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	kind := chi.URLParam(r, "kind")
	key := r.URL.Query().Get("key")
	if err := validateObjectKey(key, orgID, eventID, kind); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)
	if err := r.ParseMultipartForm(h.maxUploadBytes); err != nil {
		apperr.WriteError(w, r, ErrFileTooLarge)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "missing file field"))
		return
	}
	defer file.Close()
	if !validImageContentType(header.Header.Get("Content-Type")) {
		apperr.WriteError(w, r, ErrInvalidContent)
		return
	}
	if err := h.svc.store.Put(r.Context(), key, file, header.Header.Get("Content-Type")); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ConfirmMedia validates the key and persists it onto the event.
func (h *Handler) ConfirmMedia(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	kind := chi.URLParam(r, "kind")
	if kind != "banner" && kind != "logo" {
		apperr.WriteError(w, r, ErrInvalidMediaKind)
		return
	}
	var req confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	if err := validateObjectKey(req.ObjectKey, orgID, eventID, kind); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	ev, err := h.svc.SetMedia(r.Context(), orgID, eventID, kind, req.ObjectKey)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}

var _ = errors.Is
var _ = db.SetEventMediaKeyParams{}
var _ = pgtype.Text{}
```
Note: remove the trailing `var _ =` lines if unused. `h.maxUploadBytes` is a new field on `Handler` — add it (see Step 5).

- [ ] **Step 4: Add `SetMedia` to the events service**

Add to `services/api/internal/modules/events/service.go`:
```go
func (s *Service) SetMedia(ctx context.Context, orgID, eventID uuid.UUID, kind, objectKey string) (Response, error) {
	if _, err := s.loadOrgEvent(ctx, orgID, eventID); err != nil {
		return Response{}, err
	}
	arg := db.SetEventMediaKeyParams{ID: eventID, OrganizationID: orgID}
	if kind == "banner" {
		arg.BannerObjectKey = pgtype.Text{String: objectKey, Valid: true}
	} else {
		arg.LogoObjectKey = pgtype.Text{String: objectKey, Valid: true}
	}
	e, err := s.repo.SetEventMediaKey(ctx, arg)
	if err != nil {
		return Response{}, err
	}
	return s.toResponse(e), nil
}
```
Note: `SetEventMediaKey` uses `COALESCE` so the unset kind stays nil; passing `Valid:false` for the other field leaves it unchanged.

- [ ] **Step 5: Add maxUploadBytes to Handler**

In `services/api/internal/modules/events/handler.go`, change `Handler` and `NewHandler`:
```go
type Handler struct {
	svc            *Service
	maxUploadBytes int64
}

func NewHandler(svc *Service, maxUploadBytes int64) *Handler {
	return &Handler{svc: svc, maxUploadBytes: maxUploadBytes}
}
```
The service's `store` field is accessed from `media.go` (`h.svc.store`) — confirm `store` is a field on `Service` (it is, from Part 2 Task 6). Since `media.go` is in the same package, unexported access is fine.

- [ ] **Step 6: Add media routes**

In `services/api/internal/modules/events/routes.go`, add inside the `/events` route block (within the `{eventId}` paths):
```go
		r.With(middleware.RequirePermission(loader, "event.edit")).Post("/{eventId}/media/{kind}", h.RequestTicket)
		r.With(middleware.RequirePermission(loader, "event.edit")).Post("/{eventId}/media/{kind}/upload", h.UploadDirect)
		r.With(middleware.RequirePermission(loader, "event.edit")).Put("/{eventId}/media/{kind}/confirm", h.ConfirmMedia)
```

- [ ] **Step 7: Update server.go for new NewHandler signature**

In `services/api/internal/app/server.go`, update:
```go
	eventHandler := eventsmod.NewHandler(
		eventsmod.NewService(eventsmod.NewRepository(pool), store, auditLog),
		cfg.StorageUploadMaxBytes,
	)
```

- [ ] **Step 8: Run tests and build**

Run:
```bash
cd services/api && go test ./internal/modules/events/ -v && go build ./...; cd ../..
```
Expected: PASS (media + earlier event tests); build OK.

- [ ] **Step 9: Commit**

```bash
git add services/api/internal/modules/events services/api/internal/app/server.go
git commit -m "feat(events): add media upload flow (ticket, direct upload, confirm)"
```

---

## Task 10: Public catalog module

**Files:**
- Create: `services/api/internal/modules/publiccatalog/dto.go`
- Create: `services/api/internal/modules/publiccatalog/repository.go`
- Create: `services/api/internal/modules/publiccatalog/service.go`
- Create: `services/api/internal/modules/publiccatalog/handler.go`
- Create: `services/api/internal/modules/publiccatalog/routes.go`
- Test: `services/api/internal/modules/publiccatalog/service_test.go`
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Repository queries (already added in Part 1 Task 4 Step 3)**

`ListPublishedEventsByOrgSlug` and `GetPublishedEventByOrgAndSlug` are already in `database/queries/events.sql`. Also need published categories. Append to `database/queries/event_categories.sql`:
```sql
-- name: ListCategoriesByEventForPublic :many
SELECT * FROM event_categories WHERE event_id = $1 ORDER BY price;
```
Then regenerate:
```bash
make sqlc && cd services/api && go build ./...; cd ../..
```
Expected: new query methods generated.

- [ ] **Step 2: DTOs**

Create `services/api/internal/modules/publiccatalog/dto.go`:
```go
package publiccatalog

import (
	"time"

	"github.com/google/uuid"
)

type EventResponse struct {
	ID          uuid.UUID          `json:"id"`
	Name        string             `json:"name"`
	Slug        string             `json:"slug"`
	EventType   string             `json:"eventType"`
	Description string             `json:"description"`
	BannerURL   string             `json:"bannerUrl"`
	LogoURL     string             `json:"logoUrl"`
	VenueName   string             `json:"venueName"`
	StartsAt    *time.Time         `json:"startsAt"`
	EndsAt      *time.Time         `json:"endsAt"`
	Categories  []CategoryResponse `json:"categories,omitempty"`
}

type CategoryResponse struct {
	ID                   uuid.UUID `json:"id"`
	Name                 string    `json:"name"`
	Price                int64     `json:"price"`
	RegistrationOpensAt  time.Time `json:"registrationOpensAt"`
	RegistrationClosesAt time.Time `json:"registrationClosesAt"`
}
```

- [ ] **Step 3: Repository**

Create `services/api/internal/modules/publiccatalog/repository.go`:
```go
package publiccatalog

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ListPublishedEventsByOrgSlug(ctx context.Context, slug string) ([]db.Event, error)
	GetPublishedEventByOrgAndSlug(ctx context.Context, arg db.GetPublishedEventByOrgAndSlugParams) (db.Event, error)
	ListCategoriesByEventForPublic(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) ListPublishedEventsByOrgSlug(ctx context.Context, slug string) ([]db.Event, error) {
	return r.q.ListPublishedEventsByOrgSlug(ctx, slug)
}
func (r *sqlcRepo) GetPublishedEventByOrgAndSlug(ctx context.Context, arg db.GetPublishedEventByOrgAndSlugParams) (db.Event, error) {
	return r.q.GetPublishedEventByOrgAndSlug(ctx, arg)
}
func (r *sqlcRepo) ListCategoriesByEventForPublic(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error) {
	return r.q.ListCategoriesByEventForPublic(ctx, eventID)
}
```

- [ ] **Step 4: Write failing service test**

Create `services/api/internal/modules/publiccatalog/service_test.go`:
```go
package publiccatalog

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type fakeRepo struct {
	events     []db.Event
	byNotFound bool
	categories map[uuid.UUID][]db.EventCategory
}

func (f *fakeRepo) ListPublishedEventsByOrgSlug(_ context.Context, _ string) ([]db.Event, error) {
	return f.events, nil
}
func (f *fakeRepo) GetPublishedEventByOrgAndSlug(_ context.Context, _ db.GetPublishedEventByOrgAndSlugParams) (db.Event, error) {
	if f.byNotFound {
		return db.Event{}, pgx.ErrNoRows
	}
	return f.events[0], nil
}
func (f *fakeRepo) ListCategoriesByEventForPublic(_ context.Context, eventID uuid.UUID) ([]db.EventCategory, error) {
	return f.categories[eventID], nil
}

type fakeStore struct{}

func (fakeStore) PublicURL(key string) string { return "http://cdn/" + key }

func TestListEvents(t *testing.T) {
	repo := &fakeRepo{events: []db.Event{{ID: uuid.New(), Name: "E", Slug: "e", Status: "published"}}}
	svc := NewService(repo, fakeStore{})
	out, err := svc.ListEvents(context.Background(), "org")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 1 || out[0].Slug != "e" {
		t.Errorf("unexpected: %+v", out)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	repo := &fakeRepo{byNotFound: true}
	svc := NewService(repo, fakeStore{})
	if _, err := svc.GetEvent(context.Background(), "org", "missing"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 5: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/publiccatalog/ -v; cd ../..
```
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 6: Implement service + errors**

Create `services/api/internal/modules/publiccatalog/service.go`:
```go
package publiccatalog

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var ErrNotFound = apperr.New(http.StatusNotFound, "EVENT_NOT_FOUND", "event not found")

// urlBuilder is the subset of storage.Storage this module needs.
type urlBuilder interface {
	PublicURL(key string) string
}

type Service struct {
	repo  Repository
	store urlBuilder
}

func NewService(repo Repository, store urlBuilder) *Service {
	return &Service{repo: repo, store: store}
}

func (s *Service) ListEvents(ctx context.Context, orgSlug string) ([]EventResponse, error) {
	rows, err := s.repo.ListPublishedEventsByOrgSlug(ctx, orgSlug)
	if err != nil {
		return nil, err
	}
	out := make([]EventResponse, 0, len(rows))
	for _, e := range rows {
		out = append(out, s.toEventResponse(e, nil))
	}
	return out, nil
}

func (s *Service) GetEvent(ctx context.Context, orgSlug, eventSlug string) (EventResponse, error) {
	e, err := s.repo.GetPublishedEventByOrgAndSlug(ctx, db.GetPublishedEventByOrgAndSlugParams{Slug: orgSlug, Slug_2: eventSlug})
	if errors.Is(err, pgx.ErrNoRows) {
		return EventResponse{}, ErrNotFound
	} else if err != nil {
		return EventResponse{}, err
	}
	cats, err := s.repo.ListCategoriesByEventForPublic(ctx, e.ID)
	if err != nil {
		return EventResponse{}, err
	}
	return s.toEventResponse(e, cats), nil
}

func (s *Service) toEventResponse(e db.Event, cats []db.EventCategory) EventResponse {
	r := EventResponse{
		ID: e.ID, Name: e.Name, Slug: e.Slug, EventType: e.EventType,
		Description: e.Description.String,
		VenueName:   e.VenueName.String,
		StartsAt:    tptr(e.StartsAt), EndsAt: tptr(e.EndsAt),
	}
	if e.BannerObjectKey.Valid {
		r.BannerURL = s.store.PublicURL(e.BannerObjectKey.String)
	}
	if e.LogoObjectKey.Valid {
		r.LogoURL = s.store.PublicURL(e.LogoObjectKey.String)
	}
	for _, c := range cats {
		r.Categories = append(r.Categories, CategoryResponse{
			ID: c.ID, Name: c.Name, Price: c.Price,
			RegistrationOpensAt:  c.RegistrationOpensAt.Time,
			RegistrationClosesAt: c.RegistrationClosesAt.Time,
		})
	}
	return r
}

func tptr(t pgtypeTimestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
```
**Correction:** `tptr` must accept `pgtype.Timestamptz`. Replace the bogus `pgtypeTimestamptz` with the real import. Add `"github.com/jackc/pgx/v5/pgtype"` to imports and change the signature to `func tptr(t pgtype.Timestamptz) *time.Time`.

Note: `GetPublishedEventByOrgAndSlugParams` field names — sqlc names duplicate `$1`/`$2` text params. Two `slug` columns (`o.slug`, `e.slug`) likely generate `Slug` and `Slug_2`. **Verify the exact generated field names** in `events.sql.go` and adjust the struct literal. If sqlc named them differently (e.g. by column), match that.

- [ ] **Step 7: Handler + routes**

Create `services/api/internal/modules/publiccatalog/handler.go`:
```go
package publiccatalog

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	orgSlug := chi.URLParam(r, "orgSlug")
	out, err := h.svc.ListEvents(r.Context(), orgSlug)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	orgSlug := chi.URLParam(r, "orgSlug")
	eventSlug := chi.URLParam(r, "eventSlug")
	ev, err := h.svc.GetEvent(r.Context(), orgSlug, eventSlug)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}
```

Create `services/api/internal/modules/publiccatalog/routes.go`:
```go
package publiccatalog

import "github.com/go-chi/chi/v5"

// RegisterRoutes mounts public read-only catalog endpoints. No auth.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/public/organizations/{orgSlug}/events", func(r chi.Router) {
		r.Get("/", h.ListEvents)
		r.Get("/{eventSlug}", h.GetEvent)
	})
}
```

- [ ] **Step 8: Run tests**

Run:
```bash
cd services/api && go test ./internal/modules/publiccatalog/ -v; cd ../..
```
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add database/queries services/api/internal/db services/api/internal/modules/publiccatalog
git commit -m "feat(public): add public catalog module for published events"
```

---

## Task 11: Mount public catalog + static media route

**Files:**
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Mount public catalog under /api/v1 (no authn)**

In `services/api/internal/app/server.go`, add import `publicmod "github.com/varin/ivyticketing/services/api/internal/modules/publiccatalog"`, build handler:
```go
	publicHandler := publicmod.NewHandler(publicmod.NewService(publicmod.NewRepository(pool), store))
```
Inside `r.Route("/api/v1", ...)`, BEFORE the authenticated group, mount:
```go
		// Public read-only (no auth).
		publicHandler.RegisterRoutes(r)
```

- [ ] **Step 2: Serve local media files (driver local only)**

When `cfg.StorageDriver == "local"`, serve files from `STORAGE_LOCAL_PATH` at `/media/`. After building the router (top level, not under /api/v1), add:
```go
	if cfg.StorageDriver == "local" {
		fs := http.StripPrefix("/media/", http.FileServer(http.Dir(cfg.StorageLocalPath)))
		r.Get("/media/*", fs.ServeHTTP)
	}
```
Note: `http.FileServer` already prevents path traversal above its root. This serves only when driver is local; cloud serves via CDN/bucket URL directly.

- [ ] **Step 3: Build and full test**

Run:
```bash
cd services/api && go build ./... && go vet ./... && go test ./...; cd ../..
```
Expected: build OK; vet clean; all unit tests PASS. If chi reports a route conflict on `/events/{eventId}`, consolidate the event + category + media sub-routes into one composition (see Task 8 Step 10 note).

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/app/server.go
git commit -m "feat(api): mount public catalog and local media file server"
```

---
