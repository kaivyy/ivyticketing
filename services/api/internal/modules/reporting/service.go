package reporting

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	"github.com/varin/ivyticketing/services/api/internal/platform/storage"
)

// Service coordinates report summaries + async CSV exports.
type Service struct {
	repo  Repository
	store storage.Storage
	audit *audit.Logger
	log   *slog.Logger
}

// NewService constructs a reporting Service.
func NewService(repo Repository, store storage.Storage, auditLog *audit.Logger, log *slog.Logger) *Service {
	return &Service{repo: repo, store: store, audit: auditLog, log: log}
}

// CreateExport validates the request and enqueues a PENDING export job. The
// worker picks it up and generates the CSV — the request never blocks on the
// (potentially large) query, per the masterplan acceptance criteria.
func (s *Service) CreateExport(ctx context.Context, orgID, userID uuid.UUID, reportType string, eventID *uuid.UUID) (ExportJobResponse, error) {
	if !IsValidReportType(reportType) {
		return ExportJobResponse{}, ErrUnknownReportType
	}
	params, _ := json.Marshal(map[string]any{})
	job, err := s.repo.CreateExportJob(ctx, db.CreateExportJobParams{
		OrganizationID: orgID,
		EventID:        eventID,
		RequestedBy:    userID,
		ReportType:     reportType,
		Format:         FormatCSV,
		Params:         params,
	})
	if err != nil {
		return ExportJobResponse{}, err
	}
	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &userID,
			Action:         "report.export_requested",
			TargetType:     "export_job",
			TargetID:       job.ID.String(),
			Metadata:       map[string]any{"report_type": reportType},
		})
	}
	return toJobResponse(job), nil
}

// GetJob returns a single export job scoped to the org.
func (s *Service) GetJob(ctx context.Context, id, orgID uuid.UUID) (ExportJobResponse, error) {
	job, err := s.repo.GetExportJob(ctx, id, orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExportJobResponse{}, ErrJobNotFound
		}
		return ExportJobResponse{}, err
	}
	return toJobResponse(job), nil
}

// ListJobs returns recent export jobs for the org.
func (s *Service) ListJobs(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]ExportJobResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	jobs, err := s.repo.ListExportJobsByOrg(ctx, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]ExportJobResponse, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, toJobResponse(j))
	}
	return out, nil
}

// GetSummary returns an on-screen aggregate for a report type as a generic map.
func (s *Service) GetSummary(ctx context.Context, orgID uuid.UUID, reportType string, eventID *uuid.UUID) (any, error) {
	switch reportType {
	case ReportParticipant:
		return s.repo.ParticipantSummary(ctx, orgID, eventID)
	case ReportSales:
		return s.repo.SalesSummary(ctx, orgID, eventID)
	case ReportPayment:
		return s.repo.PaymentSummary(ctx, orgID, eventID)
	case ReportCoupon:
		return s.repo.CouponSummary(ctx, orgID, eventID)
	case ReportQueue:
		return s.repo.QueueSummary(ctx, orgID, eventID)
	case ReportBallot:
		return s.repo.BallotSummary(ctx, orgID, eventID)
	case ReportRacepack:
		return s.repo.RacepackSummary(ctx, orgID, eventID)
	case ReportRevenue:
		return s.repo.RevenueSummary(ctx, orgID, eventID)
	default:
		return nil, ErrUnknownReportType
	}
}

// PlatformRevenue returns cross-org paid revenue (super-admin only).
func (s *Service) PlatformRevenue(ctx context.Context) ([]db.PlatformRevenueByOrgRow, error) {
	return s.repo.PlatformRevenueByOrg(ctx)
}

// ExportJob returns a worker Job that drains up to batch PENDING export jobs per
// tick. It stops early when the queue is empty (pgx.ErrNoRows) so an idle worker
// does no work. Per-job failures are recorded on the job row, not returned, so
// one bad job never stalls the batch.
func (s *Service) ExportJob(batch int) func(ctx context.Context) error {
	if batch <= 0 {
		batch = 10
	}
	return func(ctx context.Context) error {
		for i := 0; i < batch; i++ {
			if err := s.ProcessNextJob(ctx); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil // queue drained
				}
				return err
			}
		}
		return nil
	}
}

