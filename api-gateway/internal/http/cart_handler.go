package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	pb "github.com/fjod/go_cart/cart-service/pkg/proto"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type CartHandler struct {
	cartClient pb.CartServiceClient
	timeout    time.Duration
}

func NewCartHandler(cartClient pb.CartServiceClient, timeout time.Duration) *CartHandler {
	return &CartHandler{
		cartClient: cartClient,
		timeout:    timeout,
	}
}

type AddItemRequestDTO struct {
	ProductID int64 `json:"product_id"`
	Quantity  int32 `json:"quantity"`
}

type UpdateQuantityRequestDTO struct {
	Quantity int32 `json:"quantity"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	userID := getUserIDFromContext(r.Context())
	if userID == 0 {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing user authentication")
		return
	}

	// Parse request body
	var req AddItemRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	// Validate request
	if req.ProductID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid_product_id", "product_id must be positive")
		return
	}
	if req.Quantity <= 0 || req.Quantity > 99 {
		respondError(w, http.StatusBadRequest, "invalid_quantity", "quantity must be between 1 and 99")
		return
	}

	// Propagate metadata
	ctx = metadata.AppendToOutgoingContext(ctx, "user-id", fmt.Sprint(userID), "request-id", getRequestID(r.Context()))

	// Call gRPC service
	resp, err := h.cartClient.AddItem(ctx, &pb.AddCartItemRequest{
		UserId:    userID,
		ProductId: req.ProductID,
		Quantity:  req.Quantity,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp.Cart)
}

func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	userID := getUserIDFromContext(r.Context())
	if userID == 0 {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing user authentication")
		return
	}
	// Propagate metadata
	ctx = metadata.AppendToOutgoingContext(ctx, "user-id", fmt.Sprint(userID), "request-id", getRequestID(r.Context()))

	// Call gRPC service
	resp, err := h.cartClient.GetCart(ctx, &pb.GetCartRequest{
		UserId: userID,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp.Cart)
}

func getUserIDFromContext(ctx context.Context) int64 {
	if userID, ok := ctx.Value("user_id").(int64); ok {
		return userID
	}
	return 0
}

func getRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return ""
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, ErrorResponse{
		Error:   message,
		Code:    code,
		Details: "",
	})
}

func handleGRPCError(w http.ResponseWriter, err error) {
	// Convert gRPC status codes to HTTP status codes
	st, ok := status.FromError(err)
	if !ok {
		respondError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	var httpStatus int
	var code string

	switch st.Code().String() {
	case "InvalidArgument":
		httpStatus = http.StatusBadRequest
		code = "invalid_argument"
	case "NotFound":
		httpStatus = http.StatusNotFound
		code = "not_found"
	case "AlreadyExists":
		httpStatus = http.StatusConflict
		code = "already_exists"
	case "Unauthenticated":
		httpStatus = http.StatusUnauthorized
		code = "unauthenticated"
	case "PermissionDenied":
		httpStatus = http.StatusForbidden
		code = "permission_denied"
	case "ResourceExhausted":
		httpStatus = http.StatusTooManyRequests
		code = "rate_limit_exceeded"
	case "Unavailable":
		httpStatus = http.StatusServiceUnavailable
		code = "service_unavailable"
	case "DeadlineExceeded":
		httpStatus = http.StatusGatewayTimeout
		code = "timeout"
	default:
		httpStatus = http.StatusInternalServerError
		code = "internal_error"
	}

	respondError(w, httpStatus, code, st.Message())
}

func (h *CartHandler) UpdateQuantity(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	userID := getUserIDFromContext(r.Context())
	if userID == 0 {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing user authentication")
		return
	}

	// Get product_id from URL path
	productIDStr := chi.URLParam(r, "product_id")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid_product_id", "product_id must be a positive integer")
		return
	}

	// Parse request body
	var req UpdateQuantityRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	// Validate request
	if req.Quantity <= 0 || req.Quantity > 99 {
		respondError(w, http.StatusBadRequest, "invalid_quantity", "quantity must be between 1 and 99")
		return
	}

	// Propagate metadata
	ctx = metadata.AppendToOutgoingContext(ctx, "user-id", fmt.Sprint(userID), "request-id", getRequestID(r.Context()))

	// Call gRPC service
	resp, err := h.cartClient.UpdateQuantity(ctx, &pb.UpdateQuantityRequest{
		UserId:    userID,
		ProductId: productID,
		Quantity:  req.Quantity,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp.Cart)
}

func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	userID := getUserIDFromContext(r.Context())
	if userID == 0 {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing user authentication")
		return
	}

	// Get product_id from URL path
	productIDStr := chi.URLParam(r, "product_id")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid_product_id", "product_id must be a positive integer")
		return
	}

	// Propagate metadata
	ctx = metadata.AppendToOutgoingContext(ctx, "user-id", fmt.Sprint(userID), "request-id", getRequestID(r.Context()))

	// Call gRPC service
	resp, err := h.cartClient.RemoveItem(ctx, &pb.RemoveItemRequest{
		UserId:    userID,
		ProductId: productID,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp.Cart)
}

func (h *CartHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	userID := getUserIDFromContext(r.Context())
	if userID == 0 {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing user authentication")
		return
	}

	// Propagate metadata
	ctx = metadata.AppendToOutgoingContext(ctx, "user-id", fmt.Sprint(userID), "request-id", getRequestID(r.Context()))

	// Call gRPC service
	resp, err := h.cartClient.ClearCart(ctx, &pb.ClearCartRequest{
		UserId: userID,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp.Cart)
}
