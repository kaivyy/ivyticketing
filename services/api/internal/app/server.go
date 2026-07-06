package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/varin/ivyticketing/services/api/internal/db"
	abusemod "github.com/varin/ivyticketing/services/api/internal/modules/abuse"
	accessmod "github.com/varin/ivyticketing/services/api/internal/modules/access"
	authmod "github.com/varin/ivyticketing/services/api/internal/modules/auth"
	ballotmod "github.com/varin/ivyticketing/services/api/internal/modules/ballot"
	categoriesmod "github.com/varin/ivyticketing/services/api/internal/modules/categories"
	eventsmod "github.com/varin/ivyticketing/services/api/internal/modules/events"
	formsmod "github.com/varin/ivyticketing/services/api/internal/modules/forms"
	lifecyclemod "github.com/varin/ivyticketing/services/api/internal/modules/lifecycle"
	membersmod "github.com/varin/ivyticketing/services/api/internal/modules/members"
	notifmod "github.com/varin/ivyticketing/services/api/internal/modules/notifications"
	notifemail "github.com/varin/ivyticketing/services/api/internal/modules/notifications/email"
	notiftmpl "github.com/varin/ivyticketing/services/api/internal/modules/notifications/templates"
	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
	billingmod "github.com/varin/ivyticketing/services/api/internal/modules/billing"
	orgsmod "github.com/varin/ivyticketing/services/api/internal/modules/organizations"
	paymentsmod "github.com/varin/ivyticketing/services/api/internal/modules/payments"
	publicmod "github.com/varin/ivyticketing/services/api/internal/modules/publiccatalog"
	queuemod "github.com/varin/ivyticketing/services/api/internal/modules/queue"
	racepackmod "github.com/varin/ivyticketing/services/api/internal/modules/racepack"
	registrationmod "github.com/varin/ivyticketing/services/api/internal/modules/registration"
	reportingmod "github.com/varin/ivyticketing/services/api/internal/modules/reporting"
	rolesmod "github.com/varin/ivyticketing/services/api/internal/modules/roles"
	scannermod "github.com/varin/ivyticketing/services/api/internal/modules/scanner"
	"github.com/varin/ivyticketing/services/api/internal/modules/system"
	ticketsmod "github.com/varin/ivyticketing/services/api/internal/modules/tickets"
	waitlistmod "github.com/varin/ivyticketing/services/api/internal/modules/waitlist"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	"github.com/varin/ivyticketing/services/api/internal/platform/captcha"
	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
	appmw "github.com/varin/ivyticketing/services/api/internal/platform/middleware"
	platformqueue "github.com/varin/ivyticketing/services/api/internal/platform/queue"
	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
	"github.com/varin/ivyticketing/services/api/internal/platform/rbac"
	"github.com/varin/ivyticketing/services/api/internal/platform/security"
	"github.com/varin/ivyticketing/services/api/internal/platform/storage"
)

