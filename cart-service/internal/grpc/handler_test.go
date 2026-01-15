package grpc

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fjod/go_cart/cart-service/internal/cache"
	"github.com/fjod/go_cart/cart-service/internal/domain"
	s "github.com/fjod/go_cart/cart-service/internal/service"
	pb "github.com/fjod/go_cart/cart-service/pkg/proto"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// mockProductServiceClient implements productpb.ProductServiceClient
type mockProductServiceClient struct {
	getProductResp *productpb.GetProductResponse
	getProductErr  error
}

func (m *mockProductServiceClient) GetProduct(context.Context, *productpb.GetProductRequest, ...grpc.CallOption) (*productpb.GetProductResponse, error) {
	if m.getProductErr != nil {
		return nil, m.getProductErr
	}
	return m.getProductResp, nil
}

func (m *mockProductServiceClient) GetProducts(context.Context, *productpb.GetProductsRequest, ...grpc.CallOption) (*productpb.GetProductsResponse, error) {
	// Not needed for current tests
	return nil, nil
}

func createCacheAndRepo(c *domain.Cart) *s.CartService {
	mockRepo := &mockRepository{
		cart: c,
	}
	mc := &mockCache{
		cart: c,
	}
	return s.NewCartService(mockRepo, mc)
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
	service := createCacheAndRepo(cart)
	mockProductClient := &mockProductServiceClient{}
	server := NewCartServiceServer(service, mockProductClient)
	ret, err := server.GetCart(context.Background(), &pb.GetCartRequest{
		UserId: 123,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	t.Logf("Received cart response: %v", ret.Cart)
	l := len(ret.Cart.Cart)
	assert.Equal(t, l, 2)
	assert.Equal(t, ret.Cart.Cart[0].ProductId, int64(1))
	assert.Equal(t, ret.Cart.Cart[0].Quantity, int32(5))
	assert.Equal(t, ret.Cart.Cart[1].ProductId, int64(2))
	assert.Equal(t, ret.Cart.Cart[1].Quantity, int32(10))
}

func TestAddItem_Success(t *testing.T) {
	cart := &domain.Cart{
		Items:     []domain.CartItem{},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	service := createCacheAndRepo(cart)

	// Create mock for ProductServiceClient that returns a valid product
	mockProductClient := &mockProductServiceClient{
		getProductResp: &productpb.GetProductResponse{
			Product: &productpb.Product{
				Id:    1,
				Name:  "Test Product",
				Price: 99.99,
				Stock: 10, // Sufficient stock
			},
		},
	}

	server := NewCartServiceServer(service, mockProductClient)
	ret, err := server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    123,
		ProductId: 1,
		Quantity:  5,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	t.Logf("Received cart response: %v", ret.Cart)
	l := len(ret.Cart.Cart)
	assert.Equal(t, l, 1)
	for _, item := range ret.Cart.Cart {
		t.Logf("Item ID: %d, Quantity: %d", item.ProductId, item.Quantity)
		assert.Equal(t, item.ProductId, int64(1))
		assert.Equal(t, item.Quantity, int32(5))
	}
}

func TestAddItem_NotFound(t *testing.T) {
	cart := &domain.Cart{
		Items:     []domain.CartItem{},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	service := createCacheAndRepo(cart)

	// Create mock for ProductServiceClient that returns a valid product
	mockProductClient := &mockProductServiceClient{
		getProductResp: &productpb.GetProductResponse{
			Product: nil,
		},
		getProductErr: status.Error(codes.NotFound, "product not found"),
	}

	server := NewCartServiceServer(service, mockProductClient)
	ret, err := server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    123,
		ProductId: 1,
		Quantity:  5,
	})

	assert.Nil(t, ret)
	assert.True(t, status.Code(err) == codes.NotFound)
}

func TestAddItem_NoStock(t *testing.T) {
	cart := &domain.Cart{
		Items:     []domain.CartItem{},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	service := createCacheAndRepo(cart)

	// Create mock for ProductServiceClient that returns a valid product
	mockProductClient := &mockProductServiceClient{
		getProductResp: &productpb.GetProductResponse{
			Product: &productpb.Product{
				Id:    1,
				Name:  "Test Product",
				Price: 99.99,
				Stock: 10, // Sufficient stock
			},
		},
	}

	server := NewCartServiceServer(service, mockProductClient)
	ret, err := server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    123,
		ProductId: 1,
		Quantity:  50,
	})

	assert.Nil(t, ret)
	assert.True(t, status.Code(err) == codes.FailedPrecondition)
}

func TestUpdateQuantity_Success(t *testing.T) {
	cart := &domain.Cart{
		Items: []domain.CartItem{
			{ProductID: 1, Quantity: 5},
			{ProductID: 2, Quantity: 10},
		},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	service := createCacheAndRepo(cart)

	mockProductClient := &mockProductServiceClient{}
	server := NewCartServiceServer(service, mockProductClient)

	ret, err := server.UpdateQuantity(context.Background(), &pb.UpdateQuantityRequest{
		UserId:    123,
		ProductId: 1,
		Quantity:  15,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	assert.Equal(t, 2, len(ret.Cart.Cart))
	// Verify the quantity was updated
	assert.Equal(t, int32(15), ret.Cart.Cart[0].Quantity)
	assert.Equal(t, int64(1), ret.Cart.Cart[0].ProductId)
}

func TestUpdateQuantity_InvalidInput(t *testing.T) {
	cart := &domain.Cart{
		Items:     []domain.CartItem{{ProductID: 1, Quantity: 5}},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	service := createCacheAndRepo(cart)

	mockProductClient := &mockProductServiceClient{}
	server := NewCartServiceServer(service, mockProductClient)

	tests := []struct {
		name     string
		req      *pb.UpdateQuantityRequest
		wantCode codes.Code
	}{
		{
			name:     "zero user_id",
			req:      &pb.UpdateQuantityRequest{UserId: 0, ProductId: 1, Quantity: 5},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "zero product_id",
			req:      &pb.UpdateQuantityRequest{UserId: 123, ProductId: 0, Quantity: 5},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "zero quantity",
			req:      &pb.UpdateQuantityRequest{UserId: 123, ProductId: 1, Quantity: 0},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "quantity too high",
			req:      &pb.UpdateQuantityRequest{UserId: 123, ProductId: 1, Quantity: 100},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := server.UpdateQuantity(context.Background(), tt.req)
			assert.Nil(t, ret)
			assert.Equal(t, tt.wantCode, status.Code(err))
		})
	}
}

func TestRemoveItem_Success(t *testing.T) {
	cart := &domain.Cart{
		Items: []domain.CartItem{
			{ProductID: 1, Quantity: 5},
			{ProductID: 2, Quantity: 10},
		},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	service := createCacheAndRepo(cart)

	mockProductClient := &mockProductServiceClient{}
	server := NewCartServiceServer(service, mockProductClient)

	ret, err := server.RemoveItem(context.Background(), &pb.RemoveItemRequest{
		UserId:    123,
		ProductId: 1,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	// Should only have 1 item left
	assert.Equal(t, 1, len(ret.Cart.Cart))
	// The remaining item should be product 2
	assert.Equal(t, int64(2), ret.Cart.Cart[0].ProductId)
	assert.Equal(t, int32(10), ret.Cart.Cart[0].Quantity)
}

func TestRemoveItem_InvalidInput(t *testing.T) {
	cart := &domain.Cart{
		Items:     []domain.CartItem{{ProductID: 1, Quantity: 5}},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	service := createCacheAndRepo(cart)

	mockProductClient := &mockProductServiceClient{}
	server := NewCartServiceServer(service, mockProductClient)

	tests := []struct {
		name     string
		req      *pb.RemoveItemRequest
		wantCode codes.Code
	}{
		{
			name:     "zero user_id",
			req:      &pb.RemoveItemRequest{UserId: 0, ProductId: 1},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "zero product_id",
			req:      &pb.RemoveItemRequest{UserId: 123, ProductId: 0},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := server.RemoveItem(context.Background(), tt.req)
			assert.Nil(t, ret)
			assert.Equal(t, tt.wantCode, status.Code(err))
		})
	}
}

func TestClearCart_Success(t *testing.T) {
	cart := &domain.Cart{
		Items: []domain.CartItem{
			{ProductID: 1, Quantity: 5},
			{ProductID: 2, Quantity: 10},
		},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	service := createCacheAndRepo(cart)

	mockProductClient := &mockProductServiceClient{}
	server := NewCartServiceServer(service, mockProductClient)

	ret, err := server.ClearCart(context.Background(), &pb.ClearCartRequest{
		UserId: 123,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	// Cart should be empty
	assert.Equal(t, 0, len(ret.Cart.Cart))
	assert.Equal(t, int64(123), ret.Cart.UserId)
}

func TestClearCart_InvalidInput(t *testing.T) {
	cart := &domain.Cart{
		Items:     []domain.CartItem{{ProductID: 1, Quantity: 5}},
		UserID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	service := createCacheAndRepo(cart)

	mockProductClient := &mockProductServiceClient{}
	server := NewCartServiceServer(service, mockProductClient)

	ret, err := server.ClearCart(context.Background(), &pb.ClearCartRequest{
		UserId: 0,
	})

	assert.Nil(t, ret)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}
