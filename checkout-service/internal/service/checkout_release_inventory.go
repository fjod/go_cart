package service

import (
	"context"

	inventorypb "github.com/fjod/go_cart/inventory-service/pkg/proto"
)

func (s *CheckoutServiceImpl) releaseInventory(ctx context.Context, reservationId string) error {
	releaseRequest := &inventorypb.ReleaseRequest{
		ReservationId: reservationId,
	}

	inventoryCtx, cancel := context.WithTimeout(ctx, s.inventory.timeout)
	defer cancel()
	_, err := s.inventory.inventoryClient.Release(inventoryCtx, releaseRequest) // as inventory is stub, it will always return success.
	if err != nil {
		return err
	}

	return nil
}
