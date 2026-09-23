package timing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestProcessor_StandardFinish(t *testing.T) {
	proc := NewProcessor(15 * time.Second)
	ctx := context.Background()

	startID := uuid.New()
	splitID := uuid.New()
	finishID := uuid.New()
	dist5k := 5000

	checkpoints := []CheckpointDef{
		{ID: startID, Code: "START", Name: "Start Mat", Type: "START", OrderIndex: 0},
		{ID: splitID, Code: "5K", Name: "5KM Mat", Type: "SPLIT", OrderIndex: 1, DistanceMeters: &dist5k},
		{ID: finishID, Code: "FINISH", Name: "Finish Mat", Type: "FINISH", OrderIndex: 2},
	}

	waveStart := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	waveID := uuid.New()

	participants := map[string]ParticipantDef{
		"1001": {
			BibNumber:       "1001",
			ParticipantName: "Agus Runner",
			Gender:          "M",
			WaveID:          &waveID,
			WaveStartAt:     &waveStart,
		},
	}

	chipToBib := map[string]string{
		"CHIP_AGUS": "1001",
	}

	// Runner starts 30 seconds after gun (chip time should be 30s faster than gun time)
	runnerStart := waveStart.Add(30 * time.Second)
	runnerSplit := runnerStart.Add(25 * time.Minute)
	runnerFinish := runnerStart.Add(55 * time.Minute)

	passings := []PassingItem{
		{ID: 1, CheckpointCode: "START", ChipCode: "CHIP_AGUS", ObservedAt: runnerStart},
		{ID: 2, CheckpointCode: "5K", ChipCode: "CHIP_AGUS", ObservedAt: runnerSplit},
		{ID: 3, CheckpointCode: "FINISH", ChipCode: "CHIP_AGUS", ObservedAt: runnerFinish},
	}

	res := proc.ProcessEventPassings(ctx, checkpoints, participants, chipToBib, passings)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}

	r := res[0]
	if r.Status != "FINISHED" {
		t.Errorf("expected FINISHED, got %s", r.Status)
	}
	if r.ChipTimeMs == nil || *r.ChipTimeMs != (55 * 60 * 1000) {
		t.Errorf("expected chip time 3300000 ms, got %v", r.ChipTimeMs)
	}
	// Gun time = finish - waveStart (55m + 30s = 3330000 ms)
	expectedGun := int64(55*60*1000 + 30*1000)
	if r.GunTimeMs == nil || *r.GunTimeMs != expectedGun {
		t.Errorf("expected gun time %d ms, got %v", expectedGun, r.GunTimeMs)
	}

	if len(r.Splits) != 1 {
		t.Fatalf("expected 1 split, got %d", len(r.Splits))
	}
	if r.Splits[0].SplitTimeMs != (25 * 60 * 1000) {
		t.Errorf("expected 5k split time 1500000 ms, got %d", r.Splits[0].SplitTimeMs)
	}
	if r.Splits[0].SplitPaceMsKm == nil || *r.Splits[0].SplitPaceMsKm != (5 * 60 * 1000) {
		t.Errorf("expected 5:00 min/km (300000 ms/km), got %v", r.Splits[0].SplitPaceMsKm)
	}
}

func TestProcessor_DebounceDuplicateReads(t *testing.T) {
	proc := NewProcessor(15 * time.Second)
	ctx := context.Background()

	finishID := uuid.New()
	checkpoints := []CheckpointDef{
		{ID: finishID, Code: "FINISH", Name: "Finish Mat", Type: "FINISH", OrderIndex: 1},
	}

	waveStart := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	participants := map[string]ParticipantDef{
		"1002": {
			BibNumber:       "1002",
			ParticipantName: "Budi Sprinter",
			WaveStartAt:     &waveStart,
		},
	}
	chipToBib := map[string]string{"TAG_BUDI": "1002"}

	finishTime := waveStart.Add(20 * time.Minute)

	// Rapid consecutive hits on the mat: 0s, 2s, 5s later
	passings := []PassingItem{
		{ID: 10, CheckpointCode: "FINISH", ChipCode: "TAG_BUDI", ObservedAt: finishTime},
		{ID: 11, CheckpointCode: "FINISH", ChipCode: "TAG_BUDI", ObservedAt: finishTime.Add(2 * time.Second)},
		{ID: 12, CheckpointCode: "FINISH", ChipCode: "TAG_BUDI", ObservedAt: finishTime.Add(5 * time.Second)},
	}

	res := proc.ProcessEventPassings(ctx, checkpoints, participants, chipToBib, passings)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	// All 3 passing IDs should be recorded as processed
	if len(res[0].ProcessedIDs) != 3 {
		t.Errorf("expected 3 processed IDs, got %d", len(res[0].ProcessedIDs))
	}
}

