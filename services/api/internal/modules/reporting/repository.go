package reporting

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the data access surface for reporting + exports. Report queries
// take an optional eventID (nil = all events in the org).
type Repository interface {
	// Export job lifecycle.
	CreateExportJob(ctx context.Context, arg db.CreateExportJobParams) (db.ExportJob, error)
	GetExportJob(ctx context.Context, id, orgID uuid.UUID) (db.ExportJob, error)
	ListExportJobsByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.ExportJob, error)
	ClaimPendingExportJob(ctx context.Context) (db.ExportJob, error)
	MarkExportJobReady(ctx context.Context, id uuid.UUID, rowCount int32, fileKey, fileURL string) (db.ExportJob, error)
	MarkExportJobFailed(ctx context.Context, id uuid.UUID, errMsg string) (db.ExportJob, error)

	// Report summaries.
	ParticipantSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.ParticipantReportSummaryRow, error)
	SalesSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.SalesReportSummaryRow, error)
	PaymentSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.PaymentReportSummaryRow, error)
	CouponSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.CouponReportSummaryRow, error)
	QueueSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.QueueReportSummaryRow, error)
	BallotSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.BallotReportSummaryRow, error)
	RacepackSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.RacepackReportSummaryRow, error)
	RevenueSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.RevenueReportSummaryRow, error)

	// Report rows (for CSV export).
	ParticipantRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.ParticipantReportRowsRow, error)
	SalesRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.SalesReportRowsRow, error)
	PaymentRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.PaymentReportRowsRow, error)
	CouponRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.CouponReportRowsRow, error)
	QueueRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.QueueReportRowsRow, error)
	BallotRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.BallotReportRowsRow, error)
	RacepackRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.RacepackReportRowsRow, error)
	RevenueRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.RevenueReportRowsRow, error)

	// Super-admin cross-org.
	PlatformRevenueByOrg(ctx context.Context) ([]db.PlatformRevenueByOrgRow, error)
}

type sqlcRepo struct{ q *db.Queries }

// NewRepository constructs a Repository backed by the sqlc-generated Queries.
func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

// --- export job lifecycle ---

func (r *sqlcRepo) CreateExportJob(ctx context.Context, arg db.CreateExportJobParams) (db.ExportJob, error) {
	return r.q.CreateExportJob(ctx, arg)
}

func (r *sqlcRepo) GetExportJob(ctx context.Context, id, orgID uuid.UUID) (db.ExportJob, error) {
	return r.q.GetExportJob(ctx, db.GetExportJobParams{ID: id, OrganizationID: orgID})
}

func (r *sqlcRepo) ListExportJobsByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.ExportJob, error) {
	return r.q.ListExportJobsByOrg(ctx, db.ListExportJobsByOrgParams{OrganizationID: orgID, Limit: limit, Offset: offset})
}

func (r *sqlcRepo) ClaimPendingExportJob(ctx context.Context) (db.ExportJob, error) {
	return r.q.ClaimPendingExportJob(ctx)
}

func (r *sqlcRepo) MarkExportJobReady(ctx context.Context, id uuid.UUID, rowCount int32, fileKey, fileURL string) (db.ExportJob, error) {
	return r.q.MarkExportJobReady(ctx, db.MarkExportJobReadyParams{
		ID:       id,
		RowCount: pgInt4(rowCount),
		FileKey:  pgText(fileKey),
		FileUrl:  pgText(fileURL),
	})
}

func (r *sqlcRepo) MarkExportJobFailed(ctx context.Context, id uuid.UUID, errMsg string) (db.ExportJob, error) {
	return r.q.MarkExportJobFailed(ctx, db.MarkExportJobFailedParams{ID: id, Error: pgText(errMsg)})
}

// --- summaries ---

func (r *sqlcRepo) ParticipantSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.ParticipantReportSummaryRow, error) {
	return r.q.ParticipantReportSummary(ctx, db.ParticipantReportSummaryParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) SalesSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.SalesReportSummaryRow, error) {
	return r.q.SalesReportSummary(ctx, db.SalesReportSummaryParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) PaymentSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.PaymentReportSummaryRow, error) {
	return r.q.PaymentReportSummary(ctx, db.PaymentReportSummaryParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) CouponSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.CouponReportSummaryRow, error) {
	return r.q.CouponReportSummary(ctx, db.CouponReportSummaryParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) QueueSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.QueueReportSummaryRow, error) {
	return r.q.QueueReportSummary(ctx, db.QueueReportSummaryParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) BallotSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.BallotReportSummaryRow, error) {
	return r.q.BallotReportSummary(ctx, db.BallotReportSummaryParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) RacepackSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.RacepackReportSummaryRow, error) {
	return r.q.RacepackReportSummary(ctx, db.RacepackReportSummaryParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) RevenueSummary(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) (db.RevenueReportSummaryRow, error) {
	return r.q.RevenueReportSummary(ctx, db.RevenueReportSummaryParams{OrganizationID: orgID, EventID: eventID})
}

// --- rows ---

func (r *sqlcRepo) ParticipantRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.ParticipantReportRowsRow, error) {
	return r.q.ParticipantReportRows(ctx, db.ParticipantReportRowsParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) SalesRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.SalesReportRowsRow, error) {
	return r.q.SalesReportRows(ctx, db.SalesReportRowsParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) PaymentRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.PaymentReportRowsRow, error) {
	return r.q.PaymentReportRows(ctx, db.PaymentReportRowsParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) CouponRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.CouponReportRowsRow, error) {
	return r.q.CouponReportRows(ctx, db.CouponReportRowsParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) QueueRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.QueueReportRowsRow, error) {
	return r.q.QueueReportRows(ctx, db.QueueReportRowsParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) BallotRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.BallotReportRowsRow, error) {
	return r.q.BallotReportRows(ctx, db.BallotReportRowsParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) RacepackRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.RacepackReportRowsRow, error) {
	return r.q.RacepackReportRows(ctx, db.RacepackReportRowsParams{OrganizationID: orgID, EventID: eventID})
}
func (r *sqlcRepo) RevenueRows(ctx context.Context, orgID uuid.UUID, eventID *uuid.UUID) ([]db.RevenueReportRowsRow, error) {
	return r.q.RevenueReportRows(ctx, db.RevenueReportRowsParams{OrganizationID: orgID, EventID: eventID})
}

func (r *sqlcRepo) PlatformRevenueByOrg(ctx context.Context) ([]db.PlatformRevenueByOrgRow, error) {
	return r.q.PlatformRevenueByOrg(ctx)
}
