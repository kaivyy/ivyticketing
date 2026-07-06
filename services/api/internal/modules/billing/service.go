package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// Service coordinates platform billing: package catalog, per-org subscriptions,
// the platform fee ledger (hooked into the payments PAID transition), and
// invoices. It also exposes the PackageGate via Can().
type Service struct {
	repo  Repository
	audit *audit.Logger
	log   *slog.Logger
}

// NewService constructs a billing Service.
func NewService(repo Repository, auditLog *audit.Logger, log *slog.Logger) *Service {
	return &Service{repo: repo, audit: auditLog, log: log}
}

// --- packages ---

// ListPackages returns the package catalog. When activeOnly is true only active
// packages are returned (the organizer-facing upgrade view).
func (s *Service) ListPackages(ctx context.Context, activeOnly bool) ([]PackageResponse, error) {
	var (
		pkgs []db.SubscriptionPackage
		err  error
	)
	if activeOnly {
		pkgs, err = s.repo.ListActivePackages(ctx)
	} else {
		pkgs, err = s.repo.ListPackages(ctx)
	}
	if err != nil {
		return nil, err
	}
	out := make([]PackageResponse, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, toPackageResponse(p))
	}
	return out, nil
}

// GetPackage returns one package by id.
func (s *Service) GetPackage(ctx context.Context, id uuid.UUID) (PackageResponse, error) {
	pkg, err := s.repo.GetPackage(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PackageResponse{}, ErrPackageNotFound
		}
		return PackageResponse{}, err
	}
	return toPackageResponse(pkg), nil
}

// CreatePackage adds a new subscription package (super-admin).
func (s *Service) CreatePackage(ctx context.Context, actor uuid.UUID, req UpsertPackageRequest) (PackageResponse, error) {
	if req.Slug == "" || req.Name == "" {
		return PackageResponse{}, ErrInvalidPackage
	}
	if req.FeeBps < 0 || req.FeeBps > 10000 || req.PriceMonthly < 0 {
		return PackageResponse{}, ErrInvalidPackage
	}
	features, err := marshalFeatures(req.Features)
	if err != nil {
		return PackageResponse{}, ErrInvalidPackage
	}
	pkg, err := s.repo.CreatePackage(ctx, db.CreateSubscriptionPackageParams{
		Slug:         req.Slug,
		Name:         req.Name,
		Description:  req.Description,
		PriceMonthly: req.PriceMonthly,
		MaxEvents:    pgInt4Ptr(req.MaxEvents),
		FeeBps:       req.FeeBps,
		Features:     features,
		SortOrder:    req.SortOrder,
	})
	if err != nil {
		return PackageResponse{}, err
	}
	s.record(ctx, actor, "billing.package_created", "subscription_package", pkg.ID.String(), map[string]any{"slug": pkg.Slug})
	return toPackageResponse(pkg), nil
}

// UpdatePackage edits an existing package (super-admin). Slug is immutable.
func (s *Service) UpdatePackage(ctx context.Context, actor, id uuid.UUID, req UpsertPackageRequest) (PackageResponse, error) {
	if req.Name == "" {
		return PackageResponse{}, ErrInvalidPackage
	}
	if req.FeeBps < 0 || req.FeeBps > 10000 || req.PriceMonthly < 0 {
		return PackageResponse{}, ErrInvalidPackage
	}
	features, err := marshalFeatures(req.Features)
	if err != nil {
		return PackageResponse{}, ErrInvalidPackage
	}
	pkg, err := s.repo.UpdatePackage(ctx, db.UpdateSubscriptionPackageParams{
		ID:           id,
		Name:         req.Name,
		Description:  req.Description,
		PriceMonthly: req.PriceMonthly,
		MaxEvents:    pgInt4Ptr(req.MaxEvents),
		FeeBps:       req.FeeBps,
		Features:     features,
		IsActive:     req.IsActive,
		SortOrder:    req.SortOrder,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PackageResponse{}, ErrPackageNotFound
		}
		return PackageResponse{}, err
	}
	s.record(ctx, actor, "billing.package_updated", "subscription_package", pkg.ID.String(), map[string]any{"slug": pkg.Slug})
	return toPackageResponse(pkg), nil
}

// --- subscriptions ---

// GetSubscription returns an org's current subscription with its package.
func (s *Service) GetSubscription(ctx context.Context, orgID uuid.UUID) (SubscriptionResponse, error) {
	row, err := s.repo.GetOrgSubscription(ctx, orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubscriptionResponse{}, ErrSubscriptionNotFound
		}
		return SubscriptionResponse{}, err
	}
	return toSubscriptionResponse(row), nil
}

