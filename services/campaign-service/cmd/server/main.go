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
	pkgmiddleware "voucher-flash-sale/pkg/middleware"
	pkgredis "voucher-flash-sale/pkg/redis"
	"voucher-flash-sale/services/campaign-service/internal/config"
	deliveryhttp "voucher-flash-sale/services/campaign-service/internal/delivery/http"
	"voucher-flash-sale/services/campaign-service/internal/domain"
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
	pgRepo := repository.NewPostgresCampaignRepo(db)
	cache := repository.NewRedisCampaignCache(rdb)
	campaignUC := usecase.NewCampaignUsecase(pgRepo, cache)
	campaignHandler := deliveryhttp.NewCampaignHandler(campaignUC)

	// Voucher product catalog wiring (API baru /product/v1/)
	catalogRepo := repository.NewPostgresProductCatalogRepo(db)
	catalogCache := repository.NewRedisCampaignCache(rdb)
	listVouchersUC := usecase.NewListVouchersUsecase(catalogRepo, catalogCache)
	catalogHandler := deliveryhttp.NewVoucherCatalogHandler(listVouchersUC)

	// 3b. Warmup Redis stock counters for seeded campaigns.
	// Warmup menggunakan remaining_stock (bukan session_stock) sesuai kontrak.
	warmupCtx, warmupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := warmupCacheFromDB(warmupCtx, pgRepo, cache); err != nil {
		log.Warn("failed to warmup stock cache", "error", err)
	}
	warmupCancel()

	mux := http.NewServeMux()

	// Internal admin API (campaign CRUD)
	campaignHandler.RegisterRoutes(mux)

	// Public API v1 — membutuhkan JWT
	voucherCatalogMux := http.NewServeMux()
	catalogHandler.RegisterRoutes(voucherCatalogMux)
	mux.Handle("/product/v1/", http.StripPrefix("/product/v1",
		pkgmiddleware.AuthBearer(voucherCatalogMux)))

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

// warmupCacheFromDB mengisi counter stock Redis dari snapshot Postgres.
// Nilai yang dip patterning adalah remaining_stock (sudah dikurangi klaim),
// bukan session_stock, agar claim-service tidak bisa melebihi stok真正 tersisa.
func warmupCacheFromDB(ctx context.Context, repo domain.CampaignRepository, cache domain.StockCache) error {
	campaigns, err := repo.List(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, c := range campaigns {
		if c.Status != "ACTIVE" && c.Status != "SCHEDULED" {
			continue
		}
		if !c.EndTime.After(now) {
			continue
		}

		ttl := time.Until(c.EndTime) + time.Hour
		if ttl < time.Hour {
			ttl = time.Hour
		}

		if err := cache.Warmup(ctx, c.ID, c.RemainingStock, ttl); err != nil {
			return err
		}
	}
	return nil
}
