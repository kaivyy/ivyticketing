package access

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateAccessPool(ctx context.Context, arg db.CreateAccessPoolParams) (db.AccessPool, error)
	GetAccessPool(ctx context.Context, id uuid.UUID) (db.AccessPool, error)
	ReservePoolSlot(ctx context.Context, id uuid.UUID) (db.AccessPool, error)
	ConsumePoolSlot(ctx context.Context, id uuid.UUID) error
	ReleasePoolSlot(ctx context.Context, id uuid.UUID) error
	CreateAccessGrant(ctx context.Context, arg db.CreateAccessGrantParams) (db.AccessGrant, error)
	GetAccessGrant(ctx context.Context, id uuid.UUID) (db.AccessGrant, error)
	GetActiveGrantForParticipant(ctx context.Context, arg db.GetActiveGrantForParticipantParams) (db.AccessGrant, error)
	ExpireGrant(ctx context.Context, id uuid.UUID) error
	ConsumeGrant(ctx context.Context, arg db.ConsumeGrantParams) error
	ListExpiredActiveGrants(ctx context.Context, limit int32) ([]db.AccessGrant, error)

	// Corporate accounts
	CreateCorporateAccount(ctx context.Context, arg db.CreateCorporateAccountParams) (db.CorporateAccount, error)
	GetCorporateAccount(ctx context.Context, id uuid.UUID) (db.CorporateAccount, error)
	ListCorporateAccounts(ctx context.Context, arg db.ListCorporateAccountsParams) ([]db.CorporateAccount, error)
	ApproveCorporateAccount(ctx context.Context, arg db.ApproveCorporateAccountParams) (db.CorporateAccount, error)

	// Pool members
	AddPoolMember(ctx context.Context, arg db.AddPoolMemberParams) (db.AccessPoolMember, error)
	ListPoolMembers(ctx context.Context, arg db.ListPoolMembersParams) ([]db.AccessPoolMember, error)
	GetPoolMemberByEmail(ctx context.Context, arg db.GetPoolMemberByEmailParams) (db.AccessPoolMember, error)
	UpdatePoolMemberStatus(ctx context.Context, arg db.UpdatePoolMemberStatusParams) (db.AccessPoolMember, error)
	UpdateAccessPoolColumns(ctx context.Context, arg db.UpdateAccessPoolColumnsParams) (db.AccessPool, error)
	ListVisiblePoolsByCategory(ctx context.Context, arg db.ListVisiblePoolsByCategoryParams) ([]db.AccessPool, error)
	TransferPoolSlots(ctx context.Context, arg db.TransferPoolSlotsParams) (db.AccessPool, error)

	// Access codes
	CreateAccessCode(ctx context.Context, arg db.CreateAccessCodeParams) (db.AccessCode, error)
	GetAccessCodeByHash(ctx context.Context, arg db.GetAccessCodeByHashParams) (db.AccessCode, error)
	ListAccessCodesByEvent(ctx context.Context, arg db.ListAccessCodesByEventParams) ([]db.AccessCode, error)
	IncrementCodeUseCount(ctx context.Context, id uuid.UUID) (db.AccessCode, error)
	RevokeAccessCode(ctx context.Context, id uuid.UUID) error
	ListActiveGrantsForParticipant(ctx context.Context, arg db.ListActiveGrantsForParticipantParams) ([]db.AccessGrant, error)
}

type sqlcRepo struct{ q *db.Queries }

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

func (r *sqlcRepo) CreateAccessPool(ctx context.Context, arg db.CreateAccessPoolParams) (db.AccessPool, error) {
	return r.q.CreateAccessPool(ctx, arg)
}
func (r *sqlcRepo) GetAccessPool(ctx context.Context, id uuid.UUID) (db.AccessPool, error) {
	return r.q.GetAccessPool(ctx, id)
}
func (r *sqlcRepo) ReservePoolSlot(ctx context.Context, id uuid.UUID) (db.AccessPool, error) {
	return r.q.ReservePoolSlot(ctx, id)
}
func (r *sqlcRepo) ConsumePoolSlot(ctx context.Context, id uuid.UUID) error {
	return r.q.ConsumePoolSlot(ctx, id)
}
func (r *sqlcRepo) ReleasePoolSlot(ctx context.Context, id uuid.UUID) error {
	return r.q.ReleasePoolSlot(ctx, id)
}
func (r *sqlcRepo) CreateAccessGrant(ctx context.Context, arg db.CreateAccessGrantParams) (db.AccessGrant, error) {
	return r.q.CreateAccessGrant(ctx, arg)
}
func (r *sqlcRepo) GetAccessGrant(ctx context.Context, id uuid.UUID) (db.AccessGrant, error) {
	return r.q.GetAccessGrant(ctx, id)
}
func (r *sqlcRepo) GetActiveGrantForParticipant(ctx context.Context, arg db.GetActiveGrantForParticipantParams) (db.AccessGrant, error) {
	return r.q.GetActiveGrantForParticipant(ctx, arg)
}
func (r *sqlcRepo) ExpireGrant(ctx context.Context, id uuid.UUID) error {
	return r.q.ExpireGrant(ctx, id)
}
func (r *sqlcRepo) ConsumeGrant(ctx context.Context, arg db.ConsumeGrantParams) error {
	return r.q.ConsumeGrant(ctx, arg)
}
func (r *sqlcRepo) ListExpiredActiveGrants(ctx context.Context, limit int32) ([]db.AccessGrant, error) {
	return r.q.ListExpiredActiveGrants(ctx, limit)
}

