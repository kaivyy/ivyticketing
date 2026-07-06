package scanner

import "time"

// DisplayInfo carries strictly non-sensitive participant fields shown to staff
// during a scan. Per Requirement 3.4 it MUST NOT contain payment card data,
// passwords, or full contact details.
type DisplayInfo struct {
	ParticipantName string `json:"participantName"`
	BibNumber       string `json:"bibNumber"`    // "" when unassigned
	CategoryName    string `json:"category"`
	TicketStatus    string `json:"ticketStatus"` // VALID | USED | CANCELLED
}

// VerifyResult is the response to POST /scan/verify. It bundles the display
// info with the duplicate flags and their original timestamps (Req 6.4).
type VerifyResult struct {
	TicketID         string      `json:"ticketId"`
	EventID          string      `json:"eventId"`
	Display          DisplayInfo `json:"display"`
	AlreadyPickedUp  bool        `json:"alreadyPickedUp"`
	PickedUpAt       *time.Time  `json:"pickedUpAt,omitempty"`
	AlreadyCheckedIn bool        `json:"alreadyCheckedIn"`
	CheckedInAt      *time.Time  `json:"checkedInAt,omitempty"`
}

// CheckInRequest is the body for POST /scan/check-in. ScannedAt carries the
// original offline scan time when the operation is being replayed; nil means
// the server uses now().
type CheckInRequest struct {
	TicketID  string     `json:"ticketId"`
	ScannedAt *time.Time `json:"scannedAt,omitempty"`
}

// CheckInResult is the response to POST /scan/check-in. Duplicate is true when
// the ticket was already USED and no transition occurred.
type CheckInResult struct {
	TicketID    string    `json:"ticketId"`
	Status      string    `json:"status"` // USED
	CheckedInAt time.Time `json:"checkedInAt"`
	Duplicate   bool      `json:"duplicate"`
}

// VerifyRequest is the body for POST /scan/verify.
type VerifyRequest struct {
	QRToken string `json:"qrToken"`
}

// PermittedEvent is a single entry in the GET /scan/events response. It carries
// only non-sensitive event identity fields (id, org id, name, status) so the
// staff can pick which event to scan. It deliberately omits venue, dates, and
// any other event detail beyond what the picker needs.
type PermittedEvent struct {
	EventID        string `json:"eventId"`
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	Status         string `json:"status"`
}

// ListPermittedEventsResult is the response to GET /scan/events.
type ListPermittedEventsResult struct {
	Events []PermittedEvent `json:"events"`
}
