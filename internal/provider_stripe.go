package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/invoice"
	"github.com/stripe/stripe-go/v82/paymentintent"
	"github.com/stripe/stripe-go/v82/paymentmethod"
	"github.com/stripe/stripe-go/v82/payout"
	"github.com/stripe/stripe-go/v82/refund"
	"github.com/stripe/stripe-go/v82/subscription"
	"github.com/stripe/stripe-go/v82/transfer"
	"github.com/stripe/stripe-go/v82/webhook"
	"github.com/stripe/stripe-go/v82/webhookendpoint"
)

// Webhook-ensure mode constants for WebhookEndpointEnsureParams.Mode.
const (
	webhookEnsureModeEnsure  = "ensure"
	webhookEnsureModeReplace = "replace"
)

// errStripeKeyMissing aliases the exported payments.ErrStripeKeyMissing for
// internal use, so callers outside this package can also check it via errors.Is.
var errStripeKeyMissing = payments.ErrStripeKeyMissing

// stripeProvider implements payments.PaymentProvider using the Stripe API.
type stripeProvider struct {
	secretKey       string
	webhookSecret   string
	defaultCurrency string
	backends        stripe.Backends // nil = use stripe global defaults
}

// newStripeProvider creates a Stripe provider from config.
// secretKey is intentionally optional at init (deferred-config-init pattern):
// the provider initialises successfully with an empty key and returns
// payments.ErrStripeKeyMissing on Stripe API calls that require secretKey.
// Callers must supply secretKey before making live API requests.  This is a
// stopgap; the proper fix is setting the STRIPE_SECRET_KEY env var on the
// deployment.
func newStripeProvider(config map[string]any) (*stripeProvider, error) {
	secretKey, _ := config["secretKey"].(string)
	if secretKey == "" {
		secretKey, _ = config["secret_key"].(string)
	}
	if secretKey == "" {
		log.Printf("[WARN] stripe provider configured without secretKey — payment API calls will fail until set")
	}
	webhookSecret, _ := config["webhookSecret"].(string)
	if webhookSecret == "" {
		webhookSecret, _ = config["webhook_secret"].(string)
	}
	defaultCurrency, _ := config["defaultCurrency"].(string)
	if defaultCurrency == "" {
		defaultCurrency, _ = config["default_currency"].(string)
	}
	if defaultCurrency == "" {
		defaultCurrency = "usd"
	}
	return &stripeProvider{
		secretKey:       secretKey,
		webhookSecret:   webhookSecret,
		defaultCurrency: defaultCurrency,
	}, nil
}

// key returns a stripe.Params with the secret key set.
func (p *stripeProvider) params() *stripe.Params {
	params := &stripe.Params{}
	params.SetStripeAccount("") // clear any connected account
	return params
}

// setKey sets the global stripe key for the call.
func (p *stripeProvider) setKey() {
	stripe.Key = p.secretKey
}

// checkKey returns payments.ErrStripeKeyMissing if the provider was initialised
// without a secretKey.  Every API-call method calls this before touching the Stripe SDK.
func (p *stripeProvider) checkKey() error {
	if p.secretKey == "" {
		return errStripeKeyMissing
	}
	return nil
}

// backendFor returns the configured backend for the given API, or nil to use default.
func (p *stripeProvider) backendFor(api stripe.SupportedBackend) stripe.Backend {
	if p.backends.API != nil && api == stripe.APIBackend {
		return p.backends.API
	}
	return nil
}

func (p *stripeProvider) CreateCharge(ctx context.Context, cp payments.ChargeParams) (*payments.Charge, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	currency := cp.Currency
	if currency == "" {
		currency = p.defaultCurrency
	}

	params := &stripe.PaymentIntentParams{
		Amount:      stripe.Int64(cp.Amount),
		Currency:    stripe.String(currency),
		Description: stripe.String(cp.Description),
	}
	if cp.CustomerID != "" {
		params.Customer = stripe.String(cp.CustomerID)
	}
	if cp.CaptureMethod == "manual" {
		params.CaptureMethod = stripe.String(string(stripe.PaymentIntentCaptureMethodManual))
	} else {
		params.CaptureMethod = stripe.String(string(stripe.PaymentIntentCaptureMethodAutomatic))
	}
	for k, v := range cp.Metadata {
		params.AddMetadata(k, v)
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe CreateCharge: %w", err)
	}
	return &payments.Charge{
		ID:           pi.ID,
		ClientSecret: pi.ClientSecret,
		Status:       string(pi.Status),
		Amount:       pi.Amount,
		Currency:     string(pi.Currency),
	}, nil
}

