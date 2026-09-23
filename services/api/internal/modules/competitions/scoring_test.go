package competitions

import (
	"testing"

	"github.com/google/uuid"
)

func TestBadmintonScoring(t *testing.T) {
	// Game 1: Normal 21-15 (Home wins)
	// Game 2: Deuce 22-20 (Away wins)
	// Game 3: Sudden death at 29-29 -> 30-29 (Home wins)
	sets := []MatchScoreDetail{
		{PeriodNumber: 1, HomeScore: 21, AwayScore: 15},
		{PeriodNumber: 2, HomeScore: 20, AwayScore: 22},
		{PeriodNumber: 3, HomeScore: 30, AwayScore: 29},
	}

	outcome := EvaluateBadmintonMatch(sets)
	if outcome.HomeScore != 2 || outcome.AwayScore != 1 {
		t.Fatalf("expected 2-1 games, got %d-%d", outcome.HomeScore, outcome.AwayScore)
	}
	if outcome.MatchStatus != "COMPLETED" {
		t.Fatalf("expected COMPLETED status, got %s", outcome.MatchStatus)
	}
	if outcome.WinnerEntry == nil || *outcome.WinnerEntry != "HOME" {
		t.Fatalf("expected HOME winner, got %v", outcome.WinnerEntry)
	}
}

func TestBadmintonDeuceRules(t *testing.T) {
	// At 20-20, 21-20 does NOT win the game (must lead by 2)
	setsIncomplete := []MatchScoreDetail{
		{PeriodNumber: 1, HomeScore: 21, AwayScore: 20},
	}
	outcome := EvaluateBadmintonMatch(setsIncomplete)
	if outcome.HomeScore != 0 {
		t.Fatalf("at 21-20 in deuce, game is not over; got %d games", outcome.HomeScore)
	}
}

func TestFootballStandingsCalculation(t *testing.T) {
	teamA := uuid.New()
	teamB := uuid.New()
	teamC := uuid.New()

	entries := []ParticipantEntryView{
		{ID: teamA, DisplayName: "Garuda FC", Status: StatusActive},
		{ID: teamB, DisplayName: "Merak FC", Status: StatusActive},
		{ID: teamC, DisplayName: "Elang FC", Status: StatusActive},
	}

	// Match 1: Team A 2 - 1 Team B (A gets 3 pts)
	// Match 2: Team B 3 - 0 Team C (B gets 3 pts)
	// Match 3: Team C 1 - 0 Team A (C gets 3 pts)
	// All 3 teams have 3 points:
	// Team A: GF 2, GA 2, GD 0
	// Team B: GF 4, GA 2, GD +2
	// Team C: GF 1, GA 3, GD -2
	matches := []CompetitionMatchView{
		{
			HomeEntryID: &teamA,
			AwayEntryID: &teamB,
			HomeScore:   2,
			AwayScore:   1,
			MatchStatus: "COMPLETED",
		},
		{
			HomeEntryID: &teamB,
			AwayEntryID: &teamC,
			HomeScore:   3,
			AwayScore:   0,
			MatchStatus: "COMPLETED",
		},
		{
			HomeEntryID: &teamC,
			AwayEntryID: &teamA,
			HomeScore:   1,
			AwayScore:   0,
			MatchStatus: "COMPLETED",
		},
	}

	standings := CalculateFootballStandings(matches, entries)
	if len(standings) != 3 {
		t.Fatalf("expected 3 teams in standings, got %d", len(standings))
	}

	// Team B should be Rank 1 due to +2 GD
	if standings[0].EntryID != teamB {
		t.Errorf("Rank 1: expected Team B (Merak FC) with +2 GD, got %s", standings[0].DisplayName)
	}
	if standings[0].Points != 3 || standings[0].GoalDifference != 2 {
		t.Errorf("Rank 1 stats incorrect: pts=%d, gd=%d", standings[0].Points, standings[0].GoalDifference)
	}

	// Team A should be Rank 2 with 0 GD
	if standings[1].EntryID != teamA {
		t.Errorf("Rank 2: expected Team A (Garuda FC) with 0 GD, got %s", standings[1].DisplayName)
	}

	// Team C should be Rank 3 with -2 GD
	if standings[2].EntryID != teamC {
		t.Errorf("Rank 3: expected Team C (Elang FC) with -2 GD, got %s", standings[2].DisplayName)
	}
}

