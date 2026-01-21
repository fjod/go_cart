package service

import (
	"context"
	"errors"
	"testing"

	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository implements r.RepoInterface for testing
type MockRepository struct {
	Key    *string
	Status *d.CheckoutStatus
	err    error
}

func (m *MockRepository) Close() error {
	return nil
}

func (m *MockRepository) RunMigrations(*r.Credentials) error {
	return nil
}

func (m *MockRepository) GetCheckoutSessionByIdempotencyKey(_ context.Context, key string) (*string, *d.CheckoutStatus, error) {
	return m.Key, m.Status, m.err
}

func TestInitiateCheckout_NewRequest(t *testing.T) {
	mock := &MockRepository{
		Key:    nil,
		Status: nil,
		err:    r.ErrIdempotencyKeyNotFound,
	}
	svc := NewCheckoutService(mock)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "new-key-12345",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// Currently returns stub response for new requests
	assert.Nil(t, resp.CheckoutID)
}

func TestInitiateCheckout_DuplicateRequest(t *testing.T) {
	existingID := "checkout-abc-123"
	existingStatus := d.CheckoutStatusCompleted

	mock := &MockRepository{
		Key:    &existingID,
		Status: &existingStatus,
		err:    nil,
	}
	svc := NewCheckoutService(mock)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "existing-key",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, existingID, *resp.CheckoutID)
	assert.Equal(t, existingStatus, *resp.Status)
}

func TestInitiateCheckout_RepositoryError(t *testing.T) {
	mock := &MockRepository{
		Key:    nil,
		Status: nil,
		err:    errors.New("repository error"),
	}
	svc := NewCheckoutService(mock)

	req := &d.CheckoutRequest{
		UserID:         123,
		IdempotencyKey: "error-key",
	}

	resp, err := svc.InitiateCheckout(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to check idempotency")
}
