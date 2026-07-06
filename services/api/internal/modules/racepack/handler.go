package racepack

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// Handler is the HTTP entry point for the racepack module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// userID pulls the authenticated user from context. Returns (uuid.Nil, false)
// and writes an error if the request is unauthenticated.
func userID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

// parseOrgEvent reads orgID and eventID from the URL.
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

// readJSON decodes the request body. Caps at 1 MiB.
func readJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(v)
}

// readJSONBytes reads the raw body bytes (used when we need to compute an
// idempotency hash AND decode the JSON).
func readJSONBytes(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return io.ReadAll(r.Body)
}

// writeError maps racepack sentinel errors to HTTP status codes.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrTicketNotFound):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "TICKET_NOT_FOUND", "ticket not found"))
	case errors.Is(err, ErrCounterNotFound):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "COUNTER_NOT_FOUND", "counter not found"))
	case errors.Is(err, ErrSlotNotFound):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "SLOT_NOT_FOUND", "slot not found"))
	case errors.Is(err, ErrAlreadyPickedUp):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "ALREADY_PICKED_UP", "ticket already picked up"))
	case errors.Is(err, ErrOrderNotPaid):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "ORDER_NOT_PAID", "order not paid"))
	case errors.Is(err, ErrBibMissing):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "BIB_MISSING", "bib not assigned to ticket"))
	case errors.Is(err, ErrTicketCancelled):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "TICKET_CANCELLED", "ticket cancelled"))
	case errors.Is(err, ErrSlotFull):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "SLOT_FULL", "slot is full"))
	case errors.Is(err, ErrSlotInactive):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "SLOT_INACTIVE", "slot inactive"))
	case errors.Is(err, ErrCounterInactive):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "COUNTER_INACTIVE", "counter inactive"))
	case errors.Is(err, ErrOutsideWindow):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "OUTSIDE_WINDOW", "outside pickup window"))
	case errors.Is(err, ErrInvalidStateChange):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "INVALID_STATE_CHANGE", "invalid status transition"))
	case errors.Is(err, ErrInvalidMethod):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_METHOD", "method must be SELF or PROXY"))
	case errors.Is(err, ErrTicketEventMismatch), errors.Is(err, ErrCounterEventMismatch):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "NOT_FOUND", "not found"))
	case errors.Is(err, ErrNoProblemTarget):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "NO_PROBLEM_TARGET", "at least one of ticket_id or participant_id required"))
	case errors.Is(err, ErrIdempotencyConflict):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency-Key reused with different payload"))
	case errors.Is(err, pgx.ErrNoRows):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "NOT_FOUND", "not found"))
	default:
		apperr.WriteError(w, r, err)
	}
}

// --- counters ---

func (h *Handler) CreateCounter(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	var body CounterRequest
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	counter, err := h.svc.CreateCounter(r.Context(), orgID, eventID, body.Name, body.Location, body.Active)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, toCounterResponse(counter))
}

func (h *Handler) ListCounters(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	counters, err := h.svc.ListCounters(r.Context(), eventID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]CounterResponse, 0, len(counters))
	for _, c := range counters {
		out = append(out, toCounterResponse(c))
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) UpdateCounter(w http.ResponseWriter, r *http.Request) {
	_, _, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	counterID, err := uuid.Parse(chi.URLParam(r, "counterId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_COUNTER_ID", "invalid counter id"))
		return
	}
	var body CounterUpdateRequest
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	counter, err := h.svc.UpdateCounter(r.Context(), counterID, body.Name, body.Location, body.Active)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toCounterResponse(counter))
}

func (h *Handler) SetCounterActive(w http.ResponseWriter, r *http.Request) {
	_, _, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	counterID, err := uuid.Parse(chi.URLParam(r, "counterId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_COUNTER_ID", "invalid counter id"))
		return
	}
	var body struct {
		Active bool `json:"active"`
	}
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	counter, err := h.svc.SetCounterActive(r.Context(), counterID, body.Active)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toCounterResponse(counter))
}

// --- slots ---

func (h *Handler) CreateSlot(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	var body SlotRequest
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	date, err := time.Parse("2006-01-02", body.PickupDate)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_PICKUP_DATE", "pickupDate must be YYYY-MM-DD"))
		return
	}
	slot, err := h.svc.CreateSlot(r.Context(), orgID, eventID, body.Name, date, body.StartTime, body.EndTime, body.Capacity)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, toSlotResponse(slot))
}

func (h *Handler) ListSlots(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	slots, err := h.svc.ListSlots(r.Context(), eventID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]SlotResponse, 0, len(slots))
	for _, s := range slots {
		out = append(out, toSlotResponse(s))
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) UpdateSlot(w http.ResponseWriter, r *http.Request) {
	_, _, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	slotID, err := uuid.Parse(chi.URLParam(r, "slotId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_SLOT_ID", "invalid slot id"))
		return
	}
	var body SlotUpdateRequest
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	date, err := time.Parse("2006-01-02", body.PickupDate)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_PICKUP_DATE", "pickupDate must be YYYY-MM-DD"))
		return
	}
	slot, err := h.svc.UpdateSlot(r.Context(), slotID, body.Name, date, body.StartTime, body.EndTime, body.Capacity, body.Active)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toSlotResponse(slot))
}

