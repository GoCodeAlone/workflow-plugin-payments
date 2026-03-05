package payments

import "errors"

// Shared sentinel errors for PaymentProvider implementations.
var (
	ErrUnsupported     = errors.New("operation not supported by this provider")
	ErrNotFound        = errors.New("resource not found")
	ErrInvalidConfig   = errors.New("invalid provider configuration")
	ErrAuthFailed      = errors.New("authentication failed")
	ErrWebhookInvalid  = errors.New("webhook signature invalid")
)
