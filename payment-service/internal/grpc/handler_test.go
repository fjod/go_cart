package grpc

import (
	"context"
	"testing"

	pb "github.com/fjod/go_cart/payment-service/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStatus struct {
	st pb.ChargeStatus
	rf pb.PaymentRefusal
	s  string
}

func (m *mockStatus) GetStatus() (pb.ChargeStatus, pb.PaymentRefusal, string) {
	return m.st, m.rf, m.s
}

func TestCalculateRandomStatus(t *testing.T) {
	tests := []struct {
		name string
		v    int
		st   GetResponseStatus
	}{
		{
			name: "success",
			v:    10,
			st: &mockStatus{
				st: pb.ChargeStatus_CHARGE_STATUS_SUCCESS,
				rf: pb.PaymentRefusal_UNKNOWN,
				s:  "",
			},
		},
		{
			name: "success",
			v:    94,
			st: &mockStatus{
				st: pb.ChargeStatus_CHARGE_STATUS_SUCCESS,
				rf: pb.PaymentRefusal_UNKNOWN,
				s:  "",
			},
		},
		{
			name: "failed",
			v:    95,
			st: &mockStatus{
				st: pb.ChargeStatus_CHARGE_STATUS_FAILED,
				rf: pb.PaymentRefusal_UNKNOWN,
				s:  "unknown reason",
			},
		},
		{
			name: "failed",
			v:    96, // 97 98 99 100
			st: &mockStatus{
				st: pb.ChargeStatus_CHARGE_STATUS_FAILED,
				rf: pb.PaymentRefusal(96 - 95),
				s:  "",
			},
		},
		{
			name: "failed",
			v:    100, // 97 98 99 100
			st: &mockStatus{
				st: pb.ChargeStatus_CHARGE_STATUS_FAILED,
				rf: pb.PaymentRefusal(100 - 95),
				s:  "",
			},
		},
		{
			name: "failed",
			v:    101,
			st: &mockStatus{
				st: pb.ChargeStatus_CHARGE_STATUS_FAILED,
				rf: pb.PaymentRefusal_UNKNOWN,
				s:  "unknown reason",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			charge, refusalKnown, refusalOther := calcStatus(tt.v)
			chargeExp, refusalKnownExp, refusalOtherExp := tt.st.GetStatus()
			assert.Equal(t, charge, chargeExp)
			assert.Equal(t, refusalKnown, refusalKnownExp)
			assert.Equal(t, refusalOther, refusalOtherExp)
		})
	}
}

func TestHandler_Ok(t *testing.T) {
	tests := []struct {
		name string
		st   GetResponseStatus
	}{
		{
			name: "success",
			st: &mockStatus{
				st: pb.ChargeStatus_CHARGE_STATUS_SUCCESS,
				rf: pb.PaymentRefusal_UNKNOWN,
				s:  "",
			},
		},
		{
			name: "err_no_funds",
			st: &mockStatus{
				st: pb.ChargeStatus_CHARGE_STATUS_FAILED,
				rf: pb.PaymentRefusal_NO_FUNDS,
				s:  "",
			},
		},
		{
			name: "unknown reason",
			st: &mockStatus{
				st: pb.ChargeStatus_CHARGE_STATUS_FAILED,
				rf: pb.PaymentRefusal_UNKNOWN,
				s:  "unknown reason",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewPaymentServiceServer(tt.st)
			req := &pb.ChargeRequest{
				CheckoutId: "test",
			}
			resp, err := handler.Charge(context.Background(), req)
			charge, refusalKnown, refusalOther := tt.st.GetStatus()
			require.NoError(t, err)
			assert.Equal(t, charge, resp.Status)
			assert.Equal(t, refusalKnown, resp.GetKnownReason())
			assert.Equal(t, refusalOther, resp.GetOtherReason())
			assert.Equal(t, "test", resp.CheckoutId)
		})
	}
}