// AssignSubscription assigns or upgrades an org to a package (super-admin).
func (s *Service) AssignSubscription(ctx context.Context, actor, orgID uuid.UUID, req AssignSubscriptionRequest) (SubscriptionResponse, error) {
	packageID, err := uuid.Parse(req.PackageID)
	if err != nil {
		return SubscriptionResponse{}, ErrInvalidPackage
	}
	if _, err := s.repo.GetPackage(ctx, packageID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubscriptionResponse{}, ErrPackageNotFound
		}
		return SubscriptionResponse{}, err
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, perr := time.Parse(time.RFC3339, *req.ExpiresAt)
		if perr != nil {
			return SubscriptionResponse{}, ErrInvalidPackage
		}
		expiresAt = &t
	}
	if _, err := s.repo.UpsertOrgSubscription(ctx, orgID, packageID, pgTimestamptzPtr(expiresAt)); err != nil {
		return SubscriptionResponse{}, err
	}
	s.record(ctx, actor, "billing.subscription_assigned", "org_subscription", orgID.String(), map[string]any{"package_id": packageID.String()})
	return s.GetSubscription(ctx, orgID)
}

// --- package gate (feature entitlement) ---

// Can reports whether an org may use a gated feature. Soft gate: an org with no
// subscription is allowed everything (default-allow), so billing is additive and
// never breaks existing organizers. When a subscription exists, only the
// package's feature list is allowed.
func (s *Service) Can(ctx context.Context, orgID uuid.UUID, feature string) bool {
	row, err := s.repo.GetOrgSubscription(ctx, orgID)
	if err != nil {
		return true // no subscription (or lookup failure) → default-allow
	}
	if row.OrgSubscription.Status != SubActive {
		return true
	}
	for _, f := range parseFeatures(row.SubscriptionPackage.Features) {
		if f == feature {
			return true
		}
	}
	return false
}

// CheckEventLimit returns ErrEventLimitReached if the org has hit its package's
// max_events cap. Orgs without a subscription (or with an unlimited package) are
// never limited.
func (s *Service) CheckEventLimit(ctx context.Context, orgID uuid.UUID) error {
	row, err := s.repo.GetOrgSubscription(ctx, orgID)
	if err != nil {
		return nil // no subscription → unlimited
	}
	max := int4Ptr(row.SubscriptionPackage.MaxEvents)
	if max == nil {
		return nil // unlimited
	}
	count, err := s.repo.CountEventsByOrg(ctx, orgID)
	if err != nil {
		return err
	}
	if count >= int64(*max) {
		return ErrEventLimitReached
	}
	return nil
}

// --- platform fee ledger ---

// RecordOrderFee records the platform fee for a just-PAID order. It runs inside
// the payments PAID transaction (q is the tx querier) so the fee row commits
// atomically with the order transition. Idempotent via ON CONFLICT (order_id).
// Orgs without an active subscription incur no platform fee.
func (s *Service) RecordOrderFee(ctx context.Context, q *db.Queries, order db.Order) error {
	row, err := q.GetOrgSubscription(ctx, order.OrganizationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // org not on a billing package → no platform fee
		}
		return err
	}
	feeBps := row.SubscriptionPackage.FeeBps
	if feeBps <= 0 {
		return nil
	}
	feeAmount := order.Total * int64(feeBps) / 10000
	_, err = q.InsertPlatformFee(ctx, db.InsertPlatformFeeParams{
		OrganizationID: order.OrganizationID,
		OrderID:        order.ID,
		OrderTotal:     order.Total,
		FeeBps:         feeBps,
		FeeAmount:      feeAmount,
	})
	return err
}

// FeeSummary aggregates an org's platform-fee ledger.
func (s *Service) FeeSummary(ctx context.Context, orgID uuid.UUID) (FeeSummaryResponse, error) {
	row, err := s.repo.PlatformFeeSummary(ctx, orgID)
	if err != nil {
		return FeeSummaryResponse{}, err
	}
	return FeeSummaryResponse{Entries: row.Entries, GrossOrders: row.GrossOrders, TotalFees: row.TotalFees}, nil
}

// PlatformRevenue returns the cross-org fee aggregate (super-admin).
func (s *Service) PlatformRevenue(ctx context.Context) ([]RevenueRow, error) {
	rows, err := s.repo.PlatformRevenueSummary(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RevenueRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, RevenueRow{
			OrganizationID:   r.OrganizationID.String(),
			OrganizationName: r.OrganizationName,
			TotalFees:        r.TotalFees,
			GrossOrders:      r.GrossOrders,
			FeeEntries:       r.FeeEntries,
		})
	}
	return out, nil
}

// --- invoices ---

