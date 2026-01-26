package service

import (
	"context"
	"errors"
	"testing"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	ipb "github.com/fjod/go_cart/inventory-service/pkg/proto"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	reserveResponse := &ipb.ReserveResponse{
		ReservationId: "reserveId",
		ExpiresAt:     "",
	}
	mockInventory := &MockInventoryServiceClient{
		reserveResponse: reserveResponse,
		err:             nil,
	}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "new-key-12345",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.CheckoutID)
	assert.Equal(t, d.CheckoutStatusCompleted, *resp.Status)

	// Verify cart snapshot was created with correct total
	assert.NotNil(t, mockRepo.CreatedSession)
	assert.Equal(t, "109.97", mockRepo.CreatedSession.TotalAmount) // (29.99*2) + (49.99*1)
}

func TestInitiateCheckout_ReserveFailed(t *testing.T) {
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

	reserveResponse := &ipb.ReserveResponse{
		ReservationId: "reserveId",
		ExpiresAt:     "",
	}
	mockInventory := &MockInventoryServiceClient{
		reserveResponse: reserveResponse,
		err:             errors.New("insufficient stock"),
	}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "new-key-12345",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	assert.Contains(t, err.Error(), "insufficient stock")
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.CheckoutID)
	assert.Equal(t, d.CheckoutStatusFailed, *resp.Status)

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
	mockInventory := &MockInventoryServiceClient{}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory)

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
	mockInventory := &MockInventoryServiceClient{}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory)

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
	mockInventory := &MockInventoryServiceClient{}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory)

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
	mockInventory := &MockInventoryServiceClient{}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "missing-product-key",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to get product 999")
}

func TestReserveInventory(t *testing.T) {
	mockRepo := &MockRepository{}
	mockCart := &MockCartServiceClient{}
	mockProduct := &MockProductServiceClient{}
	reserveResponse := &ipb.ReserveResponse{
		ReservationId: "reserveId",
		ExpiresAt:     "",
	}
	mockInventory := &MockInventoryServiceClient{
		reserveResponse: reserveResponse,
		err:             nil,
	}

	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory)
	ctx := context.Background()
	items := make([]*CartSnapshotItem, 2)
	items[0] = &CartSnapshotItem{
		ProductID: 1,
		Quantity:  1,
	}
	items[1] = &CartSnapshotItem{
		ProductID: 2,
		Quantity:  2,
	}
	e := svc.reserveInventory(ctx, "checkoutId", items, d.CheckoutStatusInitiated)
	require.NoError(t, e)
	assert.Equal(t, reserveResponse.ReservationId, *mockRepo.ReservationId)
}
