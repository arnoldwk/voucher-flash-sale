package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"voucher-flash-sale/pkg/events"
	"voucher-flash-sale/services/claim-service/internal/domain"

	"github.com/redis/go-redis/v9"
)

const atomicClaimLuaScript = `
-- KEYS[1] = campaign:stock:{campaign_id}
-- KEYS[2] = campaign:claimed_users:{campaign_id}
-- KEYS[3] = stream:voucher_claims
-- ARGV[1] = campaign_id
-- ARGV[2] = user_id
-- ARGV[3] = voucher_id
-- ARGV[4] = voucher_code
-- ARGV[5] = discount_amount
-- ARGV[6] = claimed_at

-- 1. Check if user already claimed
if redis.call('SISMEMBER', KEYS[2], ARGV[2]) == 1 then
    return -1
end

-- 2. Check if stock key exists
local stockRaw = redis.call('GET', KEYS[1])
if not stockRaw then
    return -3
end

local stock = tonumber(stockRaw)
if stock <= 0 then
    return -2
end

-- 3. Atomic stock deduction
redis.call('DECR', KEYS[1])

-- 4. Mark user as claimed
redis.call('SADD', KEYS[2], ARGV[2])

-- 5. Publish to Redis Stream for async worker persistence
redis.call('XADD', KEYS[3], '*',
    'voucher_id', ARGV[3],
    'campaign_id', ARGV[1],
    'user_id', ARGV[2],
    'voucher_code', ARGV[4],
    'discount_amount', ARGV[5],
    'claimed_at', ARGV[6]
)

return 1
`

type RedisClaimRepo struct {
	rdb *redis.Client
}

func NewRedisClaimRepo(rdb *redis.Client) *RedisClaimRepo {
	return &RedisClaimRepo{
		rdb: rdb,
	}
}

func (r *RedisClaimRepo) ExecuteAtomicClaim(
	ctx context.Context,
	campaignID, userID, voucherID, voucherCode string,
	discountAmount float64,
	claimedAt time.Time,
) error {
	stockKey := fmt.Sprintf("campaign:stock:%s", campaignID)
	claimedUsersKey := fmt.Sprintf("campaign:claimed_users:%s", campaignID)
	streamKey := events.ClaimStreamName

	res, err := r.rdb.Eval(
		ctx,
		atomicClaimLuaScript,
		[]string{stockKey, claimedUsersKey, streamKey},
		campaignID,
		userID,
		voucherID,
		voucherCode,
		fmt.Sprintf("%.2f", discountAmount),
		claimedAt.Format(time.RFC3339Nano),
	).Result()

	if err != nil {
		return fmt.Errorf("redis eval lua failed: %w", err)
	}

	code, ok := res.(int64)
	if !ok {
		return fmt.Errorf("unexpected lua return type: %T", res)
	}

	switch code {
	case 1:
		return nil
	case -1:
		return domain.ErrAlreadyClaimed
	case -2:
		return domain.ErrOutOfStock
	case -3:
		return domain.ErrCampaignNotActive
	default:
		return fmt.Errorf("unknown return code: %d", code)
	}
}

func (r *RedisClaimRepo) GetRemainingStock(ctx context.Context, campaignID string) (int, error) {
	stockKey := fmt.Sprintf("campaign:stock:%s", campaignID)
	val, err := r.rdb.Get(ctx, stockKey).Result()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(val)
}
