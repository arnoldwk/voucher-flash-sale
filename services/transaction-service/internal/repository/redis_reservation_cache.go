package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// reserveOrderLuaScript atomically reserves one unit of stock for a
// customer in a session and writes the reservation + entitlement markers.
//
// Return codes:
//
//	 1  -> reservation granted
//	-2  -> out of stock (stock key missing or <= 0)
//	-3  -> customer already entitled for this category in the session
//
// KEYS:
//  1. campaign:stock:{campaign_id}
//  2. entitlement:cust:{customer_id}:session:{session_id}:cat:{category}
//
// ARGV:
//  1. reservation_key   (reservation:order:{order_id})
//  2. order_id
//  3. ttl_seconds       (reservation ttl)
//  4. order_json
const reserveOrderLuaScript = `
-- KEYS[1] = campaign:stock:{campaign_id}
-- KEYS[2] = entitlement:cust:{customer_id}:session:{session_id}:cat:{category}
-- ARGV[1] = reservation_key
-- ARGV[2] = order_id
-- ARGV[3] = ttl_seconds
-- ARGV[4] = order_json

local stockRaw = redis.call('GET', KEYS[1])
if stockRaw == false then
    return -2
end
local stock = tonumber(stockRaw)
if stock <= 0 then
    return -2
end

local ent = redis.call('GET', KEYS[2])
if ent ~= false then
    return -3
end

redis.call('DECR', KEYS[1])
redis.call('SET', ARGV[1], ARGV[4], 'EX', ARGV[3])
redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[3])
return 1
`

// releaseReservationLuaScript rolls back a reservation by restoring stock
// and deleting the reservation and entitlement markers.
const releaseReservationLuaScript = `
-- KEYS[1] = campaign:stock:{campaign_id}
-- KEYS[2] = reservation:order:{order_id}
-- KEYS[3] = entitlement:cust:{customer_id}:session:{session_id}:cat:{category}
redis.call('INCR', KEYS[1])
redis.call('DEL', KEYS[2])
redis.call('DEL', KEYS[3])
return 1
`

type RedisReservationCache struct {
	rdb *redis.Client
}

func NewRedisReservationCache(rdb *redis.Client) *RedisReservationCache {
	return &RedisReservationCache{rdb: rdb}
}

// Decrement decrements campaign stock by one. Returns the new stock value.
// Returns -1 when the key is absent.
func (c *RedisReservationCache) Decrement(ctx context.Context, campaignID string) (int, error) {
	val, err := c.rdb.Decr(ctx, fmt.Sprintf("campaign:stock:%s", campaignID)).Result()
	if err != nil {
		return 0, err
	}
	return int(val), nil
}

// Increment increments campaign stock by one (rollback). Returns new stock.
func (c *RedisReservationCache) Increment(ctx context.Context, campaignID string) (int, error) {
	val, err := c.rdb.Incr(ctx, fmt.Sprintf("campaign:stock:%s", campaignID)).Result()
	if err != nil {
		return 0, err
	}
	return int(val), nil
}

// Get returns the current stock counter or -1 if absent.
func (c *RedisReservationCache) Get(ctx context.Context, campaignID string) (int, error) {
	val, err := c.rdb.Get(ctx, fmt.Sprintf("campaign:stock:%s", campaignID)).Result()
	if err != nil {
		if err == redis.Nil {
			return -1, nil
		}
		return 0, err
	}
	i, err := strconvAtoi(val)
	if err != nil {
		return 0, err
	}
	return i, nil
}

// Reserve attempts an atomic reservation via Lua. Returns the Lua return code.
func (c *RedisReservationCache) Reserve(ctx context.Context, stockKey, entitlementKey, reservationKey, orderID, orderJSON string, ttl time.Duration) (int, error) {
	res, err := c.rdb.Eval(ctx, reserveOrderLuaScript, []string{stockKey, entitlementKey}, reservationKey, orderID, int(ttl.Seconds()), orderJSON).Result()
	if err != nil {
		return 0, err
	}
	switch v := res.(type) {
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected lua return type: %T", v)
	}
}

// SetReservationMarker writes the reservation marker with a TTL.
func (c *RedisReservationCache) SetReservationMarker(ctx context.Context, reservationKey string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, reservationKey, value, ttl).Err()
}

// DelReservationMarker removes a reservation marker.
func (c *RedisReservationCache) DelReservationMarker(ctx context.Context, reservationKey string) error {
	return c.rdb.Del(ctx, reservationKey).Err()
}

// SetEntitlementMarker writes the entitlement marker with a TTL.
func (c *RedisReservationCache) SetEntitlementMarker(ctx context.Context, entitlementKey string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, entitlementKey, value, ttl).Err()
}

// DelEntitlementMarker removes an entitlement marker.
func (c *RedisReservationCache) DelEntitlementMarker(ctx context.Context, entitlementKey string) error {
	return c.rdb.Del(ctx, entitlementKey).Err()
}

// ReleaseReservation rolls back stock, reservation marker, and entitlement marker.
func (c *RedisReservationCache) ReleaseReservation(ctx context.Context, stockKey, reservationKey, entitlementKey string) (int, error) {
	res, err := c.rdb.Eval(ctx, releaseReservationLuaScript, []string{stockKey, reservationKey, entitlementKey}).Result()
	if err != nil {
		return 0, err
	}
	switch v := res.(type) {
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected lua return type: %T", v)
	}
}

// strconvAtoi is a tiny local helper so we do not import strconv only for one call.
func strconvAtoi(s string) (int, error) {
	var n int
	neg := false
	for i, ch := range s {
		if i == 0 && ch == '-' {
			neg = true
			continue
		}
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid integer: %q", s)
		}
		n = n*10 + int(ch-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}
