package lifecycle

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// IsWindowOpen returns true when registration for the given mode is open.
// Fail-open: if no lifecycle row exists, registration is unrestricted.
func (s *Service) IsWindowOpen(ctx context.Context, categoryID uuid.UUID, mode registration.Mode) (bool, string, error) {
	lc, err := s.repo.GetLifecycleByCategoryID(ctx, categoryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, "", nil // fail-open: no lifecycle configured
	}
	if err != nil {
		return false, "", err
	}
	if lc.Status == StatusPaused {
		return false, string(ReasonLifecyclePaused), nil
	}
	if lc.Status != StatusActive {
		return false, string(ReasonModeNotInLifecycle), nil
	}
	_, err = s.repo.GetActivePhaseForMode(ctx, db.GetActivePhaseForModeParams{
		LifecycleID:      lc.ID,
		RegistrationMode: string(mode),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, string(ReasonModeNotInLifecycle), nil
	}
	if err != nil {
		return false, "", err
	}
	return true, "", nil
}

func (s *Service) PauseLifecycle(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.UpdateLifecycleStatus(ctx, db.UpdateLifecycleStatusParams{ID: id, Status: StatusPaused})
	return err
}

func (s *Service) ResumeLifecycle(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.UpdateLifecycleStatus(ctx, db.UpdateLifecycleStatusParams{ID: id, Status: StatusActive})
	return err
}

func (s *Service) CompletePhase(ctx context.Context, phaseID uuid.UUID) error {
	_, err := s.repo.UpdateLifecyclePhaseStatus(ctx, db.UpdateLifecyclePhaseStatusParams{
		ID:     phaseID,
		Status: PhaseStatusCompleted,
	})
	return err
}

func (s *Service) AdvanceToNextPhase(ctx context.Context, lifecycleID uuid.UUID) error {
	next, err := s.repo.GetNextPendingPhase(ctx, lifecycleID)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = s.repo.UpdateLifecycleStatus(ctx, db.UpdateLifecycleStatusParams{
			ID:     lifecycleID,
			Status: StatusCompleted,
		})
		return err
	}
	if err != nil {
		return err
	}
	_, err = s.repo.UpdateLifecyclePhaseStatus(ctx, db.UpdateLifecyclePhaseStatusParams{
		ID:     next.ID,
		Status: PhaseStatusActive,
	})
	return err
}
