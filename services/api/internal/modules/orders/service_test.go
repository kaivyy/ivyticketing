package orders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/varin/ivyticketing/services/api/internal/db"
	inv "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
)

// fakeRepo implements both orders.Repository and inventory.Repository.
type fakeRepo struct {
	events     map[uuid.UUID]db.Event
	categories map[uuid.UUID]db.EventCategory
	orders     map[uuid.UUID]db.Order
	reserves   map[uuid.UUID]db.InventoryReservation
	orderNums  map[string]db.Order
	nextID     uuid.UUID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		events:     make(map[uuid.UUID]db.Event),
		categories: make(map[uuid.UUID]db.EventCategory),
		orders:     make(map[uuid.UUID]db.Order),
		reserves:   make(map[uuid.UUID]db.InventoryReservation),
		orderNums:  make(map[string]db.Order),
		nextID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}
}

func (f *fakeRepo) nextUUID() uuid.UUID {
	id := f.nextID
	b := id[:]
	for i := len(b) - 1; i >= 0; i-- {
		b[i]++
		if b[i] != 0 {
			break
		}
	}
	f.nextID = id
	return id
}

func (f *fakeRepo) seed(orgID uuid.UUID, capacity, maxOrder int32) (eventID, categoryID uuid.UUID) {
	eventID = f.nextUUID()
	categoryID = f.nextUUID()
	f.events[eventID] = db.Event{
		ID:             eventID,
		OrganizationID: orgID,
		Status:         "published",
	}
	f.categories[categoryID] = db.EventCategory{
		ID:              categoryID,
		OrganizationID:  orgID,
		EventID:         eventID,
		Price:           50000,
		Capacity:        capacity,
		MaxOrderPerUser: maxOrder,
	}
	return eventID, categoryID
}

func (f *fakeRepo) seedDraft(orgID uuid.UUID) (eventID, categoryID uuid.UUID) {
	eventID = f.nextUUID()
	categoryID = f.nextUUID()
	f.events[eventID] = db.Event{
		ID:             eventID,
		OrganizationID: orgID,
		Status:         "draft",
	}
	f.categories[categoryID] = db.EventCategory{
		ID:              categoryID,
		OrganizationID:  orgID,
		EventID:         eventID,
		Price:           50000,
		Capacity:        10,
		MaxOrderPerUser: 1,
	}
	return eventID, categoryID
}

// orders.Repository methods

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(Repository) error) error {
	return fn(f)
}

func (f *fakeRepo) Inventory() inv.Repository { return f }

func (f *fakeRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return db.Event{}, pgx.ErrNoRows
	}
	return e, nil
}

func (f *fakeRepo) GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return db.Order{}, pgx.ErrNoRows
	}
	return o, nil
}

func (f *fakeRepo) GetOrderByNumber(ctx context.Context, number string) (db.Order, error) {
	o, ok := f.orderNums[number]
	if !ok {
		return db.Order{}, pgx.ErrNoRows
	}
	return o, nil
}

