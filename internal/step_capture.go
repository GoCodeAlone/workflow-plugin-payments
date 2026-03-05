package internal

import (
	"context"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// captureStep implements step.payment_capture.
type captureStep struct {
	name       string
	moduleName string
}

func newCaptureStep(name string, config map[string]any) (*captureStep, error) {
	return &captureStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *captureStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	chargeID := resolveValue("charge_id", current, config)
	if chargeID == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "charge_id is required"}}, nil
	}
	amount := resolveInt64("amount", current, config)

	charge, err := provider.CaptureCharge(ctx, chargeID, amount)
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"status": charge.Status,
		"amount": charge.Amount,
	}}, nil
}
