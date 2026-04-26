package internal

import (
	"context"
	"net/http"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// webhookStep implements step.payment_webhook_verify.
type webhookStep struct {
	name       string
	moduleName string
}

func newWebhookStep(name string, config map[string]any) (*webhookStep, error) {
	return &webhookStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *webhookStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, metadata map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	// Read payload from current["request_body"] or config.
	payload := []byte(resolveValue("request_body", current, config))
	if len(payload) == 0 {
		if v, ok := current["payload"].(string); ok {
			payload = []byte(v)
		}
	}
	if len(payload) == 0 {
		return &sdk.StepResult{Output: map[string]any{"error": "missing webhook payload (request_body)"}}, nil
	}

	// Build headers for webhook verification.
	// Supports both Stripe (Stripe-Signature) and PayPal header conventions.
	headers := http.Header{}
	headerKeys := []string{
		"Stripe-Signature",
		"Paypal-Transmission-Id",
		"Paypal-Transmission-Sig",
		"Paypal-Cert-Url",
		"Paypal-Auth-Algo",
		"Paypal-Transmission-Time",
	}
	for _, key := range headerKeys {
		if v := resolveValue(key, current, config); v != "" {
			headers.Set(key, v)
		} else if v, ok := metadata[key].(string); ok && v != "" {
			headers.Set(key, v)
		}
	}
	// Legacy fallback: stripe_signature / webhook_signature keys.
	if headers.Get("Stripe-Signature") == "" {
		if sig := resolveValue("stripe_signature", current, config); sig != "" {
			headers.Set("Stripe-Signature", sig)
		} else if v, ok := metadata["stripe_signature"].(string); ok && v != "" {
			headers.Set("Stripe-Signature", v)
		} else if sig := resolveValue("webhook_signature", current, config); sig != "" {
			headers.Set("Stripe-Signature", sig)
		}
	}

	event, err := provider.VerifyWebhook(ctx, payload, headers)
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"event_type": event.Type,
		"event_id":   event.ID,
		"data":       event.Data,
		"metadata":   event.Metadata,
	}}, nil
}
