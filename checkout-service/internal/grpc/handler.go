package grpc

import (
	"context"

	d "github.com/fjod/go_cart/checkout-service/domain"
	s "github.com/fjod/go_cart/checkout-service/internal/service"
	pb "github.com/fjod/go_cart/checkout-service/pkg/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CheckoutServiceServer struct {
	pb.UnimplementedCheckoutServiceServer
	service *s.CheckoutServiceImpl
}

func NewCheckoutServiceServer(service *s.CheckoutServiceImpl) *CheckoutServiceServer {
	return &CheckoutServiceServer{
		service: service,
	}
}

func (h *CheckoutServiceServer) InitiateCheckout(
	ctx context.Context,
	req *pb.InitiateCheckoutRequest) (*pb.InitiateCheckoutResponse, error) {

	// Validate
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be greater than 0")
	}
	if req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}

	// Call business logic
	resp, err := h.service.InitiateCheckout(ctx, &d.CheckoutRequest{
		UserID:         req.UserId,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "checkout failed: %v", err)
	}

	// Convert response (handle nil pointers from domain)
	return &pb.InitiateCheckoutResponse{
		CheckoutId: getStringValue(resp.CheckoutID),
		Status:     mapDomainStatusToProto(getStatusValue(resp.Status)),
	}, nil
}

func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func getStatusValue(s *d.CheckoutStatus) d.CheckoutStatus {
	if s == nil {
		return d.CheckoutStatusInitiated
	}
	return *s
}

func mapDomainStatusToProto(ds d.CheckoutStatus) pb.CheckoutStatus {
	switch ds {
	case d.CheckoutStatusInitiated:
		return pb.CheckoutStatus_CHECKOUT_STATUS_INITIATED
	case d.CheckoutStatusInventoryReserved:
		return pb.CheckoutStatus_CHECKOUT_STATUS_INVENTORY_RESERVED
	case d.CheckoutStatusPaymentPending:
		return pb.CheckoutStatus_CHECKOUT_STATUS_PAYMENT_PENDING
	case d.CheckoutStatusPaymentCompleted:
		return pb.CheckoutStatus_CHECKOUT_STATUS_PAYMENT_COMPLETED
	case d.CheckoutStatusCompleted:
		return pb.CheckoutStatus_CHECKOUT_STATUS_COMPLETED
	case d.CheckoutStatusFailed:
		return pb.CheckoutStatus_CHECKOUT_STATUS_FAILED
	default:
		return pb.CheckoutStatus_CHECKOUT_STATUS_INITIATED
	}
}
