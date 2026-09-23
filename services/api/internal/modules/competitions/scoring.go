package competitions

import (
	"sort"
	"strings"
)

// MatchOutcome holds the calculated score and resolution of a match.
type MatchOutcome struct {
	HomeScore    int
	AwayScore    int
	WinnerEntry  *string // "HOME", "AWAY", or nil for DRAW
	MatchStatus  string  // "COMPLETED", "LIVE", "FORFEIT"
	ScoreDetails []MatchScoreDetail
}

// EvaluateBadmintonMatch evaluates a badminton match given set/period observations.
// BWF Rules: Best of 3 games to 21 points. Must win by 2 points. Cap at 30 points (29-29 -> first to 30 wins).
func EvaluateBadmintonMatch(sets []MatchScoreDetail) MatchOutcome {
	homeGamesWon := 0
	awayGamesWon := 0

	for _, s := range sets {
		homePts := s.HomeScore
		awayPts := s.AwayScore

		// BWF game win condition:
		// 1. Minimum 21 points AND leading by at least 2 points
		// 2. OR reaching 30 points first (sudden death at 29-29)
		if (homePts >= 21 && homePts-awayPts >= 2) || (homePts == 30 && awayPts >= 29) {
			homeGamesWon++
		} else if (awayPts >= 21 && awayPts-homePts >= 2) || (awayPts == 30 && homePts >= 29) {
			awayGamesWon++
		}
	}

	outcome := MatchOutcome{
		HomeScore:    homeGamesWon,
		AwayScore:    awayGamesWon,
		ScoreDetails: sets,
		MatchStatus:  "LIVE",
	}

	// Match is won by the first side to win 2 games
	if homeGamesWon >= 2 {
		outcome.MatchStatus = "COMPLETED"
		winner := "HOME"
		outcome.WinnerEntry = &winner
	} else if awayGamesWon >= 2 {
		outcome.MatchStatus = "COMPLETED"
		winner := "AWAY"
		outcome.WinnerEntry = &winner
	}

	return outcome
}

