package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	"github.com/google/uuid"
)

func (s *CheckoutServiceImpl) InitiateCheckout(
	ctx context.Context,
	request *d.CheckoutRequest) (*d.CheckoutResponse, error) {

	// check session by idempotency key from repository
	existingSessionId, status, err := s.repo.GetCheckoutSessionByIdempotencyKey(ctx, request.IdempotencyKey)
	if err != nil && !errors.Is(err, r.ErrIdempotencyKeyNotFound) {
		return nil, fmt.Errorf("failed to check idempotency: %w", err)
	}

	if existingSessionId != nil {
		// This checkout already exists!
		// Return the cached result (could be COMPLETED, FAILED, or IN_PROGRESS)
		log.Printf("Duplicate request detected idempotency_key = %v with checkout_id = %v and status = %v", request.IdempotencyKey, *existingSessionId, status)

		return &d.CheckoutResponse{
			CheckoutID: existingSessionId,
			Status:     status,
		}, nil
	}

	snapshot, snapshotJSON, err2 := s.getCart(ctx, request)
	if err2 != nil {
		return nil, err2
	}

	sessionID := uuid.New().String()
	session := &r.CheckoutSession{
		ID:                     sessionID,
		UserID:                 fmt.Sprintf("%d", request.UserID),
		CartSnapshot:           snapshotJSON,
		Status:                 d.CheckoutStatusInitiated,
		IdempotencyKey:         request.IdempotencyKey,
		InventoryReservationID: nil,
		PaymentID:              nil,
		TotalAmount:            fmt.Sprintf("%.2f", snapshot.TotalAmount),
		Currency:               snapshot.Currency,
	}

	if err := s.repo.CreateCheckoutSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create checkout session: %w", err)
	}

	newStatus := d.CheckoutStatusInitiated
	return &d.CheckoutResponse{
		CheckoutID: &sessionID,
		Status:     &newStatus,
	}, nil
}
