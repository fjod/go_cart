package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fjod/go_cart/cart-service/internal/cache"
	"github.com/fjod/go_cart/cart-service/internal/domain"
	"github.com/fjod/go_cart/cart-service/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	m    sync.RWMutex
	cart *domain.Cart
	err  error
}

func (m *mockRepository) GetCart(context.Context, string) (*domain.Cart, error) {
	m.m.RLock()
	defer m.m.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.cart, nil
}

func (m *mockRepository) UpsertCart(_ context.Context, c *domain.Cart) error {
	m.m.Lock()
	defer m.m.Unlock()
	m.cart = c
	return m.err
}

func (m *mockRepository) AddItem(_ context.Context, _ string, item domain.CartItem) error {
	m.m.Lock()
	defer m.m.Unlock()
	if m.err != nil {
		return m.err
	}
	m.cart.Items = append(m.cart.Items, item)
	return nil
}

func (m *mockRepository) UpdateItemQuantity(_ context.Context, _ string, productID int64, quantity int) error {
	m.m.Lock()
	defer m.m.Unlock()
	if m.err != nil {
		return m.err
	}
	// Find and update the item
	for i := range m.cart.Items {
		if m.cart.Items[i].ProductID == productID {
			m.cart.Items[i].Quantity = quantity
			return nil
		}
	}
	return fmt.Errorf("item not found")
}

