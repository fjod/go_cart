package poller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	c "github.com/fjod/go_cart/cart-service/internal/cache"
	r "github.com/fjod/go_cart/cart-service/internal/repository"
	"github.com/segmentio/kafka-go"
)

type Poller struct {
	repo   r.CartRepository
	reader *kafka.Reader
	cache  *c.RedisCache
}

func NewPoller(repo r.CartRepository, cache *c.RedisCache, brokers ...string) *Poller {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    "checkout-outbox",
		GroupID:  "cart-service-consumer",
		MaxBytes: 10e6, // 10MB
	})
	return &Poller{repo, reader, cache}
}

func (p *Poller) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		p.getMessagesAndEmptyCart(ctx)
	}
}

func (p *Poller) Close() {
	err := p.reader.Close()
	if err != nil {
		fmt.Printf("error closing reader: %v\n", err)
	}
}

func (p *Poller) getMessagesAndEmptyCart(ctx context.Context) {
	m, err := p.reader.ReadMessage(ctx)
	if err != nil {
		fmt.Printf("error reading message: %v\n", err)
		return
	}

	var payload map[string]interface{}
	if errUnMarshal := json.Unmarshal(m.Value, &payload); errUnMarshal != nil {
		fmt.Printf("error parsing message: %v\n", errUnMarshal)
		return
	}
	userID, ok := payload["user_id"].(string)
	if !ok {
		fmt.Println("missing or invalid user_id")
		return
	}

	errDelete := p.repo.DeleteCart(ctx, userID)
	if errDelete != nil && !errors.Is(errDelete, r.ErrCartNotFound) {
		fmt.Printf("failed to delete cart: %v\n", errDelete)
	}

	errCacheDelete := p.cache.Delete(ctx, userID)
	if errCacheDelete != nil {
		fmt.Printf("failed to delete cache: %v\n", errCacheDelete)
	}
}
