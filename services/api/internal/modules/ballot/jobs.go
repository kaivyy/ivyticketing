package ballot

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type BatchPromoter interface {
	PromoteBatch(ctx context.Context, waitlistID uuid.UUID) error
}

type WinnerExpirer struct {
	repo     Repository
	promoter BatchPromoter
}

func NewWinnerExpirer(repo Repository, promoter BatchPromoter) *WinnerExpirer {
	return &WinnerExpirer{repo: repo, promoter: promoter}
}

func (e *WinnerExpirer) Run(ctx context.Context) error {
	winners, err := e.repo.ListExpiringWinners(ctx, 100)
	if err != nil {
		return err
	}
	affected := map[uuid.UUID]bool{}
	for _, w := range winners {
		_, err := e.repo.UpdateBallotEntryStatus(ctx, db.UpdateBallotEntryStatusParams{
			ID:     w.ID,
			Status: StatusLapsed,
		})
		if err != nil {
			continue
		}
		affected[w.DrawID] = true
	}
	for drawID := range affected {
		draw, err := e.repo.GetBallotDraw(ctx, drawID)
		if err != nil || draw.WaitlistID == nil {
			continue
		}
		_ = e.promoter.PromoteBatch(ctx, *draw.WaitlistID)
	}
	return nil
}
