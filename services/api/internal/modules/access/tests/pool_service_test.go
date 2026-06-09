package access_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

func TestPoolService_CreateAndGetPool(t *testing.T) {
	id := uuid.New()
	repo := &fakeAccessRepo{pool: db.AccessPool{ID: id}}
	svc := access.NewPoolService(repo)
	poolID, err := svc.CreatePool(context.Background(),
		uuid.New(), uuid.New(), uuid.New(),
		access.PoolTypeReserved, "Test Pool", 100, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if poolID == uuid.Nil {
		t.Fatal("poolID should not be nil")
	}
	if poolID != id {
		t.Fatalf("want %v, got %v", id, poolID)
	}
}

func TestPoolService_SetVisible(t *testing.T) {
	repo := &fakeAccessRepo{}
	svc := access.NewPoolService(repo)
	if err := svc.SetVisible(context.Background(), uuid.New(), true); err != nil {
		t.Fatal(err)
	}
}

func TestPoolService_AdjustTotalSlots_ZeroDelta(t *testing.T) {
	repo := &fakeAccessRepo{}
	svc := access.NewPoolService(repo)
	// delta=0 should be a no-op, not an error
	if err := svc.AdjustTotalSlots(context.Background(), uuid.New(), 0); err != nil {
		t.Fatal(err)
	}
}

func TestPoolService_AdjustTotalSlots_Positive(t *testing.T) {
	repo := &fakeAccessRepo{}
	svc := access.NewPoolService(repo)
	if err := svc.AdjustTotalSlots(context.Background(), uuid.New(), 10); err != nil {
		t.Fatal(err)
	}
}
