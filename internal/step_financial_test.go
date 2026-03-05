package internal

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

func TestTransferStep(t *testing.T) {
	setupMockModule(t, "test-transfer")

	step, _ := newTransferStep("transfer", map[string]any{"module": "test-transfer"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"amount":                 int64(5000),
			"currency":               "usd",
			"destination_account_id": "acct_123",
		},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["transfer_id"] == "" {
		t.Error("expected transfer_id")
	}
	if result.Output["status"] != "paid" {
		t.Errorf("expected paid, got %v", result.Output["status"])
	}
}

func TestTransferStep_MissingAmount(t *testing.T) {
	setupMockModule(t, "test-transfer-noamt")
	step, _ := newTransferStep("transfer", map[string]any{"module": "test-transfer-noamt"})
	result, _ := step.Execute(context.Background(), nil, nil, map[string]any{}, nil, map[string]any{})
	if result.Output["error"] == nil {
		t.Error("expected error for missing amount")
	}
}

func TestPayoutStep(t *testing.T) {
	setupMockModule(t, "test-payout")

	step, _ := newPayoutStep("payout", map[string]any{"module": "test-payout"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"amount":              int64(10000),
			"currency":            "usd",
			"destination_bank_id": "ba_123",
		},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["payout_id"] == "" {
		t.Error("expected payout_id")
	}
}

func TestInvoiceStep(t *testing.T) {
	mock := setupMockModule(t, "test-invoice")
	mock.invoices = []*payments.Invoice{
		{ID: "in_1", CustomerID: "cus_123", Amount: 5000, Currency: "usd", Status: "paid"},
		{ID: "in_2", CustomerID: "cus_123", Amount: 3000, Currency: "usd", Status: "open"},
	}

	step, _ := newInvoiceStep("invoice", map[string]any{"module": "test-invoice"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{"customer_id": "cus_123"},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["count"].(int) != 2 {
		t.Errorf("expected 2 invoices, got %v", result.Output["count"])
	}
}

func TestPaymentMethodAttachStep(t *testing.T) {
	setupMockModule(t, "test-pm-attach")

	step, _ := newPaymentMethodAttachStep("attach", map[string]any{"module": "test-pm-attach"})
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{
			"customer_id":       "cus_123",
			"payment_method_id": "pm_abc",
		},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["payment_method_id"] != "pm_abc" {
		t.Errorf("expected pm_abc, got %v", result.Output["payment_method_id"])
	}
}

func TestPaymentMethodListStep(t *testing.T) {
	mock := setupMockModule(t, "test-pm-list")
	ctx := context.Background()
	mock.AttachPaymentMethod(ctx, "cus_123", "pm_1")
	mock.AttachPaymentMethod(ctx, "cus_123", "pm_2")

	step, _ := newPaymentMethodListStep("list-pm", map[string]any{"module": "test-pm-list"})
	result, err := step.Execute(ctx, nil, nil,
		map[string]any{"customer_id": "cus_123"},
		nil, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["count"].(int) != 2 {
		t.Errorf("expected 2 methods, got %v", result.Output["count"])
	}
}
