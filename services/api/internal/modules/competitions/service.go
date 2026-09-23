package competitions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/results"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

var (
	ErrEventNotFound         = errors.New("event tidak ditemukan")
	ErrEventForbidden        = errors.New("akses event ditolak")
	ErrSportNotFound         = errors.New("olahraga tidak ditemukan")
	ErrDisciplineNotFound    = errors.New("disiplin olahraga tidak ditemukan")
	ErrStageNotFound         = errors.New("babak kompetisi tidak ditemukan")
	ErrMatchNotFound         = errors.New("pertandingan tidak ditemukan")
	ErrTeamNotFound          = errors.New("tim tidak ditemukan")
	ErrEntryNotFound         = errors.New("peserta kompetisi tidak ditemukan")
	ErrInvalidStatus         = errors.New("status tidak valid")
	ErrInvalidScore          = errors.New("skor tidak valid")
	ErrInvalidRosterMember   = errors.New("anggota roster tidak valid")
	ErrNotPelotonStage       = errors.New("babak bukan merupakan babak balap sepeda / peloton")
	ErrNoFinishRecords       = errors.New("tidak ada data catatan finis untuk babak ini")
	ErrDownstreamMatchLocked = errors.New("pertandingan babak selanjutnya sudah berlangsung atau selesai")
)

// Service coordinates domain operations for generic multi-sport competitions.
type Service struct {
	repo  *Repository
	audit *audit.Logger
}

// NewService constructs a multi-sport competition service.
func NewService(repo *Repository, auditLogger *audit.Logger) *Service {
	return &Service{
		repo:  repo,
		audit: auditLogger,
	}
}

// assertEvent confirms the event exists and belongs to orgID (tenant guard).
func (s *Service) assertEvent(ctx context.Context, orgID, eventID uuid.UUID) error {
	e, err := s.repo.GetEventByID(ctx, eventID)
	if err != nil {
		return ErrEventNotFound
	}
	if e.OrganizationID != orgID {
		return ErrEventForbidden
	}
	return nil
}

// ListSports returns all supported master sports.
func (s *Service) ListSports(ctx context.Context) ([]SportView, error) {
	rows, err := s.repo.ListSports(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SportView, len(rows))
	for i, r := range rows {
		out[i] = SportView{
			ID:            r.ID,
			Name:          r.Name,
			GoverningBody: r.GoverningBody,
			CreatedAt:     pgTimeToTime(r.CreatedAt),
		}
	}
	return out, nil
}

// ListDisciplines returns all disciplines for a given sport.
func (s *Service) ListDisciplines(ctx context.Context, sportID string) ([]SportDisciplineView, error) {
	rows, err := s.repo.ListDisciplinesBySport(ctx, sportID)
	if err != nil {
		return nil, err
	}
	out := make([]SportDisciplineView, len(rows))
	for i, r := range rows {
		var cfg map[string]any
		_ = json.Unmarshal(r.DefaultConfig, &cfg)
		out[i] = SportDisciplineView{
			ID:            r.ID,
			SportID:       r.SportID,
			Name:          r.Name,
			DefaultConfig: cfg,
			CreatedAt:     pgTimeToTime(r.CreatedAt),
		}
	}
	return out, nil
}

// ConfigureCompetition sets up the capability flags and rules for an event or category.
func (s *Service) ConfigureCompetition(ctx context.Context, orgID, eventID uuid.UUID, catID *uuid.UUID, cfg CompetitionConfigView) (CompetitionConfigView, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return CompetitionConfigView{}, err
	}

	rulesBytes := toJSONBytes(cfg.RulesConfig)

	rankingStrat := string(cfg.RankingStrategy)
	if rankingStrat == "" {
		rankingStrat = string(StrategyTimeAsc)
	}

	precision := string(cfg.TimePrecision)
	if precision == "" {
		precision = string(PrecisionWholeSeconds)
	}

	row, err := s.repo.UpsertConfig(ctx, db.UpsertCompetitionConfigParams{
		OrganizationID:  orgID,
		EventID:         eventID,
		CategoryID:      catID,
		SportID:         cfg.SportID,
		DisciplineID:    cfg.DisciplineID,
		HasTiming:       cfg.HasTiming,
		HasScoring:      cfg.HasScoring,
		HasMatches:      cfg.HasMatches,
		HasHeatsLanes:   cfg.HasHeatsLanes,
		HasStages:       cfg.HasStages,
		TeamBased:       cfg.TeamBased,
		MinTeamSize:     int32(cfg.MinTeamSize),
		MaxTeamSize:     int32(cfg.MaxTeamSize),
		RankingStrategy: rankingStrat,
		TimePrecision:   precision,
		RulesConfig:     rulesBytes,
	})
	if err != nil {
		return CompetitionConfigView{}, err
	}

	var rulesOut map[string]any
	_ = json.Unmarshal(row.RulesConfig, &rulesOut)

	return CompetitionConfigView{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		EventID:         row.EventID,
		CategoryID:      row.CategoryID,
		SportID:         row.SportID,
		DisciplineID:    row.DisciplineID,
		HasTiming:       row.HasTiming,
		HasScoring:      row.HasScoring,
		HasMatches:      row.HasMatches,
		HasHeatsLanes:   row.HasHeatsLanes,
		HasStages:       row.HasStages,
		TeamBased:       row.TeamBased,
		MinTeamSize:     int(row.MinTeamSize),
		MaxTeamSize:     int(row.MaxTeamSize),
		RankingStrategy: RankingStrategy(row.RankingStrategy),
		TimePrecision:   TimePrecision(row.TimePrecision),
		RulesConfig:     rulesOut,
		CreatedAt:       pgTimeToTime(row.CreatedAt),
		UpdatedAt:       pgTimeToTime(row.UpdatedAt),
	}, nil
}

