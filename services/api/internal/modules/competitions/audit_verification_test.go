package competitions

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Section 2: CROSS-TENANT MATCH MUTATION AUDIT
func TestAudit_Section2_CrossTenantMatchMutation(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgAID, eventAID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgAID, eventAID)
	_, orgBID, eventBID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgBID, eventBID)

	ctx := context.Background()

	// 1. Setup Org A: Event A -> Stage A -> Match A
	stageA, _, err := svc.CreateStage(ctx, orgAID, eventAID, nil, "Stage A", StageSingleElimination, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create Stage A: %v", err)
	}
	pA1, _ := svc.RegisterParticipantEntry(ctx, orgAID, eventAID, uuid.Nil, nil, nil, EntryIndividual, "Athlete A1", "A1", nil, nil, nil)
	pA2, _ := svc.RegisterParticipantEntry(ctx, orgAID, eventAID, uuid.Nil, nil, nil, EntryIndividual, "Athlete A2", "A2", nil, nil, nil)

	matchA, err := svc.repo.CreateMatch(ctx, db.CreateCompetitionMatchParams{
		OrganizationID: orgAID,
		EventID:        eventAID,
		StageID:        stageA.ID,
		RoundNumber:    1,
		MatchNumber:    1,
		HomeEntryID:    &pA1.ID,
		AwayEntryID:    &pA2.ID,
		HomeScore:      0,
		AwayScore:      0,
		MatchStatus:    "SCHEDULED",
		ScoreDetails:   []byte("[]"),
	})
	if err != nil {
		t.Fatalf("failed to create Match A: %v", err)
	}

	// 2. Setup Org B: Event B -> Stage B -> Match B
	stageB, _, err := svc.CreateStage(ctx, orgBID, eventBID, nil, "Stage B", StageSingleElimination, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create Stage B: %v", err)
	}
	pB1, _ := svc.RegisterParticipantEntry(ctx, orgBID, eventBID, uuid.Nil, nil, nil, EntryIndividual, "Athlete B1", "B1", nil, nil, nil)
	pB2, _ := svc.RegisterParticipantEntry(ctx, orgBID, eventBID, uuid.Nil, nil, nil, EntryIndividual, "Athlete B2", "B2", nil, nil, nil)

	matchB, err := svc.repo.CreateMatch(ctx, db.CreateCompetitionMatchParams{
		OrganizationID: orgBID,
		EventID:        eventBID,
		StageID:        stageB.ID,
		RoundNumber:    1,
		MatchNumber:    1,
		HomeEntryID:    &pB1.ID,
		AwayEntryID:    &pB2.ID,
		HomeScore:      0,
		AwayScore:      0,
		MatchStatus:    "SCHEDULED",
		ScoreDetails:   []byte("[]"),
	})
	if err != nil {
		t.Fatalf("failed to create Match B: %v", err)
	}

	assertMatchBUnchanged := func(stepName string) {
		dbB, err := svc.repo.GetMatchByID(ctx, matchB.ID)
		if err != nil {
			t.Fatalf("[%s] failed to fetch match B: %v", stepName, err)
		}
		if dbB.HomeScore != 0 || dbB.AwayScore != 0 || dbB.MatchStatus != "SCHEDULED" || dbB.WinnerID != nil {
			t.Fatalf("[%s] SECURITY DEFECT: Match B was modified! Score: %d-%d Status: %s Winner: %v",
				stepName, dbB.HomeScore, dbB.AwayScore, dbB.MatchStatus, dbB.WinnerID)
		}
	}

	// Test 1: Legitimate operation: Org A modifies Match A using Event A
	mAUpdated, err := svc.RecordMatchScore(ctx, orgAID, eventAID, matchA.ID, 21, 15, "COMPLETED", &pA1.ID, nil)
	if err != nil {
		t.Fatalf("legitimate update on Match A failed: %v", err)
	}
	if mAUpdated.HomeScore != 21 || mAUpdated.AwayScore != 15 || mAUpdated.MatchStatus != "COMPLETED" {
		t.Fatalf("Match A was not updated correctly: %+v", mAUpdated)
	}
	assertMatchBUnchanged("after legitimate Match A update")

	// Test 2: Attack: Org A modifies Match B using Event A (own event, foreign match)
	_, err = svc.RecordMatchScore(ctx, orgAID, eventAID, matchB.ID, 99, 0, "COMPLETED", &pB1.ID, nil)
	if err == nil {
		t.Fatalf("attack 2 succeeded when it should fail")
	}
	if !errors.Is(err, ErrMatchNotFound) {
		t.Fatalf("attack 2 expected ErrMatchNotFound, got %v", err)
	}
	assertMatchBUnchanged("after attack 2 (Match B via Event A)")

	// Test 3: Attack: Org A modifies Match B using Event B (foreign event, foreign match)
	_, err = svc.RecordMatchScore(ctx, orgAID, eventBID, matchB.ID, 99, 0, "COMPLETED", &pB1.ID, nil)
	if err == nil {
		t.Fatalf("attack 3 succeeded when it should fail")
	}
	if !errors.Is(err, ErrEventForbidden) {
		t.Fatalf("attack 3 expected ErrEventForbidden, got %v", err)
	}
	assertMatchBUnchanged("after attack 3 (Match B via Event B with Org A)")

	// Test 4: Attack: Org A modifies Match A using fake Event ID
	fakeEventID := uuid.New()
	_, err = svc.RecordMatchScore(ctx, orgAID, fakeEventID, matchA.ID, 99, 0, "COMPLETED", &pA1.ID, nil)
	if err == nil {
		t.Fatalf("attack 4 succeeded when it should fail")
	}
	if !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("attack 4 expected ErrEventNotFound, got %v", err)
	}
	assertMatchBUnchanged("after attack 4 (Match A via fake Event ID)")

	// Test 5: Attack: Org A modifies Match B using Org A credentials directly
	_, err = svc.RecordMatchScore(ctx, orgAID, eventAID, matchB.ID, 50, 50, "FORFEIT", &pA1.ID, nil)
	if err == nil {
		t.Fatalf("attack 5 succeeded when it should fail")
	}
	if !errors.Is(err, ErrMatchNotFound) {
		t.Fatalf("attack 5 expected ErrMatchNotFound, got %v", err)
	}
	assertMatchBUnchanged("after attack 5 (Match B via Org A credentials)")
}

