package tickets_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets"
)

// bibFakeRepo implements tickets.Repository for unit tests of the BIB service.
// It is richer than fakeRepo: it tracks state, simulates unique-constraint
// collisions for race testing, and bumps a synthetic MAX sequence.
type bibFakeRepo struct {
	mu sync.Mutex

	tickets    map[uuid.UUID]db.Ticket
	nextSeq    int64
	conflictOn int // first N AssignBib calls per repo return 23505

	assignCalls int32
	assigned    []db.Ticket
	cleared     []db.Ticket
}

func newBibFakeRepo() *bibFakeRepo {
	return &bibFakeRepo{tickets: map[uuid.UUID]db.Ticket{}}
}

func (r *bibFakeRepo) putTicket(t db.Ticket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tickets[t.ID] = t
}

func (r *bibFakeRepo) ExecTx(ctx context.Context, fn func(tickets.Repository) error) error {
	return fn(r)
}

func (r *bibFakeRepo) CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error) {
	panic("CreateTicket not exercised by BIB tests")
}
func (r *bibFakeRepo) GetTicketByID(ctx context.Context, id uuid.UUID) (db.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tickets[id]
	if !ok {
		return db.Ticket{}, pgx.ErrNoRows
	}
	return t, nil
}
func (r *bibFakeRepo) GetTicketByOrderID(ctx context.Context, orderID uuid.UUID) (db.Ticket, error) {
	panic("GetTicketByOrderID not exercised by BIB tests")
}
func (r *bibFakeRepo) ListTicketsByParticipant(ctx context.Context, participantID uuid.UUID) ([]db.Ticket, error) {
	panic("ListTicketsByParticipant not exercised by BIB tests")
}
func (r *bibFakeRepo) ListTicketsByEvent(ctx context.Context, arg db.ListTicketsByEventParams) ([]db.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []db.Ticket{}
	for _, t := range r.tickets {
		if t.EventID == arg.EventID {
			out = append(out, t)
		}
	}
	return out, nil
}
func (r *bibFakeRepo) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	panic("GetUserByID not exercised by BIB tests")
}
func (r *bibFakeRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	panic("GetEventByID not exercised by BIB tests")
}
func (r *bibFakeRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	panic("GetCategoryByID not exercised by BIB tests")
}
func (r *bibFakeRepo) GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error) {
	panic("GetOrderByID not exercised by BIB tests")
}

// AssignBib — simulate unique-constraint violation on the first N calls per repo.
func (r *bibFakeRepo) AssignBib(ctx context.Context, ticketID uuid.UUID, bib string, assignedBy uuid.UUID, method string) (db.Ticket, error) {
	atomic.AddInt32(&r.assignCalls, 1)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conflictOn > 0 {
		r.conflictOn--
		return db.Ticket{}, &pgconn.PgError{Code: "23505"}
	}
	t, ok := r.tickets[ticketID]
	if !ok {
		return db.Ticket{}, pgx.ErrNoRows
	}
	t.BibNumber = pgtype.Text{String: bib, Valid: true}
	t.BibAssignmentMethod = pgtype.Text{String: method, Valid: true}
	t.BibAssignedBy = &assignedBy
	r.tickets[ticketID] = t
	r.assigned = append(r.assigned, t)
	if isAllDigits(bib) {
		var n int64
		for _, c := range bib {
			n = n*10 + int64(c-'0')
		}
		if n > r.nextSeq {
			r.nextSeq = n
		}
	}
	return t, nil
}

func (r *bibFakeRepo) ClearBib(ctx context.Context, ticketID uuid.UUID) (db.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tickets[ticketID]
	if !ok {
		return db.Ticket{}, pgx.ErrNoRows
	}
	t.BibNumber = pgtype.Text{}
	t.BibAssignedBy = nil
	t.BibAssignmentMethod = pgtype.Text{}
	r.tickets[ticketID] = t
	r.cleared = append(r.cleared, t)
	return t, nil
}

func (r *bibFakeRepo) GetNextBibNumeric(ctx context.Context, eventID uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.nextSeq, nil
}

func (r *bibFakeRepo) ListUnassignedTicketsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []db.Ticket{}
	for _, t := range r.tickets {
		if t.EventID == eventID && !t.BibNumber.Valid && t.Status == "VALID" {
			out = append(out, t)
		}
	}
	return out, nil
}

