package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	d "github.com/fjod/go_cart/checkout-service/domain"
)

func (s *CheckoutServiceImpl) complete(ctx context.Context, checkoutId string, status d.CheckoutStatus, snapshot *d.CartSnapshot, userId string) error {

	if !d.CanTransitionTo(status, d.CheckoutStatusCompleted) {
		return IllegalTransitionError
	}
	payload := map[string]interface{}{
		"checkout_id":  checkoutId,
		"user_id":      userId,
		"items":        snapshot.Items,
		"total_amount": snapshot.TotalAmount,
		"currency":     snapshot.Currency,
		"completed_at": time.Now(),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal checkout payload: %w", err)
	}

	completedStatus := d.CheckoutStatusCompleted
	err = s.repo.CompleteCheckoutSession(ctx, &checkoutId, payloadJSON, &completedStatus)
	if err != nil {
		return err
	}
	return nil
}
