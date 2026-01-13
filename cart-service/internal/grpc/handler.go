package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/fjod/go_cart/cart-service/internal/domain"
	db "github.com/fjod/go_cart/cart-service/internal/repository"
	pb "github.com/fjod/go_cart/cart-service/pkg/proto"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CartServiceServer struct {
	pb.UnimplementedCartServiceServer
	repo          db.CartRepository
	productClient productpb.ProductServiceClient
}

func NewCartServiceServer(repo db.CartRepository, productClient productpb.ProductServiceClient) *CartServiceServer {
	return &CartServiceServer{
		repo:          repo,
		productClient: productClient,
	}
}

func (s *CartServiceServer) AddItem(
	ctx context.Context,
	req *pb.AddCartItemRequest) (*pb.AddCartItemResponse, error) {

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
	productResp, err := s.productClient.GetProduct(ctx, &productpb.GetProductRequest{
		Id: req.ProductId,
	})
	if err != nil {
		// Check if product not found
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.NotFound {
				return nil, status.Error(codes.NotFound, "product not found")
			}
		}
		return nil, status.Errorf(codes.Internal, "failed to validate product: %v", err)
	}

	// Check if product has sufficient stock
	if productResp.Product.Stock < req.Quantity {
		return nil, status.Errorf(codes.FailedPrecondition, "insufficient stock: available=%d, requested=%d",
			productResp.Product.Stock, req.Quantity)
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
	err = s.repo.AddItem(ctx, userID, cartItem)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add item to cart: %v", err)
	}

	// Get the updated cart
	cart, err := s.repo.GetCart(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get cart: %v", err)
	}

	protoCart := &pb.Cart{
		Id:        cart.ID,
		UserId:    req.UserId, // Use the request user_id
		Cart:      make([]*pb.CartItem, len(cart.Items)),
		CreatedAt: cart.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: cart.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	for i, item := range cart.Items {
		protoCart.Cart[i] = &pb.CartItem{
			ProductId: item.ProductID,
			Quantity:  int32(item.Quantity),
			AddedAt:   item.AddedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return &pb.AddCartItemResponse{
		Cart: protoCart,
	}, nil
}
