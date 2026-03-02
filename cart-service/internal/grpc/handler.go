package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fjod/go_cart/cart-service/internal/domain"
	"github.com/fjod/go_cart/cart-service/internal/repository"
	s "github.com/fjod/go_cart/cart-service/internal/service"
	pb "github.com/fjod/go_cart/cart-service/pkg/proto"
	"github.com/fjod/go_cart/pkg/logger"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const timeFormat string = "2006-01-02T15:04:05Z07:00"

type CartServiceServer struct {
	pb.UnimplementedCartServiceServer
	service       *s.CartService
	productClient productpb.ProductServiceClient
	logger        *slog.Logger
}

func NewCartServiceServer(service *s.CartService, productClient productpb.ProductServiceClient, logger *slog.Logger) *CartServiceServer {
	return &CartServiceServer{
		service:       service,
		productClient: productClient,
		logger:        logger,
	}
}

func convertCart(c domain.Cart, userId int64) *pb.Cart {
	cart := &pb.Cart{
		Id:        c.ID,
		UserId:    userId, // Use the request user_id
		Cart:      make([]*pb.CartItem, len(c.Items)),
		CreatedAt: c.CreatedAt.Format(timeFormat),
		UpdatedAt: c.UpdatedAt.Format(timeFormat),
	}

	for i, item := range c.Items {
		cart.Cart[i] = &pb.CartItem{
			ProductId: item.ProductID,
			Quantity:  int32(item.Quantity),
			AddedAt:   item.AddedAt.Format(timeFormat),
		}
	}

	return cart
}

func (s *CartServiceServer) GetCart(
	ctx context.Context,
	req *pb.GetCartRequest) (*pb.CartResponse, error) {

	log := logger.WithContext(s.logger, ctx)
	log.Info("get cart", slog.Int64("user_id", req.UserId))

	if req.UserId <= 0 {
		log.Warn("invalid user_id: must be greater than 0", slog.Int64("user_id", req.UserId))
		return nil, status.Error(codes.InvalidArgument, "user_id must be greater than 0")
	}

	userID := fmt.Sprintf("%d", req.UserId)
	cart, err := s.service.GetCart(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get cart: %v", err)
	}

	protoCart := convertCart(*cart, req.UserId)

	return &pb.CartResponse{
		Cart: protoCart,
	}, nil
}

func (s *CartServiceServer) AddItem(
	ctx context.Context,
	req *pb.AddCartItemRequest) (*pb.CartResponse, error) {

	log := logger.WithContext(s.logger, ctx)
	log.Info("adding item", slog.String("product_id", fmt.Sprintf("%d", req.ProductId)))

	// Validate input
	if req.ProductId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "product_id must be greater than 0")
	}
	if req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity must be greater than 0")
	}
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be greater than 0")
	}

	// Call product-service to validate if product exists
	_, err := s.productClient.GetProduct(ctx, &productpb.GetProductRequest{
		Id: req.ProductId,
	})
	if err != nil {
		// Check if product not found
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.NotFound {
				log.Error("product not found", slog.String("product_id", fmt.Sprintf("%d", req.ProductId)))
				return nil, status.Error(codes.NotFound, "product not found")
			}
		}
		log.Error("failed to validate product", slog.String("product_id", fmt.Sprintf("%d", req.ProductId)))
		return nil, status.Errorf(codes.Internal, "failed to validate product: %v", err)
	}

	// Convert user_id to string for MongoDB
	userID := fmt.Sprintf("%d", req.UserId)

	// Create cart item
	cartItem := domain.CartItem{
		ProductID: req.ProductId,
		Quantity:  int(req.Quantity),
		AddedAt:   time.Now(),
	}

	// Add item to cart via repository
	err = s.service.AddItem(ctx, userID, cartItem)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add item to cart: %v", err)
	}

	// Get the updated cart
	cart, err := s.service.GetCart(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get cart: %v", err)
	}

	protoCart := convertCart(*cart, req.UserId)

	return &pb.CartResponse{
		Cart: protoCart,
	}, nil
}

func (s *CartServiceServer) UpdateQuantity(
	ctx context.Context,
	req *pb.UpdateQuantityRequest) (*pb.CartResponse, error) {

	log := logger.WithContext(s.logger, ctx)
	log.Info("update quantity", slog.String("product_id", fmt.Sprintf("%d", req.ProductId)))

	// Validate input
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be greater than 0")
	}
	if req.ProductId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "product_id must be greater than 0")
	}
	if req.Quantity <= 0 || req.Quantity > 99 {
		return nil, status.Error(codes.InvalidArgument, "quantity must be between 1 and 99")
	}

	userID := fmt.Sprintf("%d", req.UserId)

	// Update item quantity in repository
	err := s.service.UpdateQuantity(ctx, userID, req.ProductId, int(req.Quantity))
	if err != nil {
		// Check if item was not found in cart
		if errors.Is(err, repository.ErrItemNotFound) {
			log.Warn("item not found in cart", slog.Int64("product_id", req.ProductId))
			return nil, status.Error(codes.NotFound, "item not found in cart")
		}
		return nil, status.Errorf(codes.Internal, "failed to update item quantity: %v", err)
	}

	// Get the updated cart
	cart, err := s.service.GetCart(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get cart: %v", err)
	}

	protoCart := convertCart(*cart, req.UserId)

	return &pb.CartResponse{
		Cart: protoCart,
	}, nil
}

func (s *CartServiceServer) RemoveItem(
	ctx context.Context,
	req *pb.RemoveItemRequest) (*pb.CartResponse, error) {

	log := logger.WithContext(s.logger, ctx)
	log.Info("remove item", slog.String("product_id", fmt.Sprintf("%d", req.ProductId)))

	// Validate input
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be greater than 0")
	}
	if req.ProductId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "product_id must be greater than 0")
	}

	userID := fmt.Sprintf("%d", req.UserId)

	// Remove item from repository
	err := s.service.RemoveItem(ctx, userID, req.ProductId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove item: %v", err)
	}

	// Get the updated cart
	cart, err := s.service.GetCart(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get cart: %v", err)
	}

	protoCart := convertCart(*cart, req.UserId)

	return &pb.CartResponse{
		Cart: protoCart,
	}, nil
}

func (s *CartServiceServer) ClearCart(
	ctx context.Context,
	req *pb.ClearCartRequest) (*pb.CartResponse, error) {

	log := logger.WithContext(s.logger, ctx)
	log.Info("clear cart", slog.String("user_id", fmt.Sprintf("%d", req.UserId)))

	// Validate input
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be greater than 0")
	}

	userID := fmt.Sprintf("%d", req.UserId)

	// Delete cart from repository
	err := s.service.ClearCart(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrCartNotFound) {
			// Cart already cleared — treat as success (idempotent operation)
		} else {
			return nil, status.Errorf(codes.Internal, "failed to clear cart: %v", err)
		}
	}

	// Return empty cart response
	emptyCart := &pb.Cart{
		UserId:    req.UserId,
		Cart:      []*pb.CartItem{},
		CreatedAt: time.Now().Format(timeFormat),
		UpdatedAt: time.Now().Format(timeFormat),
	}

	return &pb.CartResponse{
		Cart: emptyCart,
	}, nil
}
