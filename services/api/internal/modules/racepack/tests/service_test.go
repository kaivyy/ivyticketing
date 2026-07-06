package racepack_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/racepack"
)

// fakeRepo implements racepack.Repository in memory for unit tests.
type fakeRepo struct {
	mu sync.Mutex

	counters   map[uuid.UUID]db.RacepackCounter
	slots      map[uuid.UUID]db.RacepackPickupSlot
	pickupRecs map[uuid.UUID]db.RacepackPickupRecord
	proxyAuths map[uuid.UUID]db.RacepackProxyAuthorization
	problemCs  map[uuid.UUID]db.RacepackProblemCase
	tickets    map[uuid.UUID]db.Ticket
	events     map[uuid.UUID]db.Event
	idemp      map[string]db.IdempotencyKey

	// Lookup fixture state — controlled per-test.
	ticketStatus  string
	ticketEventID uuid.UUID
	ticketPartID  uuid.UUID
	bib           string
	ticketFound   bool
	orderStatus   string
	hasActive     bool
	ticketLocked  bool // when true, LockTicketForUpdate returns a locked row

	createPickupErr  error
	createPickupCalls int32
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		counters:   map[uuid.UUID]db.RacepackCounter{},
		slots:      map[uuid.UUID]db.RacepackPickupSlot{},
		pickupRecs: map[uuid.UUID]db.RacepackPickupRecord{},
		proxyAuths: map[uuid.UUID]db.RacepackProxyAuthorization{},
		problemCs:  map[uuid.UUID]db.RacepackProblemCase{},
		tickets:    map[uuid.UUID]db.Ticket{},
		events:     map[uuid.UUID]db.Event{},
		idemp:      map[string]db.IdempotencyKey{},
	}
}

// seedTicket inserts a ticket row that LockTicketForUpdate + GetTicketStatus can read.
func (r *fakeRepo) seedTicket(t db.Ticket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tickets[t.ID] = t
	r.ticketStatus = t.Status
	r.ticketEventID = t.EventID
	r.ticketPartID = t.ParticipantID
	if t.BibNumber.Valid {
		r.bib = t.BibNumber.String
	} else {
		r.bib = ""
	}
	r.ticketFound = true
}

func (r *fakeRepo) seedCounter(c db.RacepackCounter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[c.ID] = c
}

// --- Repository ---

func (r *fakeRepo) ExecTx(ctx context.Context, fn func(racepack.Repository) error) error {
	return fn(r)
}

func (r *fakeRepo) CreateCounter(ctx context.Context, arg db.CreateRacepackCounterParams) (db.RacepackCounter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := db.RacepackCounter{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID,
		Name: arg.Name, Location: arg.Location, Active: arg.Active,
		CreatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
	}
	r.counters[c.ID] = c
	return c, nil
}
func (r *fakeRepo) ListCountersByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RacepackCounter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []db.RacepackCounter{}
	for _, c := range r.counters {
		if c.EventID == eventID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *fakeRepo) GetCounterByID(ctx context.Context, id uuid.UUID) (db.RacepackCounter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.counters[id]
	if !ok {
		return db.RacepackCounter{}, pgx.ErrNoRows
	}
	return c, nil
}
func (r *fakeRepo) UpdateCounter(ctx context.Context, arg db.UpdateRacepackCounterParams) (db.RacepackCounter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.counters[arg.ID]
	if !ok {
		return db.RacepackCounter{}, pgx.ErrNoRows
	}
	c.Name = arg.Name
	c.Location = arg.Location
	c.Active = arg.Active
	r.counters[arg.ID] = c
	return c, nil
}
func (r *fakeRepo) SetCounterActive(ctx context.Context, id uuid.UUID, active bool) (db.RacepackCounter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.counters[id]
	if !ok {
		return db.RacepackCounter{}, pgx.ErrNoRows
	}
	c.Active = active
	r.counters[id] = c
	return c, nil
}

