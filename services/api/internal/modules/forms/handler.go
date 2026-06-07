package forms

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ids(w http.ResponseWriter, r *http.Request) (orgID, eventID uuid.UUID, ok bool) {
	var err error
	orgID, err = uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return uuid.Nil, uuid.Nil, false
	}
	eventID, err = uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, eventID, true
}

func (h *Handler) categoryParam(r *http.Request) (*uuid.UUID, error) {
	q := r.URL.Query().Get("categoryId")
	if q == "" {
		return nil, nil
	}
	id, err := uuid.Parse(q)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (h *Handler) GetForm(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	form, err := h.svc.GetForm(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, form)
}

func (h *Handler) UpdateForm(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var req UpdateFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	form, err := h.svc.UpdateForm(r.Context(), orgID, eventID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, form)
}

func (h *Handler) ListFields(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	form, err := h.svc.GetForm(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, form.Fields)
}

func (h *Handler) AddField(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var req FieldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FieldKey == "" || req.FieldType == "" || req.Label == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "fieldType, label, and fieldKey are required"))
		return
	}
	field, err := h.svc.AddField(r.Context(), orgID, eventID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, field)
}

func (h *Handler) UpdateField(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	fieldID, err := uuid.Parse(chi.URLParam(r, "fieldId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_FIELD_ID", "invalid field id"))
		return
	}
	var req FieldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FieldKey == "" || req.FieldType == "" || req.Label == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "fieldType, label, and fieldKey are required"))
		return
	}
	field, err := h.svc.UpdateField(r.Context(), orgID, eventID, fieldID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, field)
}

func (h *Handler) DeleteField(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	fieldID, err := uuid.Parse(chi.URLParam(r, "fieldId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_FIELD_ID", "invalid field id"))
		return
	}
	if err := h.svc.DeleteField(r.Context(), orgID, eventID, fieldID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Reorder(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var req ReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	if err := h.svc.Reorder(r.Context(), orgID, eventID, req.FieldIDs); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	form, err := h.svc.GetForm(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, form)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	catID, err := h.categoryParam(r)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	fields, err := h.svc.Preview(r.Context(), orgID, eventID, catID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, fields)
}

func (h *Handler) PreviewValidate(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	catID, err := h.categoryParam(r)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	var req PreviewValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	res, err := h.svc.PreviewValidate(r.Context(), orgID, eventID, catID, req.Answers)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, res)
}
