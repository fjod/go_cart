package http

import (
	"context"
	"net/http"
	"time"

	pb "github.com/fjod/go_cart/product-service/pkg/proto"
)

type ProductHandler struct {
	productClient pb.ProductServiceClient
	timeout       time.Duration
}

func NewProductHandler(productClient pb.ProductServiceClient, timeout time.Duration) *ProductHandler {
	return &ProductHandler{
		productClient: productClient,
		timeout:       timeout,
	}
}

type ProductResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"image_url"`
}

type ProductsResponse struct {
	Products []ProductResponse `json:"products"`
}

func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	res, err := h.productClient.GetProducts(ctx, &pb.GetProductsRequest{})
	if err != nil {
		handleGRPCError(w, err)
		return
	}
	products := make([]ProductResponse, len(res.Products))
	for i, p := range res.Products {
		products[i] = ProductResponse{
			ID:          p.Id,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price,
			ImageURL:    p.ImageUrl,
		}
	}

	respondJSON(w, http.StatusOK, &ProductsResponse{Products: products})
}
