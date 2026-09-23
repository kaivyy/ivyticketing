package competitions

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository encapsulates database access for multi-sport competitions.
type Repository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewRepository creates a new competition repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
		q:    db.New(pool),
	}
}

// ExecTx executes operations within a database transaction.
func (r *Repository) ExecTx(ctx context.Context, fn func(*Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	txRepo := &Repository{pool: r.pool, q: db.New(tx)}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// Event validation
func (r *Repository) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}

// Sports and Disciplines
func (r *Repository) ListSports(ctx context.Context) ([]db.Sport, error) {
	return r.q.ListSports(ctx)
}

func (r *Repository) GetSportByID(ctx context.Context, id string) (db.Sport, error) {
	return r.q.GetSportByID(ctx, id)
}

func (r *Repository) ListDisciplinesBySport(ctx context.Context, sportID string) ([]db.SportDiscipline, error) {
	return r.q.ListSportDisciplinesBySport(ctx, sportID)
}

func (r *Repository) GetDisciplineByID(ctx context.Context, id string) (db.SportDiscipline, error) {
	return r.q.GetSportDisciplineByID(ctx, id)
}

// Competition Configs
func (r *Repository) UpsertConfig(ctx context.Context, params db.UpsertCompetitionConfigParams) (db.CompetitionConfig, error) {
	return r.q.UpsertCompetitionConfig(ctx, params)
}

func (r *Repository) GetConfigByEvent(ctx context.Context, eventID uuid.UUID) (db.CompetitionConfig, error) {
	return r.q.GetCompetitionConfigByEvent(ctx, eventID)
}

func (r *Repository) GetConfigByCategory(ctx context.Context, eventID, categoryID uuid.UUID) (db.CompetitionConfig, error) {
	catID := categoryID
	return r.q.GetCompetitionConfigByCategory(ctx, db.GetCompetitionConfigByCategoryParams{
		EventID:    eventID,
		CategoryID: &catID,
	})
}

// Teams & Rosters
func (r *Repository) CreateTeam(ctx context.Context, params db.CreateTeamParams) (db.Team, error) {
	return r.q.CreateTeam(ctx, params)
}

func (r *Repository) GetTeamByID(ctx context.Context, id uuid.UUID) (db.Team, error) {
	return r.q.GetTeamByID(ctx, id)
}

func (r *Repository) ListTeamsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.Team, error) {
	return r.q.ListTeamsByEvent(ctx, eventID)
}

func (r *Repository) UpdateTeamStatus(ctx context.Context, id uuid.UUID, status string) (db.Team, error) {
	return r.q.UpdateTeamStatus(ctx, db.UpdateTeamStatusParams{
		ID:     id,
		Status: status,
	})
}

func (r *Repository) CreateTeamRosterMember(ctx context.Context, params db.CreateTeamRosterMemberParams) (db.TeamRoster, error) {
	return r.q.CreateTeamRosterMember(ctx, params)
}

func (r *Repository) ListTeamRoster(ctx context.Context, teamID uuid.UUID) ([]db.TeamRoster, error) {
	return r.q.ListTeamRosterByTeam(ctx, teamID)
}

func (r *Repository) DeleteTeamRosterMember(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteTeamRosterMember(ctx, id)
}

// Stages & Matches
func (r *Repository) CreateStage(ctx context.Context, params db.CreateCompetitionStageParams) (db.CompetitionStage, error) {
	return r.q.CreateCompetitionStage(ctx, params)
}

func (r *Repository) GetStageByID(ctx context.Context, id uuid.UUID) (db.CompetitionStage, error) {
	return r.q.GetCompetitionStageByID(ctx, id)
}

func (r *Repository) ListStagesByEvent(ctx context.Context, eventID uuid.UUID) ([]db.CompetitionStage, error) {
	return r.q.ListCompetitionStagesByEvent(ctx, eventID)
}

func (r *Repository) UpdateStageStatus(ctx context.Context, id uuid.UUID, status string) (db.CompetitionStage, error) {
	return r.q.UpdateCompetitionStageStatus(ctx, db.UpdateCompetitionStageStatusParams{
		ID:     id,
		Status: status,
	})
}

func (r *Repository) CreateMatch(ctx context.Context, params db.CreateCompetitionMatchParams) (db.CompetitionMatch, error) {
	return r.q.CreateCompetitionMatch(ctx, params)
}

func (r *Repository) GetMatchByID(ctx context.Context, id uuid.UUID) (db.CompetitionMatch, error) {
	return r.q.GetCompetitionMatchByID(ctx, id)
}

func (r *Repository) ListMatchesByStage(ctx context.Context, stageID uuid.UUID) ([]db.CompetitionMatch, error) {
	return r.q.ListCompetitionMatchesByStage(ctx, stageID)
}

func (r *Repository) UpdateMatchScore(ctx context.Context, params db.UpdateCompetitionMatchScoreParams) (db.CompetitionMatch, error) {
	return r.q.UpdateCompetitionMatchScore(ctx, params)
}

func (r *Repository) UpdateMatchScoreScoped(ctx context.Context, params db.UpdateCompetitionMatchScoreScopedParams) (db.CompetitionMatch, error) {
	return r.q.UpdateCompetitionMatchScoreScoped(ctx, params)
}

