package results

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// TicketOwnershipFunc resolves the event a ticket belongs to, but only when the
// ticket is owned by userID; otherwise it returns an error. It is injected from
// server.go (wrapping the tickets service) so the results module never imports
// tickets and the TICKET_QR_SECRET is never duplicated here.
type TicketOwnershipFunc func(ctx context.Context, userID, ticketID uuid.UUID) (uuid.UUID, error)

// Handler is the HTTP entry point for the results module.
type Handler struct {
	svc       *Service
	ownership TicketOwnershipFunc
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// SetTicketOwnership injects the ownership resolver used by participant
// self-service certificate/result routes.
func (h *Handler) SetTicketOwnership(fn TicketOwnershipFunc) { h.ownership = fn }

func userID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func parseOrgEvent(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
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

func readJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(v)
}

// writeError maps results sentinel errors to HTTP status codes.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrResultNotFound):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "RESULT_NOT_FOUND", "hasil tidak ditemukan"))
	case errors.Is(err, ErrTemplateNotFound):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "TEMPLATE_NOT_FOUND", "template sertifikat tidak ditemukan"))
	case errors.Is(err, ErrNoActiveTemplate):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "NO_ACTIVE_TEMPLATE", "belum ada template sertifikat aktif"))
	case errors.Is(err, ErrCertNotEligible):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "CERT_NOT_ELIGIBLE", "tidak ada hasil FINISHED untuk tiket ini"))
	case errors.Is(err, ErrEmptyCSV):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "EMPTY_CSV", "file CSV tidak berisi data"))
	case errors.Is(err, ErrMissingBibColumn):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "MISSING_BIB_COLUMN", "CSV harus punya kolom bib"))
	case errors.Is(err, ErrInvalidCSV):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CSV", "format CSV tidak valid"))
	case errors.Is(err, ErrInvalidPayload):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_PAYLOAD", "payload tidak valid"))
	case errors.Is(err, pgx.ErrNoRows):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "NOT_FOUND", "tidak ditemukan"))
	default:
		apperr.WriteError(w, r, err)
	}
}

// --- results ---

// ImportResults ingests a CSV upload (text/csv or multipart form field "file").
func (h *Handler) ImportResults(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	body, closer, err := csvBody(r)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_UPLOAD", "gagal membaca file CSV"))
		return
	}
	if closer != nil {
		defer closer.Close()
	}
	summary, err := h.svc.ImportCSV(r.Context(), orgID, eventID, uid, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, summary)
}

// csvBody returns the CSV reader from either a multipart "file" field or the
// raw request body, capped at 8 MiB.
func csvBody(r *http.Request) (io.Reader, io.Closer, error) {
	const maxCSV = 8 << 20
	ct := r.Header.Get("Content-Type")
	if len(ct) >= 19 && ct[:19] == "multipart/form-data" {
		if err := r.ParseMultipartForm(maxCSV); err != nil {
			return nil, nil, err
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			return nil, nil, err
		}
		return io.LimitReader(f, maxCSV), f, nil
	}
	return http.MaxBytesReader(nil, r.Body, maxCSV), r.Body, nil
}

// ListResults returns a paginated, filterable result table for an event.
func (h *Handler) ListResults(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	var categoryID *uuid.UUID
	if c := q.Get("categoryId"); c != "" {
		if id, err := uuid.Parse(c); err == nil {
			categoryID = &id
		}
	}
	gender := normalizeGender(q.Get("gender"))
	limit := parseInt32(q.Get("limit"), 100)
	offset := parseInt32(q.Get("offset"), 0)

	rows, total, err := h.svc.List(r.Context(), eventID, categoryID, gender, limit, offset)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, map[string]any{"results": rows, "total": total})
}

// Recompute re-runs the ranking passes for an event.
func (h *Handler) Recompute(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Recompute(r.Context(), orgID, eventID, uid); err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, map[string]any{"ranked": true})
}

// DeleteResults clears all results for an event.
func (h *Handler) DeleteResults(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteAll(r.Context(), orgID, eventID, uid); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- participant-facing ---

// GetResultByBib is a public leaderboard lookup by bib.
func (h *Handler) GetResultByBib(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	bib := chi.URLParam(r, "bib")
	view, err := h.svc.GetByBib(r.Context(), eventID, bib)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, view)
}

// GetCertificate returns the rendered certificate for a ticket.
func (h *Handler) GetCertificate(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TICKET_ID", "invalid ticket id"))
		return
	}
	cert, err := h.svc.GetCertificate(r.Context(), eventID, ticketID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, cert)
}

// --- participant self-service (authn-level, ticket ownership verified) ---

// resolveOwnedTicket parses the {ticketId} param and verifies the authenticated
// user owns it, returning the ticket's event ID. On any failure it writes the
// response and returns ok=false.
func (h *Handler) resolveOwnedTicket(w http.ResponseWriter, r *http.Request) (eventID, ticketID uuid.UUID, ok bool) {
	uid, ok := userID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TICKET_ID", "invalid ticket id"))
		return uuid.Nil, uuid.Nil, false
	}
	if h.ownership == nil {
		apperr.WriteError(w, r, apperr.New(http.StatusInternalServerError, "INTERNAL", "ownership resolver not configured"))
		return uuid.Nil, uuid.Nil, false
	}
	eventID, err = h.ownership(r.Context(), uid, ticketID)
	if err != nil {
		// A ticket the user does not own is reported as not found so ownership
		// is not leaked.
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "TICKET_NOT_FOUND", "tiket tidak ditemukan"))
		return uuid.Nil, uuid.Nil, false
	}
	return eventID, ticketID, true
}

// GetMyResult returns the finisher row linked to a ticket the caller owns.
func (h *Handler) GetMyResult(w http.ResponseWriter, r *http.Request) {
	_, ticketID, ok := h.resolveOwnedTicket(w, r)
	if !ok {
		return
	}
	view, err := h.svc.GetByTicket(r.Context(), ticketID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, view)
}

// GetMyCertificate renders the certificate for a ticket the caller owns.
func (h *Handler) GetMyCertificate(w http.ResponseWriter, r *http.Request) {
	eventID, ticketID, ok := h.resolveOwnedTicket(w, r)
	if !ok {
		return
	}
	cert, err := h.svc.GetCertificate(r.Context(), eventID, ticketID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, cert)
}

// --- certificate templates ---

func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	var body CreateTemplateRequest
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	view, err := h.svc.CreateTemplate(r.Context(), orgID, eventID, uid, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, view)
}

func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, _, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	templateID, err := uuid.Parse(chi.URLParam(r, "templateId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TEMPLATE_ID", "invalid template id"))
		return
	}
	var body CreateTemplateRequest
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	view, err := h.svc.UpdateTemplate(r.Context(), orgID, templateID, uid, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, view)
}

func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	views, err := h.svc.ListTemplates(r.Context(), eventID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, map[string]any{"templates": views})
}

func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, _, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	templateID, err := uuid.Parse(chi.URLParam(r, "templateId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TEMPLATE_ID", "invalid template id"))
		return
	}
	if err := h.svc.DeleteTemplate(r.Context(), orgID, templateID, uid); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
