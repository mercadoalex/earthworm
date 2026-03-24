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
