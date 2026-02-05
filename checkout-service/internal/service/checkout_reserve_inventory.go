package service

import (
	"context"

	d "github.com/fjod/go_cart/checkout-service/domain"
	inventorypb "github.com/fjod/go_cart/inventory-service/pkg/proto"
)

func (s *CheckoutServiceImpl) reserveInventory(ctx context.Context, checkoutId string, items []*d.CartSnapshotItem, status d.CheckoutStatus) (*string, error) {
	if !d.CanTransitionTo(status, d.CheckoutStatusInventoryReserved) {
		return nil, IllegalTransitionError
	}
	reqItems := mapItems(items)
	request := inventorypb.ReserveRequest{
		CheckoutId: checkoutId,
		Items:      reqItems,
	}

	inventoryCtx, cancel := context.WithTimeout(ctx, s.inventory.timeout)
	defer cancel()
	result, e := s.inventory.inventoryClient.Reserve(inventoryCtx, &request)
	if e != nil {
		return nil, e
	}
	newStatus := d.CheckoutStatusInventoryReserved
	dbError := s.repo.SetReservation(ctx, &checkoutId, &newStatus, &result.ReservationId)
	if dbError != nil {
		return nil, dbError
	}
	return &result.ReservationId, nil
}

func mapItems(items []*d.CartSnapshotItem) []*inventorypb.ReservationItem {
	resItems := make([]*inventorypb.ReservationItem, len(items))
	for i, item := range items {
		resItems[i] = &inventorypb.ReservationItem{
			ProductId: item.ProductID,
			Quantity:  item.Quantity,
		}
	}
	return resItems
}