// FootballMatchStandingsCalculator calculates round-robin league standings from completed matches.
// IFAB / FIFA Standards: Win = 3 pts, Draw = 1 pt, Loss = 0 pts.
// Tie-breakers:
// 1. Total Points
// 2. Head-to-Head points (if 2 teams tied)
// 3. Overall Goal Difference (GD = GF - GA)
// 4. Overall Goals For (GF)
func CalculateFootballStandings(matches []CompetitionMatchView, entries []ParticipantEntryView) []GenericStandingsRow {
	statsMap := make(map[string]*GenericStandingsRow)

	for _, entry := range entries {
		idStr := entry.ID.String()
		statsMap[idStr] = &GenericStandingsRow{
			EntryID:        entry.ID,
			DisplayName:    entry.DisplayName,
			IdentifierCode: entry.IdentifierCode,
			Status:         string(entry.Status),
			CustomMetrics:  make(map[string]any),
		}
	}

	// Track head to head between pairs
	h2hPoints := make(map[string]map[string]int)

	for _, m := range matches {
		if m.MatchStatus != "COMPLETED" {
			continue
		}
		if m.HomeEntryID == nil || m.AwayEntryID == nil {
			continue
		}

		hID := m.HomeEntryID.String()
		aID := m.AwayEntryID.String()

		homeStats, hOk := statsMap[hID]
		awayStats, aOk := statsMap[aID]
		if !hOk || !aOk {
			continue
		}

		if h2hPoints[hID] == nil {
			h2hPoints[hID] = make(map[string]int)
		}
		if h2hPoints[aID] == nil {
			h2hPoints[aID] = make(map[string]int)
		}

		homeStats.Played++
		awayStats.Played++

		homeStats.GoalsFor += m.HomeScore
		homeStats.GoalsAgainst += m.AwayScore
		awayStats.GoalsFor += m.AwayScore
		awayStats.GoalsAgainst += m.HomeScore

		if m.HomeScore > m.AwayScore {
			homeStats.Wins++
			homeStats.Points += 3
			awayStats.Losses++
			h2hPoints[hID][aID] += 3
		} else if m.AwayScore > m.HomeScore {
			awayStats.Wins++
			awayStats.Points += 3
			homeStats.Losses++
			h2hPoints[aID][hID] += 3
		} else {
			homeStats.Draws++
			homeStats.Points++
			awayStats.Draws++
			awayStats.Points++
			h2hPoints[hID][aID]++
			h2hPoints[aID][hID]++
		}
	}

	rows := make([]GenericStandingsRow, 0, len(statsMap))
	for _, row := range statsMap {
		row.GoalDifference = row.GoalsFor - row.GoalsAgainst
		rows = append(rows, *row)
	}

	// Sort standings according to FIFA rules:
	// 1. Points
	// 2. Goal Difference (GD)
	// 3. Goals For (GF)
	// 4. Head-to-Head points if tied on GD and GF
	sort.Slice(rows, func(i, j int) bool {
		// 1. Points
		if rows[i].Points != rows[j].Points {
			return rows[i].Points > rows[j].Points
		}

		// 2. Goal Difference
		if rows[i].GoalDifference != rows[j].GoalDifference {
			return rows[i].GoalDifference > rows[j].GoalDifference
		}

		// 3. Goals For
		if rows[i].GoalsFor != rows[j].GoalsFor {
			return rows[i].GoalsFor > rows[j].GoalsFor
		}

		// 4. Head-to-head points if directly played
		iID := rows[i].EntryID.String()
		jID := rows[j].EntryID.String()
		if h2hPoints[iID] != nil && h2hPoints[jID] != nil {
			ptsI := h2hPoints[iID][jID]
			ptsJ := h2hPoints[jID][iID]
			if ptsI != ptsJ {
				return ptsI > ptsJ
			}
		}

		// 5. Alphabetical fallback
		return strings.ToLower(rows[i].DisplayName) < strings.ToLower(rows[j].DisplayName)
	})

	for idx := range rows {
		rows[idx].Rank = idx + 1
	}

	return rows
}

