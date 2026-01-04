package repository_test

import (
	"context"
	"testing"
	"time"

	db "github.com/fjod/go_cart/product-service/internal/repository"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func setupTestDB(t *testing.T) *db.Repository {
	// Use in-memory database for tests
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test repository: %v", err)
	}

	// Run migrations
	if err := repo.RunMigrations("./migrations"); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return repo
}

func TestGetAllProducts_Returns5AfterMigrations(t *testing.T) {
	repo := setupTestDB(t)
	defer repo.Close()

	products, err := repo.GetAllProducts(context.Background())

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(products) != 5 { // Your migration inserts 5 products
		t.Errorf("Expected 5 products, got %d", len(products))
	}
}

func TestGetAllProducts_WithContext(t *testing.T) {
	repo := setupTestDB(t)
	defer repo.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()

	products, err := repo.GetAllProducts(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(products) != 5 { // Your migration inserts 5 products
		t.Errorf("Expected 5 products, got %d", len(products))
	}
}

func TestGetAllProducts_CancelledContext(t *testing.T) {
	repo := setupTestDB(t)
	defer repo.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetAllProducts(ctx)

	if (err == nil) || (err != nil && err.Error() != "failed to query products: context canceled") {
		t.Errorf("Expected 'failed to query products: context canceled' error, got %v", err)
	}
}

func TestGetProduct_ReturnsProduct(t *testing.T) {
	repo := setupTestDB(t)
	defer repo.Close()

	product, err := repo.GetProduct(context.Background(), 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if product == nil {
		t.Errorf("Received nil product by valid id")
	}
	t.Logf("Received product: %+v", *product)
}

func TestGetProduct_IncorrectId_ReturnsNil(t *testing.T) {
	repo := setupTestDB(t)
	defer repo.Close()

	product, err := repo.GetProduct(context.Background(), -1)

	if product != nil {
		t.Errorf("Expected a nil product for incorrect id %+v", *product)
	}

	isNil := product == nil
	t.Logf("Is returned product nil?  %t", isNil)
	assert.Equal(t, status.Code(err), codes.NotFound)
}
