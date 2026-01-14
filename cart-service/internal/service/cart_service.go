package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/fjod/go_cart/cart-service/internal/cache"
	"github.com/fjod/go_cart/cart-service/internal/domain"
	"github.com/fjod/go_cart/cart-service/internal/repository"
	"golang.org/x/sync/singleflight"
)

type CartService struct {
	repo  repository.CartRepository
	cache cache.CartCache
	sfg   singleflight.Group // Prevents cache stampede
}

func NewCartService(repo repository.CartRepository, cache cache.CartCache) *CartService {
	return &CartService{
		repo:  repo,
		cache: cache,
	}
}

func (s *CartService) GetCart(ctx context.Context, userID string) (*domain.Cart, error) {
	// Use singleflight to prevent multiple concurrent cache misses for same key
	v, err, _ := s.sfg.Do(userID, func() (interface{}, error) {

		cart, err := s.cache.Get(ctx, userID)
		if err == nil {
			return cart, nil // cart is in cache
		}

		if !errors.Is(err, cache.ErrCacheMiss) {
			_ = fmt.Errorf("cache get error: %w \n", err) // log cache error but continue
		}

		cart, errGet := s.repo.GetCart(ctx, userID)
		if errGet != nil {
			return nil, errGet // err from repo is not cache miss, return it
		}

		// set cache
		go func() {
			errSet := s.cache.Set(context.Background(), userID, cart)
			if errSet != nil {
				_ = fmt.Errorf("cache set error: %w \n", errSet)
			}
		}()

		return cart, nil // cart was not in cache, return it from repo
	})

	if err != nil {
		return nil, err
	}

	return v.(*domain.Cart), nil
}

func (s *CartService) AddItem(ctx context.Context, userID string, item domain.CartItem) error {
	errAdd := s.repo.AddItem(ctx, userID, item)
	if errAdd != nil {
		fmt.Printf("repo add item error: %v \n", errAdd)
		return errAdd
	}

	go invalidateCache(s, userID)
	return nil
}

func (s *CartService) UpdateQuantity(ctx context.Context, userID string, productID int64, quantity int) error {
	errUpdate := s.repo.UpdateItemQuantity(ctx, userID, productID, quantity)
	if errUpdate != nil {
		fmt.Printf("repo update item quantity error: %v \n", errUpdate)
		return errUpdate
	}

	go invalidateCache(s, userID)
	return nil
}

func (s *CartService) RemoveItem(ctx context.Context, userID string, productID int64) error {
	errRemove := s.repo.RemoveItem(ctx, userID, productID)
	if errRemove != nil {
		fmt.Printf("repo remove item error: %v \n", errRemove)
		return errRemove
	}

	go invalidateCache(s, userID)
	return nil
}

func (s *CartService) ClearCart(ctx context.Context, userID string) error {
	errDelete := s.repo.DeleteCart(ctx, userID)
	if errDelete != nil {
		fmt.Printf("repo delete cart error: %v \n", errDelete)
		return errDelete
	}

	go invalidateCache(s, userID)
	return nil
}

func invalidateCache(s *CartService, userID string) {
	errInvalidate := s.cache.Delete(context.Background(), userID)
	if errInvalidate != nil {
		fmt.Printf("cache invalidate error: %v \n", errInvalidate)
	}
}
