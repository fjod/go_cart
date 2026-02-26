package service

import (
	"context"

	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	"go.opentelemetry.io/otel"
	t "go.opentelemetry.io/otel/trace"
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
	tracer    t.Tracer
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
		tracer:    otel.Tracer("checkout"),
	}
}
