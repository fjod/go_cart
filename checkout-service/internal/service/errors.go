package service

import "errors"

var (
	ErrEmptyCart           = errors.New("cart is empty, nothing to checkout")
	IllegalTransitionError = errors.New("illegal transition of checkout status")
)