// Section 3: CROSS-EVENT PUBLIC READ AUDIT
func TestAudit_Section3_CrossEventPublicRead(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgAID, eventAID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgAID, eventAID)
	_, orgBID, eventBID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgBID, eventBID)

	ctx := context.Background()

	// Event A + Stage A
	stageA, _, err := svc.CreateStage(ctx, orgAID, eventAID, nil, "Stage A", StageSingleElimination, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create Stage A: %v", err)
	}
	// Event B + Stage B
	stageB, _, err := svc.CreateStage(ctx, orgBID, eventBID, nil, "Stage B", StageSingleElimination, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create Stage B: %v", err)
	}

	fakeStageID := uuid.New()
	fakeEventID := uuid.New()

	// 1. Event A + Stage A -> SUCCESS
	matchesA, err := svc.ListMatches(ctx, eventAID, stageA.ID)
	if err != nil {
		t.Fatalf("Event A + Stage A matches failed: %v", err)
	}
	_ = matchesA
	standingsA, err := svc.GetStageStandings(ctx, eventAID, stageA.ID)
	if err != nil {
		t.Fatalf("Event A + Stage A standings failed: %v", err)
	}
	_ = standingsA

	// 2. Event A + Stage B -> MUST FAIL with ErrStageNotFound
	_, err = svc.ListMatches(ctx, eventAID, stageB.ID)
	if !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("Event A + Stage B matches expected ErrStageNotFound, got %v", err)
	}
	_, err = svc.GetStageStandings(ctx, eventAID, stageB.ID)
	if !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("Event A + Stage B standings expected ErrStageNotFound, got %v", err)
	}

	// 3. Event B + Stage A -> MUST FAIL with ErrStageNotFound
	_, err = svc.ListMatches(ctx, eventBID, stageA.ID)
	if !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("Event B + Stage A matches expected ErrStageNotFound, got %v", err)
	}
	_, err = svc.GetStageStandings(ctx, eventBID, stageA.ID)
	if !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("Event B + Stage A standings expected ErrStageNotFound, got %v", err)
	}

	// 4. Event B + Stage B -> SUCCESS
	matchesB, err := svc.ListMatches(ctx, eventBID, stageB.ID)
	if err != nil {
		t.Fatalf("Event B + Stage B matches failed: %v", err)
	}
	_ = matchesB
	standingsB, err := svc.GetStageStandings(ctx, eventBID, stageB.ID)
	if err != nil {
		t.Fatalf("Event B + Stage B standings failed: %v", err)
	}
	_ = standingsB

	// 5. Non-existent Stage
	_, err = svc.ListMatches(ctx, eventAID, fakeStageID)
	if !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("fake stage matches expected ErrStageNotFound, got %v", err)
	}
	_, err = svc.GetStageStandings(ctx, eventAID, fakeStageID)
	if !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("fake stage standings expected ErrStageNotFound, got %v", err)
	}

	// 6. Non-existent Event
	_, err = svc.ListMatches(ctx, fakeEventID, stageA.ID)
	if !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("fake event matches expected ErrStageNotFound, got %v", err)
	}
	_, err = svc.GetStageStandings(ctx, fakeEventID, stageA.ID)
	if !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("fake event standings expected ErrStageNotFound, got %v", err)
	}
}

