package grpc

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	pb "github.com/fjod/go_cart/payment-service/pkg/proto"
)

type GetResponseStatus interface {
	GetStatus() (pb.ChargeStatus, pb.PaymentRefusal, string)
}

type RandomStatus struct{}

func (r RandomStatus) GetStatus() (pb.ChargeStatus, pb.PaymentRefusal, string) {
	randomInt := rand.Intn(101) // 101 because Intn is exclusive of the upper bound
	return calcStatus(randomInt)
}

func calcStatus(randomInt int) (pb.ChargeStatus, pb.PaymentRefusal, string) {
	if randomInt < 95 {
		return pb.ChargeStatus_CHARGE_STATUS_SUCCESS, pb.PaymentRefusal_UNKNOWN, ""
	}
	otherReason := randomInt - 95
	if otherReason == 0 || otherReason > 5 {
		return pb.ChargeStatus_CHARGE_STATUS_FAILED, pb.PaymentRefusal_UNKNOWN, "unknown reason"
	}

	return pb.ChargeStatus_CHARGE_STATUS_FAILED, pb.PaymentRefusal(otherReason), ""
}

type PaymentServiceServer struct {
	pb.UnimplementedPaymentServiceServer
	status GetResponseStatus
}

func NewPaymentServiceServer(s GetResponseStatus) *PaymentServiceServer {
	return &PaymentServiceServer{
		status: s,
	}
}

func (s *PaymentServiceServer) Charge(_ context.Context, r *pb.ChargeRequest) (*pb.ChargeResponse, error) {
	charge, refusalKnown, refusalOther := s.status.GetStatus()
	tsId := fmt.Sprintf("TXN-%v", time.Now())

	if refusalOther == "" {
		return &pb.ChargeResponse{
			Status:        charge,
			TransactionId: tsId,
			CheckoutId:    r.CheckoutId,
			Refusal: &pb.ChargeResponse_KnownReason{
				KnownReason: refusalKnown,
			},
		}, nil
	}
	return &pb.ChargeResponse{
		Status:        charge,
		TransactionId: tsId,
		CheckoutId:    r.CheckoutId,
		Refusal: &pb.ChargeResponse_OtherReason{
			OtherReason: refusalOther,
		},
	}, nil
}

// Refund is always success for this implementation.
func (*PaymentServiceServer) Refund(context.Context, *pb.RefundRequest) (*pb.RefundResponse, error) {
	return &pb.RefundResponse{}, nil
}
