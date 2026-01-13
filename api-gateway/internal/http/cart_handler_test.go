package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pb "github.com/fjod/go_cart/cart-service/pkg/proto"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ClientMock struct {
	cart *pb.Cart
	err  error
}

func (c ClientMock) GetCart(ctx context.Context, in *pb.GetCartRequest, opts ...grpc.CallOption) (*pb.CartResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &pb.CartResponse{
		Cart: c.cart,
	}, nil
}

func (c ClientMock) AddItem(ctx context.Context, in *pb.AddCartItemRequest, opts ...grpc.CallOption) (*pb.CartResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &pb.CartResponse{
		Cart: c.cart,
	}, nil
}

func (c ClientMock) UpdateQuantity(ctx context.Context, in *pb.UpdateQuantityRequest, opts ...grpc.CallOption) (*pb.CartResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &pb.CartResponse{
		Cart: c.cart,
	}, nil
}

func (c ClientMock) RemoveItem(ctx context.Context, in *pb.RemoveItemRequest, opts ...grpc.CallOption) (*pb.CartResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &pb.CartResponse{
		Cart: c.cart,
	}, nil
}

func (c ClientMock) ClearCart(ctx context.Context, in *pb.ClearCartRequest, opts ...grpc.CallOption) (*pb.CartResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &pb.CartResponse{
		Cart: c.cart,
	}, nil
}

func TestGetCart_Success(t *testing.T) {
	clientMock := ClientMock{
		cart: &pb.Cart{
			UserId: 1,
			Cart: []*pb.CartItem{
				{ProductId: 1, Quantity: 2},
			},
		},
		err: nil,
	}

	handler := NewCartHandler(clientMock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)

	// Add user_id to context
	ctx := context.WithValue(request.Context(), "user_id", int64(1))
	request = request.WithContext(ctx)

	handler.GetCart(recorder, request)

	// Verify response
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.Code)
	}

	var response pb.Cart
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.UserId != 1 {
		t.Errorf("Expected user_id 1, got %d", response.UserId)
	}
}

func TestGetCart_Unauthorized(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)
	// No user_id in context

	handler.GetCart(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, recorder.Code)
	}

	var response ErrorResponse
	json.NewDecoder(recorder.Body).Decode(&response)
	if response.Code != "unauthorized" {
		t.Errorf("Expected error code 'unauthorized', got '%s'", response.Code)
	}
}

func TestAddItem_Success(t *testing.T) {
	clientMock := ClientMock{
		cart: &pb.Cart{
			UserId: 1,
			Cart: []*pb.CartItem{
				{ProductId: 1, Quantity: 2},
			},
		},
		err: nil,
	}

	handler := NewCartHandler(clientMock, 5*time.Second)
	req := &AddItemRequestDTO{
		ProductID: 1,
		Quantity:  2,
	}

	reqBytes, _ := json.Marshal(req)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/items", bytes.NewReader(reqBytes))

	// Add user_id to context
	ctx := context.WithValue(request.Context(), "user_id", int64(1))
	ctx = context.WithValue(ctx, "request_id", "test-request-123")
	request = request.WithContext(ctx)

	handler.AddItem(recorder, request)

	// Verify response
	if recorder.Code != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, recorder.Code)
	}

	var response pb.Cart
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.UserId != 1 {
		t.Errorf("Expected user_id 1, got %d", response.UserId)
	}
}

func TestAddItem_Unauthorized(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	req := &AddItemRequestDTO{ProductID: 1, Quantity: 2}
	reqBytes, _ := json.Marshal(req)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/items", bytes.NewReader(reqBytes))
	// No user_id in context

	handler.AddItem(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, recorder.Code)
	}

	var response ErrorResponse
	json.NewDecoder(recorder.Body).Decode(&response)
	if response.Code != "unauthorized" {
		t.Errorf("Expected error code 'unauthorized', got '%s'", response.Code)
	}
}

func TestAddItem_InvalidJSON(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/items", bytes.NewReader([]byte("invalid json")))

	ctx := context.WithValue(request.Context(), "user_id", int64(1))
	request = request.WithContext(ctx)

	handler.AddItem(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response ErrorResponse
	json.NewDecoder(recorder.Body).Decode(&response)
	if response.Code != "invalid_request" {
		t.Errorf("Expected error code 'invalid_request', got '%s'", response.Code)
	}
}

func TestAddItem_InvalidProductID(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	tests := []struct {
		name      string
		productID int64
	}{
		{"zero product_id", 0},
		{"negative product_id", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &AddItemRequestDTO{ProductID: tt.productID, Quantity: 2}
			reqBytes, _ := json.Marshal(req)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/items", bytes.NewReader(reqBytes))

			ctx := context.WithValue(request.Context(), "user_id", int64(1))
			request = request.WithContext(ctx)

			handler.AddItem(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, recorder.Code)
			}

			var response ErrorResponse
			json.NewDecoder(recorder.Body).Decode(&response)
			if response.Code != "invalid_product_id" {
				t.Errorf("Expected error code 'invalid_product_id', got '%s'", response.Code)
			}
		})
	}
}