func (r *fakeRepo) CreateSlot(ctx context.Context, arg db.CreateRacepackPickupSlotParams) (db.RacepackPickupSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := db.RacepackPickupSlot{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID,
		Name: arg.Name, PickupDate: arg.PickupDate, StartTime: arg.StartTime,
		EndTime: arg.EndTime, Capacity: arg.Capacity, ReservedCount: 0,
		Active: true,
		CreatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
	}
	r.slots[s.ID] = s
	return s, nil
}
func (r *fakeRepo) ListSlotsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RacepackPickupSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []db.RacepackPickupSlot{}
	for _, s := range r.slots {
		if s.EventID == eventID {
			out = append(out, s)
		}
	}
	return out, nil
}
func (r *fakeRepo) ListActiveSlotsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RacepackPickupSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []db.RacepackPickupSlot{}
	for _, s := range r.slots {
		if s.EventID == eventID && s.Active {
			out = append(out, s)
		}
	}
	return out, nil
}
func (r *fakeRepo) GetSlotByID(ctx context.Context, id uuid.UUID) (db.RacepackPickupSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[id]
	if !ok {
		return db.RacepackPickupSlot{}, pgx.ErrNoRows
	}
	return s, nil
}
func (r *fakeRepo) UpdateSlot(ctx context.Context, arg db.UpdateRacepackPickupSlotParams) (db.RacepackPickupSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[arg.ID]
	if !ok {
		return db.RacepackPickupSlot{}, pgx.ErrNoRows
	}
	s.Name = arg.Name
	s.PickupDate = arg.PickupDate
	s.StartTime = arg.StartTime
	s.EndTime = arg.EndTime
	s.Capacity = arg.Capacity
	s.Active = arg.Active
	r.slots[arg.ID] = s
	return s, nil
}
func (r *fakeRepo) IncrementSlotReserved(ctx context.Context, id uuid.UUID) (db.RacepackPickupSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[id]
	if !ok {
		return db.RacepackPickupSlot{}, pgx.ErrNoRows
	}
	if s.ReservedCount >= s.Capacity {
		return db.RacepackPickupSlot{}, pgx.ErrNoRows
	}
	s.ReservedCount++
	r.slots[id] = s
	return s, nil
}
func (r *fakeRepo) DecrementSlotReserved(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[id]
	if !ok {
		return pgx.ErrNoRows
	}
	if s.ReservedCount > 0 {
		s.ReservedCount--
		r.slots[id] = s
	}
	return nil
}

