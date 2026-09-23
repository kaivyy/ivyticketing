package competitions

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// Handler handles HTTP requests for multi-sport competitions.
type Handler struct {
	svc *Service
}

// NewHandler constructs a competitions handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func writeErr(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	apperr.WriteError(w, r, apperr.New(status, code, msg))
}

// ListSports handles GET /api/v1/sports
func (h *Handler) ListSports(w http.ResponseWriter, r *http.Request) {
	sports, err := h.svc.ListSports(r.Context())
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil daftar olahraga")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sports": sports})
}

// ListDisciplines handles GET /api/v1/sports/{sportId}/disciplines
func (h *Handler) ListDisciplines(w http.ResponseWriter, r *http.Request) {
	sportID := chi.URLParam(r, "sportId")
	disciplines, err := h.svc.ListDisciplines(r.Context(), sportID)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil daftar disiplin")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"disciplines": disciplines})
}

// GetCompetitionConfig handles GET /api/v1/events/{eventId}/competition
func (h *Handler) GetCompetitionConfig(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}
	cfg, err := h.svc.GetCompetitionConfig(r.Context(), eventID)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil konfigurasi kompetisi")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// ConfigureCompetition handles PUT /api/v1/orgs/{orgId}/events/{eventId}/competition
func (h *Handler) ConfigureCompetition(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Organization ID tidak valid")
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}

	var req CompetitionConfigView
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_PAYLOAD", "Payload tidak valid")
		return
	}

	cfg, err := h.svc.ConfigureCompetition(r.Context(), orgID, eventID, req.CategoryID, req)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal menyimpan konfigurasi kompetisi")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// ListTeams handles GET /api/v1/events/{eventId}/teams
func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}
	teams, err := h.svc.ListTeams(r.Context(), eventID)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil daftar tim")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

// CreateTeamRequest is the body for creating a team.
type CreateTeamRequest struct {
	CategoryID    *uuid.UUID `json:"categoryId,omitempty"`
	Name          string     `json:"name"`
	ShortName     string     `json:"shortName,omitempty"`
	ManagerUserID *uuid.UUID `json:"managerUserId,omitempty"`
	LogoURL       string     `json:"logoUrl,omitempty"`
	SeedNumber    *int       `json:"seedNumber,omitempty"`
}

// CreateTeam handles POST /api/v1/orgs/{orgId}/events/{eventId}/teams
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Organization ID tidak valid")
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}

	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_PAYLOAD", "Payload tidak valid")
		return
	}

	team, err := h.svc.CreateTeam(r.Context(), orgID, eventID, req.CategoryID, req.Name, req.ShortName, req.ManagerUserID, req.LogoURL, req.SeedNumber)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal membuat tim")
		return
	}
	writeJSON(w, http.StatusCreated, team)
}

// ListTeamRoster handles GET /api/v1/events/{eventId}/teams/{teamId}/roster
func (h *Handler) ListTeamRoster(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "teamId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Team ID tidak valid")
		return
	}
	roster, err := h.svc.ListTeamRoster(r.Context(), teamID)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil anggota tim")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roster": roster})
}

// AddRosterMemberRequest is the body for adding an athlete to a roster.
type AddRosterMemberRequest struct {
	TicketID     *uuid.UUID `json:"ticketId,omitempty"`
	UserID       *uuid.UUID `json:"userId,omitempty"`
	PlayerName   string     `json:"playerName"`
	JerseyNumber *int       `json:"jerseyNumber,omitempty"`
	Position     string     `json:"position,omitempty"`
	IsCaptain    bool       `json:"isCaptain"`
}

// AddRosterMember handles POST /api/v1/orgs/{orgId}/events/{eventId}/teams/{teamId}/roster
func (h *Handler) AddRosterMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "teamId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Team ID tidak valid")
		return
	}

	var req AddRosterMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_PAYLOAD", "Payload tidak valid")
		return
	}

	member, err := h.svc.AddRosterMember(r.Context(), teamID, req.TicketID, req.UserID, req.PlayerName, req.JerseyNumber, req.Position, req.IsCaptain)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal menambahkan anggota tim")
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

