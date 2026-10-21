package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voucher-flash-sale/pkg/database"
	pkgmiddleware "voucher-flash-sale/pkg/middleware"
	pkgredis "voucher-flash-sale/pkg/redis"
	"voucher-flash-sale/services/transaction-service/internal/config"
	deliveryhttp "voucher-flash-sale/services/transaction-service/internal/delivery/http"
	"voucher-flash-sale/services/transaction-service/internal/repository"
	"voucher-flash-sale/services/transaction-service/internal/usecase"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("service", "transaction-service")
	cfg := config.Load()

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

	rdb, err := pkgredis.ConnectRedis(pkgredis.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
	})
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	orderRepo := repository.NewPostgresOrderRepo(db)
	stockCache := repository.NewRedisReservationCache(rdb)
	reserveUC := usecase.NewReserveOrderUsecase(orderRepo, stockCache)
	orderHandler := deliveryhttp.NewOrderHandler(reserveUC)

	expiryWorker := deliveryhttp.NewReservationExpiryWorker(orderRepo, stockCache, cfg.ExpiryWorkerInterval, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expiryWorker.Start(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	inner := http.NewServeMux()
	orderHandler.RegisterRoutes(inner)
	mux.Handle("/v1/", http.StripPrefix("/v1", pkgmiddleware.AuthBearer(inner)))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("starting transaction-service", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("shutting down transaction-service gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", "error", err)
	}
	log.Info("transaction-service stopped")
}
