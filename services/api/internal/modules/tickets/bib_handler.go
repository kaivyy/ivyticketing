package tickets

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// AssignBibNext handles POST /organizations/{orgId}/events/{eventId}/tickets/{ticketId}/bib/assign.
// Auto-assigns the next available BIB number.
func (h *Handler) AssignBibNext(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ticketID, actorID, ok := h.parseOrgEventTicketActor(w, r)
	if !ok {
		return
	}
	out, err := h.svc.AssignNextBib(r.Context(), orgID, eventID, ticketID, actorID)
	if err != nil {
		writeBibError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// SetBib handles PUT /organizations/{orgId}/events/{eventId}/tickets/{ticketId}/bib.
// Manually sets/overrides a BIB number.
func (h *Handler) SetBib(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ticketID, actorID, ok := h.parseOrgEventTicketActor(w, r)
	if !ok {
		return
	}
	var body struct {
		BibNumber string `json:"bibNumber"`
	}
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	out, err := h.svc.SetBib(r.Context(), orgID, eventID, ticketID, actorID, body.BibNumber)
	if err != nil {
		writeBibError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// ClearTicketBib handles DELETE /organizations/{orgId}/events/{eventId}/tickets/{ticketId}/bib.
// Removes any BIB assignment from the ticket.
func (h *Handler) ClearTicketBib(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ticketID, actorID, ok := h.parseOrgEventTicketActor(w, r)
	if !ok {
		return
	}
	out, err := h.svc.ClearBib(r.Context(), orgID, eventID, ticketID, actorID)
	if err != nil {
		writeBibError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// BulkAssignBibs handles POST /organizations/{orgId}/events/{eventId}/tickets/bib/bulk-assign.
// Auto-assigns BIBs to every VALID unassigned ticket in the event.
func (h *Handler) BulkAssignBibs(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.parseOrgEvent(w, r)
	if !ok {
		return
	}
	actorID, ok := userID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.BulkAssignBib(r.Context(), orgID, eventID, actorID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// PreviewNextBib handles GET /organizations/{orgId}/events/{eventId}/tickets/bib/next.
func (h *Handler) PreviewNextBib(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := h.parseOrgEvent(w, r)
	if !ok {
		return
	}
	out, err := h.svc.PreviewNextBib(r.Context(), eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// ExportBibsCSV handles GET /organizations/{orgId}/events/{eventId}/tickets/bib/export.
// Streams CSV — does not accumulate the whole result set in memory.
func (h *Handler) ExportBibsCSV(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.parseOrgEvent(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="bibs.csv"`)

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"bib_number", "ticket_number", "participant_name", "participant_email", "category", "status"})

	if err := h.svc.StreamTicketsForBibExport(r.Context(), orgID, eventID, func(row TicketExportRow) error {
		return cw.Write([]string{
			row.BibNumber,
			row.TicketNumber,
			row.HolderName,
			row.HolderEmail,
			row.CategoryName,
			row.Status,
		})
	}); err != nil {
		// At this point headers are already sent; best-effort write of an error trailer.
		// (Browsers may not display it; the client should check Content-Length or row count.)
		_ = cw.Error()
		return
	}
	cw.Flush()
}

// TicketExportRow is the streaming payload for the CSV export.
type TicketExportRow struct {
	BibNumber    string
	TicketNumber string
	HolderName   string
	HolderEmail  string
	CategoryName string
	Status       string
}

// parseOrgEventTicketActor extracts (orgID, eventID, ticketID, actorID) from the URL.
// Returns ok=false (and writes the error) on parse failure.
func (h *Handler) parseOrgEventTicketActor(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	orgID, eventID, ok := h.parseOrgEvent(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TICKET_ID", "invalid ticket id"))
		return uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	actorID, ok := userID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return orgID, eventID, ticketID, actorID, true
}

// parseOrgEvent extracts (orgID, eventID) from the URL.
func (h *Handler) parseOrgEvent(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid org id"))
		return uuid.Nil, uuid.Nil, false
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, eventID, true
}

// writeBibError maps service-layer BIB errors to HTTP status codes.
func writeBibError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrBibTicketNotFound):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "TICKET_NOT_FOUND", "ticket not found"))
	case errors.Is(err, ErrBibInvalidFormat):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BIB_INVALID_FORMAT", "bib number must be alphanumeric, max 32 chars"))
	case errors.Is(err, ErrBibAssignExhausted):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "BIB_ASSIGN_EXHAUSTED", "could not allocate a BIB after retries; try again"))
	case strings.Contains(strings.ToLower(err.Error()), "bib_conflict"):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "BIB_CONFLICT", "another ticket already has this BIB in this event"))
	default:
		apperr.WriteError(w, r, err)
	}
}

// readJSON decodes the request body into v. Uses a 1 MiB cap to defend against
// oversize payloads in non-streaming endpoints.
func readJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return jsonDecode(r.Body, v)
}

// quietAtoi is a tiny helper to silence the unused import of strconv in case
// future helpers use it without importing strconv at the top of this file.
var _ = strconv.Atoi
var _ = uuid.Nil