package access_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

type fakeAccessRepo struct {
	pool        db.AccessPool
	grant       db.AccessGrant
	reserveErr  error
	createErr   error
}

func (r *fakeAccessRepo) CreateAccessPool(_ context.Context, _ db.CreateAccessPoolParams) (db.AccessPool, error) {
	return r.pool, nil
}
func (r *fakeAccessRepo) GetAccessPool(_ context.Context, _ uuid.UUID) (db.AccessPool, error) {
	return r.pool, nil
}
func (r *fakeAccessRepo) ReservePoolSlot(_ context.Context, _ uuid.UUID) (db.AccessPool, error) {
	if r.reserveErr != nil {
		return db.AccessPool{}, r.reserveErr
	}
	return r.pool, nil
}
func (r *fakeAccessRepo) ConsumePoolSlot(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeAccessRepo) ReleasePoolSlot(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeAccessRepo) CreateAccessGrant(_ context.Context, _ db.CreateAccessGrantParams) (db.AccessGrant, error) {
	if r.createErr != nil {
		return db.AccessGrant{}, r.createErr
	}
	return r.grant, nil
}
func (r *fakeAccessRepo) GetAccessGrant(_ context.Context, _ uuid.UUID) (db.AccessGrant, error) {
	if r.grant.ID == uuid.Nil {
		return db.AccessGrant{}, pgx.ErrNoRows
	}
	return r.grant, nil
}
func (r *fakeAccessRepo) GetActiveGrantForParticipant(_ context.Context, _ db.GetActiveGrantForParticipantParams) (db.AccessGrant, error) {
	return r.grant, nil
}
func (r *fakeAccessRepo) ExpireGrant(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeAccessRepo) ConsumeGrant(_ context.Context, _ db.ConsumeGrantParams) error { return nil }
func (r *fakeAccessRepo) ListExpiredActiveGrants(_ context.Context, _ int32) ([]db.AccessGrant, error) {
	return nil, nil
}

// New Repository methods — no-op stubs to satisfy interface
func (r *fakeAccessRepo) CreateCorporateAccount(_ context.Context, _ db.CreateCorporateAccountParams) (db.CorporateAccount, error) {
	return db.CorporateAccount{}, nil
}
func (r *fakeAccessRepo) GetCorporateAccount(_ context.Context, _ uuid.UUID) (db.CorporateAccount, error) {
	return db.CorporateAccount{}, nil
}
func (r *fakeAccessRepo) ListCorporateAccounts(_ context.Context, _ db.ListCorporateAccountsParams) ([]db.CorporateAccount, error) {
	return nil, nil
}
func (r *fakeAccessRepo) ApproveCorporateAccount(_ context.Context, _ db.ApproveCorporateAccountParams) (db.CorporateAccount, error) {
	return db.CorporateAccount{}, nil
}
func (r *fakeAccessRepo) AddPoolMember(_ context.Context, _ db.AddPoolMemberParams) (db.AccessPoolMember, error) {
	return db.AccessPoolMember{}, nil
}
func (r *fakeAccessRepo) ListPoolMembers(_ context.Context, _ db.ListPoolMembersParams) ([]db.AccessPoolMember, error) {
	return nil, nil
}
func (r *fakeAccessRepo) GetPoolMemberByEmail(_ context.Context, _ db.GetPoolMemberByEmailParams) (db.AccessPoolMember, error) {
	return db.AccessPoolMember{}, nil
}
func (r *fakeAccessRepo) UpdatePoolMemberStatus(_ context.Context, _ db.UpdatePoolMemberStatusParams) (db.AccessPoolMember, error) {
	return db.AccessPoolMember{}, nil
}
func (r *fakeAccessRepo) UpdateAccessPoolColumns(_ context.Context, _ db.UpdateAccessPoolColumnsParams) (db.AccessPool, error) {
	return db.AccessPool{}, nil
}
func (r *fakeAccessRepo) ListVisiblePoolsByCategory(_ context.Context, _ db.ListVisiblePoolsByCategoryParams) ([]db.AccessPool, error) {
	return nil, nil
}
func (r *fakeAccessRepo) TransferPoolSlots(_ context.Context, _ db.TransferPoolSlotsParams) (db.AccessPool, error) {
	return db.AccessPool{}, nil
}

