package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	pb "github.com/fjod/go_cart/orders-service/pkg/proto"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/metadata"
)

type OrdersHandler struct {
	ordersClient pb.OrdersServiceClient
	timeout      time.Duration
}

func NewOrdersHandler(client pb.OrdersServiceClient, timeout time.Duration) *OrdersHandler {
	return &OrdersHandler{
		ordersClient: client,
		timeout:      timeout,
	}
}

type OrderItemDTO struct {
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int32   `json:"quantity"`
	Price       float64 `json:"price"`
}

type OrderResponseDTO struct {
	ID          string         `json:"id"`
	CheckoutID  string         `json:"checkout_id"`
	TotalAmount float64        `json:"total_amount"`
	Currency    string         `json:"currency"`
	Status      string         `json:"status"`
	Items       []OrderItemDTO `json:"items"`
	CreatedAt   string         `json:"created_at"`
}

// GET /api/v1/orders
func (h *OrdersHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	userID := getUserIDFromContext(r.Context())
	if userID == 0 {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing user authentication")
		return
	}

	ctx = metadata.AppendToOutgoingContext(ctx,
		"user-id", fmt.Sprint(userID),
		"request-id", getRequestID(r.Context()))

	resp, err := h.ordersClient.ListOrders(ctx, &pb.ListOrdersRequest{
		UserId: fmt.Sprint(userID),
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	dtos := make([]OrderResponseDTO, 0, len(resp.Orders))
	for _, o := range resp.Orders {
		dtos = append(dtos, convertProtoOrder(o))
	}

	respondJSON(w, http.StatusOK, dtos)
}

func convertProtoOrder(o *pb.Order) OrderResponseDTO {
	var dtoItems []OrderItemDTO
	if o.Items == nil {
		dtoItems = make([]OrderItemDTO, 0)
	} else {
		dtoItems = make([]OrderItemDTO, 0, len(o.Items))
		for _, item := range o.Items {
			orderItem := OrderItemDTO{
				ProductID:   item.ProductId,
				ProductName: item.ProductName,
				Quantity:    item.Quantity,
				Price:       item.Price,
			}
			dtoItems = append(dtoItems, orderItem)
		}
	}

	dto := OrderResponseDTO{
		ID:          o.Id,
		CheckoutID:  o.CheckoutId,
		TotalAmount: o.TotalAmount,
		Currency:    o.Currency,
		Status:      o.Status,
		Items:       dtoItems,
		CreatedAt:   o.CreatedAt,
	}
	return dto
}

// GET /api/v1/orders/{order_id}
func (h *OrdersHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	userID := getUserIDFromContext(r.Context())
	if userID == 0 {
		respondError(w, http.StatusUnauthorized, "unauthorized", "missing user authentication")
		return
	}

	orderID := chi.URLParam(r, "order_id")
	if orderID == "" {
		respondError(w, http.StatusBadRequest, "missing_order_id", "order_id is required")
		return
	}

	ctx = metadata.AppendToOutgoingContext(ctx,
		"user-id", fmt.Sprint(userID),
		"request-id", getRequestID(r.Context()))

	resp, err := h.ordersClient.GetOrder(ctx, &pb.GetOrderRequest{
		OrderId: orderID,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, convertProtoOrder(resp.Order))
}