// --- helpers ---

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func bibSampleTicket(eventID uuid.UUID) db.Ticket {
	return db.Ticket{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		EventID:        eventID,
		CategoryID:     uuid.New(),
		OrderID:        uuid.New(),
		ParticipantID:  uuid.New(),
		TicketNumber:   "TIX-20260619-ABCDEF",
		Status:         "VALID",
		HolderName:     "Pelari Satu",
		HolderEmail:    "p@example.com",
		EventTitle:     "Jakarta Run 2026",
		CategoryName:   "10K",
	}
}

// --- Tests ---

func TestBib_AssignNextBib_Sequential(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()

	a := bibSampleTicket(eventID)
	repo.putTicket(a)
	b := bibSampleTicket(eventID)
	repo.putTicket(b)

	out1, err := svc.AssignNextBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New())
	if err != nil {
		t.Fatalf("first assign: %v", err)
	}
	if out1.BibNumber == nil || *out1.BibNumber != "00001" {
		t.Errorf("expected 00001, got %v", out1.BibNumber)
	}

	out2, err := svc.AssignNextBib(context.Background(), a.OrganizationID, eventID, b.ID, uuid.New())
	if err != nil {
		t.Fatalf("second assign: %v", err)
	}
	if out2.BibNumber == nil || *out2.BibNumber != "00002" {
		t.Errorf("expected 00002, got %v", out2.BibNumber)
	}
}

func TestBib_SetBib_ManualThenOverride(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()
	a := bibSampleTicket(eventID)
	repo.putTicket(a)

	out, err := svc.SetBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New(), "A00042")
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if out.BibAssignmentMethod == nil || *out.BibAssignmentMethod != "MANUAL" {
		t.Errorf("expected MANUAL, got %v", out.BibAssignmentMethod)
	}
	if out.BibNumber == nil || *out.BibNumber != "A00042" {
		t.Errorf("expected A00042, got %v", out.BibNumber)
	}

	// override existing — should switch to OVERRIDE
	out2, err := svc.SetBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New(), "A00099")
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if out2.BibAssignmentMethod == nil || *out2.BibAssignmentMethod != "OVERRIDE" {
		t.Errorf("expected OVERRIDE, got %v", out2.BibAssignmentMethod)
	}
}

func TestBib_ClearBib(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()
	a := bibSampleTicket(eventID)
	repo.putTicket(a)

	if _, err := svc.SetBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New(), "X001"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, err := svc.ClearBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New())
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if out.BibNumber != nil {
		t.Errorf("expected bib nil after clear, got %v", out.BibNumber)
	}
}

func TestBib_PreviewNextBib(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()

	prev, err := svc.PreviewNextBib(context.Background(), eventID)
	if err != nil {
		t.Fatalf("preview empty: %v", err)
	}
	if prev.NextBib != "00001" || prev.NumericNext != 1 {
		t.Errorf("expected 00001/1, got %s/%d", prev.NextBib, prev.NumericNext)
	}

	a := bibSampleTicket(eventID)
	repo.putTicket(a)
	if _, err := svc.AssignNextBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New()); err != nil {
		t.Fatalf("assign: %v", err)
	}
	prev, err = svc.PreviewNextBib(context.Background(), eventID)
	if err != nil {
		t.Fatalf("preview after assign: %v", err)
	}
	if prev.NextBib != "00002" || prev.NumericNext != 2 {
		t.Errorf("expected 00002/2, got %s/%d", prev.NextBib, prev.NumericNext)
	}
}

func TestBib_AssignNextBib_RetryOnUniqueViolation(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()
	a := bibSampleTicket(eventID)
	repo.putTicket(a)

	repo.conflictOn = 3
	out, err := svc.AssignNextBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New())
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if out.BibNumber == nil {
		t.Fatalf("expected bib assigned")
	}
	if got := atomic.LoadInt32(&repo.assignCalls); got < 4 {
		t.Errorf("expected ≥4 AssignBib attempts, got %d", got)
	}
}

func TestBib_AssignNextBib_Exhausted(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()
	a := bibSampleTicket(eventID)
	repo.putTicket(a)

	repo.conflictOn = 99
	_, err := svc.AssignNextBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New())
	if err == nil || !errors.Is(err, tickets.ErrBibAssignExhausted) {
		t.Fatalf("expected ErrBibAssignExhausted, got %v", err)
	}
}

