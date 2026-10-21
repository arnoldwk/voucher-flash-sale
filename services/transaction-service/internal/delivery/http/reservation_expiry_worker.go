package http

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	domain "voucher-flash-sale/services/transaction-service/internal/domain"
	"voucher-flash-sale/services/transaction-service/internal/repository"
)

// ReservationExpiryWorker periodically releases expired PENDING orders,
// freeing their quota slots so other customers can reserve.
type ReservationExpiryWorker struct {
	repo     *repository.PostgresOrderRepo
	cache    domain.StockCache
	interval time.Duration
	logger   *slog.Logger
	running  bool
	mu       sync.Mutex
}

func NewReservationExpiryWorker(repo *repository.PostgresOrderRepo, cache domain.StockCache, interval time.Duration, logger *slog.Logger) *ReservationExpiryWorker {
	return &ReservationExpiryWorker{
		repo:     repo,
		cache:    cache,
		interval: interval,
		logger:   logger,
	}
}

func (w *ReservationExpiryWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.run(ctx)
}

func (w *ReservationExpiryWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.expireNow(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.expireNow(ctx)
		}
	}
}

func (w *ReservationExpiryWorker) expireNow(ctx context.Context) {
	expired, err := w.repo.ExpirePendingOrders(ctx)
	if err != nil {
		w.logger.Error("expiry worker failed", "error", err)
		return
	}

	for _, orderID := range expired {
		if err := w.cache.DelReservationMarker(ctx, fmt.Sprintf("reservation:order:%s", orderID)); err != nil {
			w.logger.Warn("failed to delete reservation marker", "order", orderID, "error", err)
		}
	}
	if len(expired) > 0 {
		w.logger.Info("expired reservations released", "count", len(expired))
	}
}

// Compile-time interface check.
var _ = (*ReservationExpiryWorker)(nil)
