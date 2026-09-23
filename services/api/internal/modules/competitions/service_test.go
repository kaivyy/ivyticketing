package competitions

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/ivyticketing?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping database test: %v", err)
		return nil
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping database test (cannot ping): %v", err)
		return nil
	}
	return pool
}

func setupTestContext(t *testing.T, pool *pgxpool.Pool) (*Service, uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	repo := NewRepository(pool)
	svc := NewService(repo, nil)

	// Create test organization
	orgID := uuid.New()
	slug := "test-org-" + orgID.String()[:8]
	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING
	`, orgID, "Test Org "+slug, slug)
	if err != nil {
		t.Fatalf("failed to create test org: %v", err)
	}

	// Create test event
	eventID := uuid.New()
	eventSlug := "test-event-" + eventID.String()[:8]
	_, err = pool.Exec(ctx, `
		INSERT INTO events (id, organization_id, name, slug, status, event_type, sport_id, discipline_id)
		VALUES ($1, $2, $3, $4, 'published', 'COMPETITION', 'cycling', 'road_cycling')
		ON CONFLICT (id) DO NOTHING
	`, eventID, orgID, "Test Event "+eventSlug, eventSlug)
	if err != nil {
		t.Fatalf("failed to create test event: %v", err)
	}

	return svc, orgID, eventID
}

func cleanupTestContext(pool *pgxpool.Pool, orgID, eventID uuid.UUID) {
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, eventID)
	_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
}

func TestTournamentBracketAdvancement(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)
	ctx := context.Background()

	// 1. Register 4 entries
	entry1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Athlete 1", "BIB-01", nil, nil, nil)
	entry2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Athlete 2", "BIB-02", nil, nil, nil)
	entry3, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Athlete 3", "BIB-03", nil, nil, nil)
	entry4, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Athlete 4", "BIB-04", nil, nil, nil)

	entryIDs := []uuid.UUID{entry1.ID, entry2.ID, entry3.ID, entry4.ID}

	// 2. Create single elimination stage with auto-generated fixtures
	stage, matches, err := svc.CreateStage(ctx, orgID, eventID, nil, "Knockout Stage", StageSingleElimination, 1, true, entryIDs)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	// 4 entries -> 3 matches total (2 in R1, 1 in R2)
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches generated, got %d", len(matches))
	}

	// Fetch matches from DB to verify structure
	dbMatches, err := svc.ListMatches(ctx, eventID, stage.ID)
	if err != nil {
		t.Fatalf("failed to list matches: %v", err)
	}

	var r1m1, r1m2, r2m1 *CompetitionMatchView
	for i := range dbMatches {
		m := &dbMatches[i]
		if m.RoundNumber == 1 && m.MatchNumber == 1 {
			r1m1 = m
		} else if m.RoundNumber == 1 && m.MatchNumber == 2 {
			r1m2 = m
		} else if m.RoundNumber == 2 && m.MatchNumber == 1 {
			r2m1 = m
		}
	}

	if r1m1 == nil || r1m2 == nil || r2m1 == nil {
		t.Fatalf("missing required matches: r1m1=%v, r1m2=%v, r2m1=%v", r1m1, r1m2, r2m1)
	}

	// Final match should initially have nil participants
	if r2m1.HomeEntryID != nil || r2m1.AwayEntryID != nil {
		t.Errorf("R2 M1 should initially have nil participants")
	}

	// 3. Play Round 1 Match 1: Home (Athlete 1) wins 21 - 15
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 21, 15, "COMPLETED", nil, nil)
	if err != nil {
		t.Fatalf("failed to record R1 M1 score: %v", err)
	}

	// Verify Athlete 1 advanced to Round 2 Match 1 Home slot
	updatedMatches, _ := svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.HomeEntryID == nil || *m.HomeEntryID != entry1.ID {
				t.Fatalf("expected Athlete 1 in R2 M1 Home slot, got %v", m.HomeEntryID)
			}
			if m.AwayEntryID != nil {
				t.Fatalf("expected R2 M1 Away slot to still be nil, got %v", m.AwayEntryID)
			}
		}
	}

	// 4. Play Round 1 Match 2: Away (Athlete 3) wins 18 - 21
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m2.ID, 18, 21, "COMPLETED", nil, nil)
	if err != nil {
		t.Fatalf("failed to record R1 M2 score: %v", err)
	}

	// Verify Athlete 3 advanced to Round 2 Match 1 Away slot
	updatedMatches, _ = svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.HomeEntryID == nil || *m.HomeEntryID != entry1.ID {
				t.Fatalf("expected Athlete 1 in R2 M1 Home, got %v", m.HomeEntryID)
			}
			if m.AwayEntryID == nil || *m.AwayEntryID != entry3.ID {
				t.Fatalf("expected Athlete 3 in R2 M1 Away, got %v", m.AwayEntryID)
			}
		}
	}

	// 5. Play Finals (Round 2 Match 1): Athlete 1 wins 21 - 19
	finalUpdated, err := svc.RecordMatchScore(ctx, orgID, eventID, r2m1.ID, 21, 19, "COMPLETED", nil, nil)
	if err != nil {
		t.Fatalf("failed to record Final score: %v", err)
	}
	if finalUpdated.MatchStatus != "COMPLETED" {
		t.Errorf("expected final status COMPLETED, got %s", finalUpdated.MatchStatus)
	}
}

func TestCyclingPelotonStageResultsPipeline(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)
	ctx := context.Background()

	// Configure event as cycling road race
	_, err := svc.ConfigureCompetition(ctx, orgID, eventID, nil, CompetitionConfigView{
		SportID:         "cycling",
		DisciplineID:    "road_cycling",
		HasTiming:       true,
		HasStages:       true,
		RankingStrategy: StrategyTimeAsc,
		TimePrecision:   PrecisionCentiseconds,
		RulesConfig:     map[string]any{"peloton_bunch_finish": true},
	})
	if err != nil {
		t.Fatalf("failed to configure cycling competition: %v", err)
	}

	// Create peloton stage
	stage, _, err := svc.CreateStage(ctx, orgID, eventID, nil, "Stage 1: Road Race", StagePelotonStage, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create peloton stage: %v", err)
	}

	// Register 5 cyclists
	e1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Cyclist A", "101", nil, nil, nil)
	e2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Cyclist B", "102", nil, nil, nil)
	e3, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Cyclist C", "103", nil, nil, nil)
	e4, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Cyclist D", "104", nil, nil, nil)
	e5, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Cyclist E", "105", nil, nil, nil)

	// Finish records:
	// Bunch 1: Cyclist A (10,000,000ms), B (+400ms), C (+800ms)
	// Bunch 2: Cyclist D (+1700ms gap -> 10,002,500ms), E (+500ms -> 10,003,000ms)
	records := []CyclingFinishRecord{
		{EntryID: e1.ID, RawFinishMs: 10000000, Status: "FINISHED"},
		{EntryID: e2.ID, RawFinishMs: 10000400, Status: "FINISHED"},
		{EntryID: e3.ID, RawFinishMs: 10000800, Status: "FINISHED"},
		{EntryID: e4.ID, RawFinishMs: 10002500, Status: "FINISHED"},
		{EntryID: e5.ID, RawFinishMs: 10003000, Status: "FINISHED"},
	}

	// Process stage results
	results, err := svc.ProcessCyclingStageResults(ctx, orgID, eventID, stage.ID, 1000, records)
	if err != nil {
		t.Fatalf("failed to process cycling stage results: %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// Check bunch times:
	// Bunch 1 (first 3 riders) should all have official bunch time 10,000,000 ms
	for i := 0; i < 3; i++ {
		if results[i].BunchTimeMs != 10000000 {
			t.Errorf("rider %d expected bunch time 10000000, got %d", i+1, results[i].BunchTimeMs)
		}
		if results[i].PelotonGroup != 1 {
			t.Errorf("rider %d expected group 1, got %d", i+1, results[i].PelotonGroup)
		}
		if results[i].GapMs != 0 {
			t.Errorf("rider %d expected gap 0, got %d", i+1, results[i].GapMs)
		}
		if results[i].RankOverall != i+1 {
			t.Errorf("rider %d expected rank %d, got %d", i+1, i+1, results[i].RankOverall)
		}
	}

	// Check raw finish times are preserved
	if results[0].RawFinishMs != 10000000 || results[1].RawFinishMs != 10000400 || results[2].RawFinishMs != 10000800 {
		t.Errorf("raw finish times not preserved correctly for bunch 1")
	}

	// Bunch 2 (riders 4 and 5) should have bunch time 10,002,500 ms and gap 2500 ms
	for i := 3; i < 5; i++ {
		if results[i].BunchTimeMs != 10002500 {
			t.Errorf("rider %d expected bunch time 10002500, got %d", i+1, results[i].BunchTimeMs)
		}
		if results[i].PelotonGroup != 2 {
			t.Errorf("rider %d expected group 2, got %d", i+1, results[i].PelotonGroup)
		}
		if results[i].GapMs != 2500 {
			t.Errorf("rider %d expected gap 2500, got %d", i+1, results[i].GapMs)
		}
		if results[i].RankOverall != i+1 {
			t.Errorf("rider %d expected rank %d, got %d", i+1, i+1, results[i].RankOverall)
		}
	}

	// Check stored database results
	standings, err := svc.GetStageStandings(ctx, eventID, stage.ID)
	if err != nil {
		t.Fatalf("failed to get stage standings: %v", err)
	}
	if len(standings) != 5 {
		t.Fatalf("expected 5 standings rows from DB, got %d", len(standings))
	}

	// Verify DB primary_time_ms is the official bunch time
	if *standings[0].PrimaryTimeMs != 10000000 || *standings[1].PrimaryTimeMs != 10000000 {
		t.Errorf("DB primary time mismatch for bunch 1: %v, %v", *standings[0].PrimaryTimeMs, *standings[1].PrimaryTimeMs)
	}
	if *standings[3].PrimaryTimeMs != 10002500 || *standings[4].PrimaryTimeMs != 10002500 {
		t.Errorf("DB primary time mismatch for bunch 2: %v, %v", *standings[3].PrimaryTimeMs, *standings[4].PrimaryTimeMs)
	}
}

func TestStandingsRoutingDecoupling(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)
	ctx := context.Background()

	// 1. Configure event as Badminton
	_, err := svc.ConfigureCompetition(ctx, orgID, eventID, nil, CompetitionConfigView{
		SportID:         "badminton",
		DisciplineID:    "badminton_singles",
		HasMatches:      true,
		HasScoring:      true,
		RankingStrategy: StrategyWinCount,
		RulesConfig:     map[string]any{"format": "bwf"},
	})
	if err != nil {
		t.Fatalf("failed to configure badminton: %v", err)
	}

	stage, _, err := svc.CreateStage(ctx, orgID, eventID, nil, "Group A", StageGroupRoundRobin, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create round robin stage: %v", err)
	}

	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Player 1", "P1", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Player 2", "P2", nil, nil, nil)

	// Create match and record score
	mRow, err := svc.repo.CreateMatch(ctx, db.CreateCompetitionMatchParams{
		OrganizationID: orgID,
		EventID:        eventID,
		StageID:        stage.ID,
		RoundNumber:    1,
		MatchNumber:    1,
		HomeEntryID:    &p1.ID,
		AwayEntryID:    &p2.ID,
		HomeScore:      2,
		AwayScore:      0,
		MatchStatus:    "COMPLETED",
		ScoreDetails:   []byte(`[{"periodNumber":1,"homeScore":21,"awayScore":15},{"periodNumber":2,"homeScore":21,"awayScore":18}]`),
	})
	if err != nil {
		t.Fatalf("failed to create match: %v", err)
	}
	_ = mRow

	// Get standings: should route to Badminton standings
	standings, err := svc.GetStageStandings(ctx, eventID, stage.ID)
	if err != nil {
		t.Fatalf("failed to get stage standings: %v", err)
	}

	if len(standings) != 2 {
		t.Fatalf("expected 2 standings, got %d", len(standings))
	}
	if standings[0].EntryID != p1.ID || standings[0].Wins != 1 {
		t.Errorf("expected Player 1 to win and be rank 1, got %+v", standings[0])
	}
	// Verify badminton metrics exist
	if standings[0].CustomMetrics["games_won"] == nil || standings[0].CustomMetrics["rally_points_won"] == nil {
		t.Errorf("expected badminton metrics games_won and rally_points_won, got %+v", standings[0].CustomMetrics)
	}
}