// Section 4: MATCH TRANSACTION ROLLBACK AUDIT
func TestAudit_Section4_MatchTransactionRollback(t *testing.T) {
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

	stage, _, err := svc.CreateStage(ctx, orgID, eventID, nil, "Elimination", StageSingleElimination, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	match1, err := svc.repo.CreateMatch(ctx, db.CreateCompetitionMatchParams{
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

	// Deterministic failure test:
	// Execute a transactional score update where downstream operation fails.
	forcedErr := errors.New("simulated downstream advancement crash")
	txErr := svc.repo.ExecTx(ctx, func(txRepo *Repository) error {
		// 1. First statement inside tx: update score
		_, err := txRepo.UpdateMatchScoreScoped(ctx, db.UpdateCompetitionMatchScoreScopedParams{
			ID:             match1.ID,
			HomeScore:      21,
			AwayScore:      15,
			MatchStatus:    "COMPLETED",
			WinnerID:       &p1.ID,
			ScoreDetails:   []byte("[]"),
			EventID:        eventID,
			OrganizationID: orgID,
		})
		if err != nil {
			return err
		}

		// 2. Downstream operation fails
		return forcedErr
	})

	if !errors.Is(txErr, forcedErr) {
		t.Fatalf("expected forced error, got %v", txErr)
	}

	// Verify database state: Match 1 MUST be rolled back to original state
	dbMatch, err := svc.repo.GetMatchByID(ctx, match1.ID)
	if err != nil {
		t.Fatalf("failed to fetch match: %v", err)
	}
	if dbMatch.HomeScore != 0 || dbMatch.AwayScore != 0 || dbMatch.MatchStatus != "SCHEDULED" || dbMatch.WinnerID != nil {
		t.Fatalf("TRANSACTION ROLLBACK FAILED: match was committed despite downstream failure! HomeScore=%d AwayScore=%d Status=%s Winner=%v",
			dbMatch.HomeScore, dbMatch.AwayScore, dbMatch.MatchStatus, dbMatch.WinnerID)
	}

	// Retry without failure: MUST succeed
	updated, err := svc.RecordMatchScore(ctx, orgID, eventID, match1.ID, 21, 15, "COMPLETED", &p1.ID, nil)
	if err != nil {
		t.Fatalf("retry after rollback failed: %v", err)
	}
	if updated.HomeScore != 21 || updated.MatchStatus != "COMPLETED" {
		t.Fatalf("unexpected match state after retry: %+v", updated)
	}
}

// Section 5: CYCLING TRANSACTION ROLLBACK AUDIT
func TestAudit_Section5_CyclingTransactionRollback(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)

	ctx := context.Background()

	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Rider 1", "R1", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Rider 2", "R2", nil, nil, nil)

	stage, _, err := svc.CreateStage(ctx, orgID, eventID, nil, "Cycling Stage 1", StagePelotonStage, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create cycling stage: %v", err)
	}

	// Provide 2 valid riders, but 3rd rider has non-existent EntryID
	bogusEntryID := uuid.New()
	records := []CyclingFinishRecord{
		{EntryID: p1.ID, RawFinishMs: 3600000, Status: "FINISHED"},
		{EntryID: p2.ID, RawFinishMs: 3600500, Status: "FINISHED"},
		{EntryID: bogusEntryID, RawFinishMs: 3601000, Status: "FINISHED"},
	}

	_, err = svc.ProcessCyclingStageResults(ctx, orgID, eventID, stage.ID, 1000, records)
	if !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("expected ErrEntryNotFound for bogus rider, got %v", err)
	}

	// Verify database: ZERO results exist for this stage!
	resultsInDB, err := svc.repo.ListResultsByStage(ctx, stage.ID)
	if err != nil {
		t.Fatalf("failed to query results: %v", err)
	}
	if len(resultsInDB) != 0 {
		t.Fatalf("TRANSACTION DEFECT: partial official results leaked into DB despite failure! Count=%d", len(resultsInDB))
	}

	// Retry with 2 valid riders: MUST succeed
	validRecords := []CyclingFinishRecord{
		{EntryID: p1.ID, RawFinishMs: 3600000, Status: "FINISHED"},
		{EntryID: p2.ID, RawFinishMs: 3600500, Status: "FINISHED"},
	}
	res, err := svc.ProcessCyclingStageResults(ctx, orgID, eventID, stage.ID, 1000, validRecords)
	if err != nil {
		t.Fatalf("retry after fix failed: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}

	resultsInDB, err = svc.repo.ListResultsByStage(ctx, stage.ID)
	if err != nil || len(resultsInDB) != 2 {
		t.Fatalf("expected exactly 2 results in DB after retry, got %d (err: %v)", len(resultsInDB), err)
	}
}

// Section 6: CYCLING IDEMPOTENCY AUDIT
func TestAudit_Section6_CyclingIdempotency(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)

	ctx := context.Background()

	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Rider 1", "R1", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Rider 2", "R2", nil, nil, nil)
	p3, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Rider 3", "R3", nil, nil, nil)

	stage, _, err := svc.CreateStage(ctx, orgID, eventID, nil, "Cycling Stage Idempotency", StagePelotonStage, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	records := []CyclingFinishRecord{
		{EntryID: p1.ID, RawFinishMs: 3600000, Status: "FINISHED"},
		{EntryID: p2.ID, RawFinishMs: 3600400, Status: "FINISHED"}, // bunch with p1
		{EntryID: p3.ID, RawFinishMs: 3602500, Status: "FINISHED"}, // gap > 1s -> new group
	}

	// Run 1
	res1, err := svc.ProcessCyclingStageResults(ctx, orgID, eventID, stage.ID, 1000, records)
	if err != nil {
		t.Fatalf("run 1 failed: %v", err)
	}

	// Run 2 (identical observations)
	res2, err := svc.ProcessCyclingStageResults(ctx, orgID, eventID, stage.ID, 1000, records)
	if err != nil {
		t.Fatalf("run 2 failed: %v", err)
	}

	// Verify lengths match
	if len(res1) != len(res2) {
		t.Fatalf("results length mismatch: run1=%d run2=%d", len(res1), len(res2))
	}

	// Verify exact equivalence of every item
	for i := range res1 {
		if res1[i].EntryID != res2[i].EntryID ||
			res1[i].RankOverall != res2[i].RankOverall ||
			res1[i].BunchTimeMs != res2[i].BunchTimeMs ||
			res1[i].GapMs != res2[i].GapMs ||
			res1[i].PelotonGroup != res2[i].PelotonGroup {
			t.Fatalf("idempotency mismatch at index %d: res1=%+v res2=%+v", i, res1[i], res2[i])
		}
	}

	// Verify DB contains exactly 3 results (no duplicate rows created)
	dbRows, err := svc.repo.ListResultsByStage(ctx, stage.ID)
	if err != nil {
		t.Fatalf("failed to list results: %v", err)
	}
	if len(dbRows) != 3 {
		t.Fatalf("IDEMPOTENCY DEFECT: duplicate result rows created! Count=%d", len(dbRows))
	}
}

// Section 7: FORFEIT / WALKOVER ADVANCEMENT AUDIT
func TestAudit_Section7_ForfeitWalkoverAdvancement(t *testing.T) {
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
		t.Fatalf("failed to create stage: %v", err)
	}

	var r1m1, r1m2 *CompetitionMatchView
	for i := range matches {
		if matches[i].RoundNumber == 1 && matches[i].MatchNumber == 1 {
			r1m1 = &matches[i]
		} else if matches[i].RoundNumber == 1 && matches[i].MatchNumber == 2 {
			r1m2 = &matches[i]
		}
	}

	// 1. CANCELLED match -> does NOT advance
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 0, 0, "CANCELLED", nil, nil)
	if err != nil {
		t.Fatalf("failed to record CANCELLED: %v", err)
	}
	updatedMatches, _ := svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.HomeEntryID != nil {
				t.Fatalf("CANCELLED match advanced home participant to R2: %v", m.HomeEntryID)
			}
		}
	}

	// 2. FORFEIT: Seed 1 wins by forfeit over Seed 2
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 0, 0, "FORFEIT", &p1.ID, nil)
	if err != nil {
		t.Fatalf("failed to record FORFEIT: %v", err)
	}
	updatedMatches, _ = svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.HomeEntryID == nil || *m.HomeEntryID != p1.ID {
				t.Fatalf("FORFEIT winner p1 failed to advance to R2: %v", m.HomeEntryID)
			}
		}
	}

	// 3. Repeated processing of the same FORFEIT must be idempotent
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 0, 0, "FORFEIT", &p1.ID, nil)
	if err != nil {
		t.Fatalf("repeated FORFEIT record failed: %v", err)
	}
	updatedMatches, _ = svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.HomeEntryID == nil || *m.HomeEntryID != p1.ID {
				t.Fatalf("idempotent FORFEIT corrupted R2 Home: %v", m.HomeEntryID)
			}
		}
	}

	// 4. SUSPENDED match in R1 M2 -> does NOT advance anyone
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m2.ID, 10, 8, "SUSPENDED", nil, nil)
	if err != nil {
		t.Fatalf("failed to record SUSPENDED: %v", err)
	}
	updatedMatches, _ = svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.AwayEntryID != nil {
				t.Fatalf("SUSPENDED match advanced away participant to R2: %v", m.AwayEntryID)
			}
		}
	}
}

