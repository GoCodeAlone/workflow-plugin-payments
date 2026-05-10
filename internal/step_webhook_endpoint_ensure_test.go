package internal

import (
	"context"
	"testing"
)

func TestWebhookEndpointEnsureStep_HappyPath(t *testing.T) {
	mock := newMockProvider()
	RegisterProvider("payments", mock)
	t.Cleanup(func() { UnregisterProvider("payments") })

	step, err := newWebhookEndpointEnsureStep("ensure_endpoint", map[string]any{})
	if err != nil {
		t.Fatalf("newWebhookEndpointEnsureStep: %v", err)
	}
	res, err := step.Execute(context.Background(), nil, nil, map[string]any{
		"url":    "https://example.com/api/v1/webhooks/stripe/issuing",
		"events": []any{"a", "b"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.StopPipeline {
		t.Errorf("expected StopPipeline=false on happy path")
	}
	if res.Output["created"] != true {
		t.Errorf("expected created=true, got %v", res.Output["created"])
	}
	if got, _ := res.Output["signing_secret"].(string); got == "" {
		t.Errorf("expected signing_secret populated, got empty")
	}
	if _, ok := res.Output["endpoint_id"].(string); !ok {
		t.Errorf("expected endpoint_id string, got %T", res.Output["endpoint_id"])
	}
}

func TestWebhookEndpointEnsureStep_OutputScalarOnly(t *testing.T) {
	// Verifies the structpb-canonical scalar boundary (V7): only
	// string|bool fields appear in Output, no nested maps/slices.
	mock := newMockProvider()
	RegisterProvider("payments", mock)
	t.Cleanup(func() { UnregisterProvider("payments") })

	step, _ := newWebhookEndpointEnsureStep("ensure", map[string]any{})
	res, _ := step.Execute(context.Background(), nil, nil, map[string]any{
		"url":    "https://example.com/api/v1/webhooks/stripe/issuing",
		"events": []any{"a"},
	}, nil, nil)
	for k, v := range res.Output {
		switch v.(type) {
		case string, bool:
			// ok
		default:
			t.Errorf("V7: Output[%q] must be scalar string|bool on gRPC boundary, got %T", k, v)
		}
	}
}

func TestWebhookEndpointEnsureStep_RejectsMissingURL(t *testing.T) {
	mock := newMockProvider()
	RegisterProvider("payments", mock)
	t.Cleanup(func() { UnregisterProvider("payments") })

	step, _ := newWebhookEndpointEnsureStep("ensure", map[string]any{})
	res, err := step.Execute(context.Background(), nil, nil, map[string]any{
		"events": []any{"a"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("Execute should route via output not error, got %v", err)
	}
	if !res.StopPipeline {
		t.Errorf("expected StopPipeline=true on validation error")
	}
	if _, ok := res.Output["error"]; !ok {
		t.Errorf("expected error key in Output")
	}
}

func TestWebhookEndpointEnsureStep_RejectsMissingEvents(t *testing.T) {
	mock := newMockProvider()
	RegisterProvider("payments", mock)
	t.Cleanup(func() { UnregisterProvider("payments") })

	step, _ := newWebhookEndpointEnsureStep("ensure", map[string]any{})
	res, _ := step.Execute(context.Background(), nil, nil, map[string]any{
		"url": "https://example.com/api/v1/webhooks/stripe/issuing",
	}, nil, nil)
	if !res.StopPipeline {
		t.Errorf("expected StopPipeline=true on missing events")
	}
	if _, ok := res.Output["error"]; !ok {
		t.Errorf("expected error key in Output")
	}
}

func TestWebhookEndpointEnsureStep_ProviderNotFound(t *testing.T) {
	step, _ := newWebhookEndpointEnsureStep("ensure", map[string]any{"module": "missing-module"})
	res, _ := step.Execute(context.Background(), nil, nil, map[string]any{
		"url":    "https://example.com/api/v1/webhooks/stripe/issuing",
		"events": []any{"a"},
	}, nil, nil)
	if res.Output["error"] == nil {
		t.Errorf("expected error key when provider missing")
	}
}

func TestExtractStringSlice(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{"strings", []string{"a", "b"}, []string{"a", "b"}},
		{"any-of-strings", []any{"a", "b"}, []string{"a", "b"}},
		{"nil", nil, nil},
		{"wrong-type", "not a slice", nil},
		{"empty-any-slice", []any{}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractStringSlice(tc.in)
			if len(got) != len(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got[%d]=%q, want[%d]=%q", i, got[i], i, tc.want[i])
				}
			}
		})
	}
}
