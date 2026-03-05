package internal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

// paypalProvider implements payments.PaymentProvider via PayPal REST API v2.
type paypalProvider struct {
	clientID     string
	clientSecret string
	baseURL      string
	httpClient   *http.Client

	tokenMu      sync.Mutex
	accessToken  string
	tokenExpiry  time.Time
}

func newPayPalProvider(config map[string]any) (*paypalProvider, error) {
	clientID, _ := config["clientId"].(string)
	if clientID == "" {
		clientID, _ = config["client_id"].(string)
	}
	clientSecret, _ := config["clientSecret"].(string)
	if clientSecret == "" {
		clientSecret, _ = config["client_secret"].(string)
	}
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("paypal provider: clientId and clientSecret are required")
	}

	env, _ := config["environment"].(string)
	baseURL := "https://api-m.sandbox.paypal.com"
	if env == "production" || env == "live" {
		baseURL = "https://api-m.paypal.com"
	}

	// Allow override for tests.
	if testURL, _ := config["_baseURL"].(string); testURL != "" {
		baseURL = testURL
	}

	return &paypalProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      baseURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// getAccessToken returns a cached or freshly-fetched OAuth2 token.
func (p *paypalProvider) getAccessToken(ctx context.Context) (string, error) {
	p.tokenMu.Lock()
	defer p.tokenMu.Unlock()

	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	creds := base64.StdEncoding.EncodeToString([]byte(p.clientID + ":" + p.clientSecret))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("paypal token request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("paypal token decode: %w", err)
	}
	if result.Error != "" || result.AccessToken == "" {
		return "", fmt.Errorf("paypal token error: %s", result.Error)
	}

	p.accessToken = result.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return p.accessToken, nil
}

// doJSON performs an authenticated JSON request.
func (p *paypalProvider) doJSON(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, 0, err
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, err
}

func (p *paypalProvider) CreateCharge(ctx context.Context, cp payments.ChargeParams) (*payments.Charge, error) {
	intent := "CAPTURE"
	if cp.CaptureMethod == "manual" {
		intent = "AUTHORIZE"
	}
	currency := cp.Currency
	if currency == "" {
		currency = "USD"
	}
	body := map[string]any{
		"intent": intent,
		"purchase_units": []map[string]any{
			{
				"amount": map[string]any{
					"currency_code": strings.ToUpper(currency),
					"value":         fmt.Sprintf("%.2f", float64(cp.Amount)/100.0),
				},
				"description": cp.Description,
			},
		},
	}
	respBody, statusCode, err := p.doJSON(ctx, "POST", "/v2/checkout/orders", body)
	if err != nil {
		return nil, fmt.Errorf("paypal CreateCharge: %w", err)
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("paypal CreateCharge: HTTP %d: %s", statusCode, string(respBody))
	}
	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paypal CreateCharge decode: %w", err)
	}
	return &payments.Charge{
		ID:       result.ID,
		Status:   result.Status,
		Amount:   cp.Amount,
		Currency: currency,
	}, nil
}

func (p *paypalProvider) CaptureCharge(ctx context.Context, chargeID string, _ int64) (*payments.Charge, error) {
	respBody, statusCode, err := p.doJSON(ctx, "POST", "/v2/checkout/orders/"+chargeID+"/capture", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("paypal CaptureCharge: %w", err)
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("paypal CaptureCharge: HTTP %d: %s", statusCode, string(respBody))
	}
	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paypal CaptureCharge decode: %w", err)
	}
	return &payments.Charge{
		ID:     result.ID,
		Status: result.Status,
	}, nil
}

func (p *paypalProvider) RefundCharge(ctx context.Context, rp payments.RefundParams) (*payments.Refund, error) {
	body := map[string]any{}
	if rp.Amount > 0 {
		body["amount"] = map[string]any{
			"value":         fmt.Sprintf("%.2f", float64(rp.Amount)/100.0),
			"currency_code": "USD",
		}
	}
	if rp.Reason != "" {
		body["note_to_payer"] = rp.Reason
	}
	respBody, statusCode, err := p.doJSON(ctx, "POST", "/v2/payments/captures/"+rp.ChargeID+"/refund", body)
	if err != nil {
		return nil, fmt.Errorf("paypal RefundCharge: %w", err)
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("paypal RefundCharge: HTTP %d: %s", statusCode, string(respBody))
	}
	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paypal RefundCharge decode: %w", err)
	}
	return &payments.Refund{
		ID:     result.ID,
		Status: result.Status,
		Amount: rp.Amount,
	}, nil
}

