package competitions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TestHardening_DownstreamMatchLocking verifies that when a downstream match is in an active
// or completed state (LIVE, COMPLETED, FORFEIT, WALKOVER, BYE, SUSPENDED, CANCELLED),
// any score modification to upstream feeder matches is strictly rejected and rolled back.
func TestHardening_DownstreamMatchLocking(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)

	ctx := context.Background()

	// 1. Create 4 participants
	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 1", "S1", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 2", "S2", nil, nil, nil)
	p3, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 3", "S3", nil, nil, nil)
	p4, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 4", "S4", nil, nil, nil)

	// 2. Create single elimination bracket stage with auto-generated fixtures (4 seeds -> 2 rounds: R1M1, R1M2, R2M1)
	entryIDs := []uuid.UUID{p1.ID, p2.ID, p3.ID, p4.ID}
	_, matches, err := svc.CreateStage(ctx, orgID, eventID, nil, "Championship Knockout", StageSingleElimination, 1, true, entryIDs)
	if err != nil {
		t.Fatalf("failed to create knockout stage: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches for 4 participants, got %d", len(matches))
	}

	var r1m1, r2m1 CompetitionMatchView
	for _, m := range matches {
		if m.RoundNumber == 1 && m.MatchNumber == 1 {
			r1m1 = m
		} else if m.RoundNumber == 2 && m.MatchNumber == 1 {
			r2m1 = m
		}
	}

	// Step A: Complete R1M1 with Seed 1 winning (21-15).
	// Downstream match R2M1 is SCHEDULED, so advancement must succeed.
	r1m1Updated, err := svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 21, 15, "COMPLETED", &p1.ID, nil)
	if err != nil {
		t.Fatalf("step A failed to record R1M1 score: %v", err)
	}
	if r1m1Updated.MatchStatus != "COMPLETED" || *r1m1Updated.WinnerID != p1.ID {
		t.Fatalf("step A unexpected R1M1 state: %+v", r1m1Updated)
	}

	// Verify R2M1 now has Seed 1 in Home slot
	r2m1DB, err := svc.repo.GetMatchByID(ctx, r2m1.ID)
	if err != nil {
		t.Fatalf("failed to fetch R2M1: %v", err)
	}
	if r2m1DB.HomeEntryID == nil || *r2m1DB.HomeEntryID != p1.ID {
		t.Fatalf("step A expected R2M1 HomeEntryID to be %s, got %v", p1.ID, r2m1DB.HomeEntryID)
	}
	if r2m1DB.MatchStatus != "SCHEDULED" {
		t.Fatalf("expected R2M1 to remain SCHEDULED, got %s", r2m1DB.MatchStatus)
	}

	// Step B: Correction while downstream R2M1 is still SCHEDULED.
	// Organizer corrects score: Seed 2 actually won (15-21).
	// Because R2M1 is still SCHEDULED, correction must succeed and overwrite downstream slot.
	r1m1Corrected, err := svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 15, 21, "COMPLETED", &p2.ID, nil)
	if err != nil {
		t.Fatalf("step B score correction failed while downstream was SCHEDULED: %v", err)
	}
	if *r1m1Corrected.WinnerID != p2.ID {
		t.Fatalf("step B expected corrected winner to be Seed 2, got %v", r1m1Corrected.WinnerID)
	}

	// Verify R2M1 home slot was updated to Seed 2
	r2m1DB, err = svc.repo.GetMatchByID(ctx, r2m1.ID)
	if err != nil {
		t.Fatalf("failed to fetch R2M1 after correction: %v", err)
	}
	if r2m1DB.HomeEntryID == nil || *r2m1DB.HomeEntryID != p2.ID {
		t.Fatalf("step B expected R2M1 HomeEntryID to be updated to %s, got %v", p2.ID, r2m1DB.HomeEntryID)
	}

	// Step C: Now transition downstream match R2M1 to LIVE.
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r2m1.ID, 5, 2, "LIVE", nil, nil)
	if err != nil {
		t.Fatalf("failed to set R2M1 to LIVE: %v", err)
	}

	// Step D: Adversarial attack: Attempt to overwrite R1M1 while R2M1 is LIVE.
	// Must fail with ErrDownstreamMatchLocked!
	_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 21, 10, "COMPLETED", &p1.ID, nil)
	if !errors.Is(err, ErrDownstreamMatchLocked) {
		t.Fatalf("DEFECT: expected ErrDownstreamMatchLocked when downstream is LIVE, got %v", err)
	}

	// Verify transaction rollback: R1M1 score in DB must remain the old score (15-21, Seed 2)
	r1m1PostAttack, err := svc.repo.GetMatchByID(ctx, r1m1.ID)
	if err != nil {
		t.Fatalf("failed to get R1M1 after rejected attack: %v", err)
	}
	if r1m1PostAttack.HomeScore != 15 || r1m1PostAttack.AwayScore != 21 || *r1m1PostAttack.WinnerID != p2.ID {
		t.Fatalf("DEFECT: R1M1 was modified despite transaction failure! Score: %d-%d, Winner: %v",
			r1m1PostAttack.HomeScore, r1m1PostAttack.AwayScore, r1m1PostAttack.WinnerID)
	}

	// Verify R2M1 participants were NOT corrupted
	r2m1PostAttack, err := svc.repo.GetMatchByID(ctx, r2m1.ID)
	if err != nil {
		t.Fatalf("failed to get R2M1 after rejected attack: %v", err)
	}
	if *r2m1PostAttack.HomeEntryID != p2.ID {
		t.Fatalf("DEFECT: R2M1 HomeEntryID was altered! Expected %s, got %v", p2.ID, r2m1PostAttack.HomeEntryID)
	}

	// Step E: Test all non-SCHEDULED downstream states (COMPLETED, FORFEIT, WALKOVER, SUSPENDED, CANCELLED)
	lockedStatuses := []string{"COMPLETED", "FORFEIT", "WALKOVER", "SUSPENDED", "CANCELLED"}
	for _, lockStat := range lockedStatuses {
		// Set R2M1 to locked status
		_, err = svc.RecordMatchScore(ctx, orgID, eventID, r2m1.ID, 21, 19, lockStat, &p2.ID, nil)
		if err != nil {
			t.Fatalf("failed to set R2M1 to %s: %v", lockStat, err)
		}

		// Attempt to update R1M1
		_, err = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 21, 0, "COMPLETED", &p1.ID, nil)
		if !errors.Is(err, ErrDownstreamMatchLocked) {
			t.Fatalf("DEFECT: expected ErrDownstreamMatchLocked when downstream is %s, got: %v", lockStat, err)
		}
	}

	// Step F: Final round match (R2M1) score update must succeed without downstream errors
	// because final round has no downstream match.
	finalUpdated, err := svc.RecordMatchScore(ctx, orgID, eventID, r2m1.ID, 21, 18, "COMPLETED", &p2.ID, nil)
	if err != nil {
		t.Fatalf("final round score update should succeed, got %v", err)
	}
	if finalUpdated.MatchStatus != "COMPLETED" {
		t.Fatalf("expected final round status COMPLETED, got %s", finalUpdated.MatchStatus)
	}
}

