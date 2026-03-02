package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/fjod/go_cart/cart-service/internal/cache"
	"github.com/fjod/go_cart/cart-service/internal/domain"
	"github.com/fjod/go_cart/cart-service/internal/repository"
	"github.com/fjod/go_cart/pkg/logger"
	"golang.org/x/sync/singleflight"
)

type CartService struct {
	repo   repository.CartRepository
	cache  cache.CartCache
	sfg    singleflight.Group // Prevents cache stampede
	logger *slog.Logger
}

func NewCartService(repo repository.CartRepository, cache cache.CartCache, logger *slog.Logger) *CartService {
	return &CartService{
		repo:   repo,
		cache:  cache,
		logger: logger,
	}
}

func (s *CartService) GetCart(ctx context.Context, userID string) (*domain.Cart, error) {
	// Use singleflight to prevent multiple concurrent cache misses for same key
	v, err, _ := s.sfg.Do(userID, func() (interface{}, error) {

		l := logger.WithContext(s.logger, ctx)
		cart, err := s.cache.Get(ctx, userID)
		if err == nil {
			return cart, nil // cart is in cache
		}

		if !errors.Is(err, cache.ErrCacheMiss) {
			l.Warn("cache get error", "error", err) // log cache error but continue
		}

		cart, errGet := s.repo.GetCart(ctx, userID)
		if errGet != nil && errors.Is(errGet, repository.ErrCartNotFound) { // not found cart return empty cart
			return &domain.Cart{
				UserID:    userID,
				Items:     nil,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		}
		if errGet != nil {
			return nil, errGet // err from repo is not cache miss, return it
		}

		// set cache
		go func() {
			errSet := s.cache.Set(context.Background(), userID, cart)
			if errSet != nil {
				l.Error("cache set error", "error", errSet) // log cache error but continue
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
		l := logger.WithContext(s.logger, ctx)
		l.Error("repo add item error", "error", errAdd)
		return errAdd
	}

	invalidateCache(s, userID)
	return nil
}

func (s *CartService) UpdateQuantity(ctx context.Context, userID string, productID int64, quantity int) error {
	errUpdate := s.repo.UpdateItemQuantity(ctx, userID, productID, quantity)
	if errUpdate != nil {
		l := logger.WithContext(s.logger, ctx)
		l.Error("repo update item quantity error", "error", errUpdate)
		return errUpdate
	}

	invalidateCache(s, userID)
	return nil
}

func (s *CartService) RemoveItem(ctx context.Context, userID string, productID int64) error {
	errRemove := s.repo.RemoveItem(ctx, userID, productID)
	if errRemove != nil {
		l := logger.WithContext(s.logger, ctx)
		l.Error("repo remove item error", "error", errRemove)
		return errRemove
	}

	invalidateCache(s, userID)
	return nil
}

func (s *CartService) ClearCart(ctx context.Context, userID string) error {
	errDelete := s.repo.DeleteCart(ctx, userID)
	if errDelete != nil {
		l := logger.WithContext(s.logger, ctx)
		l.Error("repo delete cart error", "error", errDelete)
		return errDelete
	}

	invalidateCache(s, userID)
	return nil
}

func invalidateCache(s *CartService, userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	errInvalidate := s.cache.Delete(ctx, userID)
	if errInvalidate != nil {
		l := logger.WithContext(s.logger, ctx)
		l.Error("cache invalidate error", "error", errInvalidate)
	}
}
