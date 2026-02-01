package service

import (
	"context"
	"errors"
	"time"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	ipb "github.com/fjod/go_cart/inventory-service/pkg/proto"
	paymentpb "github.com/fjod/go_cart/payment-service/pkg/proto"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"google.golang.org/grpc"
)

// MockRepository implements r.RepoInterface for testing
type MockRepository struct {
	GetKey         *string
	GetStatus      *d.CheckoutStatus
	GetErr         error
	CreateErr      error
	CreatedSession *r.CheckoutSession // Captures the session passed to CreateCheckoutSession
	ReservationId  *string
	PaymentId      *string
	OutboxId       *string
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

func (m *MockRepository) SetReservation(_ context.Context, _ *string, _ *d.CheckoutStatus, reserveId *string) error {
	m.ReservationId = reserveId
	return nil
}

func (m *MockRepository) SetPayment(_ context.Context, _ *string, _ *d.CheckoutStatus, payId *string) error {
	m.PaymentId = payId
	return nil
}

func (m *MockRepository) CompleteCheckoutSession(_ context.Context, id *string, _ []byte, _ *d.CheckoutStatus) error {
	m.OutboxId = id
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

// MockInventoryServiceClient implements ipb.InventoryServiceClient for testing
type MockInventoryServiceClient struct {
	stockResponse   *ipb.GetStockResponse
	reserveResponse *ipb.ReserveResponse
	confirmResponse *ipb.ConfirmResponse
	releaseResponse *ipb.ReleaseResponse
	err             error
	ReleaseId       string
}

func (m *MockInventoryServiceClient) GetStock(_ context.Context, _ *ipb.GetStockRequest, _ ...grpc.CallOption) (*ipb.GetStockResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stockResponse, nil
}

func (m *MockInventoryServiceClient) Reserve(_ context.Context, _ *ipb.ReserveRequest, _ ...grpc.CallOption) (*ipb.ReserveResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.reserveResponse, nil
}

func (m *MockInventoryServiceClient) Confirm(_ context.Context, _ *ipb.ConfirmRequest, _ ...grpc.CallOption) (*ipb.ConfirmResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.confirmResponse, nil
}

func (m *MockInventoryServiceClient) Release(_ context.Context, r *ipb.ReleaseRequest, _ ...grpc.CallOption) (*ipb.ReleaseResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.ReleaseId = r.ReservationId
	return m.releaseResponse, nil
}

type MockPaymentServiceClient struct {
	err           error
	cr            *paymentpb.ChargeResponse
	PaymentAmount string
}

func (s *MockPaymentServiceClient) Charge(_ context.Context, r *paymentpb.ChargeRequest, _ ...grpc.CallOption) (*paymentpb.ChargeResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.PaymentAmount = r.Amount
	return s.cr, nil
}

func (*MockPaymentServiceClient) Refund(context.Context, *paymentpb.RefundRequest, ...grpc.CallOption) (*paymentpb.RefundResponse, error) {
	return &paymentpb.RefundResponse{}, nil
}

// newTestCheckoutService creates a fully wired CheckoutService for testing
func newTestCheckoutService(
	repo *MockRepository,
	cartClient *MockCartServiceClient,
	productClient *MockProductServiceClient,
	inv *MockInventoryServiceClient,
	pay *MockPaymentServiceClient,
) *CheckoutServiceImpl {
	cartHandler := NewCartHandler(cartClient, 5*time.Second)
	productHandler := NewProductHandler(productClient, 5*time.Second)
	inventoryService := NewInventoryHandler(inv, 5*time.Second)
	payService := NewPaymentHandler(pay, 5*time.Second)
	return NewCheckoutService(repo, cartHandler, productHandler, inventoryService, payService)
}
