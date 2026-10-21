package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"voucher-flash-sale/services/campaign-service/internal/domain"

	"github.com/redis/go-redis/v9"
)

type RedisCampaignCache struct {
	rdb *redis.Client
}

func NewRedisCampaignCache(rdb *redis.Client) *RedisCampaignCache {
	return &RedisCampaignCache{rdb: rdb}
}

// Warmup menyimpan counter stock awal beserta TTL-nya.
func (c *RedisCampaignCache) Warmup(ctx context.Context, campaignID string, stock int, ttl time.Duration) error {
	key := fmt.Sprintf("campaign:stock:%s", campaignID)
	return c.rdb.Set(ctx, key, stock, ttl).Err()
}

// Stock mengembalikan stock terkini. Mengembalikan -1 bila belum ada di cache.
func (c *RedisCampaignCache) Stock(ctx context.Context, campaignID string) (int, error) {
	val, err := c.rdb.Get(ctx, fmt.Sprintf("campaign:stock:%s", campaignID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return -1, nil
		}
		return 0, err
	}
	return strconv.Atoi(val)
}

// PublishMeta menyimpan metadata campaign untuk validasi klaim secara atomik.
func (c *RedisCampaignCache) PublishMeta(ctx context.Context, campaignID string, meta domain.CampaignMeta) error {
	key := fmt.Sprintf("campaign:meta:%s", campaignID)
	return c.rdb.HSet(ctx, key, map[string]any{
		"category_id":     meta.CategoryID,
		"discount_amount": fmt.Sprintf("%.2f", meta.DiscountAmount),
		"start_time":      meta.StartTime.Unix(),
		"end_time":        meta.EndTime.Unix(),
	}).Err()
}

// Stocks mengambil stock banyak campaign dalam satu round-trip (MGET).
func (c *RedisCampaignCache) Stocks(ctx context.Context, campaignIDs []string) (map[string]int, error) {
	if len(campaignIDs) == 0 {
		return nil, nil
	}

	keys := make([]string, len(campaignIDs))
	for i, id := range campaignIDs {
		keys[i] = fmt.Sprintf("campaign:stock:%s", id)
	}

	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]int, len(campaignIDs))
	for i, v := range vals {
		if v == nil {
			result[campaignIDs[i]] = -1
			continue
		}
		s, convErr := strconv.Atoi(fmt.Sprint(v))
		if convErr != nil {
			result[campaignIDs[i]] = -1
			continue
		}
		result[campaignIDs[i]] = s
	}
	return result, nil
}
