package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

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
	logger *slog.Logger
}

func NewConsumer(repo repository.OrderRepository, log *slog.Logger, brokers ...string) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       "checkout-outbox",
		GroupID:     "orders-service",
		MaxBytes:    10e6, // 10MB
		StartOffset: kafka.FirstOffset,
	})
	return &Consumer{repo, reader, log}
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
		c.logger.Error("error closing kafka reader", "error", err)
	}
}

func (c *Consumer) processMessage(ctx context.Context) {
	m, err := c.reader.ReadMessage(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		c.logger.Error("error reading kafka message", "error", err)
		return
	}

	var event CheckoutCompletedEvent
	if err := json.Unmarshal(m.Value, &event); err != nil {
		c.logger.Error("error parsing kafka message payload", "error", err)
		return
	}

	checkoutID, err := uuid.Parse(event.CheckoutID)
	if err != nil {
		c.logger.Error("invalid checkout_id in event", "checkout_id", event.CheckoutID, "error", err)
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
			c.logger.Info("order already exists, skipping duplicate", "checkout_id", event.CheckoutID)
			return
		}
		c.logger.Error("failed to create order", "checkout_id", event.CheckoutID, "error", err)
		return
	}

	c.logger.Info("order created", "order_id", order.ID, "checkout_id", order.CheckoutID)
}
