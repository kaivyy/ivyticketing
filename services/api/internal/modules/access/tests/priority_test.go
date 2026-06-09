package access_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/access"
	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

// fakePriorityLifecycle satisfies access.LifecycleWindowChecker.
type fakePriorityLifecycle struct{ open bool }

func (f *fakePriorityLifecycle) IsWindowOpen(_ context.Context, _ uuid.UUID, _ registration.Mode) (bool, registration.WindowClosedReason, error) {
	return f.open, "window_expired", nil
}

// fakePriorityRepo extends fakeAccessRepoFull with controllable pool/grant results.
type fakePriorityRepo struct {
	fakeAccessRepoFull
	pool      *db.AccessPool
	grantToReturn db.AccessGrant
	noExisting    bool // if true GetActiveGrantForParticipant returns pgx.ErrNoRows
}

func (r *fakePriorityRepo) ListVisiblePoolsByCategory(_ context.Context, _ db.ListVisiblePoolsByCategoryParams) ([]db.AccessPool, error) {
	if r.pool == nil {
		return nil, nil
	}
	return []db.AccessPool{*r.pool}, nil
}

func (r *fakePriorityRepo) ReservePoolSlot(_ context.Context, _ uuid.UUID) (db.AccessPool, error) {
	if r.pool == nil {
		return db.AccessPool{}, pgx.ErrNoRows
	}
	return *r.pool, nil
}

func (r *fakePriorityRepo) GetActiveGrantForParticipant(_ context.Context, _ db.GetActiveGrantForParticipantParams) (db.AccessGrant, error) {
	if r.noExisting {
		return db.AccessGrant{}, pgx.ErrNoRows
	}
	return r.grantToReturn, nil
}

func (r *fakePriorityRepo) CreateAccessGrant(_ context.Context, _ db.CreateAccessGrantParams) (db.AccessGrant, error) {
	return db.AccessGrant{
		ID:        uuid.New(),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}, nil
}

// fakeAccessRepoFull satisfies the full access.Repository interface with no-ops.
type fakeAccessRepoFull struct{}

