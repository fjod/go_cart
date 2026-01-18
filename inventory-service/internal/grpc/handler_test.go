package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/fjod/go_cart/inventory-service/internal/domain"
	"github.com/fjod/go_cart/inventory-service/internal/store"
	pb "github.com/fjod/go_cart/inventory-service/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockStore implements store.InventoryStore for testing
type mockStore struct {
	stocks       map[int64]*domain.StockInfo
	reservations map[string]*domain.Reservation
	reserveErr   error
	confirmErr   error
	releaseErr   error
}

func newMockStore() *mockStore {
	return &mockStore{
		stocks:       make(map[int64]*domain.StockInfo),
		reservations: make(map[string]*domain.Reservation),
	}
}

func (m *mockStore) GetStock(productIDs []int64) ([]domain.StockInfo, error) {
	result := make([]domain.StockInfo, 0)
	for _, id := range productIDs {
		if stock, ok := m.stocks[id]; ok {
			result = append(result, *stock)
		}
	}
	return result, nil
}

func (m *mockStore) Reserve(checkoutID string, items []domain.ReservationItem) (*domain.Reservation, error) {
	if m.reserveErr != nil {
		return nil, m.reserveErr
	}
	reservation := &domain.Reservation{
		ID:         "test-reservation-id",
		CheckoutID: checkoutID,
		Items:      items,
		Status:     domain.StatusReserved,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}
	m.reservations[reservation.ID] = reservation
	return reservation, nil
}

func (m *mockStore) Confirm(reservationID string) error {
	if m.confirmErr != nil {
		return m.confirmErr
	}
	return nil
}

func (m *mockStore) Release(reservationID string) error {
	if m.releaseErr != nil {
		return m.releaseErr
	}
	return nil
}

func (m *mockStore) SetStock(productID int64, quantity int32) error {
	m.stocks[productID] = &domain.StockInfo{
		ProductID: productID,
		Total:     quantity,
		Reserved:  0,
	}
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

func TestHandler_GetStock(t *testing.T) {
	mock := newMockStore()
	mock.SetStock(1, 100)
	mock.SetStock(2, 200)
	handler := NewInventoryServiceServer(mock)

	resp, err := handler.GetStock(context.Background(), &pb.GetStockRequest{
		ProductIds: []int64{1, 2, 3},
	})

	require.NoError(t, err)
	assert.Len(t, resp.Stocks, 2)
}

func TestHandler_GetStock_Empty(t *testing.T) {
	mock := newMockStore()
	handler := NewInventoryServiceServer(mock)

	resp, err := handler.GetStock(context.Background(), &pb.GetStockRequest{
		ProductIds: []int64{},
	})

	require.NoError(t, err)
	assert.Empty(t, resp.Stocks)
}

func TestHandler_Reserve_Success(t *testing.T) {
	mock := newMockStore()
	mock.SetStock(1, 100)
	handler := NewInventoryServiceServer(mock)

	resp, err := handler.Reserve(context.Background(), &pb.ReserveRequest{
		CheckoutId: "checkout-123",
		Items: []*pb.ReservationItem{
			{ProductId: 1, Quantity: 10},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "test-reservation-id", resp.ReservationId)
	assert.NotEmpty(t, resp.ExpiresAt)
}

func TestHandler_Reserve_ValidationErrors(t *testing.T) {
	mock := newMockStore()
	handler := NewInventoryServiceServer(mock)

	tests := []struct {
		name    string
		request *pb.ReserveRequest
		wantMsg string
	}{
		{
			name:    "empty checkout_id",
			request: &pb.ReserveRequest{CheckoutId: "", Items: []*pb.ReservationItem{{ProductId: 1, Quantity: 1}}},
			wantMsg: "checkout_id is required",
		},
		{
			name:    "no items",
			request: &pb.ReserveRequest{CheckoutId: "test", Items: []*pb.ReservationItem{}},
			wantMsg: "at least one item is required",
		},
		{
			name:    "invalid product_id",
			request: &pb.ReserveRequest{CheckoutId: "test", Items: []*pb.ReservationItem{{ProductId: 0, Quantity: 1}}},
			wantMsg: "product_id must be greater than 0",
		},
		{
			name:    "invalid quantity",
			request: &pb.ReserveRequest{CheckoutId: "test", Items: []*pb.ReservationItem{{ProductId: 1, Quantity: 0}}},
			wantMsg: "quantity must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.Reserve(context.Background(), tt.request)
			require.Error(t, err)
			st, _ := status.FromError(err)
			assert.Equal(t, codes.InvalidArgument, st.Code())
			assert.Contains(t, st.Message(), tt.wantMsg)
		})
	}
}

func TestHandler_Reserve_InsufficientStock(t *testing.T) {
	mock := newMockStore()
	mock.reserveErr = store.ErrInsufficientStock
	handler := NewInventoryServiceServer(mock)

	_, err := handler.Reserve(context.Background(), &pb.ReserveRequest{
		CheckoutId: "checkout-123",
		Items:      []*pb.ReservationItem{{ProductId: 1, Quantity: 10}},
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestHandler_Reserve_ProductNotFound(t *testing.T) {
	mock := newMockStore()
	mock.reserveErr = store.ErrProductNotFound
	handler := NewInventoryServiceServer(mock)

	_, err := handler.Reserve(context.Background(), &pb.ReserveRequest{
		CheckoutId: "checkout-123",
		Items:      []*pb.ReservationItem{{ProductId: 999, Quantity: 10}},
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestHandler_Confirm_Success(t *testing.T) {
	mock := newMockStore()
	handler := NewInventoryServiceServer(mock)

	resp, err := handler.Confirm(context.Background(), &pb.ConfirmRequest{
		ReservationId: "test-id",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandler_Confirm_EmptyID(t *testing.T) {
	mock := newMockStore()
	handler := NewInventoryServiceServer(mock)

	_, err := handler.Confirm(context.Background(), &pb.ConfirmRequest{
		ReservationId: "",
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestHandler_Confirm_NotFound(t *testing.T) {
	mock := newMockStore()
	mock.confirmErr = store.ErrReservationNotFound
	handler := NewInventoryServiceServer(mock)

	_, err := handler.Confirm(context.Background(), &pb.ConfirmRequest{
		ReservationId: "nonexistent",
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestHandler_Release_Success(t *testing.T) {
	mock := newMockStore()
	handler := NewInventoryServiceServer(mock)

	resp, err := handler.Release(context.Background(), &pb.ReleaseRequest{
		ReservationId: "test-id",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHandler_Release_EmptyID(t *testing.T) {
	mock := newMockStore()
	handler := NewInventoryServiceServer(mock)

	_, err := handler.Release(context.Background(), &pb.ReleaseRequest{
		ReservationId: "",
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestHandler_Release_InvalidStatus(t *testing.T) {
	mock := newMockStore()
	mock.releaseErr = store.ErrInvalidStatus
	handler := NewInventoryServiceServer(mock)

	_, err := handler.Release(context.Background(), &pb.ReleaseRequest{
		ReservationId: "already-confirmed",
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}
