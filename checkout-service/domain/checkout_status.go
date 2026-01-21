package domain

type CheckoutStatus string

const (
	CheckoutStatusInitiated         CheckoutStatus = "INITIATED"
	CheckoutStatusInventoryReserved CheckoutStatus = "INVENTORY_RESERVED"
	CheckoutStatusPaymentPending    CheckoutStatus = "PAYMENT_PENDING"
	CheckoutStatusPaymentCompleted  CheckoutStatus = "PAYMENT_COMPLETED"
	CheckoutStatusCompleted         CheckoutStatus = "COMPLETED"
	CheckoutStatusFailed            CheckoutStatus = "FAILED"
)

func (s CheckoutStatus) IsTerminal() bool {
	return s == CheckoutStatusCompleted || s == CheckoutStatusFailed
}

// String representation (for logging)
func (s CheckoutStatus) String() string {
	return string(s)
}
