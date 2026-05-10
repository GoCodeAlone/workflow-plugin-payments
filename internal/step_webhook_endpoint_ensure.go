package internal

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// webhookEndpointEnsureStep implements step.payment_webhook_endpoint_ensure.
// It dispatches to the configured payments provider's WebhookEndpointEnsure
// method, surfacing the typed result back through the structpb-canonical
// scalar Output map (no nested types — strict-contracts boundary).
type webhookEndpointEnsureStep struct {
	name       string
	moduleName string
}

func newWebhookEndpointEnsureStep(name string, config map[string]any) (*webhookEndpointEnsureStep, error) {
	return &webhookEndpointEnsureStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *webhookEndpointEnsureStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	url := resolveValue("url", current, config)
	events := resolveStringSlice("events", current, config)
	description := resolveValue("description", current, config)
	mode := resolveValue("mode", current, config)

	if url == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "url is required"}, StopPipeline: true}, nil
	}
	if len(events) == 0 {
		return &sdk.StepResult{Output: map[string]any{"error": "events list is required"}, StopPipeline: true}, nil
	}

	out, err := provider.WebhookEndpointEnsure(ctx, payments.WebhookEndpointEnsureParams{
		URL:         url,
		Events:      events,
		Description: description,
		Mode:        mode,
	})
	if err != nil {
		return &sdk.StepResult{
			Output:       map[string]any{"error": fmt.Sprintf("webhook ensure: %v", err)},
			StopPipeline: true,
		}, nil
	}

	// Output is scalar-only (string|bool) so it round-trips cleanly across
	// the gRPC structpb boundary. signing_secret is empty on every non-create
	// branch (V3) — downstream secret_set chains short-circuit via
	// `if: created` to avoid overwriting an existing real secret with empty.
	return &sdk.StepResult{Output: map[string]any{
		"endpoint_id":    out.EndpointID,
		"created":        out.Created,
		"events_drift":   out.EventsDrift,
		"signing_secret": out.SigningSecret,
	}}, nil
}

// resolveStringSlice extracts a []string from current first, then config.
// Accepts either []string or []any (the gRPC structpb path lands as []any).
func resolveStringSlice(key string, current, config map[string]any) []string {
	if v := extractStringSlice(current[key]); v != nil {
		return v
	}
	return extractStringSlice(config[key])
}

func extractStringSlice(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			if str, ok := e.(string); ok {
				out = append(out, str)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}
