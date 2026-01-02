package repository

import (
	"context"

	"github.com/fjod/go_cart/cart-service/internal/domain"
)

// CartRepository defines the interface for cart data operations
// Consumers define this interface, not the MongoDB implementation
type CartRepository interface {
	GetCart(ctx context.Context, userID string) (*domain.Cart, error)
	UpsertCart(ctx context.Context, cart *domain.Cart) error
	AddItem(ctx context.Context, userID string, item domain.CartItem) error
	UpdateItemQuantity(ctx context.Context, userID string, productID int64, quantity int) error
	RemoveItem(ctx context.Context, userID string, productID int64) error
	DeleteCart(ctx context.Context, userID string) error
}
