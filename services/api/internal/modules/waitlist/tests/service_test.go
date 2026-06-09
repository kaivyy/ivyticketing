package waitlist_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/waitlist"
)

var errExhausted = waitlist.ErrWaitlistClosed // reuse as sentinel for pool exhausted

type fakeWaitlistRepo struct {
	wl      db.Waitlist
	entries []db.WaitlistEntry
	joined  []db.WaitlistEntry
}

func (r *fakeWaitlistRepo) CreateWaitlist(_ context.Context, _ db.CreateWaitlistParams) (db.Waitlist, error) {
	return db.Waitlist{}, nil
}
func (r *fakeWaitlistRepo) GetWaitlist(_ context.Context, _ uuid.UUID) (db.Waitlist, error) {
	return r.wl, nil
}
func (r *fakeWaitlistRepo) GetWaitlistByCategory(_ context.Context, _ db.GetWaitlistByCategoryParams) (db.Waitlist, error) {
	return r.wl, nil
}
func (r *fakeWaitlistRepo) SetWaitlistPool(_ context.Context, _ db.SetWaitlistPoolParams) error {
	return nil
}
func (r *fakeWaitlistRepo) SetWaitlistSeed(_ context.Context, _ db.SetWaitlistSeedParams) error {
	return nil
}
func (r *fakeWaitlistRepo) UpdateWaitlistStatus(_ context.Context, _ db.UpdateWaitlistStatusParams) error {
	return nil
}
func (r *fakeWaitlistRepo) JoinWaitlist(_ context.Context, arg db.JoinWaitlistParams) (db.WaitlistEntry, error) {
	e := db.WaitlistEntry{ID: uuid.New(), Rank: arg.Rank}
	r.joined = append(r.joined, e)
	return e, nil
}
func (r *fakeWaitlistRepo) GetWaitlistEntry(_ context.Context, _ db.GetWaitlistEntryParams) (db.WaitlistEntry, error) {
	return db.WaitlistEntry{}, pgx.ErrNoRows
}
func (r *fakeWaitlistRepo) ListWaitingEntries(_ context.Context, _ db.ListWaitingEntriesParams) ([]db.WaitlistEntry, error) {
	return r.entries, nil
}
func (r *fakeWaitlistRepo) UpdateWaitlistEntryStatus(_ context.Context, _ db.UpdateWaitlistEntryStatusParams) (db.WaitlistEntry, error) {
	return db.WaitlistEntry{}, nil
}
func (r *fakeWaitlistRepo) CountWaitlistPosition(_ context.Context, _ db.CountWaitlistPositionParams) (int64, error) {
	return 0, nil
}

type fakePoolReserver struct {
	n        int
	callCount int
}

func (f *fakePoolReserver) ReserveSlot(_ context.Context, _ uuid.UUID) error {
	if f.n <= 0 {
		return waitlist.ErrWaitlistClosed // simulate exhaustion
	}
	f.n--
	f.callCount++
	return nil
}
func (f *fakePoolReserver) CreateGrant(_ context.Context, _, _, _, _ uuid.UUID, _ time.Time) (uuid.UUID, error) {
	return uuid.New(), nil
}

func TestJoin_FIFORankIsMonotonicallyIncreasing(t *testing.T) {
	repo := &fakeWaitlistRepo{wl: db.Waitlist{Mode: waitlist.ModeFIFO, MaxPromotionBatch: 10}}
	svc := waitlist.NewService(repo, nil)
	wlID := uuid.New()
	e1, _ := svc.Join(context.Background(), wlID, uuid.New(), waitlist.SourceManual, nil)
	time.Sleep(time.Microsecond)
	e2, _ := svc.Join(context.Background(), wlID, uuid.New(), waitlist.SourceManual, nil)
	if e1.Rank >= e2.Rank {
		t.Fatal("FIFO rank should be monotonically increasing")
	}
}

func TestWithdraw_NotOnWaitlist_ReturnsError(t *testing.T) {
	repo := &fakeWaitlistRepo{}
	svc := waitlist.NewService(repo, nil)
	err := svc.Withdraw(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("withdraw from empty waitlist should return error")
	}
}
