package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// HashCode returns the sha256 hex of a plain-text code. Never store the plain text.
func HashCode(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

type CodeService struct {
	repo        Repository
	eligibility *EligibilityChecker
}

func NewCodeService(repo Repository, elig *EligibilityChecker) *CodeService {
	return &CodeService{repo: repo, eligibility: elig}
}

// Redeem validates a plain-text code, optionally reserves a pool slot, and issues an AccessGrant.
// The plain-text code is hashed immediately and never stored.
func (s *CodeService) Redeem(ctx context.Context, participantID, eventID, categoryID uuid.UUID, plainCode string) (db.AccessGrant, error) {
	hash := HashCode(plainCode)
	code, err := s.repo.GetAccessCodeByHash(ctx, db.GetAccessCodeByHashParams{
		EventID:       eventID,
		CodeValueHash: hash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.AccessGrant{}, ErrCodeNotFound
	}
	if err != nil {
		return db.AccessGrant{}, err
	}

	// Expiry check
	now := time.Now()
	if code.ValidFrom.Valid && now.Before(code.ValidFrom.Time) {
		return db.AccessGrant{}, ErrCodeExpired
	}
	if code.ValidUntil.Valid && now.After(code.ValidUntil.Time) {
		return db.AccessGrant{}, ErrCodeExpired
	}

	// Exhaustion check
	if code.UseCount >= code.MaxUses {
		return db.AccessGrant{}, ErrCodeExhausted
	}

	// Eligibility check
	if len(code.EligibilityRule) > 0 {
		ok, reason, eligErr := s.eligibility.Check(ctx, participantID, code.OrganizationID, code.EligibilityRule)
		if eligErr != nil {
			return db.AccessGrant{}, eligErr
		}
		if !ok {
			return db.AccessGrant{}, fmt.Errorf("%w: %s", ErrNotEligible, reason)
		}
	}

	// Reserve pool slot if code has a pool
	if code.PoolID != nil {
		if _, rErr := s.repo.ReservePoolSlot(ctx, *code.PoolID); rErr != nil {
			return db.AccessGrant{}, ErrPoolExhausted
		}
	}

	// Issue grant
	expiresAt := code.ValidUntil.Time
	codeID := code.ID
	grant, err := s.repo.CreateAccessGrant(ctx, db.CreateAccessGrantParams{
		PoolID:        code.PoolID,
		ParticipantID: participantID,
		EventID:       eventID,
		CategoryID:    categoryID,
		CodeID:        &codeID,
		ExpiresAt:     pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return db.AccessGrant{}, err
	}

	// Increment use_count with optimistic guard — if 0 rows returned, someone else exhausted it
	if _, incErr := s.repo.IncrementCodeUseCount(ctx, code.ID); incErr != nil {
		if errors.Is(incErr, pgx.ErrNoRows) {
			return db.AccessGrant{}, ErrCodeExhausted
		}
		return db.AccessGrant{}, incErr
	}

	return grant, nil
}

// Create stores a new access code. The plain-text code is hashed before storage.
func (s *CodeService) Create(
	ctx context.Context,
	orgID, eventID uuid.UUID,
	categoryID *uuid.UUID,
	codeType, plainCode string,
	maxUses int32,
	validFrom, validUntil time.Time,
	poolID *uuid.UUID,
	createdBy uuid.UUID,
) (db.AccessCode, error) {
	hash := HashCode(plainCode)
	params := db.CreateAccessCodeParams{
		OrganizationID: orgID,
		EventID:        eventID,
		CodeType:       codeType,
		CodeValueHash:  hash,
		IsSingleUse:    maxUses == 1,
		MaxUses:        maxUses,
		ValidFrom:      pgtype.Timestamptz{Time: validFrom, Valid: true},
		ValidUntil:     pgtype.Timestamptz{Time: validUntil, Valid: true},
		CreatedBy:      createdBy,
	}
	if categoryID != nil {
		params.CategoryID = categoryID
	}
	if poolID != nil {
		params.PoolID = poolID
	}
	return s.repo.CreateAccessCode(ctx, params)
}

// Revoke immediately expires a code by setting valid_until = now().
func (s *CodeService) Revoke(ctx context.Context, codeID uuid.UUID) error {
	return s.repo.RevokeAccessCode(ctx, codeID)
}
