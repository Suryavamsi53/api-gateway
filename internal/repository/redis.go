package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client *redis.Client
}

// NewRedisStore connects to Redis and returns a Store implementation.
func NewRedisStore(addr string) (Store, error) {
	opt := &redis.Options{
		Addr: addr,
	}
	client := redis.NewClient(opt)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &redisStore{client: client}, nil
}

// tokenBucketLua implements refill + take atomically.
var tokenBucketLua = redis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local data = redis.call('HMGET', key, 'tokens', 'last')
local tokens = tonumber(data[1]) or capacity
local last = tonumber(data[2]) or now

local delta = math.max(0, now - last)
local refill = delta * rate
tokens = math.min(capacity, tokens + refill)
local allowed = 0
if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
end
redis.call('HMSET', key, 'tokens', tokens, 'last', now)
redis.call('PEXPIRE', key, math.ceil((capacity / rate) * 1000 * 2))
return {allowed, tokens}
`)

func (r *redisStore) TokenBucket(ctx context.Context, key string, capacity int64, refillRate float64, tokens int64) (bool, int64, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	res, err := tokenBucketLua.Run(ctx, r.client, []string{key}, capacity, refillRate/1000.0, now, tokens).Result()
	if err != nil {
		return false, 0, err
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 2 {
		return false, 0, fmt.Errorf("unexpected redis response: %v", res)
	}
	allowed := arr[0].(int64) == 1
	remaining := int64(0)
	switch v := arr[1].(type) {
	case int64:
		remaining = v
	case string:
		// redis may return string
		var parsed int64
		fmt.Sscanf(v, "%d", &parsed)
		remaining = parsed
	}
	return allowed, remaining, nil
}

func (r *redisStore) SlidingWindow(ctx context.Context, key string, windowMillis int64) (int64, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	zkey := key + ":sw"
	pipe := r.client.TxPipeline()
	pipe.ZAdd(ctx, zkey, redis.Z{Score: float64(now), Member: now})
	pipe.ZRemRangeByScore(ctx, zkey, "-inf", fmt.Sprintf("%d", now-windowMillis))
	cnt := pipe.ZCard(ctx, zkey)
	pipe.PExpire(ctx, zkey, time.Duration(windowMillis*2)*time.Millisecond)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return cnt.Val(), nil
}
