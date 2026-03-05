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
