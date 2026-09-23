package competitions

import (
	"testing"

	"github.com/google/uuid"
)

func TestGenerateRoundRobin_EvenTeams(t *testing.T) {
	teams := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	fixtures := GenerateRoundRobin(teams)

	// For 4 teams: 3 rounds, 2 matches per round = 6 matches
	if len(fixtures) != 6 {
		t.Fatalf("expected 6 matches for 4 teams, got %d", len(fixtures))
	}

	// Verify each team plays 3 times
	appearances := make(map[uuid.UUID]int)
	for _, f := range fixtures {
		if f.HomeEntryID != nil {
			appearances[*f.HomeEntryID]++
		}
		if f.AwayEntryID != nil {
			appearances[*f.AwayEntryID]++
		}
	}

	for _, team := range teams {
		if appearances[team] != 3 {
			t.Errorf("team %s appeared %d times, expected 3", team, appearances[team])
		}
	}
}

func TestGenerateRoundRobin_OddTeamsWithBye(t *testing.T) {
	teams := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	fixtures := GenerateRoundRobin(teams)

	// For 3 teams (+1 dummy bye = 4): 3 rounds, 1 match per round (the other team has bye) = 3 matches
	if len(fixtures) != 3 {
		t.Fatalf("expected 3 matches for 3 teams, got %d", len(fixtures))
	}

	appearances := make(map[uuid.UUID]int)
	for _, f := range fixtures {
		appearances[*f.HomeEntryID]++
		appearances[*f.AwayEntryID]++
	}

	for _, team := range teams {
		if appearances[team] != 2 {
			t.Errorf("team %s appeared %d times, expected 2", team, appearances[team])
		}
	}
}

func TestGenerateSingleElimination_2Seeds(t *testing.T) {
	seeds := []uuid.UUID{uuid.New(), uuid.New()}
	fixtures := GenerateSingleElimination(seeds)

	// 2 seeds -> 1 round, 1 match (Finals)
	if len(fixtures) != 1 {
		t.Fatalf("expected 1 match for 2 seeds, got %d", len(fixtures))
	}

	if *fixtures[0].HomeEntryID != seeds[0] || *fixtures[0].AwayEntryID != seeds[1] {
		t.Errorf("match 1 should pair seed 1 vs seed 2")
	}
	if fixtures[0].RoundNumber != 1 || fixtures[0].MatchNumber != 1 {
		t.Errorf("expected Round 1 Match 1, got R%d M%d", fixtures[0].RoundNumber, fixtures[0].MatchNumber)
	}
}

func TestGenerateSingleElimination_3SeedsWithBye(t *testing.T) {
	seeds := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	fixtures := GenerateSingleElimination(seeds)

	// 3 seeds (P=4) -> 2 rounds: 2 matches in R1 + 1 match in R2 = 3 matches
	if len(fixtures) != 3 {
		t.Fatalf("expected 3 matches for 3 seeds, got %d", len(fixtures))
	}

	// Round 1 Match 1: Seed 1 gets a bye
	m1 := fixtures[0]
	if !m1.IsBye || m1.MatchStatus != "BYE" || *m1.WinnerID != seeds[0] {
		t.Errorf("R1 M1 should be a bye won by seed 1, got isBye=%v, status=%s", m1.IsBye, m1.MatchStatus)
	}

	// Round 1 Match 2: Seed 2 vs Seed 3
	m2 := fixtures[1]
	if m2.IsBye || *m2.HomeEntryID != seeds[1] || *m2.AwayEntryID != seeds[2] {
		t.Errorf("R1 M2 should pair seed 2 vs seed 3")
	}

	// Round 2 Match 1 (Final): Seed 1 pre-placed into Home from R1 M1 bye
	finalMatch := fixtures[2]
	if finalMatch.RoundNumber != 2 || finalMatch.MatchNumber != 1 {
		t.Errorf("expected R2 M1 for final, got R%d M%d", finalMatch.RoundNumber, finalMatch.MatchNumber)
	}
	if finalMatch.HomeEntryID == nil || *finalMatch.HomeEntryID != seeds[0] {
		t.Errorf("final match home slot should be pre-filled with seed 1 from bye")
	}
	if finalMatch.AwayEntryID != nil {
		t.Errorf("final match away slot should be nil pending R1 M2 result")
	}
	if finalMatch.SourceHomeMatchNumber != 1 || finalMatch.SourceAwayMatchNumber != 2 {
		t.Errorf("final match parentage should be M1 and M2, got %d and %d", finalMatch.SourceHomeMatchNumber, finalMatch.SourceAwayMatchNumber)
	}
}

