package grpc

import (
	"context"
	"testing"
	"time"

	c "github.com/fjod/go_cart/cart-service/internal/cache"
	r "github.com/fjod/go_cart/cart-service/internal/repository"
	s "github.com/fjod/go_cart/cart-service/internal/service"
	pb "github.com/fjod/go_cart/cart-service/pkg/proto"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (r.CartRepository, func()) {
	ctx := context.Background()

	// Start MongoDB container
	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err)

	// Get connection string
	uri, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	// Connect to MongoDB
	db, err := r.ConnectMongoDB(ctx, uri, "testdb")
	require.NoError(t, err)

	// Create repository
	repo := r.NewMongoRepository(db)

	cleanup := func() {
		if err := mongoContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}

	return repo, cleanup
}

func setupRedis(t *testing.T) (*c.RedisCache, func()) {
	ctx := context.Background()
	redisC, err := testcontainers.Run(
		ctx, "redis:latest",
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("6379/tcp"),
			wait.ForLog("Ready to accept connections"),
		),
	)
	require.NoError(t, err)

	cleanup := func() {
		testcontainers.CleanupContainer(t, redisC)
	}

	endpoint, err := redisC.Endpoint(ctx, "")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: endpoint,
	})
	cache := c.NewRedisCache(client)
	return cache, cleanup
}

func TestAddItemToCart_Success(t *testing.T) {
	repo, cancelDb := setupTestDB(t)
	defer cancelDb()
	cache, cancelRedis := setupRedis(t)
	defer cancelRedis()

	service := s.NewCartService(repo, cache)

	mockProductClient := &mockProductServiceClient{
		getProductResp: &productpb.GetProductResponse{
			Product: &productpb.Product{
				Id:    1,
				Name:  "Test Product",
				Price: 99.99,
				Stock: 10, // Sufficient stock
			},
		},
	}

	server := NewCartServiceServer(service, mockProductClient)
	ret, err := server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    123,
		ProductId: 1,
		Quantity:  5,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	t.Logf("Received cart response: %v", ret.Cart)
	l := len(ret.Cart.Cart)
	assert.Equal(t, l, 1)
	for _, item := range ret.Cart.Cart {
		t.Logf("Item ID: %d, Quantity: %d", item.ProductId, item.Quantity)
		assert.Equal(t, item.ProductId, int64(1))
		assert.Equal(t, item.Quantity, int32(5))
	}
}

func TestGetCart_Integration(t *testing.T) {
	repo, cancelDb := setupTestDB(t)
	defer cancelDb()
	cache, cancelRedis := setupRedis(t)
	defer cancelRedis()

	service := s.NewCartService(repo, cache)
	mockProductClient := &mockProductServiceClient{
		getProductResp: &productpb.GetProductResponse{
			Product: &productpb.Product{
				Id:    1,
				Name:  "Test Product",
				Price: 99.99,
				Stock: 10,
			},
		},
	}
	server := NewCartServiceServer(service, mockProductClient)

	// First add some items to the cart
	_, err := server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    456,
		ProductId: 1,
		Quantity:  3,
	})
	require.NoError(t, err)

	// Add another product
	mockProductClient.getProductResp.Product.Id = 2
	_, err = server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    456,
		ProductId: 2,
		Quantity:  7,
	})
	require.NoError(t, err)

	// Now retrieve the cart
	ret, err := server.GetCart(context.Background(), &pb.GetCartRequest{
		UserId: 456,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	assert.Equal(t, 2, len(ret.Cart.Cart))
	t.Logf("Retrieved cart with %d items", len(ret.Cart.Cart))
}

func TestUpdateQuantity_Integration(t *testing.T) {
	repo, cancelDb := setupTestDB(t)
	defer cancelDb()
	cache, cancelRedis := setupRedis(t)
	defer cancelRedis()

	service := s.NewCartService(repo, cache)
	mockProductClient := &mockProductServiceClient{
		getProductResp: &productpb.GetProductResponse{
			Product: &productpb.Product{
				Id:    1,
				Name:  "Test Product",
				Price: 99.99,
				Stock: 50,
			},
		},
	}
	server := NewCartServiceServer(service, mockProductClient)

	// First add an item
	_, err := server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    789,
		ProductId: 1,
		Quantity:  5,
	})
	require.NoError(t, err)

	// Small delay to let async cache operations complete
	// This works around a race condition between async cache invalidation
	// and subsequent GetCart calls (TODO: fix by making cache invalidation synchronous)
	time.Sleep(50 * time.Millisecond)

	// Update the quantity
	ret, err := server.UpdateQuantity(context.Background(), &pb.UpdateQuantityRequest{
		UserId:    789,
		ProductId: 1,
		Quantity:  15,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	assert.Equal(t, 1, len(ret.Cart.Cart))
	assert.Equal(t, int32(15), ret.Cart.Cart[0].Quantity)
	t.Logf("Updated quantity to %d", ret.Cart.Cart[0].Quantity)
}

func TestRemoveItem_Integration(t *testing.T) {
	repo, cancelDb := setupTestDB(t)
	defer cancelDb()
	cache, cancelRedis := setupRedis(t)
	defer cancelRedis()

	service := s.NewCartService(repo, cache)
	mockProductClient := &mockProductServiceClient{
		getProductResp: &productpb.GetProductResponse{
			Product: &productpb.Product{
				Id:    1,
				Name:  "Test Product",
				Price: 99.99,
				Stock: 50,
			},
		},
	}
	server := NewCartServiceServer(service, mockProductClient)

	// Add two items
	_, err := server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    111,
		ProductId: 1,
		Quantity:  5,
	})
	require.NoError(t, err)

	mockProductClient.getProductResp.Product.Id = 2
	_, err = server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    111,
		ProductId: 2,
		Quantity:  3,
	})
	require.NoError(t, err)

	// Remove first item
	ret, err := server.RemoveItem(context.Background(), &pb.RemoveItemRequest{
		UserId:    111,
		ProductId: 1,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	assert.Equal(t, 1, len(ret.Cart.Cart))
	assert.Equal(t, int64(2), ret.Cart.Cart[0].ProductId)
	t.Logf("Removed item, %d items remaining", len(ret.Cart.Cart))
}

func TestClearCart_Integration(t *testing.T) {
	repo, cancelDb := setupTestDB(t)
	defer cancelDb()
	cache, cancelRedis := setupRedis(t)
	defer cancelRedis()

	service := s.NewCartService(repo, cache)
	mockProductClient := &mockProductServiceClient{
		getProductResp: &productpb.GetProductResponse{
			Product: &productpb.Product{
				Id:    1,
				Name:  "Test Product",
				Price: 99.99,
				Stock: 50,
			},
		},
	}
	server := NewCartServiceServer(service, mockProductClient)

	// Add items
	_, err := server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    222,
		ProductId: 1,
		Quantity:  5,
	})
	require.NoError(t, err)

	mockProductClient.getProductResp.Product.Id = 2
	_, err = server.AddItem(context.Background(), &pb.AddCartItemRequest{
		UserId:    222,
		ProductId: 2,
		Quantity:  3,
	})
	require.NoError(t, err)

	// Clear the cart
	ret, err := server.ClearCart(context.Background(), &pb.ClearCartRequest{
		UserId: 222,
	})

	require.NoError(t, err)
	assert.NotNil(t, ret)
	assert.Equal(t, 0, len(ret.Cart.Cart))
	assert.Equal(t, int64(222), ret.Cart.UserId)
	t.Logf("Cart cleared successfully")
}