func TestAddItem_InvalidQuantity(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	tests := []struct {
		name     string
		quantity int32
	}{
		{"zero quantity", 0},
		{"negative quantity", -1},
		{"quantity too high", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &AddItemRequestDTO{ProductID: 1, Quantity: tt.quantity}
			reqBytes, _ := json.Marshal(req)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/items", bytes.NewReader(reqBytes))

			ctx := context.WithValue(request.Context(), "user_id", int64(1))
			request = request.WithContext(ctx)

			handler.AddItem(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, recorder.Code)
			}

			var response ErrorResponse
			json.NewDecoder(recorder.Body).Decode(&response)
			if response.Code != "invalid_quantity" {
				t.Errorf("Expected error code 'invalid_quantity', got '%s'", response.Code)
			}
		})
	}
}

func TestAddItem_GRPCErrors(t *testing.T) {
	tests := []struct {
		name         string
		grpcCode     codes.Code
		expectedHTTP int
		expectedCode string
	}{
		{"NotFound", codes.NotFound, http.StatusNotFound, "not_found"},
		{"InvalidArgument", codes.InvalidArgument, http.StatusBadRequest, "invalid_argument"},
		{"Unauthenticated", codes.Unauthenticated, http.StatusUnauthorized, "unauthenticated"},
		{"PermissionDenied", codes.PermissionDenied, http.StatusForbidden, "permission_denied"},
		{"ResourceExhausted", codes.ResourceExhausted, http.StatusTooManyRequests, "rate_limit_exceeded"},
		{"Unavailable", codes.Unavailable, http.StatusServiceUnavailable, "service_unavailable"},
		{"DeadlineExceeded", codes.DeadlineExceeded, http.StatusGatewayTimeout, "timeout"},
		{"Unknown", codes.Unknown, http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := ClientMock{
				err: status.Error(tt.grpcCode, "test error"),
			}
			handler := NewCartHandler(mockClient, 5*time.Second)

			req := &AddItemRequestDTO{ProductID: 1, Quantity: 2}
			reqBytes, _ := json.Marshal(req)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/items", bytes.NewReader(reqBytes))

			ctx := context.WithValue(request.Context(), "user_id", int64(1))
			request = request.WithContext(ctx)

			handler.AddItem(recorder, request)

			if recorder.Code != tt.expectedHTTP {
				t.Errorf("Expected status code %d, got %d", tt.expectedHTTP, recorder.Code)
			}

			var response ErrorResponse
			json.NewDecoder(recorder.Body).Decode(&response)
			if response.Code != tt.expectedCode {
				t.Errorf("Expected error code '%s', got '%s'", tt.expectedCode, response.Code)
			}
		})
	}
}

func TestUpdateQuantity_Success(t *testing.T) {
	clientMock := ClientMock{
		cart: &pb.Cart{
			UserId: 1,
			Cart: []*pb.CartItem{
				{ProductId: 1, Quantity: 10}, // Updated quantity
			},
		},
		err: nil,
	}

	handler := NewCartHandler(clientMock, 5*time.Second)
	req := &UpdateQuantityRequestDTO{Quantity: 10}
	reqBytes, _ := json.Marshal(req)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", "/items/1", bytes.NewReader(reqBytes))

	// Add user_id to context and URL param
	ctx := context.WithValue(request.Context(), "user_id", int64(1))
	ctx = context.WithValue(ctx, "request_id", "test-request-123")
	request = request.WithContext(ctx)

	// Mock chi.URLParam by using chi's context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("product_id", "1")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

	handler.UpdateQuantity(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.Code)
	}

	var response pb.Cart
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Cart[0].Quantity != 10 {
		t.Errorf("Expected quantity 10, got %d", response.Cart[0].Quantity)
	}
}

func TestUpdateQuantity_InvalidProductID(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	tests := []struct {
		name      string
		productID string
	}{
		{"non-numeric product_id", "abc"},
		{"zero product_id", "0"},
		{"negative product_id", "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &UpdateQuantityRequestDTO{Quantity: 5}
			reqBytes, _ := json.Marshal(req)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("PUT", "/items/"+tt.productID, bytes.NewReader(reqBytes))

			ctx := context.WithValue(request.Context(), "user_id", int64(1))
			request = request.WithContext(ctx)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("product_id", tt.productID)
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			handler.UpdateQuantity(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, recorder.Code)
			}
		})
	}
}