// GenerateInvoice issues a platform invoice for a billing period (super-admin).
func (s *Service) GenerateInvoice(ctx context.Context, actor, orgID uuid.UUID, req GenerateInvoiceRequest) (InvoiceResponse, error) {
	start, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		return InvoiceResponse{}, ErrInvalidPackage
	}
	end, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		return InvoiceResponse{}, ErrInvalidPackage
	}
	if req.SubscriptionAmount < 0 || req.FeeAmount < 0 {
		return InvoiceResponse{}, ErrInvalidPackage
	}
	total := req.SubscriptionAmount + req.FeeAmount
	inv, err := s.repo.CreatePlatformInvoice(ctx, db.CreatePlatformInvoiceParams{
		OrganizationID:     orgID,
		InvoiceNumber:      invoiceNumber(orgID, start),
		PeriodStart:        pgDate(start),
		PeriodEnd:          pgDate(end),
		SubscriptionAmount: req.SubscriptionAmount,
		FeeAmount:          req.FeeAmount,
		TotalAmount:        total,
	})
	if err != nil {
		return InvoiceResponse{}, err
	}
	s.record(ctx, actor, "billing.invoice_generated", "platform_invoice", inv.ID.String(), map[string]any{"invoice_number": inv.InvoiceNumber})
	return toInvoiceResponse(inv), nil
}

// GetInvoice returns one invoice by id.
func (s *Service) GetInvoice(ctx context.Context, id uuid.UUID) (InvoiceResponse, error) {
	inv, err := s.repo.GetPlatformInvoice(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InvoiceResponse{}, ErrInvoiceNotFound
		}
		return InvoiceResponse{}, err
	}
	return toInvoiceResponse(inv), nil
}

// ListInvoices returns an org's invoices, newest first.
func (s *Service) ListInvoices(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]InvoiceResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	invs, err := s.repo.ListPlatformInvoicesByOrg(ctx, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]InvoiceResponse, 0, len(invs))
	for _, inv := range invs {
		out = append(out, toInvoiceResponse(inv))
	}
	return out, nil
}

// MarkInvoicePaid transitions an ISSUED invoice to PAID (super-admin).
func (s *Service) MarkInvoicePaid(ctx context.Context, actor, id uuid.UUID) (InvoiceResponse, error) {
	inv, err := s.repo.MarkPlatformInvoicePaid(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InvoiceResponse{}, ErrInvoiceNotFound
		}
		return InvoiceResponse{}, err
	}
	s.record(ctx, actor, "billing.invoice_paid", "platform_invoice", inv.ID.String(), nil)
	return toInvoiceResponse(inv), nil
}

// --- helpers ---

func (s *Service) record(ctx context.Context, actor uuid.UUID, action, targetType, targetID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	a := actor
	s.audit.Record(ctx, audit.Entry{
		ActorUserID: &a,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Metadata:    meta,
	})
}

func invoiceNumber(orgID uuid.UUID, period time.Time) string {
	return fmt.Sprintf("INV-%s-%s", period.Format("200601"), orgID.String()[:8])
}

func parseFeatures(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func marshalFeatures(features []string) ([]byte, error) {
	if features == nil {
		features = []string{}
	}
	return json.Marshal(features)
}

func toPackageResponse(p db.SubscriptionPackage) PackageResponse {
	return PackageResponse{
		ID:           p.ID.String(),
		Slug:         p.Slug,
		Name:         p.Name,
		Description:  p.Description,
		PriceMonthly: p.PriceMonthly,
		MaxEvents:    int4Ptr(p.MaxEvents),
		FeeBps:       p.FeeBps,
		Features:     parseFeatures(p.Features),
		IsActive:     p.IsActive,
		SortOrder:    p.SortOrder,
	}
}

func toSubscriptionResponse(row db.GetOrgSubscriptionRow) SubscriptionResponse {
	return SubscriptionResponse{
		ID:             row.OrgSubscription.ID.String(),
		OrganizationID: row.OrgSubscription.OrganizationID.String(),
		Status:         row.OrgSubscription.Status,
		StartedAt:      tsStr(row.OrgSubscription.StartedAt),
		ExpiresAt:      tsPtr(row.OrgSubscription.ExpiresAt),
		Package:        toPackageResponse(row.SubscriptionPackage),
	}
}

func toInvoiceResponse(inv db.PlatformInvoice) InvoiceResponse {
	return InvoiceResponse{
		ID:                 inv.ID.String(),
		OrganizationID:     inv.OrganizationID.String(),
		InvoiceNumber:      inv.InvoiceNumber,
		PeriodStart:        dateStr(inv.PeriodStart),
		PeriodEnd:          dateStr(inv.PeriodEnd),
		SubscriptionAmount: inv.SubscriptionAmount,
		FeeAmount:          inv.FeeAmount,
		TotalAmount:        inv.TotalAmount,
		Status:             inv.Status,
		IssuedAt:           tsPtr(inv.IssuedAt),
		PaidAt:             tsPtr(inv.PaidAt),
		CreatedAt:          tsStr(inv.CreatedAt),
	}
}
