package reporting_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/reporting"
	"github.com/varin/ivyticketing/services/api/internal/platform/storage"
)

// --- fake storage ---

type fakeStore struct {
	mu     sync.Mutex
	puts   map[string][]byte
	putErr error
}

func newFakeStore() *fakeStore { return &fakeStore{puts: map[string][]byte{}} }

func (s *fakeStore) PresignUpload(ctx context.Context, key, contentType string, ttl time.Duration) (storage.PutTicket, bool, error) {
	return storage.PutTicket{}, false, nil
}
func (s *fakeStore) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	if s.putErr != nil {
		return s.putErr
	}
	b, _ := io.ReadAll(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts[key] = b
	return nil
}
func (s *fakeStore) PublicURL(key string) string           { return "https://cdn.test/" + key }
func (s *fakeStore) Delete(ctx context.Context, key string) error { return nil }

// --- fake repository ---

type fakeRepo struct {
	mu sync.Mutex

	pending  []db.ExportJob
	byID     map[uuid.UUID]db.ExportJob
	revenue  []db.RevenueReportRowsRow
	created  int
	readyIDs []uuid.UUID
	failed   map[uuid.UUID]string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[uuid.UUID]db.ExportJob{}, failed: map[uuid.UUID]string{}}
}

func (r *fakeRepo) CreateExportJob(ctx context.Context, arg db.CreateExportJobParams) (db.ExportJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created++
	job := db.ExportJob{
		ID:             uuid.New(),
		OrganizationID: arg.OrganizationID,
		EventID:        arg.EventID,
		RequestedBy:    arg.RequestedBy,
		ReportType:     arg.ReportType,
		Format:         arg.Format,
		Status:         reporting.JobStatusPending,
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	r.byID[job.ID] = job
	r.pending = append(r.pending, job)
	return job, nil
}

func (r *fakeRepo) GetExportJob(ctx context.Context, id, orgID uuid.UUID) (db.ExportJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.byID[id]
	if !ok || j.OrganizationID != orgID {
		return db.ExportJob{}, pgx.ErrNoRows
	}
	return j, nil
}

func (r *fakeRepo) ListExportJobsByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.ExportJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []db.ExportJob{}
	for _, j := range r.byID {
		if j.OrganizationID == orgID {
			out = append(out, j)
		}
	}
	return out, nil
}

func (r *fakeRepo) ClaimPendingExportJob(ctx context.Context) (db.ExportJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) == 0 {
		return db.ExportJob{}, pgx.ErrNoRows
	}
	j := r.pending[0]
	r.pending = r.pending[1:]
	return j, nil
}

func (r *fakeRepo) MarkExportJobReady(ctx context.Context, id uuid.UUID, rowCount int32, fileKey, fileURL string) (db.ExportJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readyIDs = append(r.readyIDs, id)
	j := r.byID[id]
	j.Status = reporting.JobStatusReady
	j.RowCount = pgtype.Int4{Int32: rowCount, Valid: true}
	j.FileUrl = pgtype.Text{String: fileURL, Valid: true}
	r.byID[id] = j
	return j, nil
}

func (r *fakeRepo) MarkExportJobFailed(ctx context.Context, id uuid.UUID, errMsg string) (db.ExportJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed[id] = errMsg
	j := r.byID[id]
	j.Status = reporting.JobStatusFailed
	r.byID[id] = j
	return j, nil
}

// Summaries — only revenue is exercised; the rest satisfy the interface.
func (r *fakeRepo) ParticipantSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.ParticipantReportSummaryRow, error) {
	return db.ParticipantReportSummaryRow{}, nil
}
func (r *fakeRepo) SalesSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.SalesReportSummaryRow, error) {
	return db.SalesReportSummaryRow{}, nil
}
func (r *fakeRepo) PaymentSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.PaymentReportSummaryRow, error) {
	return nil, nil
}
func (r *fakeRepo) CouponSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.CouponReportSummaryRow, error) {
	return nil, nil
}
func (r *fakeRepo) QueueSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.QueueReportSummaryRow, error) {
	return db.QueueReportSummaryRow{}, nil
}
func (r *fakeRepo) BallotSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.BallotReportSummaryRow, error) {
	return db.BallotReportSummaryRow{}, nil
}
func (r *fakeRepo) RacepackSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.RacepackReportSummaryRow, error) {
	return db.RacepackReportSummaryRow{}, nil
}
func (r *fakeRepo) RevenueSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.RevenueReportSummaryRow, error) {
	return db.RevenueReportSummaryRow{}, nil
}