func (p *stripeProvider) CaptureCharge(_ context.Context, chargeID string, amount int64) (*payments.Charge, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.PaymentIntentCaptureParams{}
	if amount > 0 {
		params.AmountToCapture = stripe.Int64(amount)
	}
	pi, err := paymentintent.Capture(chargeID, params)
	if err != nil {
		return nil, fmt.Errorf("stripe CaptureCharge: %w", err)
	}
	return &payments.Charge{
		ID:     pi.ID,
		Status: string(pi.Status),
		Amount: pi.Amount,
	}, nil
}

func (p *stripeProvider) RefundCharge(_ context.Context, rp payments.RefundParams) (*payments.Refund, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(rp.ChargeID),
	}
	if rp.Amount > 0 {
		params.Amount = stripe.Int64(rp.Amount)
	}
	if rp.Reason != "" {
		params.Reason = stripe.String(rp.Reason)
	}
	re, err := refund.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe RefundCharge: %w", err)
	}
	return &payments.Refund{
		ID:     re.ID,
		Status: string(re.Status),
		Amount: re.Amount,
	}, nil
}

func (p *stripeProvider) EnsureCustomer(_ context.Context, cp payments.CustomerParams) (*payments.Customer, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	// Search for existing customer by email.
	listParams := &stripe.CustomerListParams{}
	listParams.Filters.AddFilter("email", "", cp.Email)
	listParams.Limit = stripe.Int64(1)
	it := customer.List(listParams)
	for it.Next() {
		c := it.Customer()
		return &payments.Customer{
			ID:    c.ID,
			Email: string(c.Email),
			Name:  c.Name,
		}, nil
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("stripe EnsureCustomer list: %w", err)
	}

	// Create new customer.
	newParams := &stripe.CustomerParams{
		Email: stripe.String(cp.Email),
		Name:  stripe.String(cp.Name),
	}
	for k, v := range cp.Metadata {
		newParams.AddMetadata(k, v)
	}
	c, err := customer.New(newParams)
	if err != nil {
		return nil, fmt.Errorf("stripe EnsureCustomer create: %w", err)
	}
	return &payments.Customer{
		ID:    c.ID,
		Email: string(c.Email),
		Name:  c.Name,
	}, nil
}

func (p *stripeProvider) CreateSubscription(_ context.Context, sp payments.SubscriptionParams) (*payments.Subscription, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(sp.CustomerID),
		Items: []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(sp.PriceID)},
		},
	}
	for k, v := range sp.Metadata {
		params.AddMetadata(k, v)
	}
	sub, err := subscription.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe CreateSubscription: %w", err)
	}
	return &payments.Subscription{
		ID:     sub.ID,
		Status: string(sub.Status),
	}, nil
}

func (p *stripeProvider) CancelSubscription(_ context.Context, subscriptionID string, cancelAtPeriodEnd bool) (*payments.Subscription, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	if cancelAtPeriodEnd {
		params := &stripe.SubscriptionParams{
			CancelAtPeriodEnd: stripe.Bool(true),
		}
		sub, err := subscription.Update(subscriptionID, params)
		if err != nil {
			return nil, fmt.Errorf("stripe CancelSubscription (at period end): %w", err)
		}
		return &payments.Subscription{
			ID:     sub.ID,
			Status: string(sub.Status),
		}, nil
	}
	sub, err := subscription.Cancel(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("stripe CancelSubscription: %w", err)
	}
	return &payments.Subscription{
		ID:     sub.ID,
		Status: string(sub.Status),
	}, nil
}

