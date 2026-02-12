package repository

import (
	"context"
	"errors"

	"github.com/fjod/go_cart/orders-service/internal/domain"
	"github.com/google/uuid"
)

var (
	ErrOrderNotFound     = errors.New("order not found")
	ErrDuplicateCheckout = errors.New("order for this checkout already exists")
)

type Credentials struct {
	Host              string
	Port              int
	User              string
	Password          string
	DBName            string
	MigrationsDirPath string
}

type OrderRepository interface {
	CreateOrder(ctx context.Context, order *domain.Order) error
	GetOrderByID(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	ListOrdersByUserID(ctx context.Context, userID string) ([]*domain.Order, error)
	RunMigrations(*Credentials) error
	Close() error
}
