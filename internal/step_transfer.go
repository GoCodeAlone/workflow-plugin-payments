package internal

import (
	"context"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// transferStep implements step.payment_transfer.
type transferStep struct {
	name       string
	moduleName string
}

func newTransferStep(name string, config map[string]any) (*transferStep, error) {
	return &transferStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *transferStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	amount := resolveInt64("amount", current, config)
	if amount == 0 {
		return &sdk.StepResult{Output: map[string]any{"error": "amount is required"}}, nil
	}
	destination := resolveValue("destination_account_id", current, config)
	if destination == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "destination_account_id is required"}}, nil
	}

	t, err := provider.CreateTransfer(ctx, payments.TransferParams{
		Amount:               amount,
		Currency:             resolveValue("currency", current, config),
		DestinationAccountID: destination,
		Description:          resolveValue("description", current, config),
	})
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"transfer_id": t.ID,
		"status":      t.Status,
	}}, nil
}
