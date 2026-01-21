package repository

import (
	"context"
	"testing"
	"time"

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
	statusExp := "pending"
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
