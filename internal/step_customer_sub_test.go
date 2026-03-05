package internal

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

func TestCustomerStep_CreateNew(t *testing.T) {
	setupMockModule(t, "test-cust")

	step, _ := newCustomerStep("customer", map[string]any{"module": "test-cust"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{"email": "test@example.com", "name": "Test User"},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["error"] != nil {
		t.Fatalf("unexpected error: %v", result.Output["error"])
	}
	if result.Output["customer_id"] == "" {
		t.Error("expected customer_id")
	}
	if result.Output["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", result.Output["email"])
	}
}

func TestCustomerStep_ReturnExisting(t *testing.T) {
	mock := setupMockModule(t, "test-cust-exist")

	// Create customer first.
	ctx := context.Background()
	first, _ := mock.EnsureCustomer(ctx, makeCustomerParams("existing@example.com"))

	step, _ := newCustomerStep("customer", map[string]any{"module": "test-cust-exist"})
	result, _ := step.Execute(ctx, nil, nil,
		map[string]any{"email": "existing@example.com"},
		nil, map[string]any{})
	if result.Output["customer_id"] != first.ID {
		t.Errorf("expected existing customer ID %s, got %v", first.ID, result.Output["customer_id"])
	}
}

func TestCustomerStep_MissingEmail(t *testing.T) {
	setupMockModule(t, "test-cust-noemail")
	step, _ := newCustomerStep("customer", map[string]any{"module": "test-cust-noemail"})
	result, _ := step.Execute(context.Background(), nil, nil, map[string]any{}, nil, map[string]any{})
	if result.Output["error"] == nil {
		t.Error("expected error for missing email")
	}
}

func TestSubscriptionCreateStep(t *testing.T) {
	mock := setupMockModule(t, "test-sub-create")
	ctx := context.Background()
	cust, _ := mock.EnsureCustomer(ctx, makeCustomerParams("sub@example.com"))

	step, _ := newSubscriptionCreateStep("sub", map[string]any{"module": "test-sub-create"})
	result, err := step.Execute(ctx, nil, nil,
		map[string]any{"customer_id": cust.ID, "price_id": "price_123"},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["subscription_id"] == "" {
		t.Error("expected subscription_id")
	}
	if result.Output["status"] != "active" {
		t.Errorf("expected status active, got %v", result.Output["status"])
	}
}

func TestSubscriptionCancelStep(t *testing.T) {
	mock := setupMockModule(t, "test-sub-cancel")
	ctx := context.Background()
	cust, _ := mock.EnsureCustomer(ctx, makeCustomerParams("cancel@example.com"))
	sub, _ := mock.CreateSubscription(ctx, makeSubParams(cust.ID))

	step, _ := newSubscriptionCancelStep("cancel-sub", map[string]any{"module": "test-sub-cancel"})
	result, err := step.Execute(ctx, nil, nil,
		map[string]any{"subscription_id": sub.ID},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["status"] != "canceled" {
		t.Errorf("expected canceled, got %v", result.Output["status"])
	}
}

func TestSubscriptionUpdateStep(t *testing.T) {
	mock := setupMockModule(t, "test-sub-update")
	ctx := context.Background()
	cust, _ := mock.EnsureCustomer(ctx, makeCustomerParams("update@example.com"))
	sub, _ := mock.CreateSubscription(ctx, makeSubParams(cust.ID))

	step, _ := newSubscriptionUpdateStep("update-sub", map[string]any{"module": "test-sub-update"})
	result, err := step.Execute(ctx, nil, nil,
		map[string]any{"subscription_id": sub.ID, "price_id": "price_new"},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["subscription_id"] == "" {
		t.Error("expected subscription_id in result")
	}
}

// helpers

func makeCustomerParams(email string) payments.CustomerParams {
	return payments.CustomerParams{Email: email, Name: "Test"}
}

func makeSubParams(customerID string) payments.SubscriptionParams {
	return payments.SubscriptionParams{CustomerID: customerID, PriceID: "price_test"}
}
