package results

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/results/timing"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

func generateIngestionToken() (raw string, hash string, prefix string, err error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", err
	}
	hexPart := hex.EncodeToString(b)
	raw = "ivytt_" + hexPart
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	prefix = raw[:12]
	return raw, hash, prefix, nil
}

// ConfigureTiming sets up the timing provider for an event.
// ConfigureTiming sets up the timing provider for an event.
// Returns the raw ingestion token exactly once for the organizer to configure in external hardware/exporter.
func (s *Service) ConfigureTiming(
	ctx context.Context,
	orgID, eventID, userID uuid.UUID,
	provider, transport, syncMode string,
	policy map[string]any,
	externalRaceID string,
	settings map[string]any,
) (TimingConfigView, string, error) {
	provider = strings.ToUpper(strings.TrimSpace(provider))
	if provider == "" {
		provider = string(timing.ProviderRaceResult)
	}
	transport = strings.ToUpper(strings.TrimSpace(transport))
	if transport == "" {
		transport = string(timing.TransportHTTPPush)
	}
	syncMode = strings.ToUpper(strings.TrimSpace(syncMode))
	if syncMode == "" {
		syncMode = string(timing.SyncModeRawPassings)
	}

	rawToken, tokenHash, tokenPrefix, err := generateIngestionToken()
	if err != nil {
		return TimingConfigView{}, "", fmt.Errorf("failed to generate token: %w", err)
	}

	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		settingsBytes = []byte("{}")
	}

	policyBytes, err := json.Marshal(policy)
	if err != nil || len(policy) == 0 {
		policyBytes = []byte("{}")
	}

	cfg, err := s.repo.UpsertTimingConfig(ctx, db.UpsertTimingConfigParams{
		OrganizationID:       orgID,
		EventID:              eventID,
		Provider:             provider,
		Transport:            transport,
		SyncMode:             syncMode,
		Policy:               policyBytes,
		IngestionTokenHash:   tokenHash,
		IngestionTokenPrefix: tokenPrefix,
		ExternalRaceID:       pgText(externalRaceID),
		IsActive:             true,
		Settings:             settingsBytes,
	})
	if err != nil {
		return TimingConfigView{}, "", err
	}

	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &userID,
			Action:         "timing.configure",
			TargetType:     "event",
			TargetID:       eventID.String(),
			Metadata: map[string]any{
				"provider":    provider,
				"transport":   transport,
				"syncMode":    syncMode,
				"tokenPrefix": tokenPrefix,
			},
		})
	}

	return TimingConfigView{
		ID:                   cfg.ID,
		EventID:              cfg.EventID,
		Provider:             cfg.Provider,
		Transport:            cfg.Transport,
		SyncMode:             cfg.SyncMode,
		Policy:               policy,
		IngestionTokenPrefix: cfg.IngestionTokenPrefix,
		ExternalRaceID:       externalRaceID,
		IsActive:             cfg.IsActive,
		Settings:             settings,
	}, rawToken, nil
}

// GetTimingConfig retrieves the timing integration settings for an event (safe for display, no tokens).
func (s *Service) GetTimingConfig(ctx context.Context, eventID uuid.UUID) (TimingConfigView, error) {
	cfg, err := s.repo.GetTimingConfigByEvent(ctx, eventID)
	if err != nil {
		return TimingConfigView{}, err
	}
	var settings map[string]any
	_ = json.Unmarshal(cfg.Settings, &settings)

	var policy map[string]any
	if len(cfg.Policy) > 0 {
		_ = json.Unmarshal(cfg.Policy, &policy)
	}

	extID := ""
	if cfg.ExternalRaceID.Valid {
		extID = cfg.ExternalRaceID.String
	}

	return TimingConfigView{
		ID:                   cfg.ID,
		EventID:              cfg.EventID,
		Provider:             cfg.Provider,
		Transport:            cfg.Transport,
		SyncMode:             cfg.SyncMode,
		Policy:               policy,
		IngestionTokenPrefix: cfg.IngestionTokenPrefix,
		ExternalRaceID:       extID,
		IsActive:             cfg.IsActive,
		Settings:             settings,
	}, nil
}

