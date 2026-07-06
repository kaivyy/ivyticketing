package billing

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the data access surface for platform billing: subscription
// packages, per-org subscriptions, the platform fee ledger, and invoices.
type Repository interface {
	// Packages.
	ListPackages(ctx context.Context) ([]db.SubscriptionPackage, error)
	ListActivePackages(ctx context.Context) ([]db.SubscriptionPackage, error)
	GetPackage(ctx context.Context, id uuid.UUID) (db.SubscriptionPackage, error)
	GetPackageBySlug(ctx context.Context, slug string) (db.SubscriptionPackage, error)
	CreatePackage(ctx context.Context, arg db.CreateSubscriptionPackageParams) (db.SubscriptionPackage, error)
	UpdatePackage(ctx context.Context, arg db.UpdateSubscriptionPackageParams) (db.SubscriptionPackage, error)

	// Subscriptions.
	GetOrgSubscription(ctx context.Context, orgID uuid.UUID) (db.GetOrgSubscriptionRow, error)
	UpsertOrgSubscription(ctx context.Context, orgID, packageID uuid.UUID, expiresAt pgtype.Timestamptz) (db.OrgSubscription, error)
	CountEventsByOrg(ctx context.Context, orgID uuid.UUID) (int64, error)

	// Platform fee ledger.
	InsertPlatformFee(ctx context.Context, arg db.InsertPlatformFeeParams) (db.PlatformFeeLedger, error)
	ListPlatformFeesByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.PlatformFeeLedger, error)
	PlatformFeeSummary(ctx context.Context, orgID uuid.UUID) (db.PlatformFeeSummaryRow, error)
	PlatformRevenueSummary(ctx context.Context) ([]db.PlatformRevenueSummaryRow, error)

	// Invoices.
	CreatePlatformInvoice(ctx context.Context, arg db.CreatePlatformInvoiceParams) (db.PlatformInvoice, error)
	GetPlatformInvoice(ctx context.Context, id uuid.UUID) (db.PlatformInvoice, error)
	ListPlatformInvoicesByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.PlatformInvoice, error)
	MarkPlatformInvoicePaid(ctx context.Context, id uuid.UUID) (db.PlatformInvoice, error)
}

type sqlcRepo struct{ q *db.Queries }

// NewRepository constructs a Repository backed by the sqlc-generated Queries.
func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

// --- packages ---

func (r *sqlcRepo) ListPackages(ctx context.Context) ([]db.SubscriptionPackage, error) {
	return r.q.ListSubscriptionPackages(ctx)
}
func (r *sqlcRepo) ListActivePackages(ctx context.Context) ([]db.SubscriptionPackage, error) {
	return r.q.ListActiveSubscriptionPackages(ctx)
}
func (r *sqlcRepo) GetPackage(ctx context.Context, id uuid.UUID) (db.SubscriptionPackage, error) {
	return r.q.GetSubscriptionPackage(ctx, id)
}
func (r *sqlcRepo) GetPackageBySlug(ctx context.Context, slug string) (db.SubscriptionPackage, error) {
	return r.q.GetSubscriptionPackageBySlug(ctx, slug)
}
func (r *sqlcRepo) CreatePackage(ctx context.Context, arg db.CreateSubscriptionPackageParams) (db.SubscriptionPackage, error) {
	return r.q.CreateSubscriptionPackage(ctx, arg)
}
func (r *sqlcRepo) UpdatePackage(ctx context.Context, arg db.UpdateSubscriptionPackageParams) (db.SubscriptionPackage, error) {
	return r.q.UpdateSubscriptionPackage(ctx, arg)
}

// --- subscriptions ---

func (r *sqlcRepo) GetOrgSubscription(ctx context.Context, orgID uuid.UUID) (db.GetOrgSubscriptionRow, error) {
	return r.q.GetOrgSubscription(ctx, orgID)
}
func (r *sqlcRepo) UpsertOrgSubscription(ctx context.Context, orgID, packageID uuid.UUID, expiresAt pgtype.Timestamptz) (db.OrgSubscription, error) {
	return r.q.UpsertOrgSubscription(ctx, db.UpsertOrgSubscriptionParams{
		OrganizationID: orgID,
		PackageID:      packageID,
		ExpiresAt:      expiresAt,
	})
}
func (r *sqlcRepo) CountEventsByOrg(ctx context.Context, orgID uuid.UUID) (int64, error) {
	return r.q.CountEventsByOrg(ctx, orgID)
}

// --- platform fee ledger ---

func (r *sqlcRepo) InsertPlatformFee(ctx context.Context, arg db.InsertPlatformFeeParams) (db.PlatformFeeLedger, error) {
	return r.q.InsertPlatformFee(ctx, arg)
}
func (r *sqlcRepo) ListPlatformFeesByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.PlatformFeeLedger, error) {
	return r.q.ListPlatformFeesByOrg(ctx, db.ListPlatformFeesByOrgParams{OrganizationID: orgID, Limit: limit, Offset: offset})
}
func (r *sqlcRepo) PlatformFeeSummary(ctx context.Context, orgID uuid.UUID) (db.PlatformFeeSummaryRow, error) {
	return r.q.PlatformFeeSummary(ctx, orgID)
}
func (r *sqlcRepo) PlatformRevenueSummary(ctx context.Context) ([]db.PlatformRevenueSummaryRow, error) {
	return r.q.PlatformRevenueSummary(ctx)
}

// --- invoices ---

func (r *sqlcRepo) CreatePlatformInvoice(ctx context.Context, arg db.CreatePlatformInvoiceParams) (db.PlatformInvoice, error) {
	return r.q.CreatePlatformInvoice(ctx, arg)
}
func (r *sqlcRepo) GetPlatformInvoice(ctx context.Context, id uuid.UUID) (db.PlatformInvoice, error) {
	return r.q.GetPlatformInvoice(ctx, id)
}
func (r *sqlcRepo) ListPlatformInvoicesByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.PlatformInvoice, error) {
	return r.q.ListPlatformInvoicesByOrg(ctx, db.ListPlatformInvoicesByOrgParams{OrganizationID: orgID, Limit: limit, Offset: offset})
}
func (r *sqlcRepo) MarkPlatformInvoicePaid(ctx context.Context, id uuid.UUID) (db.PlatformInvoice, error) {
	return r.q.MarkPlatformInvoicePaid(ctx, id)
}
