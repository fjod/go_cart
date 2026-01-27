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

	reserveStatus := d.CheckoutStatusInitiated
	items := mapItemsToItemPointers(snapshot.Items)
	reserveId, reserveError := s.reserveInventory(ctx, sessionID, items, reserveStatus)
	if reserveError != nil {
		failedStatus := d.CheckoutStatusFailed
		err := s.repo.UpdateCheckoutSessionStatus(ctx, &sessionID, &failedStatus)
		if err != nil {
			return nil, fmt.Errorf("failed to set failed status: %w", err)
		}
		return &d.CheckoutResponse{
			CheckoutID: &sessionID,
			Status:     &failedStatus,
		}, fmt.Errorf("failed to reserve inventory: %w", reserveError)
	}

	reservedStatus := d.CheckoutStatusInventoryReserved
	payError := s.processPayment(ctx, sessionID, reservedStatus, session.TotalAmount)
	if payError != nil {
		failedStatus := d.CheckoutStatusFailed
		err := s.repo.UpdateCheckoutSessionStatus(ctx, &sessionID, &failedStatus)
		if err != nil {
			return nil, fmt.Errorf("failed to set failed status: %w", err)
		}

		// compensate inventory reservation on payment failure
		releaseError := s.releaseInventory(ctx, *reserveId)
		if releaseError != nil {
			return nil, fmt.Errorf("failed to release inventory: %w", releaseError)
		}

		return &d.CheckoutResponse{
			CheckoutID: &sessionID,
			Status:     &failedStatus,
		}, fmt.Errorf("failed to pay: %v", failedStatus)
	}

	returnStatus := d.CheckoutStatusCompleted // stub status until all steps of saga are implemented
	return &d.CheckoutResponse{
		CheckoutID: &sessionID,
		Status:     &returnStatus,
	}, nil
}

func mapItemsToItemPointers(input []CartSnapshotItem) []*CartSnapshotItem {
	result := make([]*CartSnapshotItem, len(input))
	for i, item := range input {
		result[i] = &item
	}
	return result
}
