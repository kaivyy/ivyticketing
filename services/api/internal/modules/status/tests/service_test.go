package status_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/status"
)

// --- fake repo ---

type fakeRepo struct {
	components map[string]db.StatusComponent
	incidents  map[uuid.UUID]db.Incident
	updates    map[uuid.UUID][]db.IncidentUpdate

	createdIncident bool
	statusUpdatedTo string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		components: map[string]db.StatusComponent{},
		incidents:  map[uuid.UUID]db.Incident{},
		updates:    map[uuid.UUID][]db.IncidentUpdate{},
	}
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func (r *fakeRepo) ListComponents(ctx context.Context) ([]db.StatusComponent, error) {
	out := make([]db.StatusComponent, 0, len(r.components))
	for _, c := range r.components {
		out = append(out, c)
	}
	return out, nil
}

func (r *fakeRepo) UpdateComponent(ctx context.Context, arg db.UpdateStatusComponentParams) (db.StatusComponent, error) {
	c, ok := r.components[arg.Key]
	if !ok {
		return db.StatusComponent{}, pgx.ErrNoRows
	}
	c.Status = arg.Status
	c.UpdatedAt = ts(time.Now())
	r.components[arg.Key] = c
	return c, nil
}

func (r *fakeRepo) ListActiveIncidents(ctx context.Context) ([]db.Incident, error) {
	var out []db.Incident
	for _, inc := range r.incidents {
		if !inc.ResolvedAt.Valid {
			out = append(out, inc)
		}
	}
	return out, nil
}

func (r *fakeRepo) ListRecentIncidents(ctx context.Context, arg db.ListRecentIncidentsParams) ([]db.Incident, error) {
	var out []db.Incident
	for _, inc := range r.incidents {
		out = append(out, inc)
	}
	return out, nil
}

func (r *fakeRepo) GetIncident(ctx context.Context, id uuid.UUID) (db.Incident, error) {
	inc, ok := r.incidents[id]
	if !ok {
		return db.Incident{}, pgx.ErrNoRows
	}
	return inc, nil
}

func (r *fakeRepo) CreateIncident(ctx context.Context, arg db.CreateIncidentParams) (db.Incident, error) {
	r.createdIncident = true
	inc := db.Incident{
		ID:        uuid.New(),
		Title:     arg.Title,
		Impact:    arg.Impact,
		Status:    arg.Status,
		StartedAt: ts(time.Now()),
		CreatedAt: ts(time.Now()),
		UpdatedAt: ts(time.Now()),
	}
	r.incidents[inc.ID] = inc
	return inc, nil
}

func (r *fakeRepo) UpdateIncidentStatus(ctx context.Context, arg db.UpdateIncidentStatusParams) (db.Incident, error) {
	r.statusUpdatedTo = arg.Status
	inc, ok := r.incidents[arg.ID]
	if !ok {
		return db.Incident{}, pgx.ErrNoRows
	}
	inc.Status = arg.Status
	if arg.Status == status.IncResolved {
		inc.ResolvedAt = ts(time.Now())
	}
	inc.UpdatedAt = ts(time.Now())
	r.incidents[arg.ID] = inc
	return inc, nil
}

func (r *fakeRepo) ListIncidentUpdates(ctx context.Context, incidentID uuid.UUID) ([]db.IncidentUpdate, error) {
	return r.updates[incidentID], nil
}

func (r *fakeRepo) ListUpdatesForIncidents(ctx context.Context, ids []uuid.UUID) ([]db.IncidentUpdate, error) {
	var out []db.IncidentUpdate
	for _, id := range ids {
		out = append(out, r.updates[id]...)
	}
	return out, nil
}

func (r *fakeRepo) CreateIncidentUpdate(ctx context.Context, arg db.CreateIncidentUpdateParams) (db.IncidentUpdate, error) {
	u := db.IncidentUpdate{
		ID:         uuid.New(),
		IncidentID: arg.IncidentID,
		Status:     arg.Status,
		Body:       arg.Body,
		CreatedAt:  ts(time.Now()),
	}
	r.updates[arg.IncidentID] = append(r.updates[arg.IncidentID], u)
	return u, nil
}

