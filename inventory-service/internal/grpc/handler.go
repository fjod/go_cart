package grpc

import (
	"context"
	"errors"

	"github.com/fjod/go_cart/inventory-service/internal/domain"
	"github.com/fjod/go_cart/inventory-service/internal/store"
	pb "github.com/fjod/go_cart/inventory-service/pkg/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InventoryServiceServer implements the gRPC inventory service
type InventoryServiceServer struct {
	pb.UnimplementedInventoryServiceServer
	store store.InventoryStore
}

// NewInventoryServiceServer creates a new gRPC handler
func NewInventoryServiceServer(store store.InventoryStore) *InventoryServiceServer {
	return &InventoryServiceServer{
		store: store,
	}
}

// GetStock returns stock levels for specified products
func (s *InventoryServiceServer) GetStock(_ context.Context, req *pb.GetStockRequest) (*pb.GetStockResponse, error) {
	if len(req.ProductIds) == 0 {
		return &pb.GetStockResponse{Stocks: []*pb.StockInfo{}}, nil
	}

	stocks, err := s.store.GetStock(req.ProductIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get stock: %v", err)
	}

	// Convert domain to proto
	protoStocks := make([]*pb.StockInfo, len(stocks))
	for i, stock := range stocks {
		protoStocks[i] = &pb.StockInfo{
			ProductId: stock.ProductID,
			Available: stock.Available(),
			Reserved:  stock.Reserved,
		}
	}

	return &pb.GetStockResponse{Stocks: protoStocks}, nil
}

// Reserve creates a stock reservation for checkout
func (s *InventoryServiceServer) Reserve(_ context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {
	// Validate input
	if req.CheckoutId == "" {
		return nil, status.Error(codes.InvalidArgument, "checkout_id is required")
	}
	if len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one item is required")
	}

	// Validate each item
	for _, item := range req.Items {
		if item.ProductId <= 0 {
			return nil, status.Error(codes.InvalidArgument, "product_id must be greater than 0")
		}
		if item.Quantity <= 0 {
			return nil, status.Error(codes.InvalidArgument, "quantity must be greater than 0")
		}
	}

	// Convert proto to domain
	domainItems := make([]domain.ReservationItem, len(req.Items))
	for i, item := range req.Items {
		domainItems[i] = domain.ReservationItem{
			ProductID: item.ProductId,
			Quantity:  item.Quantity,
		}
	}

	reservation, err := s.store.Reserve(req.CheckoutId, domainItems)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return &pb.ReserveResponse{
		ReservationId: reservation.ID,
		ExpiresAt:     reservation.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// Confirm finalizes a reservation after successful payment
func (s *InventoryServiceServer) Confirm(_ context.Context, req *pb.ConfirmRequest) (*pb.ConfirmResponse, error) {
	if req.ReservationId == "" {
		return nil, status.Error(codes.InvalidArgument, "reservation_id is required")
	}

	err := s.store.Confirm(req.ReservationId)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return &pb.ConfirmResponse{Success: true}, nil
}

// Release cancels a reservation on payment failure
func (s *InventoryServiceServer) Release(_ context.Context, req *pb.ReleaseRequest) (*pb.ReleaseResponse, error) {
	if req.ReservationId == "" {
		return nil, status.Error(codes.InvalidArgument, "reservation_id is required")
	}

	err := s.store.Release(req.ReservationId)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return &pb.ReleaseResponse{Success: true}, nil
}

// mapStoreError converts store errors to appropriate gRPC status codes
func mapStoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrProductNotFound):
		return status.Error(codes.NotFound, "product not found")
	case errors.Is(err, store.ErrInsufficientStock):
		return status.Error(codes.FailedPrecondition, "insufficient stock")
	case errors.Is(err, store.ErrReservationNotFound):
		return status.Error(codes.NotFound, "reservation not found")
	case errors.Is(err, store.ErrReservationExpired):
		return status.Error(codes.FailedPrecondition, "reservation has expired")
	case errors.Is(err, store.ErrInvalidStatus):
		return status.Error(codes.FailedPrecondition, "invalid reservation status")
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}
