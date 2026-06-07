package system

import (
	"context"
	"encoding/json"
	"net/http"
)

type Checker interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	postgres Checker
	redis    Checker
}

func NewHandler(postgres, redis Checker) *Handler {
	return &Handler{postgres: postgres, redis: redis}
}

type readyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checks := map[string]string{
		"postgres": pingStatus(ctx, h.postgres),
		"redis":    pingStatus(ctx, h.redis),
	}
	status := http.StatusOK
	state := "ready"
	for _, v := range checks {
		if v != "ok" {
			status = http.StatusServiceUnavailable
			state = "not_ready"
			break
		}
	}
	writeJSON(w, status, readyResponse{Status: state, Checks: checks})
}

func pingStatus(ctx context.Context, c Checker) string {
	if c == nil {
		return "down"
	}
	if err := c.Ping(ctx); err != nil {
		return "down"
	}
	return "ok"
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
