package internal

import (
	"context"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// checkoutStep implements step.payment_checkout_create.
type checkoutStep struct {
	name       string
	moduleName string
}

func newCheckoutStep(name string, config map[string]any) (*checkoutStep, error) {
	return &checkoutStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *checkoutStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	sess, err := provider.CreateCheckoutSession(ctx, payments.CheckoutParams{
		CustomerID: resolveValue("customer_id", current, config),
		PriceID:    resolveValue("price_id", current, config),
		SuccessURL: resolveValue("success_url", current, config),
		CancelURL:  resolveValue("cancel_url", current, config),
		Mode:       resolveValue("mode", current, config),
	})
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"url":        sess.URL,
		"session_id": sess.ID,
	}}, nil
}
