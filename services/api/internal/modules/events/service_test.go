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
	events   map[uuid.UUID]db.Event
	slugs    map[string]bool
	catCount map[uuid.UUID]int64
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
