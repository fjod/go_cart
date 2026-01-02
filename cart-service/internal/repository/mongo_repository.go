package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fjod/go_cart/cart-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrCartNotFound = errors.New("cart not found")
	ErrItemNotFound = errors.New("item not found in cart")
)

type mongoRepository struct {
	collection *mongo.Collection
}

func (m mongoRepository) GetCart(ctx context.Context, userID string) (*domain.Cart, error) {
	var cart domain.Cart

	filter := bson.M{"user_id": userID}
	err := m.collection.FindOne(ctx, filter).Decode(&cart)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrCartNotFound
		}
		return nil, fmt.Errorf("failed to get cart: %w", err)
	}

	return &cart, nil
}

func (m mongoRepository) UpsertCart(ctx context.Context, cart *domain.Cart) error {
	now := time.Now()

	// Set timestamps
	if cart.CreatedAt.IsZero() {
		cart.CreatedAt = now
	}
	cart.UpdatedAt = now

	filter := bson.M{"user_id": cart.UserID}
	update := bson.M{"$set": cart}
	opts := options.Update().SetUpsert(true)

	_, err := m.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert cart: %w", err)
	}

	return nil
}

func (m mongoRepository) AddItem(ctx context.Context, userID string, item domain.CartItem) error {
	now := time.Now()
	item.AddedAt = now

	filter := bson.M{"user_id": userID}

	// First, check if cart exists
	var existingCart domain.Cart
	err := m.collection.FindOne(ctx, filter).Decode(&existingCart)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Cart doesn't exist, create it with the item
			cart := &domain.Cart{
				UserID:    userID,
				Items:     []domain.CartItem{item},
				CreatedAt: now,
				UpdatedAt: now,
			}

			_, err = m.collection.InsertOne(ctx, cart)
			if err != nil {
				return fmt.Errorf("failed to create cart with item: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to check existing cart: %w", err)
	}

	// Cart exists, check if item with same product_id exists
	itemExists := false
	for _, existingItem := range existingCart.Items {
		if existingItem.ProductID == item.ProductID {
			itemExists = true
			break
		}
	}

	if itemExists {
		// Update existing item
		update := bson.M{
			"$set": bson.M{
				"items.$[elem].quantity": item.Quantity,
				"items.$[elem].added_at": now,
				"updated_at":             now,
			},
		}
		arrayFilters := options.Update().SetArrayFilters(options.ArrayFilters{
			Filters: []interface{}{
				bson.M{"elem.product_id": item.ProductID},
			},
		})

		_, err = m.collection.UpdateOne(ctx, filter, update, arrayFilters)
		if err != nil {
			return fmt.Errorf("failed to update existing item: %w", err)
		}
	} else {
		// Add new item
		update := bson.M{
			"$push": bson.M{"items": item},
			"$set":  bson.M{"updated_at": now},
		}

		_, err = m.collection.UpdateOne(ctx, filter, update)
		if err != nil {
			return fmt.Errorf("failed to add new item: %w", err)
		}
	}

	return nil
}

func (m mongoRepository) UpdateItemQuantity(ctx context.Context, userID string, productID int64, quantity int) error {
	filter := bson.M{
		"user_id":          userID,
		"items.product_id": productID,
	}

	update := bson.M{
		"$set": bson.M{
			"items.$[elem].quantity": quantity,
			"updated_at":             time.Now(),
		},
	}

	arrayFilters := options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"elem.product_id": productID},
		},
	})

	result, err := m.collection.UpdateOne(ctx, filter, update, arrayFilters)
	if err != nil {
		return fmt.Errorf("failed to update item quantity: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrItemNotFound
	}
	return nil
}

func (m mongoRepository) RemoveItem(ctx context.Context, userID string, productID int64) error {
	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$pull": bson.M{
			"items": bson.M{"product_id": productID},
		},
		"$set": bson.M{"updated_at": time.Now()},
	}

	result, err := m.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to remove item: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrCartNotFound
	}

	return nil
}

func (m mongoRepository) DeleteCart(ctx context.Context, userID string) error {
	filter := bson.M{"user_id": userID}

	result, err := m.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete cart: %w", err)
	}

	if result.DeletedCount == 0 {
		return ErrCartNotFound
	}

	return nil
}

func (m *mongoRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "updated_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(90 * 24 * 60 * 60), // 90 days TTL
		},
	}

	_, err := m.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

func NewMongoRepository(db *mongo.Database) CartRepository {
	return &mongoRepository{
		collection: db.Collection("carts"),
	}
}