// CalculateBadmintonStandings calculates BWF round-robin group standings from completed matches.
// BWF General Competition Regulations (GCR 16.2):
// 16.2.1: Ranking is decided by the number of matches won (Points).
// 16.2.2: If two teams/players have won the same number of matches, ranking is decided by the match between them (Head-to-Head).
// 16.2.3: If three or more teams/players have won the same number of matches:
//   16.2.3.1: Ranking is decided by the difference of games won and lost (Game Difference).
//   16.2.3.2: If this still leaves two teams/players tied, ranking is decided by the match between them.
//   16.2.3.3: If three or more teams/players still have the same difference of games won and lost, ranking is decided by the difference of points won and lost (Rally Point Difference).
//   16.2.3.4: If this still leaves two teams/players tied, ranking is decided by the match between them.
//   16.2.3.5: If three or more remain tied, ranking is decided by drawing lots / alphabetical fallback.
func CalculateBadmintonStandings(matches []CompetitionMatchView, entries []ParticipantEntryView) []GenericStandingsRow {
	statsMap := make(map[string]*GenericStandingsRow)
	gamesWonMap := make(map[string]int)
	gamesLostMap := make(map[string]int)
	rallyWonMap := make(map[string]int)
	rallyLostMap := make(map[string]int)
	h2hWins := make(map[string]map[string]int)

	for _, entry := range entries {
		idStr := entry.ID.String()
		statsMap[idStr] = &GenericStandingsRow{
			EntryID:        entry.ID,
			DisplayName:    entry.DisplayName,
			IdentifierCode: entry.IdentifierCode,
			Status:         string(entry.Status),
			CustomMetrics:  make(map[string]any),
		}
	}

	for _, m := range matches {
		if m.MatchStatus != "COMPLETED" {
			continue
		}
		if m.HomeEntryID == nil || m.AwayEntryID == nil {
			continue
		}

		hID := m.HomeEntryID.String()
		aID := m.AwayEntryID.String()

		homeStats, hOk := statsMap[hID]
		awayStats, aOk := statsMap[aID]
		if !hOk || !aOk {
			continue
		}

		if h2hWins[hID] == nil {
			h2hWins[hID] = make(map[string]int)
		}
		if h2hWins[aID] == nil {
			h2hWins[aID] = make(map[string]int)
		}

		homeStats.Played++
		awayStats.Played++

		// In Badminton, HomeScore and AwayScore represent games won (e.g. 2-1 or 2-0)
		homeGames := m.HomeScore
		awayGames := m.AwayScore
		gamesWonMap[hID] += homeGames
		gamesLostMap[hID] += awayGames
		gamesWonMap[aID] += awayGames
		gamesLostMap[aID] += homeGames

		// Aggregate rally points from ScoreDetails
		for _, set := range m.ScoreDetails {
			rallyWonMap[hID] += set.HomeScore
			rallyLostMap[hID] += set.AwayScore
			rallyWonMap[aID] += set.AwayScore
			rallyLostMap[aID] += set.HomeScore
		}

		if homeGames > awayGames {
			homeStats.Wins++
			homeStats.Points++
			awayStats.Losses++
			h2hWins[hID][aID]++
		} else if awayGames > homeGames {
			awayStats.Wins++
			awayStats.Points++
			homeStats.Losses++
			h2hWins[aID][hID]++
		}
	}

	for idStr, row := range statsMap {
		gDiff := gamesWonMap[idStr] - gamesLostMap[idStr]
		pDiff := rallyWonMap[idStr] - rallyLostMap[idStr]
		row.CustomMetrics["games_won"] = gamesWonMap[idStr]
		row.CustomMetrics["games_lost"] = gamesLostMap[idStr]
		row.CustomMetrics["game_difference"] = gDiff
		row.CustomMetrics["rally_points_won"] = rallyWonMap[idStr]
		row.CustomMetrics["rally_points_lost"] = rallyLostMap[idStr]
		row.CustomMetrics["rally_point_difference"] = pDiff
	}

	// Group rows by Points (matches won)
	pointsBuckets := make(map[int][]*GenericStandingsRow)
	for _, row := range statsMap {
		pointsBuckets[row.Points] = append(pointsBuckets[row.Points], row)
	}

	// Sort unique points descending
	uniquePoints := make([]int, 0, len(pointsBuckets))
	for pts := range pointsBuckets {
		uniquePoints = append(uniquePoints, pts)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(uniquePoints)))

	sortedRows := make([]GenericStandingsRow, 0, len(statsMap))
	for _, pts := range uniquePoints {
		group := pointsBuckets[pts]
		resolved := resolveBadmintonTiedGroup(group, h2hWins, gamesWonMap, gamesLostMap, rallyWonMap, rallyLostMap)
		for _, r := range resolved {
			sortedRows = append(sortedRows, *r)
		}
	}

	for idx := range sortedRows {
		sortedRows[idx].Rank = idx + 1
	}

	return sortedRows
}

