package events

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

const presignTTL = 10 * time.Minute

var imageExt = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

func validImageContentType(ct string) bool {
	_, ok := imageExt[ct]
	return ok
}

func mediaKeyPrefix(orgID, eventID uuid.UUID, kind string) string {
	return "org/" + orgID.String() + "/event/" + eventID.String() + "/" + kind + "/"
}

func validateObjectKey(key string, orgID, eventID uuid.UUID, kind string) error {
	if strings.Contains(key, "..") {
		return ErrInvalidObjectKey
	}
	if !strings.HasPrefix(key, mediaKeyPrefix(orgID, eventID, kind)) {
		return ErrInvalidObjectKey
	}
	return nil
}

type ticketRequest struct {
	ContentType string `json:"contentType"`
	FileName    string `json:"fileName"`
}

type confirmRequest struct {
	ObjectKey string `json:"objectKey"`
}

// RequestTicket issues an upload ticket (presigned for cloud, direct for local).
func (h *Handler) RequestTicket(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	kind := chi.URLParam(r, "kind")
	if kind != "banner" && kind != "logo" {
		apperr.WriteError(w, r, ErrInvalidMediaKind)
		return
	}
	var req ticketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	ext, ok := imageExt[req.ContentType]
	if !ok {
		apperr.WriteError(w, r, ErrInvalidContent)
		return
	}
	if _, err := h.svc.Get(r.Context(), orgID, eventID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}

	objectKey := mediaKeyPrefix(orgID, eventID, kind) + uuid.NewString() + "." + ext
	ticket, presigned, err := h.svc.store.PresignUpload(r.Context(), objectKey, req.ContentType, presignTTL)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	if presigned {
		apperr.WriteJSON(w, http.StatusOK, map[string]any{
			"mode": "presigned", "objectKey": objectKey, "upload": ticket,
		})
		return
	}
	// local: client POSTs multipart to the upload URL
	apperr.WriteJSON(w, http.StatusOK, map[string]any{
		"mode":      "direct",
		"objectKey": objectKey,
		"uploadUrl": "/api/v1/organizations/" + orgID.String() + "/events/" + eventID.String() + "/media/" + kind + "/upload?key=" + objectKey,
	})
}

// UploadDirect is the local-only multipart sink.
func (h *Handler) UploadDirect(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	kind := chi.URLParam(r, "kind")
	key := r.URL.Query().Get("key")
	if err := validateObjectKey(key, orgID, eventID, kind); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)
	if err := r.ParseMultipartForm(h.maxUploadBytes); err != nil {
		apperr.WriteError(w, r, ErrFileTooLarge)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "missing file field"))
		return
	}
	defer file.Close()
	if !validImageContentType(header.Header.Get("Content-Type")) {
		apperr.WriteError(w, r, ErrInvalidContent)
		return
	}
	if err := h.svc.store.Put(r.Context(), key, file, header.Header.Get("Content-Type")); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ConfirmMedia validates the key and persists it onto the event.
func (h *Handler) ConfirmMedia(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	kind := chi.URLParam(r, "kind")
	if kind != "banner" && kind != "logo" {
		apperr.WriteError(w, r, ErrInvalidMediaKind)
		return
	}
	var req confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	if err := validateObjectKey(req.ObjectKey, orgID, eventID, kind); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	ev, err := h.svc.SetMedia(r.Context(), orgID, eventID, kind, req.ObjectKey)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}

// Ensure db and pgtype imports are used (referenced in SetMedia via service).
var _ = db.SetEventMediaKeyParams{}
var _ = pgtype.Text{}
