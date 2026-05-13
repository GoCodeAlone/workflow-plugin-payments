package internal

import (
	"context"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// subscriptionCreateStep implements step.payment_subscription_create.
type subscriptionCreateStep struct {
	name       string
	moduleName string
}

func newSubscriptionCreateStep(name string, config map[string]any) (*subscriptionCreateStep, error) {
	return &subscriptionCreateStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *subscriptionCreateStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	customerID := resolveValue("customer_id", current, config)
	priceID := resolveValue("price_id", current, config)
	amount := resolveInt64("amount", current, config)
	currency := resolveValue("currency", current, config)
	interval := resolveValue("interval", current, config)
	if customerID == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "customer_id is required"}}, nil
	}
	if priceID == "" && (amount == 0 || currency == "" || interval == "") {
		return &sdk.StepResult{Output: map[string]any{"error": "price_id is required, or supply amount + currency + interval for inline pricing"}}, nil
	}

	sub, err := provider.CreateSubscription(ctx, payments.SubscriptionParams{
		CustomerID: customerID,
		PriceID:    priceID,
		Amount:     amount,
		Currency:   currency,
		Interval:   interval,
	})
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"subscription_id": sub.ID,
		"status":          sub.Status,
	}}, nil
}

// subscriptionUpdateStep implements step.payment_subscription_update.
type subscriptionUpdateStep struct {
	name       string
	moduleName string
}

func newSubscriptionUpdateStep(name string, config map[string]any) (*subscriptionUpdateStep, error) {
	return &subscriptionUpdateStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *subscriptionUpdateStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	subscriptionID := resolveValue("subscription_id", current, config)
	if subscriptionID == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "subscription_id is required"}}, nil
	}

	sub, err := provider.UpdateSubscription(ctx, subscriptionID, payments.SubscriptionUpdateParams{
		PriceID: resolveValue("price_id", current, config),
	})
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"subscription_id": sub.ID,
		"status":          sub.Status,
	}}, nil
}

// subscriptionCancelStep implements step.payment_subscription_cancel.
type subscriptionCancelStep struct {
	name       string
	moduleName string
}

func newSubscriptionCancelStep(name string, config map[string]any) (*subscriptionCancelStep, error) {
	return &subscriptionCancelStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *subscriptionCancelStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	subscriptionID := resolveValue("subscription_id", current, config)
	if subscriptionID == "" {
		return &sdk.StepResult{Output: map[string]any{"error": "subscription_id is required"}}, nil
	}

	cancelAtPeriodEnd := false
	if v, ok := current["cancel_at_period_end"].(bool); ok {
		cancelAtPeriodEnd = v
	} else if v, ok := config["cancel_at_period_end"].(bool); ok {
		cancelAtPeriodEnd = v
	}

	sub, err := provider.CancelSubscription(ctx, subscriptionID, cancelAtPeriodEnd)
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	return &sdk.StepResult{Output: map[string]any{
		"subscription_id": sub.ID,
		"status":          sub.Status,
	}}, nil
}
