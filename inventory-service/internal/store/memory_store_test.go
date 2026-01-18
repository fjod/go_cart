package store

import (
	"sync"
	"testing"
	"time"

	"github.com/fjod/go_cart/inventory-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStore(t *testing.T) *MemoryStore {
	store := NewMemoryStore()
	t.Cleanup(func() { store.Close() })
	return store
}

func TestMemoryStore_SetStock_And_GetStock(t *testing.T) {
	store := setupStore(t)

	// Set stock for products
	require.NoError(t, store.SetStock(1, 100))
	require.NoError(t, store.SetStock(2, 200))

	// Get stock
	stocks, err := store.GetStock([]int64{1, 2, 3})
	require.NoError(t, err)

	// Should return only existing products
	assert.Len(t, stocks, 2)

	stockMap := make(map[int64]domain.StockInfo)
	for _, s := range stocks {
		stockMap[s.ProductID] = s
	}

	assert.Equal(t, int32(100), stockMap[1].Total)
	assert.Equal(t, int32(100), stockMap[1].Available())
	assert.Equal(t, int32(200), stockMap[2].Total)
}

func TestMemoryStore_Reserve_Success(t *testing.T) {
	store := setupStore(t)
	require.NoError(t, store.SetStock(1, 100))
	require.NoError(t, store.SetStock(2, 50))

	items := []domain.ReservationItem{
		{ProductID: 1, Quantity: 10},
		{ProductID: 2, Quantity: 5},
	}

	reservation, err := store.Reserve("checkout-123", items)
	require.NoError(t, err)

	assert.NotEmpty(t, reservation.ID)
	assert.Equal(t, "checkout-123", reservation.CheckoutID)
	assert.Equal(t, domain.StatusReserved, reservation.Status)
	assert.Len(t, reservation.Items, 2)
	assert.True(t, reservation.ExpiresAt.After(time.Now()))

	// Check stock was reserved
	stocks, _ := store.GetStock([]int64{1, 2})
	stockMap := make(map[int64]domain.StockInfo)
	for _, s := range stocks {
		stockMap[s.ProductID] = s
	}

	assert.Equal(t, int32(90), stockMap[1].Available())
	assert.Equal(t, int32(10), stockMap[1].Reserved)
	assert.Equal(t, int32(45), stockMap[2].Available())
}

func TestMemoryStore_Reserve_InsufficientStock(t *testing.T) {
	store := setupStore(t)
	require.NoError(t, store.SetStock(1, 10))

	items := []domain.ReservationItem{
		{ProductID: 1, Quantity: 20},
	}

	_, err := store.Reserve("checkout-123", items)
	assert.ErrorIs(t, err, ErrInsufficientStock)

	// Stock should be unchanged
	stocks, _ := store.GetStock([]int64{1})
	assert.Equal(t, int32(10), stocks[0].Available())
}

func TestMemoryStore_Reserve_ProductNotFound(t *testing.T) {
	store := setupStore(t)

	items := []domain.ReservationItem{
		{ProductID: 999, Quantity: 1},
	}

	_, err := store.Reserve("checkout-123", items)
	assert.ErrorIs(t, err, ErrProductNotFound)
}

func TestMemoryStore_Confirm_Success(t *testing.T) {
	store := setupStore(t)
	require.NoError(t, store.SetStock(1, 100))

	items := []domain.ReservationItem{
		{ProductID: 1, Quantity: 10},
	}

	reservation, _ := store.Reserve("checkout-123", items)

	err := store.Confirm(reservation.ID)
	require.NoError(t, err)

	// Stock should be permanently deducted
	stocks, _ := store.GetStock([]int64{1})
	assert.Equal(t, int32(90), stocks[0].Total)
	assert.Equal(t, int32(0), stocks[0].Reserved)
	assert.Equal(t, int32(90), stocks[0].Available())
}

func TestMemoryStore_Confirm_NotFound(t *testing.T) {
	store := setupStore(t)

	err := store.Confirm("nonexistent-id")
	assert.ErrorIs(t, err, ErrReservationNotFound)
}

func TestMemoryStore_Confirm_InvalidStatus(t *testing.T) {
	store := setupStore(t)
	require.NoError(t, store.SetStock(1, 100))

	items := []domain.ReservationItem{
		{ProductID: 1, Quantity: 10},
	}

	reservation, _ := store.Reserve("checkout-123", items)
	_ = store.Release(reservation.ID) // Release first

	err := store.Confirm(reservation.ID)
	assert.ErrorIs(t, err, ErrInvalidStatus)
}

func TestMemoryStore_Release_Success(t *testing.T) {
	store := setupStore(t)
	require.NoError(t, store.SetStock(1, 100))

	items := []domain.ReservationItem{
		{ProductID: 1, Quantity: 10},
	}

	reservation, _ := store.Reserve("checkout-123", items)

	err := store.Release(reservation.ID)
	require.NoError(t, err)

	// Stock should be returned to available
	stocks, _ := store.GetStock([]int64{1})
	assert.Equal(t, int32(100), stocks[0].Total)
	assert.Equal(t, int32(0), stocks[0].Reserved)
	assert.Equal(t, int32(100), stocks[0].Available())
}

func TestMemoryStore_Release_NotFound(t *testing.T) {
	store := setupStore(t)

	err := store.Release("nonexistent-id")
	assert.ErrorIs(t, err, ErrReservationNotFound)
}

func TestMemoryStore_ConcurrentReservations(t *testing.T) {
	store := setupStore(t)
	require.NoError(t, store.SetStock(1, 100))

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Try to reserve 20 units each, 10 times concurrently
	// Only 5 should succeed (100 / 20 = 5)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			items := []domain.ReservationItem{
				{ProductID: 1, Quantity: 20},
			}
			_, err := store.Reserve("checkout-"+string(rune('0'+id)), items)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, 5, successCount)

	// All stock should be reserved
	stocks, _ := store.GetStock([]int64{1})
	assert.Equal(t, int32(0), stocks[0].Available())
	assert.Equal(t, int32(100), stocks[0].Reserved)
}

func TestMemoryStore_ExpireReservations(t *testing.T) {
	store := setupStore(t)
	require.NoError(t, store.SetStock(1, 100))

	items := []domain.ReservationItem{
		{ProductID: 1, Quantity: 10},
	}

	reservation, _ := store.Reserve("checkout-123", items)

	// Manually expire the reservation by modifying ExpiresAt
	store.mu.Lock()
	store.reservations[reservation.ID].ExpiresAt = time.Now().Add(-1 * time.Minute)
	store.mu.Unlock()

	// Trigger expiration
	store.expireReservations()

	// Check reservation is expired
	store.mu.RLock()
	status := store.reservations[reservation.ID].Status
	store.mu.RUnlock()
	assert.Equal(t, domain.StatusExpired, status)

	// Stock should be returned
	stocks, _ := store.GetStock([]int64{1})
	assert.Equal(t, int32(100), stocks[0].Available())
	assert.Equal(t, int32(0), stocks[0].Reserved)
}
