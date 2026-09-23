package competitions

import (
	"sort"

	"github.com/google/uuid"
)

// FixtureDraft represents an unpersisted match fixture.
type FixtureDraft struct {
	RoundNumber           int
	MatchNumber           int
	HomeEntryID           *uuid.UUID
	AwayEntryID           *uuid.UUID
	VenueCourtName        string
	SourceHomeMatchNumber int
	SourceAwayMatchNumber int
	IsBye                 bool
	WinnerID              *uuid.UUID
	MatchStatus           string
}

// GenerateRoundRobin generates a balanced round-robin tournament schedule.
// For N participants, generates N-1 rounds if N is even, or N rounds if N is odd.
func GenerateRoundRobin(entries []uuid.UUID) []FixtureDraft {
	n := len(entries)
	if n < 2 {
		return nil
	}

	teams := make([]*uuid.UUID, n)
	for i := range entries {
		e := entries[i]
		teams[i] = &e
	}

	// If odd number of teams, add a dummy bye (nil)
	hasDummy := false
	if n%2 != 0 {
		teams = append(teams, nil)
		n++
		hasDummy = true
	}

	totalRounds := n - 1
	matchesPerRound := n / 2
	fixtures := make([]FixtureDraft, 0, totalRounds*matchesPerRound)

	// Round-robin circle algorithm
	matchNum := 1
	for round := 1; round <= totalRounds; round++ {
		for i := 0; i < matchesPerRound; i++ {
			home := teams[i]
			away := teams[n-1-i]

			// Skip bye fixtures
			if home == nil || away == nil {
				continue
			}

			// Alternate home/away for fair venue distribution
			if round%2 == 1 {
				fixtures = append(fixtures, FixtureDraft{
					RoundNumber: round,
					MatchNumber: matchNum,
					HomeEntryID: home,
					AwayEntryID: away,
					MatchStatus: "SCHEDULED",
				})
			} else {
				fixtures = append(fixtures, FixtureDraft{
					RoundNumber: round,
					MatchNumber: matchNum,
					HomeEntryID: away,
					AwayEntryID: home,
					MatchStatus: "SCHEDULED",
				})
			}
			matchNum++
		}

		// Rotate teams, keeping the first team fixed
		newTeams := make([]*uuid.UUID, n)
		newTeams[0] = teams[0]
		newTeams[1] = teams[n-1]
		copy(newTeams[2:], teams[1:n-1])
		teams = newTeams
	}

	_ = hasDummy
	return fixtures
}

// bracketSeedOrder generates the classic tournament bracket pairing order for a power of 2.
// For p=2: [1, 2]
// For p=4: [1, 4, 2, 3] -> (1 vs 4), (2 vs 3)
// For p=8: [1, 8, 4, 5, 2, 7, 3, 6] -> (1 vs 8), (4 vs 5), (2 vs 7), (3 vs 6)
// For p=16: [1, 16, 8, 9, 4, 13, 5, 12, 2, 15, 7, 10, 3, 14, 6, 11]
func bracketSeedOrder(p int) []int {
	if p <= 2 {
		return []int{1, 2}
	}
	prev := bracketSeedOrder(p / 2)
	res := make([]int, p)
	for i, v := range prev {
		res[2*i] = v
		res[2*i+1] = p + 1 - v
	}
	return res
}