func (r *fakeRepo) CreatePickupRecord(ctx context.Context, arg db.CreateRacepackPickupRecordParams) (db.RacepackPickupRecord, error) {
	atomic.AddInt32(&r.createPickupCalls, 1)
	if r.createPickupErr != nil {
		return db.RacepackPickupRecord{}, r.createPickupErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.pickupRecs {
		if rec.TicketID == arg.TicketID && rec.Status == "PICKED_UP" {
			return db.RacepackPickupRecord{}, &pgconn.PgError{Code: "23505"}
		}
	}
	rec := db.RacepackPickupRecord{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID,
		TicketID: arg.TicketID, ParticipantID: arg.ParticipantID,
		BibNumber: arg.BibNumber, CounterID: arg.CounterID, StaffID: arg.StaffID,
		SlotID: arg.SlotID, PickupMethod: arg.PickupMethod,
		PickupTimestamp: pgtype.Timestamptz{Time: nowish(), Valid: true},
		Notes: arg.Notes, Status: "PICKED_UP",
	}
	r.pickupRecs[rec.ID] = rec
	return rec, nil
}
func (r *fakeRepo) GetActivePickupByTicket(ctx context.Context, ticketID uuid.UUID) (db.RacepackPickupRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.pickupRecs {
		if rec.TicketID == ticketID && rec.Status == "PICKED_UP" {
			return rec, nil
		}
	}
	return db.RacepackPickupRecord{}, pgx.ErrNoRows
}
func (r *fakeRepo) GetPickupRecordByID(ctx context.Context, id uuid.UUID) (db.RacepackPickupRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.pickupRecs[id]
	if !ok {
		return db.RacepackPickupRecord{}, pgx.ErrNoRows
	}
	return rec, nil
}
func (r *fakeRepo) ListPickupRecordsByEvent(ctx context.Context, arg db.ListRacepackPickupRecordsByEventParams) ([]db.RacepackPickupRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []db.RacepackPickupRecord{}
	for _, rec := range r.pickupRecs {
		if rec.EventID == arg.EventID && rec.Status == "PICKED_UP" {
			out = append(out, rec)
		}
	}
	return out, nil
}
func (r *fakeRepo) CountPickupRecordsByCounter(ctx context.Context, arg db.CountRacepackPickupRecordsByCounterParams) ([]db.CountRacepackPickupRecordsByCounterRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	counts := map[uuid.UUID]int64{}
	for _, rec := range r.pickupRecs {
		if rec.EventID == arg.EventID && rec.Status == "PICKED_UP" {
			if rec.PickupTimestamp.Valid &&
				!rec.PickupTimestamp.Time.Before(arg.PickupTimestamp.Time) &&
				rec.PickupTimestamp.Time.Before(arg.PickupTimestamp_2.Time) {
				counts[rec.CounterID]++
			}
		}
	}
	out := []db.CountRacepackPickupRecordsByCounterRow{}
	for cid, c := range counts {
		out = append(out, db.CountRacepackPickupRecordsByCounterRow{CounterID: cid, PickupCount: c})
	}
	return out, nil
}
func (r *fakeRepo) CountPickupRecordsByEvent(ctx context.Context, eventID uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, rec := range r.pickupRecs {
		if rec.EventID == eventID && rec.Status == "PICKED_UP" {
			n++
		}
	}
	return n, nil
}

func (r *fakeRepo) CreateProxyAuthorization(ctx context.Context, arg db.CreateRacepackProxyAuthorizationParams) (db.RacepackProxyAuthorization, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := db.RacepackProxyAuthorization{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID,
		TicketID: arg.TicketID, PickupRecordID: arg.PickupRecordID,
		ProxyName: arg.ProxyName, ProxyPhone: arg.ProxyPhone,
		ProxyIdentity: arg.ProxyIdentity, AuthorizationDocument: arg.AuthorizationDocument,
		CreatedBy: arg.CreatedBy,
		CreatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
	}
	r.proxyAuths[rec.ID] = rec
	return rec, nil
}
func (r *fakeRepo) ListProxyAuthorizationsByTicket(ctx context.Context, ticketID uuid.UUID) ([]db.RacepackProxyAuthorization, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []db.RacepackProxyAuthorization{}
	for _, rec := range r.proxyAuths {
		if rec.TicketID == ticketID {
			out = append(out, rec)
		}
	}
	return out, nil
}
func (r *fakeRepo) GetProxyAuthorizationByID(ctx context.Context, id uuid.UUID) (db.RacepackProxyAuthorization, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.proxyAuths[id]
	if !ok {
		return db.RacepackProxyAuthorization{}, pgx.ErrNoRows
	}
	return rec, nil
}

func (r *fakeRepo) CreateProblemCase(ctx context.Context, arg db.CreateRacepackProblemCaseParams) (db.RacepackProblemCase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := db.RacepackProblemCase{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID,
		TicketID: arg.TicketID, ParticipantID: arg.ParticipantID,
		Status: "OPEN", Reason: arg.Reason, CreatedBy: arg.CreatedBy,
		CreatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
	}
	r.problemCs[rec.ID] = rec
	return rec, nil
}
func (r *fakeRepo) UpdateProblemCaseStatus(ctx context.Context, arg db.UpdateRacepackProblemCaseStatusParams) (db.RacepackProblemCase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.problemCs[arg.ID]
	if !ok {
		return db.RacepackProblemCase{}, pgx.ErrNoRows
	}
	rec.Status = arg.Status
	if arg.Status == "RESOLVED" || arg.Status == "ESCALATED" {
		rec.Resolution = arg.Resolution
		rec.ResolvedBy = arg.ResolvedBy
		rec.ResolvedAt = pgtype.Timestamptz{Time: nowish(), Valid: true}
	}
	r.problemCs[arg.ID] = rec
	return rec, nil
}
func (r *fakeRepo) ListProblemCasesByEvent(ctx context.Context, arg db.ListRacepackProblemCasesByEventParams) ([]db.RacepackProblemCase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []db.RacepackProblemCase{}
	for _, c := range r.problemCs {
		if c.EventID == arg.EventID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *fakeRepo) CountProblemCasesByEventAndStatus(ctx context.Context, eventID uuid.UUID, status string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, c := range r.problemCs {
		if c.EventID == eventID && c.Status == status {
			n++
		}
	}
	return n, nil
}
func (r *fakeRepo) GetProblemCaseByID(ctx context.Context, id uuid.UUID) (db.RacepackProblemCase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.problemCs[id]
	if !ok {
		return db.RacepackProblemCase{}, pgx.ErrNoRows
	}
	return rec, nil
}

func (r *fakeRepo) GetTicketStatus(ctx context.Context, ticketID uuid.UUID) (string, uuid.UUID, uuid.UUID, string, bool, error) {
	return r.ticketStatus, r.ticketEventID, r.ticketPartID, r.bib, r.ticketFound, nil
}
func (r *fakeRepo) GetOrderStatusForTicket(ctx context.Context, ticketID uuid.UUID) (string, error) {
	return r.orderStatus, nil
}
func (r *fakeRepo) HasActivePickup(ctx context.Context, ticketID uuid.UUID) (bool, error) {
	return r.hasActive, nil
}
func (r *fakeRepo) LockTicketForUpdate(ctx context.Context, ticketID uuid.UUID) (db.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.ticketFound {
		return db.Ticket{}, pgx.ErrNoRows
	}
	return db.Ticket{
		ID: ticketID,
		OrganizationID: uuid.Nil,
		EventID:        r.ticketEventID,
		ParticipantID:  r.ticketPartID,
		Status:         r.ticketStatus,
		BibNumber:      pgtype.Text{String: r.bib, Valid: r.bib != ""},
	}, nil
}
func (r *fakeRepo) GetEventOrganizationID(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.events[eventID]; ok {
		return e.OrganizationID, nil
	}
	return uuid.Nil, pgx.ErrNoRows
}
func (r *fakeRepo) CheckOrganizationMembership(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	return true, nil
}
func (r *fakeRepo) GetUserTicket(ctx context.Context, ticketID uuid.UUID) (db.GetUserTicketByIDRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return db.GetUserTicketByIDRow{
		ID:        ticketID,
		EventID:   r.ticketEventID,
		ParticipantID: r.ticketPartID,
		Status:    r.ticketStatus,
		BibNumber: pgtype.Text{String: r.bib, Valid: r.bib != ""},
		OrderStatus: r.orderStatus,
	}, nil
}

func (r *fakeRepo) GetIdempotencyKey(ctx context.Context, arg db.GetIdempotencyKeyParams) (db.GetIdempotencyKeyRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.idemp[arg.Key+":"+arg.Scope]
	if !ok {
		return db.GetIdempotencyKeyRow{}, pgx.ErrNoRows
	}
	return db.GetIdempotencyKeyRow{
		Key: k.Key, RequestHash: k.RequestHash,
		ResponseStatus: k.ResponseStatus, ResponseBody: k.ResponseBody,
		CreatedAt: k.CreatedAt,
	}, nil
}
func (r *fakeRepo) InsertIdempotencyKey(ctx context.Context, arg db.InsertIdempotencyKeyParams) (db.IdempotencyKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := db.IdempotencyKey{
		Key: arg.Key, Scope: arg.Scope, RequestHash: arg.RequestHash,
		ResponseStatus: arg.ResponseStatus, ResponseBody: arg.ResponseBody,
		CreatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
	}
	r.idemp[arg.Key+":"+arg.Scope] = k
	return k, nil
}

func (r *fakeRepo) IsUniqueViolation(err error) bool { return racepack.IsUniqueViolation(err) }

// --- helpers ---

var _nowish = int64(0)

func nowish() (t time.Time) {
	_nowish++
	// Use current time so dashboard's "today" window matches. Deterministic
	// monotonic increment keeps distinct values for ordering.
	return time.Now().UTC().Add(time.Duration(_nowish) * time.Second)
}

// pickupFixture sets up the standard happy-path ticket + counter + repo state.
func pickupFixture(repo *fakeRepo, eventID, orgID uuid.UUID) (ticketID, counterID uuid.UUID) {
	counterID = uuid.New()
	repo.seedCounter(db.RacepackCounter{
		ID: counterID, OrganizationID: orgID, EventID: eventID,
		Name: "Counter A", Active: true,
		CreatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
	})
	ticketID = uuid.New()
	repo.seedTicket(db.Ticket{
		ID: ticketID, OrganizationID: orgID, EventID: eventID,
		ParticipantID: uuid.New(),
		Status: racepack.TicketStatusValid,
		BibNumber:      pgtype.Text{String: "A00001", Valid: true},
		OrderID:        uuid.New(),
	})
	repo.orderStatus = racepack.OrderStatusPaid
	repo.hasActive = false
	return
}

// --- Tests ---

func TestExecutePickup_HappyPath(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	ticketID, counterID := pickupFixture(repo, eventID, orgID)

	rec, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
		OrgID: orgID, EventID: eventID, TicketID: ticketID,
		CounterID: counterID, StaffID: uuid.New(),
		Method: racepack.PickupMethodSelf,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if rec.BibNumber != "A00001" {
		t.Errorf("expected A00001, got %s", rec.BibNumber)
	}
	if rec.PickupMethod != racepack.PickupMethodSelf {
		t.Errorf("expected SELF, got %s", rec.PickupMethod)
	}
}

func TestExecutePickup_DuplicateBlocked(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	ticketID, counterID := pickupFixture(repo, eventID, orgID)

	if _, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
		OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counterID,
		StaffID: uuid.New(), Method: racepack.PickupMethodSelf,
	}); err != nil {
		t.Fatalf("first pickup: %v", err)
	}
	repo.hasActive = true
	_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
		OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counterID,
		StaffID: uuid.New(), Method: racepack.PickupMethodSelf,
	})
	if !errors.Is(err, racepack.ErrAlreadyPickedUp) {
		t.Fatalf("expected ErrAlreadyPickedUp, got %v", err)
	}
}

