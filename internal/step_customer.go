package internal

import (
	"context"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// customerStep implements step.payment_customer_ensure.
type customerStep struct {
	name       string
	moduleName string
}

func newCustomerStep(name string, config map[string]any) (*customerStep, error) {
	return &customerStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *customerStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	email := resolveValue("email", current, config)
	if email == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "email is required"}}, nil
	}

	cust, err := provider.EnsureCustomer(ctx, payments.CustomerParams{
		Email: email,
		Name:  resolveValue("name", current, config),
	})
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"customer_id": cust.ID,
		"email":       cust.Email,
		"name":        cust.Name,
	}}, nil
}
