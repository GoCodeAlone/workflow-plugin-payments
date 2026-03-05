package internal

import (
	"context"
	"encoding/json"

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

	// Read signature from current, metadata, or config.
	signature := resolveValue("stripe_signature", current, config)
	if signature == "" {
		if v, ok := metadata["stripe_signature"].(string); ok {
			signature = v
		}
	}
	if signature == "" {
		signature = resolveValue("webhook_signature", current, config)
	}

	event, err := provider.VerifyWebhook(ctx, payload, signature)
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	dataJSON, _ := json.Marshal(event.Data)
	return &sdk.StepResult{Output: map[string]any{
		"event_type": event.Type,
		"event_id":   event.ID,
		"data":       string(dataJSON),
	}}, nil
}
