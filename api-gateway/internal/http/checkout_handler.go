package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	pb "github.com/fjod/go_cart/checkout-service/pkg/proto"
	"google.golang.org/grpc/metadata"
)

type CheckoutHandler struct {
	checkoutClient pb.CheckoutServiceClient
	timeout        time.Duration
}

func NewCheckoutHandler(client pb.CheckoutServiceClient, timeout time.Duration) *CheckoutHandler {
	return &CheckoutHandler{
		checkoutClient: client,
		timeout:        timeout,
	}
}

type InitiateCheckoutRequestDTO struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type CheckoutResponseDTO struct {
	CheckoutID string `json:"checkout_id"`
	Status     string `json:"status"`
}

// POST /api/v1/checkout
func (h *CheckoutHandler) InitiateCheckout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	userID := getUserIDFromContext(r.Context())
	if userID == 0 {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing user authentication")
		return
	}

	var req InitiateCheckoutRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.IdempotencyKey == "" {
		respondError(w, http.StatusBadRequest, "missing_idempotency_key",
			"idempotency_key is required")
		return
	}

	ctx = metadata.AppendToOutgoingContext(ctx,
		"user-id", fmt.Sprint(userID),
		"request-id", getRequestID(r.Context()))

	resp, err := h.checkoutClient.InitiateCheckout(ctx, &pb.InitiateCheckoutRequest{
		UserId:         userID,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, CheckoutResponseDTO{
		CheckoutID: resp.CheckoutId,
		Status:     mapProtoStatusToString(resp.Status),
	})
}

func mapProtoStatusToString(status pb.CheckoutStatus) string {
	switch status {
	case pb.CheckoutStatus_CHECKOUT_STATUS_INITIATED:
		return "INITIATED"
	case pb.CheckoutStatus_CHECKOUT_STATUS_INVENTORY_RESERVED:
		return "INVENTORY_RESERVED"
	case pb.CheckoutStatus_CHECKOUT_STATUS_PAYMENT_PENDING:
		return "PAYMENT_PENDING"
	case pb.CheckoutStatus_CHECKOUT_STATUS_PAYMENT_COMPLETED:
		return "PAYMENT_COMPLETED"
	case pb.CheckoutStatus_CHECKOUT_STATUS_COMPLETED:
		return "COMPLETED"
	case pb.CheckoutStatus_CHECKOUT_STATUS_FAILED:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}
