package internal

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

func setupMockModule(t *testing.T, name string) *mockProvider {
	t.Helper()
	mock := newMockProvider()
	RegisterProvider(name, mock)
	t.Cleanup(func() { UnregisterProvider(name) })
	return mock
}

func TestChargeStep_AutoCapture(t *testing.T) {
	mock := setupMockModule(t, "test")

	step, err := newChargeStep("charge", map[string]any{"module": "test"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{"amount": int64(1000), "currency": "usd"},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["error"] != nil {
		t.Fatalf("unexpected error: %v", result.Output["error"])
	}
	if result.Output["charge_id"] == "" {
		t.Error("expected charge_id")
	}
	if result.Output["status"] != "succeeded" {
		t.Errorf("expected status succeeded, got %v", result.Output["status"])
	}

	_ = mock
}

func TestChargeStep_ManualCapture(t *testing.T) {
	setupMockModule(t, "test-manual")

	step, _ := newChargeStep("charge", map[string]any{"module": "test-manual"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"amount":         int64(5000),
			"currency":       "usd",
			"capture_method": "manual",
		},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["status"] != "requires_capture" {
		t.Errorf("expected requires_capture, got %v", result.Output["status"])
	}
}

func TestChargeStep_MissingAmount(t *testing.T) {
	setupMockModule(t, "test-noamt")
	step, _ := newChargeStep("charge", map[string]any{"module": "test-noamt"})
	result, _ := step.Execute(context.Background(), nil, nil, map[string]any{}, nil, map[string]any{})
	if result.Output["error"] == nil {
		t.Error("expected error for missing amount")
	}
}

func TestCaptureStep(t *testing.T) {
	mock := setupMockModule(t, "test-cap")
	charge, _ := mock.CreateCharge(context.Background(), chargeParamsManual())

	step, _ := newCaptureStep("capture", map[string]any{"module": "test-cap"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{"charge_id": charge.ID},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["status"] != "succeeded" {
		t.Errorf("expected succeeded, got %v", result.Output["status"])
	}
}

func TestRefundStep_Full(t *testing.T) {
	mock := setupMockModule(t, "test-refund")
	charge, _ := mock.CreateCharge(context.Background(), chargeParamsAuto())

	step, _ := newRefundStep("refund", map[string]any{"module": "test-refund"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{"charge_id": charge.ID},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["error"] != nil {
		t.Fatalf("unexpected error: %v", result.Output["error"])
	}
	if result.Output["refund_id"] == "" {
		t.Error("expected refund_id")
	}
}

func TestRefundStep_Partial(t *testing.T) {
	mock := setupMockModule(t, "test-refund-partial")
	charge, _ := mock.CreateCharge(context.Background(), chargeParamsAuto())

	step, _ := newRefundStep("refund", map[string]any{"module": "test-refund-partial"})
	result, _ := step.Execute(context.Background(), nil, nil,
		map[string]any{"charge_id": charge.ID, "amount": int64(500)},
		nil, map[string]any{})
	if result.Output["error"] != nil {
		t.Fatalf("unexpected error: %v", result.Output["error"])
	}
}

func TestFeeCalcStep_Stripe(t *testing.T) {
	setupMockModule(t, "test-fees")

	step, _ := newFeeCalcStep("fees", map[string]any{"module": "test-fees"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"amount":               int64(10000), // $100.00
			"currency":             "usd",
			"platform_fee_percent": float64(5),
		},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["error"] != nil {
		t.Fatalf("fee error: %v", result.Output["error"])
	}
	processingFee := result.Output["processing_fee"].(int64)
	platformFee := result.Output["platform_fee"].(int64)
	if processingFee <= 0 {
		t.Error("expected positive processing fee")
	}
	if platformFee <= 0 {
		t.Error("expected positive platform fee")
	}
	totalCharge := result.Output["total_charge"].(int64)
	if totalCharge != 10000 {
		t.Errorf("expected total_charge 10000, got %d", totalCharge)
	}
}

// helpers

func chargeParamsAuto() payments.ChargeParams {
	return payments.ChargeParams{Amount: 1000, Currency: "usd", CaptureMethod: "automatic"}
}

func chargeParamsManual() payments.ChargeParams {
	return payments.ChargeParams{Amount: 1000, Currency: "usd", CaptureMethod: "manual"}
}
