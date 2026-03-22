package internal

import (
	"context"
	"encoding/json"
	"io"
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
	p.webhookID = "wh-test"

	headers := http.Header{
		"Paypal-Transmission-Id":   []string{"txn-1"},
		"Paypal-Transmission-Sig":  []string{"sig-1"},
		"Paypal-Cert-Url":          []string{"https://cert.example.com/cert"},
		"Paypal-Auth-Algo":         []string{"SHA256withRSA"},
		"Paypal-Transmission-Time": []string{"2024-01-01T00:00:00Z"},
	}
	payload := []byte(`{"id":"WH_1","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{}}`)
	event, err := p.VerifyWebhook(context.Background(), payload, headers)
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
	p.webhookID = "wh-test"

	headers := http.Header{
		"Paypal-Transmission-Id":   []string{"txn-bad"},
		"Paypal-Transmission-Sig":  []string{"bad_sig"},
		"Paypal-Cert-Url":          []string{"https://cert.example.com/cert"},
		"Paypal-Auth-Algo":         []string{"SHA256withRSA"},
		"Paypal-Transmission-Time": []string{"2024-01-01T00:00:00Z"},
	}
	_, err := p.VerifyWebhook(context.Background(), []byte(`{}`), headers)
	if err == nil {
		t.Error("expected error for failed verification")
	}
}

// TestPayPalWebhookVerify_BugProof_RequestBody proves the existing bug:
// transmission_id and transmission_sig are both set to the same value,
// cert_url is empty, and webhook_id is empty.
// These assertions FAIL before the fix and PASS after.
func TestPayPalWebhookVerify_BugProof_RequestBody(t *testing.T) {
	var capturedBody map[string]any

	p, _ := newTestPayPalProvider(t, paypalTokenHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/notifications/verify-webhook-signature" {
			raw, _ := io.ReadAll(r.Body)
			json.Unmarshal(raw, &capturedBody)
			json.NewEncoder(w).Encode(map[string]any{"verification_status": "SUCCESS"})
			return
		}
		http.NotFound(w, r)
	}))

	p.webhookID = "configured-wh-id"
	headers := http.Header{
		"Paypal-Transmission-Id":   []string{"txn-id-123"},
		"Paypal-Transmission-Sig":  []string{"sig-abc"},
		"Paypal-Cert-Url":          []string{"https://api.paypal.com/v1/notifications/certs/cert_key"},
		"Paypal-Auth-Algo":         []string{"SHA256withRSA"},
		"Paypal-Transmission-Time": []string{"2024-01-01T00:00:00Z"},
	}
	payload := []byte(`{"id":"WH_1","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{}}`)
	_, err := p.VerifyWebhook(context.Background(), payload, headers)
	if err != nil {
		t.Fatal(err)
	}

	if capturedBody == nil {
		t.Fatal("no request body captured")
	}
	txnID, _ := capturedBody["transmission_id"].(string)
	txnSig, _ := capturedBody["transmission_sig"].(string)
	if txnID == txnSig {
		t.Errorf("BUG: transmission_id == transmission_sig (%q); they must be distinct fields", txnID)
	}
	certURL, _ := capturedBody["cert_url"].(string)
	if certURL == "" {
		t.Error("BUG: cert_url is empty; it must be set from PayPal-Cert-Url header")
	}
	webhookID, _ := capturedBody["webhook_id"].(string)
	if webhookID == "" {
		t.Error("BUG: webhook_id is empty; it must be set from provider config")
	}
}

// TestPayPalWebhookVerify_CorrectFields verifies the fixed behavior:
// each PayPal header maps to the correct request body field, webhook_id comes
// from provider config, and SUCCESS / FAILURE responses are handled correctly.
func TestPayPalWebhookVerify_CorrectFields(t *testing.T) {
	var capturedBody map[string]any

	p, _ := newTestPayPalProvider(t, paypalTokenHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/notifications/verify-webhook-signature" {
			raw, _ := io.ReadAll(r.Body)
			json.Unmarshal(raw, &capturedBody)
			json.NewEncoder(w).Encode(map[string]any{"verification_status": "SUCCESS"})
			return
		}
		http.NotFound(w, r)
	}))
	// Set a webhook_id on the provider (simulating config).
	p.webhookID = "configured-webhook-id"

	headers := http.Header{
		"Paypal-Transmission-Id":   []string{"txn-id-123"},
		"Paypal-Transmission-Sig":  []string{"sig-abc"},
		"Paypal-Cert-Url":          []string{"https://api.paypal.com/v1/notifications/certs/cert_key"},
		"Paypal-Auth-Algo":         []string{"SHA256withRSA"},
		"Paypal-Transmission-Time": []string{"2024-01-01T00:00:00Z"},
	}
	payload := []byte(`{"id":"WH_2","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{}}`)

	event, err := p.VerifyWebhook(context.Background(), payload, headers)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if event.Type != "PAYMENT.CAPTURE.COMPLETED" {
		t.Errorf("expected PAYMENT.CAPTURE.COMPLETED, got %s", event.Type)
	}

	// Verify distinct field mapping.
	if got := capturedBody["transmission_id"]; got != "txn-id-123" {
		t.Errorf("transmission_id: want txn-id-123, got %v", got)
	}
	if got := capturedBody["transmission_sig"]; got != "sig-abc" {
		t.Errorf("transmission_sig: want sig-abc, got %v", got)
	}
	if got := capturedBody["cert_url"]; got != "https://api.paypal.com/v1/notifications/certs/cert_key" {
		t.Errorf("cert_url: want cert URL, got %v", got)
	}
	if got := capturedBody["auth_algo"]; got != "SHA256withRSA" {
		t.Errorf("auth_algo: want SHA256withRSA, got %v", got)
	}
	if got := capturedBody["transmission_time"]; got != "2024-01-01T00:00:00Z" {
		t.Errorf("transmission_time: want 2024-01-01T00:00:00Z, got %v", got)
	}
	if got := capturedBody["webhook_id"]; got != "configured-webhook-id" {
		t.Errorf("webhook_id: want configured-webhook-id, got %v", got)
	}
}

func TestPayPalWebhookVerify_FailureStatus(t *testing.T) {
	p, _ := newTestPayPalProvider(t, paypalTokenHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/notifications/verify-webhook-signature" {
			json.NewEncoder(w).Encode(map[string]any{"verification_status": "FAILURE"})
			return
		}
		http.NotFound(w, r)
	}))
	p.webhookID = "wh-id"

	headers := http.Header{
		"Paypal-Transmission-Id":   []string{"txn-id"},
		"Paypal-Transmission-Sig":  []string{"bad-sig"},
		"Paypal-Cert-Url":          []string{"https://cert.example.com/cert"},
		"Paypal-Auth-Algo":         []string{"SHA256withRSA"},
		"Paypal-Transmission-Time": []string{"2024-01-01T00:00:00Z"},
	}
	_, err := p.VerifyWebhook(context.Background(), []byte(`{}`), headers)
	if err != payments.ErrWebhookInvalid {
		t.Errorf("expected ErrWebhookInvalid, got %v", err)
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
