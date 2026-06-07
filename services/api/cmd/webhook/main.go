package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/varin/ivyticketing/services/api/internal/app"
	"github.com/varin/ivyticketing/services/api/internal/db"
	paymentsmod "github.com/varin/ivyticketing/services/api/internal/modules/payments"
	webhookhttp "github.com/varin/ivyticketing/services/api/internal/modules/payments/webhook/http"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	"github.com/varin/ivyticketing/services/api/internal/platform/database"
	"github.com/varin/ivyticketing/services/api/internal/platform/logger"
)

func main() {
	cfg, err := app.LoadConfig()
	log := logger.New(cfg.AppEnv)
	if err != nil {
		log.Error("config load failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pg, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	auditLog := audit.NewLogger(db.New(pg.Pool), log)
	registry := app.BuildPaymentRegistry(cfg)
	repo := paymentsmod.NewRepository(pg.Pool)
	proc := paymentsmod.NewProcessor(repo, auditLog)

	srv := webhookhttp.NewServer(proc, registry)
	addr := ":" + cfg.WebhookPort
	log.Info("webhook server starting", "addr", addr)

	httpSrv := &http.Server{Addr: addr, Handler: srv.Router()}
	go func() {
		<-ctx.Done()
		_ = httpSrv.Shutdown(context.Background())
	}()
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("webhook server error", "error", err)
	}
	log.Info("webhook server exited")
}