// Section 8: BRACKET CONSISTENCY AFTER SCORE CORRECTION
func TestAudit_Section8_BracketConsistencyScoreCorrection(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)

	ctx := context.Background()

	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Athlete A", "PA", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Athlete B", "PB", nil, nil, nil)
	p3, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Athlete C", "PC", nil, nil, nil)
	p4, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Athlete D", "PD", nil, nil, nil)

	entryIDs := []uuid.UUID{p1.ID, p2.ID, p3.ID, p4.ID}
	stage, matches, err := svc.CreateStage(ctx, orgID, eventID, nil, "Bracket Correction", StageSingleElimination, 1, true, entryIDs)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	var r1m1 *CompetitionMatchView
	for i := range matches {
		if matches[i].RoundNumber == 1 && matches[i].MatchNumber == 1 {
			r1m1 = &matches[i]
		}
	}

	// 1. Initial Match Result: Athlete A defeats Athlete B (21 - 15)
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 21, 15, "COMPLETED", &p1.ID, nil)
	if err != nil {
		t.Fatalf("failed to record initial score: %v", err)
	}

	// Verify Athlete A in Round 2 Match 1 Home slot
	updatedMatches, _ := svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.HomeEntryID == nil || *m.HomeEntryID != p1.ID {
				t.Fatalf("expected Athlete A in R2, got %v", m.HomeEntryID)
			}
		}
	}

	// 2. Score Correction: Referee corrects error: Athlete B actually won (15 - 21)
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 15, 21, "COMPLETED", &p2.ID, nil)
	if err != nil {
		t.Fatalf("failed to record corrected score: %v", err)
	}

	// Verify Round 2 Match 1 Home slot is now updated to Athlete B
	updatedMatches, _ = svc.ListMatches(ctx, eventID, stage.ID)
	for _, m := range updatedMatches {
		if m.RoundNumber == 2 && m.MatchNumber == 1 {
			if m.HomeEntryID == nil || *m.HomeEntryID != p2.ID {
				t.Fatalf("SCORE CORRECTION DEFECT: R2 Home participant should be Athlete B, got %v", m.HomeEntryID)
			}
		}
	}
}

