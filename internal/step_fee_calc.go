package internal

import (
	"context"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// feeCalcStep implements step.payment_fee_calculate.
type feeCalcStep struct {
	name       string
	moduleName string
}

func newFeeCalcStep(name string, config map[string]any) (*feeCalcStep, error) {
	return &feeCalcStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *feeCalcStep) Execute(_ context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	amount := resolveInt64("amount", current, config)
	if amount == 0 {
		return &sdk.StepResult{Output: map[string]any{"error": "amount is required"}}, nil
	}

	currency := resolveValue("currency", current, config)
	platformFeePercent := resolveFloat64("platform_fee_percent", current, config)

	fees, err := provider.CalculateFees(amount, currency, platformFeePercent)
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"contribution_amount": fees.ContributionAmount,
		"processing_fee":      fees.ProcessingFee,
		"platform_fee":        fees.PlatformFee,
		"total_charge":        fees.TotalCharge,
	}}, nil
}
