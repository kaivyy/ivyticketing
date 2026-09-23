package timing

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RaceResultAdapter handles data exchange with RACE RESULT 12 software.
// It is strictly a translation layer between RACE RESULT protocols and IVY normalized models.
// It contains ZERO business scoring, ranking, or result projection logic.
type RaceResultAdapter struct{}

// NewRaceResultAdapter constructs a RaceResultAdapter.
func NewRaceResultAdapter() *RaceResultAdapter {
	return &RaceResultAdapter{}
}

func (a *RaceResultAdapter) Type() ProviderType {
	return ProviderRaceResult
}

func (a *RaceResultAdapter) ProviderName() string {
	return string(ProviderRaceResult)
}

// ParsePassings implements PassingParser interface.
func (a *RaceResultAdapter) ParsePassings(ctx context.Context, defaultCheckpoint string, payload []byte) ([]TimingPassing, error) {
	return a.ParsePassingStream(ctx, defaultCheckpoint, payload)
}

// rrJSONPassing represents the JSON payload when RACE RESULT Exporter is configured with JSON format.
type rrJSONPassing struct {
	PassingNo      any    `json:"PassingNo"`
	Transponder    string `json:"Transponder"`
	Bib            any    `json:"Bib"`
	TimingPoint    string `json:"TimingPoint"`
	Time           string `json:"Time"`
	Date           string `json:"Date"`
	DateTime       string `json:"DateTime"`
	Hits           int    `json:"Hits"`
	RSSI           int    `json:"RSSI"`
	Antenna        int    `json:"Antenna"`
	DeviceID       string `json:"DeviceID"`
}

// ParsePassingStream parses incoming HTTP Forwarding data from RACE RESULT 12 Exporter.
// It supports both JSON (single object or array) and standard delimited text (semicolon/comma/tab).
func (a *RaceResultAdapter) ParsePassingStream(_ context.Context, defaultCheckpoint string, payload []byte) ([]RawObservation, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, errors.New("empty passing payload")
	}

	// Try JSON first if starts with '{' or '['
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return parseRaceResultJSON(trimmed, defaultCheckpoint)
	}

	// Fallback to delimited text (lines from Exporter stream or passing file)
	return parseRaceResultDelimited(trimmed, defaultCheckpoint)
}

func parseRaceResultJSON(data []byte, defaultCheckpoint string) ([]RawObservation, error) {
	var list []rrJSONPassing
	if data[0] == '[' {
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("raceresult: invalid json array: %w", err)
		}
	} else {
		var single rrJSONPassing
		if err := json.Unmarshal(data, &single); err != nil {
			return nil, fmt.Errorf("raceresult: invalid json object: %w", err)
		}
		list = append(list, single)
	}

	observations := make([]RawObservation, 0, len(list))
	for _, p := range list {
		if strings.TrimSpace(p.Transponder) == "" {
			continue
		}
		obsTime, err := parseRaceResultTime(p.Date, p.Time, p.DateTime)
		if err != nil {
			// Fallback to current UTC if time cannot be parsed
			obsTime = time.Now().UTC()
		}

		cp := p.TimingPoint
		if strings.TrimSpace(cp) == "" {
			cp = defaultCheckpoint
		}

		readID := fmt.Sprintf("%v", p.PassingNo)
		if readID == "<nil>" || readID == "0" {
			readID = ""
		}

		var bibPtr *string
		if p.Bib != nil {
			b := strings.TrimSpace(fmt.Sprintf("%v", p.Bib))
			if b != "" && b != "<nil>" && b != "0" {
				bibPtr = &b
			}
		}

		observations = append(observations, RawObservation{
			ExternalReadID: readID,
			CheckpointCode: strings.ToUpper(strings.TrimSpace(cp)),
			ChipCode:       strings.ToUpper(strings.TrimSpace(p.Transponder)),
			BibNumber:      bibPtr,
			ObservedAt:     obsTime,
			RawPayload:     string(data),
			Metadata: map[string]any{
				"hits":     p.Hits,
				"rssi":     p.RSSI,
				"antenna":  p.Antenna,
				"deviceId": p.DeviceID,
			},
		})
	}
	return observations, nil
}

