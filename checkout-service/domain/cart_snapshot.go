package domain

import "time"

type CartSnapshotItem struct {
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int32   `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Subtotal    float64 `json:"subtotal"`
}

// CartSnapshot represents the full cart state at checkout time
type CartSnapshot struct {
	Items       []CartSnapshotItem `json:"items"`
	TotalAmount float64            `json:"total_amount"`
	Currency    string             `json:"currency"`
	CapturedAt  time.Time          `json:"captured_at"`
}