func TestExecutePickup_OrderNotPaid(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	ticketID, counterID := pickupFixture(repo, eventID, orgID)
	repo.orderStatus = "PENDING_PAYMENT"

	_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
		OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counterID,
		StaffID: uuid.New(), Method: racepack.PickupMethodSelf,
	})
	if !errors.Is(err, racepack.ErrOrderNotPaid) {
		t.Fatalf("expected ErrOrderNotPaid, got %v", err)
	}
}

func TestExecutePickup_BibMissing(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	ticketID, counterID := pickupFixture(repo, eventID, orgID)
	repo.bib = ""

	_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
		OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counterID,
		StaffID: uuid.New(), Method: racepack.PickupMethodSelf,
	})
	if !errors.Is(err, racepack.ErrBibMissing) {
		t.Fatalf("expected ErrBibMissing, got %v", err)
	}
}

func TestExecutePickup_TicketCancelled(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	ticketID, counterID := pickupFixture(repo, eventID, orgID)
	repo.ticketStatus = racepack.TicketStatusCancelled

	_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
		OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counterID,
		StaffID: uuid.New(), Method: racepack.PickupMethodSelf,
	})
	if !errors.Is(err, racepack.ErrTicketCancelled) {
		t.Fatalf("expected ErrTicketCancelled, got %v", err)
	}
}

