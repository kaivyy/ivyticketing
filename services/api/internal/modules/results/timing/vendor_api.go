package timing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// VendorAPIAdapter parses standard JSON payloads from third-party timing systems.
type VendorAPIAdapter struct{}

// NewVendorAPIAdapter creates an adapter for open VENDOR_API integrations.
func NewVendorAPIAdapter() *VendorAPIAdapter {
	return &VendorAPIAdapter{}
}

func (a *VendorAPIAdapter) ProviderName() string {
	return string(ProviderVendorAPI)
}

func (a *VendorAPIAdapter) Type() ProviderType {
	return ProviderVendorAPI
}

// VendorAPIPassingPayload is the expected JSON schema for passing ingestion.
type VendorAPIPassingPayload struct {
	ID             string         `json:"id,omitempty"`
	ExternalReadID string         `json:"externalReadId,omitempty"`
	ExternalID     string         `json:"externalId,omitempty"`
	CheckpointCode string         `json:"checkpointCode,omitempty"`
	Checkpoint     string         `json:"checkpoint,omitempty"`
	ChipCode       string         `json:"chipCode,omitempty"`
	SnakeChipCode  string         `json:"chip_code,omitempty"`
	Transponder    string         `json:"transponder,omitempty"`
	RawChipCode    string         `json:"rawChipCode,omitempty"`
	BibNumber      *string        `json:"bibNumber,omitempty"`
	SnakeBibNumber *string        `json:"bib_number,omitempty"`
	ObservedAt     string         `json:"observedAt,omitempty"`
	Timestamp      string         `json:"timestamp,omitempty"`
	Time           string         `json:"time,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// ParsePassings decodes an open JSON array or wrapper of passings into normalized passings.
func (a *VendorAPIAdapter) ParsePassings(ctx context.Context, defaultCheckpoint string, payload []byte) ([]TimingPassing, error) {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return nil, errors.New("empty vendor api payload")
	}

	var items []VendorAPIPassingPayload
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(payload, &items); err != nil {
			return nil, fmt.Errorf("decode vendor api passing array: %w", err)
		}
	} else if strings.HasPrefix(trimmed, "{") {
		// Support wrapper format e.g. {"passings": [...]} or single object
		var wrapper struct {
			Passings []VendorAPIPassingPayload `json:"passings"`
		}
		if err := json.Unmarshal(payload, &wrapper); err == nil && len(wrapper.Passings) > 0 {
			items = wrapper.Passings
		} else {
			var single VendorAPIPassingPayload
			if err := json.Unmarshal(payload, &single); err != nil {
				return nil, fmt.Errorf("decode vendor api passing object: %w", err)
			}
			items = []VendorAPIPassingPayload{single}
		}
	} else {
		return nil, errors.New("invalid vendor api json payload")
	}

	now := time.Now().UTC()
	results := make([]TimingPassing, 0, len(items))

	for _, it := range items {
		extID := it.ExternalReadID
		if extID == "" {
			extID = it.ExternalID
		}
		if extID == "" {
			extID = it.ID
		}

		chip := strings.ToUpper(strings.TrimSpace(it.ChipCode))
		if chip == "" {
			chip = strings.ToUpper(strings.TrimSpace(it.SnakeChipCode))
		}
		if chip == "" {
			chip = strings.ToUpper(strings.TrimSpace(it.Transponder))
		}
		if chip == "" {
			continue
		}

		cp := strings.TrimSpace(it.CheckpointCode)
		if cp == "" {
			cp = strings.TrimSpace(it.Checkpoint)
		}
		if cp == "" {
			cp = defaultCheckpoint
		}

		timeStr := it.ObservedAt
		if timeStr == "" {
			timeStr = it.Timestamp
		}
		if timeStr == "" {
			timeStr = it.Time
		}

		obsTime, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			obsTime, err = time.Parse("2006-01-02 15:04:05.000", timeStr)
			if err != nil {
				obsTime, err = time.Parse("2006-01-02 15:04:05", timeStr)
				if err != nil {
					obsTime = now
				}
			}
		}

		rawChip := it.RawChipCode
		if rawChip == "" {
			rawChip = chip
		}

		bib := it.BibNumber
		if bib == nil {
			bib = it.SnakeBibNumber
		}

		results = append(results, TimingPassing{
			ExternalReadID:  extID,
			ExternalID:      extID,
			CheckpointCode:  cp,
			ChipCode:        chip,
			RawChipCode:     rawChip,
			BibNumber:       bib,
			ObservedAt:      obsTime.UTC(),
			ReceivedAt:      now,
			SourceProvider:  string(ProviderVendorAPI),
			TransportMethod: string(TransportHTTPPush),
			Metadata:        it.Metadata,
		})
	}

	return results, nil
}

// ParseFinalResults decodes open JSON array of final results.
func (a *VendorAPIAdapter) ParseFinalResults(ctx context.Context, payload []byte) ([]NormalizedResult, error) {
	var items []NormalizedResult
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil, fmt.Errorf("decode vendor api final results: %w", err)
	}

	for i := range items {
		items[i].BibNumber = strings.TrimSpace(items[i].BibNumber)
		items[i].ParticipantName = strings.TrimSpace(items[i].ParticipantName)
		items[i].Gender = strings.ToUpper(strings.TrimSpace(items[i].Gender))
		items[i].Status = strings.ToUpper(strings.TrimSpace(items[i].Status))
		if items[i].Status == "" {
			items[i].Status = "FINISHED"
		}
	}

	return items, nil
}

// Backward compatibility methods for TimingProvider interface
func (a *VendorAPIAdapter) ParsePassingStream(ctx context.Context, defaultCheckpoint string, payload []byte) ([]RawObservation, error) {
	return a.ParsePassings(ctx, defaultCheckpoint, payload)
}

func (a *VendorAPIAdapter) FormatParticipantExport(ctx context.Context, participants []ParticipantSyncRecord) ([]byte, error) {
	return json.MarshalIndent(participants, "", "  ")
}
