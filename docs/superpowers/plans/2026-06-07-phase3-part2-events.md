# Phase 3 Plan — Part 2: Events Module (Tasks 5-7)

> Part of the Phase 3 implementation plan. Index: [2026-06-07-phase3-event-category-management.md](2026-06-07-phase3-event-category-management.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **Depends on:** Part 1 (storage, migrations, sqlc queries). Verify generated `db.Event`/`db.EventCategory` types before writing repository/service.

---

## Task 5: Events errors, DTOs, repository

**Files:**
- Create: `services/api/internal/modules/events/errors.go`
- Create: `services/api/internal/modules/events/dto.go`
- Create: `services/api/internal/modules/events/slug.go`
- Test: `services/api/internal/modules/events/slug_test.go`
- Create: `services/api/internal/modules/events/repository.go`

- [ ] **Step 1: Typed errors**

Create `services/api/internal/modules/events/errors.go`:
```go
package events

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrNotFound          = apperr.New(http.StatusNotFound, "EVENT_NOT_FOUND", "event not found")
	ErrSlugTaken         = apperr.New(http.StatusConflict, "SLUG_TAKEN", "an event with this name already exists")
	ErrNoCategories      = apperr.New(http.StatusConflict, "EVENT_NO_CATEGORIES", "cannot publish an event with no categories")
	ErrInvalidTransition = apperr.New(http.StatusConflict, "INVALID_STATUS_TRANSITION", "invalid status transition")
	ErrInvalidEventType  = apperr.New(http.StatusBadRequest, "INVALID_EVENT_TYPE", "unknown event type")
	ErrInvalidObjectKey  = apperr.New(http.StatusBadRequest, "INVALID_OBJECT_KEY", "object key does not belong to this event")
	ErrInvalidContent    = apperr.New(http.StatusBadRequest, "INVALID_CONTENT_TYPE", "unsupported content type")
	ErrFileTooLarge      = apperr.New(http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "file exceeds the allowed size")
	ErrInvalidMediaKind  = apperr.New(http.StatusBadRequest, "INVALID_MEDIA_KIND", "media kind must be banner or logo")
)
```

- [ ] **Step 2: Valid event types + status constants**

Add to `errors.go` (or a small `consts.go`; keep in `errors.go` for simplicity):
```go
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"
)

var validEventTypes = map[string]bool{
	"marathon": true, "trail": true, "cycling": true, "triathlon": true,
	"funrun": true, "expo": true, "seminar": true, "concert": true, "other": true,
}
```

- [ ] **Step 3: DTOs**

Create `services/api/internal/modules/events/dto.go`:
```go
package events

import (
	"time"

	"github.com/google/uuid"
)

type CreateRequest struct {
	Name         string     `json:"name"`
	EventType    string     `json:"eventType"`
	Description  string     `json:"description"`
	VenueName    string     `json:"venueName"`
	VenueAddress string     `json:"venueAddress"`
	StartsAt     *time.Time `json:"startsAt"`
	EndsAt       *time.Time `json:"endsAt"`
	FAQ          string     `json:"faq"`
	Terms        string     `json:"terms"`
	Waiver       string     `json:"waiver"`
}

type UpdateRequest struct {
	Name         string     `json:"name"`
	EventType    string     `json:"eventType"`
	Description  string     `json:"description"`
	VenueName    string     `json:"venueName"`
	VenueAddress string     `json:"venueAddress"`
	StartsAt     *time.Time `json:"startsAt"`
	EndsAt       *time.Time `json:"endsAt"`
	FAQ          string     `json:"faq"`
	Terms        string     `json:"terms"`
	Waiver       string     `json:"waiver"`
}

type Response struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	EventType    string     `json:"eventType"`
	Status       string     `json:"status"`
	Description  string     `json:"description"`
	BannerURL    string     `json:"bannerUrl"`
	LogoURL      string     `json:"logoUrl"`
	VenueName    string     `json:"venueName"`
	VenueAddress string     `json:"venueAddress"`
	StartsAt     *time.Time `json:"startsAt"`
	EndsAt       *time.Time `json:"endsAt"`
	FAQ          string     `json:"faq"`
	Terms        string     `json:"terms"`
	Waiver       string     `json:"waiver"`
	PublishedAt  *time.Time `json:"publishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}
```

- [ ] **Step 4: Slug helper + failing test**

Create `services/api/internal/modules/events/slug_test.go`:
```go
package events

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Jakarta Marathon 2026": "jakarta-marathon-2026",
		"  Trail   Run ":         "trail-run",
		"Bali!!! Run":            "bali-run",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
```

Create `services/api/internal/modules/events/slug.go`:
```go
package events

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

- [ ] **Step 5: Run slug test**

Run:
```bash
cd services/api && go test ./internal/modules/events/ -run TestSlugify -v; cd ../..
```
Expected: PASS.

- [ ] **Step 6: Repository interface + sqlc adapter**

Create `services/api/internal/modules/events/repository.go`:
```go
package events

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetEventByOrgAndSlug(ctx context.Context, arg db.GetEventByOrgAndSlugParams) (db.Event, error)
	ListEventsByOrg(ctx context.Context, orgID uuid.UUID) ([]db.Event, error)
	UpdateEvent(ctx context.Context, arg db.UpdateEventParams) (db.Event, error)
	UpdateEventStatus(ctx context.Context, arg db.UpdateEventStatusParams) (db.Event, error)
	SetEventMediaKey(ctx context.Context, arg db.SetEventMediaKeyParams) (db.Event, error)
	DeleteEvent(ctx context.Context, arg db.DeleteEventParams) error
	CountCategoriesForEvent(ctx context.Context, eventID uuid.UUID) (int64, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	return r.q.CreateEvent(ctx, arg)
}
func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}
func (r *sqlcRepo) GetEventByOrgAndSlug(ctx context.Context, arg db.GetEventByOrgAndSlugParams) (db.Event, error) {
	return r.q.GetEventByOrgAndSlug(ctx, arg)
}
func (r *sqlcRepo) ListEventsByOrg(ctx context.Context, orgID uuid.UUID) ([]db.Event, error) {
	return r.q.ListEventsByOrg(ctx, orgID)
}
func (r *sqlcRepo) UpdateEvent(ctx context.Context, arg db.UpdateEventParams) (db.Event, error) {
	return r.q.UpdateEvent(ctx, arg)
}
func (r *sqlcRepo) UpdateEventStatus(ctx context.Context, arg db.UpdateEventStatusParams) (db.Event, error) {
	return r.q.UpdateEventStatus(ctx, arg)
}
func (r *sqlcRepo) SetEventMediaKey(ctx context.Context, arg db.SetEventMediaKeyParams) (db.Event, error) {
	return r.q.SetEventMediaKey(ctx, arg)
}
func (r *sqlcRepo) DeleteEvent(ctx context.Context, arg db.DeleteEventParams) error {
	return r.q.DeleteEvent(ctx, arg)
}
func (r *sqlcRepo) CountCategoriesForEvent(ctx context.Context, eventID uuid.UUID) (int64, error) {
	return r.q.CountCategoriesForEvent(ctx, eventID)
}
```
Note: confirm each generated `*Params` struct's field names/types against `internal/db/events.sql.go`. The `*Params` for nullable columns will use `pgtype.Text`/`pgtype.Timestamptz`.

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/modules/events/errors.go services/api/internal/modules/events/dto.go \
  services/api/internal/modules/events/slug.go services/api/internal/modules/events/slug_test.go \
  services/api/internal/modules/events/repository.go
git commit -m "feat(events): add errors, dtos, slug, and repository"
```

---

## Task 6: Events service (CRUD + lifecycle + tenant guard)

**Files:**
- Create: `services/api/internal/modules/events/service.go`
- Test: `services/api/internal/modules/events/service_test.go`

The service stores the `storage.Storage` so it can build `PublicURL` for responses and handle media (Task 9). It also takes an `AuditRecorder` (nil-safe).

- [ ] **Step 1: Write the failing tests**

Create `services/api/internal/modules/events/service_test.go`:
```go
package events

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type fakeRepo struct {
	events     map[uuid.UUID]db.Event
	slugs      map[string]bool
	catCount   map[uuid.UUID]int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		events:   map[uuid.UUID]db.Event{},
		slugs:    map[string]bool{},
		catCount: map[uuid.UUID]int64{},
	}
}