func (r *fakeAccessRepoFull) CreateAccessPool(_ context.Context, _ db.CreateAccessPoolParams) (db.AccessPool, error) {
	return db.AccessPool{}, nil
}
func (r *fakeAccessRepoFull) GetAccessPool(_ context.Context, _ uuid.UUID) (db.AccessPool, error) {
	return db.AccessPool{}, nil
}
func (r *fakeAccessRepoFull) ReservePoolSlot(_ context.Context, _ uuid.UUID) (db.AccessPool, error) {
	return db.AccessPool{}, nil
}
func (r *fakeAccessRepoFull) ConsumePoolSlot(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeAccessRepoFull) ReleasePoolSlot(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeAccessRepoFull) CreateAccessGrant(_ context.Context, _ db.CreateAccessGrantParams) (db.AccessGrant, error) {
	return db.AccessGrant{}, nil
}
func (r *fakeAccessRepoFull) GetAccessGrant(_ context.Context, _ uuid.UUID) (db.AccessGrant, error) {
	return db.AccessGrant{}, pgx.ErrNoRows
}
func (r *fakeAccessRepoFull) GetActiveGrantForParticipant(_ context.Context, _ db.GetActiveGrantForParticipantParams) (db.AccessGrant, error) {
	return db.AccessGrant{}, pgx.ErrNoRows
}
func (r *fakeAccessRepoFull) ExpireGrant(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeAccessRepoFull) ConsumeGrant(_ context.Context, _ db.ConsumeGrantParams) error {
	return nil
}
func (r *fakeAccessRepoFull) ListExpiredActiveGrants(_ context.Context, _ int32) ([]db.AccessGrant, error) {
	return nil, nil
}
func (r *fakeAccessRepoFull) CreateCorporateAccount(_ context.Context, _ db.CreateCorporateAccountParams) (db.CorporateAccount, error) {
	return db.CorporateAccount{}, nil
}
func (r *fakeAccessRepoFull) GetCorporateAccount(_ context.Context, _ uuid.UUID) (db.CorporateAccount, error) {
	return db.CorporateAccount{}, nil
}
func (r *fakeAccessRepoFull) ListCorporateAccounts(_ context.Context, _ db.ListCorporateAccountsParams) ([]db.CorporateAccount, error) {
	return nil, nil
}
func (r *fakeAccessRepoFull) ApproveCorporateAccount(_ context.Context, _ db.ApproveCorporateAccountParams) (db.CorporateAccount, error) {
	return db.CorporateAccount{}, nil
}
func (r *fakeAccessRepoFull) AddPoolMember(_ context.Context, _ db.AddPoolMemberParams) (db.AccessPoolMember, error) {
	return db.AccessPoolMember{}, nil
}
func (r *fakeAccessRepoFull) ListPoolMembers(_ context.Context, _ db.ListPoolMembersParams) ([]db.AccessPoolMember, error) {
	return nil, nil
}
func (r *fakeAccessRepoFull) GetPoolMemberByEmail(_ context.Context, _ db.GetPoolMemberByEmailParams) (db.AccessPoolMember, error) {
	return db.AccessPoolMember{}, nil
}
func (r *fakeAccessRepoFull) UpdatePoolMemberStatus(_ context.Context, _ db.UpdatePoolMemberStatusParams) (db.AccessPoolMember, error) {
	return db.AccessPoolMember{}, nil
}
func (r *fakeAccessRepoFull) UpdateAccessPoolColumns(_ context.Context, _ db.UpdateAccessPoolColumnsParams) (db.AccessPool, error) {
	return db.AccessPool{}, nil
}
func (r *fakeAccessRepoFull) ListVisiblePoolsByCategory(_ context.Context, _ db.ListVisiblePoolsByCategoryParams) ([]db.AccessPool, error) {
	return nil, nil
}
func (r *fakeAccessRepoFull) TransferPoolSlots(_ context.Context, _ db.TransferPoolSlotsParams) (db.AccessPool, error) {
	return db.AccessPool{}, nil
}
func (r *fakeAccessRepoFull) CreateAccessCode(_ context.Context, _ db.CreateAccessCodeParams) (db.AccessCode, error) {
	return db.AccessCode{}, nil
}
func (r *fakeAccessRepoFull) GetAccessCodeByHash(_ context.Context, _ db.GetAccessCodeByHashParams) (db.AccessCode, error) {
	return db.AccessCode{}, nil
}
func (r *fakeAccessRepoFull) ListAccessCodesByEvent(_ context.Context, _ db.ListAccessCodesByEventParams) ([]db.AccessCode, error) {
	return nil, nil
}
func (r *fakeAccessRepoFull) IncrementCodeUseCount(_ context.Context, _ uuid.UUID) (db.AccessCode, error) {
	return db.AccessCode{}, nil
}
func (r *fakeAccessRepoFull) RevokeAccessCode(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeAccessRepoFull) ListActiveGrantsForParticipant(_ context.Context, _ db.ListActiveGrantsForParticipantParams) ([]db.AccessGrant, error) {
	return nil, nil
}
func (r *fakeAccessRepoFull) CountPaidOrdersByUserInOrg(_ context.Context, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *fakeAccessRepoFull) GetUserMembershipID(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}
func (r *fakeAccessRepoFull) HasPaidOrderForEvent(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

// --- Tests ---

func TestPriorityChecker_WindowClosed_ReturnsError(t *testing.T) {
	lc := &fakePriorityLifecycle{open: false}
	checker := access.NewPriorityChecker(&fakePriorityRepo{noExisting: true}, lc, access.NewEligibilityChecker(&fakeEligRepo{}))
	err := checker.CheckPriorityAdmission(context.Background(), uuid.New(), uuid.New(), uuid.New(), "")
	if err == nil {
		t.Fatal("closed priority window should return error")
	}
}

func TestPriorityChecker_NoPool_ReturnsError(t *testing.T) {
	lc := &fakePriorityLifecycle{open: true}
	checker := access.NewPriorityChecker(&fakePriorityRepo{pool: nil, noExisting: true}, lc, access.NewEligibilityChecker(&fakeEligRepo{}))
	err := checker.CheckPriorityAdmission(context.Background(), uuid.New(), uuid.New(), uuid.New(), "")
	if err == nil {
		t.Fatal("no priority pool should return error")
	}
}

func TestPriorityChecker_EligibleWithOpenWindow_ReturnsNil(t *testing.T) {
	lc := &fakePriorityLifecycle{open: true}
	pool := &db.AccessPool{
		ID:         uuid.New(),
		TotalSlots: 10,
		PoolType:   access.PoolTypePriority,
	}
	checker := access.NewPriorityChecker(
		&fakePriorityRepo{pool: pool, noExisting: true},
		lc,
		access.NewEligibilityChecker(&fakeEligRepo{orderCount: 1}),
	)
	err := checker.CheckPriorityAdmission(context.Background(), uuid.New(), uuid.New(), uuid.New(), "")
	if err != nil {
		t.Fatalf("eligible user with open window should pass: %v", err)
	}
}

func TestPriorityChecker_AlreadyGranted_Idempotent(t *testing.T) {
	lc := &fakePriorityLifecycle{open: true}
	pool := &db.AccessPool{
		ID:         uuid.New(),
		TotalSlots: 10,
		PoolType:   access.PoolTypePriority,
	}
	existingGrant := db.AccessGrant{
		ID:        uuid.New(),
		Status:    access.GrantStatusActive,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}
	checker := access.NewPriorityChecker(
		&fakePriorityRepo{pool: pool, grantToReturn: existingGrant, noExisting: false},
		lc,
		access.NewEligibilityChecker(&fakeEligRepo{}),
	)
	err := checker.CheckPriorityAdmission(context.Background(), uuid.New(), uuid.New(), uuid.New(), "")
	if err != nil {
		t.Fatalf("already-granted user should return nil (idempotent): %v", err)
	}
}
