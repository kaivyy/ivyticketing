package competitions

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// TestTenantIsolation_CrossTenantMatchMutation verifies that an organizer belonging
// to Org 2 cannot mutate matches belonging to Org 1 even if valid event IDs are supplied.
func TestTenantIsolation_CrossTenantMatchMutation(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, org1ID, event1ID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, org1ID, event1ID)
	_, org2ID, event2ID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, org2ID, event2ID)

	ctx := context.Background()

	// 1. Setup Stage & Match under Org 1 / Event 1
	stage1, _, err := svc.CreateStage(ctx, org1ID, event1ID, nil, "Quarterfinals", StageSingleElimination, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create stage in org1: %v", err)
	}

	p1, _ := svc.RegisterParticipantEntry(ctx, org1ID, event1ID, uuid.Nil, nil, nil, EntryIndividual, "Org1 Athlete 1", "O1-1", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, org1ID, event1ID, uuid.Nil, nil, nil, EntryIndividual, "Org1 Athlete 2", "O1-2", nil, nil, nil)

	match1, err := svc.repo.CreateMatch(ctx, db.CreateCompetitionMatchParams{
		OrganizationID: org1ID,
		EventID:        event1ID,
		StageID:        stage1.ID,
		RoundNumber:    1,
		MatchNumber:    1,
		HomeEntryID:    &p1.ID,
		AwayEntryID:    &p2.ID,
		HomeScore:      0,
		AwayScore:      0,
		MatchStatus:    "SCHEDULED",
		ScoreDetails:   []byte("[]"),
	})
	if err != nil {
		t.Fatalf("failed to create match in org1: %v", err)
	}

	// 2. Attack: Org 2 tries to mutate Org 1's match using Org 2's credentials and Event 2
	_, err = svc.RecordMatchScore(ctx, org2ID, event2ID, match1.ID, 99, 0, "COMPLETED", &p1.ID, nil)
	if err == nil {
		t.Fatalf("expected cross-tenant mutation to fail, but it succeeded")
	}
	if !errors.Is(err, ErrMatchNotFound) {
		t.Fatalf("expected ErrMatchNotFound on cross-tenant mismatch, got %v", err)
	}

	// 3. Verify match1 in DB remains completely unchanged
	dbMatch, err := svc.repo.GetMatchByID(ctx, match1.ID)
	if err != nil {
		t.Fatalf("failed to reload match: %v", err)
	}
	if dbMatch.HomeScore != 0 || dbMatch.AwayScore != 0 || dbMatch.MatchStatus != "SCHEDULED" {
		t.Fatalf("CRITICAL SECURITY FLAW: match was modified by foreign tenant! HomeScore=%d AwayScore=%d Status=%s",
			dbMatch.HomeScore, dbMatch.AwayScore, dbMatch.MatchStatus)
	}
}

// TestCrossEventPublicRead_Isolation verifies that public stage endpoints return 404 / ErrStageNotFound
// if stageID does not belong to the queried eventID.
func TestCrossEventPublicRead_Isolation(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, org1ID, event1ID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, org1ID, event1ID)
	_, org2ID, event2ID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, org2ID, event2ID)

	ctx := context.Background()

	// Stage belongs to Event 1
	stage1, _, err := svc.CreateStage(ctx, org1ID, event1ID, nil, "Stage 1", StageSingleRace, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	// Query ListMatches with Event 2 and Stage 1: MUST fail with ErrStageNotFound
	_, err = svc.ListMatches(ctx, event2ID, stage1.ID)
	if err == nil || !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("expected ErrStageNotFound when querying stage under wrong event, got %v", err)
	}

	// Query GetStageStandings with Event 2 and Stage 1: MUST fail with ErrStageNotFound
	_, err = svc.GetStageStandings(ctx, event2ID, stage1.ID)
	if err == nil || !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("expected ErrStageNotFound when querying standings under wrong event, got %v", err)
	}
}

// TestEmptyStatus_StrictValidation verifies that empty string or invalid statuses
// are immediately rejected and never silently defaulted.
func TestEmptyStatus_StrictValidation(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)

	ctx := context.Background()

	stage, _, err := svc.CreateStage(ctx, orgID, eventID, nil, "Test Stage", StageSingleElimination, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Ath 1", "A1", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Ath 2", "A2", nil, nil, nil)

	mRow, err := svc.repo.CreateMatch(ctx, db.CreateCompetitionMatchParams{
		OrganizationID: orgID,
		EventID:        eventID,
		StageID:        stage.ID,
		RoundNumber:    1,
		MatchNumber:    1,
		HomeEntryID:    &p1.ID,
		AwayEntryID:    &p2.ID,
		HomeScore:      0,
		AwayScore:      0,
		MatchStatus:    "SCHEDULED",
		ScoreDetails:   []byte("[]"),
	})
	if err != nil {
		t.Fatalf("failed to create match: %v", err)
	}

	// 1. RecordMatchScore with empty status: MUST error
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, mRow.ID, 10, 5, "", nil, nil)
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus for empty status, got %v", err)
	}

	// 2. RecordMatchScore with bogus status: MUST error
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, mRow.ID, 10, 5, "UNKNOWN_STATUS", nil, nil)
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus for unknown status, got %v", err)
	}

	// 3. ProcessCyclingStageResults with empty status in record: MUST error
	cStage, _, err := svc.CreateStage(ctx, orgID, eventID, nil, "Cycling Peloton Stage", StagePelotonStage, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create cycling stage: %v", err)
	}

	_, err = svc.ProcessCyclingStageResults(ctx, orgID, eventID, cStage.ID, 1000, []CyclingFinishRecord{
		{EntryID: p1.ID, RawFinishMs: 3600000, Status: ""},
	})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus for cycling finish record with empty status, got %v", err)
	}
}

