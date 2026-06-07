package auth

import (
	"encoding/json"
	"net/http"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

const refreshCookieName = "refresh_token"
const refreshCookiePath = "/api/v1/auth"

type Handler struct {
	svc    *Service
	secure bool // Secure cookie flag (true in non-local env)
}

func NewHandler(svc *Service, secure bool) *Handler {
	return &Handler{svc: svc, secure: secure}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed request body"))
		return
	}
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "email, password, and fullName are required"))
		return
	}
	user, err := h.svc.Register(r.Context(), req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed request body"))
		return
	}
	res, err := h.svc.Login(r.Context(), req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	h.setRefreshCookie(w, res.RefreshToken, res.RefreshTTL)
	apperr.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	raw := readRefreshCookie(r)
	res, err := h.svc.Refresh(r.Context(), raw)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	h.setRefreshCookie(w, res.RefreshToken, res.RefreshTTL)
	apperr.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	raw := readRefreshCookie(r)
	if err := h.svc.Logout(r.Context(), raw); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	u, err := h.svc.GetUser(r.Context(), id.UserID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toUserResponse(u))
}

func (h *Handler) setRefreshCookie(w http.ResponseWriter, raw string, ttlSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    raw,
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   ttlSeconds,
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func readRefreshCookie(r *http.Request) string {
	c, err := r.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
