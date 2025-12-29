package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/fjod/go_cart/product-service/internal/domain"
	grpcHandler "github.com/fjod/go_cart/product-service/internal/grpc"
	pb "github.com/fjod/go_cart/product-service/pkg/proto"
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
				Stock:       10,
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
