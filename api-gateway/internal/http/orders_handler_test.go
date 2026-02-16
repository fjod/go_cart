package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pb "github.com/fjod/go_cart/orders-service/pkg/proto"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- Mock ---

type OrdersClientMock struct {
	order  *pb.Order
	orders []*pb.Order
	err    error
}

func (m OrdersClientMock) GetOrder(ctx context.Context, in *pb.GetOrderRequest, opts ...grpc.CallOption) (*pb.GetOrderResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &pb.GetOrderResponse{Order: m.order}, nil
}

func (m OrdersClientMock) ListOrders(ctx context.Context, in *pb.ListOrdersRequest, opts ...grpc.CallOption) (*pb.ListOrdersResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &pb.ListOrdersResponse{Orders: m.orders}, nil
}

// --- helper ---

func withUser(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), "user_id", int64(1))
	return r.WithContext(ctx)
}

func withOrderID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("order_id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- ListOrders tests ---

func TestListOrders_Success(t *testing.T) {
	mock := OrdersClientMock{
		orders: []*pb.Order{
			{
				Id:          "order-uuid-1",
				CheckoutId:  "checkout-uuid-1",
				UserId:      "1",
				TotalAmount: 1299.99,
				Currency:    "USD",
				Status:      "CONFIRMED",
				Items: []*pb.OrderItem{
					{ProductId: 1, ProductName: "Laptop", Quantity: 1, Price: 1299.99},
				},
				CreatedAt: "2026-02-12T10:00:00Z",
			},
		},
	}

	handler := NewOrdersHandler(mock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := withUser(httptest.NewRequest("GET", "/api/v1/orders", nil))

	handler.ListOrders(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, recorder.Code)
	}

	var response []OrderResponseDTO
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 1 {
		t.Fatalf("expected 1 order, got %d", len(response))
	}
	if response[0].ID != "order-uuid-1" {
		t.Errorf("expected id 'order-uuid-1', got '%s'", response[0].ID)
	}
	if response[0].TotalAmount != 1299.99 {
		t.Errorf("expected total_amount 1299.99, got %f", response[0].TotalAmount)
	}
	if len(response[0].Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(response[0].Items))
	}
	if response[0].Items[0].ProductName != "Laptop" {
		t.Errorf("expected product_name 'Laptop', got '%s'", response[0].Items[0].ProductName)
	}
}

func TestListOrders_EmptyList(t *testing.T) {
	mock := OrdersClientMock{orders: []*pb.Order{}}

	handler := NewOrdersHandler(mock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := withUser(httptest.NewRequest("GET", "/api/v1/orders", nil))

	handler.ListOrders(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, recorder.Code)
	}

	// Must be a JSON array, not null
	body := recorder.Body.String()
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if string(raw) == "null" {
		t.Error("expected empty JSON array [], got null")
	}

	var response []OrderResponseDTO
	json.NewDecoder(recorder.Body).Decode(&response)
	if len(response) != 0 {
		t.Errorf("expected empty slice, got %d items", len(response))
	}
}

func TestListOrders_Unauthorized(t *testing.T) {
	mock := OrdersClientMock{}
	handler := NewOrdersHandler(mock, 5*time.Second)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/v1/orders", nil)
	// No user_id in context

	handler.ListOrders(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, recorder.Code)
	}

	var response ErrorResponse
	json.NewDecoder(recorder.Body).Decode(&response)
	if response.Code != "unauthorized" {
		t.Errorf("expected 'unauthorized', got '%s'", response.Code)
	}
}

func TestListOrders_GRPCErrors(t *testing.T) {
	tests := []struct {
		name         string
		grpcCode     codes.Code
		expectedHTTP int
		expectedCode string
	}{
		{"Internal", codes.Internal, http.StatusInternalServerError, "internal_error"},
		{"Unavailable", codes.Unavailable, http.StatusServiceUnavailable, "service_unavailable"},
		{"DeadlineExceeded", codes.DeadlineExceeded, http.StatusGatewayTimeout, "timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := OrdersClientMock{err: status.Error(tt.grpcCode, "test error")}
			handler := NewOrdersHandler(mock, 5*time.Second)

			recorder := httptest.NewRecorder()
			request := withUser(httptest.NewRequest("GET", "/api/v1/orders", nil))

			handler.ListOrders(recorder, request)

			if recorder.Code != tt.expectedHTTP {
				t.Errorf("expected %d, got %d", tt.expectedHTTP, recorder.Code)
			}

			var response ErrorResponse
			json.NewDecoder(recorder.Body).Decode(&response)
			if response.Code != tt.expectedCode {
				t.Errorf("expected code '%s', got '%s'", tt.expectedCode, response.Code)
			}
		})
	}
}

// --- GetOrder tests ---