// EnsureCustomer returns a synthetic customer using email as ID (PayPal has no customer concept).
func (p *paypalProvider) EnsureCustomer(_ context.Context, cp payments.CustomerParams) (*payments.Customer, error) {
	return &payments.Customer{
		ID:    "paypal-customer:" + cp.Email,
		Email: cp.Email,
		Name:  cp.Name,
	}, nil
}

func (p *paypalProvider) CreateSubscription(ctx context.Context, sp payments.SubscriptionParams) (*payments.Subscription, error) {
	body := map[string]any{
		"plan_id": sp.PriceID,
		"subscriber": map[string]any{
			"email_address": sp.CustomerID,
		},
	}
	respBody, statusCode, err := p.doJSON(ctx, "POST", "/v1/billing/subscriptions", body)
	if err != nil {
		return nil, fmt.Errorf("paypal CreateSubscription: %w", err)
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("paypal CreateSubscription: HTTP %d: %s", statusCode, string(respBody))
	}
	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paypal CreateSubscription decode: %w", err)
	}
	return &payments.Subscription{
		ID:     result.ID,
		Status: result.Status,
	}, nil
}

func (p *paypalProvider) CancelSubscription(ctx context.Context, subscriptionID string, _ bool) (*payments.Subscription, error) {
	body := map[string]any{"reason": "User requested cancellation"}
	respBody, statusCode, err := p.doJSON(ctx, "POST", "/v1/billing/subscriptions/"+subscriptionID+"/cancel", body)
	if err != nil {
		return nil, fmt.Errorf("paypal CancelSubscription: %w", err)
	}
	if statusCode >= 400 && statusCode != 204 {
		return nil, fmt.Errorf("paypal CancelSubscription: HTTP %d: %s", statusCode, string(respBody))
	}
	return &payments.Subscription{
		ID:     subscriptionID,
		Status: "CANCELLED",
	}, nil
}

func (p *paypalProvider) UpdateSubscription(ctx context.Context, subscriptionID string, up payments.SubscriptionUpdateParams) (*payments.Subscription, error) {
	body := map[string]any{
		"plan_id": up.PriceID,
	}
	respBody, statusCode, err := p.doJSON(ctx, "POST", "/v1/billing/subscriptions/"+subscriptionID+"/revise", body)
	if err != nil {
		return nil, fmt.Errorf("paypal UpdateSubscription: %w", err)
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("paypal UpdateSubscription: HTTP %d: %s", statusCode, string(respBody))
	}
	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paypal UpdateSubscription decode: %w", err)
	}
	if result.ID == "" {
		result.ID = subscriptionID
	}
	return &payments.Subscription{
		ID:     result.ID,
		Status: result.Status,
	}, nil
}

func (p *paypalProvider) CreateCheckoutSession(ctx context.Context, cp payments.CheckoutParams) (*payments.CheckoutSession, error) {
	intent := "CAPTURE"
	body := map[string]any{
		"intent": intent,
		"purchase_units": []map[string]any{
			{
				"amount": map[string]any{
					"currency_code": "USD",
					"value":         "0.00",
				},
			},
		},
		"application_context": map[string]any{
			"return_url": cp.SuccessURL,
			"cancel_url": cp.CancelURL,
		},
	}
	respBody, statusCode, err := p.doJSON(ctx, "POST", "/v2/checkout/orders", body)
	if err != nil {
		return nil, fmt.Errorf("paypal CreateCheckoutSession: %w", err)
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("paypal CreateCheckoutSession: HTTP %d: %s", statusCode, string(respBody))
	}
	var result struct {
		ID    string `json:"id"`
		Links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paypal CreateCheckoutSession decode: %w", err)
	}
	approvalURL := ""
	for _, link := range result.Links {
		if link.Rel == "approve" {
			approvalURL = link.Href
			break
		}
	}
	return &payments.CheckoutSession{
		ID:  result.ID,
		URL: approvalURL,
	}, nil
}

func (p *paypalProvider) CreatePortalSession(_ context.Context, _, _ string) (*payments.PortalSession, error) {
	return nil, payments.ErrUnsupported
}

