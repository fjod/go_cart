package domain

import "time"

type Cart struct {
	ID        string     `bson:"_id,omitempty"`
	UserID    string     `bson:"user_id"`
	Items     []CartItem `bson:"items"`
	CreatedAt time.Time  `bson:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at"`
}

type CartItem struct {
	ProductID int64     `bson:"product_id"`
	Quantity  int       `bson:"quantity"`
	AddedAt   time.Time `bson:"added_at"`
}