func TestFootballHeadToHeadTiebreaker(t *testing.T) {
	teamA := uuid.New()
	teamB := uuid.New()

	entries := []ParticipantEntryView{
		{ID: teamA, DisplayName: "Team A", Status: StatusActive},
		{ID: teamB, DisplayName: "Team B", Status: StatusActive},
	}

	// Match: Team A 1 - 0 Team B
	matches := []CompetitionMatchView{
		{
			HomeEntryID: &teamA,
			AwayEntryID: &teamB,
			HomeScore:   1,
			AwayScore:   0,
			MatchStatus: "COMPLETED",
		},
	}

	standings := CalculateFootballStandings(matches, entries)
	if standings[0].EntryID != teamA {
		t.Fatalf("expected Team A rank 1 by H2H win, got %s", standings[0].DisplayName)
	}
}

func TestPelotonBunchFinishGrouping(t *testing.T) {
	// Peloton group:
	// Rider 1: 10,000,000 ms (Leader of bunch 1)
	// Rider 2: 10,000,400 ms (+400ms -> bunch 1)
	// Rider 3: 10,000,800 ms (+400ms -> bunch 1)
	// Rider 4: 10,002,500 ms (+1700ms gap -> starts bunch 2)
	// Rider 5: 10,003,100 ms (+600ms -> bunch 2)
	rawTimes := []int64{10000000, 10000400, 10000800, 10002500, 10003100}

	adjusted := CalculatePelotonGrouping(rawTimes, 1000)
	if len(adjusted) != 5 {
		t.Fatalf("expected 5 adjusted times, got %d", len(adjusted))
	}

	// First 3 riders should have exactly 10,000,000 ms
	if adjusted[0] != 10000000 || adjusted[1] != 10000000 || adjusted[2] != 10000000 {
		t.Errorf("expected bunch 1 to have 10,000,000 ms, got %v", adjusted[:3])
	}

	// Riders 4 and 5 should have 10,002,500 ms
	if adjusted[3] != 10002500 || adjusted[4] != 10002500 {
		t.Errorf("expected bunch 2 to have 10,002,500 ms, got %v", adjusted[3:])
	}
}

func TestRankResultsByStrategy(t *testing.T) {
	t1 := int64(3600000)
	t2 := int64(3605000)

	results := []CompetitionResultView{
		{
			DisplayName:   "Runner Two",
			PrimaryTimeMs: &t2,
			Status:        StatusFinished,
		},
		{
			DisplayName:   "Runner One",
			PrimaryTimeMs: &t1,
			Status:        StatusFinished,
		},
		{
			DisplayName: "Runner DNF",
			Status:      StatusDNF,
		},
	}

	ranked := RankResultsByStrategy(results, StrategyTimeAsc)
	if *ranked[0].RankOverall != 1 || ranked[0].DisplayName != "Runner One" {
		t.Errorf("expected Runner One rank 1, got %v", ranked[0].DisplayName)
	}
	if *ranked[1].RankOverall != 2 || ranked[1].DisplayName != "Runner Two" {
		t.Errorf("expected Runner Two rank 2, got %v", ranked[1].DisplayName)
	}
	if ranked[2].RankOverall != nil {
		t.Errorf("DNF runner should have nil rank, got %v", ranked[2].RankOverall)
	}
}

