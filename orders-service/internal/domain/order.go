package domain

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderStatusConfirmed  OrderStatus = "CONFIRMED"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusShipped    OrderStatus = "SHIPPED"
	OrderStatusDelivered  OrderStatus = "DELIVERED"
)

type OrderItem struct {
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"price"`
}

type Order struct {
	ID          uuid.UUID
	CheckoutID  uuid.UUID
	UserID      string
	TotalAmount float64
	Currency    string
	Status      OrderStatus
	Items       []OrderItem
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
