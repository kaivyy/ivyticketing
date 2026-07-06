package status

// ComponentResponse is the public JSON shape of a system component's status.
type ComponentResponse struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	SortOrder int32  `json:"sortOrder"`
	UpdatedAt string `json:"updatedAt"`
}

// IncidentUpdateResponse is one entry in an incident's timeline.
type IncidentUpdateResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

// IncidentResponse is the JSON shape of an incident with its update timeline.
type IncidentResponse struct {
	ID         string                   `json:"id"`
	Title      string                   `json:"title"`
	Impact     string                   `json:"impact"`
	Status     string                   `json:"status"`
	StartedAt  string                   `json:"startedAt"`
	ResolvedAt *string                  `json:"resolvedAt"`
	CreatedAt  string                   `json:"createdAt"`
	UpdatedAt  string                   `json:"updatedAt"`
	Updates    []IncidentUpdateResponse `json:"updates"`
}

// StatusPageResponse is the public status page payload: overall health, each
// component, and the currently active incidents.
type StatusPageResponse struct {
	Overall     string              `json:"overall"`
	Components   []ComponentResponse `json:"components"`
	Incidents    []IncidentResponse  `json:"incidents"`
	LastUpdated string              `json:"lastUpdated"`
}

// UpdateComponentRequest sets a single component's status (super-admin).
type UpdateComponentRequest struct {
	Status string `json:"status"`
}

// CreateIncidentRequest opens a new incident with an initial update (super-admin).
type CreateIncidentRequest struct {
	Title  string `json:"title"`
	Impact string `json:"impact"`
	Body   string `json:"body"`
}

// AddIncidentUpdateRequest appends an update to an incident and moves its status.
type AddIncidentUpdateRequest struct {
	Status string `json:"status"`
	Body   string `json:"body"`
}

// Component status values.
const (
	CompOperational = "OPERATIONAL"
	CompDegraded    = "DEGRADED"
	CompDown        = "DOWN"
)

// Incident impact values.
const (
	ImpactNone     = "NONE"
	ImpactMinor    = "MINOR"
	ImpactMajor    = "MAJOR"
	ImpactCritical = "CRITICAL"
)

// Incident status values.
const (
	IncInvestigating = "INVESTIGATING"
	IncIdentified    = "IDENTIFIED"
	IncMonitoring    = "MONITORING"
	IncResolved      = "RESOLVED"
)
