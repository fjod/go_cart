package grpc

import (
	"context"
	"errors"

	"github.com/fjod/go_cart/orders-service/internal/domain"
	"github.com/fjod/go_cart/orders-service/internal/repository"
	pb "github.com/fjod/go_cart/orders-service/pkg/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrdersHandler struct {
	pb.UnimplementedOrdersServiceServer
	repo repository.OrderRepository
}

func NewOrdersHandler(repo repository.OrderRepository) *OrdersHandler {
	return &OrdersHandler{repo: repo}
}

func (h *OrdersHandler) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	id, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_id: %v", err)
	}

	order, err := h.repo.GetOrderByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found: %s", req.OrderId)
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}

	return &pb.GetOrderResponse{Order: convertOrderToProto(order)}, nil
}

func (h *OrdersHandler) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	orders, err := h.repo.ListOrdersByUserID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list orders: %v", err)
	}

	protoOrders := make([]*pb.Order, 0, len(orders))
	for _, o := range orders {
		protoOrders = append(protoOrders, convertOrderToProto(o))
	}

	return &pb.ListOrdersResponse{Orders: protoOrders}, nil
}

func convertOrderToProto(order *domain.Order) *pb.Order {
	items := make([]*pb.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, &pb.OrderItem{
			ProductId:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    int32(item.Quantity),
			Price:       item.Price,
		})
	}
	return &pb.Order{
		Id:          order.ID.String(),
		CheckoutId:  order.CheckoutID.String(),
		UserId:      order.UserID,
		TotalAmount: order.TotalAmount,
		Currency:    order.Currency,
		Status:      string(order.Status),
		Items:       items,
		CreatedAt:   order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
