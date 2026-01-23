package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
	d "github.com/fjod/go_cart/checkout-service/domain"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
)

// CartSnapshotItem represents an item in the cart snapshot with price captured at checkout time
type CartSnapshotItem struct {
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int32   `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Subtotal    float64 `json:"subtotal"`
}

// CartSnapshot represents the full cart state at checkout time
type CartSnapshot struct {
	Items       []CartSnapshotItem `json:"items"`
	TotalAmount float64            `json:"total_amount"`
	Currency    string             `json:"currency"`
	CapturedAt  time.Time          `json:"captured_at"`
}

func (s *CheckoutServiceImpl) getCart(ctx context.Context, request *d.CheckoutRequest) (*CartSnapshot, []byte, error) {
	cartRequest := &cartpb.GetCartRequest{
		UserId: request.UserID,
	}

	cartContext, cancel := context.WithTimeout(ctx, s.cart.timeout)
	defer cancel() // releases resources if GetCart completes before timeout elapses
	cart, e := s.cart.cartClient.GetCart(cartContext, cartRequest)
	if e != nil {
		return nil, nil, fmt.Errorf("failed to get cart: %w", e)
	}

	cartItems := cart.GetCart().Cart
	if len(cartItems) == 0 {
		return nil, nil, ErrEmptyCart
	}

	// Fetch prices and build cart snapshot
	snapshot, err := s.buildCartSnapshot(ctx, cartItems)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build cart snapshot: %w", err)
	}

	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal cart snapshot: %w", err)
	}
	return snapshot, snapshotJSON, nil
}

// buildCartSnapshot fetches current prices from product service and creates a snapshot
func (s *CheckoutServiceImpl) buildCartSnapshot(ctx context.Context, cartItems []*cartpb.CartItem) (*CartSnapshot, error) {
	snapshot := &CartSnapshot{
		Items:      make([]CartSnapshotItem, 0, len(cartItems)),
		Currency:   "USD",
		CapturedAt: time.Now(),
	}

	var totalAmount float64

	// TODO: add GetProducts request to fetch multiple products at once, use context.WithTimeout for request context
	for _, item := range cartItems {
		product, err := s.product.productClient.GetProduct(ctx, &productpb.GetProductRequest{
			Id: item.ProductId,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get product %d: %w", item.ProductId, err)
		}

		subtotal := product.Product.Price * float64(item.Quantity)

		snapshot.Items = append(snapshot.Items, CartSnapshotItem{
			ProductID:   item.ProductId,
			ProductName: product.Product.Name,
			Quantity:    item.Quantity,
			UnitPrice:   product.Product.Price,
			Subtotal:    subtotal,
		})

		totalAmount += subtotal
	}

	snapshot.TotalAmount = totalAmount
	return snapshot, nil
}
