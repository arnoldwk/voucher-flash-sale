package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voucher-flash-sale/pkg/database"
	"voucher-flash-sale/pkg/logger"
	pkgredis "voucher-flash-sale/pkg/redis"
	"voucher-flash-sale/services/voucher-service/internal/config"
	"voucher-flash-sale/services/voucher-service/internal/delivery/consumer"
	deliveryhttp "voucher-flash-sale/services/voucher-service/internal/delivery/http"
	"voucher-flash-sale/services/voucher-service/internal/repository"
	"voucher-flash-sale/services/voucher-service/internal/usecase"

	"github.com/google/uuid"
)

func main() {
	log := logger.InitLogger("voucher-service")
	cfg := config.Load()

	// 1. PostgreSQL Connection
	db, err := database.ConnectPostgres(database.Config{
		Host:     cfg.PostgresHost,
		Port:     cfg.PostgresPort,
		User:     cfg.PostgresUser,
		Password: cfg.PostgresPass,
		DBName:   cfg.PostgresDB,
		SSLMode:  cfg.PostgresSSL,
	})
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 2. Redis Connection
	rdb, err := pkgredis.ConnectRedis(pkgredis.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
	})
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// 3. Clean Architecture Wiring
	repo := repository.NewPostgresVoucherRepo(db)
	uc := usecase.NewVoucherUsecase(repo)
	handler := deliveryhttp.NewVoucherHandler(uc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 4. Start Background Stream Consumer Worker
	consumerName := "worker-" + uuid.New().String()[:8]
	claimConsumer := consumer.NewClaimConsumer(rdb, uc, consumerName)
	go claimConsumer.Start(ctx)

	// 5. Start HTTP Server
	go func() {
		log.Info("starting voucher-service", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 6. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("shutting down voucher-service gracefully...")
	cancel() // Stop consumer worker loop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", "error", err)
	}
	log.Info("voucher-service stopped")
}
