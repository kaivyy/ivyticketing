package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
	authmod "github.com/varin/ivyticketing/services/api/internal/modules/auth"
	categoriesmod "github.com/varin/ivyticketing/services/api/internal/modules/categories"
	eventsmod "github.com/varin/ivyticketing/services/api/internal/modules/events"
	formsmod "github.com/varin/ivyticketing/services/api/internal/modules/forms"
	membersmod "github.com/varin/ivyticketing/services/api/internal/modules/members"
	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
	orgsmod "github.com/varin/ivyticketing/services/api/internal/modules/organizations"
	publicmod "github.com/varin/ivyticketing/services/api/internal/modules/publiccatalog"
	rolesmod "github.com/varin/ivyticketing/services/api/internal/modules/roles"
	"github.com/varin/ivyticketing/services/api/internal/modules/system"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
	appmw "github.com/varin/ivyticketing/services/api/internal/platform/middleware"
	"github.com/varin/ivyticketing/services/api/internal/platform/rbac"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
	"github.com/varin/ivyticketing/services/api/internal/platform/storage"
)

func NewRouter(cfg Config, log *slog.Logger, pool *pgxpool.Pool, pg, rdb system.Checker) (http.Handler, error) {
	r := chi.NewRouter()
	r.Use(appmw.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.WebOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		AllowCredentials: true,
	}))

	// System (Phase 1).
	system.NewHandler(pg, rdb).RegisterRoutes(r)

	// Shared deps.
	queries := db.New(pool)
	signer := security.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	loader := rbac.NewLoader(queries)
	secureCookie := cfg.AppEnv != "local"
	auditLog := audit.NewLogger(queries, log)

	authHandler := authmod.NewHandler(
		authmod.NewService(authmod.NewRepository(queries), signer, cfg.AccessTokenTTL, cfg.RefreshTokenTTL),
		secureCookie,
	)
	orgHandler := orgsmod.NewHandler(orgsmod.NewService(orgsmod.NewRepository(pool)))
	memberHandler := membersmod.NewHandler(membersmod.NewService(membersmod.NewRepository(pool), auditLog))
	roleHandler := rolesmod.NewHandler(rolesmod.NewService(rolesmod.NewRepository(pool)))

	store, err := storage.New(storage.Config{
		Driver:        cfg.StorageDriver,
		LocalPath:     cfg.StorageLocalPath,
		PublicBaseURL: cfg.StoragePublicBaseURL,
		Bucket:        cfg.StorageBucket,
		Endpoint:      cfg.StorageEndpoint,
		AccessKey:     cfg.StorageAccessKey,
		SecretKey:     cfg.StorageSecretKey,
		Region:        cfg.StorageRegion,
	})
	if err != nil {
		return nil, err
	}
	eventHandler := eventsmod.NewHandler(eventsmod.NewService(eventsmod.NewRepository(pool), store, auditLog), cfg.StorageUploadMaxBytes)
	categoryHandler := categoriesmod.NewHandler(categoriesmod.NewService(categoriesmod.NewRepository(pool)))
	formHandler := formsmod.NewHandler(formsmod.NewService(formsmod.NewRepository(pool)))
	ordersHandler := ordersmod.NewHandler(ordersmod.NewService(ordersmod.NewRepository(pool), auditLog, cfg.OrderExpiration))
	publicHandler := publicmod.NewHandler(publicmod.NewService(publicmod.NewRepository(pool), store))

	r.Route("/api/v1", func(r chi.Router) {
		// Auth (mixed public/protected; mounts its own /me behind authn).
		authHandler.RegisterRoutes(r, signer)

		// Public read-only (no auth).
		publicHandler.RegisterRoutes(r)

		// Everything else requires authentication.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authn(signer))

			orgHandler.RegisterRoutes(r)
			ordersHandler.RegisterRoutes(r)

			// Per-org sub-resources, authz enforced per route.
			r.Route("/organizations/{orgId}", func(r chi.Router) {
				memberHandler.RegisterRoutes(r, loader)
				roleHandler.RegisterRoutes(r, loader)
				eventHandler.RegisterRoutes(r, loader, func(r chi.Router) {
					categoryHandler.RegisterRoutes(r, loader)
					formHandler.RegisterRoutes(r, loader)
					ordersHandler.RegisterEventRoutes(r, loader)
				})
			})
		})
	})

	// Serve local media files (local driver only).
	if cfg.StorageDriver == "local" {
		fs := http.StripPrefix("/media/", http.FileServer(http.Dir(cfg.StorageLocalPath)))
		r.Get("/media/*", fs.ServeHTTP)
	}

	log.Info("router assembled", "web_origin", cfg.WebOrigin)
	return r, nil
}

func StartServer(ctx context.Context, cfg Config, log *slog.Logger, handler http.Handler) error {
	srv := &http.Server{Addr: ":" + cfg.APIPort, Handler: handler}
	log.Info("api listening", "port", cfg.APIPort)
	return srv.ListenAndServe()
}
