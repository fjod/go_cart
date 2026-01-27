package service

import (
	"context"

	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
)

type CheckoutService interface {
	InitiateCheckout(ctx context.Context, request *d.CheckoutRequest) (*d.CheckoutResponse, error)
}

type CheckoutServiceImpl struct {
	repo      r.RepoInterface
	cart      *CartHandler
	product   *ProductHandler
	inventory *InventoryHandler
	payment   *PaymentHandler
}

func NewCheckoutService(
	repo r.RepoInterface,
	cart *CartHandler,
	product *ProductHandler,
	inventory *InventoryHandler,
	payment *PaymentHandler,
) *CheckoutServiceImpl {
	return &CheckoutServiceImpl{
		repo:      repo,
		cart:      cart,
		product:   product,
		inventory: inventory,
		payment:   payment,
	}
}