func resolveBadmintonTiedGroup(
	group []*GenericStandingsRow,
	h2hWins map[string]map[string]int,
	gamesWonMap, gamesLostMap, rallyWonMap, rallyLostMap map[string]int,
) []*GenericStandingsRow {
	if len(group) <= 1 {
		return group
	}

	if len(group) == 2 {
		return resolveBadmintonPair(group[0], group[1], h2hWins, gamesWonMap, gamesLostMap, rallyWonMap, rallyLostMap)
	}

	// 16.2.3: 3 or more tied on match points -> rank by Game Difference
	gdBuckets := make(map[int][]*GenericStandingsRow)
	for _, r := range group {
		idStr := r.EntryID.String()
		gd := gamesWonMap[idStr] - gamesLostMap[idStr]
		gdBuckets[gd] = append(gdBuckets[gd], r)
	}

	uniqueGD := make([]int, 0, len(gdBuckets))
	for gd := range gdBuckets {
		uniqueGD = append(uniqueGD, gd)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(uniqueGD)))

	out := make([]*GenericStandingsRow, 0, len(group))
	for _, gd := range uniqueGD {
		subGroup := gdBuckets[gd]
		if len(subGroup) == 1 {
			out = append(out, subGroup[0])
		} else if len(subGroup) == 2 {
			// 16.2.3.2: 2 tied after game difference -> head-to-head between the two
			resolvedPair := resolveBadmintonPair(subGroup[0], subGroup[1], h2hWins, gamesWonMap, gamesLostMap, rallyWonMap, rallyLostMap)
			out = append(out, resolvedPair...)
		} else {
			// 16.2.3.3: 3 or more still tied on game difference -> rank by Rally Point Difference
			rpdBuckets := make(map[int][]*GenericStandingsRow)
			for _, r := range subGroup {
				idStr := r.EntryID.String()
				rpd := rallyWonMap[idStr] - rallyLostMap[idStr]
				rpdBuckets[rpd] = append(rpdBuckets[rpd], r)
			}

			uniqueRPD := make([]int, 0, len(rpdBuckets))
			for rpd := range rpdBuckets {
				uniqueRPD = append(uniqueRPD, rpd)
			}
			sort.Sort(sort.Reverse(sort.IntSlice(uniqueRPD)))

			for _, rpd := range uniqueRPD {
				rpdGroup := rpdBuckets[rpd]
				if len(rpdGroup) == 1 {
					out = append(out, rpdGroup[0])
				} else if len(rpdGroup) == 2 {
					// 16.2.3.4: 2 tied after points difference -> head-to-head between the two
					resolvedPair := resolveBadmintonPair(rpdGroup[0], rpdGroup[1], h2hWins, gamesWonMap, gamesLostMap, rallyWonMap, rallyLostMap)
					out = append(out, resolvedPair...)
				} else {
					// 16.2.3.5: Still tied -> alphabetical fallback
					sort.Slice(rpdGroup, func(i, j int) bool {
						return strings.ToLower(rpdGroup[i].DisplayName) < strings.ToLower(rpdGroup[j].DisplayName)
					})
					out = append(out, rpdGroup...)
				}
			}
		}
	}

	return out
}

func resolveBadmintonPair(
	a, b *GenericStandingsRow,
	h2hWins map[string]map[string]int,
	gamesWonMap, gamesLostMap, rallyWonMap, rallyLostMap map[string]int,
) []*GenericStandingsRow {
	aID := a.EntryID.String()
	bID := b.EntryID.String()

	// 1. Head to head match winner
	if h2hWins[aID] != nil && h2hWins[bID] != nil {
		wA := h2hWins[aID][bID]
		wB := h2hWins[bID][aID]
		if wA > wB {
			return []*GenericStandingsRow{a, b}
		}
		if wB > wA {
			return []*GenericStandingsRow{b, a}
		}
	}

	// 2. Game Difference
	gdA := gamesWonMap[aID] - gamesLostMap[aID]
	gdB := gamesWonMap[bID] - gamesLostMap[bID]
	if gdA > gdB {
		return []*GenericStandingsRow{a, b}
	}
	if gdB > gdA {
		return []*GenericStandingsRow{b, a}
	}

	// 3. Rally Point Difference
	rpdA := rallyWonMap[aID] - rallyLostMap[aID]
	rpdB := rallyWonMap[bID] - rallyLostMap[bID]
	if rpdA > rpdB {
		return []*GenericStandingsRow{a, b}
	}
	if rpdB > rpdA {
		return []*GenericStandingsRow{b, a}
	}

	// 4. Alphabetical fallback
	if strings.ToLower(a.DisplayName) <= strings.ToLower(b.DisplayName) {
		return []*GenericStandingsRow{a, b}
	}
	return []*GenericStandingsRow{b, a}
}