func (p *stripeProvider) UpdateSubscription(_ context.Context, subscriptionID string, up payments.SubscriptionUpdateParams) (*payments.Subscription, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.SubscriptionParams{}
	if up.PriceID != "" {
		// Retrieve current subscription to get item ID.
		existing, err := subscription.Get(subscriptionID, nil)
		if err != nil {
			return nil, fmt.Errorf("stripe UpdateSubscription get: %w", err)
		}
		if len(existing.Items.Data) > 0 {
			params.Items = []*stripe.SubscriptionItemsParams{
				{
					ID:    stripe.String(existing.Items.Data[0].ID),
					Price: stripe.String(up.PriceID),
				},
			}
		}
	}
	for k, v := range up.Metadata {
		params.AddMetadata(k, v)
	}
	sub, err := subscription.Update(subscriptionID, params)
	if err != nil {
		return nil, fmt.Errorf("stripe UpdateSubscription: %w", err)
	}
	return &payments.Subscription{
		ID:     sub.ID,
		Status: string(sub.Status),
	}, nil
}

func (p *stripeProvider) CreateCheckoutSession(_ context.Context, cp payments.CheckoutParams) (*payments.CheckoutSession, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	mode := cp.Mode
	if mode == "" {
		mode = "subscription"
	}
	params := &stripe.CheckoutSessionParams{
		Customer:   stripe.String(cp.CustomerID),
		Mode:       stripe.String(mode),
		SuccessURL: stripe.String(cp.SuccessURL),
		CancelURL:  stripe.String(cp.CancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(cp.PriceID),
				Quantity: stripe.Int64(1),
			},
		},
	}
	sess, err := checkoutsession.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe CreateCheckoutSession: %w", err)
	}
	return &payments.CheckoutSession{
		ID:  sess.ID,
		URL: sess.URL,
	}, nil
}

func (p *stripeProvider) CreatePortalSession(_ context.Context, customerID, returnURL string) (*payments.PortalSession, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}
	sess, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe CreatePortalSession: %w", err)
	}
	return &payments.PortalSession{
		ID:  sess.ID,
		URL: sess.URL,
	}, nil
}

func (p *stripeProvider) VerifyWebhook(_ context.Context, payload []byte, headers http.Header) (*payments.WebhookEvent, error) {
	if p.webhookSecret == "" {
		return nil, fmt.Errorf("stripe VerifyWebhook: webhookSecret not configured")
	}
	event, err := webhook.ConstructEventWithOptions(payload, headers.Get("Stripe-Signature"), p.webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		return nil, payments.ErrWebhookInvalid
	}
	data := event.Data.Object
	return &payments.WebhookEvent{
		ID:       event.ID,
		Type:     string(event.Type),
		Data:     data,
		Metadata: stripeMetadata(data),
	}, nil
}

func stripeMetadata(data map[string]any) map[string]string {
	raw, ok := data["metadata"].(map[string]any)
	if !ok {
		return nil
	}
	metadata := make(map[string]string, len(raw))
	for key, value := range raw {
		if str, ok := value.(string); ok {
			metadata[key] = str
		}
	}
	return metadata
}

func (p *stripeProvider) CreateTransfer(_ context.Context, tp payments.TransferParams) (*payments.Transfer, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.TransferParams{
		Amount:      stripe.Int64(tp.Amount),
		Currency:    stripe.String(tp.Currency),
		Destination: stripe.String(tp.DestinationAccountID),
	}
	if tp.Description != "" {
		params.Description = stripe.String(tp.Description)
	}
	for k, v := range tp.Metadata {
		params.AddMetadata(k, v)
	}
	t, err := transfer.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe CreateTransfer: %w", err)
	}
	return &payments.Transfer{
		ID:     t.ID,
		Status: "paid",
	}, nil
}

func (p *stripeProvider) CreatePayout(_ context.Context, pp payments.PayoutParams) (*payments.Payout, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.PayoutParams{
		Amount:      stripe.Int64(pp.Amount),
		Currency:    stripe.String(pp.Currency),
		Description: stripe.String(pp.Description),
	}
	if pp.DestinationBankID != "" {
		params.Destination = stripe.String(pp.DestinationBankID)
	}
	po, err := payout.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe CreatePayout: %w", err)
	}
	return &payments.Payout{
		ID:     po.ID,
		Status: string(po.Status),
	}, nil
}