// GenerateSingleElimination generates a complete single elimination tournament bracket.
// Supports 2 to 16+ participants, generating all rounds (Round 1 to Finals).
// High seeds receive byes and are automatically advanced to the next round.
func GenerateSingleElimination(seededEntries []uuid.UUID) []FixtureDraft {
	n := len(seededEntries)
	if n < 2 {
		return nil
	}

	// Find the smallest power of 2 >= n
	powerOf2 := 2
	for powerOf2 < n {
		powerOf2 *= 2
	}

	totalRounds := 0
	for temp := powerOf2; temp > 1; temp /= 2 {
		totalRounds++
	}

	seedMap := make(map[int]*uuid.UUID, n)
	for i := 0; i < n; i++ {
		e := seededEntries[i]
		seedMap[i+1] = &e
	}

	roundWinners := make(map[int]map[int]*uuid.UUID)
	for r := 1; r <= totalRounds; r++ {
		roundWinners[r] = make(map[int]*uuid.UUID)
	}

	fixtures := make([]FixtureDraft, 0, powerOf2-1)

	// 1. Round 1 matches
	order := bracketSeedOrder(powerOf2)
	r1Matches := powerOf2 / 2
	for m := 1; m <= r1Matches; m++ {
		homeSeed := order[2*(m-1)]
		awaySeed := order[2*(m-1)+1]

		home := seedMap[homeSeed]
		away := seedMap[awaySeed]

		draft := FixtureDraft{
			RoundNumber: 1,
			MatchNumber: m,
			HomeEntryID: home,
			AwayEntryID: away,
			MatchStatus: "SCHEDULED",
		}

		if away == nil && home != nil {
			// Bye: Home entry immediately advances
			draft.IsBye = true
			draft.WinnerID = home
			draft.MatchStatus = "BYE"
			roundWinners[1][m] = home
		}

		fixtures = append(fixtures, draft)
	}

	// 2. Subsequent rounds (Round 2 up to Finals)
	for r := 2; r <= totalRounds; r++ {
		matchesInRound := powerOf2 / (1 << r)
		for m := 1; m <= matchesInRound; m++ {
			srcHome := 2*m - 1
			srcAway := 2*m

			homeWinner := roundWinners[r-1][srcHome]
			awayWinner := roundWinners[r-1][srcAway]

			draft := FixtureDraft{
				RoundNumber:           r,
				MatchNumber:           m,
				HomeEntryID:           homeWinner,
				AwayEntryID:           awayWinner,
				SourceHomeMatchNumber: srcHome,
				SourceAwayMatchNumber: srcAway,
				MatchStatus:           "SCHEDULED",
			}

			fixtures = append(fixtures, draft)
		}
	}

	return fixtures
}

// SwimmerSeed holds athlete id and qualifying seed time in ms for lane seeding.
type SwimmerSeed struct {
	EntryID     uuid.UUID
	SeedTimeMs  int64
	HeatNumber  int
	LaneNumber  int
}

// AssignAquaticsLanes applies World Aquatics (FINA Rule SW 3.1) spearhead lane assignment.
// 10-lane pool: 5, 6, 4, 7, 3, 8, 2, 9, 1, 10.
// 8-lane pool:  4, 5, 3, 6, 2, 7, 1, 8.
// 6-lane pool:  3, 4, 2, 5, 1, 6.
// 4-lane pool:  2, 3, 1, 4.
func AssignAquaticsLanes(seeds []SwimmerSeed, poolLanes int) []SwimmerSeed {
	if len(seeds) == 0 {
		return nil
	}
	if poolLanes < 4 {
		poolLanes = 8
	}

	// Sort swimmers by qualifying time ascending (fastest first)
	sorted := make([]SwimmerSeed, len(seeds))
	copy(sorted, seeds)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SeedTimeMs < sorted[j].SeedTimeMs
	})

	var laneOrder []int
	switch {
	case poolLanes >= 10:
		laneOrder = []int{5, 6, 4, 7, 3, 8, 2, 9, 1, 10}
	case poolLanes >= 8:
		laneOrder = []int{4, 5, 3, 6, 2, 7, 1, 8}
	case poolLanes >= 6:
		laneOrder = []int{3, 4, 2, 5, 1, 6}
	default: // 4 lanes
		laneOrder = []int{2, 3, 1, 4}
	}

	lanesPerHeat := len(laneOrder)
	totalSwimmers := len(sorted)
	totalHeats := (totalSwimmers + lanesPerHeat - 1) / lanesPerHeat

	seeded := make([]SwimmerSeed, 0, totalSwimmers)

	// In championship swimming, the fastest heat (last heat) receives the fastest seeds
	for idx, s := range sorted {
		// Distribute across heats
		heat := totalHeats - (idx / lanesPerHeat)
		if heat < 1 {
			heat = 1
		}
		laneIdx := idx % lanesPerHeat
		lane := laneOrder[laneIdx]

		s.HeatNumber = heat
		s.LaneNumber = lane
		seeded = append(seeded, s)
	}

	return seeded
}
