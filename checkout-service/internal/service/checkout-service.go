package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	r "github.com/fjod/go_cart/checkout-service/internal/repository"
)

type CheckoutRequest struct {
	UserID         int64
	IdempotencyKey string
}

type CheckoutResponse struct {
	CheckoutID *string
	Status     *string
}

type CheckoutService interface {
	InitiateCheckout(ctx context.Context, request *CheckoutRequest) (*CheckoutResponse, error)
}

type CheckoutServiceImpl struct {
	repo r.RepoInterface
}

func NewCheckoutService(repo r.RepoInterface) *CheckoutServiceImpl {
	return &CheckoutServiceImpl{repo: repo}
}

func (s *CheckoutServiceImpl) InitiateCheckout(
	ctx context.Context,
	request *CheckoutRequest) (*CheckoutResponse, error) {

	// check session by idempotency key from repository
	existingSessionId, status, err := s.repo.GetCheckoutSessionByIdempotencyKey(ctx, request.IdempotencyKey)
	if err != nil && !errors.Is(err, r.ErrIdempotencyKeyNotFound) {
		return nil, fmt.Errorf("failed to check idempotency: %w", err)
	}

	if existingSessionId != nil {
		// This checkout already exists!
		// Return the cached result (could be COMPLETED, FAILED, or IN_PROGRESS)
		log.Printf("Duplicate request detected idempotency_key %v with checkout_id %v and status %v", request.IdempotencyKey, existingSessionId, status)

		return &CheckoutResponse{
			CheckoutID: existingSessionId,
			Status:     status,
		}, nil
	}

	return &CheckoutResponse{ // stub
	}, nil
}
