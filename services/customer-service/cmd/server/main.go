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
	"voucher-flash-sale/services/customer-service/internal/config"
	deliveryhttp "voucher-flash-sale/services/customer-service/internal/delivery/http"
	"voucher-flash-sale/services/customer-service/internal/repository"
	"voucher-flash-sale/services/customer-service/internal/usecase"
)

func main() {
	log := logger.InitLogger("customer-service")
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

	// 2. Clean Architecture Wiring
	repo := repository.NewPostgresCustomerRepo(db)
	uc := usecase.NewCustomerUsecase(repo)
	handler := deliveryhttp.NewCustomerHandler(uc, cfg.InternalAPIKey)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 3. Start Server
	go func() {
		log.Info("starting customer-service", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 4. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("shutting down customer-service gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("forced shutdown", "error", err)
	}
	log.Info("customer-service stopped")
}