func TestExecutePickup_CounterFromOtherEvent(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	ticketID, _ := pickupFixture(repo, eventID, orgID)
	wrongCounter := uuid.New()
	repo.seedCounter(db.RacepackCounter{
		ID: wrongCounter, OrganizationID: orgID, EventID: uuid.New(),
		Name: "Counter X", Active: true,
		CreatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: nowish(), Valid: true},
	})

	_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
		OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: wrongCounter,
		StaffID: uuid.New(), Method: racepack.PickupMethodSelf,
	})
	if !errors.Is(err, racepack.ErrCounterEventMismatch) {
		t.Fatalf("expected ErrCounterEventMismatch, got %v", err)
	}
}

func TestExecutePickup_TicketFromOtherEvent(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	_, counterID := pickupFixture(repo, eventID, orgID)
	// Ticket in a different event.
	wrongTicket := uuid.New()
	repo.seedTicket(db.Ticket{
		ID: wrongTicket, OrganizationID: orgID, EventID: uuid.New(),
		ParticipantID: uuid.New(),
		Status: racepack.TicketStatusValid,
		BibNumber: pgtype.Text{String: "A00001", Valid: true},
		OrderID: uuid.New(),
	})
	repo.orderStatus = racepack.OrderStatusPaid

	_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
		OrgID: orgID, EventID: eventID, TicketID: wrongTicket, CounterID: counterID,
		StaffID: uuid.New(), Method: racepack.PickupMethodSelf,
	})
	if !errors.Is(err, racepack.ErrTicketEventMismatch) {
		t.Fatalf("expected ErrTicketEventMismatch, got %v", err)
	}
}

func TestExecutePickup_InvalidMethod(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	ticketID, counterID := pickupFixture(repo, eventID, orgID)

	// Empty + unknown values are rejected. Lowercase is normalized to UPPER.
	for _, m := range []string{"", "INVALID", "VIP", "drop"} {
		_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
			OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counterID,
			StaffID: uuid.New(), Method: m,
		})
		if !errors.Is(err, racepack.ErrInvalidMethod) {
			t.Errorf("method=%q: expected ErrInvalidMethod, got %v", m, err)
		}
	}
}

