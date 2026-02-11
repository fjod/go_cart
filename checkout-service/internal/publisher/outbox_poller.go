package publisher

import (
	"context"
	"encoding/json"
	"log"
	"time"

	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	"github.com/segmentio/kafka-go"
)

type OutboxPoller struct {
	timeout      time.Duration
	eventTick    time.Duration
	recoveryTick time.Duration
	repo         r.RepoInterface
	writer       *kafka.Writer
}

func NewOutboxPoller(repo r.RepoInterface, brokers ...string) *OutboxPoller {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  "checkout-outbox",
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	return &OutboxPoller{time.Second * 5, time.Second, time.Second * 5, repo, w}
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
		log.Printf("failed to fetch events %v", err)
		return
	}

	for _, event := range events {
		errPublish := p.publishToKafka(ctx, event)
		if errPublish != nil {
			log.Printf("failed to publish event id = %v with error %v", event.ID, errPublish)
			continue
		}

		errMark := p.repo.MarkEventAsProcessed(ctx, event.ID)
		if errMark != nil {
			log.Printf("failed to mark event  as processed id = %v with error %v", event.ID, errMark)
			continue
		}
	}
}

func (p *OutboxPoller) recoverStuckSessions(ctx context.Context) {
	// stuck session is when the checkout status is PAYMENT_COMPLETED but there is no outbox event for it.
	sessions, err := p.repo.GetStuckSessions(ctx)
	if err != nil {
		log.Printf("failed to get stuck sessions: %v", err)
		return
	}
	for _, session := range sessions {
		log.Printf("recovering stuck session: %v", session.ID)

		var s d.CartSnapshot
		if err := json.Unmarshal(session.CartSnapshot, &s); err != nil {
			log.Printf("failed to unmarshal cart snapshot for session %v: %v", session.ID, err)
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
			log.Printf("failed to marshal checkout payload in poller: %v", err)
			continue
		}

		completedStatus := d.CheckoutStatusCompleted
		err = p.repo.CompleteCheckoutSession(ctx, &session.ID, payloadJSON, &completedStatus)
		if err != nil {
			log.Printf("failed to complete checkout in poller: %v", err)
		}

		log.Printf("session recovered: %v", session.ID)
	}
}

func (p *OutboxPoller) publishToKafka(ctx context.Context, event *r.OutboxEvent) error {
	msg := kafka.Message{
		Key:   []byte(event.AggregateId), // checkout_id for ordering
		Value: event.Payload,             // Already JSON from database
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
		},
	}

	err := p.writer.WriteMessages(ctx, msg)
	return err
}