// Access code stubs
func (r *fakeAccessRepo) CreateAccessCode(_ context.Context, _ db.CreateAccessCodeParams) (db.AccessCode, error) {
	return db.AccessCode{}, nil
}
func (r *fakeAccessRepo) GetAccessCodeByHash(_ context.Context, _ db.GetAccessCodeByHashParams) (db.AccessCode, error) {
	return db.AccessCode{}, nil
}
func (r *fakeAccessRepo) ListAccessCodesByEvent(_ context.Context, _ db.ListAccessCodesByEventParams) ([]db.AccessCode, error) {
	return nil, nil
}
func (r *fakeAccessRepo) IncrementCodeUseCount(_ context.Context, _ uuid.UUID) (db.AccessCode, error) {
	return db.AccessCode{}, nil
}
func (r *fakeAccessRepo) RevokeAccessCode(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeAccessRepo) ListActiveGrantsForParticipant(_ context.Context, _ db.ListActiveGrantsForParticipantParams) ([]db.AccessGrant, error) {
	return nil, nil
}

// Eligibility stubs
func (r *fakeAccessRepo) CountPaidOrdersByUserInOrg(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *fakeAccessRepo) GetUserMembershipID(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}
func (r *fakeAccessRepo) HasPaidOrderForEvent(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

func TestReserveSlot_PoolFull_ReturnsErrPoolExhausted(t *testing.T) {
	repo := &fakeAccessRepo{reserveErr: pgx.ErrNoRows}
	pm := access.NewPoolManager(repo)
	err := pm.ReserveSlot(context.Background(), uuid.New())
	if !errors.Is(err, access.ErrPoolExhausted) {
		t.Fatalf("want ErrPoolExhausted, got %v", err)
	}
}

func TestReserveSlot_AvailableSlot_ReturnsNil(t *testing.T) {
	repo := &fakeAccessRepo{pool: db.AccessPool{ID: uuid.New(), TotalSlots: 10}}
	pm := access.NewPoolManager(repo)
	if err := pm.ReserveSlot(context.Background(), uuid.New()); err != nil {
		t.Fatalf("available slot should succeed: %v", err)
	}
}

func TestCheckGrant_InvalidToken_ReturnsNotFound(t *testing.T) {
	repo := &fakeAccessRepo{}
	pm := access.NewPoolManager(repo)
	err := pm.CheckGrant(context.Background(), uuid.New(), uuid.New(), "not-a-uuid")
	if !errors.Is(err, access.ErrGrantNotFound) {
		t.Fatalf("want ErrGrantNotFound, got %v", err)
	}
}

func TestCheckGrant_ExpiredGrant_ReturnsExpired(t *testing.T) {
	grantID := uuid.New()
	repo := &fakeAccessRepo{grant: db.AccessGrant{
		ID:        grantID,
		Status:    access.GrantStatusActive,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	}}
	pm := access.NewPoolManager(repo)
	err := pm.CheckGrant(context.Background(), uuid.New(), uuid.New(), grantID.String())
	if !errors.Is(err, access.ErrGrantExpired) {
		t.Fatalf("want ErrGrantExpired, got %v", err)
	}
}

func TestCheckGrant_ActiveGrant_ReturnsNil(t *testing.T) {
	grantID := uuid.New()
	repo := &fakeAccessRepo{grant: db.AccessGrant{
		ID:        grantID,
		Status:    access.GrantStatusActive,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}}
	pm := access.NewPoolManager(repo)
	err := pm.CheckGrant(context.Background(), uuid.New(), uuid.New(), grantID.String())
	if err != nil {
		t.Fatalf("active grant should pass: %v", err)
	}
}

func TestCreatePool_ReturnsPoolID(t *testing.T) {
	id := uuid.New()
	repo := &fakeAccessRepo{pool: db.AccessPool{ID: id}}
	pm := access.NewPoolManager(repo)
	got, err := pm.CreatePool(context.Background(), uuid.New(), uuid.New(), uuid.New(), access.PoolTypeReserved, "Test", 100, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("want %v, got %v", id, got)
	}
}
