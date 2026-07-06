package whitelabel

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the data access surface for white-label branding and custom
// domains.
type Repository interface {
	GetBranding(ctx context.Context, orgID uuid.UUID) (db.OrgBranding, error)
	UpsertBranding(ctx context.Context, arg db.UpsertOrgBrandingParams) (db.OrgBranding, error)

	ListDomains(ctx context.Context, orgID uuid.UUID) ([]db.CustomDomain, error)
	GetDomain(ctx context.Context, id uuid.UUID) (db.CustomDomain, error)
	GetDomainByName(ctx context.Context, domain string) (db.CustomDomain, error)
	CreateDomain(ctx context.Context, arg db.CreateCustomDomainParams) (db.CustomDomain, error)
	MarkDomainVerified(ctx context.Context, id uuid.UUID) (db.CustomDomain, error)
	MarkDomainFailed(ctx context.Context, id uuid.UUID) (db.CustomDomain, error)
	DeleteDomain(ctx context.Context, arg db.DeleteCustomDomainParams) error
}

type sqlcRepo struct{ q *db.Queries }

// NewRepository constructs a Repository backed by the sqlc-generated Queries.
func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

func (r *sqlcRepo) GetBranding(ctx context.Context, orgID uuid.UUID) (db.OrgBranding, error) {
	return r.q.GetOrgBranding(ctx, orgID)
}
func (r *sqlcRepo) UpsertBranding(ctx context.Context, arg db.UpsertOrgBrandingParams) (db.OrgBranding, error) {
	return r.q.UpsertOrgBranding(ctx, arg)
}
func (r *sqlcRepo) ListDomains(ctx context.Context, orgID uuid.UUID) ([]db.CustomDomain, error) {
	return r.q.ListCustomDomainsByOrg(ctx, orgID)
}
func (r *sqlcRepo) GetDomain(ctx context.Context, id uuid.UUID) (db.CustomDomain, error) {
	return r.q.GetCustomDomain(ctx, id)
}
func (r *sqlcRepo) GetDomainByName(ctx context.Context, domain string) (db.CustomDomain, error) {
	return r.q.GetCustomDomainByName(ctx, domain)
}
func (r *sqlcRepo) CreateDomain(ctx context.Context, arg db.CreateCustomDomainParams) (db.CustomDomain, error) {
	return r.q.CreateCustomDomain(ctx, arg)
}
func (r *sqlcRepo) MarkDomainVerified(ctx context.Context, id uuid.UUID) (db.CustomDomain, error) {
	return r.q.MarkCustomDomainVerified(ctx, id)
}
func (r *sqlcRepo) MarkDomainFailed(ctx context.Context, id uuid.UUID) (db.CustomDomain, error) {
	return r.q.MarkCustomDomainFailed(ctx, id)
}
func (r *sqlcRepo) DeleteDomain(ctx context.Context, arg db.DeleteCustomDomainParams) error {
	return r.q.DeleteCustomDomain(ctx, arg)
}
