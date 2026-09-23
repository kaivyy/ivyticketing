package timing

import (
	"context"
	"time"
)

// ProviderType identifies the timing hardware or protocol vendor.
type ProviderType string

const (
	ProviderRaceResult ProviderType = "RACE_RESULT"
	ProviderGenericCSV ProviderType = "GENERIC_CSV"
	ProviderVendorAPI  ProviderType = "VENDOR_API"
	ProviderNativeRFID ProviderType = "NATIVE_RFID"
	ProviderManual     ProviderType = "MANUAL"

	// Backward compatibility aliases
	ProviderTypeRaceResult = ProviderRaceResult
	ProviderTypeCSV        = ProviderGenericCSV
	ProviderTypeNative     = ProviderNativeRFID
)

// TransportType identifies the transmission mechanism.
type TransportType string

const (
	TransportHTTPPush   TransportType = "HTTP_PUSH"
	TransportHTTPPull   TransportType = "HTTP_PULL"
	TransportCSVUpload  TransportType = "CSV_UPLOAD"
	TransportLocalAgent TransportType = "LOCAL_AGENT"
	TransportSFTP       TransportType = "SFTP"
)

// SyncMode indicates whether the integration targets raw passing observations or final results.
type SyncMode string

const (
	SyncModeRawPassings  SyncMode = "RAW_PASSINGS"
	SyncModeFinalResults SyncMode = "FINAL_RESULTS"
)

// TimingPassing represents a normalized raw observation from a timing antenna/mat.
type TimingPassing struct {
	ExternalReadID  string         `json:"externalReadId,omitempty"`
	ExternalID      string         `json:"externalId,omitempty"`
	CheckpointCode  string         `json:"checkpointCode"`
	ChipCode        string         `json:"chipCode"`
	RawChipCode     string         `json:"rawChipCode,omitempty"`
	BibNumber       *string        `json:"bibNumber,omitempty"`
	ObservedAt      time.Time      `json:"observedAt"`
	ReceivedAt      time.Time      `json:"receivedAt"`
	SourceProvider  string         `json:"sourceProvider"`
	TransportMethod string         `json:"transportMethod"`
	RawPayload      string         `json:"rawPayload,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// RawObservation is an alias to TimingPassing for backward compatibility.
type RawObservation = TimingPassing

// NormalizedResult represents a final finisher result directly provided by an external vendor.
type NormalizedResult struct {
	BibNumber       string     `json:"bibNumber"`
	ParticipantName string     `json:"participantName"`
	Gender          string     `json:"gender"`
	Age             *int       `json:"age,omitempty"`
	AgeGroup        string     `json:"ageGroup,omitempty"`
	Status          string     `json:"status"` // FINISHED, DNF, DNS, DSQ, OTL
	ChipTimeMs      *int64     `json:"chipTimeMs,omitempty"`
	GunTimeMs       *int64     `json:"gunTimeMs,omitempty"`
	OverallRank     *int       `json:"overallRank,omitempty"`
	GenderRank      *int       `json:"genderRank,omitempty"`
	CategoryRank    *int       `json:"categoryRank,omitempty"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
}

// ChipMappingRecord represents an association between a BIB and a transponder chip code.
type ChipMappingRecord struct {
	BibNumber string `json:"bibNumber"`
	ChipCode  string `json:"chipCode"`
	Notes     string `json:"notes,omitempty"`
}

// ParticipantSyncRecord represents participant data formatted for external timing software.
type ParticipantSyncRecord struct {
	BibNumber       string     `json:"bibNumber"`
	ParticipantName string     `json:"participantName"`
	Gender          string     `json:"gender"`
	CategoryName    string     `json:"categoryName"`
	TransponderCode string     `json:"transponderCode,omitempty"`
	DateOfBirth     *time.Time `json:"dateOfBirth,omitempty"`
}

// Capability Interfaces (Small & Idiomatic Go)

// PassingParser decodes raw antenna passing stream bytes into normalized passings.
type PassingParser interface {
	ProviderName() string
	ParsePassings(ctx context.Context, defaultCheckpoint string, payload []byte) ([]TimingPassing, error)
}

// ParticipantExporter formats participants into vendor-specific registration exchange format.
type ParticipantExporter interface {
	ProviderName() string
	FormatParticipantExport(ctx context.Context, participants []ParticipantSyncRecord) ([]byte, error)
}

// FinalResultParser decodes vendor payload directly into official normalized race results.
type FinalResultParser interface {
	ProviderName() string
	ParseFinalResults(ctx context.Context, payload []byte) ([]NormalizedResult, error)
}

// MappingParser decodes bulk BIB <-> transponder chip mapping files.
type MappingParser interface {
	ProviderName() string
	ParseChipMappings(ctx context.Context, payload []byte) ([]ChipMappingRecord, error)
}

// TimingProvider is the base adapter contract kept for backward compatibility.
type TimingProvider interface {
	Type() ProviderType
	ParsePassingStream(ctx context.Context, defaultCheckpoint string, payload []byte) ([]RawObservation, error)
	FormatParticipantExport(ctx context.Context, participants []ParticipantSyncRecord) ([]byte, error)
}

// WebAPICapable is an optional capability interface for two-way REST API sync.
type WebAPICapable interface {
	TimingProvider
	PushParticipants(ctx context.Context, apiURL, apiKey, externalRaceID string, participants []ParticipantSyncRecord) error
}
