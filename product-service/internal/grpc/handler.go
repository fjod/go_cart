package grpc

import (
	"context"

	db "github.com/fjod/go_cart/product-service/internal/repository"
	pb "github.com/fjod/go_cart/product-service/pkg/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProductServiceServer implements the gRPC ProductService
type ProductServiceServer struct {
	pb.UnimplementedProductServiceServer
	repo db.RepoInterface
}

func NewProductServiceServer(repo db.RepoInterface) *ProductServiceServer {
	return &ProductServiceServer{
		repo: repo,
	}
}

func (s *ProductServiceServer) GetProducts(
	ctx context.Context,
	_ *pb.GetProductsRequest,
) (*pb.GetProductsResponse, error) {

	// Fetch products from repository
	products, err := s.repo.GetAllProducts(ctx)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to fetch products: %v",
			err,
		)
	}

	// Convert domain models to protobuf messages
	pbProducts := make([]*pb.Product, len(products))
	for i, p := range products {
		pbProducts[i] = &pb.Product{
			Id:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price,
			ImageUrl:    p.ImageURL,
			Stock:       p.Stock,
			CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return &pb.GetProductsResponse{
		Products: pbProducts,
	}, nil
}
