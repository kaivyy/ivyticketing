package results

import (
	"time"

	"github.com/google/uuid"
)

type TimingConfigView struct {
	ID                   uuid.UUID      `json:"id"`
	EventID              uuid.UUID      `json:"eventId"`
	Provider             string         `json:"provider"`
	Transport            string         `json:"transport"`
	SyncMode             string         `json:"syncMode"`
	Policy               map[string]any `json:"policy,omitempty"`
	IngestionTokenPrefix string         `json:"ingestionTokenPrefix"`
	ExternalRaceID       string         `json:"externalRaceId,omitempty"`
	IsActive             bool           `json:"isActive"`
	Settings             map[string]any `json:"settings,omitempty"`
}

type CheckpointView struct {
	ID              uuid.UUID           `json:"id"`
	EventID         uuid.UUID           `json:"eventId"`
	Code            string              `json:"code"`
	Name            string              `json:"name"`
	CheckpointType  string              `json:"checkpointType"`
	OrderIndex      int                 `json:"orderIndex"`
	DistanceMeters  *int                `json:"distanceMeters,omitempty"`
	Aliases         []string            `json:"aliases,omitempty"`
	ProviderAliases map[string][]string `json:"providerAliases,omitempty"`
}

type WaveView struct {
	ID         uuid.UUID  `json:"id"`
	EventID    uuid.UUID  `json:"eventId"`
	CategoryID *uuid.UUID `json:"categoryId,omitempty"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	StartAt    *time.Time `json:"startAt,omitempty"`
	OrderIndex int        `json:"orderIndex"`
}

type MappingView struct {
	ID              uuid.UUID `json:"id"`
	EventID         uuid.UUID `json:"eventId"`
	BibNumber       string    `json:"bibNumber"`
	TransponderCode string    `json:"transponderCode"`
	IsActive        bool      `json:"isActive"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes,omitempty"`
}

type PassingStatsView struct {
	Total          int64 `json:"total"`
	ProcessedCount int64 `json:"processedCount"`
	PendingCount   int64 `json:"pendingCount"`
}

type SplitView struct {
	ID             uuid.UUID `json:"id"`
	CheckpointCode string    `json:"checkpointCode"`
	CheckpointName string    `json:"checkpointName"`
	DistanceMeters *int      `json:"distanceMeters,omitempty"`
	SplitTimeMs    int64     `json:"splitTimeMs"`
	SplitTime      string    `json:"splitTime"`
	Pace           string    `json:"pace,omitempty"`
	PassingTime    time.Time `json:"passingTime"`
}

type TimingProcessSummary struct {
	PassingsEvaluated int  `json:"passingsEvaluated"`
	ResultsUpdated    int  `json:"resultsUpdated"`
	Ranked            bool `json:"ranked"`
}
