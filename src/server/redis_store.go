package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const defaultTTL = 7 * 24 * time.Hour // 7 days

// RedisStore implements the Store interface using Redis sorted sets.
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore creates a new Redis-backed store.
func NewRedisStore(addr string) *RedisStore {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisStore{client: client, ttl: defaultTTL}
}

func (r *RedisStore) heartbeatKey(nodeName string) string {
	return fmt.Sprintf("heartbeat:%s", nodeName)
}

func (r *RedisStore) latestKey(nodeName string) string {
	return fmt.Sprintf("heartbeat:latest:%s", nodeName)
}

func (r *RedisStore) Save(ctx context.Context, event Heartbeat) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal heartbeat: %w", err)
	}

	score := float64(event.Timestamp.UnixMilli())
	key := r.heartbeatKey(event.NodeName)

	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, key, &redis.Z{Score: score, Member: string(data)})
	pipe.Expire(ctx, key, r.ttl)

	// Store latest for fast anomaly lookups
	latestKey := r.latestKey(event.NodeName)
	pipe.Set(ctx, latestKey, string(data), r.ttl)

	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisStore) GetByTimeRange(ctx context.Context, from, to time.Time) ([]Heartbeat, error) {
	// Scan all heartbeat keys
	keys, err := r.scanKeys(ctx, "heartbeat:*")
	if err != nil {
		return nil, err
	}

	fromScore := float64(from.UnixMilli())
	toScore := float64(to.UnixMilli())
	var result []Heartbeat

	for _, key := range keys {
		// Skip latest keys
		if len(key) > 17 && key[:17] == "heartbeat:latest:" {
			continue
		}
		members, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
			Min: fmt.Sprintf("%f", fromScore),
			Max: fmt.Sprintf("%f", toScore),
		}).Result()
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			var hb Heartbeat
			if err := json.Unmarshal([]byte(m), &hb); err == nil {
				result = append(result, hb)
			}
		}
	}
	return result, nil
}

func (r *RedisStore) GetLatestByNode(ctx context.Context, nodeName string) (*Heartbeat, error) {
	data, err := r.client.Get(ctx, r.latestKey(nodeName)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var hb Heartbeat
	if err := json.Unmarshal([]byte(data), &hb); err != nil {
		return nil, err
	}
	return &hb, nil
}

func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisStore) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	var cursor uint64
	for {
		batch, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

func (r *RedisStore) kernelEventKey(nodeName string) string {
	return fmt.Sprintf("kernel_event:%s", nodeName)
}

func (r *RedisStore) causalChainKey(nodeName string) string {
	return fmt.Sprintf("causal_chain:%s", nodeName)
}

func (r *RedisStore) SaveKernelEvent(ctx context.Context, event EnrichedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal kernel event: %w", err)
	}
	score := float64(event.Timestamp.UnixMilli())
	key := r.kernelEventKey(event.NodeName)

	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, key, &redis.Z{Score: score, Member: string(data)})
	pipe.Expire(ctx, key, r.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisStore) GetKernelEvents(ctx context.Context, nodeName string, from, to time.Time) ([]EnrichedEvent, error) {
	key := r.kernelEventKey(nodeName)
	members, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", float64(from.UnixMilli())),
		Max: fmt.Sprintf("%f", float64(to.UnixMilli())),
	}).Result()
	if err != nil {
		return nil, err
	}
	var result []EnrichedEvent
	for _, m := range members {
		var e EnrichedEvent
		if err := json.Unmarshal([]byte(m), &e); err == nil {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *RedisStore) GetKernelEventsByType(ctx context.Context, nodeName string, eventType string, from, to time.Time) ([]EnrichedEvent, error) {
	events, err := r.GetKernelEvents(ctx, nodeName, from, to)
	if err != nil {
		return nil, err
	}
	var result []EnrichedEvent
	for _, e := range events {
		if e.EventType == eventType {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *RedisStore) SaveCausalChain(ctx context.Context, chain CausalChain) error {
	data, err := json.Marshal(chain)
	if err != nil {
		return fmt.Errorf("marshal causal chain: %w", err)
	}
	score := float64(chain.Timestamp.UnixMilli())
	key := r.causalChainKey(chain.NodeName)

	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, key, &redis.Z{Score: score, Member: string(data)})
	pipe.Expire(ctx, key, r.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisStore) GetCausalChains(ctx context.Context, nodeName string, from, to time.Time) ([]CausalChain, error) {
	key := r.causalChainKey(nodeName)
	members, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", float64(from.UnixMilli())),
		Max: fmt.Sprintf("%f", float64(to.UnixMilli())),
	}).Result()
	if err != nil {
		return nil, err
	}
	var result []CausalChain
	for _, m := range members {
		var c CausalChain
		if err := json.Unmarshal([]byte(m), &c); err == nil {
			result = append(result, c)
		}
	}
	return result, nil
}
