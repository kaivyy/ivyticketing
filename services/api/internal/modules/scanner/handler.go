package scanner

import (
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

// Handler is the HTTP entry point for the scanner module.
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

// readJSONBytes reads the raw body bytes (used when we need to compute an
// idempotency hash AND decode the JSON). Caps at 1 MiB.
func readJSONBytes(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return io.ReadAll(r.Body)
}

// writeError maps scanner sentinel errors to HTTP status codes and stable
// error codes per the design "Backend error taxonomy" table. Every mapped
// error becomes an *apperr.APIError so the platform envelope masks internal
// detail; the default branch delegates to apperr.WriteError, which renders an
// *apperr.APIError verbatim and masks any other (raw internal) error as a
// generic 500 — so raw internal errors are never leaked to the client.
//
// | Sentinel               | HTTP | Code                   |
// |------------------------|------|------------------------|
// | ErrSignatureInvalid    | 422  | QR_SIGNATURE_INVALID   |
// | ErrMalformedToken      | 400  | QR_MALFORMED           |
// | ErrUnsupportedVersion  | 422  | QR_UNSUPPORTED_VERSION |
// | ErrEventMismatch       | 404  | EVENT_MISMATCH         |
// | ErrTicketNotFound      | 404  | TICKET_NOT_FOUND       |
// | ErrTicketCancelled     | 409  | TICKET_CANCELLED       |
// | ErrAlreadyCheckedIn    | 409  | ALREADY_CHECKED_IN     |
// | ErrUnauthorizedEvent   | 403  | FORBIDDEN_EVENT        |
// | ErrIdempotencyConflict | 409  | IDEMPOTENCY_CONFLICT   |
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrSignatureInvalid):
		apperr.WriteError(w, r, apperr.New(http.StatusUnprocessableEntity, "QR_SIGNATURE_INVALID", "qr signature invalid"))
	case errors.Is(err, ErrMalformedToken):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "QR_MALFORMED", "qr token malformed"))
	case errors.Is(err, ErrUnsupportedVersion):
		apperr.WriteError(w, r, apperr.New(http.StatusUnprocessableEntity, "QR_UNSUPPORTED_VERSION", "qr token version unsupported"))
	case errors.Is(err, ErrEventMismatch):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "EVENT_MISMATCH", "ticket does not belong to this event"))
	case errors.Is(err, ErrTicketNotFound):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "TICKET_NOT_FOUND", "ticket not found"))
	case errors.Is(err, ErrTicketCancelled):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "TICKET_CANCELLED", "ticket cancelled"))
	case errors.Is(err, ErrAlreadyCheckedIn):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "ALREADY_CHECKED_IN", "ticket already checked in"))
	case errors.Is(err, ErrUnauthorizedEvent):
		apperr.WriteError(w, r, apperr.New(http.StatusForbidden, "FORBIDDEN_EVENT", "not authorized for this event"))
	case errors.Is(err, ErrIdempotencyConflict):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency-Key reused with different payload"))
	case errors.Is(err, pgx.ErrNoRows):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "NOT_FOUND", "not found"))
	default:
		apperr.WriteError(w, r, err)
	}
}

// Verify handles POST /scan/verify.
//
// TODO(task 3.3/7.2): wire full verification response.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	staffID, ok := userID(w, r)
	if !ok {
		return
	}
	var body VerifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	res, err := h.svc.Verify(r.Context(), VerifyInput{
		Token:   body.QRToken,
		EventID: eventID,
		OrgID:   orgID,
		StaffID: staffID,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, res)
}

// ListEvents handles GET /scan/events.
//
// It returns the authenticated staff member's Permitted_Events — the events
// (across all their organizations) for which they hold racepack.execute or
// checkin.execute (Req 1.2). This endpoint is authenticated but not org-scoped:
// the permitted set spans every org the staff belongs to, so it is mounted at
// the top level rather than under /organizations/{orgId}/events/{eventId}
// (route mounting is task 7.2).
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	staffID, ok := userID(w, r)
	if !ok {
		return
	}
	events, err := h.svc.ListPermittedEvents(r.Context(), staffID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ListPermittedEventsResult{Events: events})
}

// CheckIn handles POST /scan/check-in.
//
// It wraps the VALID->USED transition with the platform idempotency mechanism
// (shared idempotency_keys table), mirroring racepack's POST /pickups. Same
// Idempotency-Key + same payload replays the cached response body; same key +
// different payload returns 409 IDEMPOTENCY_CONFLICT.
func (h *Handler) CheckIn(w http.ResponseWriter, r *http.Request) {
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
	var body CheckInRequest
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	ticketID, err := uuid.Parse(body.TicketID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_TICKET_ID", "invalid ticket id"))
		return
	}

	// Idempotency-Key support. Same key + same payload → return cached response.
	// Same key + different payload → 409 IDEMPOTENCY_CONFLICT.
	const scope = IdempotencyScopeCheckin
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

	res, err := h.svc.CheckIn(r.Context(), CheckInInput{
		OrgID:     orgID,
		EventID:   eventID,
		TicketID:  ticketID,
		StaffID:   staffID,
		ScannedAt: body.ScannedAt,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}

	// Marshal the fresh response ONCE and write those exact bytes. A replay
	// (above) writes the stored ResponseBody verbatim, so caching the very same
	// bytes we emit here makes fresh and replayed responses byte-identical
	// (Req 8.3 / Property 11). Previously the fresh path used apperr.WriteJSON
	// (json.Encoder, which appends a trailing "\n") while the cache stored
	// json.Marshal bytes (no newline), so the two differed by one byte.
	respBytes, err := json.Marshal(res)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}

	// Cache the response if Idempotency-Key was supplied — store EXACTLY the
	// bytes written below.
	if idempKey != "" {
		reqHash := HashRequest(r.Method, r.URL.Path, bodyBytes)
		h.svc.StoreIdempotency(r.Context(), idempKey, scope, reqHash, http.StatusOK, respBytes)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}
