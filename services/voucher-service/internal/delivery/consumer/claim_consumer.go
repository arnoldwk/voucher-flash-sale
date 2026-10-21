package consumer

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"voucher-flash-sale/pkg/events"
	"voucher-flash-sale/services/voucher-service/internal/usecase"

	"github.com/redis/go-redis/v9"
)

type ClaimConsumer struct {
	rdb          *redis.Client
	usecase      *usecase.VoucherUsecase
	consumerName string
}

func NewClaimConsumer(rdb *redis.Client, u *usecase.VoucherUsecase, consumerName string) *ClaimConsumer {
	return &ClaimConsumer{
		rdb:          rdb,
		usecase:      u,
		consumerName: consumerName,
	}
}

// Start listens to Redis Stream and persists voucher claims into PostgreSQL.
func (c *ClaimConsumer) Start(ctx context.Context) {
	// 1. Ensure stream and consumer group exist
	err := c.rdb.XGroupCreateMkStream(ctx, events.ClaimStreamName, events.ClaimGroupName, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		slog.Warn("XGroupCreateMkStream notice", "error", err)
	}

	slog.Info("claim stream consumer started", "stream", events.ClaimStreamName, "group", events.ClaimGroupName)

	for {
		select {
		case <-ctx.Done():
			slog.Info("claim stream consumer stopping...")
			return
		default:
			streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    events.ClaimGroupName,
				Consumer: c.consumerName,
				Streams:  []string{events.ClaimStreamName, ">"},
				Count:    20,
				Block:    2 * time.Second,
			}).Result()

			if err != nil {
				if err != redis.Nil && !strings.Contains(err.Error(), "context canceled") {
					slog.Error("error reading from claim stream", "error", err)
					time.Sleep(500 * time.Millisecond)
				}
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					ev := c.parseMessage(msg.Values)
					if err := c.usecase.ProcessClaimEvent(ctx, ev); err != nil {
						slog.Error("failed to process claim event", "voucher_id", ev.VoucherID, "error", err)
						continue
					}

					// Acknowledge processed message
					_ = c.rdb.XAck(ctx, events.ClaimStreamName, events.ClaimGroupName, msg.ID).Err()
				}
			}
		}
	}
}

func (c *ClaimConsumer) parseMessage(values map[string]any) events.VoucherClaimedEvent {
	getString := func(key string) string {
		if val, ok := values[key]; ok {
			if s, ok := val.(string); ok {
				return s
			}
		}
		return ""
	}

	discount, _ := strconv.ParseFloat(getString("discount_amount"), 64)
	claimedAt, _ := time.Parse(time.RFC3339Nano, getString("claimed_at"))

	return events.VoucherClaimedEvent{
		VoucherID:      getString("voucher_id"),
		CampaignID:     getString("campaign_id"),
		UserID:         getString("user_id"),
		VoucherCode:    getString("voucher_code"),
		DiscountAmount: discount,
		ClaimedAt:      claimedAt,
	}
}