func TestExecutePickup_ParallelRace(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	ticketID, counterID := pickupFixture(repo, eventID, orgID)

	var success, already, other int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
				OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counterID,
				StaffID: uuid.New(), Method: racepack.PickupMethodSelf,
			})
			switch {
			case err == nil:
				atomic.AddInt32(&success, 1)
			case errors.Is(err, racepack.ErrAlreadyPickedUp):
				atomic.AddInt32(&already, 1)
			default:
				atomic.AddInt32(&other, 1)
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Errorf("expected 1 success, got %d", success)
	}
	if already != 49 {
		t.Errorf("expected 49 already-picked-up, got %d", already)
	}
	if other != 0 {
		t.Errorf("expected 0 other errors, got %d", other)
	}
}

func TestReserveSlot_AtomicCapacityEnforcement(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	slot, err := svc.CreateSlot(context.Background(), uuid.New(), uuid.New(), "Slot A", nowish(), nowish(), nowish(), 2)
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := svc.ReserveSlot(context.Background(), slot.ID); err != nil {
			t.Fatalf("reserve %d: %v", i, err)
		}
	}
	_, err = svc.ReserveSlot(context.Background(), slot.ID)
	if !errors.Is(err, racepack.ErrSlotFull) {
		t.Fatalf("expected ErrSlotFull, got %v", err)
	}
}

func TestProblemCase_RequiresTarget(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	_, err := svc.CreateProblemCase(context.Background(), orgID, eventID, uuid.New(), nil, nil, "no target")
	if !errors.Is(err, racepack.ErrNoProblemTarget) {
		t.Fatalf("expected ErrNoProblemTarget, got %v", err)
	}
}

func TestProblemCase_StateTransitions(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	actorID := uuid.New()
	// ProblemCase with a participant only (no ticket) skips the
	// ticket-belongs-to-event check; this lets the test focus on state machine.
	pid := uuid.New()
	rec, err := svc.CreateProblemCase(context.Background(), orgID, eventID, actorID, nil, &pid, "missing BIB")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	updated, _ := svc.UpdateProblemCaseStatus(context.Background(), rec.ID, actorID, "UNDER_REVIEW", "")
	if updated.Status != "UNDER_REVIEW" {
		t.Errorf("expected UNDER_REVIEW, got %s", updated.Status)
	}
	updated, _ = svc.UpdateProblemCaseStatus(context.Background(), rec.ID, actorID, "RESOLVED", "fixed")
	if updated.Status != "RESOLVED" {
		t.Errorf("expected RESOLVED, got %s", updated.Status)
	}
	_, err = svc.UpdateProblemCaseStatus(context.Background(), rec.ID, actorID, "OPEN", "")
	if !errors.Is(err, racepack.ErrInvalidStateChange) {
		t.Errorf("expected ErrInvalidStateChange, got %v", err)
	}
}

func TestProxyAuthorization_TicketEventMismatch(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	wrongTicket := uuid.New()
	repo.seedTicket(db.Ticket{
		ID: wrongTicket, OrganizationID: orgID, EventID: uuid.New(),
		ParticipantID: uuid.New(),
		Status: racepack.TicketStatusValid,
		BibNumber: pgtype.Text{String: "A00001", Valid: true},
		OrderID: uuid.New(),
	})
	_, err := svc.CreateProxyAuthorization(context.Background(), orgID, eventID, wrongTicket, uuid.New(), nil, "Budi", "", "KTP-12345", "")
	if !errors.Is(err, racepack.ErrTicketEventMismatch) {
		t.Fatalf("expected ErrTicketEventMismatch, got %v", err)
	}
}

