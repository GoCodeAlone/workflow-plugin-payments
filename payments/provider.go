package payments

import (
	"context"
	"net/http"
)

// PaymentProvider is the unified interface all payment providers must implement.
type PaymentProvider interface {
	// CreateCharge creates a payment intent / order.
	CreateCharge(ctx context.Context, p ChargeParams) (*Charge, error)
	// CaptureCharge captures a previously authorized charge.
	CaptureCharge(ctx context.Context, chargeID string, amount int64) (*Charge, error)
	// RefundCharge refunds a completed charge.
	RefundCharge(ctx context.Context, p RefundParams) (*Refund, error)

	// EnsureCustomer returns an existing customer by email or creates a new one.
	EnsureCustomer(ctx context.Context, p CustomerParams) (*Customer, error)

	// CreateSubscription creates a recurring subscription.
	CreateSubscription(ctx context.Context, p SubscriptionParams) (*Subscription, error)
	// CancelSubscription cancels an active subscription.
	CancelSubscription(ctx context.Context, subscriptionID string, cancelAtPeriodEnd bool) (*Subscription, error)
	// UpdateSubscription updates an existing subscription.
	UpdateSubscription(ctx context.Context, subscriptionID string, p SubscriptionUpdateParams) (*Subscription, error)

	// CreateCheckoutSession creates a hosted checkout page session.
	CreateCheckoutSession(ctx context.Context, p CheckoutParams) (*CheckoutSession, error)
	// CreatePortalSession creates a customer billing-portal session.
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (*PortalSession, error)

	// VerifyWebhook validates an inbound webhook payload and returns the parsed event.
	// headers should contain the provider-specific signature headers (e.g. Stripe-Signature,
	// PayPal-Transmission-Id, PayPal-Transmission-Sig, etc.).
	VerifyWebhook(ctx context.Context, payload []byte, headers http.Header) (*WebhookEvent, error)

	// CreateTransfer initiates a platform-level transfer to a connected account.
	CreateTransfer(ctx context.Context, p TransferParams) (*Transfer, error)
	// CreatePayout initiates a payout to an external bank account.
	CreatePayout(ctx context.Context, p PayoutParams) (*Payout, error)

	// ListInvoices lists invoices matching the given parameters.
	ListInvoices(ctx context.Context, p InvoiceListParams) ([]*Invoice, error)

	// AttachPaymentMethod attaches a payment method to a customer.
	AttachPaymentMethod(ctx context.Context, customerID, paymentMethodID string) (*PaymentMethod, error)
	// ListPaymentMethods lists payment methods for a customer.
	ListPaymentMethods(ctx context.Context, customerID, pmType string) ([]*PaymentMethod, error)

	// CalculateFees computes the fee breakdown for a given transaction amount.
	CalculateFees(amount int64, currency string, platformFeePercent float64) (*FeeBreakdown, error)

	// WebhookEndpointEnsure idempotently provisions a webhook endpoint on the
	// provider. Returns Created=true with a populated SigningSecret on
	// fresh-create. On URL match with identical events the call is a no-op
	// (Created=false, SigningSecret=""). Events drift triggers an update
	// (Created=false, EventsDrift=true). Mode "replace" rotates the signing
	// secret via delete+create; never invoked unless explicitly requested.
	WebhookEndpointEnsure(ctx context.Context, p WebhookEndpointEnsureParams) (*WebhookEndpointEnsureResult, error)
}
