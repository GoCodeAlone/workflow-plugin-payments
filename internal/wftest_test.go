package internal_test

import (
	"testing"

	"github.com/GoCodeAlone/workflow/wftest"
)

func TestWFTest_ChargePipeline(t *testing.T) {
	rec := wftest.RecordStep("step.payment_charge")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  charge:
    steps:
      - name: charge
        type: step.payment_charge
        config:
          module: payments
          amount_key: amount
          currency_key: currency
`),
		rec.WithOutput(map[string]any{
			"charge_id": "ch_test123",
			"status":    "succeeded",
		}),
	)

	result := h.ExecutePipeline("charge", map[string]any{
		"amount":   int64(5000),
		"currency": "usd",
	})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if rec.CallCount() != 1 {
		t.Errorf("expected charge step called once, got %d", rec.CallCount())
	}
	if result.StepResults["charge"]["charge_id"] != "ch_test123" {
		t.Errorf("unexpected charge_id: %v", result.StepResults["charge"]["charge_id"])
	}
}

func TestWFTest_RefundPipeline(t *testing.T) {
	rec := wftest.RecordStep("step.payment_refund")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  refund:
    steps:
      - name: refund
        type: step.payment_refund
        config:
          module: payments
          charge_id_key: charge_id
`),
		rec.WithOutput(map[string]any{
			"refund_id": "re_test456",
			"status":    "succeeded",
		}),
	)

	result := h.ExecutePipeline("refund", map[string]any{"charge_id": "ch_test123"})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if rec.CallCount() != 1 {
		t.Errorf("expected refund step called once, got %d", rec.CallCount())
	}
	if result.StepResults["refund"]["refund_id"] != "re_test456" {
		t.Errorf("unexpected refund_id: %v", result.StepResults["refund"]["refund_id"])
	}
}

func TestWFTest_CustomerSubscriptionPipeline(t *testing.T) {
	custRec := wftest.RecordStep("step.payment_customer_ensure")
	subRec := wftest.RecordStep("step.payment_subscription_create")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  customer-sub:
    steps:
      - name: customer
        type: step.payment_customer_ensure
        config:
          module: payments
          email_key: email
      - name: subscribe
        type: step.payment_subscription_create
        config:
          module: payments
          customer_id_key: customer_id
          price_id: price_monthly
`),
		custRec.WithOutput(map[string]any{"customer_id": "cus_test789"}),
		subRec.WithOutput(map[string]any{
			"subscription_id": "sub_test012",
			"status":          "active",
		}),
	)

	result := h.ExecutePipeline("customer-sub", map[string]any{"email": "user@example.com"})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if custRec.CallCount() != 1 {
		t.Errorf("expected customer_ensure called once, got %d", custRec.CallCount())
	}
	if subRec.CallCount() != 1 {
		t.Errorf("expected subscription_create called once, got %d", subRec.CallCount())
	}
	if result.StepResults["subscribe"]["status"] != "active" {
		t.Errorf("expected status=active, got %v", result.StepResults["subscribe"]["status"])
	}
}