func newSvc(r status.Repository) *status.Service {
	return status.NewService(r, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func seedComponents(r *fakeRepo) {
	r.components["queue"] = db.StatusComponent{Key: "queue", Name: "Antrian Virtual", Status: status.CompOperational, SortOrder: 1, UpdatedAt: ts(time.Now())}
	r.components["payment"] = db.StatusComponent{Key: "payment", Name: "Pembayaran", Status: status.CompOperational, SortOrder: 2, UpdatedAt: ts(time.Now())}
}

// --- public status ---

func TestGetPublicStatus_AllOperational(t *testing.T) {
	repo := newFakeRepo()
	seedComponents(repo)
	svc := newSvc(repo)
	resp, err := svc.GetPublicStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Overall != status.CompOperational {
		t.Fatalf("expected OPERATIONAL overall, got %q", resp.Overall)
	}
	if len(resp.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(resp.Components))
	}
	if len(resp.Incidents) != 0 {
		t.Fatalf("expected no incidents, got %d", len(resp.Incidents))
	}
}

func TestGetPublicStatus_WorstComponentWins(t *testing.T) {
	repo := newFakeRepo()
	seedComponents(repo)
	c := repo.components["payment"]
	c.Status = status.CompDown
	repo.components["payment"] = c
	svc := newSvc(repo)
	resp, err := svc.GetPublicStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Overall != status.CompDown {
		t.Fatalf("expected DOWN overall, got %q", resp.Overall)
	}
}

func TestGetPublicStatus_ActiveIncidentDegrades(t *testing.T) {
	repo := newFakeRepo()
	seedComponents(repo)
	svc := newSvc(repo)
	if _, err := svc.CreateIncident(context.Background(), uuid.New(), status.CreateIncidentRequest{
		Title: "Gangguan pembayaran", Impact: status.ImpactMajor, Body: "Sedang diselidiki",
	}); err != nil {
		t.Fatalf("create incident: %v", err)
	}
	resp, err := svc.GetPublicStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Overall != status.CompDegraded {
		t.Fatalf("expected DEGRADED overall from active incident, got %q", resp.Overall)
	}
	if len(resp.Incidents) != 1 {
		t.Fatalf("expected 1 active incident, got %d", len(resp.Incidents))
	}
	if len(resp.Incidents[0].Updates) != 1 {
		t.Fatalf("expected initial update hydrated, got %d", len(resp.Incidents[0].Updates))
	}
}

// --- components ---

func TestUpdateComponent_RejectsBadStatus(t *testing.T) {
	repo := newFakeRepo()
	seedComponents(repo)
	svc := newSvc(repo)
	if _, err := svc.UpdateComponent(context.Background(), uuid.New(), "queue", "BROKEN"); !errors.Is(err, status.ErrInvalidComponent) {
		t.Fatalf("expected ErrInvalidComponent, got %v", err)
	}
}

func TestUpdateComponent_UnknownKey(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	if _, err := svc.UpdateComponent(context.Background(), uuid.New(), "nope", status.CompDown); !errors.Is(err, status.ErrInvalidComponent) {
		t.Fatalf("expected ErrInvalidComponent for unknown key, got %v", err)
	}
}

func TestUpdateComponent_Succeeds(t *testing.T) {
	repo := newFakeRepo()
	seedComponents(repo)
	svc := newSvc(repo)
	c, err := svc.UpdateComponent(context.Background(), uuid.New(), "queue", "degraded")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if c.Status != status.CompDegraded {
		t.Fatalf("expected DEGRADED, got %q", c.Status)
	}
}

// --- incidents ---

func TestCreateIncident_RejectsEmpty(t *testing.T) {
	svc := newSvc(newFakeRepo())
	if _, err := svc.CreateIncident(context.Background(), uuid.New(), status.CreateIncidentRequest{Title: "", Body: ""}); !errors.Is(err, status.ErrInvalidIncident) {
		t.Fatalf("expected ErrInvalidIncident, got %v", err)
	}
}

func TestCreateIncident_DefaultsImpactAndStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	inc, err := svc.CreateIncident(context.Background(), uuid.New(), status.CreateIncidentRequest{
		Title: "Masalah antrian", Body: "Investigasi awal",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if inc.Impact != status.ImpactMinor {
		t.Fatalf("expected default MINOR impact, got %q", inc.Impact)
	}
	if inc.Status != status.IncInvestigating {
		t.Fatalf("expected INVESTIGATING, got %q", inc.Status)
	}
	if len(inc.Updates) != 1 {
		t.Fatalf("expected initial update, got %d", len(inc.Updates))
	}
}

func TestAddIncidentUpdate_ResolvesIncident(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	inc, _ := svc.CreateIncident(context.Background(), uuid.New(), status.CreateIncidentRequest{
		Title: "Gangguan", Impact: status.ImpactMajor, Body: "awal",
	})
	id := uuid.MustParse(inc.ID)
	out, err := svc.AddIncidentUpdate(context.Background(), uuid.New(), id, status.AddIncidentUpdateRequest{
		Status: status.IncResolved, Body: "Sudah pulih",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != status.IncResolved {
		t.Fatalf("expected RESOLVED, got %q", out.Status)
	}
	if out.ResolvedAt == nil {
		t.Fatalf("expected resolvedAt set")
	}
	if repo.statusUpdatedTo != status.IncResolved {
		t.Fatalf("expected UpdateIncidentStatus called with RESOLVED")
	}
}

func TestAddIncidentUpdate_NotFound(t *testing.T) {
	svc := newSvc(newFakeRepo())
	if _, err := svc.AddIncidentUpdate(context.Background(), uuid.New(), uuid.New(), status.AddIncidentUpdateRequest{
		Status: status.IncMonitoring, Body: "x",
	}); !errors.Is(err, status.ErrIncidentNotFound) {
		t.Fatalf("expected ErrIncidentNotFound, got %v", err)
	}
}

func TestAddIncidentUpdate_RejectsBadStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	inc, _ := svc.CreateIncident(context.Background(), uuid.New(), status.CreateIncidentRequest{Title: "x", Body: "y"})
	id := uuid.MustParse(inc.ID)
	if _, err := svc.AddIncidentUpdate(context.Background(), uuid.New(), id, status.AddIncidentUpdateRequest{
		Status: "NOPE", Body: "z",
	}); !errors.Is(err, status.ErrInvalidIncident) {
		t.Fatalf("expected ErrInvalidIncident, got %v", err)
	}
}
