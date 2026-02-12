package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fjod/go_cart/orders-service/internal/domain"
	"github.com/fjod/go_cart/orders-service/internal/repository"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// eventItem mirrors the Kafka payload item shape from the checkout-service outbox.
// The checkout-service publishes prices as "unit_price" (from CartSnapshotItem),
// which differs from domain.OrderItem's "price" json tag.
type eventItem struct {
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"unit_price"`
}

type CheckoutCompletedEvent struct {
	CheckoutID  string      `json:"checkout_id"`
	UserID      string      `json:"user_id"`
	Items       []eventItem `json:"items"`
	TotalAmount float64     `json:"total_amount"`
	Currency    string      `json:"currency"`
}

type Consumer struct {
	repo   repository.OrderRepository
	reader *kafka.Reader
}

func NewConsumer(repo repository.OrderRepository, brokers ...string) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    "checkout-outbox",
		GroupID:  "orders-service",
		MaxBytes: 10e6, // 10MB
	})
	return &Consumer{repo, reader}
}

func (c *Consumer) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		c.processMessage(ctx)
	}
}

func (c *Consumer) Close() {
	err := c.reader.Close()
	if err != nil {
		fmt.Printf("error closing kafka reader: %v\n", err)
	}
}

func (c *Consumer) processMessage(ctx context.Context) {
	m, err := c.reader.ReadMessage(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		fmt.Printf("error reading message: %v\n", err)
		return
	}

	var event CheckoutCompletedEvent
	if err := json.Unmarshal(m.Value, &event); err != nil {
		fmt.Printf("error parsing message: %v\n", err)
		return
	}

	checkoutID, err := uuid.Parse(event.CheckoutID)
	if err != nil {
		fmt.Printf("invalid checkout_id %q: %v\n", event.CheckoutID, err)
		return
	}

	currency := event.Currency
	if currency == "" {
		currency = "USD"
	}

	items := make([]domain.OrderItem, len(event.Items))
	for i, item := range event.Items {
		items[i] = domain.OrderItem{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			Price:       item.Price,
		}
	}

	order := &domain.Order{
		ID:          uuid.New(),
		CheckoutID:  checkoutID,
		UserID:      event.UserID,
		TotalAmount: event.TotalAmount,
		Currency:    currency,
		Status:      domain.OrderStatusConfirmed,
		Items:       items,
	}

	if err := c.repo.CreateOrder(ctx, order); err != nil {
		if errors.Is(err, repository.ErrDuplicateCheckout) {
			fmt.Printf("order for checkout %s already exists, skipping\n", event.CheckoutID)
			return
		}
		fmt.Printf("failed to create order for checkout %s: %v\n", event.CheckoutID, err)
		return
	}

	fmt.Printf("order %s created for checkout %s\n", order.ID, order.CheckoutID)
}