// ListStages handles GET /api/v1/events/{eventId}/stages
func (h *Handler) ListStages(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}
	stages, err := h.svc.ListStages(r.Context(), eventID)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil daftar babak")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stages": stages})
}

// CreateStageRequest is the body for creating a stage.
type CreateStageRequest struct {
	CategoryID           *uuid.UUID  `json:"categoryId,omitempty"`
	Name                 string      `json:"name"`
	StageType            StageType   `json:"stageType"`
	SequenceOrder        int         `json:"sequenceOrder"`
	AutoGenerateFixtures bool        `json:"autoGenerateFixtures"`
	EntryIDs             []uuid.UUID `json:"entryIds,omitempty"`
}

// CreateStage handles POST /api/v1/orgs/{orgId}/events/{eventId}/stages
func (h *Handler) CreateStage(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Organization ID tidak valid")
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}

	var req CreateStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_PAYLOAD", "Payload tidak valid")
		return
	}

	stage, matches, err := h.svc.CreateStage(r.Context(), orgID, eventID, req.CategoryID, req.Name, req.StageType, req.SequenceOrder, req.AutoGenerateFixtures, req.EntryIDs)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal membuat babak kompetisi")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"stage":   stage,
		"matches": matches,
	})
}

// ListMatches handles GET /api/v1/events/{eventId}/stages/{stageId}/matches
func (h *Handler) ListMatches(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}
	stageID, err := uuid.Parse(chi.URLParam(r, "stageId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Stage ID tidak valid")
		return
	}
	matches, err := h.svc.ListMatches(r.Context(), eventID, stageID)
	if err != nil {
		if errors.Is(err, ErrStageNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "Babak kompetisi tidak ditemukan")
			return
		}
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil daftar pertandingan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"matches": matches})
}

// RecordMatchScoreRequest is the body for updating match scores.
type RecordMatchScoreRequest struct {
	HomeScore int                `json:"homeScore"`
	AwayScore int                `json:"awayScore"`
	Status    string             `json:"status"`
	WinnerID  *uuid.UUID         `json:"winnerId,omitempty"`
	Details   []MatchScoreDetail `json:"details,omitempty"`
}

// RecordMatchScore handles POST /api/v1/orgs/{orgId}/events/{eventId}/matches/{matchId}/score
func (h *Handler) RecordMatchScore(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Organization ID tidak valid")
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}
	matchID, err := uuid.Parse(chi.URLParam(r, "matchId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Match ID tidak valid")
		return
	}

	var req RecordMatchScoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_PAYLOAD", "Payload tidak valid")
		return
	}

	match, err := h.svc.RecordMatchScore(r.Context(), orgID, eventID, matchID, req.HomeScore, req.AwayScore, req.Status, req.WinnerID, req.Details)
	if err != nil {
		if errors.Is(err, ErrInvalidStatus) {
			writeErr(w, r, http.StatusBadRequest, "INVALID_STATUS", "Status pertandingan tidak valid")
			return
		}
		if errors.Is(err, ErrInvalidScore) {
			writeErr(w, r, http.StatusBadRequest, "INVALID_SCORE", "Skor pertandingan tidak valid")
			return
		}
		if errors.Is(err, ErrDownstreamMatchLocked) {
			writeErr(w, r, http.StatusConflict, "DOWNSTREAM_MATCH_LOCKED", "Pertandingan babak selanjutnya sudah berlangsung atau selesai")
			return
		}
		if errors.Is(err, ErrMatchNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "Pertandingan tidak ditemukan")
			return
		}
		if errors.Is(err, ErrStageNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "Babak kompetisi tidak ditemukan")
			return
		}
		if errors.Is(err, ErrEventNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "Event tidak ditemukan")
			return
		}
		if errors.Is(err, ErrEventForbidden) {
			writeErr(w, r, http.StatusForbidden, "FORBIDDEN", "Akses event ditolak")
			return
		}
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mencatat skor pertandingan")
		return
	}
	writeJSON(w, http.StatusOK, match)
}

