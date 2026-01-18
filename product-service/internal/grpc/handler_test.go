package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/fjod/go_cart/product-service/internal/domain"
	grpcHandler "github.com/fjod/go_cart/product-service/internal/grpc"
	pb "github.com/fjod/go_cart/product-service/pkg/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Mock repository for testing
type mockRepository struct {
	products []*domain.Product
	err      error
}

func (m *mockRepository) GetAllProducts(context.Context) ([]*domain.Product, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.products, m.err
}

func (m *mockRepository) GetProduct(_ context.Context, id int64) (*domain.Product, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, p := range m.products {
		if p.ID == id {
			return p, m.err
		}
	}
	return nil, m.err
}

func (m *mockRepository) Close() error                 { return nil }
func (m *mockRepository) RunMigrations(_ string) error { return nil }

func TestGetAllProducts_Success(t *testing.T) {
	mockRepo := &mockRepository{
		products: []*domain.Product{
			{
				ID:          1,
				Name:        "Test Product",
				Description: "Test Description",
				Price:       99.99,
				ImageURL:    "http://example.com/image.jpg",
				CreatedAt:   time.Now(),
			},
		},
	}

	server := grpcHandler.NewProductServiceServer(mockRepo)

	resp, err := server.GetProducts(context.Background(), &pb.GetProductsRequest{})

	if err != nil {
		t.Error("Expected no errors")
	}

	if resp == nil {
		t.Error("Expected response, got nil")
	}

	if len(resp.Products) != 1 {
		t.Errorf("Expected 1 product, got %d", len(resp.Products))
	}
}

func TestGetProduct_Success(t *testing.T) {
	mockRepo := &mockRepository{
		products: []*domain.Product{
			{
				ID:          1,
				Name:        "Test Product",
				Description: "Test Description",
				Price:       99.99,
				ImageURL:    "http://example.com/image.jpg",
				CreatedAt:   time.Now(),
			},
		},
	}

	server := grpcHandler.NewProductServiceServer(mockRepo)

	resp, err := server.GetProduct(context.Background(), &pb.GetProductRequest{Id: int64(1)})

	if err != nil {
		t.Error("Expected no errors")
	}

	if resp == nil {
		t.Error("Expected response, got nil")
	}

	assert.Equal(t, "Test Product", resp.Product.Name)
}

func TestGetProduct_NotFound(t *testing.T) {
	mockRepo := &mockRepository{
		products: []*domain.Product{
			{
				ID:          1,
				Name:        "Test Product",
				Description: "Test Description",
				Price:       99.99,
				ImageURL:    "http://example.com/image.jpg",
				CreatedAt:   time.Now(),
			},
		},
		err: status.Error(codes.NotFound, "product not found"),
	}

	server := grpcHandler.NewProductServiceServer(mockRepo)

	resp, err := server.GetProduct(context.Background(), &pb.GetProductRequest{Id: int64(2)})

	assert.NotNil(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, status.Code(err), codes.NotFound)
}
