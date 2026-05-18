package internal

import (
	"context"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

type stablecoinDepositIntentStep struct {
	name       string
	moduleName string
}

func newStablecoinDepositIntentStep(name string, config map[string]any) (*stablecoinDepositIntentStep, error) {
	return &stablecoinDepositIntentStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *stablecoinDepositIntentStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}
	amount := resolveInt64("amount", current, config)
	if amount == 0 {
		return &sdk.StepResult{Output: map[string]any{"error": "amount is required"}}, nil
	}
	intent, err := provider.CreateStablecoinDepositIntent(ctx, payments.StablecoinDepositIntentParams{
		Amount:         amount,
		Currency:       resolveValue("currency", current, config),
		Networks:       resolveStringSlice("networks", current, config),
		Stablecoin:     resolveValue("stablecoin", current, config),
		IdempotencyKey: resolveValue("idempotency_key", current, config),
		Description:    resolveValue("description", current, config),
	})
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}
	addresses := make([]map[string]any, 0, len(intent.DepositAddresses))
	for _, address := range intent.DepositAddresses {
		addresses = append(addresses, map[string]any{
			"network":                address.Network,
			"address":                address.Address,
			"stablecoin":             address.Stablecoin,
			"token_contract_address": address.TokenContractAddress,
		})
	}
	return &sdk.StepResult{Output: map[string]any{
		"payment_intent_id": intent.ID,
		"client_secret":     intent.ClientSecret,
		"status":            intent.Status,
		"amount":            intent.Amount,
		"currency":          intent.Currency,
		"stablecoin":        intent.Stablecoin,
		"deposit_addresses": addresses,
	}}, nil
}
