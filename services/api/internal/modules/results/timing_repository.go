package results

import (
	"context"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Timing Configs
func (r *sqlcRepo) UpsertTimingConfig(ctx context.Context, arg db.UpsertTimingConfigParams) (db.TimingConfig, error) {
	return r.q.UpsertTimingConfig(ctx, arg)
}

func (r *sqlcRepo) GetTimingConfigByEvent(ctx context.Context, eventID uuid.UUID) (db.TimingConfig, error) {
	return r.q.GetTimingConfigByEvent(ctx, eventID)
}

func (r *sqlcRepo) GetTimingConfigByTokenHash(ctx context.Context, arg db.GetTimingConfigByTokenHashParams) (db.TimingConfig, error) {
	return r.q.GetTimingConfigByTokenHash(ctx, arg)
}

// Race Waves
func (r *sqlcRepo) CreateRaceWave(ctx context.Context, arg db.CreateRaceWaveParams) (db.RaceWafe, error) {
	return r.q.CreateRaceWave(ctx, arg)
}

func (r *sqlcRepo) UpdateRaceWave(ctx context.Context, arg db.UpdateRaceWaveParams) (db.RaceWafe, error) {
	return r.q.UpdateRaceWave(ctx, arg)
}

func (r *sqlcRepo) ListRaceWavesByEvent(ctx context.Context, eventID uuid.UUID) ([]db.RaceWafe, error) {
	return r.q.ListRaceWavesByEvent(ctx, eventID)
}

func (r *sqlcRepo) GetRaceWaveByID(ctx context.Context, id uuid.UUID) (db.RaceWafe, error) {
	return r.q.GetRaceWaveByID(ctx, id)
}

func (r *sqlcRepo) DeleteRaceWave(ctx context.Context, arg db.DeleteRaceWaveParams) error {
	return r.q.DeleteRaceWave(ctx, arg)
}

// Timing Checkpoints
func (r *sqlcRepo) UpsertTimingCheckpoint(ctx context.Context, arg db.UpsertTimingCheckpointParams) (db.TimingCheckpoint, error) {
	return r.q.UpsertTimingCheckpoint(ctx, arg)
}

func (r *sqlcRepo) ListTimingCheckpointsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.TimingCheckpoint, error) {
	return r.q.ListTimingCheckpointsByEvent(ctx, eventID)
}

func (r *sqlcRepo) GetTimingCheckpointByCode(ctx context.Context, arg db.GetTimingCheckpointByCodeParams) (db.TimingCheckpoint, error) {
	return r.q.GetTimingCheckpointByCode(ctx, arg)
}

func (r *sqlcRepo) DeleteTimingCheckpoint(ctx context.Context, arg db.DeleteTimingCheckpointParams) error {
	return r.q.DeleteTimingCheckpoint(ctx, arg)
}

// BIB Transponder Mappings
func (r *sqlcRepo) InsertBibTransponderMapping(ctx context.Context, arg db.InsertBibTransponderMappingParams) (db.BibTransponderMapping, error) {
	return r.q.InsertBibTransponderMapping(ctx, arg)
}

func (r *sqlcRepo) DeactivateBibMappings(ctx context.Context, arg db.DeactivateBibMappingsParams) error {
	return r.q.DeactivateBibMappings(ctx, arg)
}

func (r *sqlcRepo) DeactivateChipMappings(ctx context.Context, arg db.DeactivateChipMappingsParams) error {
	return r.q.DeactivateChipMappings(ctx, arg)
}

func (r *sqlcRepo) GetActiveMappingByChip(ctx context.Context, arg db.GetActiveMappingByChipParams) (db.BibTransponderMapping, error) {
	return r.q.GetActiveMappingByChip(ctx, arg)
}

func (r *sqlcRepo) GetActiveMappingByBib(ctx context.Context, arg db.GetActiveMappingByBibParams) (db.BibTransponderMapping, error) {
	return r.q.GetActiveMappingByBib(ctx, arg)
}

func (r *sqlcRepo) ListMappingsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.BibTransponderMapping, error) {
	return r.q.ListMappingsByEvent(ctx, eventID)
}

// Timing Passings
func (r *sqlcRepo) InsertTimingPassing(ctx context.Context, arg db.InsertTimingPassingParams) (db.TimingPassing, error) {
	return r.q.InsertTimingPassing(ctx, arg)
}

func (r *sqlcRepo) InsertTimingPassingFallback(ctx context.Context, arg db.InsertTimingPassingFallbackParams) (db.TimingPassing, error) {
	return r.q.InsertTimingPassingFallback(ctx, arg)
}

func (r *sqlcRepo) ListUnprocessedPassings(ctx context.Context, arg db.ListUnprocessedPassingsParams) ([]db.TimingPassing, error) {
	return r.q.ListUnprocessedPassings(ctx, arg)
}

func (r *sqlcRepo) MarkPassingsProcessed(ctx context.Context, ids []int64) error {
	return r.q.MarkPassingsProcessed(ctx, ids)
}

func (r *sqlcRepo) CountPassingsByEvent(ctx context.Context, eventID uuid.UUID) (db.CountPassingsByEventRow, error) {
	return r.q.CountPassingsByEvent(ctx, eventID)
}

// Race Split Times
func (r *sqlcRepo) UpsertRaceSplitTime(ctx context.Context, arg db.UpsertRaceSplitTimeParams) (db.RaceSplitTime, error) {
	return r.q.UpsertRaceSplitTime(ctx, arg)
}

func (r *sqlcRepo) ListSplitsByResult(ctx context.Context, raceResultID uuid.UUID) ([]db.ListSplitsByResultRow, error) {
	return r.q.ListSplitsByResult(ctx, raceResultID)
}

// Tickets
func (r *sqlcRepo) ListTicketsByEvent(ctx context.Context, arg db.ListTicketsByEventParams) ([]db.Ticket, error) {
	return r.q.ListTicketsByEvent(ctx, arg)
}
