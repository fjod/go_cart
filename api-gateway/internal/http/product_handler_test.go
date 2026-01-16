package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pb "github.com/fjod/go_cart/product-service/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProductClientMock struct {
	products []*pb.Product
	err      error
}

func (m ProductClientMock) GetProducts(context.Context, *pb.GetProductsRequest, ...grpc.CallOption) (*pb.GetProductsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &pb.GetProductsResponse{
		Products: m.products,
	}, nil
}

func (m ProductClientMock) GetProduct(context.Context, *pb.GetProductRequest, ...grpc.CallOption) (*pb.GetProductResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return first product if available
	if len(m.products) > 0 {
		return &pb.GetProductResponse{
			Product: m.products[0],
		}, nil
	}
	return nil, status.Error(codes.NotFound, "product not found")
}

func TestGetProducts_Success(t *testing.T) {
	clientMock := ProductClientMock{
		products: []*pb.Product{
			{
				Id:          1,
				Name:        "Laptop",
				Description: "A powerful laptop",
				Price:       1299.99,
				ImageUrl:    "https://example.com/laptop.jpg",
			},
			{
				Id:          2,
				Name:        "Mouse",
				Description: "Wireless mouse",
				Price:       29.99,
				ImageUrl:    "https://example.com/mouse.jpg",
			},
		},
		err: nil,
	}

	handler := NewProductHandler(clientMock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)

	handler.Get(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.Code)
	}

	var response ProductsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Products) != 2 {
		t.Errorf("Expected 2 products, got %d", len(response.Products))
	}

	// Verify first product
	if response.Products[0].ID != 1 {
		t.Errorf("Expected product ID 1, got %d", response.Products[0].ID)
	}
	if response.Products[0].Name != "Laptop" {
		t.Errorf("Expected product name 'Laptop', got '%s'", response.Products[0].Name)
	}
	if response.Products[0].Price != 1299.99 {
		t.Errorf("Expected product price 1299.99, got %f", response.Products[0].Price)
	}

	// Verify second product
	if response.Products[1].ID != 2 {
		t.Errorf("Expected product ID 2, got %d", response.Products[1].ID)
	}
	if response.Products[1].Name != "Mouse" {
		t.Errorf("Expected product name 'Mouse', got '%s'", response.Products[1].Name)
	}
}

func TestGetProducts_EmptyList(t *testing.T) {
	clientMock := ProductClientMock{
		products: []*pb.Product{},
		err:      nil,
	}

	handler := NewProductHandler(clientMock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)

	handler.Get(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.Code)
	}

	var response ProductsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Products) != 0 {
		t.Errorf("Expected 0 products, got %d", len(response.Products))
	}
}

func TestGetProducts_GRPCErrors(t *testing.T) {
	tests := []struct {
		name         string
		grpcCode     codes.Code
		expectedHTTP int
		expectedCode string
	}{
		{"Unavailable", codes.Unavailable, http.StatusServiceUnavailable, "service_unavailable"},
		{"DeadlineExceeded", codes.DeadlineExceeded, http.StatusGatewayTimeout, "timeout"},
		{"Internal", codes.Internal, http.StatusInternalServerError, "internal_error"},
		{"Unknown", codes.Unknown, http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientMock := ProductClientMock{
				err: status.Error(tt.grpcCode, "test error"),
			}

			handler := NewProductHandler(clientMock, 5*time.Second)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "/", nil)

			handler.Get(recorder, request)

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

func TestGetProducts_AllFields(t *testing.T) {
	clientMock := ProductClientMock{
		products: []*pb.Product{
			{
				Id:          42,
				Name:        "Test Product",
				Description: "A test product description",
				Price:       99.99,
				ImageUrl:    "https://example.com/test.jpg",
			},
		},
		err: nil,
	}

	handler := NewProductHandler(clientMock, 5*time.Second)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)

	handler.Get(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.Code)
	}

	var response ProductsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	product := response.Products[0]
	if product.ID != 42 {
		t.Errorf("Expected ID 42, got %d", product.ID)
	}
	if product.Name != "Test Product" {
		t.Errorf("Expected name 'Test Product', got '%s'", product.Name)
	}
	if product.Description != "A test product description" {
		t.Errorf("Expected description 'A test product description', got '%s'", product.Description)
	}
	if product.Price != 99.99 {
		t.Errorf("Expected price 99.99, got %f", product.Price)
	}
	if product.ImageURL != "https://example.com/test.jpg" {
		t.Errorf("Expected image URL 'https://example.com/test.jpg', got '%s'", product.ImageURL)
	}
}
