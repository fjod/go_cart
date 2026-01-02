package repository

import (
	"context"
	"testing"
	"time"

	"github.com/fjod/go_cart/cart-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
)

func setupTestDB(t *testing.T) (CartRepository, func()) {
	ctx := context.Background()

	// Start MongoDB container
	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err)

	// Get connection string
	uri, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	// Connect to MongoDB
	db, err := ConnectMongoDB(ctx, uri, "testdb")
	require.NoError(t, err)

	// Create repository
	repo := NewMongoRepository(db)

	// Create indexes
	mongoRepo := repo.(*mongoRepository)
	err = mongoRepo.CreateIndexes(ctx)
	require.NoError(t, err)

	cleanup := func() {
		if err := mongoContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}

	return repo, cleanup
}

func TestGetCart_NotFound(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	cart, err := repo.GetCart(ctx, "nonexistent")

	assert.ErrorIs(t, err, ErrCartNotFound)
	assert.Nil(t, cart)
}

func TestAddItem_NewCart(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	userID := "user123"
	ctx := context.Background()
	item := domain.CartItem{
		ProductID: 1,
		Quantity:  3,
	}
	err := repo.AddItem(ctx, userID, item)
	require.NoError(t, err)

	cart, err := repo.GetCart(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, cart.UserID)
	assert.Len(t, cart.Items, 1)
	assert.Equal(t, int64(1), cart.Items[0].ProductID)
	assert.Equal(t, 3, cart.Items[0].Quantity)
}

func TestAddItem_ExistingItem_UpdatesQuantity(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user123"

	// Add item first time
	item1 := domain.CartItem{ProductID: 1, Quantity: 2}
	err := repo.AddItem(ctx, userID, item1)
	require.NoError(t, err)

	// Add same item again with different quantity
	item2 := domain.CartItem{ProductID: 1, Quantity: 5}
	err = repo.AddItem(ctx, userID, item2)
	require.NoError(t, err)

	// Verify quantity was updated, not added
	cart, err := repo.GetCart(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, cart.Items, 1)
	assert.Equal(t, 5, cart.Items[0].Quantity)
}

func TestUpdateItemQuantity(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user123"

	// Add item
	item := domain.CartItem{ProductID: 1, Quantity: 2}
	err := repo.AddItem(ctx, userID, item)
	require.NoError(t, err)

	// Update quantity
	err = repo.UpdateItemQuantity(ctx, userID, 1, 10)
	require.NoError(t, err)

	// Verify
	cart, err := repo.GetCart(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 10, cart.Items[0].Quantity)
}

func TestRemoveItem(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user123"

	// Add two items
	err := repo.AddItem(ctx, userID, domain.CartItem{ProductID: 1, Quantity: 2})
	require.NoError(t, err)
	err = repo.AddItem(ctx, userID, domain.CartItem{ProductID: 2, Quantity: 3})
	require.NoError(t, err)

	// Remove one item
	err = repo.RemoveItem(ctx, userID, 1)
	require.NoError(t, err)

	// Verify only one item remains
	cart, err := repo.GetCart(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, cart.Items, 1)
	assert.Equal(t, int64(2), cart.Items[0].ProductID)
}

func TestDeleteCart(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user123"

	// Add item to create cart
	err := repo.AddItem(ctx, userID, domain.CartItem{ProductID: 1, Quantity: 2})
	require.NoError(t, err)

	// Delete cart
	err = repo.DeleteCart(ctx, userID)
	require.NoError(t, err)

	// Verify cart is gone
	_, err = repo.GetCart(ctx, userID)
	assert.ErrorIs(t, err, ErrCartNotFound)
}

func TestContextCancellation(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure context is cancelled

	_, err := repo.GetCart(ctx, "user123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}
