package service

import (
	"context"
	"errors"
	"fmt"

	d "github.com/fjod/go_cart/checkout-service/domain"
	paymentpb "github.com/fjod/go_cart/payment-service/pkg/proto"
)

func (s *CheckoutServiceImpl) processPayment(ctx context.Context, checkoutId string, status d.CheckoutStatus, amount string) error {
	if !d.CanTransitionTo(status, d.CheckoutStatusPaymentPending) {
		return IllegalTransitionError
	}
	pendingStatus := d.CheckoutStatusPaymentPending
	err := s.repo.UpdateCheckoutSessionStatus(ctx, &checkoutId, &pendingStatus)
	if err != nil {
		return err
	}

	paymentCtx, cancel := context.WithTimeout(ctx, s.payment.timeout)
	defer cancel()
	payRequest := &paymentpb.ChargeRequest{
		CheckoutId: checkoutId,
		Amount:     amount,
	}
	payResult, payErr := s.payment.paymentClient.Charge(paymentCtx, payRequest)
	if payErr != nil {
		return payErr // unknown error, all known errors will be in payResult
	}

	if payResult.Status == paymentpb.ChargeStatus_CHARGE_STATUS_SUCCESS {
		paidStatus := d.CheckoutStatusPaymentCompleted
		dbError := s.repo.SetPayment(ctx, &checkoutId, &paidStatus, &payResult.PaymentId)
		if dbError != nil {
			return dbError
		}
		return nil
	}

	return errors.New(convertError(payResult))
}

func convertError(result *paymentpb.ChargeResponse) string {
	if result.GetOtherReason() != "" {
		return fmt.Sprintf("Payment failed: %v", result.GetOtherReason())
	}
	return fmt.Sprintf("Payment failed: %v", result.GetKnownReason().String())
}
