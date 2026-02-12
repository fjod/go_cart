package repository

import (
	"context"
	"testing"
	"time"

	"github.com/fjod/go_cart/orders-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (*Repository, func()) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	host, err := pgContainer.Host(ctx)
	require.NoError(t, err)

	port, err := pgContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	creds := &Credentials{
		Host:              host,
		Port:              port.Int(),
		User:              "testuser",
		Password:          "testpass",
		DBName:            "testdb",
		MigrationsDirPath: "./migrations",
	}

	repo, err := NewRepository(creds)
	require.NoError(t, err)

	err = repo.RunMigrations(creds)
	require.NoError(t, err)

	cleanup := func() {
		repo.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}

	return repo, cleanup
}

func newTestOrder(checkoutID uuid.UUID) *domain.Order {
	return &domain.Order{
		ID:          uuid.New(),
		CheckoutID:  checkoutID,
		UserID:      "user-123",
		TotalAmount: 99.99,
		Currency:    "USD",
		Status:      domain.OrderStatusConfirmed,
		Items: []domain.OrderItem{
			{ProductID: 1, ProductName: "Laptop", Quantity: 1, Price: 99.99},
		},
	}
}

func TestCreateOrder_Success(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	checkoutID := uuid.New()
	order := newTestOrder(checkoutID)

	err := repo.CreateOrder(ctx, order)
	require.NoError(t, err)

	fetched, err := repo.GetOrderByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, fetched.ID)
	assert.Equal(t, order.CheckoutID, fetched.CheckoutID)
	assert.Equal(t, order.UserID, fetched.UserID)
	assert.Equal(t, order.TotalAmount, fetched.TotalAmount)
	assert.Equal(t, order.Currency, fetched.Currency)
	assert.Equal(t, order.Status, fetched.Status)
	assert.Len(t, fetched.Items, 1)
	assert.Equal(t, order.Items[0].ProductID, fetched.Items[0].ProductID)
}

func TestCreateOrder_DuplicateCheckout(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	checkoutID := uuid.New()

	order1 := newTestOrder(checkoutID)
	err := repo.CreateOrder(ctx, order1)
	require.NoError(t, err)

	order2 := newTestOrder(checkoutID) // same checkoutID
	err = repo.CreateOrder(ctx, order2)
	assert.ErrorIs(t, err, ErrDuplicateCheckout)
}

func TestGetOrderByID_Found(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	checkoutID := uuid.New()
	order := newTestOrder(checkoutID)

	err := repo.CreateOrder(ctx, order)
	require.NoError(t, err)

	fetched, err := repo.GetOrderByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, fetched.ID)
}

func TestGetOrderByID_NotFound(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, err := repo.GetOrderByID(ctx, uuid.New())
	assert.ErrorIs(t, err, ErrOrderNotFound)
}

func TestListOrdersByUserID(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-list-test"

	order1 := &domain.Order{
		ID:          uuid.New(),
		CheckoutID:  uuid.New(),
		UserID:      userID,
		TotalAmount: 10.00,
		Currency:    "USD",
		Status:      domain.OrderStatusConfirmed,
		Items:       []domain.OrderItem{{ProductID: 1, ProductName: "Mouse", Quantity: 1, Price: 10.00}},
	}
	require.NoError(t, repo.CreateOrder(ctx, order1))

	// Small sleep to ensure different created_at timestamps
	time.Sleep(10 * time.Millisecond)

	order2 := &domain.Order{
		ID:          uuid.New(),
		CheckoutID:  uuid.New(),
		UserID:      userID,
		TotalAmount: 20.00,
		Currency:    "USD",
		Status:      domain.OrderStatusConfirmed,
		Items:       []domain.OrderItem{{ProductID: 2, ProductName: "Keyboard", Quantity: 1, Price: 20.00}},
	}
	require.NoError(t, repo.CreateOrder(ctx, order2))

	orders, err := repo.ListOrdersByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, orders, 2)

	// Verify ordered by created_at DESC (order2 created last, should be first)
	assert.Equal(t, order2.ID, orders[0].ID)
	assert.Equal(t, order1.ID, orders[1].ID)
}
