package waitlist

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type PoolReserver interface {
	ReserveSlot(ctx context.Context, poolID uuid.UUID) error
	CreateGrant(ctx context.Context, poolID, participantID, eventID, categoryID uuid.UUID, expiresAt time.Time) (uuid.UUID, error)
}

type Service struct {
	repo     Repository
	reserver PoolReserver
}

func NewService(repo Repository, reserver PoolReserver) *Service {
	return &Service{repo: repo, reserver: reserver}
}

func (s *Service) Join(ctx context.Context, waitlistID, participantID uuid.UUID, source string, sourceRefID *uuid.UUID) (db.WaitlistEntry, error) {
	rank := FIFORank(time.Now())
	return s.repo.JoinWaitlist(ctx, db.JoinWaitlistParams{
		WaitlistID:    waitlistID,
		ParticipantID: participantID,
		Source:        source,
		SourceRefID:   sourceRefID,
		Rank:          rank,
	})
}

func (s *Service) PromoteBatch(ctx context.Context, waitlistID uuid.UUID) error {
	wl, err := s.repo.GetWaitlist(ctx, waitlistID)
	if err != nil {
		return err
	}
	entries, err := s.repo.ListWaitingEntries(ctx, db.ListWaitingEntriesParams{
		WaitlistID: waitlistID,
		Limit:      int32(wl.MaxPromotionBatch),
	})
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var poolID uuid.UUID
		if wl.PoolID != nil {
			poolID = *wl.PoolID
		}
		if s.reserver != nil && wl.PoolID != nil {
			if err := s.reserver.ReserveSlot(ctx, poolID); err != nil {
				break // pool exhausted — stop promoting
			}
			grantID, err := s.reserver.CreateGrant(ctx, poolID, entry.ParticipantID,
				entry.EventID, entry.CategoryID,
				time.Now().Add(time.Duration(wl.PromotionWindowHours)*time.Hour))
			if err != nil {
				continue
			}
			_, _ = s.repo.UpdateWaitlistEntryStatus(ctx, db.UpdateWaitlistEntryStatusParams{
				ID:            entry.ID,
				Status:        StatusPromoted,
				AccessGrantID: &grantID,
			})
		} else {
			_, _ = s.repo.UpdateWaitlistEntryStatus(ctx, db.UpdateWaitlistEntryStatusParams{
				ID:     entry.ID,
				Status: StatusPromoted,
			})
		}
	}
	return nil
}

func (s *Service) Expire(ctx context.Context, entryID uuid.UUID) error {
	_, err := s.repo.UpdateWaitlistEntryStatus(ctx, db.UpdateWaitlistEntryStatusParams{
		ID:     entryID,
		Status: StatusExpired,
	})
	return err
}

func (s *Service) Withdraw(ctx context.Context, waitlistID, participantID uuid.UUID) error {
	entry, err := s.repo.GetWaitlistEntry(ctx, db.GetWaitlistEntryParams{
		WaitlistID:    waitlistID,
		ParticipantID: participantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotOnWaitlist
	}
	if err != nil {
		return err
	}
	_, err = s.repo.UpdateWaitlistEntryStatus(ctx, db.UpdateWaitlistEntryStatusParams{
		ID:     entry.ID,
		Status: StatusWithdrawn,
	})
	return err
}
