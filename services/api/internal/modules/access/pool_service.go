package access

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// PoolService handles access pool lifecycle operations.
type PoolService struct {
	repo Repository
}

// NewPoolService returns a new PoolService.
func NewPoolService(repo Repository) *PoolService {
	return &PoolService{repo: repo}
}

// CreatePool creates a new access pool and returns its ID.
func (s *PoolService) CreatePool(ctx context.Context, orgID, eventID, categoryID uuid.UUID, poolType, name string, totalSlots int, createdBy uuid.UUID) (uuid.UUID, error) {
	pool, err := s.repo.CreateAccessPool(ctx, db.CreateAccessPoolParams{
		OrganizationID: orgID,
		EventID:        eventID,
		CategoryID:     categoryID,
		PoolType:       poolType,
		Name:           name,
		TotalSlots:     int32(totalSlots),
		CreatedBy:      createdBy,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return pool.ID, nil
}

// SetVisible sets the is_visible_to_participants flag on a pool.
func (s *PoolService) SetVisible(ctx context.Context, poolID uuid.UUID, visible bool) error {
	_, err := s.repo.UpdateAccessPoolColumns(ctx, db.UpdateAccessPoolColumnsParams{
		ID:                      poolID,
		IsVisibleToParticipants: visible,
	})
	return err
}

// SetEligibilityRule sets the eligibility_rule jsonb on a pool.
func (s *PoolService) SetEligibilityRule(ctx context.Context, poolID uuid.UUID, rule []byte) error {
	_, err := s.repo.UpdateAccessPoolColumns(ctx, db.UpdateAccessPoolColumnsParams{
		ID:              poolID,
		EligibilityRule: rule,
	})
	return err
}

// AdjustTotalSlots adds delta to total_slots (delta may be negative to shrink).
// A delta of 0 is a no-op.
func (s *PoolService) AdjustTotalSlots(ctx context.Context, poolID uuid.UUID, delta int) error {
	if delta == 0 {
		return nil
	}
	_, err := s.repo.TransferPoolSlots(ctx, db.TransferPoolSlotsParams{
		ID:         poolID,
		TotalSlots: int32(delta),
	})
	return err
}