// ProcessNextJob claims one PENDING export job, generates its CSV, uploads it to
// storage, and marks the job READY (or FAILED). Returns pgx.ErrNoRows when there
// is nothing to do — the worker treats that as a no-op tick.
func (s *Service) ProcessNextJob(ctx context.Context) error {
	job, err := s.repo.ClaimPendingExportJob(ctx)
	if err != nil {
		return err // pgx.ErrNoRows when queue empty
	}

	rowCount, csvBytes, genErr := s.generateCSV(ctx, job)
	if genErr != nil {
		_, _ = s.repo.MarkExportJobFailed(ctx, job.ID, genErr.Error())
		s.log.Error("export job failed", "job", job.ID, "type", job.ReportType, "error", genErr)
		return nil
	}

	key := fmt.Sprintf("exports/%s/%s.csv", job.OrganizationID, job.ID)
	if err := s.store.Put(ctx, key, bytes.NewReader(csvBytes), "text/csv"); err != nil {
		_, _ = s.repo.MarkExportJobFailed(ctx, job.ID, "storage upload failed")
		s.log.Error("export upload failed", "job", job.ID, "error", err)
		return nil
	}
	url := s.store.PublicURL(key)
	if _, err := s.repo.MarkExportJobReady(ctx, job.ID, int32(rowCount), key, url); err != nil {
		s.log.Error("export mark-ready failed", "job", job.ID, "error", err)
		return nil
	}
	s.log.Info("export job ready", "job", job.ID, "type", job.ReportType, "rows", rowCount)
	return nil
}

// generateCSV dispatches on report type, returning row count + CSV bytes.
func (s *Service) generateCSV(ctx context.Context, job db.ExportJob) (int, []byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	orgID := job.OrganizationID
	eventID := job.EventID

	var count int
	var err error
	switch job.ReportType {
	case ReportParticipant:
		count, err = s.writeParticipant(ctx, w, orgID, eventID)
	case ReportSales:
		count, err = s.writeSales(ctx, w, orgID, eventID)
	case ReportPayment:
		count, err = s.writePayment(ctx, w, orgID, eventID)
	case ReportCoupon:
		count, err = s.writeCoupon(ctx, w, orgID, eventID)
	case ReportQueue:
		count, err = s.writeQueue(ctx, w, orgID, eventID)
	case ReportBallot:
		count, err = s.writeBallot(ctx, w, orgID, eventID)
	case ReportRacepack:
		count, err = s.writeRacepack(ctx, w, orgID, eventID)
	case ReportRevenue:
		count, err = s.writeRevenue(ctx, w, orgID, eventID)
	default:
		return 0, nil, ErrUnknownReportType
	}
	if err != nil {
		return 0, nil, err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return 0, nil, err
	}
	return count, buf.Bytes(), nil
}

// --- per-report CSV writers ---

func (s *Service) writeParticipant(ctx context.Context, w *csv.Writer, orgID uuid.UUID, eventID *uuid.UUID) (int, error) {
	rows, err := s.repo.ParticipantRows(ctx, orgID, eventID)
	if err != nil {
		return 0, err
	}
	_ = w.Write([]string{"ticket_number", "holder_name", "holder_email", "event", "category", "status", "bib", "issued_at", "used_at"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.TicketNumber, r.HolderName, r.HolderEmail, r.EventTitle, r.CategoryName,
			r.Status, textVal(r.BibNumber), tsVal(r.IssuedAt), tsVal(r.UsedAt),
		})
	}
	return len(rows), nil
}

func (s *Service) writeSales(ctx context.Context, w *csv.Writer, orgID uuid.UUID, eventID *uuid.UUID) (int, error) {
	rows, err := s.repo.SalesRows(ctx, orgID, eventID)
	if err != nil {
		return 0, err
	}
	_ = w.Write([]string{"order_number", "status", "subtotal", "fee", "discount", "total", "participant_name", "participant_email", "created_at"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.OrderNumber, r.Status, money(r.Subtotal), money(r.Fee), money(r.Discount), money(r.Total),
			r.ParticipantName, r.ParticipantEmail, tsVal(r.CreatedAt),
		})
	}
	return len(rows), nil
}

func (s *Service) writePayment(ctx context.Context, w *csv.Writer, orgID uuid.UUID, eventID *uuid.UUID) (int, error) {
	rows, err := s.repo.PaymentRows(ctx, orgID, eventID)
	if err != nil {
		return 0, err
	}
	_ = w.Write([]string{"merchant_reference", "gateway", "method", "channel", "status", "amount", "currency", "gateway_reference", "paid_at", "created_at"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.MerchantReference, r.Gateway, r.Method, textVal(r.Channel), r.Status,
			money(r.Amount), r.Currency, textVal(r.GatewayReference), tsVal(r.PaidAt), tsVal(r.CreatedAt),
		})
	}
	return len(rows), nil
}

