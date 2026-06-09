package access

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type PoolManager struct{ repo Repository }

func NewPoolManager(repo Repository) *PoolManager { return &PoolManager{repo: repo} }

// CreatePool creates a new access pool and returns its ID.
func (p *PoolManager) CreatePool(ctx context.Context, orgID, eventID, categoryID uuid.UUID, poolType, name string, totalSlots int, createdBy uuid.UUID) (uuid.UUID, error) {
	pool, err := p.repo.CreateAccessPool(ctx, db.CreateAccessPoolParams{
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

// ReserveSlot atomically increments reserved_slots. Returns ErrPoolExhausted if full.
func (p *PoolManager) ReserveSlot(ctx context.Context, poolID uuid.UUID) error {
	_, err := p.repo.ReservePoolSlot(ctx, poolID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPoolExhausted
	}
	return err
}

// CreateGrant issues an AccessGrant for a participant against a pool.
func (p *PoolManager) CreateGrant(ctx context.Context, poolID, participantID, eventID, categoryID uuid.UUID, expiresAt time.Time) (uuid.UUID, error) {
	grant, err := p.repo.CreateAccessGrant(ctx, db.CreateAccessGrantParams{
		PoolID:        &poolID,
		ParticipantID: participantID,
		EventID:       eventID,
		CategoryID:    categoryID,
		ExpiresAt:     pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return uuid.Nil, err
	}
	if err := p.repo.ConsumePoolSlot(ctx, poolID); err != nil {
		return uuid.Nil, err
	}
	return grant.ID, nil
}

// CheckGrant validates an existing grant is ACTIVE and not expired.
// grantToken is the grant UUID as a string (passed as admissionToken at checkout).
func (p *PoolManager) CheckGrant(ctx context.Context, participantID, categoryID uuid.UUID, grantToken string) error {
	grantID, err := uuid.Parse(grantToken)
	if err != nil {
		return ErrGrantNotFound
	}
	grant, err := p.repo.GetAccessGrant(ctx, grantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrGrantNotFound
	}
	if err != nil {
		return err
	}
	if grant.Status == GrantStatusExpired {
		return ErrGrantExpired
	}
	if grant.Status == GrantStatusConsumed {
		return ErrGrantAlreadyConsumed
	}
	if grant.ExpiresAt.Valid && time.Now().After(grant.ExpiresAt.Time) {
		return ErrGrantExpired
	}
	return nil
}

// ExpireStaleGrants scans ACTIVE grants past their expiry and marks them EXPIRED.
func (p *PoolManager) ExpireStaleGrants(ctx context.Context, batchSize int32) error {
	grants, err := p.repo.ListExpiredActiveGrants(ctx, batchSize)
	if err != nil {
		return err
	}
	for _, g := range grants {
		if err := p.repo.ExpireGrant(ctx, g.ID); err != nil {
			continue
		}
		if g.PoolID != nil {
			_ = p.repo.ReleasePoolSlot(ctx, *g.PoolID)
		}
	}
	return nil
}
