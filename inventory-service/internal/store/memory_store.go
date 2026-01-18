package store

import (
	"sync"
	"time"

	"github.com/fjod/go_cart/inventory-service/internal/domain"
	"github.com/google/uuid"
)

const (
	// ReservationTTL is how long a reservation is valid before auto-expiring
	ReservationTTL = 5 * time.Minute

	// CleanupInterval is how often the background cleanup runs
	CleanupInterval = 30 * time.Second
)

// MemoryStore implements InventoryStore with in-memory storage
type MemoryStore struct {
	mu           sync.RWMutex
	stocks       map[int64]*domain.StockInfo    // productID -> stock info
	reservations map[string]*domain.Reservation // reservationID -> reservation

	stopCleanup chan struct{}
	wg          sync.WaitGroup
}

// NewMemoryStore creates a new in-memory inventory store
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		stocks:       make(map[int64]*domain.StockInfo),
		reservations: make(map[string]*domain.Reservation),
		stopCleanup:  make(chan struct{}),
	}

	// Start background cleanup goroutine
	s.wg.Add(1)
	go s.cleanupLoop()

	return s
}

// cleanupLoop periodically checks and expires old reservations
func (s *MemoryStore) cleanupLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.expireReservations()
		case <-s.stopCleanup:
			return
		}
	}
}

// expireReservations finds and expires all reservations past their TTL
func (s *MemoryStore) expireReservations() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, reservation := range s.reservations {
		if reservation.Status == domain.StatusReserved && reservation.IsExpired() {
			reservation.Status = domain.StatusExpired
			for _, item := range reservation.Items {
				s.stocks[item.ProductID].Reserved -= item.Quantity
			}
		}
	}
}

// GetStock returns stock information for the given product IDs
func (s *MemoryStore) GetStock(productIDs []int64) ([]domain.StockInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.StockInfo, 0, len(productIDs))
	for _, id := range productIDs {
		if stock, exists := s.stocks[id]; exists {
			result = append(result, *stock)
		}
	}
	return result, nil
}

// Reserve creates a new reservation for checkout
func (s *MemoryStore) Reserve(checkoutID string, items []domain.ReservationItem) (*domain.Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First pass: validate all items have sufficient stock
	for _, item := range items {
		stock, exists := s.stocks[item.ProductID]
		if !exists {
			return nil, ErrProductNotFound
		}
		if stock.Available() < item.Quantity {
			return nil, ErrInsufficientStock
		}
	}

	// Second pass: reserve stock for all items
	for _, item := range items {
		s.stocks[item.ProductID].Reserved += item.Quantity
	}

	// Create the reservation
	now := time.Now()
	reservation := &domain.Reservation{
		ID:         uuid.New().String(),
		CheckoutID: checkoutID,
		Items:      items,
		Status:     domain.StatusReserved,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ReservationTTL),
	}

	s.reservations[reservation.ID] = reservation
	return reservation, nil
}

// Confirm finalizes a reservation after successful payment
func (s *MemoryStore) Confirm(reservationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	reservation, exists := s.reservations[reservationID]
	if !exists {
		return ErrReservationNotFound
	}

	if reservation.Status != domain.StatusReserved {
		return ErrInvalidStatus
	}

	if reservation.IsExpired() {
		return ErrReservationExpired
	}

	// Deduct from total stock (reserved already holds the quantity)
	for _, item := range reservation.Items {
		stock := s.stocks[item.ProductID]
		stock.Total -= item.Quantity
		stock.Reserved -= item.Quantity
	}

	reservation.Status = domain.StatusConfirmed
	return nil
}

// Release cancels a reservation on payment failure
func (s *MemoryStore) Release(reservationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	reservation, exists := s.reservations[reservationID]
	if !exists {
		return ErrReservationNotFound
	}

	if reservation.Status != domain.StatusReserved {
		return ErrInvalidStatus
	}

	// Return reserved stock to available pool
	for _, item := range reservation.Items {
		s.stocks[item.ProductID].Reserved -= item.Quantity
	}

	reservation.Status = domain.StatusReleased
	return nil
}

// SetStock sets the stock level for a product
func (s *MemoryStore) SetStock(productID int64, quantity int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stocks[productID] = &domain.StockInfo{
		ProductID: productID,
		Total:     quantity,
		Reserved:  0,
	}
	return nil
}

// Close stops the background cleanup and waits for it to finish
func (s *MemoryStore) Close() error {
	close(s.stopCleanup)
	s.wg.Wait()
	return nil
}