// CalculatePointsStandings calculates generic points-based round-robin standings (e.g. Chess, general sports).
func CalculatePointsStandings(matches []CompetitionMatchView, entries []ParticipantEntryView, winPts, drawPts int) []GenericStandingsRow {
	statsMap := make(map[string]*GenericStandingsRow)
	h2hWins := make(map[string]map[string]int)

	for _, entry := range entries {
		idStr := entry.ID.String()
		statsMap[idStr] = &GenericStandingsRow{
			EntryID:        entry.ID,
			DisplayName:    entry.DisplayName,
			IdentifierCode: entry.IdentifierCode,
			Status:         string(entry.Status),
			CustomMetrics:  make(map[string]any),
		}
	}

	for _, m := range matches {
		if m.MatchStatus != "COMPLETED" {
			continue
		}
		if m.HomeEntryID == nil || m.AwayEntryID == nil {
			continue
		}

		hID := m.HomeEntryID.String()
		aID := m.AwayEntryID.String()

		homeStats, hOk := statsMap[hID]
		awayStats, aOk := statsMap[aID]
		if !hOk || !aOk {
			continue
		}

		if h2hWins[hID] == nil {
			h2hWins[hID] = make(map[string]int)
		}
		if h2hWins[aID] == nil {
			h2hWins[aID] = make(map[string]int)
		}

		homeStats.Played++
		awayStats.Played++

		if m.HomeScore > m.AwayScore {
			homeStats.Wins++
			homeStats.Points += winPts
			awayStats.Losses++
			h2hWins[hID][aID]++
		} else if m.AwayScore > m.HomeScore {
			awayStats.Wins++
			awayStats.Points += winPts
			homeStats.Losses++
			h2hWins[aID][hID]++
		} else {
			homeStats.Draws++
			homeStats.Points += drawPts
			awayStats.Draws++
			awayStats.Points += drawPts
		}
	}

	rows := make([]GenericStandingsRow, 0, len(statsMap))
	for _, row := range statsMap {
		rows = append(rows, *row)
	}

	sort.Slice(rows, func(i, j int) bool {
		// 1. Points
		if rows[i].Points != rows[j].Points {
			return rows[i].Points > rows[j].Points
		}

		// 2. Wins
		if rows[i].Wins != rows[j].Wins {
			return rows[i].Wins > rows[j].Wins
		}

		// 3. Head-to-head
		iID := rows[i].EntryID.String()
		jID := rows[j].EntryID.String()
		if h2hWins[iID] != nil && h2hWins[jID] != nil {
			wI := h2hWins[iID][jID]
			wJ := h2hWins[jID][iID]
			if wI != wJ {
				return wI > wJ
			}
		}

		// 4. Alphabetical fallback
		return strings.ToLower(rows[i].DisplayName) < strings.ToLower(rows[j].DisplayName)
	})

	for idx := range rows {
		rows[idx].Rank = idx + 1
	}

	return rows
}

// CalculatePelotonGrouping applies UCI road cycling peloton bunch-finish rule.
// Riders finishing in a continuous bunch (gap to preceding rider < maxGapMs, typically 1000ms)
// receive the official time of the leading rider of that bunch.
func CalculatePelotonGrouping(riderTimes []int64, maxGapMs int64) []int64 {
	if len(riderTimes) == 0 {
		return nil
	}
	if maxGapMs <= 0 {
		maxGapMs = 1000 // 1 second UCI default
	}

	adjusted := make([]int64, len(riderTimes))
	adjusted[0] = riderTimes[0]
	packLeadTime := riderTimes[0]

	for i := 1; i < len(riderTimes); i++ {
		gap := riderTimes[i] - riderTimes[i-1]
		if gap <= maxGapMs {
			// Part of the same bunch
			adjusted[i] = packLeadTime
		} else {
			// Split in the peloton, start new bunch
			packLeadTime = riderTimes[i]
			adjusted[i] = packLeadTime
		}
	}

	return adjusted
}

