package payments

import "context"

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
	VerifyWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error)

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
}