func TestUpdateQuantity_InvalidQuantity(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	tests := []struct {
		name     string
		quantity int32
	}{
		{"zero quantity", 0},
		{"negative quantity", -1},
		{"quantity too high", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &UpdateQuantityRequestDTO{Quantity: tt.quantity}
			reqBytes, _ := json.Marshal(req)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("PUT", "/items/1", bytes.NewReader(reqBytes))

			ctx := context.WithValue(request.Context(), "user_id", int64(1))
			request = request.WithContext(ctx)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("product_id", "1")
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			handler.UpdateQuantity(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, recorder.Code)
			}

			var response ErrorResponse
			json.NewDecoder(recorder.Body).Decode(&response)
			if response.Code != "invalid_quantity" {
				t.Errorf("Expected error code 'invalid_quantity', got '%s'", response.Code)
			}
		})
	}
}

func TestRemoveItem_Success(t *testing.T) {
	clientMock := ClientMock{
		cart: &pb.Cart{
			UserId: 1,
			Cart:   []*pb.CartItem{}, // Item removed
		},
		err: nil,
	}

	handler := NewCartHandler(clientMock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("DELETE", "/items/1", nil)

	ctx := context.WithValue(request.Context(), "user_id", int64(1))
	ctx = context.WithValue(ctx, "request_id", "test-request-123")
	request = request.WithContext(ctx)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("product_id", "1")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

	handler.RemoveItem(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.Code)
	}

	var response pb.Cart
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Cart) != 0 {
		t.Errorf("Expected empty cart, got %d items", len(response.Cart))
	}
}

func TestRemoveItem_InvalidProductID(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	tests := []struct {
		name      string
		productID string
	}{
		{"non-numeric product_id", "abc"},
		{"zero product_id", "0"},
		{"negative product_id", "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("DELETE", "/items/"+tt.productID, nil)

			ctx := context.WithValue(request.Context(), "user_id", int64(1))
			request = request.WithContext(ctx)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("product_id", tt.productID)
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			handler.RemoveItem(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, recorder.Code)
			}

			var response ErrorResponse
			json.NewDecoder(recorder.Body).Decode(&response)
			if response.Code != "invalid_product_id" {
				t.Errorf("Expected error code 'invalid_product_id', got '%s'", response.Code)
			}
		})
	}
}

func TestRemoveItem_Unauthorized(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("DELETE", "/items/1", nil)
	// No user_id in context

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("product_id", "1")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

	handler.RemoveItem(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestClearCart_Success(t *testing.T) {
	clientMock := ClientMock{
		cart: &pb.Cart{
			UserId: 1,
			Cart:   []*pb.CartItem{}, // Empty cart
		},
		err: nil,
	}

	handler := NewCartHandler(clientMock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("DELETE", "/", nil)

	ctx := context.WithValue(request.Context(), "user_id", int64(1))
	ctx = context.WithValue(ctx, "request_id", "test-request-123")
	request = request.WithContext(ctx)

	handler.ClearCart(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.Code)
	}

	var response pb.Cart
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Cart) != 0 {
		t.Errorf("Expected empty cart, got %d items", len(response.Cart))
	}

	if response.UserId != 1 {
		t.Errorf("Expected user_id 1, got %d", response.UserId)
	}
}

func TestClearCart_Unauthorized(t *testing.T) {
	clientMock := ClientMock{cart: &pb.Cart{}, err: nil}
	handler := NewCartHandler(clientMock, 5*time.Second)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("DELETE", "/", nil)
	// No user_id in context

	handler.ClearCart(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, recorder.Code)
	}

	var response ErrorResponse
	json.NewDecoder(recorder.Body).Decode(&response)
	if response.Code != "unauthorized" {
		t.Errorf("Expected error code 'unauthorized', got '%s'", response.Code)
	}
}

func TestClearCart_GRPCError(t *testing.T) {
	clientMock := ClientMock{
		cart: nil,
		err:  status.Error(codes.Internal, "database error"),
	}

	handler := NewCartHandler(clientMock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("DELETE", "/", nil)

	ctx := context.WithValue(request.Context(), "user_id", int64(1))
	request = request.WithContext(ctx)

	handler.ClearCart(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, recorder.Code)
	}

	var response ErrorResponse
	json.NewDecoder(recorder.Body).Decode(&response)
	if response.Code != "internal_error" {
		t.Errorf("Expected error code 'internal_error', got '%s'", response.Code)
	}
}
