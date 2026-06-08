package abuse

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ListSettings(ctx context.Context) ([]db.PlatformSetting, error)
	UpsertSetting(ctx context.Context, arg db.UpsertPlatformSettingParams) (db.PlatformSetting, error)

	GetBlockedSubject(ctx context.Context, arg db.GetBlockedSubjectParams) (db.BlockedSubject, error)
	ListBlockedSubjects(ctx context.Context, arg db.ListBlockedSubjectsParams) ([]db.BlockedSubject, error)
	UpsertBlockedSubject(ctx context.Context, arg db.UpsertBlockedSubjectParams) (db.BlockedSubject, error)
	DeleteBlockedSubject(ctx context.Context, arg db.DeleteBlockedSubjectParams) error

	ListIPRules(ctx context.Context) ([]db.IpRule, error)
	CreateIPRule(ctx context.Context, arg db.CreateIPRuleParams) (db.IpRule, error)
	DeleteIPRule(ctx context.Context, id uuid.UUID) error

	InsertAbuseLog(ctx context.Context, arg db.InsertAbuseLogParams) error
	ListAbuseLog(ctx context.Context, arg db.ListAbuseLogParams) ([]db.AbuseLog, error)

	GetReputation(ctx context.Context, arg db.GetReputationParams) (db.IpReputation, error)
	BumpReputation(ctx context.Context, arg db.BumpReputationParams) (db.IpReputation, error)

	CountActiveQueueTokensByUser(ctx context.Context, participantID uuid.UUID) (int64, error)
}

type sqlcRepo struct {
	q *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

func (r *sqlcRepo) ListSettings(ctx context.Context) ([]db.PlatformSetting, error) {
	return r.q.ListPlatformSettings(ctx)
}

func (r *sqlcRepo) UpsertSetting(ctx context.Context, arg db.UpsertPlatformSettingParams) (db.PlatformSetting, error) {
	return r.q.UpsertPlatformSetting(ctx, arg)
}

func (r *sqlcRepo) GetBlockedSubject(ctx context.Context, arg db.GetBlockedSubjectParams) (db.BlockedSubject, error) {
	return r.q.GetBlockedSubject(ctx, arg)
}

func (r *sqlcRepo) ListBlockedSubjects(ctx context.Context, arg db.ListBlockedSubjectsParams) ([]db.BlockedSubject, error) {
	return r.q.ListBlockedSubjects(ctx, arg)
}

func (r *sqlcRepo) UpsertBlockedSubject(ctx context.Context, arg db.UpsertBlockedSubjectParams) (db.BlockedSubject, error) {
	return r.q.UpsertBlockedSubject(ctx, arg)
}

func (r *sqlcRepo) DeleteBlockedSubject(ctx context.Context, arg db.DeleteBlockedSubjectParams) error {
	return r.q.DeleteBlockedSubject(ctx, arg)
}

func (r *sqlcRepo) ListIPRules(ctx context.Context) ([]db.IpRule, error) {
	return r.q.ListIPRules(ctx)
}

func (r *sqlcRepo) CreateIPRule(ctx context.Context, arg db.CreateIPRuleParams) (db.IpRule, error) {
	return r.q.CreateIPRule(ctx, arg)
}

func (r *sqlcRepo) DeleteIPRule(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteIPRule(ctx, id)
}

func (r *sqlcRepo) InsertAbuseLog(ctx context.Context, arg db.InsertAbuseLogParams) error {
	return r.q.InsertAbuseLog(ctx, arg)
}

func (r *sqlcRepo) ListAbuseLog(ctx context.Context, arg db.ListAbuseLogParams) ([]db.AbuseLog, error) {
	return r.q.ListAbuseLog(ctx, arg)
}

func (r *sqlcRepo) GetReputation(ctx context.Context, arg db.GetReputationParams) (db.IpReputation, error) {
	return r.q.GetReputation(ctx, arg)
}

func (r *sqlcRepo) BumpReputation(ctx context.Context, arg db.BumpReputationParams) (db.IpReputation, error) {
	return r.q.BumpReputation(ctx, arg)
}

func (r *sqlcRepo) CountActiveQueueTokensByUser(ctx context.Context, participantID uuid.UUID) (int64, error) {
	return r.q.CountActiveQueueTokensByUser(ctx, participantID)
}