// UpsertCheckpoint creates or updates a timing mat/checkpoint for an event.
func (s *Service) UpsertCheckpoint(
	ctx context.Context,
	orgID, eventID, userID uuid.UUID,
	code, name, cpType string,
	orderIndex int32,
	distanceMeters *int32,
	aliases []string,
	providerAliases map[string][]string,
) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return errors.New("checkpoint code required")
	}
	cpType = strings.ToUpper(strings.TrimSpace(cpType))
	if cpType != "START" && cpType != "SPLIT" && cpType != "FINISH" {
		cpType = "SPLIT"
	}

	var dist pgtype.Int4
	if distanceMeters != nil && *distanceMeters > 0 {
		dist = pgtype.Int4{Int32: *distanceMeters, Valid: true}
	}

	var provAliasesBytes []byte
	if len(providerAliases) > 0 {
		provAliasesBytes, _ = json.Marshal(providerAliases)
	}
	if len(provAliasesBytes) == 0 {
		provAliasesBytes = []byte("{}")
	}

	_, err := s.repo.UpsertTimingCheckpoint(ctx, db.UpsertTimingCheckpointParams{
		OrganizationID:  orgID,
		EventID:         eventID,
		Code:            code,
		Name:            name,
		CheckpointType:  cpType,
		OrderIndex:      orderIndex,
		DistanceMeters:  dist,
		Aliases:         aliases,
		ProviderAliases: provAliasesBytes,
	})
	return err
}

// ListCheckpoints returns all checkpoints for an event.
func (s *Service) ListCheckpoints(ctx context.Context, eventID uuid.UUID) ([]CheckpointView, error) {
	rows, err := s.repo.ListTimingCheckpointsByEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]CheckpointView, 0, len(rows))
	for _, r := range rows {
		var d *int
		if r.DistanceMeters.Valid {
			v := int(r.DistanceMeters.Int32)
			d = &v
		}
		var provAliases map[string][]string
		if len(r.ProviderAliases) > 0 {
			_ = json.Unmarshal(r.ProviderAliases, &provAliases)
		}
		out = append(out, CheckpointView{
			ID:              r.ID,
			EventID:         r.EventID,
			Code:            r.Code,
			Name:            r.Name,
			CheckpointType:  r.CheckpointType,
			OrderIndex:      int(r.OrderIndex),
			DistanceMeters:  d,
			Aliases:         r.Aliases,
			ProviderAliases: provAliases,
		})
	}
	return out, nil
}

// CreateWave creates a start wave/corral for an event with official gun start time.
func (s *Service) CreateWave(
	ctx context.Context,
	orgID, eventID, userID uuid.UUID,
	categoryID *uuid.UUID,
	code, name string,
	startAt *time.Time,
	orderIndex int32,
) (WaveView, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return WaveView{}, errors.New("wave code required")
	}

	w, err := s.repo.CreateRaceWave(ctx, db.CreateRaceWaveParams{
		OrganizationID: orgID,
		EventID:        eventID,
		CategoryID:     categoryID,
		Code:           code,
		Name:           name,
		StartAt:        pgTimestamptzPtr(startAt),
		OrderIndex:     orderIndex,
	})
	if err != nil {
		return WaveView{}, err
	}

	var st *time.Time
	if w.StartAt.Valid {
		t := w.StartAt.Time
		st = &t
	}

	return WaveView{
		ID:         w.ID,
		EventID:    w.EventID,
		CategoryID: w.CategoryID,
		Code:       w.Code,
		Name:       w.Name,
		StartAt:    st,
		OrderIndex: int(w.OrderIndex),
	}, nil
}

// ListWaves returns all start waves for an event.
func (s *Service) ListWaves(ctx context.Context, eventID uuid.UUID) ([]WaveView, error) {
	rows, err := s.repo.ListRaceWavesByEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]WaveView, 0, len(rows))
	for _, r := range rows {
		var st *time.Time
		if r.StartAt.Valid {
			t := r.StartAt.Time
			st = &t
		}
		out = append(out, WaveView{
			ID:         r.ID,
			EventID:    r.EventID,
			CategoryID: r.CategoryID,
			Code:       r.Code,
			Name:       r.Name,
			StartAt:    st,
			OrderIndex: int(r.OrderIndex),
		})
	}
	return out, nil
}

// AssignMapping maps a BIB to a transponder/chip code.
func (s *Service) AssignMapping(
	ctx context.Context,
	orgID, eventID, userID uuid.UUID,
	bib, chip string,
	replaceOld bool,
	notes string,
) error {
	bib = strings.TrimSpace(bib)
	chip = strings.ToUpper(strings.TrimSpace(chip))
	if bib == "" || chip == "" {
		return errors.New("bib and chip code required")
	}

	if replaceOld {
		_ = s.repo.DeactivateBibMappings(ctx, db.DeactivateBibMappingsParams{
			EventID:   eventID,
			BibNumber: bib,
		})
		_ = s.repo.DeactivateChipMappings(ctx, db.DeactivateChipMappingsParams{
			EventID:         eventID,
			TransponderCode: chip,
		})
	}

	_, err := s.repo.InsertBibTransponderMapping(ctx, db.InsertBibTransponderMappingParams{
		OrganizationID:  orgID,
		EventID:         eventID,
		BibNumber:       bib,
		TransponderCode: chip,
		IsActive:        true,
		Status:          "ACTIVE",
		Notes:           pgText(notes),
	})
	return err
}

