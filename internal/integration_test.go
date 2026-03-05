package internal

import (
	"context"
	"testing"
)

// TestIntegration_ChargeCaptureRefundFlow tests a full charge → capture → refund flow.
func TestIntegration_ChargeCaptureRefundFlow(t *testing.T) {
	setupMockModule(t, "integration")
	ctx := context.Background()

	chargeStep, _ := newChargeStep("charge", map[string]any{"module": "integration"})
	captureStep, _ := newCaptureStep("capture", map[string]any{"module": "integration"})
	refundStep, _ := newRefundStep("refund", map[string]any{"module": "integration"})

	// 1. Create a charge (manual capture).
	chargeResult, err := chargeStep.Execute(ctx, nil, nil,
		map[string]any{"amount": int64(5000), "currency": "usd", "capture_method": "manual"},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if chargeResult.Output["error"] != nil {
		t.Fatalf("charge error: %v", chargeResult.Output["error"])
	}
	chargeID, _ := chargeResult.Output["charge_id"].(string)
	if chargeID == "" {
		t.Fatal("expected charge_id")
	}

	// 2. Capture the charge.
	captureResult, err := captureStep.Execute(ctx, nil, nil,
		map[string]any{"charge_id": chargeID},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if captureResult.Output["status"] != "succeeded" {
		t.Errorf("expected capture succeeded, got %v", captureResult.Output["status"])
	}

	// 3. Refund the charge.
	refundResult, err := refundStep.Execute(ctx, nil, nil,
		map[string]any{"charge_id": chargeID},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if refundResult.Output["error"] != nil {
		t.Fatalf("refund error: %v", refundResult.Output["error"])
	}
	if refundResult.Output["refund_id"] == "" {
		t.Error("expected refund_id")
	}
}

// TestIntegration_CustomerSubscriptionFlow tests customer → subscription → cancel.
func TestIntegration_CustomerSubscriptionFlow(t *testing.T) {
	setupMockModule(t, "integration-sub")
	ctx := context.Background()

	customerStep, _ := newCustomerStep("customer", map[string]any{"module": "integration-sub"})
	subCreateStep, _ := newSubscriptionCreateStep("sub-create", map[string]any{"module": "integration-sub"})
	subCancelStep, _ := newSubscriptionCancelStep("sub-cancel", map[string]any{"module": "integration-sub"})

	// 1. Ensure customer.
	custResult, err := customerStep.Execute(ctx, nil, nil,
		map[string]any{"email": "integration@example.com", "name": "Integration User"},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	customerID, _ := custResult.Output["customer_id"].(string)
	if customerID == "" {
		t.Fatal("expected customer_id")
	}

	// 2. Create subscription.
	subResult, err := subCreateStep.Execute(ctx, nil, nil,
		map[string]any{"customer_id": customerID, "price_id": "price_123"},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if subResult.Output["status"] != "active" {
		t.Errorf("expected active, got %v", subResult.Output["status"])
	}
	subID, _ := subResult.Output["subscription_id"].(string)

	// 3. Cancel subscription.
	cancelResult, err := subCancelStep.Execute(ctx, nil, nil,
		map[string]any{"subscription_id": subID},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if cancelResult.Output["status"] != "canceled" {
		t.Errorf("expected canceled, got %v", cancelResult.Output["status"])
	}
}

// TestIntegration_PluginManifestAndStepTypes verifies the plugin manifest and step types.
func TestIntegration_PluginManifestAndStepTypes(t *testing.T) {
	plugin := NewPaymentsPlugin()

	manifest := plugin.Manifest()
	if manifest.Name != "workflow-plugin-payments" {
		t.Errorf("unexpected plugin name: %s", manifest.Name)
	}

	pp := plugin.(*paymentsPlugin)
	types := pp.StepTypes()
	if len(types) != 16 {
		t.Errorf("expected 16 step types, got %d", len(types))
	}
}

// TestIntegration_PluginCreateAllSteps verifies all step types can be created.
func TestIntegration_PluginCreateAllSteps(t *testing.T) {
	pp := &paymentsPlugin{}
	for _, stepType := range pp.StepTypes() {
		_, err := pp.CreateStep(stepType, "test-"+stepType, map[string]any{})
		if err != nil {
			t.Errorf("CreateStep(%q) failed: %v", stepType, err)
		}
	}
}
