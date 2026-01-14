package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/fjod/go_cart/cart-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRedis creates a miniredis server and returns a RedisCache instance
func setupTestRedis(t *testing.T) (*RedisCache, *miniredis.Miniredis, func()) {
	// Create an in-memory Redis server
	mr := miniredis.RunT(t)

	// Create Redis client pointing to miniredis
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create cache instance
	cache := NewRedisCache(client)

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return cache, mr, cleanup
}

func TestGet_Success(t *testing.T) {
	cache, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user123"

	// Prepare test data
	cart := &domain.Cart{
		UserID: userID,
		Items: []domain.CartItem{
			{ProductID: 1, Quantity: 2},
			{ProductID: 2, Quantity: 3},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Manually set data in miniredis
	cartJSON, _ := json.Marshal(cart)
	mr.Set(cacheKey(userID), string(cartJSON))

	// Test Get
	result, err := cache.Get(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, result.UserID)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(1), result.Items[0].ProductID)
}

func TestGet_CacheMiss(t *testing.T) {
	cache, _, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	result, err := cache.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrCacheMiss)
	assert.Nil(t, result)
}

func TestGet_InvalidJSON(t *testing.T) {
	cache, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user123"
	key := cacheKey(userID)

	cart := &domain.Cart{
		UserID: userID,
		Items: []domain.CartItem{
			{ProductID: 10, Quantity: 5},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	jsonCart, err := json.Marshal(cart)
	require.NoError(t, err)
	invalidCart := jsonCart[0:10]
	e2 := mr.Set(key, string(invalidCart))
	require.NoError(t, e2)

	_, cacheError := cache.Get(ctx, userID)
	require.ErrorContains(t, cacheError, "unmarshal cart failed")
}

func TestSet_Success(t *testing.T) {
	cache, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user456"

	cart := &domain.Cart{
		UserID: userID,
		Items: []domain.CartItem{
			{ProductID: 10, Quantity: 5},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Set cart in cache
	err := cache.Set(ctx, userID, cart)
	require.NoError(t, err)

	// Verify data was stored correctly in miniredis
	stored, e2 := mr.Get(cacheKey(userID))
	assert.NotEmpty(t, stored)
	require.NoError(t, e2)

	var storedCart domain.Cart
	err = json.Unmarshal([]byte(stored), &storedCart)
	require.NoError(t, err)
	assert.Equal(t, userID, storedCart.UserID)
	assert.Len(t, storedCart.Items, 1)
}

func TestSet_WithTTL(t *testing.T) {
	cache, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user789"

	cart := &domain.Cart{
		UserID: userID,
		Items:  []domain.CartItem{},
	}

	err := cache.Set(ctx, userID, cart)
	require.NoError(t, err)

	// Check that TTL was set (miniredis tracks TTL)
	ttl := mr.TTL(cacheKey(userID))
	assert.True(t, ttl > 15*time.Minute, "TTL should be at least base TTL")
	assert.True(t, ttl <= 20*time.Minute, "TTL should be base + max jitter")
}

func TestDelete_Success(t *testing.T) {
	cache, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user999"

	// Set some data first
	cart := &domain.Cart{UserID: userID}
	cartJSON, _ := json.Marshal(cart)
	mr.Set(cacheKey(userID), string(cartJSON))

	// Verify data exists
	assert.True(t, mr.Exists(cacheKey(userID)))

	// Delete
	err := cache.Delete(ctx, userID)
	require.NoError(t, err)

	// Verify data was deleted
	assert.False(t, mr.Exists(cacheKey(userID)))
}

func TestDelete_NonExistentKey(t *testing.T) {
	cache, _, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	// Deleting non-existent key should not error
	err := cache.Delete(ctx, "nonexistent")
	assert.NoError(t, err)
}

func TestCacheKey_Format(t *testing.T) {
	userID := "test123"
	key := cacheKey(userID)
	assert.Equal(t, "cart:test123", key)
}
