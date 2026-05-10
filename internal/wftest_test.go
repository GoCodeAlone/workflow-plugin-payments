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

func TestWFTest_CapturePipeline(t *testing.T) {
	rec := wftest.RecordStep("step.payment_capture")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  capture:
    steps:
      - name: capture
        type: step.payment_capture
        config:
          module: payments
`),
		rec.WithOutput(map[string]any{
			"charge_id": "ch_test123",
			"status":    "succeeded",
		}),
	)

	result := h.ExecutePipeline("capture", map[string]any{"charge_id": "ch_test123"})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if rec.CallCount() != 1 {
		t.Errorf("expected capture step called once, got %d", rec.CallCount())
	}
	if result.StepResults["capture"]["status"] != "succeeded" {
		t.Errorf("expected status=succeeded, got %v", result.StepResults["capture"]["status"])
	}
}

func TestWFTest_FeeCalculatePipeline(t *testing.T) {
	rec := wftest.RecordStep("step.payment_fee_calculate")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  fee-calc:
    steps:
      - name: calc
        type: step.payment_fee_calculate
        config:
          module: payments
`),
		rec.WithOutput(map[string]any{
			"fee_amount":    int64(145),
			"net_amount":    int64(4855),
			"fee_breakdown": map[string]any{"stripe": int64(145)},
		}),
	)

	result := h.ExecutePipeline("fee-calc", map[string]any{
		"amount":   int64(5000),
		"currency": "usd",
		"provider": "stripe",
	})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if rec.CallCount() != 1 {
		t.Errorf("expected fee_calculate step called once, got %d", rec.CallCount())
	}
	if result.StepResults["calc"]["fee_amount"] != int64(145) {
		t.Errorf("unexpected fee_amount: %v", result.StepResults["calc"]["fee_amount"])
	}
}

func TestWFTest_SubscriptionUpdateCancelPipeline(t *testing.T) {
	updateRec := wftest.RecordStep("step.payment_subscription_update")
	cancelRec := wftest.RecordStep("step.payment_subscription_cancel")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  sub-lifecycle:
    steps:
      - name: update
        type: step.payment_subscription_update
        config:
          module: payments
      - name: cancel
        type: step.payment_subscription_cancel
        config:
          module: payments
`),
		updateRec.WithOutput(map[string]any{
			"subscription_id": "sub_test012",
			"status":          "active",
		}),
		cancelRec.WithOutput(map[string]any{
			"subscription_id": "sub_test012",
			"status":          "canceled",
		}),
	)

	result := h.ExecutePipeline("sub-lifecycle", map[string]any{
		"subscription_id": "sub_test012",
		"price_id":        "price_new",
	})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if updateRec.CallCount() != 1 {
		t.Errorf("expected subscription_update called once, got %d", updateRec.CallCount())
	}
	if cancelRec.CallCount() != 1 {
		t.Errorf("expected subscription_cancel called once, got %d", cancelRec.CallCount())
	}
	if result.StepResults["cancel"]["status"] != "canceled" {
		t.Errorf("expected status=canceled, got %v", result.StepResults["cancel"]["status"])
	}
}

func TestWFTest_CheckoutPortalPipeline(t *testing.T) {
	checkoutRec := wftest.RecordStep("step.payment_checkout_create")
	portalRec := wftest.RecordStep("step.payment_portal_create")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  checkout-portal:
    steps:
      - name: checkout
        type: step.payment_checkout_create
        config:
          module: payments
      - name: portal
        type: step.payment_portal_create
        config:
          module: payments
`),
		checkoutRec.WithOutput(map[string]any{
			"url":        "https://checkout.stripe.com/pay/cs_test123",
			"session_id": "cs_test123",
		}),
		portalRec.WithOutput(map[string]any{
			"url":        "https://billing.stripe.com/session/bps_test123",
			"session_id": "bps_test123",
		}),
	)

	result := h.ExecutePipeline("checkout-portal", map[string]any{
		"customer_id": "cus_test789",
		"price_id":    "price_monthly",
		"success_url": "https://example.com/success",
		"cancel_url":  "https://example.com/cancel",
		"return_url":  "https://example.com/billing",
	})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if checkoutRec.CallCount() != 1 {
		t.Errorf("expected checkout_create called once, got %d", checkoutRec.CallCount())
	}
	if portalRec.CallCount() != 1 {
		t.Errorf("expected portal_create called once, got %d", portalRec.CallCount())
	}
	if result.StepResults["checkout"]["session_id"] != "cs_test123" {
		t.Errorf("unexpected checkout session_id: %v", result.StepResults["checkout"]["session_id"])
	}
}

