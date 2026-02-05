package service

import (
	"context"
	"errors"
	"testing"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	ipb "github.com/fjod/go_cart/inventory-service/pkg/proto"
	paymentpb "github.com/fjod/go_cart/payment-service/pkg/proto"
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
	mockPay := &MockPaymentServiceClient{
		err: nil,
		cr: &paymentpb.ChargeResponse{
			Status: paymentpb.ChargeStatus_CHARGE_STATUS_SUCCESS,
		},
	}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)

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
	assert.Equal(t, "109.97", mockRepo.CreatedSession.TotalAmount)          // (29.99*2) + (49.99*1) snapshot sum is fine
	assert.Equal(t, reserveResponse.ReservationId, *mockRepo.ReservationId) // reserved
	assert.Equal(t, "109.97", mockPay.PaymentAmount)                        // paid
	assert.Equal(t, resp.CheckoutID, mockRepo.OutboxId)                     // saved to outbox
}

func TestInitiateCheckout_ReleaseInventory(t *testing.T) {
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
		ReservationId: "reserveId-1122",
		ExpiresAt:     "",
	}
	mockInventory := &MockInventoryServiceClient{
		reserveResponse: reserveResponse,
		err:             nil,
	}
	mockPay := &MockPaymentServiceClient{
		err: nil,
		cr: &paymentpb.ChargeResponse{
			Status:  paymentpb.ChargeStatus_CHARGE_STATUS_FAILED,
			Refusal: &paymentpb.ChargeResponse_KnownReason{KnownReason: paymentpb.PaymentRefusal_NO_FUNDS},
		},
	}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "new-key-12345",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FAILED")
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.CheckoutID)
	assert.Equal(t, d.CheckoutStatusFailed, *resp.Status)

	// Verify cart snapshot was created with correct total
	assert.NotNil(t, mockRepo.CreatedSession)
	assert.Equal(t, "109.97", mockRepo.CreatedSession.TotalAmount) // (29.99*2) + (49.99*1)
	assert.Equal(t, "109.97", mockPay.PaymentAmount)
	assert.Equal(t, "reserveId-1122", mockInventory.ReleaseId) // we called inventory release for reservationId
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

	mockPay := &MockPaymentServiceClient{}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)

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
	mockPay := &MockPaymentServiceClient{}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)

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
	mockPay := &MockPaymentServiceClient{}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)

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
	mockPay := &MockPaymentServiceClient{}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)

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
	mockPay := &MockPaymentServiceClient{}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)

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
	mockPay := &MockPaymentServiceClient{
		err: nil,
		cr: &paymentpb.ChargeResponse{
			Status: paymentpb.ChargeStatus_CHARGE_STATUS_SUCCESS,
		},
	}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)
	ctx := context.Background()
	items := make([]*d.CartSnapshotItem, 2)
	items[0] = &d.CartSnapshotItem{
		ProductID: 1,
		Quantity:  1,
	}
	items[1] = &d.CartSnapshotItem{
		ProductID: 2,
		Quantity:  2,
	}
	id, e := svc.reserveInventory(ctx, "checkoutId", items, d.CheckoutStatusInitiated)
	require.NoError(t, e)
	assert.Equal(t, reserveResponse.ReservationId, *mockRepo.ReservationId)
	assert.Equal(t, reserveResponse.ReservationId, *id)
}

func TestPayment_NoError(t *testing.T) {
	mockRepo := &MockRepository{}
	mockCart := &MockCartServiceClient{}
	mockProduct := &MockProductServiceClient{}
	mockInventory := &MockInventoryServiceClient{}
	mockPay := &MockPaymentServiceClient{
		err: nil,
		cr: &paymentpb.ChargeResponse{
			Status:    paymentpb.ChargeStatus_CHARGE_STATUS_SUCCESS,
			PaymentId: "payId",
		},
	}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)
	e := svc.processPayment(context.Background(), "checkoutId", d.CheckoutStatusInventoryReserved, "1234.56")
	require.NoError(t, e)
	assert.Equal(t, "payId", *mockRepo.PaymentId)
	assert.Equal(t, "1234.56", mockPay.PaymentAmount)
}

func TestPayment_KnownError(t *testing.T) {
	mockRepo := &MockRepository{}
	mockCart := &MockCartServiceClient{}
	mockProduct := &MockProductServiceClient{}
	mockInventory := &MockInventoryServiceClient{}
	mockPay := &MockPaymentServiceClient{
		err: nil,
		cr: &paymentpb.ChargeResponse{
			Status:  paymentpb.ChargeStatus_CHARGE_STATUS_FAILED,
			Refusal: &paymentpb.ChargeResponse_KnownReason{KnownReason: paymentpb.PaymentRefusal_NO_FUNDS},
		},
	}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)
	err := svc.processPayment(context.Background(), "checkoutId", d.CheckoutStatusInventoryReserved, "1234.56")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NO_FUNDS")
	assert.Equal(t, "1234.56", mockPay.PaymentAmount)
}

func TestPayment_OtherError(t *testing.T) {
	mockRepo := &MockRepository{}
	mockCart := &MockCartServiceClient{}
	mockProduct := &MockProductServiceClient{}
	mockInventory := &MockInventoryServiceClient{}
	mockPay := &MockPaymentServiceClient{
		err: nil,
		cr: &paymentpb.ChargeResponse{
			Status:  paymentpb.ChargeStatus_CHARGE_STATUS_FAILED,
			Refusal: &paymentpb.ChargeResponse_OtherReason{OtherReason: "other failure"},
		},
	}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)
	err := svc.processPayment(context.Background(), "checkoutId", d.CheckoutStatusInventoryReserved, "1234.56")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "other failure")
	assert.Equal(t, "1234.56", mockPay.PaymentAmount)
}

func TestCompleteCheckout(t *testing.T) {
	mockRepo := &MockRepository{}
	mockCart := &MockCartServiceClient{}
	mockProduct := &MockProductServiceClient{}
	mockInventory := &MockInventoryServiceClient{}
	mockPay := &MockPaymentServiceClient{}
	svc := newTestCheckoutService(mockRepo, mockCart, mockProduct, mockInventory, mockPay)
	ctx := context.Background()

	snapshot := &d.CartSnapshot{}
	e := svc.complete(ctx, "checkoutId", d.CheckoutStatusPaymentCompleted, snapshot, "user")
	require.NoError(t, e)
	assert.Equal(t, "checkoutId", *mockRepo.OutboxId)
}