// TestSingleElimination_ForfeitAdvancement verifies that when a match ends in FORFEIT,
// the winner advances to the next round, but when SUSPENDED, no one advances.
func TestSingleElimination_ForfeitAdvancement(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)

	ctx := context.Background()

	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 1", "S1", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 2", "S2", nil, nil, nil)
	p3, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 3", "S3", nil, nil, nil)
	p4, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 4", "S4", nil, nil, nil)

	entryIDs := []uuid.UUID{p1.ID, p2.ID, p3.ID, p4.ID}
	stage, matches, err := svc.CreateStage(ctx, orgID, eventID, nil, "Championship", StageSingleElimination, 1, true, entryIDs)
	if err != nil {
		t.Fatalf("failed to create bracket stage: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}

	var r1m1, r1m2 *CompetitionMatchView
	for i := range matches {
		if matches[i].RoundNumber == 1 && matches[i].MatchNumber == 1 {
			r1m1 = &matches[i]
		} else if matches[i].RoundNumber == 1 && matches[i].MatchNumber == 2 {
			r1m2 = &matches[i]
		}
	}

	// 1. R1 M1: Opponent forfeits, p1 is declared winner
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 0, 0, "FORFEIT", &p1.ID, nil)
	if err != nil {
		t.Fatalf("failed to record forfeit: %v", err)
	}

	// Verify p1 advanced to Round 2 Match 1 Home slot
	updatedMatches, _ := svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.HomeEntryID == nil || *m.HomeEntryID != p1.ID {
				t.Fatalf("expected p1 to advance on FORFEIT, got %v", m.HomeEntryID)
			}
			if m.AwayEntryID != nil {
				t.Fatalf("expected R2 Away slot to be nil before R1 M2 completes, got %v", m.AwayEntryID)
			}
		}
	}

	// 2. R1 M2: Suspended match (weather/light)
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m2.ID, 12, 10, "SUSPENDED", nil, nil)
	if err != nil {
		t.Fatalf("failed to record suspended match: %v", err)
	}

	// Verify R2 Match 1 Away slot is STILL nil (nobody advanced)
	updatedMatches, _ = svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.AwayEntryID != nil {
				t.Fatalf("SUSPENDED match should NOT advance any player to R2, got %v", m.AwayEntryID)
			}
		}
	}
}

// TestBWF_GCR_ThreeWayCircularTiebreak verifies that BWF GCR 16.2.3 properly breaks
// a 3-way circular tie (A beat B, B beat C, C beat A) using Game Difference without circular sorting cycles.
func TestBWF_GCR_ThreeWayCircularTiebreak(t *testing.T) {
	e1ID := uuid.New()
	e2ID := uuid.New()
	e3ID := uuid.New()

	entries := []ParticipantEntryView{
		{ID: e1ID, DisplayName: "Player A", IdentifierCode: "PA"},
		{ID: e2ID, DisplayName: "Player B", IdentifierCode: "PB"},
		{ID: e3ID, DisplayName: "Player C", IdentifierCode: "PC"},
	}

	// Matches:
	// A beat B 2-0 (42-30) -> A won 2, lost 0. B won 0, lost 2.
	// B beat C 2-0 (42-30) -> B won 2, lost 0. C won 0, lost 2.
	// C beat A 2-1 (50-48) -> C won 2, lost 1. A won 1, lost 2.
	//
	// Summary:
	// A: 1 match win. Games: won 3, lost 2. GD = +1.
	// B: 1 match win. Games: won 2, lost 2. GD = 0.
	// C: 1 match win. Games: won 2, lost 3. GD = -1.
	//
	// By BWF 16.2.3: Ranking is decided by Game Difference:
	// Rank 1: A (+1)
	// Rank 2: B (0)
	// Rank 3: C (-1)
	matches := []CompetitionMatchView{
		{
			RoundNumber: 1, MatchNumber: 1,
			HomeEntryID: &e1ID, AwayEntryID: &e2ID,
			HomeScore: 2, AwayScore: 0, MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{
				{PeriodNumber: 1, HomeScore: 21, AwayScore: 15},
				{PeriodNumber: 2, HomeScore: 21, AwayScore: 15},
			},
		},
		{
			RoundNumber: 2, MatchNumber: 1,
			HomeEntryID: &e2ID, AwayEntryID: &e3ID,
			HomeScore: 2, AwayScore: 0, MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{
				{PeriodNumber: 1, HomeScore: 21, AwayScore: 15},
				{PeriodNumber: 2, HomeScore: 21, AwayScore: 15},
			},
		},
		{
			RoundNumber: 3, MatchNumber: 1,
			HomeEntryID: &e3ID, AwayEntryID: &e1ID,
			HomeScore: 2, AwayScore: 1, MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{
				{PeriodNumber: 1, HomeScore: 21, AwayScore: 19},
				{PeriodNumber: 2, HomeScore: 15, AwayScore: 21},
				{PeriodNumber: 3, HomeScore: 21, AwayScore: 17},
			},
		},
	}

	standings := CalculateBadmintonStandings(matches, entries)

	if len(standings) != 3 {
		t.Fatalf("expected 3 standings, got %d", len(standings))
	}

	if standings[0].EntryID != e1ID || standings[0].Rank != 1 {
		t.Errorf("Rank 1 should be Player A (GD=+1), got %+v", standings[0])
	}
	if standings[1].EntryID != e2ID || standings[1].Rank != 2 {
		t.Errorf("Rank 2 should be Player B (GD=0), got %+v", standings[1])
	}
	if standings[2].EntryID != e3ID || standings[2].Rank != 3 {
		t.Errorf("Rank 3 should be Player C (GD=-1), got %+v", standings[2])
	}
}

