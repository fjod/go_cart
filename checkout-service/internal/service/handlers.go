package service

import (
	"time"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
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
