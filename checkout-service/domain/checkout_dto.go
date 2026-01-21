package domain

type CheckoutRequest struct {
	UserID         int64
	IdempotencyKey string
}

type CheckoutResponse struct {
	CheckoutID *string
	Status     *CheckoutStatus
}
