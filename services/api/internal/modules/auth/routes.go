package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
)

// RegisterRoutes mounts auth endpoints under /api/v1/auth.
func (h *Handler) RegisterRoutes(r chi.Router, signer *security.JWTSigner, loginGuard, registerGuard func(http.Handler) http.Handler) {
	r.Route("/auth", func(r chi.Router) {
		r.With(registerGuard).Post("/register", h.Register)
		r.With(loginGuard).Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
		r.With(middleware.Authn(signer)).Get("/me", h.Me)
	})
}
