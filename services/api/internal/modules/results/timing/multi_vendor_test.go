package timing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestProviderRegistry_DefaultAndCustom(t *testing.T) {
	reg := DefaultRegistry()
	providers := reg.ListProviders()
	if len(providers) < 3 {
		t.Fatalf("expected at least 3 default providers, got %d", len(providers))
	}

	rr, ok := reg.Get("RACE_RESULT")
	if !ok || rr == nil {
		t.Fatalf("expected RACE_RESULT provider to be registered")
	}

	csv, ok := reg.Get("GENERIC_CSV")
	if !ok || csv == nil {
		t.Fatalf("expected GENERIC_CSV provider to be registered")
	}

	vendor, ok := reg.Get("VENDOR_API")
	if !ok || vendor == nil {
		t.Fatalf("expected VENDOR_API provider to be registered")
	}

	// Register a mock custom adapter
	custom := NewVendorAPIAdapter()
	reg.Register("CUSTOM_CHRONOTRACK", custom)
	if _, ok := reg.Get("CUSTOM_CHRONOTRACK"); !ok {
		t.Fatalf("failed to retrieve registered custom provider")
	}
}

func TestVendorAPIAdapter_PassingsAndFinalResults(t *testing.T) {
	adapter := NewVendorAPIAdapter()
	ctx := context.Background()

	// JSON array payload with various casing
	payload := []byte(`[
		{"id": "read-01", "chipCode": "TAG-101", "bibNumber": "1001", "timestamp": "2026-09-17T06:05:10Z", "checkpoint": "START"},
		{"id": "read-02", "chip_code": "TAG-101", "bib_number": "1001", "timestamp": "2026-09-17T06:45:20Z", "checkpoint": "FINISH"}
	]`)

	passings, err := adapter.ParsePassingStream(ctx, "DEFAULT", payload)
	if err != nil {
		t.Fatalf("vendor api parse passing failed: %v", err)
	}
	if len(passings) != 2 {
		t.Fatalf("expected 2 passings, got %d", len(passings))
	}
	if passings[0].ExternalReadID != "read-01" || passings[0].ChipCode != "TAG-101" {
		t.Errorf("unexpected passing 0: %+v", passings[0])
	}
	if passings[1].CheckpointCode != "FINISH" {
		t.Errorf("unexpected checkpoint: %s", passings[1].CheckpointCode)
	}

	// Final results JSON parsing
	finalPayload := []byte(`[
		{"bibNumber": "1001", "participantName": "Budi", "gunTimeMs": 3600000, "chipTimeMs": 3550000, "status": "FINISHED", "overallRank": 1, "genderRank": 1, "categoryRank": 1},
		{"bibNumber": "1002", "participantName": "Citra", "gunTimeMs": 7200000, "chipTimeMs": 7100000, "status": "OTL"}
	]`)

	results, err := adapter.ParseFinalResults(ctx, finalPayload)
	if err != nil {
		t.Fatalf("vendor api parse final results failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].BibNumber != "1001" || results[0].Status != "FINISHED" || results[0].OverallRank == nil || *results[0].OverallRank != 1 {
		t.Errorf("unexpected result 0: %+v", results[0])
	}
	if results[1].Status != "OTL" {
		t.Errorf("unexpected result 1 status: %s", results[1].Status)
	}
}

func TestGenericCSVAdapter_PassingAndFinalResults(t *testing.T) {
	adapter := NewGenericCSVAdapter()
	ctx := context.Background()

	// Semicolon delimited CSV passings
	csvData := []byte("read_id;chip_code;checkpoint;timestamp\nREAD_A1;TAG_A1;START;2026-09-17T06:00:15Z\nREAD_A2;TAG_A1;FINISH;2026-09-17T06:50:30Z\n")
	passings, err := adapter.ParsePassingStream(ctx, "DEFAULT", csvData)
	if err != nil {
		t.Fatalf("csv parse passings failed: %v", err)
	}
	if len(passings) != 2 {
		t.Fatalf("expected 2 passings, got %d", len(passings))
	}
	if passings[0].ChipCode != "TAG_A1" || passings[0].CheckpointCode != "START" {
		t.Errorf("unexpected passing 0: %+v", passings[0])
	}

	// Final results CSV with duration strings and DSQ / OTL statuses
	finalCSV := []byte("bib,name,gun_time,chip_time,status\n2001,Andi,00:45:00,00:44:30,FINISHED\n2002,Bambang,01:10:00,01:09:00,DSQ\n2003,Caca,02:15:00,02:14:00,OTL\n")
	normResults, err := adapter.ParseFinalResults(ctx, finalCSV)
	if err != nil {
		t.Fatalf("csv parse final results failed: %v", err)
	}
	if len(normResults) != 3 {
		t.Fatalf("expected 3 results, got %d", len(normResults))
	}
	if normResults[0].BibNumber != "2001" || normResults[0].GunTimeMs == nil || *normResults[0].GunTimeMs != (45 * 60 * 1000) {
		t.Errorf("unexpected result 0: %+v", normResults[0])
	}
	if normResults[1].Status != "DSQ" {
		t.Errorf("expected DSQ, got %s", normResults[1].Status)
	}
	if normResults[2].Status != "OTL" {
		t.Errorf("expected OTL, got %s", normResults[2].Status)
	}
}

func TestProcessor_StrictGunTimeNoFallback(t *testing.T) {
	// Rule: If wave start time is absent, GunTimeMs must remain nil (no fake fallback to ChipTimeMs)
	proc := NewProcessor(10 * time.Second)
	ctx := context.Background()

	checkpoints := []CheckpointDef{
		{ID: uuid.New(), Code: "START", Type: "START", OrderIndex: 0},
		{ID: uuid.New(), Code: "FINISH", Type: "FINISH", OrderIndex: 1},
	}

	// WaveStartAt is nil!
	participants := map[string]ParticipantDef{
		"3001": {
			BibNumber:       "3001",
			ParticipantName: "No Wave Runner",
			WaveStartAt:     nil,
		},
	}
	chipToBib := map[string]string{"CHIP_NW": "3001"}

	t0 := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	passings := []PassingItem{
		{ID: 1, CheckpointCode: "START", ChipCode: "CHIP_NW", ObservedAt: t0},
		{ID: 2, CheckpointCode: "FINISH", ChipCode: "CHIP_NW", ObservedAt: t0.Add(40 * time.Minute)},
	}

	results := proc.ProcessEventPassings(ctx, checkpoints, participants, chipToBib, passings)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "FINISHED" {
		t.Errorf("expected FINISHED, got %s", r.Status)
	}
	if r.ChipTimeMs == nil || *r.ChipTimeMs != (40 * 60 * 1000) {
		t.Errorf("expected chip time 40m, got %v", r.ChipTimeMs)
	}
	if r.GunTimeMs != nil {
		t.Errorf("CRITICAL: GunTimeMs must be nil when wave start time is missing, but got %v", *r.GunTimeMs)
	}
}

func TestProcessor_CheckpointAliasesAndScoringPolicy(t *testing.T) {
	proc := NewProcessor(10 * time.Second)
	ctx := context.Background()

	finishID := uuid.New()
	checkpoints := []CheckpointDef{
		{
			ID:         finishID,
			Code:       "FINISH",
			Type:       "FINISH",
			OrderIndex: 1,
			Aliases:    []string{"MAT_FINISH_01", "LINE_F"},
		},
	}

	waveStart := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	participants := map[string]ParticipantDef{
		"4001": {
			BibNumber:       "4001",
			ParticipantName: "Alias Runner",
			WaveStartAt:     &waveStart,
		},
	}
	chipToBib := map[string]string{"TAG_ALIAS": "4001"}

	// Passing uses alias "MAT_FINISH_01" instead of canonical "FINISH"
	passings := []PassingItem{
		{ID: 100, CheckpointCode: "MAT_FINISH_01", ChipCode: "TAG_ALIAS", ObservedAt: waveStart.Add(35 * time.Minute)},
	}

	policy := EventPolicy{
		DebounceWindowSeconds: 5,
		CutoffMinutes:         30, // Cutoff at 30 mins, runner took 35 mins -> OTL!
		AllowMissingStart:     true,
	}
	proc.WithPolicy(policy)

	results := proc.ProcessEventPassings(ctx, checkpoints, participants, chipToBib, passings)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "OTL" {
		t.Errorf("expected status OTL because time exceeded CutoffMinutes, got %s", r.Status)
	}
}
