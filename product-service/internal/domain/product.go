package domain

import "time"

type Product struct {
	ID          int64
	Name        string
	Description string
	Price       float64
	ImageURL    string
	CreatedAt   time.Time
}