// GetCompetitionConfig returns the event-level competition capability settings.
func (s *Service) GetCompetitionConfig(ctx context.Context, eventID uuid.UUID) (CompetitionConfigView, error) {
	row, err := s.repo.GetConfigByEvent(ctx, eventID)
	if err != nil {
		// If not configured yet, return reasonable defaults based on running
		return CompetitionConfigView{
			EventID:         eventID,
			SportID:         "running",
			DisciplineID:    "road_running",
			HasTiming:       true,
			HasScoring:      false,
			HasMatches:      false,
			TeamBased:       false,
			RankingStrategy: StrategyTimeAsc,
			TimePrecision:   PrecisionCeilingSec,
			RulesConfig:     map[string]any{"source": "World Athletics"},
		}, nil
	}

	var rulesOut map[string]any
	_ = json.Unmarshal(row.RulesConfig, &rulesOut)

	return CompetitionConfigView{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		EventID:         row.EventID,
		CategoryID:      row.CategoryID,
		SportID:         row.SportID,
		DisciplineID:    row.DisciplineID,
		HasTiming:       row.HasTiming,
		HasScoring:      row.HasScoring,
		HasMatches:      row.HasMatches,
		HasHeatsLanes:   row.HasHeatsLanes,
		HasStages:       row.HasStages,
		TeamBased:       row.TeamBased,
		MinTeamSize:     int(row.MinTeamSize),
		MaxTeamSize:     int(row.MaxTeamSize),
		RankingStrategy: RankingStrategy(row.RankingStrategy),
		TimePrecision:   TimePrecision(row.TimePrecision),
		RulesConfig:     rulesOut,
		CreatedAt:       pgTimeToTime(row.CreatedAt),
		UpdatedAt:       pgTimeToTime(row.UpdatedAt),
	}, nil
}

// CreateTeam registers a team / club in a competition.
func (s *Service) CreateTeam(ctx context.Context, orgID, eventID uuid.UUID, catID *uuid.UUID, name string, shortName string, managerID *uuid.UUID, logoURL string, seedNum *int) (TeamView, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return TeamView{}, err
	}

	var seedInt4 pgtype.Int4
	if seedNum != nil {
		seedInt4 = pgtype.Int4{Int32: int32(*seedNum), Valid: true}
	}

	row, err := s.repo.CreateTeam(ctx, db.CreateTeamParams{
		OrganizationID: orgID,
		EventID:        eventID,
		CategoryID:     catID,
		Name:           name,
		ShortName:      pgText(shortName),
		ManagerUserID:  managerID,
		LogoUrl:        pgText(logoURL),
		SeedNumber:     seedInt4,
		Status:         "ACTIVE",
	})
	if err != nil {
		return TeamView{}, err
	}

	var sNum *int
	if row.SeedNumber.Valid {
		n := int(row.SeedNumber.Int32)
		sNum = &n
	}

	return TeamView{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		EventID:        row.EventID,
		CategoryID:     row.CategoryID,
		Name:           row.Name,
		ShortName:      pgTextValue(row.ShortName),
		ManagerUserID:  row.ManagerUserID,
		LogoURL:        pgTextValue(row.LogoUrl),
		SeedNumber:     sNum,
		Status:         row.Status,
		CreatedAt:      pgTimeToTime(row.CreatedAt),
		UpdatedAt:      pgTimeToTime(row.UpdatedAt),
	}, nil
}

// AddRosterMember adds an athlete or ticket holder to a team roster.
func (s *Service) AddRosterMember(ctx context.Context, teamID uuid.UUID, ticketID *uuid.UUID, userID *uuid.UUID, playerName string, jerseyNumber *int, position string, isCaptain bool) (TeamRosterView, error) {
	if playerName == "" {
		return TeamRosterView{}, ErrInvalidRosterMember
	}

	var jerseyInt4 pgtype.Int4
	if jerseyNumber != nil {
		jerseyInt4 = pgtype.Int4{Int32: int32(*jerseyNumber), Valid: true}
	}

	row, err := s.repo.CreateTeamRosterMember(ctx, db.CreateTeamRosterMemberParams{
		TeamID:       teamID,
		TicketID:     ticketID,
		UserID:       userID,
		PlayerName:   playerName,
		JerseyNumber: jerseyInt4,
		Position:     pgText(position),
		IsCaptain:    isCaptain,
		Status:       "ACTIVE",
	})
	if err != nil {
		return TeamRosterView{}, err
	}

	var jNum *int
	if row.JerseyNumber.Valid {
		n := int(row.JerseyNumber.Int32)
		jNum = &n
	}

	return TeamRosterView{
		ID:           row.ID,
		TeamID:       row.TeamID,
		TicketID:     row.TicketID,
		UserID:       row.UserID,
		PlayerName:   row.PlayerName,
		JerseyNumber: jNum,
		Position:     pgTextValue(row.Position),
		IsCaptain:    row.IsCaptain,
		Status:       row.Status,
		CreatedAt:    pgTimeToTime(row.CreatedAt),
		UpdatedAt:    pgTimeToTime(row.UpdatedAt),
	}, nil
}