// GetStageStandings handles GET /api/v1/events/{eventId}/stages/{stageId}/standings
func (h *Handler) GetStageStandings(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}
	stageID, err := uuid.Parse(chi.URLParam(r, "stageId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Stage ID tidak valid")
		return
	}

	standings, err := h.svc.GetStageStandings(r.Context(), eventID, stageID)
	if err != nil {
		if errors.Is(err, ErrStageNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "Babak kompetisi tidak ditemukan")
			return
		}
		if errors.Is(err, ErrEventNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "Event tidak ditemukan")
			return
		}
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil klasemen")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"standings": standings})
}

// AdjudicateEntryRequest is the body for modifying an entry's result status.
type AdjudicateEntryRequest struct {
	NewStatus   CompetitionStatus `json:"newStatus"`
	Reason      string            `json:"reason"`
	ActorUserID *uuid.UUID        `json:"actorUserId,omitempty"`
}

// AdjudicateEntry handles POST /organizations/{orgId}/events/{eventId}/competition/stages/{stageId}/entries/{entryId}/adjudicate
func (h *Handler) AdjudicateEntry(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Organization ID tidak valid")
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}
	stageID, err := uuid.Parse(chi.URLParam(r, "stageId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Stage ID tidak valid")
		return
	}
	entryID, err := uuid.Parse(chi.URLParam(r, "entryId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Entry ID tidak valid")
		return
	}

	var req AdjudicateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_PAYLOAD", "Payload tidak valid")
		return
	}

	actorID := uuid.Nil
	if req.ActorUserID != nil {
		actorID = *req.ActorUserID
	}

	if err := h.svc.AdjudicateEntry(r.Context(), orgID, eventID, stageID, entryID, req.NewStatus, req.Reason, actorID); err != nil {
		if errors.Is(err, ErrInvalidStatus) {
			writeErr(w, r, http.StatusBadRequest, "INVALID_STATUS", "Status adjudikasi tidak valid")
			return
		}
		if errors.Is(err, ErrEventNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "Event tidak ditemukan")
			return
		}
		if errors.Is(err, ErrEventForbidden) {
			writeErr(w, r, http.StatusForbidden, "FORBIDDEN", "Akses event ditolak")
			return
		}
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal menyimpan adjudikasi")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// ProcessCyclingStageRequest represents request payload to process peloton grouping for a cycling stage.
type ProcessCyclingStageRequest struct {
	MaxGapMs int64                 `json:"maxGapMs"`
	Records  []CyclingFinishRecord `json:"records,omitempty"`
}

// ProcessCyclingStage handles POST /organizations/{orgId}/events/{eventId}/competition/stages/{stageId}/process-cycling
func (h *Handler) ProcessCyclingStage(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Organization ID tidak valid")
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Event ID tidak valid")
		return
	}
	stageID, err := uuid.Parse(chi.URLParam(r, "stageId"))
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "INVALID_ID", "Stage ID tidak valid")
		return
	}

	var req ProcessCyclingStageRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	results, err := h.svc.ProcessCyclingStageResults(r.Context(), orgID, eventID, stageID, req.MaxGapMs, req.Records)
	if err != nil {
		if errors.Is(err, ErrNotPelotonStage) {
			writeErr(w, r, http.StatusBadRequest, "NOT_CYCLING_STAGE", "Babak ini bukan merupakan babak balap sepeda / peloton")
			return
		}
		if errors.Is(err, ErrNoFinishRecords) {
			writeErr(w, r, http.StatusBadRequest, "NO_FINISH_RECORDS", "Tidak ditemukan data catatan finis untuk babak ini")
			return
		}
		if errors.Is(err, ErrStageNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "Babak kompetisi tidak ditemukan")
			return
		}
		if errors.Is(err, ErrEventNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "Event tidak ditemukan")
			return
		}
		if errors.Is(err, ErrEventForbidden) {
			writeErr(w, r, http.StatusForbidden, "FORBIDDEN", "Akses event ditolak")
			return
		}
		writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memproses hasil babak balap sepeda")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// Helper to write JSON responses
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// Helper to parse int params
func parseIntQuery(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}
