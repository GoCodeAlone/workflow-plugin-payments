package internal

import (
	"context"
	"encoding/json"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// paymentMethodAttachStep implements step.payment_method_attach.
type paymentMethodAttachStep struct {
	name       string
	moduleName string
}

func newPaymentMethodAttachStep(name string, config map[string]any) (*paymentMethodAttachStep, error) {
	return &paymentMethodAttachStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *paymentMethodAttachStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	customerID := resolveValue("customer_id", current, config)
	paymentMethodID := resolveValue("payment_method_id", current, config)
	if customerID == "" || paymentMethodID == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "customer_id and payment_method_id are required"}}, nil
	}

	pm, err := provider.AttachPaymentMethod(ctx, customerID, paymentMethodID)
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"payment_method_id": pm.ID,
		"type":              pm.Type,
	}}, nil
}

// paymentMethodListStep implements step.payment_method_list.
type paymentMethodListStep struct {
	name       string
	moduleName string
}

func newPaymentMethodListStep(name string, config map[string]any) (*paymentMethodListStep, error) {
	return &paymentMethodListStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *paymentMethodListStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	customerID := resolveValue("customer_id", current, config)
	if customerID == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "customer_id is required"}}, nil
	}

	methods, err := provider.ListPaymentMethods(ctx, customerID, resolveValue("type", current, config))
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	data, _ := json.Marshal(methods)
	return &sdk.StepResult{Output: map[string]any{
		"payment_methods": string(data),
		"count":           len(methods),
	}}, nil
}
