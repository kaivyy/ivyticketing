package results

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the persistence surface for race results and certificate
// templates. It wraps the sqlc-generated queries so the service layer works in
// domain terms.
type Repository interface {
	UpsertRaceResult(ctx context.Context, arg db.UpsertRaceResultParams) (db.RaceResult, error)
	GetRaceResultByID(ctx context.Context, id uuid.UUID) (db.RaceResult, error)
	GetRaceResultByBib(ctx context.Context, arg db.GetRaceResultByBibParams) (db.RaceResult, error)
	GetRaceResultByTicket(ctx context.Context, ticketID *uuid.UUID) (db.RaceResult, error)
	ListRaceResults(ctx context.Context, arg db.ListRaceResultsParams) ([]db.RaceResult, error)
	CountRaceResults(ctx context.Context, arg db.CountRaceResultsParams) (int64, error)
	DeleteRaceResultsByEvent(ctx context.Context, eventID uuid.UUID) error

	RankOverall(ctx context.Context, eventID uuid.UUID) error
	RankGender(ctx context.Context, eventID uuid.UUID) error
	RankCategory(ctx context.Context, eventID uuid.UUID) error
	RankAgeGroup(ctx context.Context, eventID uuid.UUID) error

	CreateCertificateTemplate(ctx context.Context, arg db.CreateCertificateTemplateParams) (db.CertificateTemplate, error)
	UpdateCertificateTemplate(ctx context.Context, arg db.UpdateCertificateTemplateParams) (db.CertificateTemplate, error)
	GetActiveCertificateTemplate(ctx context.Context, eventID uuid.UUID) (db.CertificateTemplate, error)
	GetCertificateTemplateByID(ctx context.Context, id uuid.UUID) (db.CertificateTemplate, error)
	ListCertificateTemplatesByEvent(ctx context.Context, eventID uuid.UUID) ([]db.CertificateTemplate, error)
	DeactivateCertificateTemplatesForEvent(ctx context.Context, eventID uuid.UUID) error
	DeleteCertificateTemplate(ctx context.Context, arg db.DeleteCertificateTemplateParams) error
}

type sqlcRepo struct {
	q *db.Queries
}

// NewRepository builds a Repository backed by the shared pgx pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{q: db.New(pool)}
}

func (r *sqlcRepo) UpsertRaceResult(ctx context.Context, arg db.UpsertRaceResultParams) (db.RaceResult, error) {
	return r.q.UpsertRaceResult(ctx, arg)
}

func (r *sqlcRepo) GetRaceResultByID(ctx context.Context, id uuid.UUID) (db.RaceResult, error) {
	return r.q.GetRaceResultByID(ctx, id)
}

func (r *sqlcRepo) GetRaceResultByBib(ctx context.Context, arg db.GetRaceResultByBibParams) (db.RaceResult, error) {
	return r.q.GetRaceResultByBib(ctx, arg)
}

func (r *sqlcRepo) GetRaceResultByTicket(ctx context.Context, ticketID *uuid.UUID) (db.RaceResult, error) {
	return r.q.GetRaceResultByTicket(ctx, ticketID)
}

func (r *sqlcRepo) ListRaceResults(ctx context.Context, arg db.ListRaceResultsParams) ([]db.RaceResult, error) {
	return r.q.ListRaceResults(ctx, arg)
}

func (r *sqlcRepo) CountRaceResults(ctx context.Context, arg db.CountRaceResultsParams) (int64, error) {
	return r.q.CountRaceResults(ctx, arg)
}

func (r *sqlcRepo) DeleteRaceResultsByEvent(ctx context.Context, eventID uuid.UUID) error {
	return r.q.DeleteRaceResultsByEvent(ctx, eventID)
}

func (r *sqlcRepo) RankOverall(ctx context.Context, eventID uuid.UUID) error {
	return r.q.RankOverall(ctx, eventID)
}

func (r *sqlcRepo) RankGender(ctx context.Context, eventID uuid.UUID) error {
	return r.q.RankGender(ctx, eventID)
}

func (r *sqlcRepo) RankCategory(ctx context.Context, eventID uuid.UUID) error {
	return r.q.RankCategory(ctx, eventID)
}

func (r *sqlcRepo) RankAgeGroup(ctx context.Context, eventID uuid.UUID) error {
	return r.q.RankAgeGroup(ctx, eventID)
}

func (r *sqlcRepo) CreateCertificateTemplate(ctx context.Context, arg db.CreateCertificateTemplateParams) (db.CertificateTemplate, error) {
	return r.q.CreateCertificateTemplate(ctx, arg)
}

func (r *sqlcRepo) UpdateCertificateTemplate(ctx context.Context, arg db.UpdateCertificateTemplateParams) (db.CertificateTemplate, error) {
	return r.q.UpdateCertificateTemplate(ctx, arg)
}

func (r *sqlcRepo) GetActiveCertificateTemplate(ctx context.Context, eventID uuid.UUID) (db.CertificateTemplate, error) {
	return r.q.GetActiveCertificateTemplate(ctx, eventID)
}

func (r *sqlcRepo) GetCertificateTemplateByID(ctx context.Context, id uuid.UUID) (db.CertificateTemplate, error) {
	return r.q.GetCertificateTemplateByID(ctx, id)
}

func (r *sqlcRepo) ListCertificateTemplatesByEvent(ctx context.Context, eventID uuid.UUID) ([]db.CertificateTemplate, error) {
	return r.q.ListCertificateTemplatesByEvent(ctx, eventID)
}

func (r *sqlcRepo) DeactivateCertificateTemplatesForEvent(ctx context.Context, eventID uuid.UUID) error {
	return r.q.DeactivateCertificateTemplatesForEvent(ctx, eventID)
}

func (r *sqlcRepo) DeleteCertificateTemplate(ctx context.Context, arg db.DeleteCertificateTemplateParams) error {
	return r.q.DeleteCertificateTemplate(ctx, arg)
}
