package internal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	stripe "github.com/stripe/stripe-go/v82"
)

// newTestStripeProvider creates a stripeProvider wired to a mock HTTP server.
func newTestStripeProvider(t *testing.T, handler http.HandlerFunc) (*stripeProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// Override Stripe backend to use the test server.
	backend := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		URL:           stripe.String(srv.URL),
		LeveledLogger: &stripe.LeveledLogger{Level: stripe.LevelNull},
	})
	stripe.SetBackend(stripe.APIBackend, backend)
	t.Cleanup(func() {
		stripe.SetBackend(stripe.APIBackend, nil)
	})

	p := &stripeProvider{
		secretKey:       "sk_test_fake",
		webhookSecret:   "whsec_test",
		defaultCurrency: "usd",
	}
	stripe.Key = p.secretKey
	return p, srv
}

func stripePaymentIntentResponse(id, status string, amount int64) map[string]any {
	return map[string]any{
		"id":             id,
		"object":         "payment_intent",
		"amount":         amount,
		"currency":       "usd",
		"status":         status,
		"client_secret":  id + "_secret",
		"capture_method": "automatic",
		"livemode":       false,
	}
}

func stripeCustomerResponse(id, email string) map[string]any {
	return map[string]any{
		"id":     id,
		"object": "customer",
		"email":  email,
		"name":   "Test User",
	}
}

func stripeListResponse(data []any) map[string]any {
	return map[string]any{
		"object":   "list",
		"data":     data,
		"has_more": false,
		"url":      "/v1/customers",
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// buildStripeSignatureHeader constructs a valid Stripe webhook Stripe-Signature header.
func buildStripeSignatureHeader(payload []byte, ts int64, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d", ts)))
	mac.Write([]byte("."))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}

func TestStripeCreateCharge_Auto(t *testing.T) {
	p, _ := newTestStripeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/payment_intents" && r.Method == "POST" {
			writeJSON(w, stripePaymentIntentResponse("pi_test1", "succeeded", 1000))
			return
		}
		http.NotFound(w, r)
	})

	charge, err := p.CreateCharge(context.Background(), chargeParamsAuto())
	if err != nil {
		t.Fatal(err)
	}
	if charge.ID != "pi_test1" {
		t.Errorf("expected pi_test1, got %s", charge.ID)
	}
	if charge.Status != "succeeded" {
		t.Errorf("expected succeeded, got %s", charge.Status)
	}
}

func TestStripeCreateCharge_Manual(t *testing.T) {
	p, _ := newTestStripeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/payment_intents" && r.Method == "POST" {
			writeJSON(w, stripePaymentIntentResponse("pi_test2", "requires_capture", 5000))
			return
		}
		http.NotFound(w, r)
	})

	charge, err := p.CreateCharge(context.Background(), chargeParamsManual())
	if err != nil {
		t.Fatal(err)
	}
	if charge.Status != "requires_capture" {
		t.Errorf("expected requires_capture, got %s", charge.Status)
	}
}

func TestStripeRefundCharge_Full(t *testing.T) {
	p, _ := newTestStripeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/refunds" && r.Method == "POST" {
			writeJSON(w, map[string]any{
				"id":     "re_test1",
				"object": "refund",
				"status": "succeeded",
				"amount": int64(1000),
			})
			return
		}
		http.NotFound(w, r)
	})

	re, err := p.RefundCharge(context.Background(), payments.RefundParams{
		ChargeID: "pi_test1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if re.Status != "succeeded" {
		t.Errorf("expected succeeded, got %s", re.Status)
	}
}

func TestStripeRefundCharge_Partial(t *testing.T) {
	p, _ := newTestStripeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/refunds" {
			writeJSON(w, map[string]any{
				"id":     "re_partial",
				"object": "refund",
				"status": "succeeded",
				"amount": int64(500),
			})
			return
		}
		http.NotFound(w, r)
	})

	re, err := p.RefundCharge(context.Background(), payments.RefundParams{
		ChargeID: "pi_test1",
		Amount:   500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if re.ID != "re_partial" {
		t.Errorf("expected re_partial, got %s", re.ID)
	}
}

func TestStripeEnsureCustomer_CreateNew(t *testing.T) {
	p, _ := newTestStripeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/customers" && r.Method == "GET":
			writeJSON(w, stripeListResponse(nil))
		case r.URL.Path == "/v1/customers" && r.Method == "POST":
			writeJSON(w, stripeCustomerResponse("cus_new1", "new@example.com"))
		default:
			http.NotFound(w, r)
		}
	})

	cust, err := p.EnsureCustomer(context.Background(), payments.CustomerParams{
		Email: "new@example.com",
		Name:  "New User",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cust.ID != "cus_new1" {
		t.Errorf("expected cus_new1, got %s", cust.ID)
	}
}

func TestStripeEnsureCustomer_ReturnExisting(t *testing.T) {
	p, _ := newTestStripeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/customers" && r.Method == "GET" {
			writeJSON(w, stripeListResponse([]any{
				stripeCustomerResponse("cus_existing1", "existing@example.com"),
			}))
			return
		}
		http.NotFound(w, r)
	})

	cust, err := p.EnsureCustomer(context.Background(), payments.CustomerParams{
		Email: "existing@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cust.ID != "cus_existing1" {
		t.Errorf("expected cus_existing1, got %s", cust.ID)
	}
}

