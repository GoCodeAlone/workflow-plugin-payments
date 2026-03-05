package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

func newTestPayPalProvider(t *testing.T, handler http.HandlerFunc) (*paypalProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	p, err := newPayPalProvider(map[string]any{
		"client_id":     "test_client_id",
		"client_secret": "test_secret",
		"_baseURL":      srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p, srv
}

func paypalTokenHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth2/token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "test_access_token",
				"expires_in":   3600,
				"token_type":   "Bearer",
			})
			return
		}
		next(w, r)
	}
}

func TestPayPalProviderConfig_Missing(t *testing.T) {
	_, err := newPayPalProvider(map[string]any{})
	if err == nil {
		t.Error("expected error for missing credentials")
	}
}

func TestPayPalTokenRefresh(t *testing.T) {
	called := 0
	p, _ := newTestPayPalProvider(t, paypalTokenHandler(func(w http.ResponseWriter, r *http.Request) {
		called++
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	token1, err := p.getAccessToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if token1 == "" {
		t.Error("expected token")
	}
	// Second call should use cached token.
	token2, err := p.getAccessToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if token1 != token2 {
		t.Error("expected same cached token")
	}
}

func TestPayPalCreateCharge(t *testing.T) {
	p, _ := newTestPayPalProvider(t, paypalTokenHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/checkout/orders" && r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]any{
				"id":     "ORDER_123",
				"status": "CREATED",
			})
			return
		}
		http.NotFound(w, r)
	}))

	charge, err := p.CreateCharge(context.Background(), chargeParamsAuto())
	if err != nil {
		t.Fatal(err)
	}
	if charge.ID != "ORDER_123" {
		t.Errorf("expected ORDER_123, got %s", charge.ID)
	}
}

func TestPayPalWebhookVerify_Success(t *testing.T) {
	p, _ := newTestPayPalProvider(t, paypalTokenHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/notifications/verify-webhook-signature" {
			json.NewEncoder(w).Encode(map[string]any{
				"verification_status": "SUCCESS",
			})
			return
		}
		http.NotFound(w, r)
	}))

	payload := []byte(`{"id":"WH_1","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{}}`)
	event, err := p.VerifyWebhook(context.Background(), payload, "test_sig")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "PAYMENT.CAPTURE.COMPLETED" {
		t.Errorf("expected PAYMENT.CAPTURE.COMPLETED, got %s", event.Type)
	}
}

func TestPayPalWebhookVerify_Failure(t *testing.T) {
	p, _ := newTestPayPalProvider(t, paypalTokenHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/notifications/verify-webhook-signature" {
			json.NewEncoder(w).Encode(map[string]any{
				"verification_status": "FAILURE",
			})
			return
		}
		http.NotFound(w, r)
	}))

	_, err := p.VerifyWebhook(context.Background(), []byte(`{}`), "bad_sig")
	if err == nil {
		t.Error("expected error for failed verification")
	}
}

func TestPayPalEnsureCustomer_Synthetic(t *testing.T) {
	p, _ := newTestPayPalProvider(t, paypalTokenHandler(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))

	cust, err := p.EnsureCustomer(context.Background(), payments.CustomerParams{
		Email: "paypal@example.com",
		Name:  "PayPal User",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cust.Email != "paypal@example.com" {
		t.Errorf("expected paypal@example.com, got %s", cust.Email)
	}
	if cust.ID == "" {
		t.Error("expected synthetic customer ID")
	}
}

func TestPayPalCalculateFees(t *testing.T) {
	p := &paypalProvider{}
	fees, err := p.CalculateFees(10000, "usd", 0)
	if err != nil {
		t.Fatal(err)
	}
	// 2.99% of 10000 = 299, + 49 = 348
	if fees.ProcessingFee < 348 {
		t.Errorf("expected processing fee >= 348, got %d", fees.ProcessingFee)
	}
}