func TestGenerateSingleElimination_4Seeds(t *testing.T) {
	seeds := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	fixtures := GenerateSingleElimination(seeds)

	// 4 seeds -> 2 rounds: 2 in R1, 1 in R2 = 3 matches
	if len(fixtures) != 3 {
		t.Fatalf("expected 3 matches for 4 seeds, got %d", len(fixtures))
	}

	// R1 M1: Seed 1 vs Seed 4
	if *fixtures[0].HomeEntryID != seeds[0] || *fixtures[0].AwayEntryID != seeds[3] {
		t.Errorf("match 1 should pair seed 1 vs seed 4")
	}

	// R1 M2: Seed 2 vs Seed 3
	if *fixtures[1].HomeEntryID != seeds[1] || *fixtures[1].AwayEntryID != seeds[2] {
		t.Errorf("match 2 should pair seed 2 vs seed 3")
	}

	// R2 M1: Final between winners of M1 and M2
	if fixtures[2].RoundNumber != 2 || fixtures[2].MatchNumber != 1 {
		t.Errorf("expected R2 M1, got R%d M%d", fixtures[2].RoundNumber, fixtures[2].MatchNumber)
	}
	if fixtures[2].SourceHomeMatchNumber != 1 || fixtures[2].SourceAwayMatchNumber != 2 {
		t.Errorf("expected source matches 1 and 2, got %d and %d", fixtures[2].SourceHomeMatchNumber, fixtures[2].SourceAwayMatchNumber)
	}
}

func TestGenerateSingleElimination_5Seeds(t *testing.T) {
	seeds := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	fixtures := GenerateSingleElimination(seeds)

	// 5 seeds (P=8) -> 3 rounds: 4 in R1 + 2 in R2 + 1 in R3 = 7 matches
	if len(fixtures) != 7 {
		t.Fatalf("expected 7 matches for 5 seeds, got %d", len(fixtures))
	}

	// In R1:
	// M1: 1 vs 8 (bye for 1)
	if !fixtures[0].IsBye || *fixtures[0].WinnerID != seeds[0] {
		t.Errorf("R1 M1 should be bye for seed 1")
	}
	// M2: 4 vs 5 (scheduled)
	if fixtures[1].IsBye || *fixtures[1].HomeEntryID != seeds[3] || *fixtures[1].AwayEntryID != seeds[4] {
		t.Errorf("R1 M2 should pair seed 4 vs seed 5")
	}
	// M3: 2 vs 7 (bye for 2)
	if !fixtures[2].IsBye || *fixtures[2].WinnerID != seeds[1] {
		t.Errorf("R1 M3 should be bye for seed 2")
	}
	// M4: 3 vs 6 (bye for 3)
	if !fixtures[3].IsBye || *fixtures[3].WinnerID != seeds[2] {
		t.Errorf("R1 M4 should be bye for seed 3")
	}

	// In R2:
	// M1: Home is seed 1 (bye), Away is nil (waits for 4 vs 5)
	if fixtures[4].HomeEntryID == nil || *fixtures[4].HomeEntryID != seeds[0] {
		t.Errorf("R2 M1 Home should be seed 1")
	}
	if fixtures[4].AwayEntryID != nil {
		t.Errorf("R2 M1 Away should be nil pending R1 M2")
	}
	// M2: Home is seed 2 (bye), Away is seed 3 (bye)
	if fixtures[5].HomeEntryID == nil || *fixtures[5].HomeEntryID != seeds[1] {
		t.Errorf("R2 M2 Home should be seed 2")
	}
	if fixtures[5].AwayEntryID == nil || *fixtures[5].AwayEntryID != seeds[2] {
		t.Errorf("R2 M2 Away should be seed 3")
	}
}