func TestStatusSafety_NeverSilentlyFinished(t *testing.T) {
	invalidStatuses := []CompetitionStatus{
		"UNKNOWN", "MALFORMED", "INVALID", "", "FOO",
	}

	for _, st := range invalidStatuses {
		if st.IsOfficialValidFinisher() {
			t.Errorf("status %q should never be considered an official valid finisher", st)
		}
	}
}

func TestRankResultsByStrategy_ScoreAsc(t *testing.T) {
	results := []CompetitionResultView{
		{DisplayName: "Golfer B", PointsScored: 72, Status: StatusFinished},
		{DisplayName: "Golfer A", PointsScored: 68, Status: StatusFinished},
		{DisplayName: "Golfer C", PointsScored: 75, Status: StatusFinished},
	}

	ranked := RankResultsByStrategy(results, StrategyScoreAsc)
	if *ranked[0].RankOverall != 1 || ranked[0].DisplayName != "Golfer A" {
		t.Errorf("expected Golfer A with 68 to be rank 1, got %s (rank %v)", ranked[0].DisplayName, ranked[0].RankOverall)
	}
	if *ranked[1].RankOverall != 2 || ranked[1].DisplayName != "Golfer B" {
		t.Errorf("expected Golfer B with 72 to be rank 2, got %s (rank %v)", ranked[1].DisplayName, ranked[1].RankOverall)
	}
	if *ranked[2].RankOverall != 3 || ranked[2].DisplayName != "Golfer C" {
		t.Errorf("expected Golfer C with 75 to be rank 3, got %s (rank %v)", ranked[2].DisplayName, ranked[2].RankOverall)
	}
}

func TestRankResultsByStrategy_TiesSharedRank(t *testing.T) {
	t1 := int64(3600000)
	t2 := int64(3600000) // identical time to t1 -> tie for 1st
	t3 := int64(3700000) // 3rd

	results := []CompetitionResultView{
		{DisplayName: "Runner B", PrimaryTimeMs: &t1, Status: StatusFinished},
		{DisplayName: "Runner A", PrimaryTimeMs: &t2, Status: StatusFinished},
		{DisplayName: "Runner C", PrimaryTimeMs: &t3, Status: StatusFinished},
	}

	ranked := RankResultsByStrategy(results, StrategyTimeAsc)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 results, got %d", len(ranked))
	}

	// Both Runner A and Runner B have the exact same time: should share rank 1
	if *ranked[0].RankOverall != 1 {
		t.Errorf("expected rank 1 for first runner, got %d", *ranked[0].RankOverall)
	}
	if *ranked[1].RankOverall != 1 {
		t.Errorf("expected rank 1 (tie) for second runner with identical time, got %d", *ranked[1].RankOverall)
	}
	// Third runner should be rank 3 (standard competition tie skip)
	if *ranked[2].RankOverall != 3 {
		t.Errorf("expected rank 3 for runner after 2-way tie for 1st, got %d", *ranked[2].RankOverall)
	}
}

