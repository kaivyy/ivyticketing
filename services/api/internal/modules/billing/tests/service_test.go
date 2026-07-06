package billing_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/billing"
)

// --- fake repository ---

type fakeRepo struct {
	sub       db.GetOrgSubscriptionRow
	subErr    error
	eventCnt  int64
	pkgByID   map[uuid.UUID]db.SubscriptionPackage
	created   int
	updated   int
	upserts   int
	invoices  int
	invByID   map[uuid.UUID]db.PlatformInvoice
	feeSum    db.PlatformFeeSummaryRow
	revenue   []db.PlatformRevenueSummaryRow
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		subErr:  pgx.ErrNoRows,
		pkgByID: map[uuid.UUID]db.SubscriptionPackage{},
		invByID: map[uuid.UUID]db.PlatformInvoice{},
	}
}

func (r *fakeRepo) ListPackages(ctx context.Context) ([]db.SubscriptionPackage, error) {
	out := make([]db.SubscriptionPackage, 0, len(r.pkgByID))
	for _, p := range r.pkgByID {
		out = append(out, p)
	}
	return out, nil
}
func (r *fakeRepo) ListActivePackages(ctx context.Context) ([]db.SubscriptionPackage, error) {
	return r.ListPackages(ctx)
}
func (r *fakeRepo) GetPackage(ctx context.Context, id uuid.UUID) (db.SubscriptionPackage, error) {
	p, ok := r.pkgByID[id]
	if !ok {
		return db.SubscriptionPackage{}, pgx.ErrNoRows
	}
	return p, nil
}
func (r *fakeRepo) GetPackageBySlug(ctx context.Context, slug string) (db.SubscriptionPackage, error) {
	for _, p := range r.pkgByID {
		if p.Slug == slug {
			return p, nil
		}
	}
	return db.SubscriptionPackage{}, pgx.ErrNoRows
}
func (r *fakeRepo) CreatePackage(ctx context.Context, arg db.CreateSubscriptionPackageParams) (db.SubscriptionPackage, error) {
	r.created++
	p := db.SubscriptionPackage{
		ID: uuid.New(), Slug: arg.Slug, Name: arg.Name, Description: arg.Description,
		PriceMonthly: arg.PriceMonthly, MaxEvents: arg.MaxEvents, FeeBps: arg.FeeBps,
		Features: arg.Features, IsActive: true, SortOrder: arg.SortOrder,
	}
	r.pkgByID[p.ID] = p
	return p, nil
}
func (r *fakeRepo) UpdatePackage(ctx context.Context, arg db.UpdateSubscriptionPackageParams) (db.SubscriptionPackage, error) {
	if _, ok := r.pkgByID[arg.ID]; !ok {
		return db.SubscriptionPackage{}, pgx.ErrNoRows
	}
	r.updated++
	p := db.SubscriptionPackage{
		ID: arg.ID, Name: arg.Name, Description: arg.Description, PriceMonthly: arg.PriceMonthly,
		MaxEvents: arg.MaxEvents, FeeBps: arg.FeeBps, Features: arg.Features,
		IsActive: arg.IsActive, SortOrder: arg.SortOrder,
	}
	r.pkgByID[arg.ID] = p
	return p, nil
}
func (r *fakeRepo) GetOrgSubscription(ctx context.Context, orgID uuid.UUID) (db.GetOrgSubscriptionRow, error) {
	if r.subErr != nil {
		return db.GetOrgSubscriptionRow{}, r.subErr
	}
	return r.sub, nil
}
func (r *fakeRepo) UpsertOrgSubscription(ctx context.Context, orgID, packageID uuid.UUID, expiresAt pgtype.Timestamptz) (db.OrgSubscription, error) {
	r.upserts++
	return db.OrgSubscription{ID: uuid.New(), OrganizationID: orgID, PackageID: packageID, Status: "ACTIVE"}, nil
}
func (r *fakeRepo) CountEventsByOrg(ctx context.Context, orgID uuid.UUID) (int64, error) {
	return r.eventCnt, nil
}
func (r *fakeRepo) InsertPlatformFee(ctx context.Context, arg db.InsertPlatformFeeParams) (db.PlatformFeeLedger, error) {
	return db.PlatformFeeLedger{}, nil
}
func (r *fakeRepo) ListPlatformFeesByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.PlatformFeeLedger, error) {
	return nil, nil
}
func (r *fakeRepo) PlatformFeeSummary(ctx context.Context, orgID uuid.UUID) (db.PlatformFeeSummaryRow, error) {
	return r.feeSum, nil
}
func (r *fakeRepo) PlatformRevenueSummary(ctx context.Context) ([]db.PlatformRevenueSummaryRow, error) {
	return r.revenue, nil
}
func (r *fakeRepo) CreatePlatformInvoice(ctx context.Context, arg db.CreatePlatformInvoiceParams) (db.PlatformInvoice, error) {
	r.invoices++
	inv := db.PlatformInvoice{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, InvoiceNumber: arg.InvoiceNumber,
		PeriodStart: arg.PeriodStart, PeriodEnd: arg.PeriodEnd,
		SubscriptionAmount: arg.SubscriptionAmount, FeeAmount: arg.FeeAmount,
		TotalAmount: arg.TotalAmount, Status: "ISSUED",
	}
	r.invByID[inv.ID] = inv
	return inv, nil
}
func (r *fakeRepo) GetPlatformInvoice(ctx context.Context, id uuid.UUID) (db.PlatformInvoice, error) {
	inv, ok := r.invByID[id]
	if !ok {
		return db.PlatformInvoice{}, pgx.ErrNoRows
	}
	return inv, nil
}
func (r *fakeRepo) ListPlatformInvoicesByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.PlatformInvoice, error) {
	return nil, nil
}
func (r *fakeRepo) MarkPlatformInvoicePaid(ctx context.Context, id uuid.UUID) (db.PlatformInvoice, error) {
	inv, ok := r.invByID[id]
	if !ok {
		return db.PlatformInvoice{}, pgx.ErrNoRows
	}
	inv.Status = "PAID"
	return inv, nil
}

