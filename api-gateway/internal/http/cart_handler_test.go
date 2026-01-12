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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ClientMock struct {
	cart *pb.Cart
	err  error
}

func (c ClientMock) AddItem(
	ctx context.Context,
	in *pb.AddCartItemRequest,
	opts ...grpc.CallOption) (*pb.AddCartItemResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &pb.AddCartItemResponse{
		Cart: c.cart,
	}, nil
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