func parseRaceResultDelimited(data []byte, defaultCheckpoint string) ([]RawObservation, error) {
	lines := strings.Split(string(data), "\n")
	observations := make([]RawObservation, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Detect delimiter: semicolon, tab, or comma
		var parts []string
		switch {
		case strings.Contains(line, ";"):
			parts = strings.Split(line, ";")
		case strings.Contains(line, "\t"):
			parts = strings.Split(line, "\t")
		default:
			parts = strings.Split(line, ",")
		}

		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		// Standard RACE RESULT format variants:
		// 1) PassingNo;Transponder;Time;TimingPoint
		// 2) PassingNo;Transponder;Time;Date;Hits;RSSI;Antenna;TimingPoint
		// 3) Transponder;Time;TimingPoint
		if len(parts) < 2 {
			continue
		}

		var readID, chip, timeStr, dateStr, cp string
		var hits, rssi, ant int

		if len(parts) >= 4 {
			// Check if part[0] is numeric PassingNo
			if _, err := strconv.Atoi(parts[0]); err == nil {
				readID = parts[0]
				chip = parts[1]
				timeStr = parts[2]
				if len(parts) >= 8 {
					dateStr = parts[3]
					hits, _ = strconv.Atoi(parts[4])
					rssi, _ = strconv.Atoi(parts[5])
					ant, _ = strconv.Atoi(parts[6])
					cp = parts[7]
				} else {
					cp = parts[3]
				}
			} else {
				chip = parts[0]
				timeStr = parts[1]
				cp = parts[2]
			}
		} else if len(parts) == 3 {
			chip = parts[0]
			timeStr = parts[1]
			cp = parts[2]
		} else {
			chip = parts[0]
			timeStr = parts[1]
		}

		if chip == "" {
			continue
		}
		if cp == "" {
			cp = defaultCheckpoint
		}

		obsTime, err := parseRaceResultTime(dateStr, timeStr, "")
		if err != nil {
			obsTime = time.Now().UTC()
		}

		observations = append(observations, RawObservation{
			ExternalReadID: readID,
			CheckpointCode: strings.ToUpper(strings.TrimSpace(cp)),
			ChipCode:       strings.ToUpper(strings.TrimSpace(chip)),
			ObservedAt:     obsTime,
			RawPayload:     line,
			Metadata: map[string]any{
				"hits":    hits,
				"rssi":    rssi,
				"antenna": ant,
			},
		})
	}

	return observations, nil
}

func parseRaceResultTime(dateStr, timeStr, dateTimeStr string) (time.Time, error) {
	if dateTimeStr != "" {
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02 15:04:05.999",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
		}
		for _, f := range formats {
			if t, err := time.Parse(f, dateTimeStr); err == nil {
				return t.UTC(), nil
			}
		}
	}

	today := time.Now().UTC().Format("2006-01-02")
	if dateStr != "" {
		// e.g. "2026-09-17" or "17.09.2026"
		if strings.Contains(dateStr, ".") {
			p := strings.Split(dateStr, ".")
			if len(p) == 3 {
				dateStr = fmt.Sprintf("%s-%s-%s", p[2], p[1], p[0])
			}
		}
		today = dateStr
	}

	full := fmt.Sprintf("%s %s", today, timeStr)
	timeFormats := []string{
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, f := range timeFormats {
		if t, err := time.Parse(f, full); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, errors.New("unparseable time")
}

// FormatParticipantExport formats IVY participants to RACE RESULT 12 participant exchange CSV.
// Semicolon-separated format compatible with RACE RESULT 12 Participant Import.
func (a *RaceResultAdapter) FormatParticipantExport(_ context.Context, participants []ParticipantSyncRecord) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'

	header := []string{"BIB", "FIRSTNAME", "LASTNAME", "GENDER", "DATEOFBIRTH", "CONTEST", "TRANSPONDER1"}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, p := range participants {
		first, last := splitName(p.ParticipantName)
		dob := ""
		if p.DateOfBirth != nil {
			dob = p.DateOfBirth.Format("2006-01-02")
		}

		row := []string{
			p.BibNumber,
			first,
			last,
			p.Gender,
			dob,
			p.CategoryName,
			p.TransponderCode,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func splitName(fullName string) (first, last string) {
	parts := strings.Fields(strings.TrimSpace(fullName))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}
