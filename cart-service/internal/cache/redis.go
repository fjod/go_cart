package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/fjod/go_cart/cart-service/internal/domain"
	"github.com/redis/go-redis/v9"
)

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{
		client:  client,
		baseTTL: 15 * time.Minute,
	}
}

type RedisCache struct {
	client  *redis.Client
	baseTTL time.Duration
}

func (r RedisCache) Get(ctx context.Context, userID string) (*domain.Cart, error) {
	key := cacheKey(userID)

	data, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	var cart domain.Cart
	if err2 := json.Unmarshal(data, &cart); err2 != nil {
		return nil, fmt.Errorf("unmarshal cart failed: %w", err2)
	}

	return &cart, nil
}

func (r RedisCache) Set(ctx context.Context, userID string, cart *domain.Cart) error {
	key := cacheKey(userID)
	jsonCart, err := json.Marshal(cart)
	if err != nil {
		return fmt.Errorf("marshal cart failed: %w", err)
	}

	jitter := time.Duration(rand.Intn(5)) * time.Minute
	ttl := r.baseTTL + jitter
	ret := r.client.Set(ctx, key, string(jsonCart), ttl)
	if ret.Err() != nil {
		return fmt.Errorf("redis set failed: %w", err)
	}
	return nil
}

func (r RedisCache) Delete(ctx context.Context, userID string) error {
	key := cacheKey(userID)
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete failed: %w", err)
	}

	return nil
}

func cacheKey(userID string) string {
	return fmt.Sprintf("cart:%s", userID)
}