// Rows — only revenue is exercised.
func (r *fakeRepo) ParticipantRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.ParticipantReportRowsRow, error) {
	return nil, nil
}
func (r *fakeRepo) SalesRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.SalesReportRowsRow, error) {
	return nil, nil
}
func (r *fakeRepo) PaymentRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.PaymentReportRowsRow, error) {
	return nil, nil
}
func (r *fakeRepo) CouponRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.CouponReportRowsRow, error) {
	return nil, nil
}
func (r *fakeRepo) QueueRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.QueueReportRowsRow, error) {
	return nil, nil
}
func (r *fakeRepo) BallotRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.BallotReportRowsRow, error) {
	return nil, nil
}
func (r *fakeRepo) RacepackRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.RacepackReportRowsRow, error) {
	return nil, nil
}
func (r *fakeRepo) RevenueRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.RevenueReportRowsRow, error) {
	return r.revenue, nil
}

func (r *fakeRepo) PlatformRevenueByOrg(ctx context.Context) ([]db.PlatformRevenueByOrgRow, error) {
	return nil, nil
}

// --- tests ---

func newSvc(repo reporting.Repository, store *fakeStore) *reporting.Service {
	return reporting.NewService(repo, store, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCreateExport_RejectsUnknownType(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo, newFakeStore())
	_, err := svc.CreateExport(context.Background(), uuid.New(), uuid.New(), "nonsense", nil)
	if !errors.Is(err, reporting.ErrUnknownReportType) {
		t.Fatalf("expected ErrUnknownReportType, got %v", err)
	}
	if repo.created != 0 {
		t.Errorf("expected no job created, got %d", repo.created)
	}
}

func TestCreateExport_EnqueuesPending(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo, newFakeStore())
	resp, err := svc.CreateExport(context.Background(), uuid.New(), uuid.New(), reporting.ReportRevenue, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Status != reporting.JobStatusPending {
		t.Errorf("expected PENDING, got %s", resp.Status)
	}
	if repo.created != 1 {
		t.Errorf("expected 1 job, got %d", repo.created)
	}
}

func TestProcessNextJob_GeneratesCSVAndMarksReady(t *testing.T) {
	repo := newFakeRepo()
	repo.revenue = []db.RevenueReportRowsRow{
		{Day: pgtype.Date{Time: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Valid: true}, PaidOrders: 3, GrossRevenue: 150000, TotalDiscount: 5000},
		{Day: pgtype.Date{Time: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Valid: true}, PaidOrders: 1, GrossRevenue: 50000, TotalDiscount: 0},
	}
	store := newFakeStore()
	svc := newSvc(repo, store)

	if _, err := svc.CreateExport(context.Background(), uuid.New(), uuid.New(), reporting.ReportRevenue, nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.ProcessNextJob(context.Background()); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(repo.readyIDs) != 1 {
		t.Fatalf("expected 1 ready job, got %d (failed: %v)", len(repo.readyIDs), repo.failed)
	}
	if len(store.puts) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(store.puts))
	}
	var csv []byte
	for _, b := range store.puts {
		csv = b
	}
	s := string(csv)
	if !strings.Contains(s, "day,paid_orders,gross_revenue,total_discount") {
		t.Errorf("missing header, got: %q", s)
	}
	if !strings.Contains(s, "2026-07-01,3,150000,5000") {
		t.Errorf("missing data row, got: %q", s)
	}
}

func TestProcessNextJob_EmptyQueueReturnsNoRows(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo, newFakeStore())
	err := svc.ProcessNextJob(context.Background())
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestExportJob_DrainsThenStops(t *testing.T) {
	repo := newFakeRepo()
	repo.revenue = []db.RevenueReportRowsRow{{Day: pgtype.Date{Valid: false}, PaidOrders: 1, GrossRevenue: 100, TotalDiscount: 0}}
	store := newFakeStore()
	svc := newSvc(repo, store)
	org := uuid.New()
	for i := 0; i < 3; i++ {
		if _, err := svc.CreateExport(context.Background(), org, uuid.New(), reporting.ReportRevenue, nil); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	if err := svc.ExportJob(10)(context.Background()); err != nil {
		t.Fatalf("export job: %v", err)
	}
	if len(repo.readyIDs) != 3 {
		t.Fatalf("expected 3 ready, got %d", len(repo.readyIDs))
	}
}