func TestBib_AssignNextBib_Parallel(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()
	a := bibSampleTicket(eventID)
	repo.putTicket(a)
	// 49 losing races; the 50th attempt wins.
	repo.conflictOn = 49

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.AssignNextBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New())
		}()
	}
	wg.Wait()

	got, err := repo.GetTicketByID(context.Background(), a.ID)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !got.BibNumber.Valid {
		t.Fatalf("expected bib assigned after parallel race")
	}
}

func TestBib_BulkAssignBib(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()

	var firstOrg uuid.UUID
	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		tk := bibSampleTicket(eventID)
		if i == 0 {
			firstOrg = tk.OrganizationID
		}
		repo.putTicket(tk)
		ids = append(ids, tk.ID)
	}
	res, err := svc.BulkAssignBib(context.Background(), firstOrg, eventID, uuid.New())
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if res.Assigned != 5 {
		t.Errorf("expected 5 assigned, got %d", res.Assigned)
	}
	if res.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", res.Failed)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		tk, _ := repo.GetTicketByID(context.Background(), id)
		if !tk.BibNumber.Valid {
			t.Fatalf("ticket %s missing bib", id)
		}
		b := tk.BibNumber.String
		if seen[b] {
			t.Fatalf("duplicate bib %s in bulk result", b)
		}
		seen[b] = true
	}
}

func TestBib_SanitizeBib(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()
	a := bibSampleTicket(eventID)
	repo.putTicket(a)

	cases := []struct {
		in     string
		wantOK bool
	}{
		{"A00042", true},
		{"  A00042  ", true},
		{"", false},
		{"with space", false},
		{"with;semicolon", false},
		{"0123456789012345678901234567890123", false}, // 34 chars (limit 32)
	}
	for _, c := range cases {
		_, err := svc.SetBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New(), c.in)
		if c.wantOK && err != nil {
			t.Errorf("input %q should pass, got %v", c.in, err)
		}
		if !c.wantOK && err == nil {
			t.Errorf("input %q should fail", c.in)
		}
	}
}

func TestBib_IsUniqueViolation(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505"}
	if !tickets.IsUniqueViolation(pgErr) {
		t.Error("expected true for 23505")
	}
	other := &pgconn.PgError{Code: "23502"}
	if tickets.IsUniqueViolation(other) {
		t.Error("expected false for 23502")
	}
	if tickets.IsUniqueViolation(errors.New("random")) {
		t.Error("expected false for non-pg error")
	}
	if tickets.IsUniqueViolation(nil) {
		t.Error("expected false for nil")
	}
}

func TestBib_StreamTicketsForBibExport(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()

	t1 := bibSampleTicket(eventID)
	t1.HolderName = "Alpha"
	t1.HolderEmail = "a@example.com"
	t1.BibNumber = pgtype.Text{String: "00001", Valid: true}
	t1.Status = "VALID"
	repo.putTicket(t1)
	t2 := bibSampleTicket(eventID)
	t2.HolderName = "Bravo"
	t2.HolderEmail = "b@example.com"
	t2.Status = "VALID"
	repo.putTicket(t2)

	rows := []tickets.TicketExportRow{}
	err := svc.StreamTicketsForBibExport(context.Background(), t1.OrganizationID, eventID, func(row tickets.TicketExportRow) error {
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// Alpha is the one with BIB assigned
	var alpha, bravo *tickets.TicketExportRow
	for i := range rows {
		if rows[i].HolderName == "Alpha" {
			alpha = &rows[i]
		}
		if rows[i].HolderName == "Bravo" {
			bravo = &rows[i]
		}
	}
	if alpha == nil || bravo == nil {
		t.Fatalf("missing row")
	}
	if alpha.BibNumber != "00001" {
		t.Errorf("alpha bib: %s", alpha.BibNumber)
	}
	if bravo.BibNumber != "" {
		t.Errorf("bravo should have empty bib, got %s", bravo.BibNumber)
	}
}

func TestBib_BibConflictErrorSurfaced(t *testing.T) {
	repo := newBibFakeRepo()
	svc := tickets.NewService(repo, nil, nil)
	eventID := uuid.New()
	a := bibSampleTicket(eventID)
	repo.putTicket(a)

	// Every SetBib attempt collides — should surface as a conflict error.
	repo.conflictOn = 99
	_, err := svc.SetBib(context.Background(), a.OrganizationID, eventID, a.ID, uuid.New(), "A00042")
	if err == nil {
		t.Fatalf("expected conflict, got nil")
	}
	if !containsSubstr(err.Error(), "bib_conflict") && !containsSubstr(err.Error(), "23505") {
		t.Errorf("expected bib_conflict/23505, got %v", err)
	}
}