func TestWFTest_WebhookVerifyPipeline(t *testing.T) {
	rec := wftest.RecordStep("step.payment_webhook_verify")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  webhook-verify:
    steps:
      - name: verify
        type: step.payment_webhook_verify
        config:
          module: payments
`),
		rec.WithOutput(map[string]any{
			"valid":        true,
			"event_type":   "payment_intent.succeeded",
			"event_id":     "evt_test123",
			"payload_json": `{"id":"evt_test123"}`,
		}),
	)

	result := h.ExecutePipeline("webhook-verify", map[string]any{
		"request_body":       `{"id":"evt_test123","type":"payment_intent.succeeded"}`,
		"stripe_signature":   "t=1234,v1=abc",
		"webhook_secret_key": "whsec_test",
	})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if rec.CallCount() != 1 {
		t.Errorf("expected webhook_verify called once, got %d", rec.CallCount())
	}
	if result.StepResults["verify"]["valid"] != true {
		t.Errorf("expected valid=true, got %v", result.StepResults["verify"]["valid"])
	}
}

func TestWFTest_TransferPayoutPipeline(t *testing.T) {
	transferRec := wftest.RecordStep("step.payment_transfer")
	payoutRec := wftest.RecordStep("step.payment_payout")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  transfer-payout:
    steps:
      - name: transfer
        type: step.payment_transfer
        config:
          module: payments
      - name: payout
        type: step.payment_payout
        config:
          module: payments
`),
		transferRec.WithOutput(map[string]any{
			"transfer_id": "tr_test123",
			"status":      "paid",
		}),
		payoutRec.WithOutput(map[string]any{
			"payout_id": "po_test456",
			"status":    "in_transit",
		}),
	)

	result := h.ExecutePipeline("transfer-payout", map[string]any{
		"amount":                 int64(10000),
		"currency":               "usd",
		"destination_account_id": "acct_test123",
		"destination_bank_id":    "bank_test456",
	})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if transferRec.CallCount() != 1 {
		t.Errorf("expected transfer called once, got %d", transferRec.CallCount())
	}
	if payoutRec.CallCount() != 1 {
		t.Errorf("expected payout called once, got %d", payoutRec.CallCount())
	}
	if result.StepResults["transfer"]["transfer_id"] != "tr_test123" {
		t.Errorf("unexpected transfer_id: %v", result.StepResults["transfer"]["transfer_id"])
	}
}

func TestWFTest_InvoiceListPipeline(t *testing.T) {
	rec := wftest.RecordStep("step.payment_invoice_list")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  invoice-list:
    steps:
      - name: list_invoices
        type: step.payment_invoice_list
        config:
          module: payments
`),
		rec.WithOutput(map[string]any{
			"invoices": `[{"id":"in_test1","amount":5000,"status":"paid"}]`,
			"count":    1,
		}),
	)

	result := h.ExecutePipeline("invoice-list", map[string]any{"customer_id": "cus_test789"})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if rec.CallCount() != 1 {
		t.Errorf("expected invoice_list called once, got %d", rec.CallCount())
	}
	if result.StepResults["list_invoices"]["count"] != 1 {
		t.Errorf("expected count=1, got %v", result.StepResults["list_invoices"]["count"])
	}
}

func TestWFTest_PaymentMethodAttachListPipeline(t *testing.T) {
	attachRec := wftest.RecordStep("step.payment_method_attach")
	listRec := wftest.RecordStep("step.payment_method_list")
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  method-lifecycle:
    steps:
      - name: attach
        type: step.payment_method_attach
        config:
          module: payments
      - name: list_methods
        type: step.payment_method_list
        config:
          module: payments
`),
		attachRec.WithOutput(map[string]any{
			"payment_method_id": "pm_test123",
			"type":              "card",
		}),
		listRec.WithOutput(map[string]any{
			"payment_methods": `[{"id":"pm_test123","type":"card"}]`,
			"count":           1,
		}),
	)

	result := h.ExecutePipeline("method-lifecycle", map[string]any{
		"customer_id":       "cus_test789",
		"payment_method_id": "pm_test123",
	})
	if result.Error != nil {
		t.Fatalf("pipeline error: %v", result.Error)
	}
	if attachRec.CallCount() != 1 {
		t.Errorf("expected method_attach called once, got %d", attachRec.CallCount())
	}
	if listRec.CallCount() != 1 {
		t.Errorf("expected method_list called once, got %d", listRec.CallCount())
	}
	if result.StepResults["attach"]["payment_method_id"] != "pm_test123" {
		t.Errorf("unexpected payment_method_id: %v", result.StepResults["attach"]["payment_method_id"])
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