// TestHardening_ByeAndWalkoverFirstClass verifies first-class handling of BYE and WALKOVER states.
func TestHardening_ByeAndWalkoverFirstClass(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)

	ctx := context.Background()

	// 1. Create 3 participants (causing a BYE in a 4-bracket single elimination)
	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 1", "S1", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 2", "S2", nil, nil, nil)
	p3, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "Seed 3", "S3", nil, nil, nil)

	entryIDs := []uuid.UUID{p1.ID, p2.ID, p3.ID}
	_, matches, err := svc.CreateStage(ctx, orgID, eventID, nil, "3-Seed Tournament", StageSingleElimination, 1, true, entryIDs)
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	var byeMatch, playMatch, finalMatch CompetitionMatchView
	for _, m := range matches {
		if m.RoundNumber == 1 && m.MatchNumber == 1 {
			byeMatch = m
		} else if m.RoundNumber == 1 && m.MatchNumber == 2 {
			playMatch = m
		} else if m.RoundNumber == 2 && m.MatchNumber == 1 {
			finalMatch = m
		}
	}

	// 2. Verify BYE is first-class
	if byeMatch.MatchStatus != "BYE" {
		t.Fatalf("expected bye match status to be BYE, got %s", byeMatch.MatchStatus)
	}
	if byeMatch.AwayEntryID != nil {
		t.Fatalf("expected bye match AwayEntryID to be nil, got %v", byeMatch.AwayEntryID)
	}
	if byeMatch.WinnerID == nil || *byeMatch.WinnerID != p1.ID {
		t.Fatalf("expected bye match winner to be Seed 1, got %v", byeMatch.WinnerID)
	}

	// Verify Seed 1 was pre-advanced to Final
	if finalMatch.HomeEntryID == nil || *finalMatch.HomeEntryID != p1.ID {
		t.Fatalf("expected final match HomeEntryID to be pre-advanced Seed 1, got %v", finalMatch.HomeEntryID)
	}
	if finalMatch.AwayEntryID != nil {
		t.Fatalf("expected final match AwayEntryID to be nil pending R1M2, got %v", finalMatch.AwayEntryID)
	}

	// 3. Test WALKOVER state on R1M2
	// Seed 3 does not show up. Seed 2 is awarded a WALKOVER.
	walkoverMatch, err := svc.RecordMatchScore(ctx, orgID, eventID, playMatch.ID, 0, 0, "WALKOVER", &p2.ID, nil)
	if err != nil {
		t.Fatalf("failed to record WALKOVER match: %v", err)
	}
	if walkoverMatch.MatchStatus != "WALKOVER" {
		t.Fatalf("expected match status WALKOVER, got %s", walkoverMatch.MatchStatus)
	}
	if walkoverMatch.WinnerID == nil || *walkoverMatch.WinnerID != p2.ID {
		t.Fatalf("expected walkover winner to be Seed 2, got %v", walkoverMatch.WinnerID)
	}

	// 4. Verify WALKOVER winner advanced to Final Away slot
	finalDB, err := svc.repo.GetMatchByID(ctx, finalMatch.ID)
	if err != nil {
		t.Fatalf("failed to get final match: %v", err)
	}
	if finalDB.AwayEntryID == nil || *finalDB.AwayEntryID != p2.ID {
		t.Fatalf("expected final match AwayEntryID to be Seed 2 from WALKOVER, got %v", finalDB.AwayEntryID)
	}

	// 5. Final match: Seed 1 vs Seed 2 completed normally
	completedFinal, err := svc.RecordMatchScore(ctx, orgID, eventID, finalMatch.ID, 21, 19, "COMPLETED", &p1.ID, nil)
	if err != nil {
		t.Fatalf("failed to complete final match: %v", err)
	}
	if completedFinal.MatchStatus != "COMPLETED" || *completedFinal.WinnerID != p1.ID {
		t.Fatalf("expected final champion Seed 1, got %+v", completedFinal)
	}
}

