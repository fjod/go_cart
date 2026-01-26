package repository

import (
	"context"
	"testing"
	"time"

	d "github.com/fjod/go_cart/checkout-service/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (*Repository, func()) {
	ctx := context.Background()

	// Start PostgreSQL container
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

	// Get connection details
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

	// Connect to database
	repo, err := NewRepository(creds)
	require.NoError(t, err)

	// Run migrations
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

func TestGetCheckoutSessionByIdempotencyKey_NotFound(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	id, status, err := repo.GetCheckoutSessionByIdempotencyKey(ctx, "nonexistent-key")

	assert.ErrorIs(t, err, ErrIdempotencyKeyNotFound)
	assert.Nil(t, id)
	assert.Nil(t, status)
}

func TestGetCheckoutSessionByIdempotencyKey_Found(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	query := `INSERT INTO checkout_sessions (id, user_id, cart_snapshot, idempotency_key,  status, total_amount, created_at, updated_at) 
               VALUES ($1, $2, $3, $4, $5, $6,  NOW(), NOW())`
	keyExp := "existing"
	statusExp := d.CheckoutStatusCompleted
	expId := uuid.New()
	ctx := context.Background()
	_, insertErr := repo.db.ExecContext(ctx, query,
		expId,     // id
		"userId",  // user_id
		"{}",      // cart_snapshot
		keyExp,    // idempotency_key
		statusExp, // status
		0)         // total_amount
	assert.NoError(t, insertErr)

	id, status, err := repo.GetCheckoutSessionByIdempotencyKey(ctx, keyExp)

	assert.NoError(t, err)
	assert.Equal(t, expId.String(), *id)
	assert.Equal(t, statusExp, *status)
}

func TestContextCancellation(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure context is cancelled

	_, _, err := repo.GetCheckoutSessionByIdempotencyKey(ctx, "any-key")
	assert.Error(t, err)
}

func TestCreateCheckoutSession_Success(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := uuid.New().String()
	session := &CheckoutSession{
		ID:             sessionID,
		UserID:         "user-123",
		CartSnapshot:   []byte(`{"items":[{"product_id":1,"quantity":2}]}`),
		IdempotencyKey: "idem-key-123",
		TotalAmount:    "99.99",
	}

	err := repo.CreateCheckoutSession(ctx, session)
	require.NoError(t, err)

	// Verify the session was created with correct status
	id, status, err := repo.GetCheckoutSessionByIdempotencyKey(ctx, "idem-key-123")
	require.NoError(t, err)
	assert.Equal(t, sessionID, *id)
	assert.Equal(t, d.CheckoutStatusInitiated, *status) // Always starts as INITIATED
}

func TestCreateCheckoutSession_DuplicateIdempotencyKey(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	session := &CheckoutSession{
		ID:             uuid.New().String(),
		UserID:         "user-123",
		CartSnapshot:   []byte(`{}`),
		IdempotencyKey: "duplicate-key",
		TotalAmount:    "50.00",
	}

	err := repo.CreateCheckoutSession(ctx, session)
	require.NoError(t, err)

	// Try to create another session with the same idempotency key
	session2 := &CheckoutSession{
		ID:             uuid.New().String(),
		UserID:         "user-456",
		CartSnapshot:   []byte(`{}`),
		IdempotencyKey: "duplicate-key", // Same key
		TotalAmount:    "75.00",
	}

	err = repo.CreateCheckoutSession(ctx, session2)
	assert.Error(t, err) // Should fail due to unique constraint
}

func TestUpdateCheckoutSession_Success(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := uuid.New().String()

	// First create a session
	session := &CheckoutSession{
		ID:             sessionID,
		UserID:         "user-123",
		CartSnapshot:   []byte(`{}`),
		IdempotencyKey: "update-test-key",
		TotalAmount:    "100.00",
	}
	err := repo.CreateCheckoutSession(ctx, session)
	require.NoError(t, err)

	// Update the status
	newStatus := d.CheckoutStatusPaymentCompleted
	err = repo.UpdateCheckoutSessionStatus(ctx, &sessionID, &newStatus)
	require.NoError(t, err)

	// Verify the status was updated
	_, status, err := repo.GetCheckoutSessionByIdempotencyKey(ctx, "update-test-key")
	require.NoError(t, err)
	assert.Equal(t, d.CheckoutStatusPaymentCompleted, *status)
}

func TestReserveItem_Success(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := uuid.New().String()

	// First create a session
	session := &CheckoutSession{
		ID:             sessionID,
		UserID:         "user-123",
		CartSnapshot:   []byte(`{}`),
		IdempotencyKey: "update-test-key",
		TotalAmount:    "100.00",
	}
	err := repo.CreateCheckoutSession(ctx, session)
	require.NoError(t, err)

	// Update the status
	newStatus := d.CheckoutStatusInventoryReserved
	reserveId := "reserve"
	err = repo.SetReservation(ctx, &sessionID, &newStatus, &reserveId)
	require.NoError(t, err)

	// Verify the status was updated
	_, status, err := repo.GetCheckoutSessionByIdempotencyKey(ctx, "update-test-key")
	require.NoError(t, err)
	assert.Equal(t, d.CheckoutStatusInventoryReserved, *status)

	reserveQuery := `select inventory_reservation_id from checkout_sessions where id = $1`
	ret := repo.db.QueryRow(reserveQuery, sessionID)
	var inventoryReservationID string
	require.NoError(t, ret.Scan(&inventoryReservationID))
	assert.Equal(t, "reserve", inventoryReservationID)
}

func TestUpdateCheckoutSession_StatusProgression(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	sessionID := uuid.New().String()

	// Create initial session (starts as INITIATED)
	session := &CheckoutSession{
		ID:             sessionID,
		UserID:         "user-789",
		CartSnapshot:   []byte(`{"items":[]}`),
		IdempotencyKey: "progression-key",
		TotalAmount:    "200.00",
	}
	err := repo.CreateCheckoutSession(ctx, session)
	require.NoError(t, err)

	// Progress through status transitions
	statusProgression := []d.CheckoutStatus{
		d.CheckoutStatusInventoryReserved,
		d.CheckoutStatusPaymentPending,
		d.CheckoutStatusPaymentCompleted,
		d.CheckoutStatusCompleted,
	}

	for _, expectedStatus := range statusProgression {
		err = repo.UpdateCheckoutSessionStatus(ctx, &sessionID, &expectedStatus)
		require.NoError(t, err)

		_, actualStatus, err := repo.GetCheckoutSessionByIdempotencyKey(ctx, "progression-key")
		require.NoError(t, err)
		assert.Equal(t, expectedStatus, *actualStatus)
	}
}