// Corporate accounts
func (r *sqlcRepo) CreateCorporateAccount(ctx context.Context, arg db.CreateCorporateAccountParams) (db.CorporateAccount, error) {
	return r.q.CreateCorporateAccount(ctx, arg)
}
func (r *sqlcRepo) GetCorporateAccount(ctx context.Context, id uuid.UUID) (db.CorporateAccount, error) {
	return r.q.GetCorporateAccount(ctx, id)
}
func (r *sqlcRepo) ListCorporateAccounts(ctx context.Context, arg db.ListCorporateAccountsParams) ([]db.CorporateAccount, error) {
	return r.q.ListCorporateAccounts(ctx, arg)
}
func (r *sqlcRepo) ApproveCorporateAccount(ctx context.Context, arg db.ApproveCorporateAccountParams) (db.CorporateAccount, error) {
	return r.q.ApproveCorporateAccount(ctx, arg)
}

// Pool members
func (r *sqlcRepo) AddPoolMember(ctx context.Context, arg db.AddPoolMemberParams) (db.AccessPoolMember, error) {
	return r.q.AddPoolMember(ctx, arg)
}
func (r *sqlcRepo) ListPoolMembers(ctx context.Context, arg db.ListPoolMembersParams) ([]db.AccessPoolMember, error) {
	return r.q.ListPoolMembers(ctx, arg)
}
func (r *sqlcRepo) GetPoolMemberByEmail(ctx context.Context, arg db.GetPoolMemberByEmailParams) (db.AccessPoolMember, error) {
	return r.q.GetPoolMemberByEmail(ctx, arg)
}
func (r *sqlcRepo) UpdatePoolMemberStatus(ctx context.Context, arg db.UpdatePoolMemberStatusParams) (db.AccessPoolMember, error) {
	return r.q.UpdatePoolMemberStatus(ctx, arg)
}
func (r *sqlcRepo) UpdateAccessPoolColumns(ctx context.Context, arg db.UpdateAccessPoolColumnsParams) (db.AccessPool, error) {
	return r.q.UpdateAccessPoolColumns(ctx, arg)
}
func (r *sqlcRepo) ListVisiblePoolsByCategory(ctx context.Context, arg db.ListVisiblePoolsByCategoryParams) ([]db.AccessPool, error) {
	return r.q.ListVisiblePoolsByCategory(ctx, arg)
}
func (r *sqlcRepo) TransferPoolSlots(ctx context.Context, arg db.TransferPoolSlotsParams) (db.AccessPool, error) {
	return r.q.TransferPoolSlots(ctx, arg)
}

// EligibilityRepo adapters
func (r *sqlcRepo) CountPaidOrdersByUserInOrg(ctx context.Context, userID, orgID uuid.UUID) (int64, error) {
	return r.q.CountPaidOrdersByUserInOrg(ctx, db.CountPaidOrdersByUserInOrgParams{
		ParticipantID:  userID,
		OrganizationID: orgID,
	})
}
func (r *sqlcRepo) GetUserMembershipID(ctx context.Context, userID uuid.UUID) (string, error) {
	return r.q.GetUserMembershipID(ctx, userID)
}
func (r *sqlcRepo) HasPaidOrderForEvent(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	return r.q.HasPaidOrderForEvent(ctx, db.HasPaidOrderForEventParams{
		ParticipantID: userID,
		EventID:       eventID,
	})
}

// Access code methods
func (r *sqlcRepo) CreateAccessCode(ctx context.Context, arg db.CreateAccessCodeParams) (db.AccessCode, error) {
	return r.q.CreateAccessCode(ctx, arg)
}
func (r *sqlcRepo) GetAccessCodeByHash(ctx context.Context, arg db.GetAccessCodeByHashParams) (db.AccessCode, error) {
	return r.q.GetAccessCodeByHash(ctx, arg)
}
func (r *sqlcRepo) ListAccessCodesByEvent(ctx context.Context, arg db.ListAccessCodesByEventParams) ([]db.AccessCode, error) {
	return r.q.ListAccessCodesByEvent(ctx, arg)
}
func (r *sqlcRepo) IncrementCodeUseCount(ctx context.Context, id uuid.UUID) (db.AccessCode, error) {
	return r.q.IncrementCodeUseCount(ctx, id)
}
func (r *sqlcRepo) RevokeAccessCode(ctx context.Context, id uuid.UUID) error {
	return r.q.RevokeAccessCode(ctx, id)
}
func (r *sqlcRepo) ListActiveGrantsForParticipant(ctx context.Context, arg db.ListActiveGrantsForParticipantParams) ([]db.AccessGrant, error) {
	return r.q.ListActiveGrantsForParticipant(ctx, arg)
}
