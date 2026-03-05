package internal

import (
	"context"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// portalStep implements step.payment_portal_create.
type portalStep struct {
	name       string
	moduleName string
}

func newPortalStep(name string, config map[string]any) (*portalStep, error) {
	return &portalStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *portalStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	customerID := resolveValue("customer_id", current, config)
	if customerID == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "customer_id is required"}}, nil
	}
	returnURL := resolveValue("return_url", current, config)

	sess, err := provider.CreatePortalSession(ctx, customerID, returnURL)
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"url":        sess.URL,
		"session_id": sess.ID,
	}}, nil
}
