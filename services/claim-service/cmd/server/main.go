package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voucher-flash-sale/pkg/logger"
	pkgredis "voucher-flash-sale/pkg/redis"
	"voucher-flash-sale/services/claim-service/internal/config"
	deliveryhttp "voucher-flash-sale/services/claim-service/internal/delivery/http"
	"voucher-flash-sale/services/claim-service/internal/repository"
	"voucher-flash-sale/services/claim-service/internal/usecase"
)

func main() {
	log := logger.InitLogger("claim-service")
	cfg := config.Load()

	// 1. Redis Connection (Ultra-low latency memory layer)
	rdb, err := pkgredis.ConnectRedis(pkgredis.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
	})
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// 2. Clean Architecture Wiring
	repo := repository.NewRedisClaimRepo(rdb)
	uc := usecase.NewClaimUsecase(repo)
	handler := deliveryhttp.NewClaimHandler(uc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// 3. Graceful Server Lifecycle
	go func() {
		log.Info("starting claim-service (high-throughput engine)", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("shutting down claim-service gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("forced shutdown", "error", err)
	}
	log.Info("claim-service stopped")
}
