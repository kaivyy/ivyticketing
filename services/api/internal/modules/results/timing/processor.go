package timing

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CheckpointDef holds checkpoint metadata needed for scoring.
type CheckpointDef struct {
	ID              uuid.UUID
	Code            string
	Name            string
	Type            string // START, SPLIT, FINISH
	OrderIndex      int
	DistanceMeters  *int
	Aliases         []string
	ProviderAliases map[string][]string
}

// CheckpointAnomalyAction defines how missing mandatory checkpoints are adjudicated.
type CheckpointAnomalyAction string

const (
	AnomalyActionWarning          CheckpointAnomalyAction = "WARNING"
	AnomalyActionPendingReview    CheckpointAnomalyAction = "PENDING_REVIEW"
	AnomalyActionDNF              CheckpointAnomalyAction = "DNF"
	AnomalyActionDSQ              CheckpointAnomalyAction = "DSQ"
	AnomalyActionManualAdjudicate CheckpointAnomalyAction = "MANUAL_ADJUDICATION"
)

// EventPolicy holds configurable race rules for scoring.
type EventPolicy struct {
	DebounceWindowSeconds   int                     `json:"debounce_window_seconds"`
	CutoffMinutes           int                     `json:"cutoff_minutes"`
	StartMode               string                  `json:"start_mode"` // CHIP_PREFERRED, GUN_ONLY, CHIP_MANDATORY
	AllowMissingStart       bool                    `json:"allow_missing_start"`
	MandatoryCheckpoints    []string                `json:"mandatory_checkpoints"`
	MissingCheckpointAction CheckpointAnomalyAction `json:"missing_checkpoint_action"`
}

// ParticipantDef holds participant profile info needed for result projection.
type ParticipantDef struct {
	TicketID        *uuid.UUID
	CategoryID      *uuid.UUID
	BibNumber       string
	ParticipantName string
	Gender          string
	Age             *int
	AgeGroup        string
	WaveID          *uuid.UUID
	WaveStartAt     *time.Time
}

// PassingItem represents an ingested passing observation.
type PassingItem struct {
	ID             int64
	CheckpointCode string
	ChipCode       string
	BibNumber      string
	ObservedAt     time.Time
}

// CalculatedSplit is an intermediate split ready for persistence.
type CalculatedSplit struct {
	CheckpointID   uuid.UUID
	PassingID      *int64
	SplitTimeMs    int64
	SplitPaceMsKm  *int64
	PassingTime    time.Time
	OrderIndex     int
}

// ParticipantResultOutput holds calculated scoring outputs for a participant.
type ParticipantResultOutput struct {
	TicketID        *uuid.UUID
	CategoryID      *uuid.UUID
	WaveID          *uuid.UUID
	BibNumber       string
	ParticipantName string
	Gender          string
	Age             *int
	AgeGroup        string
	Status             string // FINISHED, DNF, DNS, DSQ, OTL, PENDING_REVIEW
	ChipTimeMs         *int64
	GunTimeMs          *int64
	FinishedAt         *time.Time
	Splits             []CalculatedSplit
	ProcessedIDs       []int64
	Anomalies          []string
	MissingCheckpoints []string
}

// Processor coordinates deterministic, vendor-agnostic scoring.
type Processor struct {
	DebounceWindow time.Duration
	Policy         EventPolicy
}

// NewProcessor constructs a scoring processor with default debounce.
func NewProcessor(debounceWindow time.Duration) *Processor {
	if debounceWindow <= 0 {
		debounceWindow = 15 * time.Second
	}
	return &Processor{
		DebounceWindow: debounceWindow,
		Policy: EventPolicy{
			DebounceWindowSeconds: int(debounceWindow.Seconds()),
			StartMode:             "CHIP_PREFERRED",
			AllowMissingStart:     true,
		},
	}
}

// WithPolicy attaches dynamic event policies to the processor.
func (p *Processor) WithPolicy(policy EventPolicy) *Processor {
	p.Policy = policy
	if policy.DebounceWindowSeconds > 0 {
		p.DebounceWindow = time.Duration(policy.DebounceWindowSeconds) * time.Second
	}
	return p
}

