package service

import (
	"context"
	"errors"
	"testing"
	"time"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// MockRepository implements r.RepoInterface for testing
type MockRepository struct {
	GetKey         *string
	GetStatus      *d.CheckoutStatus
	GetErr         error
	CreateErr      error
	CreatedSession *r.CheckoutSession // Captures the session passed to CreateCheckoutSession
}

func (m *MockRepository) Close() error {
	return nil
}

func (m *MockRepository) RunMigrations(*r.Credentials) error {
	return nil
}

func (m *MockRepository) GetCheckoutSessionByIdempotencyKey(_ context.Context, _ string) (*string, *d.CheckoutStatus, error) {
	return m.GetKey, m.GetStatus, m.GetErr
}

func (m *MockRepository) CreateCheckoutSession(_ context.Context, session *r.CheckoutSession) error {
	m.CreatedSession = session
	return m.CreateErr
}

func (m *MockRepository) UpdateCheckoutSessionStatus(_ context.Context, _ *string, _ *d.CheckoutStatus) error {
	return nil
}

// MockCartServiceClient implements cartpb.CartServiceClient for testing
type MockCartServiceClient struct {
	CartResponse *cartpb.CartResponse
	Err          error
}

func (m *MockCartServiceClient) AddItem(_ context.Context, _ *cartpb.AddCartItemRequest, _ ...grpc.CallOption) (*cartpb.CartResponse, error) {
	return m.CartResponse, m.Err
}

func (m *MockCartServiceClient) GetCart(_ context.Context, _ *cartpb.GetCartRequest, _ ...grpc.CallOption) (*cartpb.CartResponse, error) {
	return m.CartResponse, m.Err
}

func (m *MockCartServiceClient) UpdateQuantity(_ context.Context, _ *cartpb.UpdateQuantityRequest, _ ...grpc.CallOption) (*cartpb.CartResponse, error) {
	return m.CartResponse, m.Err
}

func (m *MockCartServiceClient) RemoveItem(_ context.Context, _ *cartpb.RemoveItemRequest, _ ...grpc.CallOption) (*cartpb.CartResponse, error) {
	return m.CartResponse, m.Err
}

func (m *MockCartServiceClient) ClearCart(_ context.Context, _ *cartpb.ClearCartRequest, _ ...grpc.CallOption) (*cartpb.CartResponse, error) {
	return m.CartResponse, m.Err
}

// MockProductServiceClient implements productpb.ProductServiceClient for testing
type MockProductServiceClient struct {
	Products map[int64]*productpb.Product // Map of product ID to product
	Err      error
}

func (m *MockProductServiceClient) GetProducts(_ context.Context, _ *productpb.GetProductsRequest, _ ...grpc.CallOption) (*productpb.GetProductsResponse, error) {
	var products []*productpb.Product
	for _, p := range m.Products {
		products = append(products, p)
	}
	return &productpb.GetProductsResponse{Products: products}, m.Err
}

func (m *MockProductServiceClient) GetProduct(_ context.Context, req *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.GetProductResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	product, exists := m.Products[req.Id]
	if !exists {
		return nil, errors.New("product not found")
	}
	return &productpb.GetProductResponse{Product: product}, nil
}

// Helper to create a fully wired CheckoutService for testing
func newTestCheckoutService(repo *MockRepository, cartClient *MockCartServiceClient, productClient *MockProductServiceClient) *CheckoutServiceImpl {
	cartHandler := NewCartHandler(cartClient, 5*time.Second)
	productHandler := NewProductHandler(productClient, 5*time.Second)
	return NewCheckoutService(repo, cartHandler, productHandler)
}

func TestInitiateCheckout_NewRequest(t *testing.T) {
	mockRepo := &MockRepository{
		GetKey:    nil,
		GetStatus: nil,
		GetErr:    r.ErrIdempotencyKeyNotFound,
	}

	mockCart := &MockCartServiceClient{
		CartResponse: &cartpb.CartResponse{
			Cart: &cartpb.Cart{
				Cart: []*cartpb.CartItem{
					{ProductId: 1, Quantity: 2},
					{ProductId: 2, Quantity: 1},
				},
			},
		},
	}

	mockProduct := &MockProductServiceClient{
		Products: map[int64]*productpb.Product{
			1: {Id: 1, Name: "Widget", Price: 29.99},
			2: {Id: 2, Name: "Gadget", Price: 49.99},
		},
	}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "new-key-12345",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.CheckoutID)
	assert.Equal(t, d.CheckoutStatusInitiated, *resp.Status)

	// Verify cart snapshot was created with correct total
	assert.NotNil(t, mockRepo.CreatedSession)
	assert.Equal(t, "109.97", mockRepo.CreatedSession.TotalAmount) // (29.99*2) + (49.99*1)
}

func TestInitiateCheckout_DuplicateRequest(t *testing.T) {
	existingID := "checkout-abc-123"
	existingStatus := d.CheckoutStatusCompleted

	mockRepo := &MockRepository{
		GetKey:    &existingID,
		GetStatus: &existingStatus,
		GetErr:    nil,
	}

	// Cart and product mocks won't be called for duplicate requests
	mockCart := &MockCartServiceClient{}
	mockProduct := &MockProductServiceClient{}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "existing-key",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, existingID, *resp.CheckoutID)
	assert.Equal(t, existingStatus, *resp.Status)
}

func TestInitiateCheckout_RepositoryError(t *testing.T) {
	mockRepo := &MockRepository{
		GetKey:    nil,
		GetStatus: nil,
		GetErr:    errors.New("repository error"),
	}

	// Cart and product mocks won't be called when repo errors
	mockCart := &MockCartServiceClient{}
	mockProduct := &MockProductServiceClient{}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "error-key",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to check idempotency")
}

func TestInitiateCheckout_EmptyCart(t *testing.T) {
	mockRepo := &MockRepository{
		GetKey:    nil,
		GetStatus: nil,
		GetErr:    r.ErrIdempotencyKeyNotFound,
	}

	mockCart := &MockCartServiceClient{
		CartResponse: &cartpb.CartResponse{
			Cart: &cartpb.Cart{
				Cart: []*cartpb.CartItem{}, // Empty cart
			},
		},
	}

	mockProduct := &MockProductServiceClient{}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "empty-cart-key",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(ErrEmptyCart, err))
}

func TestInitiateCheckout_ProductNotFound(t *testing.T) {
	mockRepo := &MockRepository{
		GetKey:    nil,
		GetStatus: nil,
		GetErr:    r.ErrIdempotencyKeyNotFound,
	}

	mockCart := &MockCartServiceClient{
		CartResponse: &cartpb.CartResponse{
			Cart: &cartpb.Cart{
				Cart: []*cartpb.CartItem{
					{ProductId: 999, Quantity: 1}, // Product doesn't exist
				},
			},
		},
	}

	mockProduct := &MockProductServiceClient{
		Products: map[int64]*productpb.Product{}, // Empty - no products
	}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "missing-product-key",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to get product 999")
}