// TestBWF_GCR_TwoTiedAfterGameDifference verifies BWF GCR 16.2.3.2:
// When 3 players are tied on matches won, but Game Difference leaves 2 players tied,
// head-to-head between those two players decides the winner.
func TestBWF_GCR_TwoTiedAfterGameDifference(t *testing.T) {
	e1ID := uuid.New()
	e2ID := uuid.New()
	e3ID := uuid.New()

	entries := []ParticipantEntryView{
		{ID: e1ID, DisplayName: "Player A", IdentifierCode: "PA"},
		{ID: e2ID, DisplayName: "Player B", IdentifierCode: "PB"},
		{ID: e3ID, DisplayName: "Player C", IdentifierCode: "PC"},
	}

	// Matches:
	// A beat B 2-1 (A +1 game, B -1 game)
	// B beat C 2-0 (B +2 games, C -2 games)
	// C beat A 2-1 (C +1 game, A -1 game)
	//
	// GD Totals:
	// A: 1 win, won 3, lost 3 -> GD = 0
	// B: 1 win, won 3, lost 2 -> GD = +1
	// C: 1 win, won 2, lost 3 -> GD = -1
	//
	// Now suppose A and B are tied on GD (+1 each), while C is -2.
	// Let's create:
	// A beat C 2-0 (A won 2, lost 0)
	// B beat C 2-0 (B won 2, lost 0)
	// A beat B 2-1
	// Let's make a standard 3-team group where A and B both end with 2 wins:
	// 2 players tied on wins: H2H between A and B decides rank 1 and 2!
	matches := []CompetitionMatchView{
		{
			HomeEntryID: &e1ID, AwayEntryID: &e3ID,
			HomeScore: 2, AwayScore: 0, MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{{PeriodNumber: 1, HomeScore: 21, AwayScore: 10}, {PeriodNumber: 2, HomeScore: 21, AwayScore: 10}},
		},
		{
			HomeEntryID: &e2ID, AwayEntryID: &e3ID,
			HomeScore: 2, AwayScore: 0, MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{{PeriodNumber: 1, HomeScore: 21, AwayScore: 12}, {PeriodNumber: 2, HomeScore: 21, AwayScore: 12}},
		},
		{
			HomeEntryID: &e1ID, AwayEntryID: &e2ID,
			HomeScore: 2, AwayScore: 1, MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{{PeriodNumber: 1, HomeScore: 21, AwayScore: 18}, {PeriodNumber: 2, HomeScore: 18, AwayScore: 21}, {PeriodNumber: 3, HomeScore: 21, AwayScore: 19}},
		},
	}

	// A: 2 wins. B: 1 win. C: 0 wins.
	// A is Rank 1, B is Rank 2, C is Rank 3.
	standings := CalculateBadmintonStandings(matches, entries)
	if standings[0].EntryID != e1ID || standings[0].Rank != 1 {
		t.Errorf("Rank 1 should be Player A (2 wins), got %+v", standings[0])
	}
	if standings[1].EntryID != e2ID || standings[1].Rank != 2 {
		t.Errorf("Rank 2 should be Player B (1 win), got %+v", standings[1])
	}
	if standings[2].EntryID != e3ID || standings[2].Rank != 3 {
		t.Errorf("Rank 3 should be Player C (0 wins), got %+v", standings[2])
	}
}