func TestGenerateSingleElimination_8Seeds(t *testing.T) {
	seeds := make([]uuid.UUID, 8)
	for i := range seeds {
		seeds[i] = uuid.New()
	}
	fixtures := GenerateSingleElimination(seeds)

	// 8 seeds -> 4 in R1 + 2 in R2 + 1 in R3 = 7 matches
	if len(fixtures) != 7 {
		t.Fatalf("expected 7 matches for 8 seeds, got %d", len(fixtures))
	}

	// Pairings:
	// M1: 1 vs 8
	if *fixtures[0].HomeEntryID != seeds[0] || *fixtures[0].AwayEntryID != seeds[7] {
		t.Errorf("R1 M1 should pair 1 vs 8")
	}
	// M2: 4 vs 5
	if *fixtures[1].HomeEntryID != seeds[3] || *fixtures[1].AwayEntryID != seeds[4] {
		t.Errorf("R1 M2 should pair 4 vs 5")
	}
	// M3: 2 vs 7
	if *fixtures[2].HomeEntryID != seeds[1] || *fixtures[2].AwayEntryID != seeds[6] {
		t.Errorf("R1 M3 should pair 2 vs 7")
	}
	// M4: 3 vs 6
	if *fixtures[3].HomeEntryID != seeds[2] || *fixtures[3].AwayEntryID != seeds[5] {
		t.Errorf("R1 M4 should pair 3 vs 6")
	}
}

func TestGenerateSingleElimination_16Seeds(t *testing.T) {
	seeds := make([]uuid.UUID, 16)
	for i := range seeds {
		seeds[i] = uuid.New()
	}
	fixtures := GenerateSingleElimination(seeds)

	// 16 seeds -> 8 in R1 + 4 in R2 + 2 in R3 + 1 in R4 = 15 matches
	if len(fixtures) != 15 {
		t.Fatalf("expected 15 matches for 16 seeds, got %d", len(fixtures))
	}

	// Verify round counts
	roundCounts := make(map[int]int)
	for _, f := range fixtures {
		roundCounts[f.RoundNumber]++
	}
	if roundCounts[1] != 8 || roundCounts[2] != 4 || roundCounts[3] != 2 || roundCounts[4] != 1 {
		t.Errorf("unexpected round counts: %+v", roundCounts)
	}
}

func TestAssignAquaticsLanes_Spearhead(t *testing.T) {
	// 8 swimmers with qualifying times from 21.50s to 25.00s
	seeds := []SwimmerSeed{
		{EntryID: uuid.New(), SeedTimeMs: 25000}, // 8th
		{EntryID: uuid.New(), SeedTimeMs: 22000}, // 2nd
		{EntryID: uuid.New(), SeedTimeMs: 21500}, // 1st (fastest)
		{EntryID: uuid.New(), SeedTimeMs: 22500}, // 3rd
		{EntryID: uuid.New(), SeedTimeMs: 23000}, // 4th
		{EntryID: uuid.New(), SeedTimeMs: 23500}, // 5th
		{EntryID: uuid.New(), SeedTimeMs: 24000}, // 6th
		{EntryID: uuid.New(), SeedTimeMs: 24500}, // 7th
	}

	assigned := AssignAquaticsLanes(seeds, 8)
	if len(assigned) != 8 {
		t.Fatalf("expected 8 assigned swimmers, got %d", len(assigned))
	}

	// In an 8-lane pool, spearhead order is: Lane 4 (fastest), Lane 5 (2nd), Lane 3 (3rd), Lane 6 (4th), etc.
	laneByTime := make(map[int64]int)
	for _, s := range assigned {
		laneByTime[s.SeedTimeMs] = s.LaneNumber
	}

	if laneByTime[21500] != 4 {
		t.Errorf("fastest swimmer (21.50s) should be in Lane 4, got Lane %d", laneByTime[21500])
	}
	if laneByTime[22000] != 5 {
		t.Errorf("2nd swimmer (22.00s) should be in Lane 5, got Lane %d", laneByTime[22000])
	}
	if laneByTime[22500] != 3 {
		t.Errorf("3rd swimmer (22.50s) should be in Lane 3, got Lane %d", laneByTime[22500])
	}
	if laneByTime[23000] != 6 {
		t.Errorf("4th swimmer (23.00s) should be in Lane 6, got Lane %d", laneByTime[23000])
	}
}