func TestBadmintonStandingsCalculation(t *testing.T) {
	playerA := uuid.New()
	playerB := uuid.New()
	playerC := uuid.New()

	entries := []ParticipantEntryView{
		{ID: playerA, DisplayName: "Anthony Ginting", Status: StatusActive},
		{ID: playerB, DisplayName: "Viktor Axelsen", Status: StatusActive},
		{ID: playerC, DisplayName: "Kodai Naraoka", Status: StatusActive},
	}

	// Match 1: Axelsen beats Ginting 2-0 (21-15, 21-18)
	// Match 2: Ginting beats Naraoka 2-0 (21-10, 21-12)
	// Match 3: Axelsen beats Naraoka 2-0 (21-14, 21-16)
	matches := []CompetitionMatchView{
		{
			HomeEntryID: &playerB,
			AwayEntryID: &playerA,
			HomeScore:   2,
			AwayScore:   0,
			MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{
				{PeriodNumber: 1, HomeScore: 21, AwayScore: 15},
				{PeriodNumber: 2, HomeScore: 21, AwayScore: 18},
			},
		},
		{
			HomeEntryID: &playerA,
			AwayEntryID: &playerC,
			HomeScore:   2,
			AwayScore:   0,
			MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{
				{PeriodNumber: 1, HomeScore: 21, AwayScore: 10},
				{PeriodNumber: 2, HomeScore: 21, AwayScore: 12},
			},
		},
		{
			HomeEntryID: &playerB,
			AwayEntryID: &playerC,
			HomeScore:   2,
			AwayScore:   0,
			MatchStatus: "COMPLETED",
			ScoreDetails: []MatchScoreDetail{
				{PeriodNumber: 1, HomeScore: 21, AwayScore: 14},
				{PeriodNumber: 2, HomeScore: 21, AwayScore: 16},
			},
		},
	}

	standings := CalculateBadmintonStandings(matches, entries)
	if len(standings) != 3 {
		t.Fatalf("expected 3 entries in standings, got %d", len(standings))
	}

	// Axelsen should be Rank 1 (2 wins)
	if standings[0].EntryID != playerB {
		t.Errorf("expected Axelsen rank 1, got %s", standings[0].DisplayName)
	}
	if standings[0].Wins != 2 || standings[0].Points != 2 {
		t.Errorf("expected 2 wins / 2 pts for Axelsen, got wins=%d, pts=%d", standings[0].Wins, standings[0].Points)
	}

	// Ginting should be Rank 2 (1 win)
	if standings[1].EntryID != playerA {
		t.Errorf("expected Ginting rank 2, got %s", standings[1].DisplayName)
	}
	if standings[1].Wins != 1 {
		t.Errorf("expected 1 win for Ginting, got %d", standings[1].Wins)
	}

	// Naraoka should be Rank 3 (0 wins)
	if standings[2].EntryID != playerC {
		t.Errorf("expected Naraoka rank 3, got %s", standings[2].DisplayName)
	}
}

func TestPointsStandingsCalculation_CustomPoints(t *testing.T) {
	teamX := uuid.New()
	teamY := uuid.New()
	teamZ := uuid.New()

	entries := []ParticipantEntryView{
		{ID: teamX, DisplayName: "Team X", Status: StatusActive},
		{ID: teamY, DisplayName: "Team Y", Status: StatusActive},
		{ID: teamZ, DisplayName: "Team Z", Status: StatusActive},
	}

	// Win = 2 pts, Draw = 1 pt
	matches := []CompetitionMatchView{
		{
			HomeEntryID: &teamX,
			AwayEntryID: &teamY,
			HomeScore:   3,
			AwayScore:   1,
			MatchStatus: "COMPLETED",
		},
		{
			HomeEntryID: &teamY,
			AwayEntryID: &teamZ,
			HomeScore:   2,
			AwayScore:   2,
			MatchStatus: "COMPLETED",
		},
		{
			HomeEntryID: &teamX,
			AwayEntryID: &teamZ,
			HomeScore:   1,
			AwayScore:   0,
			MatchStatus: "COMPLETED",
		},
	}

	standings := CalculatePointsStandings(matches, entries, 2, 1)
	if len(standings) != 3 {
		t.Fatalf("expected 3 entries in standings, got %d", len(standings))
	}

	// Team X: 2 wins = 4 pts (Rank 1)
	if standings[0].EntryID != teamX || standings[0].Points != 4 {
		t.Errorf("expected Team X rank 1 with 4 pts, got %s (%d pts)", standings[0].DisplayName, standings[0].Points)
	}

	// Team Y: 1 draw = 1 pt (Rank 2 or 3 depending on tiebreaker)
	// Team Z: 1 draw = 1 pt
	if standings[1].Points != 1 || standings[2].Points != 1 {
		t.Errorf("expected 1 pt for Y and Z, got %d and %d", standings[1].Points, standings[2].Points)
	}
}

