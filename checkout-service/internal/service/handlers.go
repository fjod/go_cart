package service

import (
	"time"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
	inventorypb "github.com/fjod/go_cart/inventory-service/pkg/proto"
	paymentpb "github.com/fjod/go_cart/payment-service/pkg/proto"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
)

type CartHandler struct {
	cartClient cartpb.CartServiceClient
	timeout    time.Duration
}

func NewCartHandler(cartClient cartpb.CartServiceClient, timeout time.Duration) *CartHandler {
	return &CartHandler{
		cartClient: cartClient,
		timeout:    timeout,
	}
}

type ProductHandler struct {
	productClient productpb.ProductServiceClient
	timeout       time.Duration
}

func NewProductHandler(productClient productpb.ProductServiceClient, timeout time.Duration) *ProductHandler {
	return &ProductHandler{
		productClient: productClient,
		timeout:       timeout,
	}
}

type InventoryHandler struct {
	inventoryClient inventorypb.InventoryServiceClient
	timeout         time.Duration
}

func NewInventoryHandler(inventoryClient inventorypb.InventoryServiceClient, timeout time.Duration) *InventoryHandler {
	return &InventoryHandler{
		inventoryClient: inventoryClient,
		timeout:         timeout,
	}
}

type PaymentHandler struct {
	paymentClient paymentpb.PaymentServiceClient
	timeout       time.Duration
}

func NewPaymentHandler(paymentClient paymentpb.PaymentServiceClient, timeout time.Duration) *PaymentHandler {
	return &PaymentHandler{
		paymentClient: paymentClient,
		timeout:       timeout,
	}
}
