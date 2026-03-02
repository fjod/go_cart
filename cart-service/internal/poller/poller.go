package poller

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	c "github.com/fjod/go_cart/cart-service/internal/cache"
	r "github.com/fjod/go_cart/cart-service/internal/repository"
	"github.com/fjod/go_cart/pkg/logger"
	pk "github.com/fjod/go_cart/pkg/tracing"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

type Poller struct {
	repo   r.CartRepository
	reader *kafka.Reader
	cache  *c.RedisCache
	logger *slog.Logger
}

func NewPoller(repo r.CartRepository, cache *c.RedisCache, log *slog.Logger, brokers ...string) *Poller {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       "checkout-outbox",
		GroupID:     "cart-service-consumer",
		MaxBytes:    10e6, // 10MB
		StartOffset: kafka.FirstOffset,
	})
	return &Poller{repo, reader, cache, log}
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
		p.logger.Error("error closing kafka reader", "error", err)
	}
}

func (p *Poller) getMessagesAndEmptyCart(ctx context.Context) {
	m, err := p.reader.ReadMessage(ctx)
	if err != nil {
		p.logger.Error("error reading kafka message", "error", err)
		return
	}

	mapping := make(map[string]string)
	for _, h := range m.Headers {
		mapping[h.Key] = string(h.Value)
	}
	ctx = pk.Extract(ctx, mapping)
	ctx, span := otel.Tracer("cart").Start(ctx, "kafka - consume - checkout.processed")
	defer span.End()

	log := logger.WithContext(p.logger, ctx)

	var payload map[string]interface{}
	if errUnMarshal := json.Unmarshal(m.Value, &payload); errUnMarshal != nil {
		log.Error("error parsing kafka message payload", "error", errUnMarshal)
		return
	}
	userID, ok := payload["user_id"].(string)
	if !ok {
		log.Warn("missing or invalid user_id in checkout event payload")
		return
	}

	errDelete := p.repo.DeleteCart(ctx, userID)
	if errDelete != nil && !errors.Is(errDelete, r.ErrCartNotFound) {
		log.Error("failed to delete cart after checkout", "user_id", userID, "error", errDelete)
	}

	errCacheDelete := p.cache.Delete(ctx, userID)
	if errCacheDelete != nil {
		log.Error("failed to invalidate cart cache after checkout", "user_id", userID, "error", errCacheDelete)
	}
}
