package access

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// LifecycleWindowChecker verifies whether a registration window is open.
type LifecycleWindowChecker interface {
	IsWindowOpen(ctx context.Context, categoryID uuid.UUID, mode registration.Mode) (bool, registration.WindowClosedReason, error)
}

// PriorityChecker auto-issues access grants for PRIORITY pools when the
// PRIORITY_ACCESS lifecycle window is open and the user is eligible.
// Implements registration.PriorityChecker (duck-typed via CheckPriorityAdmission).
type PriorityChecker struct {
	repo      Repository
	lifecycle LifecycleWindowChecker
	elig      *EligibilityChecker
}

// NewPriorityChecker returns a PriorityChecker.
func NewPriorityChecker(repo Repository, lc LifecycleWindowChecker, elig *EligibilityChecker) *PriorityChecker {
	return &PriorityChecker{repo: repo, lifecycle: lc, elig: elig}
}

// CheckPriorityAdmission verifies priority admission for a participant.
//
// If grantToken is non-empty it delegates to CheckGrant (token-based path used
// at checkout after a grant has already been issued).
//
// Otherwise it:
//  1. Confirms the PRIORITY_ACCESS window is open via LifecycleWindowChecker.
//  2. Locates the PRIORITY pool for the category.
//  3. Checks eligibility against the pool's eligibility_rule.
//  4. Issues a grant (idempotent — skips if one already exists).
func (p *PriorityChecker) CheckPriorityAdmission(ctx context.Context, participantID, eventID, categoryID uuid.UUID, grantToken string) error {
	// Token-based path: grant already issued, just verify it.
	if grantToken != "" {
		pm := NewPoolManager(p.repo)
		return pm.CheckGrant(ctx, participantID, categoryID, grantToken)
	}

	// 1. Priority window check.
	open, reason, err := p.lifecycle.IsWindowOpen(ctx, categoryID, registration.ModePriorityAccess)
	if err != nil {
		return err
	}
	if !open {
		return apperr.New(409, "PRIORITY_WINDOW_CLOSED", string(reason))
	}

	// 2. Find the PRIORITY pool for this category.
	pools, err := p.repo.ListVisiblePoolsByCategory(ctx, db.ListVisiblePoolsByCategoryParams{
		EventID:    eventID,
		CategoryID: categoryID,
	})
	if err != nil {
		return err
	}
	var priorityPool *db.AccessPool
	for i := range pools {
		if pools[i].PoolType == PoolTypePriority {
			priorityPool = &pools[i]
			break
		}
	}
	if priorityPool == nil {
		return ErrPoolExhausted
	}

	// 3. Eligibility check (skip if rule is empty/null).
	if len(priorityPool.EligibilityRule) > 0 {
		ok, reason, err := p.elig.Check(ctx, participantID, priorityPool.OrganizationID, priorityPool.EligibilityRule)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: %s", ErrNotEligible, reason)
		}
	}

	// 4. Check if participant already has an active grant (idempotent).
	existing, err := p.repo.GetActiveGrantForParticipant(ctx, db.GetActiveGrantForParticipantParams{
		ParticipantID: participantID,
		CategoryID:    categoryID,
	})
	if err == nil && existing.Status == GrantStatusActive {
		return nil // already granted
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// 5. Reserve slot atomically.
	_, err = p.repo.ReservePoolSlot(ctx, priorityPool.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPoolExhausted
	}
	if err != nil {
		return err
	}

	// 6. Issue the grant. ValidUntil from pool; fall back to 24h from now.
	expiresAt := pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true}
	if priorityPool.ValidUntil.Valid {
		expiresAt = priorityPool.ValidUntil
	}

	_, err = p.repo.CreateAccessGrant(ctx, db.CreateAccessGrantParams{
		PoolID:        &priorityPool.ID,
		ParticipantID: participantID,
		CategoryID:    categoryID,
		EventID:       priorityPool.EventID,
		ExpiresAt:     expiresAt,
	})
	return err
}
