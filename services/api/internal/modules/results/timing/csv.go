package timing

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// CSVAdapter handles timing passings ingested from generic CSV files.
type CSVAdapter struct{}

// NewCSVAdapter constructs a CSVAdapter.
func NewCSVAdapter() *CSVAdapter {
	return &CSVAdapter{}
}

// NewGenericCSVAdapter is an alias to NewCSVAdapter for naming consistency with provider types.
func NewGenericCSVAdapter() *CSVAdapter {
	return NewCSVAdapter()
}

func (a *CSVAdapter) Type() ProviderType {
	return ProviderGenericCSV
}

func (a *CSVAdapter) ProviderName() string {
	return string(ProviderGenericCSV)
}

func detectDelimiter(payload []byte) rune {
	firstLine := payload
	if idx := bytes.IndexByte(payload, '\n'); idx != -1 {
		firstLine = payload[:idx]
	}
	semis := bytes.Count(firstLine, []byte(";"))
	tabs := bytes.Count(firstLine, []byte("\t"))
	commas := bytes.Count(firstLine, []byte(","))

	if semis > commas && semis > tabs {
		return ';'
	}
	if tabs > commas && tabs > semis {
		return '\t'
	}
	return ','
}

// ParsePassings implements PassingParser interface.
func (a *CSVAdapter) ParsePassings(ctx context.Context, defaultCheckpoint string, payload []byte) ([]TimingPassing, error) {
	return a.ParsePassingStream(ctx, defaultCheckpoint, payload)
}

// ParsePassingStream parses CSV passings.
// Expected columns: bib/chip, checkpoint, time, [passing_id]
func (a *CSVAdapter) ParsePassingStream(_ context.Context, defaultCheckpoint string, payload []byte) ([]RawObservation, error) {
	r := csv.NewReader(bytes.NewReader(payload))
	r.Comma = detectDelimiter(payload)
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	// Read header
	header, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("empty csv")
		}
		return nil, err
	}

	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[strings.ToLower(strings.TrimSpace(col))] = i
	}

	observations := make([]RawObservation, 0)
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			continue
		}

		get := func(names ...string) string {
			for _, name := range names {
				if idx, ok := colIdx[name]; ok && idx < len(rec) {
					return strings.TrimSpace(rec[idx])
				}
			}
			return ""
		}

		chip := get("chip", "transponder", "chip_code", "bib")
		if chip == "" {
			continue
		}

		cp := get("checkpoint", "point", "mat")
		if cp == "" {
			cp = defaultCheckpoint
		}

		timeStr := get("time", "observed_at", "timestamp")
		t, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05.999", timeStr)
			if err != nil {
				t, err = time.Parse("2006-01-02 15:04:05", timeStr)
				if err != nil {
					t = time.Now().UTC()
				}
			}
		}

		readID := get("id", "passing_id", "external_id")

		observations = append(observations, RawObservation{
			ExternalReadID: readID,
			CheckpointCode: strings.ToUpper(cp),
			ChipCode:       strings.ToUpper(chip),
			ObservedAt:     t.UTC(),
			RawPayload:     strings.Join(rec, ","),
		})
	}

	return observations, nil
}

// FormatParticipantExport formats participants as standard CSV.
func (a *CSVAdapter) FormatParticipantExport(_ context.Context, participants []ParticipantSyncRecord) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"bib", "name", "gender", "category", "chip"}); err != nil {
		return nil, err
	}
	for _, p := range participants {
		if err := w.Write([]string{p.BibNumber, p.ParticipantName, p.Gender, p.CategoryName, p.TransponderCode}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// ParseFinalResults decodes a results CSV into normalized race results.
func (a *CSVAdapter) ParseFinalResults(_ context.Context, payload []byte) ([]NormalizedResult, error) {
	r := csv.NewReader(bytes.NewReader(payload))
	r.Comma = detectDelimiter(payload)
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, err
	}

	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[strings.ToLower(strings.TrimSpace(col))] = i
	}

	results := make([]NormalizedResult, 0)
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			continue
		}

		get := func(names ...string) string {
			for _, name := range names {
				if idx, ok := colIdx[name]; ok && idx < len(rec) {
					return strings.TrimSpace(rec[idx])
				}
			}
			return ""
		}

		bib := get("bib", "bib_number", "number")
		if bib == "" {
			continue
		}

		name := get("name", "participant_name", "runner")
		gender := strings.ToUpper(get("gender", "sex"))
		status := strings.ToUpper(get("status"))
		if status == "" {
			status = "FINISHED"
		}

		gunStr := get("gun_time", "guntime", "gun")
		var gunMs *int64
		if gunStr != "" {
			if ms, err := parseDurationToMs(gunStr); err == nil {
				gunMs = &ms
			}
		}

		chipStr := get("chip_time", "chiptime", "net_time", "net")
		var chipMs *int64
		if chipStr != "" {
			if ms, err := parseDurationToMs(chipStr); err == nil {
				chipMs = &ms
			}
		}

		results = append(results, NormalizedResult{
			BibNumber:       bib,
			ParticipantName: name,
			Gender:          gender,
			Status:          status,
			GunTimeMs:       gunMs,
			ChipTimeMs:      chipMs,
		})
	}

	return results, nil
}

// ParseChipMappings decodes a CSV of BIB <-> transponder chip mappings.
func (a *CSVAdapter) ParseChipMappings(_ context.Context, payload []byte) ([]ChipMappingRecord, error) {
	r := csv.NewReader(bytes.NewReader(payload))
	r.Comma = detectDelimiter(payload)
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, err
	}

	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[strings.ToLower(strings.TrimSpace(col))] = i
	}

	mappings := make([]ChipMappingRecord, 0)
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			continue
		}

		get := func(names ...string) string {
			for _, name := range names {
				if idx, ok := colIdx[name]; ok && idx < len(rec) {
					return strings.TrimSpace(rec[idx])
				}
			}
			return ""
		}

		bib := get("bib", "bib_number")
		chip := get("chip", "transponder", "chip_code")
		if bib == "" || chip == "" {
			continue
		}

		mappings = append(mappings, ChipMappingRecord{
			BibNumber: bib,
			ChipCode:  strings.ToUpper(chip),
			Notes:     get("notes", "note"),
		})
	}

	return mappings, nil
}

func parseDurationToMs(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty duration")
	}
	// Check if purely numeric (milliseconds)
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return ms, nil
	}
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		h, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, err
		}
		m, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, err
		}
		secParts := strings.Split(parts[2], ".")
		sec, err := strconv.ParseInt(secParts[0], 10, 64)
		if err != nil {
			return 0, err
		}
		var millis int64
		if len(secParts) > 1 {
			msStr := secParts[1]
			if len(msStr) > 3 {
				msStr = msStr[:3]
			}
			for len(msStr) < 3 {
				msStr += "0"
			}
			millis, _ = strconv.ParseInt(msStr, 10, 64)
		}
		return (h*3600+m*60+sec)*1000 + millis, nil
	} else if len(parts) == 2 {
		m, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, err
		}
		secParts := strings.Split(parts[1], ".")
		sec, err := strconv.ParseInt(secParts[0], 10, 64)
		if err != nil {
			return 0, err
		}
		var millis int64
		if len(secParts) > 1 {
			msStr := secParts[1]
			if len(msStr) > 3 {
				msStr = msStr[:3]
			}
			for len(msStr) < 3 {
				msStr += "0"
			}
			millis, _ = strconv.ParseInt(msStr, 10, 64)
		}
		return (m*60+sec)*1000 + millis, nil
	}
	return 0, fmt.Errorf("unknown duration format %s", s)
}