func (p *stripeProvider) ListInvoices(_ context.Context, lp payments.InvoiceListParams) ([]*payments.Invoice, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.InvoiceListParams{}
	if lp.CustomerID != "" {
		params.Customer = stripe.String(lp.CustomerID)
	}
	if lp.Limit > 0 {
		params.Limit = stripe.Int64(lp.Limit)
	}
	if lp.Status != "" {
		params.Status = stripe.String(lp.Status)
	}

	var invoices []*payments.Invoice
	it := invoice.List(params)
	for it.Next() {
		inv := it.Invoice()
		customerID := ""
		if inv.Customer != nil {
			customerID = inv.Customer.ID
		}
		invoices = append(invoices, &payments.Invoice{
			ID:         inv.ID,
			CustomerID: customerID,
			Amount:     inv.AmountDue,
			Currency:   string(inv.Currency),
			Status:     string(inv.Status),
			Created:    inv.Created,
		})
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("stripe ListInvoices: %w", err)
	}
	return invoices, nil
}

func (p *stripeProvider) AttachPaymentMethod(_ context.Context, customerID, paymentMethodID string) (*payments.PaymentMethod, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(customerID),
	}
	pm, err := paymentmethod.Attach(paymentMethodID, params)
	if err != nil {
		return nil, fmt.Errorf("stripe AttachPaymentMethod: %w", err)
	}
	return &payments.PaymentMethod{
		ID:         pm.ID,
		Type:       string(pm.Type),
		CustomerID: customerID,
	}, nil
}

func (p *stripeProvider) ListPaymentMethods(_ context.Context, customerID, pmType string) ([]*payments.PaymentMethod, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	p.setKey()
	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerID),
	}
	if pmType != "" {
		params.Type = stripe.String(pmType)
	}
	var methods []*payments.PaymentMethod
	it := paymentmethod.List(params)
	for it.Next() {
		pm := it.PaymentMethod()
		methods = append(methods, &payments.PaymentMethod{
			ID:         pm.ID,
			Type:       string(pm.Type),
			CustomerID: customerID,
		})
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("stripe ListPaymentMethods: %w", err)
	}
	return methods, nil
}

func (p *stripeProvider) CalculateFees(amount int64, _ string, platformFeePercent float64) (*payments.FeeBreakdown, error) {
	// Stripe: 2.9% + $0.30 (30 cents)
	const processingFeeRate = 0.029
	const processingFeeFixed = int64(30) // cents

	processingFee := int64(math.Ceil(float64(amount)*processingFeeRate)) + processingFeeFixed
	platformFee := int64(math.Ceil(float64(amount) * platformFeePercent / 100.0))
	totalCharge := amount
	contributionAmount := amount - processingFee - platformFee

	return &payments.FeeBreakdown{
		ContributionAmount: contributionAmount,
		ProcessingFee:      processingFee,
		PlatformFee:        platformFee,
		TotalCharge:        totalCharge,
		ProcessingFeeRate:  processingFeeRate,
		ProcessingFeeFixed: processingFeeFixed,
	}, nil
}

