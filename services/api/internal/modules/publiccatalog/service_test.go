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