// ListTeams returns all teams for an event.
func (s *Service) ListTeams(ctx context.Context, eventID uuid.UUID) ([]TeamView, error) {
	rows, err := s.repo.ListTeamsByEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]TeamView, len(rows))
	for i, r := range rows {
		var sNum *int
		if r.SeedNumber.Valid {
			n := int(r.SeedNumber.Int32)
			sNum = &n
		}
		out[i] = TeamView{
			ID:             r.ID,
			OrganizationID: r.OrganizationID,
			EventID:        r.EventID,
			CategoryID:     r.CategoryID,
			Name:           r.Name,
			ShortName:      pgTextValue(r.ShortName),
			ManagerUserID:  r.ManagerUserID,
			LogoURL:        pgTextValue(r.LogoUrl),
			SeedNumber:     sNum,
			Status:         r.Status,
			CreatedAt:      pgTimeToTime(r.CreatedAt),
			UpdatedAt:      pgTimeToTime(r.UpdatedAt),
		}
	}
	return out, nil
}

// ListTeamRoster returns all roster members of a team.
func (s *Service) ListTeamRoster(ctx context.Context, teamID uuid.UUID) ([]TeamRosterView, error) {
	rows, err := s.repo.ListTeamRoster(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]TeamRosterView, len(rows))
	for i, r := range rows {
		var jNum *int
		if r.JerseyNumber.Valid {
			n := int(r.JerseyNumber.Int32)
			jNum = &n
		}
		out[i] = TeamRosterView{
			ID:           r.ID,
			TeamID:       r.TeamID,
			TicketID:     nil, // omit internal ticket ID from public view
			UserID:       nil, // omit internal user ID from public view
			PlayerName:   r.PlayerName,
			JerseyNumber: jNum,
			Position:     pgTextValue(r.Position),
			IsCaptain:    r.IsCaptain,
			Status:       r.Status,
			CreatedAt:    pgTimeToTime(r.CreatedAt),
			UpdatedAt:    pgTimeToTime(r.UpdatedAt),
		}
	}
	return out, nil
}

// RegisterParticipantEntry bridges an individual ticket or a team into the competition registry.
func (s *Service) RegisterParticipantEntry(
	ctx context.Context,
	orgID, eventID, catID uuid.UUID,
	ticketID, teamID *uuid.UUID,
	entryType EntryType,
	displayName, identifierCode string,
	seedNum, laneNum, heatNum *int,
) (ParticipantEntryView, error) {
	var sInt4, lInt4, hInt4 pgtype.Int4
	if seedNum != nil {
		sInt4 = pgtype.Int4{Int32: int32(*seedNum), Valid: true}
	}
	if laneNum != nil {
		lInt4 = pgtype.Int4{Int32: int32(*laneNum), Valid: true}
	}
	if heatNum != nil {
		hInt4 = pgtype.Int4{Int32: int32(*heatNum), Valid: true}
	}

	var catUUID *uuid.UUID
	if catID != uuid.Nil {
		catUUID = &catID
	}
	row, err := s.repo.CreateParticipantEntry(ctx, db.CreateParticipantEntryParams{
		OrganizationID: orgID,
		EventID:        eventID,
		CategoryID:     catUUID,
		TicketID:       ticketID,
		TeamID:         teamID,
		EntryType:      string(entryType),
		DisplayName:    displayName,
		IdentifierCode: pgText(identifierCode),
		SeedNumber:     sInt4,
		LaneNumber:     lInt4,
		HeatNumber:     hInt4,
		Status:         string(StatusRegistered),
	})
	if err != nil {
		return ParticipantEntryView{}, err
	}

	return toEntryView(row), nil
}

// ListParticipantEntries returns all entries for an event.
func (s *Service) ListParticipantEntries(ctx context.Context, eventID uuid.UUID) ([]ParticipantEntryView, error) {
	rows, err := s.repo.ListParticipantEntriesByEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]ParticipantEntryView, len(rows))
	for i, r := range rows {
		out[i] = toEntryView(r)
	}
	return out, nil
}

