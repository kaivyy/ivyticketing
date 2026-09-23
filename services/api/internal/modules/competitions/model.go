package competitions

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RankingStrategy defines the algorithm used to rank results or standings.
type RankingStrategy string

const (
	StrategyTimeAsc        RankingStrategy = "TIME_ASC"
	StrategyTimeDesc       RankingStrategy = "TIME_DESC"
	StrategyScoreAsc       RankingStrategy = "SCORE_ASC"
	StrategyScoreDesc      RankingStrategy = "SCORE_DESC"
	StrategyPointsDesc     RankingStrategy = "POINTS_DESC"
	StrategyWinCount       RankingStrategy = "WIN_COUNT"
	StrategyGoalDifference RankingStrategy = "GOAL_DIFFERENCE"
	StrategyHeadToHead     RankingStrategy = "HEAD_TO_HEAD"
)

// TimePrecision defines the temporal granularity used for observations and official times.
type TimePrecision string

const (
	PrecisionMilliseconds TimePrecision = "ms"
	PrecisionCentiseconds TimePrecision = "cs"
	PrecisionWholeSeconds TimePrecision = "s"
	PrecisionCeilingSec   TimePrecision = "ceil_s" // World Athletics TR 19.24.5 road race ceiling
)

// ObservationType defines canonical in-competition event types.
type ObservationType string

const (
	ObsStart          ObservationType = "START"
	ObsFinish         ObservationType = "FINISH"
	ObsSplit          ObservationType = "SPLIT"
	ObsCheckpoint     ObservationType = "CHECKPOINT"
	ObsGoal           ObservationType = "GOAL"
	ObsPoint          ObservationType = "POINT"
	ObsCardYellow     ObservationType = "CARD_YELLOW"
	ObsCardRed        ObservationType = "CARD_RED"
	ObsCardIndirect   ObservationType = "CARD_INDIRECT_RED"
	ObsPenalty        ObservationType = "PENALTY"
	ObsFoul           ObservationType = "FOUL"
	ObsSubstitution   ObservationType = "SUBSTITUTION"
	ObsSetScore       ObservationType = "SET_SCORE"
	ObsLap            ObservationType = "LAP"
	ObsAnomaly        ObservationType = "ANOMALY"
	ObsStatusChange   ObservationType = "STATUS_CHANGE"
	ObsAdjudication   ObservationType = "ADJUDICATION"
)

// CompetitionStatus defines the canonical competition and result states.
type CompetitionStatus string

const (
	StatusRegistered    CompetitionStatus = "REGISTERED"
	StatusCheckedIn     CompetitionStatus = "CHECKED_IN"
	StatusStarted       CompetitionStatus = "STARTED"
	StatusActive        CompetitionStatus = "ACTIVE"
	StatusCompleted     CompetitionStatus = "COMPLETED"
	StatusFinished      CompetitionStatus = "FINISHED"
	StatusDNF           CompetitionStatus = "DNF"
	StatusDNS           CompetitionStatus = "DNS"
	StatusDSQ           CompetitionStatus = "DSQ"
	StatusOTL           CompetitionStatus = "OTL"
	StatusWithdrawn     CompetitionStatus = "WITHDRAWN"
	StatusRetired       CompetitionStatus = "RETIRED"
	StatusNoShow        CompetitionStatus = "NO_SHOW"
	StatusEliminated    CompetitionStatus = "ELIMINATED"
	StatusQualified     CompetitionStatus = "QUALIFIED"
	StatusDisqualified  CompetitionStatus = "DISQUALIFIED"
	StatusForfeit       CompetitionStatus = "FORFEIT"
	StatusWalkover      CompetitionStatus = "WALKOVER"
	StatusPendingReview CompetitionStatus = "PENDING_REVIEW"
)

// IsOfficialValidFinisher checks whether the status represents an official completion.
func (s CompetitionStatus) IsOfficialValidFinisher() bool {
	return s == StatusFinished || s == StatusCompleted || s == StatusQualified
}

// StageType defines tournament or competition progression models.
type StageType string

const (
	StageSingleRace        StageType = "single_race"
	StageGroupRoundRobin   StageType = "group_round_robin"
	StageSingleElimination StageType = "single_elimination"
	StageDoubleElimination StageType = "double_elimination"
	StageHeats             StageType = "heats"
	StagePelotonStage      StageType = "peloton_stage"
)

// EntryType defines whether the participant is individual or group-based.
type EntryType string

const (
	EntryIndividual EntryType = "INDIVIDUAL"
	EntryTeam       EntryType = "TEAM"
	EntryPair       EntryType = "PAIR"
	EntryRelay      EntryType = "RELAY"
	EntrySquad      EntryType = "SQUAD"
)