func TestAssignAquaticsLanes_6LanePool(t *testing.T) {
	// 6-lane pool: 3, 4, 2, 5, 1, 6
	seeds := []SwimmerSeed{
		{EntryID: uuid.New(), SeedTimeMs: 24000}, // 4th -> Lane 5
		{EntryID: uuid.New(), SeedTimeMs: 22000}, // 2nd -> Lane 4
		{EntryID: uuid.New(), SeedTimeMs: 21000}, // 1st -> Lane 3
		{EntryID: uuid.New(), SeedTimeMs: 23000}, // 3rd -> Lane 2
		{EntryID: uuid.New(), SeedTimeMs: 25000}, // 5th -> Lane 1
		{EntryID: uuid.New(), SeedTimeMs: 26000}, // 6th -> Lane 6
	}

	assigned := AssignAquaticsLanes(seeds, 6)
	if len(assigned) != 6 {
		t.Fatalf("expected 6 swimmers, got %d", len(assigned))
	}

	laneByTime := make(map[int64]int)
	for _, s := range assigned {
		laneByTime[s.SeedTimeMs] = s.LaneNumber
	}

	if laneByTime[21000] != 3 {
		t.Errorf("1st swimmer in 6-lane pool should be in Lane 3, got %d", laneByTime[21000])
	}
	if laneByTime[22000] != 4 {
		t.Errorf("2nd swimmer in 6-lane pool should be in Lane 4, got %d", laneByTime[22000])
	}
	if laneByTime[23000] != 2 {
		t.Errorf("3rd swimmer in 6-lane pool should be in Lane 2, got %d", laneByTime[23000])
	}
	if laneByTime[24000] != 5 {
		t.Errorf("4th swimmer in 6-lane pool should be in Lane 5, got %d", laneByTime[24000])
	}
	if laneByTime[25000] != 1 {
		t.Errorf("5th swimmer in 6-lane pool should be in Lane 1, got %d", laneByTime[25000])
	}
	if laneByTime[26000] != 6 {
		t.Errorf("6th swimmer in 6-lane pool should be in Lane 6, got %d", laneByTime[26000])
	}
}

func TestAssignAquaticsLanes_4LanePool(t *testing.T) {
	// 4-lane pool: 2, 3, 1, 4
	seeds := []SwimmerSeed{
		{EntryID: uuid.New(), SeedTimeMs: 22000}, // 2nd -> Lane 3
		{EntryID: uuid.New(), SeedTimeMs: 21000}, // 1st -> Lane 2
		{EntryID: uuid.New(), SeedTimeMs: 23000}, // 3rd -> Lane 1
		{EntryID: uuid.New(), SeedTimeMs: 24000}, // 4th -> Lane 4
	}

	assigned := AssignAquaticsLanes(seeds, 4)
	laneByTime := make(map[int64]int)
	for _, s := range assigned {
		laneByTime[s.SeedTimeMs] = s.LaneNumber
	}

	if laneByTime[21000] != 2 {
		t.Errorf("1st swimmer in 4-lane pool should be in Lane 2, got %d", laneByTime[21000])
	}
	if laneByTime[22000] != 3 {
		t.Errorf("2nd swimmer in 4-lane pool should be in Lane 3, got %d", laneByTime[22000])
	}
	if laneByTime[23000] != 1 {
		t.Errorf("3rd swimmer in 4-lane pool should be in Lane 1, got %d", laneByTime[23000])
	}
	if laneByTime[24000] != 4 {
		t.Errorf("4th swimmer in 4-lane pool should be in Lane 4, got %d", laneByTime[24000])
	}
}