// ProcessEventPassings evaluates a batch of passings for all participants in an event.
func (p *Processor) ProcessEventPassings(
	_ context.Context,
	checkpoints []CheckpointDef,
	participants map[string]ParticipantDef, // keyed by bib_number
	chipToBib map[string]string,             // keyed by chip_code -> bib_number
	passings []PassingItem,
) []ParticipantResultOutput {
	// 1. Build canonical alias mapping table
	canonicalByAlias := make(map[string]string)
	var startCP *CheckpointDef
	var finishCP *CheckpointDef
	splitsByCode := make(map[string]CheckpointDef)

	for i := range checkpoints {
		cp := checkpoints[i]
		canonicalCode := cp.Code
		canonicalByAlias[strings.ToUpper(cp.Code)] = canonicalCode

		for _, al := range cp.Aliases {
			alTrimmed := strings.ToUpper(strings.TrimSpace(al))
			if alTrimmed != "" {
				canonicalByAlias[alTrimmed] = canonicalCode
			}
		}

		for _, aliases := range cp.ProviderAliases {
			for _, al := range aliases {
				alTrimmed := strings.ToUpper(strings.TrimSpace(al))
				if alTrimmed != "" {
					canonicalByAlias[alTrimmed] = canonicalCode
				}
			}
		}

		switch cp.Type {
		case "START":
			if startCP == nil || cp.OrderIndex < startCP.OrderIndex {
				startCP = &cp
			}
		case "FINISH":
			if finishCP == nil || cp.OrderIndex > finishCP.OrderIndex {
				finishCP = &cp
			}
		default:
			splitsByCode[cp.Code] = cp
		}
	}

	// 2. Resolve BIB for every passing and group by BIB
	passingsByBib := make(map[string][]PassingItem)
	for _, pass := range passings {
		// Canonicalize checkpoint code if alias is present
		if canonical, ok := canonicalByAlias[strings.ToUpper(pass.CheckpointCode)]; ok {
			pass.CheckpointCode = canonical
		}

		bib := pass.BibNumber
		if bib == "" {
			if resolved, ok := chipToBib[pass.ChipCode]; ok {
				bib = resolved
			}
		}
		if bib == "" {
			continue // Unmapped transponder; skip scoring for this pass
		}
		passingsByBib[bib] = append(passingsByBib[bib], pass)
	}

	results := make([]ParticipantResultOutput, 0, len(participants))

	// 3. Process each participant with passings
	for bib, partPassings := range passingsByBib {
		part, ok := participants[bib]
		if !ok {
			// Participant not in event registry
			part = ParticipantDef{
				BibNumber:       bib,
				ParticipantName: "BIB " + bib,
			}
		}

		// Sort passings chronologically
		sort.Slice(partPassings, func(i, j int) bool {
			return partPassings[i].ObservedAt.Before(partPassings[j].ObservedAt)
		})

		// Debounce duplicate reads of same checkpoint within window
		filtered := make([]PassingItem, 0, len(partPassings))
		lastObsByCP := make(map[string]time.Time)
		processedIDs := make([]int64, 0, len(partPassings))

		for _, item := range partPassings {
			processedIDs = append(processedIDs, item.ID)
			if last, seen := lastObsByCP[item.CheckpointCode]; seen {
				if item.ObservedAt.Sub(last) < p.DebounceWindow {
					continue // duplicate read debounce
				}
			}
			lastObsByCP[item.CheckpointCode] = item.ObservedAt
			filtered = append(filtered, item)
		}

		// Find start and finish passing
		var startObs *PassingItem
		var finishObs *PassingItem
		splitObs := make(map[string]PassingItem)

		for i := range filtered {
			item := filtered[i]
			if startCP != nil && item.CheckpointCode == startCP.Code {
				if startObs == nil {
					startObs = &item
				}
			} else if finishCP != nil && item.CheckpointCode == finishCP.Code {
				finishObs = &item
			} else if _, isSplit := splitsByCode[item.CheckpointCode]; isSplit {
				splitObs[item.CheckpointCode] = item
			}
		}

		// Evaluate start timing according to policy
		var startTime time.Time
		hasStart := false

		if startObs != nil {
			startTime = startObs.ObservedAt
			hasStart = true
		} else if p.Policy.StartMode != "CHIP_MANDATORY" && part.WaveStartAt != nil && p.Policy.AllowMissingStart {
			startTime = *part.WaveStartAt
			hasStart = true
		}

		var gunTimeMs *int64
		var chipTimeMs *int64
		var finishedAt *time.Time
		status := "DNS"

		if hasStart {
			status = "DNF"
		}

		if finishObs != nil {
			finishedAt = &finishObs.ObservedAt
			if hasStart && finishObs.ObservedAt.After(startTime) {
				netDuration := finishObs.ObservedAt.Sub(startTime)
				netMs := netDuration.Milliseconds()
				chipTimeMs = &netMs
				status = "FINISHED"

				// Check cutoff
				if p.Policy.CutoffMinutes > 0 {
					cutoffMs := int64(p.Policy.CutoffMinutes) * 60 * 1000
					if netMs > cutoffMs {
						status = "OTL"
					}
				}
			}

			// Gun time based on wave start_at (NO silent fallback to net time)
			if part.WaveStartAt != nil && finishObs.ObservedAt.After(*part.WaveStartAt) {
				gunDuration := finishObs.ObservedAt.Sub(*part.WaveStartAt)
				gunMs := gunDuration.Milliseconds()
				gunTimeMs = &gunMs
			}
		}

		// Calculate intermediate splits
		calculatedSplits := make([]CalculatedSplit, 0, len(splitObs))
		if hasStart {
			for cpCode, item := range splitObs {
				cpDef, exists := splitsByCode[cpCode]
				if !exists {
					continue
				}
				if item.ObservedAt.After(startTime) {
					splitDuration := item.ObservedAt.Sub(startTime)
					splitMs := splitDuration.Milliseconds()

					var paceMsKm *int64
					if cpDef.DistanceMeters != nil && *cpDef.DistanceMeters > 0 {
						distKm := float64(*cpDef.DistanceMeters) / 1000.0
						pace := int64(float64(splitMs) / distKm)
						paceMsKm = &pace
					}

					pID := item.ID
					calculatedSplits = append(calculatedSplits, CalculatedSplit{
						CheckpointID:  cpDef.ID,
						PassingID:     &pID,
						SplitTimeMs:   splitMs,
						SplitPaceMsKm: paceMsKm,
						PassingTime:   item.ObservedAt,
						OrderIndex:    cpDef.OrderIndex,
					})
				}
			}
		}

		// Evaluate mandatory checkpoints
		var anomalies []string
		var missingCPs []string
		if len(p.Policy.MandatoryCheckpoints) > 0 {
			for _, mcp := range p.Policy.MandatoryCheckpoints {
				mcpCanonical := strings.ToUpper(strings.TrimSpace(mcp))
				if canonical, ok := canonicalByAlias[mcpCanonical]; ok {
					mcpCanonical = canonical
				}
				observed := false
				if startObs != nil && strings.EqualFold(startObs.CheckpointCode, mcpCanonical) {
					observed = true
				} else if finishObs != nil && strings.EqualFold(finishObs.CheckpointCode, mcpCanonical) {
					observed = true
				} else if _, ok := splitObs[mcpCanonical]; ok {
					observed = true
				}

				if !observed {
					anomalies = append(anomalies, "MISSING_MANDATORY_CHECKPOINT:"+mcp)
					missingCPs = append(missingCPs, mcp)
				}
			}

			if len(missingCPs) > 0 && status == "FINISHED" {
				action := p.Policy.MissingCheckpointAction
				if action == "" {
					action = AnomalyActionPendingReview
				}
				switch action {
				case AnomalyActionDSQ:
					status = "DSQ"
				case AnomalyActionDNF:
					status = "DNF"
				case AnomalyActionPendingReview, AnomalyActionManualAdjudicate:
					status = "PENDING_REVIEW"
				case AnomalyActionWarning:
					// Status remains FINISHED, anomaly recorded
				default:
					status = "PENDING_REVIEW"
				}
			}
		}

		results = append(results, ParticipantResultOutput{
			TicketID:           part.TicketID,
			CategoryID:         part.CategoryID,
			WaveID:             part.WaveID,
			BibNumber:          bib,
			ParticipantName:    part.ParticipantName,
			Gender:             part.Gender,
			Age:                part.Age,
			AgeGroup:           part.AgeGroup,
			Status:             status,
			ChipTimeMs:         chipTimeMs,
			GunTimeMs:          gunTimeMs,
			FinishedAt:         finishedAt,
			Splits:             calculatedSplits,
			ProcessedIDs:       processedIDs,
			Anomalies:          anomalies,
			MissingCheckpoints: missingCPs,
		})
	}

	return results
}
