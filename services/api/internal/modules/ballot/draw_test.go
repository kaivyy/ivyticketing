package ballot

import (
	"fmt"
	"testing"
)

func TestShuffle_Deterministic(t *testing.T) {
	entries := make([]DrawEntry, 100)
	for i := range entries {
		entries[i] = DrawEntry{ID: fmt.Sprintf("entry-%d", i)}
	}
	seed := "test-seed-abc"
	first := Shuffle(seed, entries)
	for i := 0; i < 999; i++ {
		result := Shuffle(seed, entries)
		for j := range result {
			if result[j].ID != first[j].ID {
				t.Fatalf("shuffle not deterministic at run %d, position %d", i, j)
			}
		}
	}
}

func TestShuffle_NoDuplicates(t *testing.T) {
	entries := make([]DrawEntry, 1000)
	for i := range entries {
		entries[i] = DrawEntry{ID: fmt.Sprintf("e-%d", i)}
	}
	result := Shuffle("seed-xyz", entries)
	seen := map[string]bool{}
	for _, e := range result {
		if seen[e.ID] {
			t.Fatalf("duplicate entry %s in shuffle result", e.ID)
		}
		seen[e.ID] = true
	}
	if len(seen) != 1000 {
		t.Fatalf("expected 1000 entries, got %d", len(seen))
	}
}

func TestAssign_CorrectOutcomes(t *testing.T) {
	entries := make([]DrawEntry, 10)
	for i := range entries {
		entries[i] = DrawEntry{ID: fmt.Sprintf("e-%d", i)}
	}
	results := Assign("seed", entries, 3, 2)
	winners, waitlisted, notSelected := 0, 0, 0
	for _, r := range results {
		switch r.Outcome {
		case OutcomeWinner:
			winners++
		case OutcomeWaitlisted:
			waitlisted++
		case OutcomeNotSelected:
			notSelected++
		}
	}
	if winners != 3 {
		t.Fatalf("want 3 winners, got %d", winners)
	}
	if waitlisted != 2 {
		t.Fatalf("want 2 waitlisted, got %d", waitlisted)
	}
	if notSelected != 5 {
		t.Fatalf("want 5 not_selected, got %d", notSelected)
	}
}

func TestAssign_ResultHashUnique(t *testing.T) {
	entries := make([]DrawEntry, 50)
	for i := range entries {
		entries[i] = DrawEntry{ID: fmt.Sprintf("e-%d", i)}
	}
	results := Assign("seed-hash-test", entries, 10, 10)
	seen := map[string]bool{}
	for _, r := range results {
		if seen[r.ResultHash] {
			t.Fatalf("duplicate result_hash: %s", r.ResultHash)
		}
		seen[r.ResultHash] = true
	}
}
