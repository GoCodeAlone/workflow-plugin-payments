package internal

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

func TestCheckoutStep(t *testing.T) {
	setupMockModule(t, "test-checkout")

	step, _ := newCheckoutStep("checkout", map[string]any{"module": "test-checkout"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"customer_id": "cus_123",
			"price_id":    "price_123",
			"success_url": "https://example.com/success",
			"cancel_url":  "https://example.com/cancel",
		},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["url"] == "" {
		t.Error("expected url in result")
	}
}

func TestCheckoutStep_SubscriptionInlinePriceAndMetadata(t *testing.T) {
	mock := setupMockModule(t, "test-checkout-inline")

	step, _ := newCheckoutStep("checkout", map[string]any{"module": "test-checkout-inline"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"customer_id":           "cus_123",
			"amount":                int64(2500),
			"currency":              "usd",
			"interval":              "month",
			"product_name":          "Wishlist contribution",
			"success_url":           "https://example.com/success",
			"cancel_url":            "https://example.com/cancel",
			"mode":                  "subscription",
			"allow_promotion_codes": true,
			"metadata": map[string]any{
				"wishlist_id": "wish_123",
				"target_type": "wishlist",
			},
			"subscription_metadata": map[string]string{
				"recurring_contribution_id": "rc_123",
			},
		},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["url"] == "" {
		t.Error("expected url in result")
	}
	if mock.lastCheckout.CustomerID != "cus_123" {
		t.Fatalf("expected customer_id passthrough, got %q", mock.lastCheckout.CustomerID)
	}
	if mock.lastCheckout.Amount != 2500 {
		t.Fatalf("expected amount=2500, got %d", mock.lastCheckout.Amount)
	}
	if mock.lastCheckout.Currency != "usd" {
		t.Fatalf("expected currency=usd, got %q", mock.lastCheckout.Currency)
	}
	if mock.lastCheckout.Interval != "month" {
		t.Fatalf("expected interval=month, got %q", mock.lastCheckout.Interval)
	}
	if mock.lastCheckout.ProductName != "Wishlist contribution" {
		t.Fatalf("expected product_name passthrough, got %q", mock.lastCheckout.ProductName)
	}
	if !mock.lastCheckout.AllowPromotionCodes {
		t.Fatal("expected allow_promotion_codes passthrough")
	}
	if mock.lastCheckout.Metadata["wishlist_id"] != "wish_123" || mock.lastCheckout.Metadata["target_type"] != "wishlist" {
		t.Fatalf("expected checkout metadata passthrough, got %#v", mock.lastCheckout.Metadata)
	}
	if mock.lastCheckout.SubscriptionMetadata["recurring_contribution_id"] != "rc_123" {
		t.Fatalf("expected subscription metadata passthrough, got %#v", mock.lastCheckout.SubscriptionMetadata)
	}
}

func TestCheckoutStep_CurrentFalseOverridesConfigBool(t *testing.T) {
	mock := setupMockModule(t, "test-checkout-bool")

	step, _ := newCheckoutStep("checkout", map[string]any{
		"module":                "test-checkout-bool",
		"allow_promotion_codes": true,
	})
	_, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"customer_id":           "cus_123",
			"price_id":              "price_123",
			"success_url":           "https://example.com/success",
			"cancel_url":            "https://example.com/cancel",
			"allow_promotion_codes": false,
		},
		nil, map[string]any{"allow_promotion_codes": true})
	if err != nil {
		t.Fatal(err)
	}
	if mock.lastCheckout.AllowPromotionCodes {
		t.Fatal("expected current allow_promotion_codes=false to override config=true")
	}
}

func TestPortalStep(t *testing.T) {
	setupMockModule(t, "test-portal")

	step, _ := newPortalStep("portal", map[string]any{"module": "test-portal"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"customer_id": "cus_123",
			"return_url":  "https://example.com/return",
		},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["url"] == "" {
		t.Error("expected url in result")
	}
}

func TestPortalStep_MissingCustomerID(t *testing.T) {
	setupMockModule(t, "test-portal-nocust")
	step, _ := newPortalStep("portal", map[string]any{"module": "test-portal-nocust"})
	result, _ := step.Execute(context.Background(), nil, nil, map[string]any{}, nil, map[string]any{})
	if result.Output["error"] == nil {
		t.Error("expected error for missing customer_id")
	}
}

func TestWebhookStep_Valid(t *testing.T) {
	setupMockModule(t, "test-webhook")

	step, _ := newWebhookStep("webhook", map[string]any{"module": "test-webhook"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"request_body":     `{"id":"evt_1","type":"payment_intent.succeeded"}`,
			"stripe_signature": "t=1234,v1=abc",
		},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["event_type"] == "" {
		t.Error("expected event_type")
	}
	data, ok := result.Output["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured webhook data map, got %T", result.Output["data"])
	}
	if data["id"] != "pi_mock" {
		t.Fatalf("expected verified payment object id pi_mock, got %v", data["id"])
	}
	metadata, ok := result.Output["metadata"].(map[string]string)
	if !ok {
		t.Fatalf("expected webhook metadata map, got %T", result.Output["metadata"])
	}
	if metadata["wishlist_id"] != "wishlist_mock" {
		t.Fatalf("expected wishlist metadata, got %v", metadata["wishlist_id"])
	}
}

func TestWebhookStep_InvalidSignature(t *testing.T) {
	mock := setupMockModule(t, "test-webhook-bad")
	mock.webhookErr = payments.ErrWebhookInvalid

	step, _ := newWebhookStep("webhook", map[string]any{"module": "test-webhook-bad"})
	result, _ := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"request_body":     `{}`,
			"stripe_signature": "bad_sig",
		},
		nil, map[string]any{})
	if result.Output["error"] == nil {
		t.Error("expected error for invalid webhook")
	}
}

func TestWebhookStep_MissingPayload(t *testing.T) {
	setupMockModule(t, "test-webhook-nopayload")
	step, _ := newWebhookStep("webhook", map[string]any{"module": "test-webhook-nopayload"})
	result, _ := step.Execute(context.Background(), nil, nil, map[string]any{}, nil, map[string]any{})
	if result.Output["error"] == nil {
		t.Error("expected error for missing payload")
	}
}