// RankResultsByStrategy ranks competition results in memory based on strategy.
func RankResultsByStrategy(results []CompetitionResultView, strategy RankingStrategy) []CompetitionResultView {
	ranked := make([]CompetitionResultView, len(results))
	copy(ranked, results)

	sort.Slice(ranked, func(i, j int) bool {
		validI := ranked[i].Status.IsOfficialValidFinisher()
		validJ := ranked[j].Status.IsOfficialValidFinisher()

		// Valid finishers always rank above DNF / DNS / DSQ
		if validI != validJ {
			return validI
		}

		switch strategy {
		case StrategyTimeAsc:
			var tI int64 = 1<<62 - 1
			if ranked[i].PrimaryTimeMs != nil {
				tI = *ranked[i].PrimaryTimeMs
			} else if ranked[i].SecondaryTimeMs != nil {
				tI = *ranked[i].SecondaryTimeMs
			}

			var tJ int64 = 1<<62 - 1
			if ranked[j].PrimaryTimeMs != nil {
				tJ = *ranked[j].PrimaryTimeMs
			} else if ranked[j].SecondaryTimeMs != nil {
				tJ = *ranked[j].SecondaryTimeMs
			}

			if tI != tJ {
				return tI < tJ
			}
			return strings.ToLower(ranked[i].DisplayName) < strings.ToLower(ranked[j].DisplayName)

		case StrategyPointsDesc, StrategyScoreDesc:
			if ranked[i].PointsScored != ranked[j].PointsScored {
				return ranked[i].PointsScored > ranked[j].PointsScored
			}
			return strings.ToLower(ranked[i].DisplayName) < strings.ToLower(ranked[j].DisplayName)

		case StrategyScoreAsc:
			if ranked[i].PointsScored != ranked[j].PointsScored {
				return ranked[i].PointsScored < ranked[j].PointsScored
			}
			return strings.ToLower(ranked[i].DisplayName) < strings.ToLower(ranked[j].DisplayName)

		case StrategyTimeDesc:
			var tI int64 = 0
			if ranked[i].PrimaryTimeMs != nil {
				tI = *ranked[i].PrimaryTimeMs
			}
			var tJ int64 = 0
			if ranked[j].PrimaryTimeMs != nil {
				tJ = *ranked[j].PrimaryTimeMs
			}
			if tI != tJ {
				return tI > tJ
			}
			return strings.ToLower(ranked[i].DisplayName) < strings.ToLower(ranked[j].DisplayName)

		default:
			return strings.ToLower(ranked[i].DisplayName) < strings.ToLower(ranked[j].DisplayName)
		}
	})

	currentRank := 1
	for idx := range ranked {
		if ranked[idx].Status.IsOfficialValidFinisher() {
			if idx > 0 && ranked[idx-1].Status.IsOfficialValidFinisher() && ranked[idx-1].RankOverall != nil && isEqualMetric(ranked[idx-1], ranked[idx], strategy) {
				r := *ranked[idx-1].RankOverall
				ranked[idx].RankOverall = &r
			} else {
				r := currentRank
				ranked[idx].RankOverall = &r
			}
			currentRank++
		} else {
			ranked[idx].RankOverall = nil
		}
	}

	return ranked
}

func isEqualMetric(a, b CompetitionResultView, strategy RankingStrategy) bool {
	switch strategy {
	case StrategyTimeAsc, StrategyTimeDesc:
		var tA, tB *int64
		if a.PrimaryTimeMs != nil {
			tA = a.PrimaryTimeMs
		} else {
			tA = a.SecondaryTimeMs
		}
		if b.PrimaryTimeMs != nil {
			tB = b.PrimaryTimeMs
		} else {
			tB = b.SecondaryTimeMs
		}
		if tA != nil && tB != nil {
			return *tA == *tB
		}
		return tA == nil && tB == nil
	case StrategyPointsDesc, StrategyScoreDesc, StrategyScoreAsc:
		return a.PointsScored == b.PointsScored
	default:
		return false
	}
}