func (f *fakeRepo) CreateOrder(ctx context.Context, arg db.CreateOrderParams) (db.Order, error) {
	id := f.nextUUID()
	o := db.Order{
		ID:             id,
		OrganizationID: arg.OrganizationID,
		EventID:        arg.EventID,
		CategoryID:     arg.CategoryID,
		ParticipantID:  arg.ParticipantID,
		OrderNumber:    arg.OrderNumber,
		Status:         arg.Status,
		Subtotal:       arg.Subtotal,
		Fee:            arg.Fee,
		Discount:       arg.Discount,
		Total:          arg.Total,
		ExpiredAt:      arg.ExpiredAt,
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	f.orders[id] = o
	f.orderNums[arg.OrderNumber] = o
	return o, nil
}

func (f *fakeRepo) UpdateOrderStatus(ctx context.Context, arg db.UpdateOrderStatusParams) (db.Order, error) {
	o, ok := f.orders[arg.ID]
	if !ok {
		return db.Order{}, pgx.ErrNoRows
	}
	if o.Status != arg.Status_2 {
		return db.Order{}, errors.New("status mismatch")
	}
	o.Status = arg.Status
	f.orders[arg.ID] = o
	f.orderNums[o.OrderNumber] = o
	return o, nil
}

func (f *fakeRepo) ListOrdersByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Order, error) {
	var out []db.Order
	for _, o := range f.orders {
		if o.ParticipantID == participantID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListOrdersByOrgEvent(ctx context.Context, arg db.ListOrdersByOrgEventParams) ([]db.Order, error) {
	var out []db.Order
	for _, o := range f.orders {
		if o.OrganizationID == arg.OrganizationID && o.EventID == arg.EventID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeRepo) CountActiveOrdersForUserCategory(ctx context.Context, arg db.CountActiveOrdersForUserCategoryParams) (int64, error) {
	var count int64
	for _, o := range f.orders {
		if o.CategoryID == arg.CategoryID && o.ParticipantID == arg.ParticipantID &&
			(o.Status == StatusPendingPayment || o.Status == StatusPaid) {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepo) ListExpiredPendingOrders(_ context.Context, limit int32) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	now := time.Now()
	for _, o := range f.orders {
		if o.Status == StatusPendingPayment && o.ExpiredAt.Valid && o.ExpiredAt.Time.Before(now) {
			ids = append(ids, o.ID)
		}
	}
	return ids, nil
}

// inventory.Repository methods

func (f *fakeRepo) LockCategoryForUpdate(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	cat, ok := f.categories[id]
	if !ok {
		return db.EventCategory{}, pgx.ErrNoRows
	}
	return cat, nil
}

func (f *fakeRepo) CountActiveReservationsByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error) {
	var count int64
	for _, r := range f.reserves {
		if r.CategoryID == categoryID && r.Status == ReservationActive {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepo) CountPaidByCategory(ctx context.Context, categoryID uuid.UUID) (int64, error) {
	var count int64
	for _, o := range f.orders {
		if o.CategoryID == categoryID && o.Status == StatusPaid {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepo) CreateReservation(ctx context.Context, arg db.CreateReservationParams) (db.InventoryReservation, error) {
	id := f.nextUUID()
	r := db.InventoryReservation{
		ID:             id,
		OrganizationID: arg.OrganizationID,
		EventID:        arg.EventID,
		CategoryID:     arg.CategoryID,
		OrderID:        arg.OrderID,
		ParticipantID:  arg.ParticipantID,
		Status:         ReservationActive,
		ExpiresAt:      arg.ExpiresAt,
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	f.reserves[id] = r
	return r, nil
}

func (f *fakeRepo) ExpireReservationsForOrder(ctx context.Context, orderID uuid.UUID) error {
	for id, r := range f.reserves {
		if r.OrderID == orderID && r.Status == ReservationActive {
			r.Status = ReservationExpired
			f.reserves[id] = r
		}
	}
	return nil
}

func (f *fakeRepo) UpdateReservationStatusByOrder(ctx context.Context, arg db.UpdateReservationStatusByOrderParams) error {
	for id, r := range f.reserves {
		if r.OrderID == arg.OrderID && r.Status == ReservationActive {
			r.Status = arg.Status
			f.reserves[id] = r
		}
	}
	return nil
}

func TestCheckout_Success(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	orgID := uuid.New()
	participantID := uuid.New()
	eventID, categoryID := repo.seed(orgID, 10, 2)

	svc := NewService(repo, nil, 15*time.Minute)
	resp, err := svc.Checkout(ctx, participantID, eventID, categoryID)
	require.NoError(t, err)
	assert.Equal(t, StatusPendingPayment, resp.Status)
	assert.Equal(t, int64(50000), resp.Total)
	assert.NotEmpty(t, resp.OrderNumber)

	// Verify reservation was created
	var found bool
	for _, r := range repo.reserves {
		if r.OrderID == resp.ID {
			found = true
			assert.Equal(t, ReservationActive, r.Status)
		}
	}
	assert.True(t, found, "reservation should be created")
}

func TestCheckout_SoldOut(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	orgID := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()
	eventID, categoryID := repo.seed(orgID, 1, 1)

	svc := NewService(repo, nil, 15*time.Minute)
	_, err := svc.Checkout(ctx, p1, eventID, categoryID)
	require.NoError(t, err)

	_, err = svc.Checkout(ctx, p2, eventID, categoryID)
	assert.ErrorIs(t, err, inv.ErrSoldOut)
}

func TestCheckout_MaxOrderExceeded(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	orgID := uuid.New()
	participantID := uuid.New()
	eventID, categoryID := repo.seed(orgID, 10, 1)

	svc := NewService(repo, nil, 15*time.Minute)
	_, err := svc.Checkout(ctx, participantID, eventID, categoryID)
	require.NoError(t, err)

	_, err = svc.Checkout(ctx, participantID, eventID, categoryID)
	assert.ErrorIs(t, err, ErrMaxOrderExceeded)
}

func TestCheckout_EventNotPublished(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	orgID := uuid.New()
	participantID := uuid.New()
	eventID, categoryID := repo.seedDraft(orgID)

	svc := NewService(repo, nil, 15*time.Minute)
	_, err := svc.Checkout(ctx, participantID, eventID, categoryID)
	assert.ErrorIs(t, err, ErrEventNotPublished)
}

func TestCancel_ReleasesReservation(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	orgID := uuid.New()
	participantID := uuid.New()
	eventID, categoryID := repo.seed(orgID, 10, 2)

	svc := NewService(repo, nil, 15*time.Minute)
	resp, err := svc.Checkout(ctx, participantID, eventID, categoryID)
	require.NoError(t, err)

	err = svc.Cancel(ctx, participantID, resp.ID)
	require.NoError(t, err)

	order, ok := repo.orders[resp.ID]
	require.True(t, ok)
	assert.Equal(t, StatusCancelled, order.Status)

	var resStatus string
	for _, r := range repo.reserves {
		if r.OrderID == resp.ID {
			resStatus = r.Status
		}
	}
	assert.Equal(t, ReservationReleased, resStatus)
}

func TestCancel_NotOwner(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	orgID := uuid.New()
	participantID := uuid.New()
	otherUser := uuid.New()
	eventID, categoryID := repo.seed(orgID, 10, 2)

	svc := NewService(repo, nil, 15*time.Minute)
	resp, err := svc.Checkout(ctx, participantID, eventID, categoryID)
	require.NoError(t, err)

	err = svc.Cancel(ctx, otherUser, resp.ID)
	assert.ErrorIs(t, err, ErrOrderNotFound)
}

func TestGet_NotOwnerIsNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	orgID := uuid.New()
	participantID := uuid.New()
	otherUser := uuid.New()
	eventID, categoryID := repo.seed(orgID, 10, 2)

	svc := NewService(repo, nil, 15*time.Minute)
	resp, err := svc.Checkout(ctx, participantID, eventID, categoryID)
	require.NoError(t, err)

	_, err = svc.GetForParticipant(ctx, otherUser, resp.ID)
	assert.ErrorIs(t, err, ErrOrderNotFound)
}