// SportView is the API shape for a sport catalog entry.
type SportView struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	GoverningBody string    `json:"governingBody"`
	CreatedAt     time.Time `json:"createdAt"`
}

// SportDisciplineView is the API shape for a sport discipline.
type SportDisciplineView struct {
	ID            string         `json:"id"`
	SportID       string         `json:"sportId"`
	Name          string         `json:"name"`
	DefaultConfig map[string]any `json:"defaultConfig"`
	CreatedAt     time.Time      `json:"createdAt"`
}

// CompetitionConfigView describes the capability flags and rules for an event/category.
type CompetitionConfigView struct {
	ID              uuid.UUID       `json:"id"`
	OrganizationID  uuid.UUID       `json:"organizationId"`
	EventID         uuid.UUID       `json:"eventId"`
	CategoryID      *uuid.UUID      `json:"categoryId,omitempty"`
	SportID         string          `json:"sportId"`
	DisciplineID    string          `json:"disciplineId"`
	HasTiming       bool            `json:"hasTiming"`
	HasScoring      bool            `json:"hasScoring"`
	HasMatches      bool            `json:"hasMatches"`
	HasHeatsLanes   bool            `json:"hasHeatsLanes"`
	HasStages       bool            `json:"hasStages"`
	TeamBased       bool            `json:"teamBased"`
	MinTeamSize     int             `json:"minTeamSize"`
	MaxTeamSize     int             `json:"maxTeamSize"`
	RankingStrategy RankingStrategy `json:"rankingStrategy"`
	TimePrecision   TimePrecision   `json:"timePrecision"`
	RulesConfig     map[string]any  `json:"rulesConfig"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// TeamView describes a club or squad competing in an event.
type TeamView struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organizationId"`
	EventID        uuid.UUID  `json:"eventId"`
	CategoryID     *uuid.UUID `json:"categoryId,omitempty"`
	Name           string     `json:"name"`
	ShortName      string     `json:"shortName,omitempty"`
	ManagerUserID  *uuid.UUID `json:"managerUserId,omitempty"`
	LogoURL        string     `json:"logoUrl,omitempty"`
	SeedNumber     *int       `json:"seedNumber,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// TeamRosterView represents an individual member within a team roster.
type TeamRosterView struct {
	ID           uuid.UUID  `json:"id"`
	TeamID       uuid.UUID  `json:"teamId"`
	TicketID     *uuid.UUID `json:"ticketId,omitempty"`
	UserID       *uuid.UUID `json:"userId,omitempty"`
	PlayerName   string     `json:"playerName"`
	JerseyNumber *int       `json:"jerseyNumber,omitempty"`
	Position     string     `json:"position,omitempty"`
	IsCaptain    bool       `json:"isCaptain"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// CompetitionStageView represents a tournament round, group, or race heat.
type CompetitionStageView struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organizationId"`
	EventID        uuid.UUID      `json:"eventId"`
	CategoryID     *uuid.UUID     `json:"categoryId,omitempty"`
	Name           string         `json:"name"`
	StageType      StageType      `json:"stageType"`
	SequenceOrder  int            `json:"sequenceOrder"`
	Status         string         `json:"status"`
	StageConfig    map[string]any `json:"stageConfig"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// MatchScoreDetail represents period or set scores (e.g. Badminton sets, Football halves).
type MatchScoreDetail struct {
	PeriodNumber int `json:"periodNumber"`
	HomeScore    int `json:"homeScore"`
	AwayScore    int `json:"awayScore"`
}

// CompetitionMatchView represents a match fixture between two sides.
type CompetitionMatchView struct {
	ID                 uuid.UUID          `json:"id"`
	OrganizationID     uuid.UUID          `json:"organizationId"`
	EventID            uuid.UUID          `json:"eventId"`
	StageID            uuid.UUID          `json:"stageId"`
	RoundNumber        int                `json:"roundNumber"`
	MatchNumber        int                `json:"matchNumber"`
	HomeTeamID         *uuid.UUID         `json:"homeTeamId,omitempty"`
	AwayTeamID         *uuid.UUID         `json:"awayTeamId,omitempty"`
	HomeEntryID        *uuid.UUID         `json:"homeEntryId,omitempty"`
	AwayEntryID        *uuid.UUID         `json:"awayEntryId,omitempty"`
	ScheduledStartTime *time.Time         `json:"scheduledStartTime,omitempty"`
	VenueCourtName     string             `json:"venueCourtName,omitempty"`
	HomeScore          int                `json:"homeScore"`
	AwayScore          int                `json:"awayScore"`
	MatchStatus        string             `json:"matchStatus"`
	WinnerID           *uuid.UUID         `json:"winnerId,omitempty"`
	ScoreDetails       []MatchScoreDetail `json:"scoreDetails,omitempty"`
	CreatedAt          time.Time          `json:"createdAt"`
	UpdatedAt          time.Time          `json:"updatedAt"`
}

// ParticipantEntryView is the unified identity for any entrant in a competition.
type ParticipantEntryView struct {
	ID             uuid.UUID         `json:"id"`
	OrganizationID uuid.UUID         `json:"organizationId"`
	EventID        uuid.UUID         `json:"eventId"`
	CategoryID     *uuid.UUID        `json:"categoryId,omitempty"`
	TicketID       *uuid.UUID        `json:"ticketId,omitempty"`
	TeamID         *uuid.UUID        `json:"teamId,omitempty"`
	EntryType      EntryType         `json:"entryType"`
	DisplayName    string            `json:"displayName"`
	IdentifierCode string            `json:"identifierCode,omitempty"`
	SeedNumber     *int              `json:"seedNumber,omitempty"`
	LaneNumber     *int              `json:"laneNumber,omitempty"`
	HeatNumber     *int              `json:"heatNumber,omitempty"`
	Status         CompetitionStatus `json:"status"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

// CompetitionObservationView is an individual raw or normalized in-competition event.
type CompetitionObservationView struct {
	ID              uuid.UUID       `json:"id"`
	OrganizationID  uuid.UUID       `json:"organizationId"`
	EventID         uuid.UUID       `json:"eventId"`
	StageID         *uuid.UUID      `json:"stageId,omitempty"`
	MatchID         *uuid.UUID      `json:"matchId,omitempty"`
	EntryID         *uuid.UUID      `json:"entryId,omitempty"`
	ObservationType ObservationType `json:"observationType"`
	Source          string          `json:"source"`
	NumericValue    *float64        `json:"numericValue,omitempty"`
	TextValue       string          `json:"textValue,omitempty"`
	ObservedAtMs    int64           `json:"observedAtMs"`
	Metadata        map[string]any  `json:"metadata,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
}

// CompetitionResultView represents a finalized or live result in a stage or event.
type CompetitionResultView struct {
	ID              uuid.UUID         `json:"id"`
	OrganizationID  uuid.UUID         `json:"organizationId"`
	EventID         uuid.UUID         `json:"eventId"`
	StageID         *uuid.UUID        `json:"stageId,omitempty"`
	CategoryID      *uuid.UUID        `json:"categoryId,omitempty"`
	EntryID         uuid.UUID         `json:"entryId"`
	DisplayName     string            `json:"displayName"`
	IdentifierCode  string            `json:"identifierCode,omitempty"`
	EntryType       EntryType         `json:"entryType"`
	TeamID          *uuid.UUID        `json:"teamId,omitempty"`
	RankOverall     *int              `json:"rankOverall,omitempty"`
	RankCategory    *int              `json:"rankCategory,omitempty"`
	Status          CompetitionStatus `json:"status"`
	PrimaryTimeMs   *int64            `json:"primaryTimeMs,omitempty"`
	SecondaryTimeMs *int64            `json:"secondaryTimeMs,omitempty"`
	PrimaryTime     string            `json:"primaryTime,omitempty"`
	SecondaryTime   string            `json:"secondaryTime,omitempty"`
	PointsScored    float64           `json:"pointsScored"`
	Metrics         map[string]any    `json:"metrics,omitempty"`
	Notes           string            `json:"notes,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

// GenericStandingsRow represents a unified league table or leaderboard row.
type GenericStandingsRow struct {
	Rank           int            `json:"rank"`
	EntryID        uuid.UUID      `json:"entryId"`
	DisplayName    string         `json:"displayName"`
	IdentifierCode string         `json:"identifierCode,omitempty"`
	Played         int            `json:"played"`
	Wins           int            `json:"wins"`
	Draws          int            `json:"draws"`
	Losses         int            `json:"losses"`
	GoalsFor       int            `json:"goalsFor"`
	GoalsAgainst   int            `json:"goalsAgainst"`
	GoalDifference int            `json:"goalDifference"`
	Points         int            `json:"points"`
	PrimaryTimeMs  *int64         `json:"primaryTimeMs,omitempty"`
	PrimaryTime    string         `json:"primaryTime,omitempty"`
	Status         string         `json:"status"`
	CustomMetrics  map[string]any `json:"customMetrics,omitempty"`
}

// Helpers for JSON marshaling
func toJSONBytes(v any) []byte {
	if v == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
