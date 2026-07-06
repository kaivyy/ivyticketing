package reporting

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// Handler is the HTTP entry point for the reporting module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func parseOrg(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid org id"))
		return uuid.Nil, false
	}
	return orgID, true
}

func userID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

// optionalEventID reads an optional ?eventId= query param.
func optionalEventID(w http.ResponseWriter, r *http.Request) (*uuid.UUID, bool) {
	v := r.URL.Query().Get("eventId")
	if v == "" {
		return nil, true
	}
	id, err := uuid.Parse(v)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return nil, false
	}
	return &id, true
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUnknownReportType):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "UNKNOWN_REPORT_TYPE", "unknown report type"))
	case errors.Is(err, ErrUnsupportedFormat):
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "UNSUPPORTED_FORMAT", "unsupported export format"))
	case errors.Is(err, ErrJobNotFound), errors.Is(err, pgx.ErrNoRows):
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "JOB_NOT_FOUND", "export job not found"))
	case errors.Is(err, ErrJobNotReady):
		apperr.WriteError(w, r, apperr.New(http.StatusConflict, "JOB_NOT_READY", "export job not ready"))
	default:
		apperr.WriteError(w, r, err)
	}
}

// GetSummary returns the on-screen aggregate for a report type.
// GET /organizations/{orgId}/reports/{reportType}/summary?eventId=
func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	reportType := chi.URLParam(r, "reportType")
	if !IsValidReportType(reportType) {
		writeError(w, r, ErrUnknownReportType)
		return
	}
	eventID, ok := optionalEventID(w, r)
	if !ok {
		return
	}
	summary, err := h.svc.GetSummary(r.Context(), orgID, reportType, eventID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, summary)
}

// CreateExport enqueues an async export job.
// POST /organizations/{orgId}/reports/exports
func (h *Handler) CreateExport(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	var body CreateExportRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return
	}
	var eventID *uuid.UUID
	if body.EventID != nil && *body.EventID != "" {
		id, err := uuid.Parse(*body.EventID)
		if err != nil {
			apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
			return
		}
		eventID = &id
	}
	job, err := h.svc.CreateExport(r.Context(), orgID, uid, body.ReportType, eventID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusAccepted, job)
}

// ListExports returns recent export jobs for the org.
// GET /organizations/{orgId}/reports/exports?limit=&offset=
func (h *Handler) ListExports(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	limit, offset := paginationFromQuery(r)
	jobs, err := h.svc.ListJobs(r.Context(), orgID, limit, offset)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, jobs)
}

// GetExport returns a single export job.
// GET /organizations/{orgId}/reports/exports/{jobId}
func (h *Handler) GetExport(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	jobID, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_JOB_ID", "invalid job id"))
		return
	}
	job, err := h.svc.GetJob(r.Context(), jobID, orgID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, job)
}

// PlatformRevenue returns cross-org paid revenue (super-admin only).
// GET /admin/reports/platform-revenue
func (h *Handler) PlatformRevenue(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.PlatformRevenue(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, rows)
}

func paginationFromQuery(r *http.Request) (int32, int32) {
	limit := int32(50)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
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