// --- pickups ---

func (h *Handler) CreatePickup(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	staffID, ok := userID(w, r)
	if !ok {
		return
	}
	bodyBytes, err := readJSONBytes(r)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	var body PickupRequest
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}

	// Idempotency-Key support. Same key + same payload → return cached response.
	// Same key + different payload → 409 IDEMPOTENCY_CONFLICT.
	const scope = "racepack.execute_pickup"
	idempKey := r.Header.Get("Idempotency-Key")
	if idempKey != "" {
		hit, err := h.svc.LookupIdempotency(r.Context(), idempKey, scope)
		if err != nil {
			apperr.WriteError(w, r, err)
			return
		}
		if hit != nil && hit.Found {
			reqHash := HashRequest(r.Method, r.URL.Path, bodyBytes)
			if hit.RequestHash != reqHash {
				writeError(w, r, ErrIdempotencyConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(int(hit.Status))
			_, _ = w.Write(hit.ResponseBody)
			return
		}
	}

	rec, err := h.svc.ExecutePickup(r.Context(), ExecutePickupInput{
		OrgID:     orgID,
		EventID:   eventID,
		TicketID:  body.TicketID,
		CounterID: body.CounterID,
		SlotID:    body.SlotID,
		StaffID:   staffID,
		Method:    body.Method,
		Notes:     body.Notes,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	resp := toPickupResponse(rec)

	// Cache the response if Idempotency-Key was supplied.
	if idempKey != "" {
		respBytes, _ := json.Marshal(resp)
		reqHash := HashRequest(r.Method, r.URL.Path, bodyBytes)
		h.svc.StoreIdempotency(r.Context(), idempKey, scope, reqHash, http.StatusCreated, respBytes)
	}

	apperr.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) GetPickupStatus(w http.ResponseWriter, r *http.Request) {
	_, _, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	ticketIDStr := r.URL.Query().Get("ticket_id")
	ticketID, err := uuid.Parse(ticketIDStr)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TICKET_ID", "invalid ticket_id"))
		return
	}
	rec, err := h.svc.GetPickupStatusByTicket(r.Context(), ticketID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toPickupResponse(rec))
}

func (h *Handler) ListPickups(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	limit, offset := paginationFromQuery(r)
	rows, err := h.svc.ListPickupRecordsByEvent(r.Context(), eventID, limit, offset)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]PickupResponse, 0, len(rows))
	for _, p := range rows {
		out = append(out, toPickupResponse(p))
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// --- proxy authorizations ---

func (h *Handler) CreateProxyAuthorization(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	actorID, ok := userID(w, r)
	if !ok {
		return
	}
	var body ProxyAuthorizationRequest
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	rec, err := h.svc.CreateProxyAuthorization(r.Context(), orgID, eventID, body.TicketID, actorID, body.PickupRecordID, body.ProxyName, body.ProxyPhone, body.ProxyIdentity, body.AuthorizationDocument)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, toProxyAuthResponse(rec))
}

func (h *Handler) ListProxyAuthorizations(w http.ResponseWriter, r *http.Request) {
	_, _, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	ticketIDStr := r.URL.Query().Get("ticket_id")
	ticketID, err := uuid.Parse(ticketIDStr)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TICKET_ID", "invalid ticket_id"))
		return
	}
	rows, err := h.svc.ListProxyAuthorizationsByTicket(r.Context(), ticketID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]ProxyAuthorizationResponse, 0, len(rows))
	for _, p := range rows {
		out = append(out, toProxyAuthResponse(p))
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// --- problem cases ---

func (h *Handler) CreateProblemCase(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	actorID, ok := userID(w, r)
	if !ok {
		return
	}
	var body ProblemCaseRequest
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	rec, err := h.svc.CreateProblemCase(r.Context(), orgID, eventID, actorID, body.TicketID, body.ParticipantID, body.Reason)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, toProblemCaseResponse(rec))
}

func (h *Handler) UpdateProblemCase(w http.ResponseWriter, r *http.Request) {
	_, _, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CASE_ID", "invalid case id"))
		return
	}
	actorID, ok := userID(w, r)
	if !ok {
		return
	}
	var body ProblemCaseUpdateRequest
	if err := readJSON(r, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	rec, err := h.svc.UpdateProblemCaseStatus(r.Context(), caseID, actorID, body.Status, body.Resolution)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toProblemCaseResponse(rec))
}

func (h *Handler) ListProblemCases(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	limit, offset := paginationFromQuery(r)
	rows, err := h.svc.ListProblemCases(r.Context(), eventID, limit, offset)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]ProblemCaseResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, toProblemCaseResponse(c))
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// --- dashboard ---

func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	summary, err := h.svc.DashboardSummary(r.Context(), eventID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, summary)
}