// Section 9: BWF RULE VERIFICATION AUDIT
func TestAudit_Section9_BWFMultiWayTieBreaking(t *testing.T) {
	e1ID := uuid.New()
	e2ID := uuid.New()
	e3ID := uuid.New()

	entries := []ParticipantEntryView{
		{ID: e1ID, DisplayName: "Player A", IdentifierCode: "PA"},
		{ID: e2ID, DisplayName: "Player B", IdentifierCode: "PB"},
		{ID: e3ID, DisplayName: "Player C", IdentifierCode: "PC"},
	}

	// 3-way circular tie: A beat B 2-0, B beat C 2-0, C beat A 2-1
	// By BWF GCR 16.2.3: Game Difference resolves:
	// A: 3 won, 2 lost (+1) -> Rank 1
	// B: 2 won, 2 lost (0)  -> Rank 2
	// C: 2 won, 3 lost (-1) -> Rank 3
	matches := []CompetitionMatchView{
		{
			RoundNumber: 1, MatchNumber: 1,
			HomeEntryID: &e1ID, AwayEntryID: &e2ID,
			HomeScore: 2, AwayScore: 0, MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{{PeriodNumber: 1, HomeScore: 21, AwayScore: 15}, {PeriodNumber: 2, HomeScore: 21, AwayScore: 15}},
		},
		{
			RoundNumber: 2, MatchNumber: 1,
			HomeEntryID: &e2ID, AwayEntryID: &e3ID,
			HomeScore: 2, AwayScore: 0, MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{{PeriodNumber: 1, HomeScore: 21, AwayScore: 15}, {PeriodNumber: 2, HomeScore: 21, AwayScore: 15}},
		},
		{
			RoundNumber: 3, MatchNumber: 1,
			HomeEntryID: &e3ID, AwayEntryID: &e1ID,
			HomeScore: 2, AwayScore: 1, MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{{PeriodNumber: 1, HomeScore: 21, AwayScore: 19}, {PeriodNumber: 2, HomeScore: 15, AwayScore: 21}, {PeriodNumber: 3, HomeScore: 21, AwayScore: 17}},
		},
	}

	// Test Input Order Invariance: shuffle entries and matches, ranking MUST remain identical
	permutations := [][]ParticipantEntryView{
		{entries[0], entries[1], entries[2]},
		{entries[1], entries[2], entries[0]},
		{entries[2], entries[0], entries[1]},
		{entries[2], entries[1], entries[0]},
		{entries[0], entries[2], entries[1]},
		{entries[1], entries[0], entries[2]},
	}

	for pIdx, pEntries := range permutations {
		standings := CalculateBadmintonStandings(matches, pEntries)
		if len(standings) != 3 {
			t.Fatalf("[perm %d] expected 3 standings, got %d", pIdx, len(standings))
		}
		if standings[0].EntryID != e1ID || standings[0].Rank != 1 {
			t.Fatalf("[perm %d] Rank 1 must be Player A, got %s (Rank %d)", pIdx, standings[0].DisplayName, standings[0].Rank)
		}
		if standings[1].EntryID != e2ID || standings[1].Rank != 2 {
			t.Fatalf("[perm %d] Rank 2 must be Player B, got %s (Rank %d)", pIdx, standings[1].DisplayName, standings[1].Rank)
		}
		if standings[2].EntryID != e3ID || standings[2].Rank != 3 {
			t.Fatalf("[perm %d] Rank 3 must be Player C, got %s (Rank %d)", pIdx, standings[2].DisplayName, standings[2].Rank)
		}
	}

	// Test 4-way group with distinct match points
	e4ID := uuid.New()
	entries4 := append(entries, ParticipantEntryView{ID: e4ID, DisplayName: "Player D", IdentifierCode: "PD"})
	// Player D loses all 3 matches
	matches4 := append(matches,
		CompetitionMatchView{
			HomeEntryID: &e1ID, AwayEntryID: &e4ID,
			HomeScore: 2, AwayScore: 0, MatchStatus: "COMPLETED",
		},
	)
	standings4 := CalculateBadmintonStandings(matches4, entries4)
	if standings4[0].EntryID != e1ID || standings4[0].Rank != 1 {
		t.Fatalf("expected Player A to lead 4-way group with most wins, got %+v", standings4[0])
	}
}

