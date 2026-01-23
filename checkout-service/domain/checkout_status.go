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

// validTransitions defines which states can transition to which other states.
// Key = current state, Value = set of valid next states
var validTransitions = map[CheckoutStatus]map[CheckoutStatus]bool{
	CheckoutStatusInitiated: {
		CheckoutStatusInventoryReserved: true,
		CheckoutStatusFailed:            true,
	},
	CheckoutStatusInventoryReserved: {
		CheckoutStatusPaymentPending: true,
		CheckoutStatusFailed:         true,
	},
	CheckoutStatusPaymentPending: {
		CheckoutStatusPaymentCompleted: true,
		CheckoutStatusFailed:           true,
	},
	CheckoutStatusPaymentCompleted: {
		CheckoutStatusCompleted: true,
		CheckoutStatusFailed:    true,
	},
}

// CanTransitionTo checks if transitioning from current status to next status is valid.
func CanTransitionTo(current, next CheckoutStatus) bool {
	allowedNextStates, exists := validTransitions[current]
	if !exists {
		return false // Terminal state or unknown state
	}
	return allowedNextStates[next]
}

// String representation (for logging)
func (s CheckoutStatus) String() string {
	return string(s)
}