func TestStripeCalculateFees(t *testing.T) {
	p := &stripeProvider{secretKey: "sk_test", defaultCurrency: "usd"}

	fees, err := p.CalculateFees(10000, "usd", 5.0)
	if err != nil {
		t.Fatal(err)
	}
	// 2.9% of 10000 = 290, + 30 = 320
	if fees.ProcessingFee < 320 {
		t.Errorf("expected processing fee >= 320, got %d", fees.ProcessingFee)
	}
	// 5% of 10000 = 500
	if fees.PlatformFee != 500 {
		t.Errorf("expected platform fee 500, got %d", fees.PlatformFee)
	}
	if fees.TotalCharge != 10000 {
		t.Errorf("expected total 10000, got %d", fees.TotalCharge)
	}
}

func TestStripeVerifyWebhook_Valid(t *testing.T) {
	secret := "whsec_test123"
	payload := []byte(`{"id":"evt_1","type":"payment_intent.succeeded","data":{"object":{}}}`)
	ts := time.Now().Unix()
	sigHeader := buildStripeSignatureHeader(payload, ts, secret)

	p := &stripeProvider{secretKey: "sk_test", webhookSecret: secret}
	headers := http.Header{"Stripe-Signature": []string{sigHeader}}
	event, err := p.VerifyWebhook(context.Background(), payload, headers)
	if err != nil {
		t.Fatalf("expected valid webhook, got: %v", err)
	}
	if event.Type != "payment_intent.succeeded" {
		t.Errorf("expected payment_intent.succeeded, got %s", event.Type)
	}
}

func TestStripeVerifyWebhook_Invalid(t *testing.T) {
	p := &stripeProvider{secretKey: "sk_test", webhookSecret: "whsec_real"}
	headers := http.Header{"Stripe-Signature": []string{"t=1234,v1=badsig"}}
	_, err := p.VerifyWebhook(context.Background(), []byte(`{}`), headers)
	if err == nil {
		t.Error("expected error for invalid webhook")
	}
}

// TestStripeInitEmptySecretKey verifies that newStripeProvider succeeds when
// secretKey is absent (deferred-config-init pattern).
func TestStripeInitEmptySecretKey(t *testing.T) {
	p, err := newStripeProvider(map[string]any{})
	if err != nil {
		t.Fatalf("expected init to succeed with empty secretKey, got: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

// TestStripeAPICallWithEmptySecretKey verifies that each API-call method
// returns errStripeKeyMissing when secretKey was not supplied at init.
func TestStripeAPICallWithEmptySecretKey(t *testing.T) {
	p := &stripeProvider{defaultCurrency: "usd"} // secretKey intentionally empty
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"CreateCharge", func() error {
			_, err := p.CreateCharge(ctx, payments.ChargeParams{Amount: 100})
			return err
		}},
		{"CaptureCharge", func() error {
			_, err := p.CaptureCharge(ctx, "pi_test", 0)
			return err
		}},
		{"RefundCharge", func() error {
			_, err := p.RefundCharge(ctx, payments.RefundParams{ChargeID: "pi_test"})
			return err
		}},
		{"EnsureCustomer", func() error {
			_, err := p.EnsureCustomer(ctx, payments.CustomerParams{Email: "a@b.com"})
			return err
		}},
		{"CreateSubscription", func() error {
			_, err := p.CreateSubscription(ctx, payments.SubscriptionParams{})
			return err
		}},
		{"CancelSubscription", func() error {
			_, err := p.CancelSubscription(ctx, "sub_test", false)
			return err
		}},
		{"UpdateSubscription", func() error {
			_, err := p.UpdateSubscription(ctx, "sub_test", payments.SubscriptionUpdateParams{})
			return err
		}},
		{"CreateCheckoutSession", func() error {
			_, err := p.CreateCheckoutSession(ctx, payments.CheckoutParams{})
			return err
		}},
		{"CreatePortalSession", func() error {
			_, err := p.CreatePortalSession(ctx, "cus_test", "https://example.com")
			return err
		}},
		{"CreateTransfer", func() error {
			_, err := p.CreateTransfer(ctx, payments.TransferParams{})
			return err
		}},
		{"CreatePayout", func() error {
			_, err := p.CreatePayout(ctx, payments.PayoutParams{})
			return err
		}},
		{"ListInvoices", func() error {
			_, err := p.ListInvoices(ctx, payments.InvoiceListParams{})
			return err
		}},
		{"AttachPaymentMethod", func() error {
			_, err := p.AttachPaymentMethod(ctx, "cus_test", "pm_test")
			return err
		}},
		{"ListPaymentMethods", func() error {
			_, err := p.ListPaymentMethods(ctx, "cus_test", "card")
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatalf("%s: expected errStripeKeyMissing, got nil", tc.name)
			}
			if err != errStripeKeyMissing {
				t.Fatalf("%s: expected errStripeKeyMissing, got: %v", tc.name, err)
			}
		})
	}
}
