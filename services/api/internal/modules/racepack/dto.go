package racepack

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// --- counters ---

// CounterRequest is the body for POST /counters.
// Accepts both camelCase (ticketId) and snake_case (ticket_id) JSON keys for
// backward compatibility with existing clients.
type CounterRequest struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Active   bool   `json:"active"`
}

// CounterUpdateRequest is the body for PUT /counters/{counterId}.
type CounterUpdateRequest struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Active   bool   `json:"active"`
}

// CounterResponse is the JSON view of a counter.
type CounterResponse struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	EventID        string    `json:"eventId"`
	Name           string    `json:"name"`
	Location       string    `json:"location,omitempty"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// --- slots ---

// SlotRequest is the body for POST /slots.
type SlotRequest struct {
	Name       string    `json:"name"`
	PickupDate string    `json:"pickupDate"` // YYYY-MM-DD
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime"`
	Capacity   int32     `json:"capacity"`
}

// SlotUpdateRequest is the body for PUT /slots/{slotId}.
type SlotUpdateRequest struct {
	Name       string    `json:"name"`
	PickupDate string    `json:"pickupDate"`
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime"`
	Capacity   int32     `json:"capacity"`
	Active     bool      `json:"active"`
}

// SlotResponse is the JSON view of a slot.
type SlotResponse struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	EventID        string    `json:"eventId"`
	Name           string    `json:"name"`
	PickupDate     string    `json:"pickupDate"`
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
	Capacity       int32     `json:"capacity"`
	ReservedCount  int32     `json:"reservedCount"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// --- pickups ---

// PickupRequest is the body for POST /pickups.
// Accepts BOTH camelCase and snake_case JSON keys for backward compatibility.
// SlotID is optional: when provided, pickup must occur within that slot.
type PickupRequest struct {
	TicketID  uuid.UUID  `json:"-"`
	CounterID uuid.UUID  `json:"-"`
	SlotID    *uuid.UUID `json:"-"`
	Method    string     `json:"-"`
	Notes     string     `json:"-"`

	// Camel-case aliases for snake_case compatibility.
	TicketIDCamel    uuid.UUID  `json:"ticketId"`
	CounterIDCamel   uuid.UUID  `json:"counterId"`
	SlotIDCamel      *uuid.UUID `json:"slotId,omitempty"`
	MethodCamel      string     `json:"method,omitempty"`
	NotesCamel       string     `json:"notes,omitempty"`

	// Snake-case aliases. Method and Notes are single words whose snake_case
	// form is identical to their camelCase form, so they need no snake alias.
	TicketIDSnake    uuid.UUID  `json:"ticket_id"`
	CounterIDSnake   uuid.UUID  `json:"counter_id"`
	SlotIDSnake      *uuid.UUID `json:"slot_id,omitempty"`
}

// UnmarshalJSON accepts both camelCase and snake_case keys; whichever is
// present wins. This exists so the existing snake_case frontend (Phase 14
// initial implementation) keeps working alongside the camelCase convention
// used by every other module.
func (r *PickupRequest) UnmarshalJSON(b []byte) error {
	type alias PickupRequest
	var raw alias
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*r = PickupRequest(raw)
	if r.TicketID == uuid.Nil {
		r.TicketID = r.TicketIDCamel
	}
	if r.TicketID == uuid.Nil {
		r.TicketID = r.TicketIDSnake
	}
	if r.CounterID == uuid.Nil {
		r.CounterID = r.CounterIDCamel
	}
	if r.CounterID == uuid.Nil {
		r.CounterID = r.CounterIDSnake
	}
	if r.SlotID == nil {
		r.SlotID = r.SlotIDCamel
	}
	if r.SlotID == nil {
		r.SlotID = r.SlotIDSnake
	}
	if r.Method == "" {
		r.Method = r.MethodCamel
	}
	if r.Notes == "" {
		r.Notes = r.NotesCamel
	}
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	return nil
}

// PickupResponse is the JSON view of a pickup record.
type PickupResponse struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organizationId"`
	EventID         string    `json:"eventId"`
	TicketID        string    `json:"ticketId"`
	ParticipantID   string    `json:"participantId"`
	BibNumber       string    `json:"bibNumber"`
	CounterID       string    `json:"counterId"`
	SlotID          string    `json:"slotId,omitempty"`
	StaffID         string    `json:"staffId"`
	PickupMethod    string    `json:"pickupMethod"`
	PickupTimestamp time.Time `json:"pickupTimestamp"`
	Notes           string    `json:"notes,omitempty"`
	Status          string    `json:"status"`
}

// --- proxy authorizations ---

// ProxyAuthorizationRequest is the body for POST /proxy-authorizations.
// Accepts BOTH camelCase and snake_case keys.
type ProxyAuthorizationRequest struct {
	TicketID              uuid.UUID  `json:"-"`
	PickupRecordID        *uuid.UUID `json:"-"`
	ProxyName             string     `json:"-"`
	ProxyPhone            string     `json:"-"`
	ProxyIdentity         string     `json:"-"`
	AuthorizationDocument string     `json:"-"`

	TicketIDCamel              uuid.UUID  `json:"ticketId"`
	PickupRecordIDCamel        *uuid.UUID `json:"pickupRecordId,omitempty"`
	ProxyNameCamel             string     `json:"proxyName"`
	ProxyPhoneCamel            string     `json:"proxyPhone,omitempty"`
	ProxyIdentityCamel         string     `json:"proxyIdentity"`
	AuthorizationDocumentCamel string     `json:"authorizationDocument,omitempty"`

	TicketIDSnake              uuid.UUID  `json:"ticket_id"`
	PickupRecordIDSnake        *uuid.UUID `json:"pickup_record_id,omitempty"`
	ProxyNameSnake             string     `json:"proxy_name"`
	ProxyPhoneSnake            string     `json:"proxy_phone,omitempty"`
	ProxyIdentitySnake         string     `json:"proxy_identity"`
	AuthorizationDocumentSnake string     `json:"authorization_document,omitempty"`
}