func (s *Service) writeCoupon(ctx context.Context, w *csv.Writer, orgID uuid.UUID, eventID *uuid.UUID) (int, error) {
	rows, err := s.repo.CouponRows(ctx, orgID, eventID)
	if err != nil {
		return 0, err
	}
	_ = w.Write([]string{"code_type", "single_use", "max_uses", "use_count", "valid_from", "valid_until", "created_at"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.CodeType, strconv.FormatBool(r.IsSingleUse), strconv.Itoa(int(r.MaxUses)), strconv.Itoa(int(r.UseCount)),
			tsVal(r.ValidFrom), tsVal(r.ValidUntil), tsVal(r.CreatedAt),
		})
	}
	return len(rows), nil
}

func (s *Service) writeQueue(ctx context.Context, w *csv.Writer, orgID uuid.UUID, eventID *uuid.UUID) (int, error) {
	rows, err := s.repo.QueueRows(ctx, orgID, eventID)
	if err != nil {
		return 0, err
	}
	_ = w.Write([]string{"pool", "status", "score", "joined_at", "allowed_at", "completed_at", "expired_at", "participant_name", "participant_email"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.Pool, r.Status, strconv.FormatInt(r.Score, 10), tsVal(r.JoinedAt), tsVal(r.AllowedAt),
			tsVal(r.CompletedAt), tsVal(r.ExpiredAt), r.ParticipantName, r.ParticipantEmail,
		})
	}
	return len(rows), nil
}

func (s *Service) writeBallot(ctx context.Context, w *csv.Writer, orgID uuid.UUID, eventID *uuid.UUID) (int, error) {
	rows, err := s.repo.BallotRows(ctx, orgID, eventID)
	if err != nil {
		return 0, err
	}
	_ = w.Write([]string{"status", "applied_at", "payment_deadline", "converted_at", "promoted_round", "participant_name", "participant_email"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.Status, tsVal(r.AppliedAt), tsVal(r.PaymentDeadline), tsVal(r.ConvertedAt),
			strconv.Itoa(int(r.PromotedRound)), r.ParticipantName, r.ParticipantEmail,
		})
	}
	return len(rows), nil
}

func (s *Service) writeRacepack(ctx context.Context, w *csv.Writer, orgID uuid.UUID, eventID *uuid.UUID) (int, error) {
	rows, err := s.repo.RacepackRows(ctx, orgID, eventID)
	if err != nil {
		return 0, err
	}
	_ = w.Write([]string{"bib", "pickup_method", "pickup_timestamp", "notes", "participant_name", "participant_email", "staff_name"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.BibNumber, r.PickupMethod, tsVal(r.PickupTimestamp), textVal(r.Notes),
			r.ParticipantName, r.ParticipantEmail, r.StaffName,
		})
	}
	return len(rows), nil
}

func (s *Service) writeRevenue(ctx context.Context, w *csv.Writer, orgID uuid.UUID, eventID *uuid.UUID) (int, error) {
	rows, err := s.repo.RevenueRows(ctx, orgID, eventID)
	if err != nil {
		return 0, err
	}
	_ = w.Write([]string{"day", "paid_orders", "gross_revenue", "total_discount"})
	for _, r := range rows {
		day := ""
		if r.Day.Valid {
			day = r.Day.Time.Format("2006-01-02")
		}
		_ = w.Write([]string{
			day, strconv.FormatInt(r.PaidOrders, 10), money(r.GrossRevenue), money(r.TotalDiscount),
		})
	}
	return len(rows), nil
}

// --- value formatters ---

func money(v int64) string { return strconv.FormatInt(v, 10) }

func textVal(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func tsVal(t pgtype.Timestamptz) string {
	if t.Valid {
		return t.Time.Format(time.RFC3339)
	}
	return ""
}

func toJobResponse(j db.ExportJob) ExportJobResponse {
	resp := ExportJobResponse{
		ID:          j.ID.String(),
		ReportType:  j.ReportType,
		Format:      j.Format,
		Status:      j.Status,
		RequestedBy: j.RequestedBy.String(),
	}
	if j.CreatedAt.Valid {
		resp.CreatedAt = j.CreatedAt.Time.Format(time.RFC3339)
	}
	if j.RowCount.Valid {
		resp.RowCount = &j.RowCount.Int32
	}
	if j.FileUrl.Valid {
		resp.FileURL = &j.FileUrl.String
	}
	if j.Error.Valid {
		resp.Error = &j.Error.String
	}
	if j.EventID != nil {
		id := j.EventID.String()
		resp.EventID = &id
	}
	if j.CompletedAt.Valid {
		c := j.CompletedAt.Time.Format(time.RFC3339)
		resp.CompletedAt = &c
	}
	return resp
}
