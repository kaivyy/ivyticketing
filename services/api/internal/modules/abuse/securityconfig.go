package abuse

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// SecurityConfigHandler serves the public client config (turnstile on/off + site key).
type SecurityConfigHandler struct{ svc *Service }

func NewSecurityConfigHandler(svc *Service) *SecurityConfigHandler {
	return &SecurityConfigHandler{svc: svc}
}

func (h *SecurityConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	apperr.WriteJSON(w, http.StatusOK, h.svc.SecurityConfig())
}