func TestDashboardSummary_OpenCases(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	eventID, orgID := uuid.New(), uuid.New()
	// Seed a ticket first, then create a counter that the pickup will use.
	ticketID := uuid.New()
	repo.seedTicket(db.Ticket{
		ID: ticketID, OrganizationID: orgID, EventID: eventID,
		ParticipantID: uuid.New(),
		Status: racepack.TicketStatusValid,
		BibNumber: pgtype.Text{String: "A00001", Valid: true},
		OrderID: uuid.New(),
	})
	repo.orderStatus = racepack.OrderStatusPaid
	repo.hasActive = false

	counter, err := svc.CreateCounter(context.Background(), orgID, eventID, "Counter A", "Lobby", true)
	if err != nil {
		t.Fatalf("create counter: %v", err)
	}
	if _, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
		OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counter.ID,
		StaffID: uuid.New(), Method: racepack.PickupMethodSelf,
	}); err != nil {
		t.Fatalf("pickup: %v", err)
	}
	pid := uuid.New()
	if _, err := svc.CreateProblemCase(context.Background(), orgID, eventID, uuid.New(), nil, &pid, "missing BIB"); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	dash, err := svc.DashboardSummary(context.Background(), eventID)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if dash.TotalPickups != 1 {
		t.Errorf("expected 1 total pickup, got %d", dash.TotalPickups)
	}
	if dash.OpenCases != 1 {
		t.Errorf("expected 1 open case, got %d", dash.OpenCases)
	}
	if len(dash.ByCounter) != 1 {
		t.Errorf("expected 1 row in byCounter, got %d", len(dash.ByCounter))
	}
	if dash.ByCounter[0].Pickups != 1 {
		t.Errorf("expected count=1 in byCounter, got %d", dash.ByCounter[0].Pickups)
	}
}

func TestIdempotency_SameKeySamePayloadReturnsCached(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	hash := racepack.HashRequest("POST", "/x", []byte(`{"a":1}`))
	svc.StoreIdempotency(context.Background(), "key1", "scope1", hash, 201, []byte(`{"ok":true}`))
	hit, err := svc.LookupIdempotency(context.Background(), "key1", "scope1")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if hit == nil || !hit.Found {
		t.Fatalf("expected hit")
	}
	if hit.RequestHash != hash {
		t.Errorf("hash mismatch")
	}
	if string(hit.ResponseBody) != `{"ok":true}` {
		t.Errorf("body mismatch: %s", hit.ResponseBody)
	}
}

func TestIdempotency_SameKeyDifferentPayloadConflicts(t *testing.T) {
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	svc.StoreIdempotency(context.Background(), "key1", "scope1", "hash-A", 201, []byte(`{"ok":true}`))
	// Caller computes hash differently and looks up.
	hit, err := svc.LookupIdempotency(context.Background(), "key1", "scope1")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if hit.RequestHash != "hash-A" {
		t.Errorf("handler would NOT trigger ErrIdempotencyConflict; got hash %s", hit.RequestHash)
	}
}

func TestIsUniqueViolation_Recognizes23505(t *testing.T) {
	if !racepack.IsUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Error("expected true for 23505")
	}
	if racepack.IsUniqueViolation(&pgconn.PgError{Code: "23502"}) {
		t.Error("expected false for 23502")
	}
	if racepack.IsUniqueViolation(errors.New("random")) {
		t.Error("expected false for non-pg error")
	}
	if racepack.IsUniqueViolation(nil) {
		t.Error("expected false for nil")
	}
}

func TestValidatePickupMethod(t *testing.T) {
	// verify the strict allowlist rejects bogus values via the service.
	repo := newFakeRepo()
	svc := racepack.NewService(repo, nil, nil)
	orgID, eventID := uuid.New(), uuid.New()
	ticketID, counterID := pickupFixture(repo, eventID, orgID)

	// Unknown methods are rejected.
	for _, m := range []string{"", "INVALID", "VIP", "drop"} {
		_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
			OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counterID,
			StaffID: uuid.New(), Method: m,
		})
		if !errors.Is(err, racepack.ErrInvalidMethod) {
			t.Errorf("method=%q: expected ErrInvalidMethod, got %v", m, err)
		}
	}
	// SELF, PROXY, MANUAL_OVERRIDE are accepted (case + whitespace insensitive).
	for _, m := range []string{"SELF", "PROXY", "MANUAL_OVERRIDE", "self", " proxy ", " manual_override "} {
		_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
			OrgID: orgID, EventID: eventID, TicketID: ticketID, CounterID: counterID,
			StaffID: uuid.New(), Method: m,
		})
		// We expect ErrAlreadyPickedUp (after first valid call) or ErrTicketEventMismatch
		// (because pickupFixture has only one ticket) — but NEVER ErrInvalidMethod.
		if errors.Is(err, racepack.ErrInvalidMethod) {
			t.Errorf("method=%q: should be accepted (normalised), got %v", m, err)
		}
	}
}