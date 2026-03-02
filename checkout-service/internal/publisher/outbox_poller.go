package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	pk "github.com/fjod/go_cart/pkg/tracing"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

type OutboxPoller struct {
	timeout      time.Duration
	eventTick    time.Duration
	recoveryTick time.Duration
	repo         r.RepoInterface
	writer       *kafka.Writer
	logger       *slog.Logger
}

func NewOutboxPoller(repo r.RepoInterface, log *slog.Logger, brokers ...string) *OutboxPoller {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  "checkout-outbox",
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	return &OutboxPoller{time.Second * 5, time.Second, time.Second * 5, repo, w, log}
}

func (p *OutboxPoller) Run(ctx context.Context) {
	eventTicker := time.NewTicker(p.eventTick)
	recoveryTicker := time.NewTicker(p.recoveryTick)
	defer eventTicker.Stop()
	defer recoveryTicker.Stop()
	for {
		select {
		case <-eventTicker.C:
			p.processUnpublishedEvents(ctx)
		case <-recoveryTicker.C:
			p.recoverStuckSessions(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (p *OutboxPoller) processUnpublishedEvents(ctx context.Context) {
	events, err := p.repo.GetUnprocessedEvents(ctx, 100)
	if err != nil {
		p.logger.Error("failed to fetch unprocessed events", "error", err)
		return
	}

	for _, event := range events {
		errPublish := p.publishToKafka(ctx, event)
		if errPublish != nil {
			p.logger.Error("failed to publish event", "event_id", event.ID, "error", errPublish)
			continue
		}

		errMark := p.repo.MarkEventAsProcessed(ctx, event.ID)
		if errMark != nil {
			p.logger.Error("failed to mark event as processed", "event_id", event.ID, "error", errMark)
			continue
		}
	}
}

func (p *OutboxPoller) recoverStuckSessions(ctx context.Context) {
	// stuck session is when the checkout status is PAYMENT_COMPLETED but there is no outbox event for it.
	sessions, err := p.repo.GetStuckSessions(ctx)
	if err != nil {
		p.logger.Error("failed to get stuck sessions", "error", err)
		return
	}
	for _, session := range sessions {
		p.logger.Info("recovering stuck session", "session_id", session.ID)

		var s d.CartSnapshot
		if err := json.Unmarshal(session.CartSnapshot, &s); err != nil {
			p.logger.Error("failed to unmarshal cart snapshot", "session_id", session.ID, "error", err)
			continue
		}

		payload := map[string]interface{}{
			"checkout_id":  session.ID,
			"user_id":      session.UserID,
			"items":        s.Items,
			"total_amount": s.TotalAmount,
			"currency":     s.Currency,
			"completed_at": session.UpdatedAt,
		}

		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			p.logger.Error("failed to marshal checkout payload", "session_id", session.ID, "error", err)
			continue
		}

		completedStatus := d.CheckoutStatusCompleted
		err = p.repo.CompleteCheckoutSession(ctx, &session.ID, payloadJSON, &completedStatus)
		if err != nil {
			p.logger.Error("failed to complete checkout in recovery", "session_id", session.ID, "error", err)
			continue
		}

		p.logger.Info("session recovered", "session_id", session.ID)
	}
}

func (p *OutboxPoller) publishToKafka(ctx context.Context, event *r.OutboxEvent) error {
	messageName := "checkout.processed"
	tr := otel.Tracer("kafka")
	spanCtx, messageSpan := tr.Start(ctx, fmt.Sprintf("kafka - publish - %s", messageName))
	defer messageSpan.End()

	headers := []kafka.Header{
		{Key: "event_type", Value: []byte(event.EventType)},
	}
	for k, v := range pk.Inject(spanCtx) {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}

	msg := kafka.Message{
		Key:     []byte(event.AggregateId),
		Value:   event.Payload,
		Headers: headers,
	}
	return p.writer.WriteMessages(ctx, msg)
}
