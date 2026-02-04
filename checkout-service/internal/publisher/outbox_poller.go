package publisher

import (
	"context"
	"time"

	r "github.com/fjod/go_cart/checkout-service/internal/repository"
)

type OutboxPoller struct {
	timeout time.Duration
	tick    time.Duration
	repo    r.RepoInterface
}

func NewOutboxPoller(repo r.RepoInterface) *OutboxPoller {
	return &OutboxPoller{time.Second * 5, time.Second * 1, repo}
}

func (p *OutboxPoller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.processUnpublishedEvents(ctx)
			p.recoverStuckSessions(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (p *OutboxPoller) processUnpublishedEvents(ctx context.Context) {
	// TODO: implement
}

func (p *OutboxPoller) recoverStuckSessions(ctx context.Context) {
	// TODO: implement
}