// CreateStage creates a competition stage and optionally generates initial fixtures.
func (s *Service) CreateStage(
	ctx context.Context,
	orgID, eventID uuid.UUID,
	catID *uuid.UUID,
	name string,
	stageType StageType,
	sequenceOrder int,
	autoFixtures bool,
	entryIDs []uuid.UUID,
) (CompetitionStageView, []CompetitionMatchView, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return CompetitionStageView{}, nil, err
	}

	var stageView CompetitionStageView
	var generatedMatches []CompetitionMatchView

	err := s.repo.ExecTx(ctx, func(txRepo *Repository) error {
		row, err := txRepo.CreateStage(ctx, db.CreateCompetitionStageParams{
			OrganizationID: orgID,
			EventID:        eventID,
			CategoryID:     catID,
			Name:           name,
			StageType:      string(stageType),
			SequenceOrder:  int32(sequenceOrder),
			Status:         "SCHEDULED",
			StageConfig:    []byte("{}"),
		})
		if err != nil {
			return err
		}

		stageView = CompetitionStageView{
			ID:             row.ID,
			OrganizationID: row.OrganizationID,
			EventID:        row.EventID,
			CategoryID:     row.CategoryID,
			Name:           row.Name,
			StageType:      StageType(row.StageType),
			SequenceOrder:  int(row.SequenceOrder),
			Status:         row.Status,
			CreatedAt:      pgTimeToTime(row.CreatedAt),
			UpdatedAt:      pgTimeToTime(row.UpdatedAt),
		}

		if autoFixtures && len(entryIDs) >= 2 {
			var drafts []FixtureDraft
			switch stageType {
			case StageGroupRoundRobin:
				drafts = GenerateRoundRobin(entryIDs)
			case StageSingleElimination:
				drafts = GenerateSingleElimination(entryIDs)
			}

			for _, d := range drafts {
				matchStatus := "SCHEDULED"
				if d.MatchStatus != "" {
					matchStatus = d.MatchStatus
				}
				mRow, err := txRepo.CreateMatch(ctx, db.CreateCompetitionMatchParams{
					OrganizationID: orgID,
					EventID:        eventID,
					StageID:        row.ID,
					RoundNumber:    int32(d.RoundNumber),
					MatchNumber:    int32(d.MatchNumber),
					HomeEntryID:    d.HomeEntryID,
					AwayEntryID:    d.AwayEntryID,
					VenueCourtName: pgText(d.VenueCourtName),
					HomeScore:      0,
					AwayScore:      0,
					MatchStatus:    matchStatus,
					WinnerID:       d.WinnerID,
					ScoreDetails:   []byte("[]"),
				})
				if err != nil {
					return err
				}
				generatedMatches = append(generatedMatches, toMatchView(mRow))
			}
		}
		return nil
	})

	if err != nil {
		return CompetitionStageView{}, nil, err
	}

	return stageView, generatedMatches, nil
}

// ListStages returns all stages for an event.
func (s *Service) ListStages(ctx context.Context, eventID uuid.UUID) ([]CompetitionStageView, error) {
	rows, err := s.repo.ListStagesByEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]CompetitionStageView, len(rows))
	for i, r := range rows {
		out[i] = CompetitionStageView{
			ID:             r.ID,
			OrganizationID: r.OrganizationID,
			EventID:        r.EventID,
			CategoryID:     r.CategoryID,
			Name:           r.Name,
			StageType:      StageType(r.StageType),
			SequenceOrder:  int(r.SequenceOrder),
			Status:         r.Status,
			CreatedAt:      pgTimeToTime(r.CreatedAt),
			UpdatedAt:      pgTimeToTime(r.UpdatedAt),
		}
	}
	return out, nil
}

// ListMatches returns all match fixtures for a stage after verifying event ownership.
func (s *Service) ListMatches(ctx context.Context, eventID, stageID uuid.UUID) ([]CompetitionMatchView, error) {
	stage, err := s.repo.GetStageByID(ctx, stageID)
	if err != nil || stage.EventID != eventID {
		return nil, ErrStageNotFound
	}

	rows, err := s.repo.ListMatchesByStage(ctx, stageID)
	if err != nil {
		return nil, err
	}
	out := make([]CompetitionMatchView, len(rows))
	for i, r := range rows {
		out[i] = toMatchView(r)
	}
	return out, nil
}