// WebhookEndpointEnsure idempotently provisions a Stripe webhook endpoint.
// Behaviour:
//   - URL match + events identical → no-op (Created=false; SigningSecret="").
//   - URL match + events drift, mode=ensure → POST update; EventsDrift=true.
//   - No URL match → POST create; SigningSecret populated.
//   - Mode=replace + URL match → DELETE then POST; rotates signing secret.
//
// Events are sort+dedup+lowercased before equality so reorder/duplicate
// in the requested list does not register as drift.
//
// Per-call clients use p.backendFor(stripe.APIBackend) so injected test
// backends are honored; fall back to a freshly-resolved package backend
// when no test override is configured.
func (p *stripeProvider) WebhookEndpointEnsure(_ context.Context, in payments.WebhookEndpointEnsureParams) (*payments.WebhookEndpointEnsureResult, error) {
	if err := p.checkKey(); err != nil {
		return nil, err
	}
	if in.URL == "" {
		return nil, errors.New("webhook ensure: url is required")
	}
	if len(in.Events) == 0 {
		return nil, errors.New("webhook ensure: events list must contain at least one event")
	}
	mode := in.Mode
	if mode == "" {
		mode = webhookEnsureModeEnsure
	}
	if mode != webhookEnsureModeEnsure && mode != webhookEnsureModeReplace {
		return nil, fmt.Errorf("webhook ensure: mode must be %q or %q (got %q)",
			webhookEnsureModeEnsure, webhookEnsureModeReplace, mode)
	}

	wantEvents := normalizeWebhookEvents(in.Events)
	client := p.webhookEndpointClient()

	existing, err := findWebhookByURL(client, in.URL)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}

	switch mode {
	case webhookEnsureModeReplace:
		if existing != nil {
			if _, err := client.Del(existing.ID, nil); err != nil {
				return nil, fmt.Errorf("delete existing endpoint %s for replace: %w", existing.ID, err)
			}
			log.Printf("payments-plugin: rotated webhook endpoint id=%s url=%s", existing.ID, in.URL)
		}
		return createWebhookEndpoint(client, in, wantEvents)

	case webhookEnsureModeEnsure:
		if existing == nil {
			return createWebhookEndpoint(client, in, wantEvents)
		}
		gotEvents := normalizeWebhookEvents(existing.EnabledEvents)
		if webhookEventsEqual(wantEvents, gotEvents) {
			return &payments.WebhookEndpointEnsureResult{
				EndpointID:    existing.ID,
				Created:       false,
				EventsDrift:   false,
				SigningSecret: "",
			}, nil
		}
		params := &stripe.WebhookEndpointParams{
			EnabledEvents: stripeStringSlice(wantEvents),
		}
		if in.Description != "" {
			params.Description = stripe.String(in.Description)
		}
		updated, err := client.Update(existing.ID, params)
		if err != nil {
			return nil, fmt.Errorf("update endpoint %s events: %w", existing.ID, err)
		}
		return &payments.WebhookEndpointEnsureResult{
			EndpointID:    updated.ID,
			Created:       false,
			EventsDrift:   true,
			SigningSecret: "",
		}, nil
	}
	return nil, fmt.Errorf("webhook ensure: unreachable mode %q", mode)
}

// webhookEndpointClient builds a Client honoring any injected test backend,
// falling back to the stripe-go default backend when none is configured.
func (p *stripeProvider) webhookEndpointClient() webhookendpoint.Client {
	be := p.backendFor(stripe.APIBackend)
	if be == nil {
		be = stripe.GetBackend(stripe.APIBackend)
	}
	return webhookendpoint.Client{B: be, Key: p.secretKey}
}

func findWebhookByURL(client webhookendpoint.Client, url string) (*stripe.WebhookEndpoint, error) {
	listParams := &stripe.WebhookEndpointListParams{}
	listParams.Limit = stripe.Int64(100)
	iter := client.List(listParams)
	for iter.Next() {
		ep := iter.WebhookEndpoint()
		if ep != nil && ep.URL == url {
			return ep, nil
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func createWebhookEndpoint(client webhookendpoint.Client, in payments.WebhookEndpointEnsureParams, events []string) (*payments.WebhookEndpointEnsureResult, error) {
	params := &stripe.WebhookEndpointParams{
		URL:           stripe.String(in.URL),
		EnabledEvents: stripeStringSlice(events),
	}
	if in.Description != "" {
		params.Description = stripe.String(in.Description)
	}
	created, err := client.New(params)
	if err != nil {
		return nil, fmt.Errorf("create webhook endpoint: %w", err)
	}
	return &payments.WebhookEndpointEnsureResult{
		EndpointID:    created.ID,
		Created:       true,
		EventsDrift:   false,
		SigningSecret: created.Secret,
	}, nil
}

// normalizeWebhookEvents sorts, lowercases, and deduplicates an event list so
// equality comparison between requested and existing events is order- and
// duplicate-independent.
func normalizeWebhookEvents(events []string) []string {
	seen := make(map[string]struct{}, len(events))
	out := make([]string, 0, len(events))
	for _, e := range events {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if _, dup := seen[e]; dup {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

func webhookEventsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stripeStringSlice(s []string) []*string {
	out := make([]*string, len(s))
	for i, v := range s {
		out[i] = stripe.String(v)
	}
	return out
}
