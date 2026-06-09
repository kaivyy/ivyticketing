package lifecycle_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/lifecycle"
	"github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

type fakeRepo struct {
	lc    *db.RegistrationLifecycle
	phase *db.LifecyclePhase
}

func (r *fakeRepo) CreateLifecycle(_ context.Context, _ db.CreateLifecycleParams) (db.RegistrationLifecycle, error) {
	return db.RegistrationLifecycle{}, nil
}
func (r *fakeRepo) GetLifecycleByCategory(_ context.Context, _ db.GetLifecycleByCategoryParams) (db.RegistrationLifecycle, error) {
	if r.lc == nil {
		return db.RegistrationLifecycle{}, pgx.ErrNoRows
	}
	return *r.lc, nil
}
func (r *fakeRepo) GetLifecycleByCategoryID(_ context.Context, _ uuid.UUID) (db.RegistrationLifecycle, error) {
	if r.lc == nil {
		return db.RegistrationLifecycle{}, pgx.ErrNoRows
	}
	return *r.lc, nil
}
func (r *fakeRepo) ActivateLifecycle(_ context.Context, _ uuid.UUID) (db.RegistrationLifecycle, error) {
	return db.RegistrationLifecycle{}, nil
}
func (r *fakeRepo) UpdateLifecycleStatus(_ context.Context, arg db.UpdateLifecycleStatusParams) (db.RegistrationLifecycle, error) {
	if r.lc != nil {
		r.lc.Status = arg.Status
	}
	return db.RegistrationLifecycle{}, nil
}
func (r *fakeRepo) CreateLifecyclePhase(_ context.Context, _ db.CreateLifecyclePhaseParams) (db.LifecyclePhase, error) {
	return db.LifecyclePhase{}, nil
}
func (r *fakeRepo) GetActivePhaseForMode(_ context.Context, _ db.GetActivePhaseForModeParams) (db.LifecyclePhase, error) {
	if r.phase == nil {
		return db.LifecyclePhase{}, pgx.ErrNoRows
	}
	return *r.phase, nil
}
func (r *fakeRepo) ListPhasesForLifecycle(_ context.Context, _ uuid.UUID) ([]db.LifecyclePhase, error) {
	return nil, nil
}
func (r *fakeRepo) UpdateLifecyclePhaseStatus(_ context.Context, arg db.UpdateLifecyclePhaseStatusParams) (db.LifecyclePhase, error) {
	if r.phase != nil {
		r.phase.Status = arg.Status
	}
	return db.LifecyclePhase{}, nil
}
func (r *fakeRepo) ListPhasesForAutoAdvance(_ context.Context) ([]db.LifecyclePhase, error) {
	if r.phase != nil {
		return []db.LifecyclePhase{*r.phase}, nil
	}
	return nil, nil
}
func (r *fakeRepo) GetNextPendingPhase(_ context.Context, _ uuid.UUID) (db.LifecyclePhase, error) {
	return db.LifecyclePhase{}, pgx.ErrNoRows
}

func TestIsWindowOpen_NoLifecycle_FailOpen(t *testing.T) {
	svc := lifecycle.NewService(&fakeRepo{})
	open, _, err := svc.IsWindowOpen(context.Background(), uuid.New(), registration.ModeNormal)
	if err != nil {
		t.Fatal(err)
	}
	if !open {
		t.Fatal("no lifecycle row should fail-open (return true)")
	}
}

func TestIsWindowOpen_LifecyclePaused(t *testing.T) {
	lc := &db.RegistrationLifecycle{Status: lifecycle.StatusPaused}
	svc := lifecycle.NewService(&fakeRepo{lc: lc})
	open, reason, _ := svc.IsWindowOpen(context.Background(), uuid.New(), registration.ModeNormal)
	if open {
		t.Fatal("paused lifecycle should return false")
	}
	if reason != string(lifecycle.ReasonLifecyclePaused) {
		t.Fatalf("want %q got %q", lifecycle.ReasonLifecyclePaused, reason)
	}
}

func TestIsWindowOpen_ActivePhaseForMode(t *testing.T) {
	lc := &db.RegistrationLifecycle{Status: lifecycle.StatusActive}
	ph := &db.LifecyclePhase{Status: lifecycle.PhaseStatusActive, RegistrationMode: string(registration.ModeNormal)}
	svc := lifecycle.NewService(&fakeRepo{lc: lc, phase: ph})
	open, _, err := svc.IsWindowOpen(context.Background(), uuid.New(), registration.ModeNormal)
	if err != nil {
		t.Fatal(err)
	}
	if !open {
		t.Fatal("active phase for matching mode should return true")
	}
}

func TestIsWindowOpen_NoPhaseForMode(t *testing.T) {
	lc := &db.RegistrationLifecycle{Status: lifecycle.StatusActive}
	svc := lifecycle.NewService(&fakeRepo{lc: lc})
	open, reason, _ := svc.IsWindowOpen(context.Background(), uuid.New(), registration.ModeNormal)
	if open {
		t.Fatal("no active phase for mode should return false")
	}
	if reason != string(lifecycle.ReasonModeNotInLifecycle) {
		t.Fatalf("want %q got %q", lifecycle.ReasonModeNotInLifecycle, reason)
	}
}
