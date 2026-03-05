package internal

import (
	"context"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// chargeStep implements step.payment_charge.
type chargeStep struct {
	name       string
	moduleName string
}

func newChargeStep(name string, config map[string]any) (*chargeStep, error) {
	return &chargeStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *chargeStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	amount := resolveInt64("amount", current, config)
	if amount == 0 {
		return &sdk.StepResult{Output: map[string]any{"error": "amount is required"}}, nil
	}

	params := payments.ChargeParams{
		Amount:        amount,
		Currency:      resolveValue("currency", current, config),
		CustomerID:    resolveValue("customer_id", current, config),
		CaptureMethod: resolveValue("capture_method", current, config),
		Description:   resolveValue("description", current, config),
	}

	charge, err := provider.CreateCharge(ctx, params)
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"charge_id":     charge.ID,
		"client_secret": charge.ClientSecret,
		"status":        charge.Status,
		"amount":        charge.Amount,
	}}, nil
}
