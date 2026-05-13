package internal

import (
	"context"
	"testing"

	paymentsv1 "github.com/GoCodeAlone/workflow-plugin-payments/proto/payments/v1"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
	"google.golang.org/protobuf/types/known/anypb"
)

// typedPlugin creates a fresh paymentsPlugin for typed interface tests.
func typedPlugin() *paymentsPlugin { return &paymentsPlugin{} }

// --- TypedModuleProvider ---

func TestTypedModuleProvider_Types(t *testing.T) {
	p := typedPlugin()
	types := p.TypedModuleTypes()
	if len(types) != 1 || types[0] != "payments.provider" {
		t.Errorf("expected [payments.provider], got %v", types)
	}
}

func TestTypedModuleProvider_UnknownType(t *testing.T) {
	p := typedPlugin()
	_, err := p.CreateTypedModule("unknown.type", "m", nil)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestTypedModuleProvider_NilConfig(t *testing.T) {
	p := typedPlugin()
	// CreateTypedModule with nil config fails immediately: newProviderModule validates the
	// provider name and returns an error when the config is absent.
	_, err := p.CreateTypedModule("payments.provider", "p", nil)
	if err == nil {
		t.Fatal("expected error for missing provider field in config")
	}
}

func TestTypedModuleProvider_StripeConfig(t *testing.T) {
	p := typedPlugin()
	cfg := &paymentsv1.ProviderConfig{
		Provider:  "stripe",
		SecretKey: "sk_test_dummy",
	}
	anyConfig, err := anypb.New(cfg)
	if err != nil {
		t.Fatalf("anypb.New: %v", err)
	}
	mod, err := p.CreateTypedModule("payments.provider", "stripe-mod", anyConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod == nil {
		t.Fatal("expected non-nil module")
	}
}

// --- TypedStepProvider ---

func TestTypedStepProvider_Types(t *testing.T) {
	p := typedPlugin()
	types := p.TypedStepTypes()
	// Must match the 17 step types also registered in StepTypes().
	if len(types) != len(p.StepTypes()) {
		t.Errorf("TypedStepTypes count %d != StepTypes count %d", len(types), len(p.StepTypes()))
	}
}

func TestTypedStepProvider_UnknownType(t *testing.T) {
	p := typedPlugin()
	_, err := p.CreateTypedStep("unknown.step", "s", nil)
	if err == nil {
		t.Fatal("expected error for unknown step type")
	}
}

func TestTypedStepProvider_AllTypesCreatable(t *testing.T) {
	p := typedPlugin()
	for _, typ := range p.TypedStepTypes() {
		t.Run(typ, func(t *testing.T) {
			// Create with nil config — the factory validates typed_config, nil is allowed.
			step, err := p.CreateTypedStep(typ, "s", nil)
			if err != nil {
				t.Fatalf("CreateTypedStep(%q) error: %v", typ, err)
			}
			if step == nil {
				t.Fatalf("CreateTypedStep(%q) returned nil step", typ)
			}
		})
	}
}

// --- Typed step handlers (via typed Execute path) ---

func TestTypedCharge_Execute(t *testing.T) {
	mock := setupMockModule(t, "typed-test")
	_ = mock

	cfg := &paymentsv1.PaymentChargeConfig{Module: "typed-test"}
	anyConfig, _ := anypb.New(cfg)

	p := typedPlugin()
	step, err := p.CreateTypedStep("step.payment_charge", "charge", anyConfig)
	if err != nil {
		t.Fatalf("create step: %v", err)
	}
	// TypedStepInstance.Execute returns (nil, error) for the untyped path,
	// because typed steps require the typed_input RPC path instead.
	result, err := step.Execute(context.Background(), nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error from typed Execute (requires typed_input payload)")
	}
	if result != nil {
		t.Errorf("expected nil result from typed Execute, got: %v", result)
	}
}

func TestTypedCharge_Handler_MissingProvider(t *testing.T) {
	result, err := handleTypedCharge(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentChargeConfig, *paymentsv1.PaymentChargeInput]{
		Config: &paymentsv1.PaymentChargeConfig{Module: "nonexistent-provider"},
		Input:  &paymentsv1.PaymentChargeInput{Amount: 1000},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error == "" {
		t.Error("expected error in output for missing provider")
	}
}

func TestTypedCharge_Handler_MissingAmount(t *testing.T) {
	setupMockModule(t, "typed-charge-noamt")
	result, err := handleTypedCharge(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentChargeConfig, *paymentsv1.PaymentChargeInput]{
		Config: &paymentsv1.PaymentChargeConfig{Module: "typed-charge-noamt"},
		Input:  &paymentsv1.PaymentChargeInput{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error == "" {
		t.Error("expected error for missing amount")
	}
}

func TestTypedCharge_Handler_Success(t *testing.T) {
	setupMockModule(t, "typed-charge-ok")
	result, err := handleTypedCharge(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentChargeConfig, *paymentsv1.PaymentChargeInput]{
		Config: &paymentsv1.PaymentChargeConfig{Module: "typed-charge-ok"},
		Input:  &paymentsv1.PaymentChargeInput{Amount: 2000, Currency: "usd"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected output error: %s", result.Output.Error)
	}
	if result.Output.ChargeId == "" {
		t.Error("expected charge_id in output")
	}
	if result.Output.Status != "succeeded" {
		t.Errorf("expected status=succeeded, got %s", result.Output.Status)
	}
}

func TestTypedCapture_Handler_Success(t *testing.T) {
	mock := setupMockModule(t, "typed-capture-ok")
	charge, _ := mock.CreateCharge(context.Background(), chargeParamsManual())
	result, err := handleTypedCapture(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentCaptureConfig, *paymentsv1.PaymentCaptureInput]{
		Config: &paymentsv1.PaymentCaptureConfig{Module: "typed-capture-ok"},
		Input:  &paymentsv1.PaymentCaptureInput{ChargeId: charge.ID},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.Status != "succeeded" {
		t.Errorf("expected succeeded, got %s", result.Output.Status)
	}
}

func TestTypedRefund_Handler_Success(t *testing.T) {
	mock := setupMockModule(t, "typed-refund-ok")
	charge, _ := mock.CreateCharge(context.Background(), chargeParamsAuto())
	result, err := handleTypedRefund(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentRefundConfig, *paymentsv1.PaymentRefundInput]{
		Config: &paymentsv1.PaymentRefundConfig{Module: "typed-refund-ok"},
		Input:  &paymentsv1.PaymentRefundInput{ChargeId: charge.ID},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.RefundId == "" {
		t.Error("expected refund_id")
	}
}

func TestTypedCustomerEnsure_Handler_Success(t *testing.T) {
	setupMockModule(t, "typed-cust-ok")
	result, err := handleTypedCustomerEnsure(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentCustomerEnsureConfig, *paymentsv1.PaymentCustomerEnsureInput]{
		Config: &paymentsv1.PaymentCustomerEnsureConfig{Module: "typed-cust-ok"},
		Input:  &paymentsv1.PaymentCustomerEnsureInput{Email: "user@example.com", Name: "Test User"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.CustomerId == "" {
		t.Error("expected customer_id")
	}
}

func TestTypedWebhookEndpointEnsure_Handler_Success(t *testing.T) {
	setupMockModule(t, "typed-wh-ok")
	result, err := handleTypedWebhookEndpointEnsure(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentWebhookEndpointEnsureConfig, *paymentsv1.PaymentWebhookEndpointEnsureInput]{
		Config: &paymentsv1.PaymentWebhookEndpointEnsureConfig{Module: "typed-wh-ok"},
		Input: &paymentsv1.PaymentWebhookEndpointEnsureInput{
			Url:    "https://example.com/webhook",
			Events: []string{"payment_intent.succeeded"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.EndpointId == "" {
		t.Error("expected endpoint_id")
	}
	if !result.Output.Created {
		t.Error("expected created=true")
	}
}

func TestTypedModuleProvider_ImplementsInterface(t *testing.T) {
	var _ sdk.TypedModuleProvider = (*paymentsPlugin)(nil)
}

func TestTypedStepProvider_ImplementsInterface(t *testing.T) {
	var _ sdk.TypedStepProvider = (*paymentsPlugin)(nil)
}

// --- v0.4.3: Config-field precedence over Input (BMW templated-YAML pattern) ---

// TestTypedCapture_Handler_ConfigChargeID verifies Config.ChargeId takes
// precedence over Input.ChargeId so BMW's `config: { charge_id: "{{ .id }}" }`
// pattern works under strict-proto dispatch.
func TestTypedCapture_Handler_ConfigChargeID(t *testing.T) {
	mock := setupMockModule(t, "typed-capture-cfg")
	charge, _ := mock.CreateCharge(context.Background(), chargeParamsManual())
	result, err := handleTypedCapture(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentCaptureConfig, *paymentsv1.PaymentCaptureInput]{
		Config: &paymentsv1.PaymentCaptureConfig{
			Module:   "typed-capture-cfg",
			ChargeId: charge.ID,
		},
		Input: &paymentsv1.PaymentCaptureInput{}, // empty: Config must win
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.Status != "succeeded" {
		t.Errorf("expected succeeded, got %s", result.Output.Status)
	}
}

// TestTypedCapture_Handler_ConfigAmount verifies Config.Amount takes
// precedence over Input.Amount when non-zero.
func TestTypedCapture_Handler_ConfigAmount(t *testing.T) {
	mock := setupMockModule(t, "typed-capture-amt")
	charge, _ := mock.CreateCharge(context.Background(), chargeParamsManual())
	result, err := handleTypedCapture(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentCaptureConfig, *paymentsv1.PaymentCaptureInput]{
		Config: &paymentsv1.PaymentCaptureConfig{
			Module:   "typed-capture-amt",
			ChargeId: charge.ID,
			Amount:   500,
		},
		Input: &paymentsv1.PaymentCaptureInput{Amount: 999}, // ignored
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
}

// TestTypedFeeCalculate_Handler_ConfigFields verifies amount/currency/
// platform_fee_percent flow from Config when set, falling back to Input.
func TestTypedFeeCalculate_Handler_ConfigFields(t *testing.T) {
	setupMockModule(t, "typed-fee-cfg")
	result, err := handleTypedFeeCalculate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentFeeCalculateConfig, *paymentsv1.PaymentFeeCalculateInput]{
		Config: &paymentsv1.PaymentFeeCalculateConfig{
			Module:             "typed-fee-cfg",
			Amount:             1000,
			Currency:           "usd",
			PlatformFeePercent: 5.0,
		},
		Input: &paymentsv1.PaymentFeeCalculateInput{}, // empty: Config must win
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected output error: %s", result.Output.Error)
	}
	if result.Output.TotalCharge == 0 {
		t.Error("expected non-zero total_charge from config-driven calc")
	}
	if result.Output.PlatformFee == 0 {
		t.Error("expected non-zero platform_fee with platform_fee_percent=5.0")
	}
}

// TestTypedFeeCalculate_Handler_ConfigFallsBackToInput verifies that when
// Config.Amount is zero, Input.Amount is used.
func TestTypedFeeCalculate_Handler_ConfigFallsBackToInput(t *testing.T) {
	setupMockModule(t, "typed-fee-fallback")
	result, err := handleTypedFeeCalculate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentFeeCalculateConfig, *paymentsv1.PaymentFeeCalculateInput]{
		Config: &paymentsv1.PaymentFeeCalculateConfig{Module: "typed-fee-fallback"},
		Input: &paymentsv1.PaymentFeeCalculateInput{
			Amount:             2000,
			Currency:           "usd",
			PlatformFeePercent: 3.0,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected output error: %s", result.Output.Error)
	}
	if result.Output.TotalCharge == 0 {
		t.Error("expected non-zero total_charge from input fallback")
	}
}

// TestTypedWebhookEndpointEnsure_Handler_ConfigDescription verifies
// Config.Description takes precedence over Input.Description (BMW pattern).
func TestTypedWebhookEndpointEnsure_Handler_ConfigDescription(t *testing.T) {
	setupMockModule(t, "typed-wh-cfg")
	result, err := handleTypedWebhookEndpointEnsure(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentWebhookEndpointEnsureConfig, *paymentsv1.PaymentWebhookEndpointEnsureInput]{
		Config: &paymentsv1.PaymentWebhookEndpointEnsureConfig{
			Module:      "typed-wh-cfg",
			Description: "BMW Issuing webhook",
		},
		Input: &paymentsv1.PaymentWebhookEndpointEnsureInput{
			Url:    "https://example.com/webhook",
			Events: []string{"payment_intent.succeeded"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.EndpointId == "" {
		t.Error("expected endpoint_id")
	}
}
