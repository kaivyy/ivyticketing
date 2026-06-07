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