func NewRouter(cfg Config, log *slog.Logger, pool *pgxpool.Pool, pg, rdb system.Checker, redisClient *goredis.Client) (http.Handler, error) {
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
	registrationRepo := registrationmod.NewRepository(pool)
	registrationSvc := registrationmod.NewService(registrationRepo)
	registrationHandler := registrationmod.NewHandler(registrationSvc)

	// platform/queue adapter
	queueAdapter := platformqueue.New(redisClient)
	queueStore := queuemod.NewStore(queueAdapter)
	queueRepo := queuemod.NewRepository(pool)
	queueEventReader := queuemod.NewDBEventReader(db.New(pool))
	queueSvc := queuemod.NewService(queueRepo, queueStore, auditLog, queueEventReader, int32(cfg.QueueDefaultReleaseRate), registrationSvc)
	queueHandler := queuemod.NewHandler(queueSvc)

	lifecycleRepo := lifecyclemod.NewRepository(pool)
	lifecycleSvc := lifecyclemod.NewService(lifecycleRepo)

	// Ballot module (Phase 10 Part 2)
	accessRepo := accessmod.NewRepository(pool)
	poolMgr := accessmod.NewPoolManager(accessRepo)
	eligibilityChecker := accessmod.NewEligibilityChecker(accessRepo)
	poolSvc := accessmod.NewPoolService(accessRepo)
	corporateSvc := accessmod.NewCorporateService(accessRepo)
	codeSvc := accessmod.NewCodeService(accessRepo, eligibilityChecker)
	accessHandler := accessmod.NewHandler(codeSvc, poolSvc, corporateSvc)
	waitlistRepo := waitlistmod.NewRepository(pool)
	waitlistSvc := waitlistmod.NewService(waitlistRepo, poolMgr)
	accessHandler.WithPriorityAndWaitlist(
		accessmod.NewPriorityChecker(accessRepo, lifecycleSvc, eligibilityChecker),
		waitlistRepo,
		waitlistSvc,
	)
	ballotRepo := ballotmod.NewRepository(pool)
	ballotSvc := ballotmod.NewService(ballotRepo, auditLog, poolMgr, poolMgr, waitlistSvc)
	ballotHandler := ballotmod.NewHandler(ballotSvc)
	ballotExpirer := ballotmod.NewWinnerExpirer(ballotRepo, waitlistSvc)
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				_ = ballotExpirer.Run(context.Background())
			}
		}
	}()

	priorityChecker := accessmod.NewPriorityChecker(accessRepo, lifecycleSvc, eligibilityChecker)
	registrationGate := registrationmod.NewGate(registrationSvc, queueSvc, lifecycleSvc, ballotSvc, poolMgr, priorityChecker)

	ordersHandler := ordersmod.NewHandler(ordersmod.NewService(ordersmod.NewRepository(pool), auditLog, cfg.OrderExpiration, registrationGate, queueSvc))
	publicHandler := publicmod.NewHandler(publicmod.NewService(publicmod.NewRepository(pool), store))

	qrSigner := ticketsmod.NewQRSigner(cfg.TicketQRSecret)
	ticketsRepo := ticketsmod.NewRepository(pool)
	ticketIssuer := ticketsmod.NewIssuer(auditLog)
	ticketsSvc := ticketsmod.NewService(ticketsRepo, qrSigner, auditLog)
	ticketsHandler := ticketsmod.NewHandler(ticketsSvc)

	paymentsRegistry := BuildPaymentRegistry(cfg)
	paymentsRepo := paymentsmod.NewRepository(pool)
	paymentsProc := paymentsmod.NewProcessor(paymentsRepo, auditLog, ticketIssuer)
	paymentsSvc := paymentsmod.NewService(paymentsRepo, paymentsRegistry, auditLog, cfg.PaymentDefaultExpiry)
	paymentsReconciler := paymentsmod.NewReconciler(paymentsRepo, paymentsRegistry, paymentsProc)
	paymentsHandler := paymentsmod.NewHandler(paymentsSvc, paymentsReconciler)

	// Notifications (Phase 12)
	notifRepo := notifmod.NewRepository(pool)
	notifSender := notifemail.NewSenderFromConfig(notifemail.SenderConfig{
		Driver:      cfg.EmailDriver,
		SMTPHost:    cfg.SMTPHost,
		SMTPPort:    cfg.SMTPPort,
		SMTPUser:    cfg.SMTPUser,
		SMTPPass:    cfg.SMTPPass,
		FromName:    cfg.EmailFromName,
		FromAddress: cfg.EmailFromAddress,
	}, log)
	notifLookup := notifmod.NewParticipantLookup(queries)
	notifResolver := notiftmpl.NewResolver(notifRepo)
	notifSvc := notifmod.NewService(notifRepo, notifSender, notifLookup, notifResolver, log)
	// Wire notifier into each service via WithNotifier (duck-typed, no circular imports).
	// Extract orders service to call WithNotifier on it.
	ordersSvc := ordersmod.NewService(ordersmod.NewRepository(pool), auditLog, cfg.OrderExpiration, registrationGate, queueSvc)
	ordersSvc.WithNotifier(notifSvc)
	ordersSvc.WithLogger(log)
	ordersHandler = ordersmod.NewHandler(ordersSvc)
	paymentsProc.WithNotifier(notifSvc)
	queueSvc.WithNotifier(notifSvc)
	ballotSvc.WithNotifier(notifSvc)
	waitlistSvc.WithNotifier(notifSvc)

	// Notification status endpoint (mounted under /api/v1/admin/notifications/status with RequirePlatformAdmin)
	smtpConfigured := cfg.EmailDriver == "smtp" && cfg.SMTPHost != "" && cfg.SMTPPort != ""
	notifStatusHandler := notifmod.NewStatusHandler(cfg.EmailDriver, smtpConfigured, notifmod.MaxRetryAttempts)

	// Racepack (Phase 14)
	racepackRepo := racepackmod.NewRepository(pool)
	racepackSvc := racepackmod.NewService(racepackRepo, auditLog, log)
	racepackHandler := racepackmod.NewHandler(racepackSvc)

	// Scanner (Phase 15). Composes the SAME qr.Signer built for tickets above
	// (the TICKET_QR_SECRET is never duplicated), a read-only ticket display
	// surface, the racepack service as the pickup collaborator + pickup-status
	// reader, and the shared audit logger. Routes are mounted below.
	scannerSvc := scannermod.NewService(
		qrSigner,
		scannermod.NewTicketReader(pool),
		racepackSvc,
		scannermod.NewRepository(pool),
		auditLog,
		log,
	)
	scannerHandler := scannermod.NewHandler(scannerSvc)

	// Reporting & Export (Phase 16). Summaries are read synchronously; exports
	// are enqueued as PENDING jobs and generated by the worker (see cmd/worker).
	reportingSvc := reportingmod.NewService(reportingmod.NewRepository(pool), store, auditLog, log)
	reportingHandler := reportingmod.NewHandler(reportingSvc)

	// Billing (Phase 17). Package catalog, per-org subscriptions, platform fee
	// ledger (hooked into the payments PAID transition below), and invoices.
	billingSvc := billingmod.NewService(billingmod.NewRepository(pool), auditLog, log)
	billingHandler := billingmod.NewHandler(billingSvc)
	paymentsProc.WithFeeRecorder(billingSvc)

	// Anti-bot / abuse (Phase 9)
	abuseRepo := abusemod.NewRepository(pool)
	abuseSettings := abusemod.NewSettings(abuseRepo)
	_ = abuseSettings.Refresh(context.Background())
	abuseSettings.StartRefresh(context.Background(), cfg.AbuseSettingsRefresh)
	rateLimiter := ratelimit.New(redisClient)
	abuseRate := abusemod.NewRateChecker(rateLimiter)
	abuseBlocklist := abusemod.NewBlocklist(abuseRepo)
	abuseReputation := abusemod.NewReputation(abuseRepo, cfg.ReputationChallengeThreshold, cfg.ReputationDenyThreshold)
	var captchaVerifier captcha.Verifier = captcha.NewTurnstile(cfg.TurnstileSecret)
	abuseSvc := abusemod.NewService(abuseRepo, abuseSettings, auditLog, cfg.MaxActiveQueuePerUser, cfg.TurnstileSiteKey)
	abuseGuard := abusemod.NewGuard(abuseSettings, abuseBlocklist, abuseRate, abuseReputation, captchaVerifier, abuseSvc, abuseSvc)
	abuseHandler := abusemod.NewHandler(abuseSvc)
	securityConfigHandler := abusemod.NewSecurityConfigHandler(abuseSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Auth (mixed public/protected; mounts its own /me behind authn).
		authHandler.RegisterRoutes(r, signer,
			abuseGuard.Middleware(abusemod.CategoryAuthLogin),
			abuseGuard.Middleware(abusemod.CategoryAuthRegister),
		)

		// Public read-only (no auth).
		publicHandler.RegisterRoutes(r)

		// Security config (public, no auth).
		r.Get("/security/config", securityConfigHandler.Get)

		// Everything else requires authentication.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authn(signer))

			orgHandler.RegisterRoutes(r)
			ordersHandler.RegisterRoutes(r)
			paymentsHandler.RegisterRoutes(r)
			ticketsHandler.RegisterRoutes(r)
			queueHandler.RegisterRoutes(r, abuseGuard.Middleware(abusemod.CategoryQueueJoin))
			ballotHandler.RegisterParticipantRoutes(r, abuseGuard.Middleware(abusemod.CategoryBallotApply))
			accessHandler.RegisterParticipantRoutes(r)
			racepackHandler.RegisterParticipantRoutes(r, func(h http.Handler) http.Handler {
				return middleware.Authn(signer)(h)
			})

			// Scanner permitted-events listing (Phase 15). Authenticated but NOT
			// org-scoped: GET /scan/events returns the caller's Permitted_Events
			// across every org they belong to, so it is mounted here rather than
			// under /organizations/{orgId}/events/{eventId}.
			scannerHandler.RegisterUserRoutes(r)

			// Super-admin abuse + access management.
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequirePlatformAdmin())
				r.Route("/admin", func(r chi.Router) {
					abuseHandler.RegisterAdminRoutes(r)
					reportingHandler.RegisterAdminRoutes(r)
					billingHandler.RegisterAdminRoutes(r)
				})
				accessHandler.RegisterAdminRoutes(r)
				notifStatusHandler.RegisterRoutes(r)
			})

			// Per-org sub-resources, authz enforced per route.
			r.Route("/organizations/{orgId}", func(r chi.Router) {
				memberHandler.RegisterRoutes(r, loader)
				roleHandler.RegisterRoutes(r, loader)
				eventHandler.RegisterRoutes(r, loader, func(r chi.Router) {
					categoryHandler.RegisterRoutes(r, loader)
					formHandler.RegisterRoutes(r, loader)
					ordersHandler.RegisterEventRoutes(r, loader, abuseGuard.Middleware(abusemod.CategoryCheckout))
					ticketsHandler.RegisterEventRoutes(r, loader)
					registrationHandler.RegisterEventRoutes(r, loader)
					queueHandler.RegisterOrgRoutes(r, loader)
					ballotHandler.RegisterOrganizerRoutes(r)
					accessHandler.RegisterOrganizerRoutes(r)
					racepackHandler.RegisterEventRoutes(r, loader)
					scannerHandler.RegisterEventRoutes(r, loader)
				})
				paymentsHandler.RegisterOrgRoutes(r, loader)
				reportingHandler.RegisterOrgRoutes(r, loader)
				billingHandler.RegisterOrgRoutes(r, loader)
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