func (f *fakeRepo) CreateEvent(_ context.Context, arg db.CreateEventParams) (db.Event, error) {
	if f.slugs[arg.Slug] {
		return db.Event{}, &pgconnUnique{}
	}
	e := db.Event{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, Name: arg.Name,
		Slug: arg.Slug, EventType: arg.EventType, Status: StatusDraft,
	}
	f.events[e.ID] = e
	f.slugs[arg.Slug] = true
	return e, nil
}
func (f *fakeRepo) GetEventByID(_ context.Context, id uuid.UUID) (db.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return db.Event{}, pgx.ErrNoRows
	}
	return e, nil
}
func (f *fakeRepo) GetEventByOrgAndSlug(_ context.Context, arg db.GetEventByOrgAndSlugParams) (db.Event, error) {
	for _, e := range f.events {
		if e.OrganizationID == arg.OrganizationID && e.Slug == arg.Slug {
			return e, nil
		}
	}
	return db.Event{}, pgx.ErrNoRows
}
func (f *fakeRepo) ListEventsByOrg(_ context.Context, orgID uuid.UUID) ([]db.Event, error) {
	var out []db.Event
	for _, e := range f.events {
		if e.OrganizationID == orgID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeRepo) UpdateEvent(_ context.Context, arg db.UpdateEventParams) (db.Event, error) {
	e := f.events[arg.ID]
	e.Name = arg.Name
	e.EventType = arg.EventType
	f.events[arg.ID] = e
	return e, nil
}
func (f *fakeRepo) UpdateEventStatus(_ context.Context, arg db.UpdateEventStatusParams) (db.Event, error) {
	e := f.events[arg.ID]
	e.Status = arg.Status
	e.PublishedAt = arg.PublishedAt
	f.events[arg.ID] = e
	return e, nil
}
func (f *fakeRepo) SetEventMediaKey(_ context.Context, arg db.SetEventMediaKeyParams) (db.Event, error) {
	e := f.events[arg.ID]
	if arg.BannerObjectKey.Valid {
		e.BannerObjectKey = arg.BannerObjectKey
	}
	if arg.LogoObjectKey.Valid {
		e.LogoObjectKey = arg.LogoObjectKey
	}
	f.events[arg.ID] = e
	return e, nil
}
func (f *fakeRepo) DeleteEvent(_ context.Context, arg db.DeleteEventParams) error {
	delete(f.events, arg.ID)
	return nil
}
func (f *fakeRepo) CountCategoriesForEvent(_ context.Context, eventID uuid.UUID) (int64, error) {
	return f.catCount[eventID], nil
}

// pgconnUnique simulates a unique-violation error type for slug collisions.
type pgconnUnique struct{}

func (e *pgconnUnique) Error() string { return "unique_violation" }

func newSvc(repo Repository) *Service { return NewService(repo, nil, nil) }

func TestCreate_GeneratesSlugAndDraft(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	got, err := svc.Create(context.Background(), orgID, CreateRequest{Name: "Jakarta Marathon", EventType: "marathon"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.Slug != "jakarta-marathon" {
		t.Errorf("slug = %q, want jakarta-marathon", got.Slug)
	}
	if got.Status != StatusDraft {
		t.Errorf("status = %q, want draft", got.Status)
	}
}

func TestCreate_RejectsUnknownEventType(t *testing.T) {
	svc := newSvc(newFakeRepo())
	_, err := svc.Create(context.Background(), uuid.New(), CreateRequest{Name: "X", EventType: "rocket-race"})
	if err != ErrInvalidEventType {
		t.Fatalf("err = %v, want ErrInvalidEventType", err)
	}
}

func TestPublish_RejectsWithoutCategories(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, _ := svc.Create(context.Background(), orgID, CreateRequest{Name: "E", EventType: "marathon"})
	repo.catCount[ev.ID] = 0

	_, err := svc.Publish(context.Background(), orgID, ev.ID)
	if err != ErrNoCategories {
		t.Fatalf("err = %v, want ErrNoCategories", err)
	}
}

func TestPublish_SucceedsWithCategories(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, _ := svc.Create(context.Background(), orgID, CreateRequest{Name: "E", EventType: "marathon"})
	repo.catCount[ev.ID] = 2

	got, err := svc.Publish(context.Background(), orgID, ev.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got.Status != StatusPublished {
		t.Errorf("status = %q, want published", got.Status)
	}
}

func TestUnpublish_RejectsNonPublished(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgID := uuid.New()
	ev, _ := svc.Create(context.Background(), orgID, CreateRequest{Name: "E", EventType: "marathon"})
	// still draft
	if _, err := svc.Unpublish(context.Background(), orgID, ev.ID); err != ErrInvalidTransition {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestGet_TenantMismatchIsNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	orgA := uuid.New()
	ev, _ := svc.Create(context.Background(), orgA, CreateRequest{Name: "E", EventType: "marathon"})
	orgB := uuid.New()
	if _, err := svc.Get(context.Background(), orgB, ev.ID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

var _ = pgtype.Text{}
```
Note: remove the trailing `var _ = pgtype.Text{}` if unused after writing the service; it's there only if the test file ends up not referencing pgtype directly.

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/events/ -run 'TestCreate|TestPublish|TestUnpublish|TestGet' -v; cd ../..
```
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 3: Implement the service**

Create `services/api/internal/modules/events/service.go`:
```go
package events

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	"github.com/varin/ivyticketing/services/api/internal/platform/storage"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type Service struct {
	repo    Repository
	store   storage.Storage
	audit   AuditRecorder
}

func NewService(repo Repository, store storage.Storage, recorder AuditRecorder) *Service {
	return &Service{repo: repo, store: store, audit: recorder}
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, req CreateRequest) (Response, error) {
	if !validEventTypes[req.EventType] {
		return Response{}, ErrInvalidEventType
	}
	slug := slugify(req.Name)
	if _, err := s.repo.GetEventByOrgAndSlug(ctx, db.GetEventByOrgAndSlugParams{OrganizationID: orgID, Slug: slug}); err == nil {
		return Response{}, ErrSlugTaken
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Response{}, err
	}

	e, err := s.repo.CreateEvent(ctx, db.CreateEventParams{
		OrganizationID: orgID,
		Name:           req.Name,
		Slug:           slug,
		Description:    nullText(req.Description),
		EventType:      req.EventType,
		VenueName:      nullText(req.VenueName),
		VenueAddress:   nullText(req.VenueAddress),
		StartsAt:       nullTime(req.StartsAt),
		EndsAt:         nullTime(req.EndsAt),
		Faq:            nullText(req.FAQ),
		Terms:          nullText(req.Terms),
		Waiver:         nullText(req.Waiver),
	})
	if err != nil {
		return Response{}, err
	}
	return s.toResponse(e), nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Response, error) {
	rows, err := s.repo.ListEventsByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]Response, 0, len(rows))
	for _, e := range rows {
		out = append(out, s.toResponse(e))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, orgID, eventID uuid.UUID) (Response, error) {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return Response{}, err
	}
	return s.toResponse(e), nil
}

func (s *Service) Update(ctx context.Context, orgID, eventID uuid.UUID, req UpdateRequest) (Response, error) {
	if !validEventTypes[req.EventType] {
		return Response{}, ErrInvalidEventType
	}
	if _, err := s.loadOrgEvent(ctx, orgID, eventID); err != nil {
		return Response{}, err
	}
	e, err := s.repo.UpdateEvent(ctx, db.UpdateEventParams{
		ID:             eventID,
		Name:           req.Name,
		Description:    nullText(req.Description),
		EventType:      req.EventType,
		VenueName:      nullText(req.VenueName),
		VenueAddress:   nullText(req.VenueAddress),
		StartsAt:       nullTime(req.StartsAt),
		EndsAt:         nullTime(req.EndsAt),
		Faq:            nullText(req.FAQ),
		Terms:          nullText(req.Terms),
		Waiver:         nullText(req.Waiver),
		OrganizationID: orgID,
	})
	if err != nil {
		return Response{}, err
	}
	return s.toResponse(e), nil
}

func (s *Service) Publish(ctx context.Context, orgID, eventID uuid.UUID) (Response, error) {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return Response{}, err
	}
	if e.Status != StatusDraft {
		return Response{}, ErrInvalidTransition
	}
	n, err := s.repo.CountCategoriesForEvent(ctx, eventID)
	if err != nil {
		return Response{}, err
	}
	if n == 0 {
		return Response{}, ErrNoCategories
	}
	updated, err := s.repo.UpdateEventStatus(ctx, db.UpdateEventStatusParams{
		ID:             eventID,
		Status:         StatusPublished,
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		OrganizationID: orgID,
	})
	if err != nil {
		return Response{}, err
	}
	s.record(ctx, orgID, "event.publish", eventID)
	return s.toResponse(updated), nil
}

func (s *Service) Unpublish(ctx context.Context, orgID, eventID uuid.UUID) (Response, error) {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return Response{}, err
	}
	if e.Status != StatusPublished {
		return Response{}, ErrInvalidTransition
	}
	updated, err := s.repo.UpdateEventStatus(ctx, db.UpdateEventStatusParams{
		ID:             eventID,
		Status:         StatusDraft,
		PublishedAt:    pgtype.Timestamptz{Valid: false},
		OrganizationID: orgID,
	})
	if err != nil {
		return Response{}, err
	}
	s.record(ctx, orgID, "event.unpublish", eventID)
	return s.toResponse(updated), nil
}

func (s *Service) Archive(ctx context.Context, orgID, eventID uuid.UUID) (Response, error) {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return Response{}, err
	}
	if e.Status == StatusArchived {
		return Response{}, ErrInvalidTransition
	}
	updated, err := s.repo.UpdateEventStatus(ctx, db.UpdateEventStatusParams{
		ID:             eventID,
		Status:         StatusArchived,
		PublishedAt:    e.PublishedAt,
		OrganizationID: orgID,
	})
	if err != nil {
		return Response{}, err
	}
	s.record(ctx, orgID, "event.archive", eventID)
	return s.toResponse(updated), nil
}

func (s *Service) Delete(ctx context.Context, orgID, eventID uuid.UUID) error {
	e, err := s.loadOrgEvent(ctx, orgID, eventID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteEvent(ctx, db.DeleteEventParams{ID: eventID, OrganizationID: orgID}); err != nil {
		return err
	}
	// best-effort media cleanup
	if s.store != nil {
		if e.BannerObjectKey.Valid {
			_ = s.store.Delete(ctx, e.BannerObjectKey.String)
		}
		if e.LogoObjectKey.Valid {
			_ = s.store.Delete(ctx, e.LogoObjectKey.String)
		}
	}
	s.record(ctx, orgID, "event.delete", eventID)
	return nil
}

// loadOrgEvent fetches an event and confirms tenant ownership (mismatch → ErrNotFound).
func (s *Service) loadOrgEvent(ctx context.Context, orgID, eventID uuid.UUID) (db.Event, error) {
	e, err := s.repo.GetEventByID(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Event{}, ErrNotFound
	} else if err != nil {
		return db.Event{}, err
	}
	if e.OrganizationID != orgID {
		return db.Event{}, ErrNotFound
	}
	return e, nil
}

func (s *Service) record(ctx context.Context, orgID uuid.UUID, action string, eventID uuid.UUID) {
	if s.audit == nil {
		return
	}
	oid := orgID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid,
		Action:         action,
		TargetType:     "event",
		TargetID:       eventID.String(),
	})
}

func (s *Service) toResponse(e db.Event) Response {
	r := Response{
		ID: e.ID, Name: e.Name, Slug: e.Slug, EventType: e.EventType, Status: e.Status,
		Description:  e.Description.String,
		VenueName:    e.VenueName.String,
		VenueAddress: e.VenueAddress.String,
		FAQ:          e.Faq.String,
		Terms:        e.Terms.String,
		Waiver:       e.Waiver.String,
		StartsAt:     timePtr(e.StartsAt),
		EndsAt:       timePtr(e.EndsAt),
		PublishedAt:  timePtr(e.PublishedAt),
		CreatedAt:    e.CreatedAt.Time,
	}
	if s.store != nil {
		if e.BannerObjectKey.Valid {
			r.BannerURL = s.store.PublicURL(e.BannerObjectKey.String)
		}
		if e.LogoObjectKey.Valid {
			r.LogoURL = s.store.PublicURL(e.LogoObjectKey.String)
		}
	}
	return r
}

func nullText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func nullTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
```
Note: field names like `e.Faq` follow sqlc's capitalization of `faq` → confirm against generated `db.Event` (likely `Faq`). Adjust `toResponse`/params if sqlc named them differently.

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/modules/events/ -v; cd ../..
```
Expected: PASS (all events service + slug tests).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/events/service.go services/api/internal/modules/events/service_test.go
git commit -m "feat(events): add service with CRUD, status lifecycle, and tenant guard"
```

---

## Task 7: Events handler, routes, and wiring

**Files:**
- Create: `services/api/internal/modules/events/handler.go`
- Create: `services/api/internal/modules/events/routes.go`
- Modify: `services/api/internal/app/server.go`

(Media endpoints are added in Part 3 Task 9; this task wires CRUD + lifecycle.)

- [ ] **Step 1: Handler**

Create `services/api/internal/modules/events/handler.go`:
```go
package events

import (
	"encoding/json"
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

func (h *Handler) orgID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) eventID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.EventType == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name and eventType are required"))
		return
	}
	ev, err := h.svc.Create(r.Context(), orgID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, ev)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.List(r.Context(), orgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	ev, err := h.svc.Get(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.EventType == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name and eventType are required"))
		return
	}
	ev, err := h.svc.Update(r.Context(), orgID, eventID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request)   { h.transition(w, r, h.svc.Publish) }
func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) { h.transition(w, r, h.svc.Unpublish) }
func (h *Handler) Archive(w http.ResponseWriter, r *http.Request)   { h.transition(w, r, h.svc.Archive) }

func (h *Handler) transition(w http.ResponseWriter, r *http.Request, fn func(ctx interface{ Done() <-chan struct{} }, orgID, eventID uuid.UUID) (Response, error)) {
	// placeholder signature — replaced below
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), orgID, eventID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var _ = authctx.FromContext
```
**Correction:** the `transition` helper above is wrong (placeholder). Replace the three transition handlers and the helper with explicit handlers:
```go
func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.svc.Publish)
}
func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.svc.Unpublish)
}
func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.svc.Archive)
}

func (h *Handler) runTransition(w http.ResponseWriter, r *http.Request, fn func(context.Context, uuid.UUID, uuid.UUID) (Response, error)) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	ev, err := fn(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}
```
Add `"context"` to imports and remove the `var _ = authctx.FromContext` line and the `authctx` import (not needed here — orgId comes from URL, identity already checked by authz middleware).

- [ ] **Step 2: Routes**

Create `services/api/internal/modules/events/routes.go`:
```go
package events

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts event endpoints under an existing /organizations/{orgId}
// router (already behind authn). Each route enforces its own permission.
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/events", func(r chi.Router) {
		r.With(middleware.RequirePermission(loader, "event.create")).Post("/", h.Create)
		r.With(middleware.RequirePermission(loader, "event.edit")).Get("/", h.List)
		r.With(middleware.RequirePermission(loader, "event.edit")).Get("/{eventId}", h.Get)
		r.With(middleware.RequirePermission(loader, "event.edit")).Put("/{eventId}", h.Update)
		r.With(middleware.RequirePermission(loader, "event.publish")).Post("/{eventId}/publish", h.Publish)
		r.With(middleware.RequirePermission(loader, "event.publish")).Post("/{eventId}/unpublish", h.Unpublish)
		r.With(middleware.RequirePermission(loader, "event.publish")).Post("/{eventId}/archive", h.Archive)
		r.With(middleware.RequirePermission(loader, "event.delete")).Delete("/{eventId}", h.Delete)
	})
}
```

- [ ] **Step 3: Wire into server.go**

In `services/api/internal/app/server.go`:

Add import:
```go
	eventsmod "github.com/varin/ivyticketing/services/api/internal/modules/events"
	"github.com/varin/ivyticketing/services/api/internal/platform/storage"
```

Build storage + events handler after the existing handlers (before `r.Route("/api/v1", ...)`):
```go
	store, err := storage.New(storage.Config{
		Driver:        cfg.StorageDriver,
		LocalPath:     cfg.StorageLocalPath,
		PublicBaseURL: cfg.StoragePublicBaseURL,
		Bucket:        cfg.StorageBucket,
		Endpoint:      cfg.StorageEndpoint,
		AccessKey:     cfg.StorageAccessKey,
		SecretKey:     cfg.StorageSecretKey,
		Region:        cfg.StorageRegion,
	})
	if err != nil {
		log.Error("storage init failed", "error", err)
		// fail fast: storage is required for media; but keep API bootable for non-media routes
	}
	eventHandler := eventsmod.NewHandler(eventsmod.NewService(eventsmod.NewRepository(pool), store, auditLog))
```
**Important:** `NewRouter` currently returns only `http.Handler`. To surface the storage init error cleanly, change `NewRouter` to return `(http.Handler, error)` OR handle the error by logging and continuing. Simplest: keep signature, log error, and proceed (store may be nil → events service guards nil store). Given the cleaner contract, **change `NewRouter` to return `(http.Handler, error)`** and update its single caller in `cmd/api/main.go`:
```go
	handler, err := app.NewRouter(cfg, log, pg.Pool, pg, rdb)
	if err != nil {
		log.Error("router init failed", "error", err)
		os.Exit(1)
	}
```
And in `NewRouter`, return `nil, err` on storage failure, and `return r, nil` at the end.

Mount events inside the per-org authenticated block:
```go
			r.Route("/organizations/{orgId}", func(r chi.Router) {
				memberHandler.RegisterRoutes(r, loader)
				roleHandler.RegisterRoutes(r, loader)
				eventHandler.RegisterRoutes(r, loader)
			})
```

- [ ] **Step 4: Build and test**

Run:
```bash
cd services/api && go build ./... && go test ./...; cd ../..
```
Expected: build OK; all tests PASS. Fix the `NewRouter` signature change in any integration helper if present (none yet for Phase 3).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/events/handler.go services/api/internal/modules/events/routes.go \
  services/api/internal/app/server.go services/api/cmd/api/main.go
git commit -m "feat(events): add handler, routes, and wire into router with storage"
```

---