func (p *paypalProvider) VerifyWebhook(ctx context.Context, payload []byte, signature string) (*payments.WebhookEvent, error) {
	body := map[string]any{
		"auth_algo":         "SHA256withRSA",
		"cert_url":          "",
		"transmission_id":   signature,
		"transmission_sig":  signature,
		"transmission_time": time.Now().UTC().Format(time.RFC3339),
		"webhook_id":        "",
		"webhook_event":     json.RawMessage(payload),
	}
	respBody, statusCode, err := p.doJSON(ctx, "POST", "/v1/notifications/verify-webhook-signature", body)
	if err != nil {
		return nil, fmt.Errorf("paypal VerifyWebhook: %w", err)
	}
	if statusCode >= 400 {
		return nil, payments.ErrWebhookInvalid
	}
	var verifyResult struct {
		VerificationStatus string `json:"verification_status"`
	}
	if err := json.Unmarshal(respBody, &verifyResult); err != nil {
		return nil, payments.ErrWebhookInvalid
	}
	if verifyResult.VerificationStatus != "SUCCESS" {
		return nil, payments.ErrWebhookInvalid
	}

	var event struct {
		ID        string         `json:"id"`
		EventType string         `json:"event_type"`
		Resource  map[string]any `json:"resource"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("paypal VerifyWebhook parse: %w", err)
	}
	return &payments.WebhookEvent{
		ID:   event.ID,
		Type: event.EventType,
		Data: event.Resource,
	}, nil
}

func (p *paypalProvider) CreateTransfer(ctx context.Context, tp payments.TransferParams) (*payments.Transfer, error) {
	currency := strings.ToUpper(tp.Currency)
	if currency == "" {
		currency = "USD"
	}
	body := map[string]any{
		"sender_batch_header": map[string]any{
			"email_subject": tp.Description,
		},
		"items": []map[string]any{
			{
				"recipient_type": "PAYPAL_ID",
				"amount": map[string]any{
					"value":    fmt.Sprintf("%.2f", float64(tp.Amount)/100.0),
					"currency": currency,
				},
				"receiver": tp.DestinationAccountID,
			},
		},
	}
	respBody, statusCode, err := p.doJSON(ctx, "POST", "/v1/payments/payouts", body)
	if err != nil {
		return nil, fmt.Errorf("paypal CreateTransfer: %w", err)
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("paypal CreateTransfer: HTTP %d: %s", statusCode, string(respBody))
	}
	var result struct {
		BatchHeader struct {
			PayoutBatchID string `json:"payout_batch_id"`
			BatchStatus   string `json:"batch_status"`
		} `json:"batch_header"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paypal CreateTransfer decode: %w", err)
	}
	return &payments.Transfer{
		ID:     result.BatchHeader.PayoutBatchID,
		Status: result.BatchHeader.BatchStatus,
	}, nil
}

// CreatePayout is the same as CreateTransfer for PayPal.
func (p *paypalProvider) CreatePayout(ctx context.Context, pp payments.PayoutParams) (*payments.Payout, error) {
	t, err := p.CreateTransfer(ctx, payments.TransferParams{
		Amount:               pp.Amount,
		Currency:             pp.Currency,
		DestinationAccountID: pp.DestinationBankID,
		Description:          pp.Description,
	})
	if err != nil {
		return nil, err
	}
	return &payments.Payout{
		ID:     t.ID,
		Status: t.Status,
	}, nil
}

func (p *paypalProvider) ListInvoices(ctx context.Context, lp payments.InvoiceListParams) ([]*payments.Invoice, error) {
	path := "/v2/invoicing/invoices"
	if lp.Limit > 0 {
		path += fmt.Sprintf("?page_size=%d", lp.Limit)
	}
	respBody, statusCode, err := p.doJSON(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("paypal ListInvoices: %w", err)
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("paypal ListInvoices: HTTP %d: %s", statusCode, string(respBody))
	}
	var result struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Detail struct {
				InvoiceDate string `json:"invoice_date"`
			} `json:"detail"`
			Amount struct {
				Value    string `json:"value"`
				Currency string `json:"currency_code"`
			} `json:"amount"`
		} `json:"items"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("paypal ListInvoices decode: %w", err)
	}
	var invoices []*payments.Invoice
	for _, item := range result.Items {
		invoices = append(invoices, &payments.Invoice{
			ID:         item.ID,
			CustomerID: lp.CustomerID,
			Status:     item.Status,
			Currency:   item.Amount.Currency,
		})
	}
	return invoices, nil
}

func (p *paypalProvider) AttachPaymentMethod(_ context.Context, _, _ string) (*payments.PaymentMethod, error) {
	return nil, payments.ErrUnsupported
}

func (p *paypalProvider) ListPaymentMethods(_ context.Context, _, _ string) ([]*payments.PaymentMethod, error) {
	return nil, payments.ErrUnsupported
}

func (p *paypalProvider) CalculateFees(amount int64, _ string, platformFeePercent float64) (*payments.FeeBreakdown, error) {
	// PayPal: 2.99% + $0.49 (49 cents)
	const processingFeeRate = 0.0299
	const processingFeeFixed = int64(49)

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
