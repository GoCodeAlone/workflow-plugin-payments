// Package payments defines the provider interface and shared types for the
// workflow-plugin-payments external plugin.
package payments

// ChargeParams holds parameters for creating a payment charge.
type ChargeParams struct {
	Amount        int64             // amount in smallest currency unit (e.g. cents)
	Currency      string            // ISO 4217 currency code (e.g. "usd")
	CustomerID    string            // optional customer ID
	CaptureMethod string            // "automatic" or "manual"
	Description   string
	Metadata      map[string]string
}

// Charge represents a payment charge result.
type Charge struct {
	ID           string
	ClientSecret string
	Status       string
	Amount       int64
	Currency     string
}

// RefundParams holds parameters for refunding a charge.
type RefundParams struct {
	ChargeID string
	Amount   int64  // 0 = full refund
	Reason   string // "duplicate", "fraudulent", "requested_by_customer"
}

// Refund represents a refund result.
type Refund struct {
	ID     string
	Status string
	Amount int64
}

// CustomerParams holds parameters for ensuring a customer record.
type CustomerParams struct {
	Email    string
	Name     string
	Metadata map[string]string
}

// Customer represents a payment provider customer.
type Customer struct {
	ID    string
	Email string
	Name  string
}

// SubscriptionParams holds parameters for creating a subscription.
type SubscriptionParams struct {
	CustomerID string
	PriceID    string
	Metadata   map[string]string
}

// SubscriptionUpdateParams holds parameters for updating a subscription.
type SubscriptionUpdateParams struct {
	PriceID  string
	Metadata map[string]string
}

// Subscription represents a subscription result.
type Subscription struct {
	ID     string
	Status string
}

// CheckoutParams holds parameters for creating a hosted checkout session.
type CheckoutParams struct {
	CustomerID string
	PriceID    string
	SuccessURL string
	CancelURL  string
	Mode       string // "subscription", "payment", "setup"
}

// CheckoutSession represents a hosted checkout session.
type CheckoutSession struct {
	ID  string
	URL string
}

// PortalSession represents a customer portal session.
type PortalSession struct {
	ID  string
	URL string
}

// WebhookEvent represents a parsed webhook event.
type WebhookEvent struct {
	ID       string
	Type     string
	Data     map[string]any
	Metadata map[string]string
}

// TransferParams holds parameters for a platform transfer.
type TransferParams struct {
	Amount               int64
	Currency             string
	DestinationAccountID string
	Description          string
	Metadata             map[string]string
}

// Transfer represents a transfer result.
type Transfer struct {
	ID     string
	Status string
}

// PayoutParams holds parameters for a payout to an external bank.
type PayoutParams struct {
	Amount            int64
	Currency          string
	DestinationBankID string
	Description       string
}

// Payout represents a payout result.
type Payout struct {
	ID     string
	Status string
}

// FeeBreakdown contains the calculated fee components for a transaction.
type FeeBreakdown struct {
	ContributionAmount int64   // net amount received after all fees
	ProcessingFee      int64   // payment processor fee (in smallest unit)
	PlatformFee        int64   // platform's additional fee
	TotalCharge        int64   // total amount charged to the customer
	ProcessingFeeRate  float64 // e.g. 0.029
	ProcessingFeeFixed int64   // e.g. 30 cents
}

// InvoiceListParams holds parameters for listing invoices.
type InvoiceListParams struct {
	CustomerID string
	Limit      int64
	Status     string
}

// Invoice represents a single invoice.
type Invoice struct {
	ID         string
	CustomerID string
	Amount     int64
	Currency   string
	Status     string
	Created    int64
}

// PaymentMethod represents a stored payment method.
type PaymentMethod struct {
	ID         string
	Type       string
	CustomerID string
}