func (m *mockRepository) RemoveItem(_ context.Context, _ string, productID int64) error {
	m.m.Lock()
	defer m.m.Unlock()
	if m.err != nil {
		return m.err
	}
	// Find and remove the item
	for i, item := range m.cart.Items {
		if item.ProductID == productID {
			m.cart.Items = append(m.cart.Items[:i], m.cart.Items[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("item not found")
}

func (m *mockRepository) DeleteCart(_ context.Context, _ string) error {
	m.m.Lock()
	defer m.m.Unlock()
	if m.err != nil {
		return m.err
	}
	// Clear all items
	m.cart.Items = []domain.CartItem{}
	return nil
}

type mockCache struct {
	m    sync.RWMutex
	cart *domain.Cart
	err  error
}

func (m *mockCache) Get(context.Context, string) (*domain.Cart, error) {
	m.m.RLock()
	defer m.m.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.cart == nil {
		return nil, cache.ErrCacheMiss
	}
	return m.cart, nil
}

func (m *mockCache) Set(_ context.Context, _ string, cart *domain.Cart) error {
	m.m.Lock()
	defer m.m.Unlock()
	m.cart = cart
	return m.err
}

func (m *mockCache) Delete(context.Context, string) error {
	m.m.Lock()
	defer m.m.Unlock()
	m.cart = nil
	return m.err
}

func (m *mockCache) getCart() *domain.Cart {
	m.m.RLock()
	defer m.m.RUnlock()
	return m.cart
}

func TestGetCart_Success(t *testing.T) {
	cart := &domain.Cart{
		Items: []domain.CartItem{
			{ProductID: 1, Quantity: 5},
			{ProductID: 2, Quantity: 10},
		},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo := &mockRepository{
		cart: cart,
	}
	mockC := &mockCache{
		cart: nil,
	}

	sut := NewCartService(mockRepo, mockC)
	ret, err := sut.GetCart(context.Background(), "123")
	require.NoError(t, err)
	assert.NotNil(t, ret)
	t.Logf("Received cart response: %v", ret)
	l := len(ret.Items)
	assert.Equal(t, 2, l)
	assert.Equal(t, int64(1), ret.Items[0].ProductID)
	assert.Equal(t, 5, ret.Items[0].Quantity)
	assert.Equal(t, int64(2), ret.Items[1].ProductID)
	assert.Equal(t, 10, ret.Items[1].Quantity)

	require.Eventually(t, func() bool {
		return mockC.getCart() != nil
	}, 100*time.Millisecond, 10*time.Millisecond, "cart was not set in cache")
}

func TestGetCart_RepoError(t *testing.T) {

	mockRepo := &mockRepository{
		err: fmt.Errorf("database error"),
	}
	mockC := &mockCache{
		cart: nil,
	}

	sut := NewCartService(mockRepo, mockC)
	ret, err := sut.GetCart(context.Background(), "123")
	require.ErrorContains(t, err, "database error")
	assert.Nil(t, ret)
	assert.Nil(t, mockC.cart)
}

func TestGetCart_CacheHit(t *testing.T) {
	cart := &domain.Cart{
		Items:     []domain.CartItem{{ProductID: 1, Quantity: 3}},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo := &mockRepository{
		cart: nil, // repo should NOT be called
	}
	mockC := &mockCache{
		cart: cart, // cache has the cart
	}

	sut := NewCartService(mockRepo, mockC)
	ret, err := sut.GetCart(context.Background(), "123")
	require.NoError(t, err)
	assert.Equal(t, 1, len(ret.Items))
	assert.Equal(t, int64(1), ret.Items[0].ProductID)
}

func TestGetCart_CartNotFound_ReturnsEmptyCart(t *testing.T) {
	mockRepo := &mockRepository{
		err: repository.ErrCartNotFound,
	}
	mockC := &mockCache{
		cart: nil,
	}

	sut := NewCartService(mockRepo, mockC)
	ret, err := sut.GetCart(context.Background(), "123")
	require.NoError(t, err)
	assert.NotNil(t, ret)
	assert.Equal(t, "123", ret.UserID)
	assert.Empty(t, ret.Items)
}

func TestAddItem_Success(t *testing.T) {
	cart := &domain.Cart{
		Items:     []domain.CartItem{},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo := &mockRepository{cart: cart}
	mockC := &mockCache{cart: cart}

	sut := NewCartService(mockRepo, mockC)
	err := sut.AddItem(context.Background(), "123", domain.CartItem{
		ProductID: 1,
		Quantity:  5,
		AddedAt:   time.Now(),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, len(mockRepo.cart.Items))
	assert.Equal(t, int64(1), mockRepo.cart.Items[0].ProductID)
	assert.Equal(t, 5, mockRepo.cart.Items[0].Quantity)

	// Verify cache was invalidated
	require.Eventually(t, func() bool {
		return mockC.getCart() == nil
	}, 100*time.Millisecond, 10*time.Millisecond, "cache was not invalidated")
}

func TestAddItem_RepoError(t *testing.T) {
	mockRepo := &mockRepository{
		cart: &domain.Cart{Items: []domain.CartItem{}},
		err:  fmt.Errorf("database error"),
	}
	mockC := &mockCache{cart: nil}

	sut := NewCartService(mockRepo, mockC)
	err := sut.AddItem(context.Background(), "123", domain.CartItem{ProductID: 1, Quantity: 5})
	require.ErrorContains(t, err, "database error")
}

func TestUpdateQuantity_Success(t *testing.T) {
	cart := &domain.Cart{
		Items: []domain.CartItem{
			{ProductID: 1, Quantity: 5},
			{ProductID: 2, Quantity: 10},
		},
		UserID: "123",
	}
	mockRepo := &mockRepository{cart: cart}
	mockC := &mockCache{cart: cart}

	sut := NewCartService(mockRepo, mockC)
	err := sut.UpdateQuantity(context.Background(), "123", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 20, mockRepo.cart.Items[0].Quantity)

	// Verify cache was invalidated
	require.Eventually(t, func() bool {
		return mockC.getCart() == nil
	}, 100*time.Millisecond, 10*time.Millisecond, "cache was not invalidated")
}

func TestUpdateQuantity_RepoError(t *testing.T) {
	mockRepo := &mockRepository{
		cart: &domain.Cart{Items: []domain.CartItem{{ProductID: 1, Quantity: 5}}},
		err:  fmt.Errorf("database error"),
	}
	mockC := &mockCache{}

	sut := NewCartService(mockRepo, mockC)
	err := sut.UpdateQuantity(context.Background(), "123", 1, 20)
	require.ErrorContains(t, err, "database error")
}

func TestRemoveItem_Success(t *testing.T) {
	cart := &domain.Cart{
		Items: []domain.CartItem{
			{ProductID: 1, Quantity: 5},
			{ProductID: 2, Quantity: 10},
		},
		UserID: "123",
	}
	mockRepo := &mockRepository{cart: cart}
	mockC := &mockCache{cart: cart}

	sut := NewCartService(mockRepo, mockC)
	err := sut.RemoveItem(context.Background(), "123", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(mockRepo.cart.Items))
	assert.Equal(t, int64(2), mockRepo.cart.Items[0].ProductID)

	// Verify cache was invalidated
	require.Eventually(t, func() bool {
		return mockC.getCart() == nil
	}, 100*time.Millisecond, 10*time.Millisecond, "cache was not invalidated")
}

func TestRemoveItem_RepoError(t *testing.T) {
	mockRepo := &mockRepository{
		cart: &domain.Cart{Items: []domain.CartItem{{ProductID: 1, Quantity: 5}}},
		err:  fmt.Errorf("database error"),
	}
	mockC := &mockCache{}

	sut := NewCartService(mockRepo, mockC)
	err := sut.RemoveItem(context.Background(), "123", 1)
	require.ErrorContains(t, err, "database error")
}

func TestClearCart_Success(t *testing.T) {
	cart := &domain.Cart{
		Items: []domain.CartItem{
			{ProductID: 1, Quantity: 5},
			{ProductID: 2, Quantity: 10},
		},
		UserID: "123",
	}
	mockRepo := &mockRepository{cart: cart}
	mockC := &mockCache{cart: cart}

	sut := NewCartService(mockRepo, mockC)
	err := sut.ClearCart(context.Background(), "123")
	require.NoError(t, err)
	assert.Empty(t, mockRepo.cart.Items)

	// Verify cache was invalidated
	require.Eventually(t, func() bool {
		return mockC.getCart() == nil
	}, 100*time.Millisecond, 10*time.Millisecond, "cache was not invalidated")
}

func TestClearCart_RepoError(t *testing.T) {
	mockRepo := &mockRepository{
		cart: &domain.Cart{Items: []domain.CartItem{{ProductID: 1, Quantity: 5}}},
		err:  fmt.Errorf("database error"),
	}
	mockC := &mockCache{}

	sut := NewCartService(mockRepo, mockC)
	err := sut.ClearCart(context.Background(), "123")
	require.ErrorContains(t, err, "database error")
}