// RecordMatchScore updates the official score of a match and advances single elimination winners atomically.
func (s *Service) RecordMatchScore(
	ctx context.Context,
	orgID, eventID, matchID uuid.UUID,
	homeScore, awayScore int,
	status string,
	winnerID *uuid.UUID,
	details []MatchScoreDetail,
) (CompetitionMatchView, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return CompetitionMatchView{}, err
	}

	if homeScore < 0 || awayScore < 0 {
		return CompetitionMatchView{}, ErrInvalidScore
	}

	// Strictly validate status: NO empty string fallback
	status = strings.TrimSpace(status)
	switch status {
	case "SCHEDULED", "LIVE", "COMPLETED", "SUSPENDED", "CANCELLED", "FORFEIT", "WALKOVER", "BYE":
		// valid
	default:
		return CompetitionMatchView{}, ErrInvalidStatus
	}

	detailsBytes := toJSONBytes(details)

	var updatedMatch db.CompetitionMatch
	err := s.repo.ExecTx(ctx, func(txRepo *Repository) error {
		row, err := txRepo.UpdateMatchScoreScoped(ctx, db.UpdateCompetitionMatchScoreScopedParams{
			ID:             matchID,
			HomeScore:      int32(homeScore),
			AwayScore:      int32(awayScore),
			MatchStatus:    status,
			WinnerID:       winnerID,
			ScoreDetails:   detailsBytes,
			EventID:        eventID,
			OrganizationID: orgID,
		})
		if err != nil {
			return ErrMatchNotFound
		}
		updatedMatch = row

		// Single elimination bracket advancement and downstream lock protection
		stage, stageErr := txRepo.GetStageByID(ctx, row.StageID)
		if stageErr == nil && stage.StageType == string(StageSingleElimination) {
			nextRound := int(row.RoundNumber) + 1
			nextMatch := (int(row.MatchNumber) + 1) / 2

			// Check if downstream match exists in the bracket
			downstream, downstreamErr := txRepo.GetMatchByRoundAndNumber(ctx, row.StageID, nextRound, nextMatch)
			if downstreamErr != nil && !errors.Is(downstreamErr, pgx.ErrNoRows) {
				return downstreamErr
			}
			hasNextRound := downstreamErr == nil

			if hasNextRound {
				// Reject propagation/overwrites if downstream match is not SCHEDULED
				// (e.g. LIVE, COMPLETED, FORFEIT, WALKOVER, BYE, SUSPENDED, CANCELLED)
				if downstream.MatchStatus != "SCHEDULED" {
					return ErrDownstreamMatchLocked
				}
			}

			// Single elimination bracket advancement for COMPLETED, FORFEIT, WALKOVER, or BYE
			if status == "COMPLETED" || status == "FORFEIT" || status == "WALKOVER" || status == "BYE" {
				var winEntry *uuid.UUID
				if winnerID != nil {
					winEntry = winnerID
				} else if row.HomeEntryID != nil && row.AwayEntryID == nil {
					winEntry = row.HomeEntryID
				} else if homeScore > awayScore && row.HomeEntryID != nil {
					winEntry = row.HomeEntryID
				} else if awayScore > homeScore && row.AwayEntryID != nil {
					winEntry = row.AwayEntryID
				}

				if winEntry != nil && hasNextRound {
					var homeNext, awayNext *uuid.UUID
					if int(row.MatchNumber)%2 == 1 {
						homeNext = winEntry
					} else {
						awayNext = winEntry
					}
					rowsAffected, err := txRepo.UpdateMatchParticipantsScheduled(ctx, row.StageID, nextRound, nextMatch, homeNext, awayNext)
					if err != nil {
						return err
					}
					if rowsAffected == 0 {
						return ErrDownstreamMatchLocked
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		return CompetitionMatchView{}, err
	}

	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			Action:         "competitions.match.score_update",
			TargetType:     "match",
			TargetID:       matchID.String(),
			Metadata: map[string]any{
				"homeScore": homeScore,
				"awayScore": awayScore,
				"status":    status,
			},
		})
	}

	return toMatchView(updatedMatch), nil
}

// RecordObservation logs an in-competition event (goal, card, split time, lap) and stores traceably.
func (s *Service) RecordObservation(
	ctx context.Context,
	orgID, eventID uuid.UUID,
	stageID, matchID, entryID *uuid.UUID,
	obsType ObservationType,
	source string,
	numVal *float64,
	textVal string,
	observedAtMs int64,
	metadata map[string]any,
) (CompetitionObservationView, error) {
	var numNumeric pgtype.Numeric
	if numVal != nil {
		_ = numNumeric.Scan(fmt.Sprintf("%.4f", *numVal))
	}

	metaBytes := toJSONBytes(metadata)
	if source == "" {
		source = "MANUAL"
	}

	row, err := s.repo.CreateObservation(ctx, db.CreateCompetitionObservationParams{
		OrganizationID:  orgID,
		EventID:         eventID,
		StageID:         stageID,
		MatchID:         matchID,
		EntryID:         entryID,
		ObservationType: string(obsType),
		Source:          source,
		NumericValue:    numNumeric,
		TextValue:       pgText(textVal),
		ObservedAtMs:    observedAtMs,
		Metadata:        metaBytes,
	})
	if err != nil {
		return CompetitionObservationView{}, err
	}

	return toObservationView(row), nil
}

// GetStageStandings computes and returns the official standings for a stage after verifying event ownership.
func (s *Service) GetStageStandings(ctx context.Context, eventID, stageID uuid.UUID) ([]GenericStandingsRow, error) {
	stage, err := s.repo.GetStageByID(ctx, stageID)
	if err != nil || stage.EventID != eventID {
		return nil, ErrStageNotFound
	}

	entries, err := s.ListParticipantEntries(ctx, eventID)
	if err != nil {
		return nil, err
	}

	// For group round robin: calculate from matches based on sport configuration
	if stage.StageType == string(StageGroupRoundRobin) {
		matches, err := s.ListMatches(ctx, eventID, stageID)
		if err != nil {
			return nil, err
		}

		cfg, _ := s.GetCompetitionConfig(ctx, eventID)
		sportID := strings.ToLower(cfg.SportID)

		if sportID == "badminton" || (cfg.RulesConfig != nil && cfg.RulesConfig["format"] == "bwf") {
			return CalculateBadmintonStandings(matches, entries), nil
		}
		if sportID == "football" || sportID == "futsal" || (cfg.RulesConfig != nil && cfg.RulesConfig["format"] == "fifa") {
			return CalculateFootballStandings(matches, entries), nil
		}

		winPts := 3
		drawPts := 1
		if cfg.RulesConfig != nil {
			if w, ok := cfg.RulesConfig["win_points"].(float64); ok {
				winPts = int(w)
			}
			if d, ok := cfg.RulesConfig["draw_points"].(float64); ok {
				drawPts = int(d)
			}
		}
		return CalculatePointsStandings(matches, entries, winPts, drawPts), nil
	}

	// For time-based stages (Running, Swimming, Cycling): fetch stored competition results
	resRows, err := s.repo.ListResultsByStage(ctx, stageID)
	if err != nil {
		return nil, err
	}

	out := make([]GenericStandingsRow, len(resRows))
	for i, r := range resRows {
		var rankVal int
		if r.RankOverall.Valid {
			rankVal = int(r.RankOverall.Int32)
		} else {
			rankVal = i + 1
		}

		var timeMs *int64
		var timeFormatted string
		if r.PrimaryTimeMs.Valid {
			ms := r.PrimaryTimeMs.Int64
			timeMs = &ms
			timeFormatted = results.FormatDurationWithPrecision(ms, "ceil_s")
		}

		var meta map[string]any
		_ = json.Unmarshal(r.Metrics, &meta)

		var pts int
		if f, err := r.PointsScored.Float64Value(); err == nil && f.Valid {
			pts = int(f.Float64)
		}

		out[i] = GenericStandingsRow{
			Rank:           rankVal,
			EntryID:        r.EntryID,
			DisplayName:    r.DisplayName,
			IdentifierCode: pgTextValue(r.IdentifierCode),
			Points:         pts,
			PrimaryTimeMs:  timeMs,
			PrimaryTime:    timeFormatted,
			Status:         r.Status,
			CustomMetrics:  meta,
		}
	}

	return out, nil
}

// AdjudicateEntry modifies a participant result status (e.g. DSQ, DNF, PENDING_REVIEW) traceably with audit logging.
func (s *Service) AdjudicateEntry(
	ctx context.Context,
	orgID, eventID, stageID, entryID uuid.UUID,
	newStatus CompetitionStatus,
	reason string,
	actorUserID uuid.UUID,
) error {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return err
	}

	stage, err := s.repo.GetStageByID(ctx, stageID)
	if err != nil || stage.EventID != eventID {
		return ErrStageNotFound
	}

	entry, err := s.repo.GetParticipantEntryByID(ctx, entryID)
	if err != nil || entry.EventID != eventID {
		return ErrEntryNotFound
	}

	// Status safety check
	switch newStatus {
	case StatusFinished, StatusDNF, StatusDNS, StatusDSQ, StatusOTL, StatusPendingReview, StatusWithdrawn, StatusDisqualified:
		// valid
	default:
		return ErrInvalidStatus
	}

	sID := stageID
	_, err = s.repo.UpsertResult(ctx, db.UpsertCompetitionResultParams{
		OrganizationID: orgID,
		EventID:        eventID,
		StageID:        &sID,
		CategoryID:     entry.CategoryID,
		EntryID:        entryID,
		Status:         string(newStatus),
		Notes:          pgText(reason),
		Metrics:        []byte("{}"),
	})
	if err != nil {
		return err
	}

	// Also record canonical observation for the adjudication
	nowMs := time.Now().UnixMilli()
	_, _ = s.RecordObservation(
		ctx, orgID, eventID, &stageID, nil, &entryID,
		ObsAdjudication, "OFFICIAL_ADJUDICATION", nil,
		string(newStatus)+": "+reason, nowMs,
		map[string]any{"adjudicatorUserId": actorUserID.String(), "reason": reason},
	)

	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &actorUserID,
			Action:         "competitions.adjudicate",
			TargetType:     "participant_entry",
			TargetID:       entryID.String(),
			Metadata: map[string]any{
				"newStatus": newStatus,
				"reason":    reason,
				"stageId":   stageID.String(),
			},
		})
	}

	return nil
}