func newSvc(r *fakeRepo) *billing.Service {
	return billing.NewService(r, nil, slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{})))
}

func featuresJSON(t *testing.T, fs ...string) []byte {
	t.Helper()
	b, err := json.Marshal(fs)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// --- PackageGate.Can ---

func TestCan_NoSubscription_DefaultAllow(t *testing.T) {
	r := newFakeRepo() // subErr = pgx.ErrNoRows
	svc := newSvc(r)
	if !svc.Can(context.Background(), uuid.New(), billing.FeatureQueue) {
		t.Fatal("org with no subscription should default-allow all features")
	}
}

func TestCan_ActiveWithFeature_Allow(t *testing.T) {
	r := newFakeRepo()
	r.subErr = nil
	r.sub = db.GetOrgSubscriptionRow{
		OrgSubscription:     db.OrgSubscription{Status: billing.SubActive},
		SubscriptionPackage: db.SubscriptionPackage{Features: featuresJSON(t, billing.FeatureQueue, billing.FeatureBallot)},
	}
	svc := newSvc(r)
	if !svc.Can(context.Background(), uuid.New(), billing.FeatureQueue) {
		t.Fatal("feature present in package should be allowed")
	}
}

func TestCan_ActiveWithoutFeature_Deny(t *testing.T) {
	r := newFakeRepo()
	r.subErr = nil
	r.sub = db.GetOrgSubscriptionRow{
		OrgSubscription:     db.OrgSubscription{Status: billing.SubActive},
		SubscriptionPackage: db.SubscriptionPackage{Features: featuresJSON(t, billing.FeatureBasicRegistration)},
	}
	svc := newSvc(r)
	if svc.Can(context.Background(), uuid.New(), billing.FeatureWhitelabel) {
		t.Fatal("feature absent from active package should be denied")
	}
}

func TestCan_CancelledSubscription_DefaultAllow(t *testing.T) {
	r := newFakeRepo()
	r.subErr = nil
	r.sub = db.GetOrgSubscriptionRow{
		OrgSubscription:     db.OrgSubscription{Status: billing.SubCancelled},
		SubscriptionPackage: db.SubscriptionPackage{Features: featuresJSON(t, billing.FeatureBasicRegistration)},
	}
	svc := newSvc(r)
	if !svc.Can(context.Background(), uuid.New(), billing.FeatureWhitelabel) {
		t.Fatal("non-active subscription should fall back to default-allow")
	}
}

// --- CheckEventLimit ---

func TestCheckEventLimit_NoSubscription_Unlimited(t *testing.T) {
	r := newFakeRepo()
	r.eventCnt = 999
	svc := newSvc(r)
	if err := svc.CheckEventLimit(context.Background(), uuid.New()); err != nil {
		t.Fatalf("no subscription should be unlimited, got %v", err)
	}
}

func TestCheckEventLimit_UnlimitedPackage(t *testing.T) {
	r := newFakeRepo()
	r.subErr = nil
	r.eventCnt = 100
	r.sub = db.GetOrgSubscriptionRow{
		SubscriptionPackage: db.SubscriptionPackage{MaxEvents: pgtype.Int4{Valid: false}},
	}
	svc := newSvc(r)
	if err := svc.CheckEventLimit(context.Background(), uuid.New()); err != nil {
		t.Fatalf("null max_events means unlimited, got %v", err)
	}
}

func TestCheckEventLimit_UnderCap(t *testing.T) {
	r := newFakeRepo()
	r.subErr = nil
	r.eventCnt = 2
	r.sub = db.GetOrgSubscriptionRow{
		SubscriptionPackage: db.SubscriptionPackage{MaxEvents: pgtype.Int4{Int32: 3, Valid: true}},
	}
	svc := newSvc(r)
	if err := svc.CheckEventLimit(context.Background(), uuid.New()); err != nil {
		t.Fatalf("2 < 3 should be allowed, got %v", err)
	}
}

func TestCheckEventLimit_AtCap_Rejected(t *testing.T) {
	r := newFakeRepo()
	r.subErr = nil
	r.eventCnt = 3
	r.sub = db.GetOrgSubscriptionRow{
		SubscriptionPackage: db.SubscriptionPackage{MaxEvents: pgtype.Int4{Int32: 3, Valid: true}},
	}
	svc := newSvc(r)
	if err := svc.CheckEventLimit(context.Background(), uuid.New()); !errors.Is(err, billing.ErrEventLimitReached) {
		t.Fatalf("3 >= 3 should reject with ErrEventLimitReached, got %v", err)
	}
}

// --- CreatePackage validation ---

func TestCreatePackage_RejectsEmptySlug(t *testing.T) {
	svc := newSvc(newFakeRepo())
	_, err := svc.CreatePackage(context.Background(), uuid.New(), billing.UpsertPackageRequest{Name: "X"})
	if !errors.Is(err, billing.ErrInvalidPackage) {
		t.Fatalf("empty slug should be rejected, got %v", err)
	}
}

func TestCreatePackage_RejectsFeeOutOfRange(t *testing.T) {
	svc := newSvc(newFakeRepo())
	_, err := svc.CreatePackage(context.Background(), uuid.New(), billing.UpsertPackageRequest{
		Slug: "x", Name: "X", FeeBps: 20000,
	})
	if !errors.Is(err, billing.ErrInvalidPackage) {
		t.Fatalf("fee > 10000 should be rejected, got %v", err)
	}
}

func TestCreatePackage_RoundTripsFeatures(t *testing.T) {
	r := newFakeRepo()
	svc := newSvc(r)
	got, err := svc.CreatePackage(context.Background(), uuid.New(), billing.UpsertPackageRequest{
		Slug: "pro", Name: "Pro", FeeBps: 300, Features: []string{billing.FeatureQueue, billing.FeatureBallot},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Features) != 2 || got.Features[0] != billing.FeatureQueue {
		t.Fatalf("features not round-tripped: %v", got.Features)
	}
}

// --- GenerateInvoice validation ---

func TestGenerateInvoice_RejectsBadDate(t *testing.T) {
	svc := newSvc(newFakeRepo())
	_, err := svc.GenerateInvoice(context.Background(), uuid.New(), uuid.New(), billing.GenerateInvoiceRequest{
		PeriodStart: "not-a-date", PeriodEnd: "2026-01-31",
	})
	if !errors.Is(err, billing.ErrInvalidPackage) {
		t.Fatalf("bad date should be rejected, got %v", err)
	}
}

func TestGenerateInvoice_ComputesTotal(t *testing.T) {
	r := newFakeRepo()
	svc := newSvc(r)
	inv, err := svc.GenerateInvoice(context.Background(), uuid.New(), uuid.New(), billing.GenerateInvoiceRequest{
		PeriodStart: "2026-01-01", PeriodEnd: "2026-01-31", SubscriptionAmount: 49900000, FeeAmount: 150000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.TotalAmount != 49900000+150000 {
		t.Fatalf("total = subscription + fee, got %d", inv.TotalAmount)
	}
}

// --- AssignSubscription ---

func TestAssignSubscription_UnknownPackage(t *testing.T) {
	r := newFakeRepo()
	svc := newSvc(r)
	_, err := svc.AssignSubscription(context.Background(), uuid.New(), uuid.New(), billing.AssignSubscriptionRequest{
		PackageID: uuid.New().String(),
	})
	if !errors.Is(err, billing.ErrPackageNotFound) {
		t.Fatalf("assigning unknown package should fail with ErrPackageNotFound, got %v", err)
	}
}