// ImportMappingsCSV imports BIB to Chip mappings from CSV.
func (s *Service) ImportMappingsCSV(
	ctx context.Context,
	orgID, eventID, userID uuid.UUID,
	body io.Reader,
	replaceOld bool,
) (int, error) {
	r := csv.NewReader(body)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err != nil {
		return 0, err
	}

	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[strings.ToLower(strings.TrimSpace(col))] = i
	}

	imported := 0
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || len(rec) < 2 {
			continue
		}

		get := func(names ...string) string {
			for _, name := range names {
				if idx, ok := colIdx[name]; ok && idx < len(rec) {
					return strings.TrimSpace(rec[idx])
				}
			}
			return ""
		}

		bib := get("bib", "bib_number", "no_dada")
		chip := get("chip", "transponder", "chip_code", "rfid")
		if bib == "" && len(rec) >= 1 {
			bib = strings.TrimSpace(rec[0])
		}
		if chip == "" && len(rec) >= 2 {
			chip = strings.TrimSpace(rec[1])
		}

		if bib == "" || chip == "" {
			continue
		}

		notes := get("notes", "keterangan")
		if err := s.AssignMapping(ctx, orgID, eventID, userID, bib, chip, replaceOld, notes); err == nil {
			imported++
		}
	}

	return imported, nil
}

// ListMappings returns all transponder mappings for an event.
func (s *Service) ListMappings(ctx context.Context, eventID uuid.UUID) ([]MappingView, error) {
	rows, err := s.repo.ListMappingsByEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]MappingView, 0, len(rows))
	for _, r := range rows {
		notes := ""
		if r.Notes.Valid {
			notes = r.Notes.String
		}
		out = append(out, MappingView{
			ID:              r.ID,
			EventID:         r.EventID,
			BibNumber:       r.BibNumber,
			TransponderCode: r.TransponderCode,
			IsActive:        r.IsActive,
			Status:          r.Status,
			Notes:           notes,
		})
	}
	return out, nil
}

// ValidateIngestionToken checks whether an incoming token matches the event's configured ingestion token hash.
func (s *Service) ValidateIngestionToken(ctx context.Context, eventID uuid.UUID, rawToken string) bool {
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])
	_, err := s.repo.GetTimingConfigByTokenHash(ctx, db.GetTimingConfigByTokenHashParams{
		EventID:            eventID,
		IngestionTokenHash: hash,
	})
	return err == nil
}

// IngestPassings accepts raw telemetry bytes from a timing provider (RACE RESULT, Vendor API, CSV)
// and persists observations into timing_passings with strict idempotency.
func (s *Service) IngestPassings(
	ctx context.Context,
	orgID, eventID uuid.UUID,
	defaultCheckpoint string,
	payload []byte,
) (int, error) {
	cfg, err := s.repo.GetTimingConfigByEvent(ctx, eventID)
	if err != nil {
		return 0, fmt.Errorf("timing not configured for event: %w", err)
	}

	reg := timing.DefaultRegistry()
	parser, ok := reg.GetPassingParser(timing.ProviderType(cfg.Provider))
	if !ok {
		return 0, fmt.Errorf("unsupported provider %s for passing stream", cfg.Provider)
	}

	observations, err := parser.ParsePassings(ctx, defaultCheckpoint, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to parse passings: %w", err)
	}

	ingested := 0
	for _, obs := range observations {
		metaBytes, _ := json.Marshal(obs.Metadata)

		var rawChipText pgtype.Text
		if obs.RawChipCode != "" {
			rawChipText = pgText(obs.RawChipCode)
		} else {
			rawChipText = pgText(obs.ChipCode)
		}

		var res db.TimingPassing
		obsTime := pgtype.Timestamptz{Time: obs.ObservedAt, Valid: true}
		if obs.ExternalReadID != "" {
			res, err = s.repo.InsertTimingPassing(ctx, db.InsertTimingPassingParams{
				OrganizationID:  orgID,
				EventID:         eventID,
				CheckpointCode:  obs.CheckpointCode,
				Provider:        cfg.Provider,
				ExternalReadID:  pgText(obs.ExternalReadID),
				ChipCode:        obs.ChipCode,
				RawChipCode:     rawChipText,
				BibNumber:       pgTextPtr(obs.BibNumber),
				ObservedAt:      obsTime,
				RawPayload:      pgText(obs.RawPayload),
				Metadata:        metaBytes,
			})
		} else {
			res, err = s.repo.InsertTimingPassingFallback(ctx, db.InsertTimingPassingFallbackParams{
				OrganizationID:  orgID,
				EventID:         eventID,
				CheckpointCode:  obs.CheckpointCode,
				Provider:        cfg.Provider,
				ExternalReadID:  pgText(obs.ExternalReadID),
				ChipCode:        obs.ChipCode,
				RawChipCode:     rawChipText,
				BibNumber:       pgTextPtr(obs.BibNumber),
				ObservedAt:      obsTime,
				RawPayload:      pgText(obs.RawPayload),
				Metadata:        metaBytes,
			})
		}

		if err == nil && res.ID > 0 {
			ingested++
		}
	}

	return ingested, nil
}