// CyclingFinishRecord represents a single rider finish timestamp.
type CyclingFinishRecord struct {
	EntryID     uuid.UUID `json:"entryId"`
	RawFinishMs int64     `json:"rawFinishMs"`
	Status      string    `json:"status,omitempty"` // FINISHED, DNF, DNS, DSQ, OTL
}

// CyclingStageResultItem represents the computed result for a cycling stage with peloton bunch times.
type CyclingStageResultItem struct {
	EntryID      uuid.UUID `json:"entryId"`
	DisplayName  string    `json:"displayName"`
	RankOverall  int       `json:"rankOverall"`
	RawFinishMs  int64     `json:"rawFinishMs"`
	BunchTimeMs  int64     `json:"bunchTimeMs"`
	GapMs        int64     `json:"gapMs"`
	PelotonGroup int       `json:"pelotonGroup"`
	Status       string    `json:"status"`
}

// ProcessCyclingStageResults processes finish times for a cycling stage applying UCI peloton grouping rules.
// Preserves raw photo-finish/chip timestamps while assigning official bunch times and computing gaps atomically.
func (s *Service) ProcessCyclingStageResults(
	ctx context.Context,
	orgID, eventID, stageID uuid.UUID,
	maxGapMs int64,
	records []CyclingFinishRecord,
) ([]CyclingStageResultItem, error) {
	if err := s.assertEvent(ctx, orgID, eventID); err != nil {
		return nil, err
	}

	stage, err := s.repo.GetStageByID(ctx, stageID)
	if err != nil || stage.EventID != eventID {
		return nil, ErrStageNotFound
	}

	cfg, _ := s.GetCompetitionConfig(ctx, eventID)
	sportID := strings.ToLower(cfg.SportID)
	isCycling := sportID == "cycling" || stage.StageType == string(StagePelotonStage)
	if !isCycling {
		if cfg.RulesConfig != nil {
			if val, ok := cfg.RulesConfig["peloton_bunch_finish"].(bool); !ok || !val {
				return nil, ErrNotPelotonStage
			}
		} else {
			return nil, ErrNotPelotonStage
		}
	}

	if maxGapMs <= 0 {
		maxGapMs = 1000 // UCI 1-second default
	}

	entries, err := s.ListParticipantEntries(ctx, eventID)
	if err != nil {
		return nil, err
	}
	entryMap := make(map[uuid.UUID]ParticipantEntryView, len(entries))
	for _, e := range entries {
		entryMap[e.ID] = e
	}

	// If no records passed directly in request, look up stage finish observations
	if len(records) == 0 {
		obsList, err := s.repo.ListObservationsByStage(ctx, stageID)
		if err == nil {
			for _, obs := range obsList {
				if (obs.ObservationType == string(ObsFinish) || obs.ObservationType == "FINISH") && obs.EntryID != nil {
					records = append(records, CyclingFinishRecord{
						EntryID:     *obs.EntryID,
						RawFinishMs: obs.ObservedAtMs,
						Status:      "FINISHED",
					})
				}
			}
		}
	}

	if len(records) == 0 {
		return nil, ErrNoFinishRecords
	}

	// Separate finishers from non-finishers (DNF, DNS, etc.)
	type finisherWithRecord struct {
		record CyclingFinishRecord
		entry  ParticipantEntryView
	}

	var finishers []finisherWithRecord
	var nonFinishers []finisherWithRecord

	for _, rec := range records {
		entry, exists := entryMap[rec.EntryID]
		if !exists {
			return nil, ErrEntryNotFound
		}

		st := strings.TrimSpace(strings.ToUpper(rec.Status))
		if st == "" {
			return nil, ErrInvalidStatus
		}

		switch st {
		case "FINISHED", "COMPLETED":
			rec.Status = "FINISHED"
			finishers = append(finishers, finisherWithRecord{record: rec, entry: entry})
		case "DNF", "DNS", "DSQ", "OTL":
			rec.Status = st
			nonFinishers = append(nonFinishers, finisherWithRecord{record: rec, entry: entry})
		default:
			return nil, ErrInvalidStatus
		}
	}

	// Sort finishers by raw finish time ascending
	sort.Slice(finishers, func(i, j int) bool {
		return finishers[i].record.RawFinishMs < finishers[j].record.RawFinishMs
	})

	riderTimes := make([]int64, len(finishers))
	for i, f := range finishers {
		riderTimes[i] = f.record.RawFinishMs
	}

	bunchTimes := CalculatePelotonGrouping(riderTimes, maxGapMs)

	// Determine peloton groups: riders with the same bunch time belong to the same group
	groupMap := make(map[int64]int)
	currentGroup := 1
	for _, bt := range bunchTimes {
		if _, exists := groupMap[bt]; !exists {
			groupMap[bt] = currentGroup
			currentGroup++
		}
	}

	var leaderBunchTime int64
	if len(bunchTimes) > 0 {
		leaderBunchTime = bunchTimes[0]
	}

	var zeroNumeric pgtype.Numeric
	_ = zeroNumeric.Scan("0")

	resultsOut := make([]CyclingStageResultItem, 0, len(records))

	// Persist all results inside a single atomic database transaction
	txErr := s.repo.ExecTx(ctx, func(txRepo *Repository) error {
		for i, f := range finishers {
			rank := i + 1
			bunchTime := bunchTimes[i]
			gapMs := bunchTime - leaderBunchTime
			groupNum := groupMap[bunchTime]

			metrics := map[string]any{
				"raw_finish_ms":    f.record.RawFinishMs,
				"bunch_time_ms":    bunchTime,
				"gap_to_leader_ms": gapMs,
				"peloton_group":    groupNum,
			}
			metricsBytes := toJSONBytes(metrics)

			sID := stageID
			catID := f.entry.CategoryID
			_, err := txRepo.UpsertResult(ctx, db.UpsertCompetitionResultParams{
				OrganizationID:  orgID,
				EventID:         eventID,
				StageID:         &sID,
				CategoryID:      catID,
				EntryID:         f.record.EntryID,
				RankOverall:     pgtype.Int4{Int32: int32(rank), Valid: true},
				RankCategory:    pgtype.Int4{Int32: int32(rank), Valid: true},
				Status:          "FINISHED",
				PrimaryTimeMs:   pgtype.Int8{Int64: bunchTime, Valid: true},
				SecondaryTimeMs: pgtype.Int8{Int64: f.record.RawFinishMs, Valid: true},
				PointsScored:    zeroNumeric,
				Metrics:         metricsBytes,
				Notes:           pgText("UCI Peloton bunch finish"),
			})
			if err != nil {
				return err
			}

			resultsOut = append(resultsOut, CyclingStageResultItem{
				EntryID:      f.record.EntryID,
				DisplayName:  f.entry.DisplayName,
				RankOverall:  rank,
				RawFinishMs:  f.record.RawFinishMs,
				BunchTimeMs:  bunchTime,
				GapMs:        gapMs,
				PelotonGroup: groupNum,
				Status:       "FINISHED",
			})
		}

		// Persist non-finishers
		for _, nf := range nonFinishers {
			sID := stageID
			catID := nf.entry.CategoryID
			_, err := txRepo.UpsertResult(ctx, db.UpsertCompetitionResultParams{
				OrganizationID:  orgID,
				EventID:         eventID,
				StageID:         &sID,
				CategoryID:      catID,
				EntryID:         nf.record.EntryID,
				RankOverall:     pgtype.Int4{Valid: false},
				RankCategory:    pgtype.Int4{Valid: false},
				Status:          nf.record.Status,
				PrimaryTimeMs:   pgtype.Int8{Valid: false},
				SecondaryTimeMs: pgtype.Int8{Valid: false},
				PointsScored:    zeroNumeric,
				Metrics:         []byte("{}"),
				Notes:           pgText(nf.record.Status),
			})
			if err != nil {
				return err
			}

			resultsOut = append(resultsOut, CyclingStageResultItem{
				EntryID:     nf.record.EntryID,
				DisplayName: nf.entry.DisplayName,
				Status:      nf.record.Status,
			})
		}

		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	if s.audit != nil {
		s.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			Action:         "competitions.cycling.process_stage",
			TargetType:     "competition_stage",
			TargetID:       stageID.String(),
			Metadata: map[string]any{
				"totalParticipants": len(records),
				"finishersCount":    len(finishers),
				"maxGapMs":          maxGapMs,
			},
		})
	}

	return resultsOut, nil
}

