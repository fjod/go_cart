package cache

import (
	"context"
	"errors"

	"github.com/fjod/go_cart/cart-service/internal/domain"
)

type CartCache interface {
	Get(ctx context.Context, userID string) (*domain.Cart, error)
	Set(ctx context.Context, userID string, cart *domain.Cart) error
	Delete(ctx context.Context, userID string) error
}

var ErrCacheMiss = errors.New("cache miss")
