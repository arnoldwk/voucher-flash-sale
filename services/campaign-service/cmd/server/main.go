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
	"voucher-flash-sale/services/campaign-service/internal/config"
	deliveryhttp "voucher-flash-sale/services/campaign-service/internal/delivery/http"
	"voucher-flash-sale/services/campaign-service/internal/repository"
	"voucher-flash-sale/services/campaign-service/internal/usecase"
)

func main() {
	log := logger.InitLogger("campaign-service")
	cfg := config.Load()

	// 1. Database Connection
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
	repo := repository.NewPostgresCampaignRepo(db, rdb)
	uc := usecase.NewCampaignUsecase(repo)
	handler := deliveryhttp.NewCampaignHandler(uc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 4. Graceful Shutdown
	go func() {
		log.Info("starting campaign-service", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("shutting down campaign-service gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("forced shutdown", "error", err)
	}
	log.Info("campaign-service stopped")
}