// --- participant endpoints ---

// ListActiveSlotsForParticipant returns the active slots for an event so a
// participant can pick one to reserve.
func (h *Handler) ListActiveSlotsForParticipant(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	slots, err := h.svc.ListActiveSlots(r.Context(), eventID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]SlotResponse, 0, len(slots))
	for _, s := range slots {
		out = append(out, toSlotResponse(s))
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// ReserveSlotForParticipant reserves capacity in a slot. Returns 409 SLOT_FULL
// when capacity is exhausted.
func (h *Handler) ReserveSlotForParticipant(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	slotID, err := uuid.Parse(chi.URLParam(r, "slotId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_SLOT_ID", "invalid slot id"))
		return
	}
	slot, err := h.svc.ReserveSlot(r.Context(), slotID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if slot.EventID != eventID {
		writeError(w, r, ErrSlotNotFound)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toSlotResponse(slot))
}

// --- helpers ---

func paginationFromQuery(r *http.Request) (int32, int32) {
	limit := int32(50)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	return limit, offset
}

func toCounterResponse(c db.RacepackCounter) CounterResponse {
	resp := CounterResponse{
		ID:             c.ID.String(),
		OrganizationID: c.OrganizationID.String(),
		EventID:        c.EventID.String(),
		Name:           c.Name,
		Active:         c.Active,
		CreatedAt:      c.CreatedAt.Time,
		UpdatedAt:      c.UpdatedAt.Time,
	}
	if c.Location.Valid {
		resp.Location = c.Location.String
	}
	return resp
}

func toSlotResponse(s db.RacepackPickupSlot) SlotResponse {
	resp := SlotResponse{
		ID:             s.ID.String(),
		OrganizationID: s.OrganizationID.String(),
		EventID:        s.EventID.String(),
		Name:           s.Name,
		Capacity:       s.Capacity,
		ReservedCount:  s.ReservedCount,
		Active:         s.Active,
		CreatedAt:      s.CreatedAt.Time,
		UpdatedAt:      s.UpdatedAt.Time,
	}
	if s.PickupDate.Valid {
		resp.PickupDate = s.PickupDate.Time.Format("2006-01-02")
	}
	if s.StartTime.Valid {
		resp.StartTime = s.StartTime.Time
	}
	if s.EndTime.Valid {
		resp.EndTime = s.EndTime.Time
	}
	return resp
}

func toPickupResponse(p db.RacepackPickupRecord) PickupResponse {
	resp := PickupResponse{
		ID:              p.ID.String(),
		OrganizationID:  p.OrganizationID.String(),
		EventID:         p.EventID.String(),
		TicketID:        p.TicketID.String(),
		ParticipantID:   p.ParticipantID.String(),
		BibNumber:       p.BibNumber,
		CounterID:       p.CounterID.String(),
		StaffID:         p.StaffID.String(),
		PickupMethod:    p.PickupMethod,
		PickupTimestamp: p.PickupTimestamp.Time,
		Status:          p.Status,
	}
	if p.Notes.Valid {
		resp.Notes = p.Notes.String
	}
	return resp
}

func toProxyAuthResponse(p db.RacepackProxyAuthorization) ProxyAuthorizationResponse {
	resp := ProxyAuthorizationResponse{
		ID:            p.ID.String(),
		OrganizationID: p.OrganizationID.String(),
		EventID:       p.EventID.String(),
		TicketID:      p.TicketID.String(),
		ProxyName:     p.ProxyName,
		ProxyIdentity: p.ProxyIdentity,
		CreatedBy:     p.CreatedBy.String(),
		CreatedAt:     p.CreatedAt.Time,
	}
	if p.PickupRecordID != nil {
		resp.PickupRecordID = p.PickupRecordID.String()
	}
	if p.ProxyPhone.Valid {
		resp.ProxyPhone = p.ProxyPhone.String
	}
	if p.AuthorizationDocument.Valid {
		resp.AuthorizationDocument = p.AuthorizationDocument.String
	}
	return resp
}

func toProblemCaseResponse(c db.RacepackProblemCase) ProblemCaseResponse {
	resp := ProblemCaseResponse{
		ID:             c.ID.String(),
		OrganizationID: c.OrganizationID.String(),
		EventID:        c.EventID.String(),
		Status:         c.Status,
		Reason:         c.Reason,
		CreatedBy:      c.CreatedBy.String(),
		CreatedAt:      c.CreatedAt.Time,
		UpdatedAt:      c.UpdatedAt.Time,
	}
	if c.TicketID != nil {
		resp.TicketID = c.TicketID.String()
	}
	if c.ParticipantID != nil {
		resp.ParticipantID = c.ParticipantID.String()
	}
	if c.Resolution.Valid {
		resp.Resolution = c.Resolution.String
	}
	if c.ResolvedBy != nil {
		resp.ResolvedBy = c.ResolvedBy.String()
	}
	if c.ResolvedAt.Valid {
		t := c.ResolvedAt.Time
		resp.ResolvedAt = &t
	}
	return resp
}
