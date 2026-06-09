package publiccatalog

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type fakeRepo struct {
	events          []db.Event
	byNotFound      bool
	categories      map[uuid.UUID][]db.EventCategory
	categoriesWMode map[uuid.UUID][]db.EventCategoryWithMode
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
func (f *fakeRepo) ListCategoriesByEventForPublicWithMode(_ context.Context, eventID uuid.UUID) ([]db.EventCategoryWithMode, error) {
	return f.categoriesWMode[eventID], nil
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

func TestGetEvent_RegistrationMode(t *testing.T) {
	evID := uuid.New()
	catID := uuid.New()
	repo := &fakeRepo{
		events: []db.Event{{ID: evID, Name: "E", Slug: "e", Status: "published"}},
		categoriesWMode: map[uuid.UUID][]db.EventCategoryWithMode{
			evID: {
				{ID: catID, Name: "VIP", Price: 500000, RegistrationMode: pgtype.Text{String: "BALLOT", Valid: true}},
				{ID: uuid.New(), Name: "General", Price: 100000}, // no mode → NORMAL
			},
		},
	}
	svc := NewService(repo, fakeStore{})
	out, err := svc.GetEvent(context.Background(), "org", "e")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if len(out.Categories) != 2 {
		t.Fatalf("want 2 categories, got %d", len(out.Categories))
	}
	if out.Categories[0].RegistrationMode != "BALLOT" {
		t.Errorf("cat[0] mode = %q, want BALLOT", out.Categories[0].RegistrationMode)
	}
	if out.Categories[1].RegistrationMode != "NORMAL" {
		t.Errorf("cat[1] mode = %q, want NORMAL", out.Categories[1].RegistrationMode)
	}
}
