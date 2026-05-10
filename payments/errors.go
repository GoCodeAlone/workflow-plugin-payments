package payments

import "errors"

// Shared sentinel errors for PaymentProvider implementations.
var (
	ErrUnsupported    = errors.New("operation not supported by this provider")
	ErrNotFound       = errors.New("resource not found")
	ErrInvalidConfig  = errors.New("invalid provider configuration")
	ErrAuthFailed     = errors.New("authentication failed")
	ErrWebhookInvalid = errors.New("webhook signature invalid")
	// ErrStripeKeyMissing is returned by Stripe API methods that require a
	// secretKey when none was provided at provider init. Callers can detect it
	// with errors.Is. The fix is to set STRIPE_SECRET_KEY in the deployment.
	ErrStripeKeyMissing = errors.New("stripe provider not configured: secretKey missing — set STRIPE_SECRET_KEY env var")
)
