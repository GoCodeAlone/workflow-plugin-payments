package internal

import (
	"context"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// payoutStep implements step.payment_payout.
type payoutStep struct {
	name       string
	moduleName string
}

func newPayoutStep(name string, config map[string]any) (*payoutStep, error) {
	return &payoutStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *payoutStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	amount := resolveInt64("amount", current, config)
	if amount == 0 {
		return &sdk.StepResult{Output: map[string]any{"error": "amount is required"}}, nil
	}

	po, err := provider.CreatePayout(ctx, payments.PayoutParams{
		Amount:            amount,
		Currency:          resolveValue("currency", current, config),
		DestinationBankID: resolveValue("destination_bank_id", current, config),
		Description:       resolveValue("description", current, config),
	})
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"payout_id": po.ID,
		"status":    po.Status,
	}}, nil
}