func (r *Repository) GetMatchScoped(ctx context.Context, id, eventID, orgID uuid.UUID) (db.CompetitionMatch, error) {
	return r.q.GetCompetitionMatchScoped(ctx, db.GetCompetitionMatchScopedParams{
		ID:             id,
		EventID:        eventID,
		OrganizationID: orgID,
	})
}

func (r *Repository) UpdateMatchParticipants(ctx context.Context, stageID uuid.UUID, roundNum, matchNum int, homeEntryID, awayEntryID *uuid.UUID) error {
	return r.q.UpdateCompetitionMatchParticipants(ctx, db.UpdateCompetitionMatchParticipantsParams{
		StageID:     stageID,
		HomeEntryID: homeEntryID,
		AwayEntryID: awayEntryID,
		RoundNumber: int32(roundNum),
		MatchNumber: int32(matchNum),
	})
}

func (r *Repository) GetMatchByRoundAndNumber(ctx context.Context, stageID uuid.UUID, roundNum, matchNum int) (db.CompetitionMatch, error) {
	return r.q.GetCompetitionMatchByRoundAndNumber(ctx, db.GetCompetitionMatchByRoundAndNumberParams{
		StageID:     stageID,
		RoundNumber: int32(roundNum),
		MatchNumber: int32(matchNum),
	})
}

func (r *Repository) UpdateMatchParticipantsScheduled(ctx context.Context, stageID uuid.UUID, roundNum, matchNum int, homeEntryID, awayEntryID *uuid.UUID) (int64, error) {
	return r.q.UpdateCompetitionMatchParticipantsScheduled(ctx, db.UpdateCompetitionMatchParticipantsScheduledParams{
		StageID:     stageID,
		HomeEntryID: homeEntryID,
		AwayEntryID: awayEntryID,
		RoundNumber: int32(roundNum),
		MatchNumber: int32(matchNum),
	})
}


// Participant Entries
func (r *Repository) CreateParticipantEntry(ctx context.Context, params db.CreateParticipantEntryParams) (db.ParticipantEntry, error) {
	return r.q.CreateParticipantEntry(ctx, params)
}

func (r *Repository) GetParticipantEntryByID(ctx context.Context, id uuid.UUID) (db.ParticipantEntry, error) {
	return r.q.GetParticipantEntryByID(ctx, id)
}

func (r *Repository) GetParticipantEntryByTicket(ctx context.Context, ticketID uuid.UUID) (db.ParticipantEntry, error) {
	tID := ticketID
	return r.q.GetParticipantEntryByTicket(ctx, &tID)
}

func (r *Repository) ListParticipantEntriesByEvent(ctx context.Context, eventID uuid.UUID) ([]db.ParticipantEntry, error) {
	return r.q.ListParticipantEntriesByEvent(ctx, eventID)
}

func (r *Repository) UpdateParticipantEntryStatus(ctx context.Context, id uuid.UUID, status string) (db.ParticipantEntry, error) {
	return r.q.UpdateParticipantEntryStatus(ctx, db.UpdateParticipantEntryStatusParams{
		ID:     id,
		Status: status,
	})
}

// Observations
func (r *Repository) CreateObservation(ctx context.Context, params db.CreateCompetitionObservationParams) (db.CompetitionObservation, error) {
	return r.q.CreateCompetitionObservation(ctx, params)
}

func (r *Repository) ListObservationsByMatch(ctx context.Context, matchID uuid.UUID) ([]db.CompetitionObservation, error) {
	mID := matchID
	return r.q.ListObservationsByMatch(ctx, &mID)
}

func (r *Repository) ListObservationsByStage(ctx context.Context, stageID uuid.UUID) ([]db.CompetitionObservation, error) {
	sID := stageID
	return r.q.ListObservationsByStage(ctx, &sID)
}

func (r *Repository) ListObservationsByEntry(ctx context.Context, entryID uuid.UUID) ([]db.CompetitionObservation, error) {
	eID := entryID
	return r.q.ListObservationsByEntry(ctx, &eID)
}

// Results & Standings
func (r *Repository) UpsertResult(ctx context.Context, params db.UpsertCompetitionResultParams) (db.CompetitionResult, error) {
	return r.q.UpsertCompetitionResult(ctx, params)
}

func (r *Repository) GetResultByEntry(ctx context.Context, stageID, entryID uuid.UUID) (db.CompetitionResult, error) {
	sID := stageID
	return r.q.GetCompetitionResultByEntry(ctx, db.GetCompetitionResultByEntryParams{
		StageID: &sID,
		EntryID: entryID,
	})
}

func (r *Repository) ListResultsByStage(ctx context.Context, stageID uuid.UUID) ([]db.ListCompetitionResultsByStageRow, error) {
	sID := stageID
	return r.q.ListCompetitionResultsByStage(ctx, &sID)
}

func (r *Repository) ListResultsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.ListCompetitionResultsByEventRow, error) {
	return r.q.ListCompetitionResultsByEvent(ctx, eventID)
}

func (r *Repository) RankStageByTime(ctx context.Context, stageID uuid.UUID) error {
	sID := stageID
	return r.q.RankCompetitionStageByTime(ctx, &sID)
}

func (r *Repository) RankStageByPoints(ctx context.Context, stageID uuid.UUID) error {
	sID := stageID
	return r.q.RankCompetitionStageByPoints(ctx, &sID)
}
