package store

import (
	"errors"

	"github.com/fjod/go_cart/inventory-service/internal/domain"
)

// Common errors returned by the store
var (
	ErrProductNotFound     = errors.New("product not found")
	ErrInsufficientStock   = errors.New("insufficient stock")
	ErrReservationNotFound = errors.New("reservation not found")
	ErrReservationExpired  = errors.New("reservation has expired")
	ErrInvalidStatus       = errors.New("invalid reservation status for this operation")
)

// InventoryStore defines the interface for inventory storage operations
type InventoryStore interface {
	// GetStock returns stock information for the given product IDs
	GetStock(productIDs []int64) ([]domain.StockInfo, error)

	// Reserve creates a new reservation, reducing available stock
	// Returns the created reservation or an error if insufficient stock
	Reserve(checkoutID string, items []domain.ReservationItem) (*domain.Reservation, error)

	// Confirm finalizes a reservation, permanently deducting stock
	// Can only be called on reservations with status "reserved"
	Confirm(reservationID string) error

	// Release cancels a reservation, returning stock to available pool
	// Can only be called on reservations with status "reserved"
	Release(reservationID string) error

	// SetStock sets the stock level for a product (used for initialization)
	SetStock(productID int64, quantity int32) error

	// Close shuts down the store and any background processes
	Close() error
}