// UnmarshalJSON accepts both camelCase and snake_case keys.
func (r *ProxyAuthorizationRequest) UnmarshalJSON(b []byte) error {
	type alias ProxyAuthorizationRequest
	var raw alias
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*r = ProxyAuthorizationRequest(raw)
	if r.TicketID == uuid.Nil {
		r.TicketID = r.TicketIDCamel
	}
	if r.TicketID == uuid.Nil {
		r.TicketID = r.TicketIDSnake
	}
	if r.PickupRecordID == nil {
		r.PickupRecordID = r.PickupRecordIDCamel
	}
	if r.PickupRecordID == nil {
		r.PickupRecordID = r.PickupRecordIDSnake
	}
	if r.ProxyName == "" {
		r.ProxyName = r.ProxyNameCamel
	}
	if r.ProxyName == "" {
		r.ProxyName = r.ProxyNameSnake
	}
	if r.ProxyPhone == "" {
		r.ProxyPhone = r.ProxyPhoneCamel
	}
	if r.ProxyPhone == "" {
		r.ProxyPhone = r.ProxyPhoneSnake
	}
	if r.ProxyIdentity == "" {
		r.ProxyIdentity = r.ProxyIdentityCamel
	}
	if r.ProxyIdentity == "" {
		r.ProxyIdentity = r.ProxyIdentitySnake
	}
	if r.AuthorizationDocument == "" {
		r.AuthorizationDocument = r.AuthorizationDocumentCamel
	}
	if r.AuthorizationDocument == "" {
		r.AuthorizationDocument = r.AuthorizationDocumentSnake
	}
	return nil
}

// ProxyAuthorizationResponse is the JSON view of a proxy authorization.
type ProxyAuthorizationResponse struct {
	ID                    string    `json:"id"`
	OrganizationID        string    `json:"organizationId"`
	EventID               string    `json:"eventId"`
	TicketID              string    `json:"ticketId"`
	PickupRecordID        string    `json:"pickupRecordId,omitempty"`
	ProxyName             string    `json:"proxyName"`
	ProxyPhone            string    `json:"proxyPhone,omitempty"`
	ProxyIdentity         string    `json:"proxyIdentity"`
	AuthorizationDocument string    `json:"authorizationDocument,omitempty"`
	CreatedBy             string    `json:"createdBy"`
	CreatedAt             time.Time `json:"createdAt"`
}

// --- problem cases ---

// ProblemCaseRequest is the body for POST /problem-cases.
// Accepts BOTH camelCase and snake_case keys.
type ProblemCaseRequest struct {
	TicketID      *uuid.UUID `json:"-"`
	ParticipantID *uuid.UUID `json:"-"`

	TicketIDCamel      *uuid.UUID `json:"ticketId,omitempty"`
	ParticipantIDCamel *uuid.UUID `json:"participantId,omitempty"`

	TicketIDSnake      *uuid.UUID `json:"ticket_id,omitempty"`
	ParticipantIDSnake *uuid.UUID `json:"participant_id,omitempty"`

	Reason string `json:"reason"`
}

// UnmarshalJSON accepts both camelCase and snake_case keys.
func (r *ProblemCaseRequest) UnmarshalJSON(b []byte) error {
	type alias ProblemCaseRequest
	var raw alias
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*r = ProblemCaseRequest(raw)
	if r.TicketID == nil {
		r.TicketID = r.TicketIDCamel
	}
	if r.TicketID == nil {
		r.TicketID = r.TicketIDSnake
	}
	if r.ParticipantID == nil {
		r.ParticipantID = r.ParticipantIDCamel
	}
	if r.ParticipantID == nil {
		r.ParticipantID = r.ParticipantIDSnake
	}
	return nil
}

// ProblemCaseUpdateRequest is the body for PUT /problem-cases/{caseId}.
type ProblemCaseUpdateRequest struct {
	Status     string `json:"status"`
	Resolution string `json:"resolution,omitempty"`
}

// ProblemCaseResponse is the JSON view of a problem case.
type ProblemCaseResponse struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	EventID        string     `json:"eventId"`
	TicketID       string     `json:"ticketId,omitempty"`
	ParticipantID  string     `json:"participantId,omitempty"`
	Status         string     `json:"status"`
	Reason         string     `json:"reason"`
	Resolution     string     `json:"resolution,omitempty"`
	CreatedBy      string     `json:"createdBy"`
	ResolvedBy     string     `json:"resolvedBy,omitempty"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// --- dashboard ---

// DashboardResponse is the JSON view of the pickup dashboard.
// Uses `byCounter` and `openCases` to match the frontend Dashboard interface
// (Fix 2 — unified contract).
type DashboardResponse struct {
	TotalPickups  int64                   `json:"totalPickups"`
	ByCounter     []DashboardCounterCount `json:"byCounter"`
	OpenCases     int64                   `json:"openCases"`
	TotalCounters int                     `json:"totalCounters"`
	ActiveCounters int                    `json:"activeCounters"`
}

// DashboardCounterCount is the per-counter breakdown row.
// Uses snake_case JSON keys to match the frontend's expectation.
type DashboardCounterCount struct {
	CounterID   uuid.UUID `json:"counter_id"`
	CounterName string    `json:"counter_name,omitempty"`
	Pickups     int64     `json:"count"`
	Active      bool      `json:"active"`
}