package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/varin/ivyticketing/services/api/internal/modules/system"
	appmw "github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

func NewRouter(cfg Config, log *slog.Logger, pg, rdb system.Checker) http.Handler {
	r := chi.NewRouter()
	r.Use(appmw.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.WebOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Request-Id"},
		AllowCredentials: false,
	}))

	sys := system.NewHandler(pg, rdb)
	sys.RegisterRoutes(r)

	log.Info("router assembled", "web_origin", cfg.WebOrigin)
	return r
}

func StartServer(ctx context.Context, cfg Config, log *slog.Logger, handler http.Handler) error {
	srv := &http.Server{Addr: ":" + cfg.APIPort, Handler: handler}
	log.Info("api listening", "port", cfg.APIPort)
	return srv.ListenAndServe()
}