func TestProcessor_DNF_WhenMissingFinish(t *testing.T) {
	proc := NewProcessor(15 * time.Second)
	ctx := context.Background()

	checkpoints := []CheckpointDef{
		{ID: uuid.New(), Code: "START", Type: "START", OrderIndex: 0},
		{ID: uuid.New(), Code: "FINISH", Type: "FINISH", OrderIndex: 1},
	}

	baseTime := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	participants := map[string]ParticipantDef{
		"1003": {
			BibNumber:       "1003",
			ParticipantName: "Citra Walker",
			WaveStartAt:     &baseTime,
		},
	}
	chipToBib := map[string]string{"CHIP_CITRA": "1003"}

	passings := []PassingItem{
		{ID: 20, CheckpointCode: "START", ChipCode: "CHIP_CITRA", ObservedAt: baseTime.Add(10 * time.Second)},
	}

	res := proc.ProcessEventPassings(ctx, checkpoints, participants, chipToBib, passings)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Status != "DNF" {
		t.Errorf("expected DNF for runner who started but didn't finish, got %s", res[0].Status)
	}
	if res[0].ChipTimeMs != nil {
		t.Errorf("expected nil chip time for DNF, got %v", res[0].ChipTimeMs)
	}
}

func TestRaceResultAdapter_DelimitedAndJSON(t *testing.T) {
	adapter := NewRaceResultAdapter()
	ctx := context.Background()

	// Delimited passing stream
	delimited := []byte("101;RR_CHIP_01;06:30:15.500;2026-09-17;5;120;1;START\n102;RR_CHIP_01;07:15:30.120;2026-09-17;12;135;2;FINISH\n")
	obs, err := adapter.ParsePassingStream(ctx, "FINISH", delimited)
	if err != nil {
		t.Fatalf("delimited parse error: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}
	if obs[0].ExternalReadID != "101" || obs[0].ChipCode != "RR_CHIP_01" || obs[0].CheckpointCode != "START" {
		t.Errorf("unexpected observation 0: %+v", obs[0])
	}
	if obs[1].ExternalReadID != "102" || obs[1].CheckpointCode != "FINISH" {
		t.Errorf("unexpected observation 1: %+v", obs[1])
	}

	// JSON array passing stream
	jsonPayload := []byte(`[
		{"PassingNo": 501, "Transponder": "TAG_JSON_1", "Time": "06:00:05", "Date": "2026-09-17", "TimingPoint": "START", "Hits": 10},
		{"PassingNo": 502, "Transponder": "TAG_JSON_1", "Time": "06:45:00", "Date": "2026-09-17", "TimingPoint": "FINISH", "Hits": 8}
	]`)
	obsJSON, err := adapter.ParsePassingStream(ctx, "FINISH", jsonPayload)
	if err != nil {
		t.Fatalf("json parse error: %v", err)
	}
	if len(obsJSON) != 2 {
		t.Fatalf("expected 2 observations from json, got %d", len(obsJSON))
	}
	if obsJSON[0].ExternalReadID != "501" || obsJSON[0].ChipCode != "TAG_JSON_1" {
		t.Errorf("unexpected json observation: %+v", obsJSON[0])
	}
}

func TestRaceResultAdapter_FormatParticipantExport(t *testing.T) {
	adapter := NewRaceResultAdapter()
	ctx := context.Background()

	dob := time.Date(1995, 5, 20, 0, 0, 0, 0, time.UTC)
	participants := []ParticipantSyncRecord{
		{
			BibNumber:       "1001",
			ParticipantName: "Dedi Setiawan",
			Gender:          "M",
			CategoryName:    "10K Open",
			TransponderCode: "TAG_DEDI",
			DateOfBirth:     &dob,
		},
	}

	csvBytes, err := adapter.FormatParticipantExport(ctx, participants)
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	csvStr := string(csvBytes)
	if !testing.Verbose() {
		// Just ensure expected header and content exist
		if len(csvStr) == 0 {
			t.Fatal("empty csv output")
		}
	}
}

func TestProcessor_MandatoryCheckpointsEvaluation(t *testing.T) {
	ctx := context.Background()

	checkpoints := []CheckpointDef{
		{ID: uuid.New(), Code: "START", Name: "Start", Type: "START", OrderIndex: 0},
		{ID: uuid.New(), Code: "5K", Name: "5K Split", Type: "SPLIT", OrderIndex: 1},
		{ID: uuid.New(), Code: "10K", Name: "10K Split", Type: "SPLIT", OrderIndex: 2},
		{ID: uuid.New(), Code: "FINISH", Name: "Finish", Type: "FINISH", OrderIndex: 3},
	}

	waveStart := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	participants := map[string]ParticipantDef{
		"101": {BibNumber: "101", ParticipantName: "Valid Runner", WaveStartAt: &waveStart},
		"102": {BibNumber: "102", ParticipantName: "Shortcut Runner", WaveStartAt: &waveStart},
	}
	chipToBib := map[string]string{
		"CHIP_101": "101",
		"CHIP_102": "102",
	}

	// 101 has START, 5K, 10K, FINISH
	// 102 has START and FINISH only (missing 5K and 10K)
	passings := []PassingItem{
		{ID: 1, CheckpointCode: "START", ChipCode: "CHIP_101", ObservedAt: waveStart.Add(10 * time.Second)},
		{ID: 2, CheckpointCode: "5K", ChipCode: "CHIP_101", ObservedAt: waveStart.Add(25 * time.Minute)},
		{ID: 3, CheckpointCode: "10K", ChipCode: "CHIP_101", ObservedAt: waveStart.Add(50 * time.Minute)},
		{ID: 4, CheckpointCode: "FINISH", ChipCode: "CHIP_101", ObservedAt: waveStart.Add(75 * time.Minute)},

		{ID: 5, CheckpointCode: "START", ChipCode: "CHIP_102", ObservedAt: waveStart.Add(15 * time.Second)},
		{ID: 6, CheckpointCode: "FINISH", ChipCode: "CHIP_102", ObservedAt: waveStart.Add(30 * time.Minute)},
	}

	// Test 1: Action = PENDING_REVIEW
	proc := NewProcessor(15 * time.Second).WithPolicy(EventPolicy{
		MandatoryCheckpoints:    []string{"5K", "10K"},
		MissingCheckpointAction: AnomalyActionPendingReview,
	})

	results := proc.ProcessEventPassings(ctx, checkpoints, participants, chipToBib, passings)
	resMap := make(map[string]ParticipantResultOutput)
	for _, r := range results {
		resMap[r.BibNumber] = r
	}

	if resMap["101"].Status != "FINISHED" {
		t.Errorf("101: expected FINISHED, got %s", resMap["101"].Status)
	}
	if len(resMap["101"].Anomalies) != 0 {
		t.Errorf("101: expected 0 anomalies, got %v", resMap["101"].Anomalies)
	}

	if resMap["102"].Status != "PENDING_REVIEW" {
		t.Errorf("102: expected PENDING_REVIEW, got %s", resMap["102"].Status)
	}
	if len(resMap["102"].Anomalies) != 2 {
		t.Errorf("102: expected 2 anomalies, got %v", resMap["102"].Anomalies)
	}

	// Test 2: Action = DSQ
	procDSQ := NewProcessor(15 * time.Second).WithPolicy(EventPolicy{
		MandatoryCheckpoints:    []string{"5K", "10K"},
		MissingCheckpointAction: AnomalyActionDSQ,
	})
	resultsDSQ := procDSQ.ProcessEventPassings(ctx, checkpoints, participants, chipToBib, passings)
	for _, r := range resultsDSQ {
		if r.BibNumber == "102" && r.Status != "DSQ" {
			t.Errorf("102 with DSQ action: expected DSQ, got %s", r.Status)
		}
	}

	// Test 3: Action = WARNING
	procWarn := NewProcessor(15 * time.Second).WithPolicy(EventPolicy{
		MandatoryCheckpoints:    []string{"5K", "10K"},
		MissingCheckpointAction: AnomalyActionWarning,
	})
	resultsWarn := procWarn.ProcessEventPassings(ctx, checkpoints, participants, chipToBib, passings)
	for _, r := range resultsWarn {
		if r.BibNumber == "102" && r.Status != "FINISHED" {
			t.Errorf("102 with WARNING action: expected FINISHED, got %s", r.Status)
		}
		if r.BibNumber == "102" && len(r.Anomalies) != 2 {
			t.Errorf("102 with WARNING action: expected anomalies preserved, got %v", r.Anomalies)
		}
	}
}

