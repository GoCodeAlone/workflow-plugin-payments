package internal

import (
	"context"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// refundStep implements step.payment_refund.
type refundStep struct {
	name       string
	moduleName string
}

func newRefundStep(name string, config map[string]any) (*refundStep, error) {
	return &refundStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *refundStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	chargeID := resolveValue("charge_id", current, config)
	if chargeID == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "charge_id is required"}}, nil
	}

	re, err := provider.RefundCharge(ctx, payments.RefundParams{
		ChargeID: chargeID,
		Amount:   resolveInt64("amount", current, config),
		Reason:   resolveValue("reason", current, config),
	})
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"refund_id": re.ID,
		"status":    re.Status,
	}}, nil
}
