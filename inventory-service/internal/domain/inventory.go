package domain

import "time"

// ReservationStatus represents the state of a stock reservation
type ReservationStatus string

const (
	StatusReserved  ReservationStatus = "reserved"
	StatusConfirmed ReservationStatus = "confirmed"
	StatusReleased  ReservationStatus = "released"
	StatusExpired   ReservationStatus = "expired"
)

// ReservationItem represents a single product reservation within a reservation
type ReservationItem struct {
	ProductID int64
	Quantity  int32
}

// Reservation represents a stock reservation made during checkout
type Reservation struct {
	ID         string
	CheckoutID string
	Items      []ReservationItem
	Status     ReservationStatus
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// IsExpired checks if the reservation has expired
func (r *Reservation) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

// StockInfo contains stock information for a product
type StockInfo struct {
	ProductID int64
	Total     int32 // Total stock in inventory
	Reserved  int32 // Currently reserved (pending checkout)
}

// Available returns the available stock (total - reserved)
func (s StockInfo) Available() int32 {
	return s.Total - s.Reserved
}
