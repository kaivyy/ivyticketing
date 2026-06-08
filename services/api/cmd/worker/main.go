package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/varin/ivyticketing/services/api/internal/app"
	"github.com/varin/ivyticketing/services/api/internal/db"
	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
	queuemod "github.com/varin/ivyticketing/services/api/internal/modules/queue"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	"github.com/varin/ivyticketing/services/api/internal/platform/database"
	"github.com/varin/ivyticketing/services/api/internal/platform/logger"
	platformqueue "github.com/varin/ivyticketing/services/api/internal/platform/queue"
	"github.com/varin/ivyticketing/services/api/internal/platform/redis"
	"github.com/varin/ivyticketing/services/api/internal/platform/worker"
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
	svc := ordersmod.NewService(ordersmod.NewRepository(pg.Pool), auditLog, cfg.OrderExpiration, nil)

	rdb, err := redis.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Error("redis connect failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	queueSvc := queuemod.NewService(
		queuemod.NewRepository(pg.Pool),
		queuemod.NewStore(platformqueue.New(rdb.Client)),
		auditLog,
		queuemod.NewDBEventReader(db.New(pg.Pool)),
		int32(cfg.QueueDefaultReleaseRate),
	)

	releaseRunner := worker.New("queue_release", cfg.QueueReleaseInterval, queueSvc.ReleaseJob(cfg.QueueCheckoutWindow), log)
	expiryRunner := worker.New("queue_admission_expiry", cfg.QueueReleaseInterval, queueSvc.AdmissionExpiryJob(500), log)

	go releaseRunner.Run(ctx)
	go expiryRunner.Run(ctx)

	runner := worker.New("expire_orders", cfg.WorkerInterval, svc.ExpireJob(100), log)
	log.Info("worker starting", "job", "expire_orders", "interval", cfg.WorkerInterval.String())
	runner.Run(ctx)
	log.Info("worker exited")
}
