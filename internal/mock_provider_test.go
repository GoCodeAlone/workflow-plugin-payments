package internal

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

// mockProvider is an in-memory PaymentProvider for testing.
type mockProvider struct {
	mu          sync.Mutex
	charges     map[string]*payments.Charge
	refunds     map[string]*payments.Refund
	customers   map[string]*payments.Customer
	subs        map[string]*payments.Subscription
	sessions    map[string]*payments.CheckoutSession
	portals     map[string]*payments.PortalSession
	transfers   map[string]*payments.Transfer
	payouts     map[string]*payments.Payout
	invoices    []*payments.Invoice
	pmethods    map[string]*payments.PaymentMethod
	webhookErr  error
	counter     int
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		charges:   make(map[string]*payments.Charge),
		refunds:   make(map[string]*payments.Refund),
		customers: make(map[string]*payments.Customer),
		subs:      make(map[string]*payments.Subscription),
		sessions:  make(map[string]*payments.CheckoutSession),
		portals:   make(map[string]*payments.PortalSession),
		transfers: make(map[string]*payments.Transfer),
		payouts:   make(map[string]*payments.Payout),
		pmethods:  make(map[string]*payments.PaymentMethod),
	}
}

func (m *mockProvider) nextID(prefix string) string {
	m.counter++
	return fmt.Sprintf("%s_mock_%d", prefix, m.counter)
}

func (m *mockProvider) CreateCharge(_ context.Context, p payments.ChargeParams) (*payments.Charge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := &payments.Charge{
		ID:           m.nextID("pi"),
		ClientSecret: m.nextID("pi_secret"),
		Status:       "requires_confirmation",
		Amount:       p.Amount,
		Currency:     p.Currency,
	}
	if p.CaptureMethod == "manual" {
		c.Status = "requires_capture"
	} else {
		c.Status = "succeeded"
	}
	m.charges[c.ID] = c
	return c, nil
}

func (m *mockProvider) CaptureCharge(_ context.Context, chargeID string, amount int64) (*payments.Charge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.charges[chargeID]
	if !ok {
		return nil, fmt.Errorf("charge %q not found", chargeID)
	}
	c.Status = "succeeded"
	if amount > 0 {
		c.Amount = amount
	}
	return c, nil
}

func (m *mockProvider) RefundCharge(_ context.Context, p payments.RefundParams) (*payments.Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	amount := p.Amount
	if amount == 0 {
		if c, ok := m.charges[p.ChargeID]; ok {
			amount = c.Amount
		}
	}
	re := &payments.Refund{
		ID:     m.nextID("re"),
		Status: "succeeded",
		Amount: amount,
	}
	m.refunds[re.ID] = re
	return re, nil
}

func (m *mockProvider) EnsureCustomer(_ context.Context, p payments.CustomerParams) (*payments.Customer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.customers {
		if c.Email == p.Email {
			return c, nil
		}
	}
	c := &payments.Customer{
		ID:    m.nextID("cus"),
		Email: p.Email,
		Name:  p.Name,
	}
	m.customers[c.ID] = c
	return c, nil
}

func (m *mockProvider) CreateSubscription(_ context.Context, p payments.SubscriptionParams) (*payments.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub := &payments.Subscription{
		ID:     m.nextID("sub"),
		Status: "active",
	}
	m.subs[sub.ID] = sub
	return sub, nil
}

func (m *mockProvider) CancelSubscription(_ context.Context, subscriptionID string, _ bool) (*payments.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subs[subscriptionID]
	if !ok {
		return nil, fmt.Errorf("subscription %q not found", subscriptionID)
	}
	sub.Status = "canceled"
	return sub, nil
}

func (m *mockProvider) UpdateSubscription(_ context.Context, subscriptionID string, p payments.SubscriptionUpdateParams) (*payments.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subs[subscriptionID]
	if !ok {
		return nil, fmt.Errorf("subscription %q not found", subscriptionID)
	}
	return sub, nil
}

func (m *mockProvider) CreateCheckoutSession(_ context.Context, p payments.CheckoutParams) (*payments.CheckoutSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess := &payments.CheckoutSession{
		ID:  m.nextID("cs"),
		URL: "https://checkout.example.com/" + m.nextID("sess"),
	}
	m.sessions[sess.ID] = sess
	return sess, nil
}

func (m *mockProvider) CreatePortalSession(_ context.Context, customerID, returnURL string) (*payments.PortalSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess := &payments.PortalSession{
		ID:  m.nextID("bps"),
		URL: "https://portal.example.com/" + customerID,
	}
	m.portals[sess.ID] = sess
	return sess, nil
}

func (m *mockProvider) VerifyWebhook(_ context.Context, payload []byte, _ http.Header) (*payments.WebhookEvent, error) {
	if m.webhookErr != nil {
		return nil, m.webhookErr
	}
	return &payments.WebhookEvent{
		ID:   m.nextID("evt"),
		Type: "payment_intent.succeeded",
		Data: map[string]any{"raw": string(payload)},
	}, nil
}

func (m *mockProvider) CreateTransfer(_ context.Context, p payments.TransferParams) (*payments.Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := &payments.Transfer{
		ID:     m.nextID("tr"),
		Status: "paid",
	}
	m.transfers[t.ID] = t
	return t, nil
}

func (m *mockProvider) CreatePayout(_ context.Context, p payments.PayoutParams) (*payments.Payout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	po := &payments.Payout{
		ID:     m.nextID("po"),
		Status: "pending",
	}
	m.payouts[po.ID] = po
	return po, nil
}

func (m *mockProvider) ListInvoices(_ context.Context, p payments.InvoiceListParams) ([]*payments.Invoice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.invoices, nil
}

func (m *mockProvider) AttachPaymentMethod(_ context.Context, customerID, paymentMethodID string) (*payments.PaymentMethod, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pm := &payments.PaymentMethod{
		ID:         paymentMethodID,
		Type:       "card",
		CustomerID: customerID,
	}
	m.pmethods[pm.ID] = pm
	return pm, nil
}

func (m *mockProvider) ListPaymentMethods(_ context.Context, customerID, pmType string) ([]*payments.PaymentMethod, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*payments.PaymentMethod
	for _, pm := range m.pmethods {
		if pm.CustomerID == customerID {
			result = append(result, pm)
		}
	}
	return result, nil
}

func (m *mockProvider) CalculateFees(amount int64, currency string, platformFeePercent float64) (*payments.FeeBreakdown, error) {
	// Use Stripe formula for mock.
	const feeRate = 0.029
	const feeFixed = int64(30)
	processingFee := int64(float64(amount)*feeRate) + feeFixed
	platformFee := int64(float64(amount) * platformFeePercent / 100.0)
	return &payments.FeeBreakdown{
		ContributionAmount: amount - processingFee - platformFee,
		ProcessingFee:      processingFee,
		PlatformFee:        platformFee,
		TotalCharge:        amount,
		ProcessingFeeRate:  feeRate,
		ProcessingFeeFixed: feeFixed,
	}, nil
}