// TestHardening_HandlerConflictMapping verifies that the HTTP handler returns 409 Conflict
// when ErrDownstreamMatchLocked is returned from the service.
func TestHardening_HandlerConflictMapping(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	svc, orgID, eventID := setupTestContext(t, pool)
	defer cleanupTestContext(pool, orgID, eventID)

	ctx := context.Background()

	p1, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "A", "A", nil, nil, nil)
	p2, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "B", "B", nil, nil, nil)
	p3, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "C", "C", nil, nil, nil)
	p4, _ := svc.RegisterParticipantEntry(ctx, orgID, eventID, uuid.Nil, nil, nil, EntryIndividual, "D", "D", nil, nil, nil)

	_, matches, err := svc.CreateStage(ctx, orgID, eventID, nil, "Handler Test Stage", StageSingleElimination, 1, true, []uuid.UUID{p1.ID, p2.ID, p3.ID, p4.ID})
	if err != nil {
		t.Fatalf("failed to create stage: %v", err)
	}

	var r1m1, r2m1 CompetitionMatchView
	for _, m := range matches {
		if m.RoundNumber == 1 && m.MatchNumber == 1 {
			r1m1 = m
		} else if m.RoundNumber == 2 && m.MatchNumber == 1 {
			r2m1 = m
		}
	}

	// Advance R1M1 and then put R2M1 to LIVE
	_, _ = svc.RecordMatchScore(ctx, orgID, eventID, r1m1.ID, 21, 10, "COMPLETED", &p1.ID, nil)
	_, _ = svc.RecordMatchScore(ctx, orgID, eventID, r2m1.ID, 0, 0, "LIVE", nil, nil)

	// Now send HTTP request to update R1M1 score
	h := NewHandler(svc)
	r := chi.NewRouter()
	r.Post("/orgs/{orgId}/events/{eventId}/matches/{matchId}/score", h.RecordMatchScore)

	body, _ := json.Marshal(RecordMatchScoreRequest{
		HomeScore: 10,
		AwayScore: 21,
		Status:    "COMPLETED",
		WinnerID:  &p2.ID,
	})

	req := httptest.NewRequest("POST", "/orgs/"+orgID.String()+"/events/"+eventID.String()+"/matches/"+r1m1.ID.String()+"/score", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected HTTP 409 Conflict, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error.Code != "DOWNSTREAM_MATCH_LOCKED" {
		t.Fatalf("expected error code DOWNSTREAM_MATCH_LOCKED, got %v", resp.Error.Code)
	}
}