// Helpers
func pgTimeToTime(t pgtype.Timestamptz) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}

func toEntryView(r db.ParticipantEntry) ParticipantEntryView {
	var sNum, lNum, hNum *int
	if r.SeedNumber.Valid {
		n := int(r.SeedNumber.Int32)
		sNum = &n
	}
	if r.LaneNumber.Valid {
		n := int(r.LaneNumber.Int32)
		lNum = &n
	}
	if r.HeatNumber.Valid {
		n := int(r.HeatNumber.Int32)
		hNum = &n
	}

	return ParticipantEntryView{
		ID:             r.ID,
		OrganizationID: r.OrganizationID,
		EventID:        r.EventID,
		CategoryID:     r.CategoryID,
		TicketID:       r.TicketID,
		TeamID:         r.TeamID,
		EntryType:      EntryType(r.EntryType),
		DisplayName:    r.DisplayName,
		IdentifierCode: pgTextValue(r.IdentifierCode),
		SeedNumber:     sNum,
		LaneNumber:     lNum,
		HeatNumber:     hNum,
		Status:         CompetitionStatus(r.Status),
		CreatedAt:      pgTimeToTime(r.CreatedAt),
		UpdatedAt:      pgTimeToTime(r.UpdatedAt),
	}
}

func toMatchView(r db.CompetitionMatch) CompetitionMatchView {
	var details []MatchScoreDetail
	_ = json.Unmarshal(r.ScoreDetails, &details)

	var startTime *time.Time
	if r.ScheduledStartTime.Valid {
		t := r.ScheduledStartTime.Time
		startTime = &t
	}

	return CompetitionMatchView{
		ID:                 r.ID,
		OrganizationID:     r.OrganizationID,
		EventID:            r.EventID,
		StageID:            r.StageID,
		RoundNumber:        int(r.RoundNumber),
		MatchNumber:        int(r.MatchNumber),
		HomeTeamID:         r.HomeTeamID,
		AwayTeamID:         r.AwayTeamID,
		HomeEntryID:        r.HomeEntryID,
		AwayEntryID:        r.AwayEntryID,
		ScheduledStartTime: startTime,
		VenueCourtName:     pgTextValue(r.VenueCourtName),
		HomeScore:          int(r.HomeScore),
		AwayScore:          int(r.AwayScore),
		MatchStatus:        r.MatchStatus,
		WinnerID:           r.WinnerID,
		ScoreDetails:       details,
		CreatedAt:          pgTimeToTime(r.CreatedAt),
		UpdatedAt:          pgTimeToTime(r.UpdatedAt),
	}
}

func toObservationView(r db.CompetitionObservation) CompetitionObservationView {
	var meta map[string]any
	_ = json.Unmarshal(r.Metadata, &meta)

	var numVal *float64
	if r.NumericValue.Valid {
		f, err := r.NumericValue.Float64Value()
		if err == nil && f.Valid {
			numVal = &f.Float64
		}
	}

	return CompetitionObservationView{
		ID:              r.ID,
		OrganizationID:  r.OrganizationID,
		EventID:         r.EventID,
		StageID:         r.StageID,
		MatchID:         r.MatchID,
		EntryID:         r.EntryID,
		ObservationType: ObservationType(r.ObservationType),
		Source:          r.Source,
		NumericValue:    numVal,
		TextValue:       pgTextValue(r.TextValue),
		ObservedAtMs:    r.ObservedAtMs,
		Metadata:        meta,
		CreatedAt:       pgTimeToTime(r.CreatedAt),
	}
}

func pgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func pgTextValue(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}