// ProcessTiming runs the vendor-agnostic scoring pipeline for unprocessed passings.
func (s *Service) ProcessTiming(ctx context.Context, orgID, eventID, userID uuid.UUID) (TimingProcessSummary, error) {
	// 1. Fetch checkpoints
	cps, err := s.repo.ListTimingCheckpointsByEvent(ctx, eventID)
	if err != nil {
		return TimingProcessSummary{}, err
	}
	cpDefs := make([]timing.CheckpointDef, 0, len(cps))
	cpIDByCode := make(map[string]uuid.UUID)
	for _, c := range cps {
		var dist *int
		if c.DistanceMeters.Valid {
			v := int(c.DistanceMeters.Int32)
			dist = &v
		}
		var provAliases map[string][]string
		if len(c.ProviderAliases) > 0 {
			_ = json.Unmarshal(c.ProviderAliases, &provAliases)
		}
		cpDefs = append(cpDefs, timing.CheckpointDef{
			ID:              c.ID,
			Code:            c.Code,
			Name:            c.Name,
			Type:            c.CheckpointType,
			OrderIndex:      int(c.OrderIndex),
			DistanceMeters:  dist,
			Aliases:         c.Aliases,
			ProviderAliases: provAliases,
		})
		cpIDByCode[c.Code] = c.ID
	}

	// 2. Fetch waves to resolve gun start times
	waves, err := s.repo.ListRaceWavesByEvent(ctx, eventID)
	if err != nil {
		return TimingProcessSummary{}, err
	}
	waveStartMap := make(map[uuid.UUID]*time.Time)
	for _, w := range waves {
		if w.StartAt.Valid {
			t := w.StartAt.Time
			waveStartMap[w.ID] = &t
		}
	}

	// 3. Fetch tickets (source of truth for participants and BIB numbers)
	tickets, err := s.repo.ListTicketsByEvent(ctx, db.ListTicketsByEventParams{
		OrganizationID: orgID,
		EventID:        eventID,
	})
	if err != nil {
		return TimingProcessSummary{}, err
	}
	partMap := make(map[string]timing.ParticipantDef)
	for _, t := range tickets {
		if !t.BibNumber.Valid || t.BibNumber.String == "" {
			continue
		}
		bib := t.BibNumber.String
		tid := t.ID
		cid := t.CategoryID
		var wStart *time.Time
		if t.WaveID != nil {
			wStart = waveStartMap[*t.WaveID]
		}
		partMap[bib] = timing.ParticipantDef{
			TicketID:        &tid,
			CategoryID:      &cid,
			BibNumber:       bib,
			ParticipantName: t.HolderName,
			WaveID:          t.WaveID,
			WaveStartAt:     wStart,
		}
	}

	// 4. Fetch active chip to BIB mappings
	mappings, err := s.repo.ListMappingsByEvent(ctx, eventID)
	if err != nil {
		return TimingProcessSummary{}, err
	}
	chipToBib := make(map[string]string)
	for _, m := range mappings {
		if m.IsActive {
			chipToBib[m.TransponderCode] = m.BibNumber
		}
	}

	// 5. Fetch unprocessed passings
	passings, err := s.repo.ListUnprocessedPassings(ctx, db.ListUnprocessedPassingsParams{
		EventID: eventID,
		Limit:   5000,
	})
	if err != nil {
		return TimingProcessSummary{}, err
	}
	if len(passings) == 0 {
		return TimingProcessSummary{Ranked: true}, nil
	}

	passingItems := make([]timing.PassingItem, 0, len(passings))
	for _, p := range passings {
		bib := ""
		if p.BibNumber.Valid {
			bib = p.BibNumber.String
		}
		passingItems = append(passingItems, timing.PassingItem{
			ID:             p.ID,
			CheckpointCode: p.CheckpointCode,
			ChipCode:       p.ChipCode,
			BibNumber:      bib,
			ObservedAt:     p.ObservedAt.Time,
		})
	}

	// 6. Run processor engine
	proc := timing.NewProcessor(15 * time.Second)
	if cfg, err := s.repo.GetTimingConfigByEvent(ctx, eventID); err == nil && len(cfg.Policy) > 0 {
		var policy timing.EventPolicy
		if err := json.Unmarshal(cfg.Policy, &policy); err == nil {
			proc.WithPolicy(policy)
		}
	}
	outputs := proc.ProcessEventPassings(ctx, cpDefs, partMap, chipToBib, passingItems)

	allProcessedIDs := make([]int64, 0, len(passings))
	updatedCount := 0

	for _, out := range outputs {
		allProcessedIDs = append(allProcessedIDs, out.ProcessedIDs...)

		var chipMs, gunMs pgtype.Int8
		if out.ChipTimeMs != nil {
			chipMs = pgtype.Int8{Int64: *out.ChipTimeMs, Valid: true}
		}
		if out.GunTimeMs != nil {
			gunMs = pgtype.Int8{Int64: *out.GunTimeMs, Valid: true}
		}

		res, err := s.repo.UpsertRaceResult(ctx, db.UpsertRaceResultParams{
			OrganizationID:  orgID,
			EventID:         eventID,
			CategoryID:      out.CategoryID,
			TicketID:        out.TicketID,
			BibNumber:       out.BibNumber,
			ParticipantName: out.ParticipantName,
			Gender:          pgText(out.Gender),
			AgeGroup:        pgText(out.AgeGroup),
			Status:          out.Status,
			ChipTimeMs:      chipMs,
			GunTimeMs:       gunMs,
			Source:          SourceTimingAPI,
			FinishedAt:      pgTimestamptzPtr(out.FinishedAt),
		})
		if err != nil {
			s.log.Error("upsert timing race result failed", "bib", out.BibNumber, "error", err)
			continue
		}
		updatedCount++

		// Save intermediate splits
		for _, sp := range out.Splits {
			var pace pgtype.Int8
			if sp.SplitPaceMsKm != nil {
				pace = pgtype.Int8{Int64: *sp.SplitPaceMsKm, Valid: true}
			}
			var passID pgtype.Int8
			if sp.PassingID != nil {
				passID = pgtype.Int8{Int64: *sp.PassingID, Valid: true}
			}

			_, _ = s.repo.UpsertRaceSplitTime(ctx, db.UpsertRaceSplitTimeParams{
				RaceResultID:   res.ID,
				CheckpointID:   sp.CheckpointID,
				PassingID:      passID,
				SplitTimeMs:    sp.SplitTimeMs,
				SplitPaceMsKm:  pace,
				PassingTime:    pgtype.Timestamptz{Time: sp.PassingTime, Valid: true},
				OrderIndex:     int32(sp.OrderIndex),
			})
		}
	}

	// Mark all evaluated passings as processed
	if len(allProcessedIDs) > 0 {
		_ = s.repo.MarkPassingsProcessed(ctx, allProcessedIDs)
	}

	// Recompute all ranks
	if updatedCount > 0 {
		_ = s.recomputeRanks(ctx, eventID)
	}

	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &userID,
			Action:         "timing.process",
			TargetType:     "event",
			TargetID:       eventID.String(),
			Metadata: map[string]any{
				"passings": len(allProcessedIDs),
				"updated":  updatedCount,
			},
		})
	}

	return TimingProcessSummary{
		PassingsEvaluated: len(allProcessedIDs),
		ResultsUpdated:    updatedCount,
		Ranked:            true,
	}, nil
}

// GetPassingStats returns passing telemetry counters for an event.
func (s *Service) GetPassingStats(ctx context.Context, eventID uuid.UUID) (PassingStatsView, error) {
	row, err := s.repo.CountPassingsByEvent(ctx, eventID)
	if err != nil {
		return PassingStatsView{}, err
	}
	return PassingStatsView{
		Total:          row.Total,
		ProcessedCount: row.ProcessedCount,
		PendingCount:   row.Total - row.ProcessedCount,
	}, nil
}

func pgTextPtr(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}