func TestGetOrder_Success(t *testing.T) {
	mock := OrdersClientMock{
		order: &pb.Order{
			Id:          "order-uuid-1",
			CheckoutId:  "checkout-uuid-1",
			UserId:      "1",
			TotalAmount: 29.99,
			Currency:    "USD",
			Status:      "CONFIRMED",
			Items: []*pb.OrderItem{
				{ProductId: 2, ProductName: "Mouse", Quantity: 1, Price: 29.99},
			},
			CreatedAt: "2026-02-12T10:00:00Z",
		},
	}

	handler := NewOrdersHandler(mock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := withOrderID(withUser(httptest.NewRequest("GET", "/api/v1/orders/order-uuid-1", nil)), "order-uuid-1")

	handler.GetOrder(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, recorder.Code)
	}

	var response OrderResponseDTO
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ID != "order-uuid-1" {
		t.Errorf("expected id 'order-uuid-1', got '%s'", response.ID)
	}
	if response.Status != "CONFIRMED" {
		t.Errorf("expected status 'CONFIRMED', got '%s'", response.Status)
	}
}

func TestGetOrder_Unauthorized(t *testing.T) {
	mock := OrdersClientMock{}
	handler := NewOrdersHandler(mock, 5*time.Second)

	recorder := httptest.NewRecorder()
	request := withOrderID(httptest.NewRequest("GET", "/api/v1/orders/order-uuid-1", nil), "order-uuid-1")
	// No user_id in context

	handler.GetOrder(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestGetOrder_MissingOrderID(t *testing.T) {
	mock := OrdersClientMock{}
	handler := NewOrdersHandler(mock, 5*time.Second)

	recorder := httptest.NewRecorder()
	// No chi route context → order_id is empty string
	request := withUser(httptest.NewRequest("GET", "/api/v1/orders/", nil))

	handler.GetOrder(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response ErrorResponse
	json.NewDecoder(recorder.Body).Decode(&response)
	if response.Code != "missing_order_id" {
		t.Errorf("expected 'missing_order_id', got '%s'", response.Code)
	}
}

func TestGetOrder_GRPCErrors(t *testing.T) {
	tests := []struct {
		name         string
		grpcCode     codes.Code
		expectedHTTP int
		expectedCode string
	}{
		{"NotFound", codes.NotFound, http.StatusNotFound, "not_found"},
		{"InvalidArgument", codes.InvalidArgument, http.StatusBadRequest, "invalid_argument"},
		{"Internal", codes.Internal, http.StatusInternalServerError, "internal_error"},
		{"Unavailable", codes.Unavailable, http.StatusServiceUnavailable, "service_unavailable"},
		{"DeadlineExceeded", codes.DeadlineExceeded, http.StatusGatewayTimeout, "timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := OrdersClientMock{err: status.Error(tt.grpcCode, "test error")}
			handler := NewOrdersHandler(mock, 5*time.Second)

			recorder := httptest.NewRecorder()
			request := withOrderID(withUser(httptest.NewRequest("GET", "/api/v1/orders/some-id", nil)), "some-id")

			handler.GetOrder(recorder, request)

			if recorder.Code != tt.expectedHTTP {
				t.Errorf("expected %d, got %d", tt.expectedHTTP, recorder.Code)
			}

			var response ErrorResponse
			json.NewDecoder(recorder.Body).Decode(&response)
			if response.Code != tt.expectedCode {
				t.Errorf("expected code '%s', got '%s'", tt.expectedCode, response.Code)
			}
		})
	}
}

// --- convertProtoOrder tests ---

func TestConvertProtoOrder_AllFields(t *testing.T) {
	order := &pb.Order{
		Id:          "abc-123",
		CheckoutId:  "chk-456",
		UserId:      "1",
		TotalAmount: 399.98,
		Currency:    "USD",
		Status:      "CONFIRMED",
		Items: []*pb.OrderItem{
			{ProductId: 4, ProductName: "Monitor", Quantity: 2, Price: 199.99},
		},
		CreatedAt: "2026-02-12T10:00:00Z",
	}

	dto := convertProtoOrder(order)

	if dto.ID != "abc-123" {
		t.Errorf("ID: expected 'abc-123', got '%s'", dto.ID)
	}
	if dto.CheckoutID != "chk-456" {
		t.Errorf("CheckoutID: expected 'chk-456', got '%s'", dto.CheckoutID)
	}
	if dto.TotalAmount != 399.98 {
		t.Errorf("TotalAmount: expected 399.98, got %f", dto.TotalAmount)
	}
	if dto.Currency != "USD" {
		t.Errorf("Currency: expected 'USD', got '%s'", dto.Currency)
	}
	if dto.Status != "CONFIRMED" {
		t.Errorf("Status: expected 'CONFIRMED', got '%s'", dto.Status)
	}
	if dto.CreatedAt != "2026-02-12T10:00:00Z" {
		t.Errorf("CreatedAt: expected '2026-02-12T10:00:00Z', got '%s'", dto.CreatedAt)
	}
	if len(dto.Items) != 1 {
		t.Fatalf("Items: expected 1, got %d", len(dto.Items))
	}
	item := dto.Items[0]
	if item.ProductID != 4 || item.ProductName != "Monitor" || item.Quantity != 2 || item.Price != 199.99 {
		t.Errorf("Item fields mismatch: %+v", item)
	}
}

func TestConvertProtoOrder_EmptyItems(t *testing.T) {
	order := &pb.Order{
		Id:    "abc-123",
		Items: []*pb.OrderItem{},
	}

	dto := convertProtoOrder(order)

	if dto.Items == nil {
		t.Error("Items should not be nil — must serialise as [] not null")
	}
	if len(dto.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(dto.Items))
	}
}