// Section 10: STATUS SAFETY AUDIT
func TestAudit_Section10_StatusSafety(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)

	ctx := context.Background()

	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Ath 1", "A1", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Ath 2", "A2", nil, nil, nil)

	stage, _, err := svc.CreateStage(ctx, orgID, eventID, nil, "Status Test", StageSingleElimination, 1, false, nil)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	m, err := svc.repo.CreateMatch(ctx, db.CreateCompetitionMatchParams{
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

	invalidStatuses := []string{
		"",
		" ",
		"   ",
		"\t",
		"unknown",
		"invalid",
		"FINISHED", // not a valid match status (only COMPLETED, FORFEIT, LIVE, SCHEDULED, SUSPENDED, CANCELLED)
		"finished",
		"completed",
		"forfeit",
		"null",
		"NaN",
		"COMPLETED; DROP TABLE competition_matches; --",
	}

	for _, badStatus := range invalidStatuses {
		_, err := svc.RecordMatchScore(ctx, orgID, eventID, m.ID, 10, 5, badStatus, nil, nil)
		if !errors.Is(err, ErrInvalidStatus) {
			t.Fatalf("STATUS SAFETY DEFECT: invalid status %q did not produce ErrInvalidStatus, got: %v", badStatus, err)
		}

		// Ensure match was NOT updated to this bad status in the database
		dbM, _ := svc.repo.GetMatchByID(ctx, m.ID)
		if dbM.MatchStatus != "SCHEDULED" {
			t.Fatalf("STATUS SAFETY DEFECT: match status changed to %q in DB!", dbM.MatchStatus)
		}
	}
}
