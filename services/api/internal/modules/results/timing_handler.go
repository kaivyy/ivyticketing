package results

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type configureTimingReq struct {
	Provider       string         `json:"provider"`
	Transport      string         `json:"transport"`
	SyncMode       string         `json:"syncMode"`
	Policy         map[string]any `json:"policy"`
	ExternalRaceID string         `json:"externalRaceId"`
	Settings       map[string]any `json:"settings"`
}

type upsertCheckpointReq struct {
	Code            string              `json:"code"`
	Name            string              `json:"name"`
	CheckpointType  string              `json:"checkpointType"`
	OrderIndex      int32               `json:"orderIndex"`
	DistanceMeters  *int32              `json:"distanceMeters"`
	Aliases         []string            `json:"aliases"`
	ProviderAliases map[string][]string `json:"providerAliases"`
}

type createWaveReq struct {
	CategoryID *uuid.UUID `json:"categoryId"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	StartAt    *time.Time `json:"startAt"`
	OrderIndex int32      `json:"orderIndex"`
}

type assignMappingReq struct {
	BibNumber       string `json:"bibNumber"`
	TransponderCode string `json:"transponderCode"`
	ReplaceOld      bool   `json:"replaceOld"`
	Notes           string `json:"notes"`
}

// GetTimingConfig returns configuration for the timing provider (no secret tokens).
func (h *Handler) GetTimingConfig(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	cfg, err := h.svc.GetTimingConfig(r.Context(), eventID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusNotFound, "TIMING_NOT_CONFIGURED", "timing integration not configured for event"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

// ConfigureTiming updates or sets up timing integration and returns the one-time ingestion token.
func (h *Handler) ConfigureTiming(w http.ResponseWriter, r *http.Request) {
	uID, ok := userID(w, r)
	if !ok {
		return
	}
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}

	var req configureTimingReq
	if err := readJSON(r, &req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}

	cfg, rawToken, err := h.svc.ConfigureTiming(
		r.Context(), orgID, eventID, uID,
		req.Provider, req.Transport, req.SyncMode,
		req.Policy, req.ExternalRaceID, req.Settings,
	)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusInternalServerError, "CONFIG_FAILED", err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"config":         cfg,
		"ingestionToken": rawToken,
	})
}

// UpsertCheckpoint creates or updates a checkpoint.
func (h *Handler) UpsertCheckpoint(w http.ResponseWriter, r *http.Request) {
	uID, ok := userID(w, r)
	if !ok {
		return
	}
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}

	var req upsertCheckpointReq
	if err := readJSON(r, &req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}

	if err := h.svc.UpsertCheckpoint(
		r.Context(), orgID, eventID, uID,
		req.Code, req.Name, req.CheckpointType,
		req.OrderIndex, req.DistanceMeters,
		req.Aliases, req.ProviderAliases,
	); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "CHECKPOINT_SAVE_FAILED", err.Error()))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListCheckpoints returns all checkpoints for an event.
func (h *Handler) ListCheckpoints(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListCheckpoints(r.Context(), eventID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusInternalServerError, "LIST_FAILED", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"checkpoints": list})
}

// CreateWave creates a start wave for an event.
func (h *Handler) CreateWave(w http.ResponseWriter, r *http.Request) {
	uID, ok := userID(w, r)
	if !ok {
		return
	}
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}

	var req createWaveReq
	if err := readJSON(r, &req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}

	wv, err := h.svc.CreateWave(r.Context(), orgID, eventID, uID, req.CategoryID, req.Code, req.Name, req.StartAt, req.OrderIndex)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "WAVE_CREATE_FAILED", err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(wv)
}

// ListWaves returns all start waves for an event.
func (h *Handler) ListWaves(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListWaves(r.Context(), eventID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusInternalServerError, "LIST_FAILED", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"waves": list})
}

// AssignMapping maps a BIB to a transponder.
func (h *Handler) AssignMapping(w http.ResponseWriter, r *http.Request) {
	uID, ok := userID(w, r)
	if !ok {
		return
	}
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}

	var req assignMappingReq
	if err := readJSON(r, &req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}

	if err := h.svc.AssignMapping(r.Context(), orgID, eventID, uID, req.BibNumber, req.TransponderCode, req.ReplaceOld, req.Notes); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "MAPPING_SAVE_FAILED", err.Error()))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ImportMappings handles CSV upload of BIB to chip mapping.
func (h *Handler) ImportMappings(w http.ResponseWriter, r *http.Request) {
	uID, ok := userID(w, r)
	if !ok {
		return
	}
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}

	replaceOld := r.URL.Query().Get("replace") == "true"
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	imported, err := h.svc.ImportMappingsCSV(r.Context(), orgID, eventID, uID, r.Body, replaceOld)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "IMPORT_FAILED", err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"imported": imported})
}

// ListMappings lists transponder mappings for an event.
func (h *Handler) ListMappings(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListMappings(r.Context(), eventID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusInternalServerError, "LIST_FAILED", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"mappings": list})
}

// IngestPassings receives raw timing observations pushed by RACE RESULT Exporter or forwarded stream.
func (h *Handler) IngestPassings(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}

	token := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	} else if th := r.Header.Get("X-Timing-Token"); th != "" {
		token = th
	} else {
		token = r.URL.Query().Get("token")
	}

	if token == "" || !h.svc.ValidateIngestionToken(r.Context(), eventID, token) {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "INVALID_TIMING_TOKEN", "invalid timing token"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "PAYLOAD_TOO_LARGE", "payload too large or unreadable"))
		return
	}

	defaultCP := r.URL.Query().Get("checkpoint")
	if defaultCP == "" {
		defaultCP = r.Header.Get("X-Checkpoint")
	}

	count, err := h.svc.IngestPassings(r.Context(), orgID, eventID, defaultCP, body)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INGESTION_FAILED", err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":   "accepted",
		"ingested": count,
	})
}

// ProcessTiming triggers processor evaluation of unprocessed passings.
func (h *Handler) ProcessTiming(w http.ResponseWriter, r *http.Request) {
	uID, ok := userID(w, r)
	if !ok {
		return
	}
	orgID, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}

	summary, err := h.svc.ProcessTiming(r.Context(), orgID, eventID, uID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusInternalServerError, "PROCESS_FAILED", err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

// GetPassingStats returns passing telemetry counters.
func (h *Handler) GetPassingStats(w http.ResponseWriter, r *http.Request) {
	_, eventID, ok := parseOrgEvent(w, r)
	if !ok {
		return
	}
	stats, err := h.svc.GetPassingStats(r.Context(), eventID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusInternalServerError, "STATS_FAILED", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}
