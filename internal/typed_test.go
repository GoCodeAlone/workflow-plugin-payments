package internal

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
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
// v0.4.4: Config.Amount is now `string` (BMW templates render as strings).
func TestTypedFeeCalculate_Handler_ConfigFields(t *testing.T) {
	setupMockModule(t, "typed-fee-cfg")
	result, err := handleTypedFeeCalculate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentFeeCalculateConfig, *paymentsv1.PaymentFeeCalculateInput]{
		Config: &paymentsv1.PaymentFeeCalculateConfig{
			Module:             "typed-fee-cfg",
			Amount:             "1000",
			Currency:           "usd",
			PlatformFeePercent: "5.0",
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

// --- v0.4.4: string-typed Config amount/bool fields (BMW YAML-template pattern) ---

// TestParseConfigInt64 covers the helper's contract: empty/garbage→0,
// well-formed numeric string→int64.
func TestParseConfigInt64(t *testing.T) {
	cases := map[string]int64{
		"":      0,
		"0":     0,
		"42":    42,
		"4200":  4200,
		"-100":  -100,
		"abc":   0,
		"1.5":   0,
		"99 ":   0, // surrounding whitespace not tolerated
	}
	for in, want := range cases {
		if got := parseConfigInt64(in); got != want {
			t.Errorf("parseConfigInt64(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestParseConfigBool covers the helper's contract: empty/garbage→fallback,
// well-formed bool string→parsed bool.
func TestParseConfigBool(t *testing.T) {
	type c struct {
		in       string
		fallback bool
		want     bool
	}
	cases := []c{
		{"", false, false},
		{"", true, true},
		{"true", false, true},
		{"false", true, false},
		{"True", false, true},
		{"1", false, true},
		{"0", true, false},
		{"yes", true, true},  // garbage → fallback
		{"nope", false, false},
	}
	for _, tc := range cases {
		if got := parseConfigBool(tc.in, tc.fallback); got != tc.want {
			t.Errorf("parseConfigBool(%q, %v) = %v, want %v", tc.in, tc.fallback, got, tc.want)
		}
	}
}

// TestTypedFeeCalculate_Handler_ConfigStringAmount verifies that a templated
// string amount like "1000" decodes correctly and reaches the provider as
// int64 (closes the v0.4.3 strict-proto decode failure on BMW templates).
func TestTypedFeeCalculate_Handler_ConfigStringAmount(t *testing.T) {
	setupMockModule(t, "typed-fee-strcfg")
	result, err := handleTypedFeeCalculate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentFeeCalculateConfig, *paymentsv1.PaymentFeeCalculateInput]{
		Config: &paymentsv1.PaymentFeeCalculateConfig{
			Module:             "typed-fee-strcfg",
			Amount:             "1000", // BMW renders "{{ .body.amount }}" as a string
			Currency:           "usd",
			PlatformFeePercent: "5.0", // v0.4.5: also string
		},
		Input: &paymentsv1.PaymentFeeCalculateInput{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected output error: %s", result.Output.Error)
	}
	if result.Output.TotalCharge == 0 {
		t.Error("expected non-zero total_charge from parsed string amount")
	}
}

// TestTypedFeeCalculate_Handler_ConfigStringAmountInvalid verifies that an
// unparseable Config.Amount falls back to Input.Amount rather than crashing.
func TestTypedFeeCalculate_Handler_ConfigStringAmountInvalid(t *testing.T) {
	setupMockModule(t, "typed-fee-strcfg-bad")
	result, err := handleTypedFeeCalculate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentFeeCalculateConfig, *paymentsv1.PaymentFeeCalculateInput]{
		Config: &paymentsv1.PaymentFeeCalculateConfig{
			Module: "typed-fee-strcfg-bad",
			Amount: "not-a-number",
		},
		Input: &paymentsv1.PaymentFeeCalculateInput{Amount: 2000, Currency: "usd"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected output error: %s", result.Output.Error)
	}
	if result.Output.TotalCharge == 0 {
		t.Error("expected non-zero total_charge from Input fallback when Config.Amount is unparseable")
	}
}

// TestTypedRefund_Handler_ConfigFields verifies all three new Config fields
// (charge_id, amount as string, reason) take precedence over Input.
func TestTypedRefund_Handler_ConfigFields(t *testing.T) {
	mock := setupMockModule(t, "typed-refund-cfg")
	charge, _ := mock.CreateCharge(context.Background(), chargeParamsAuto())
	result, err := handleTypedRefund(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentRefundConfig, *paymentsv1.PaymentRefundInput]{
		Config: &paymentsv1.PaymentRefundConfig{
			Module:   "typed-refund-cfg",
			ChargeId: charge.ID,
			Amount:   "500", // partial refund via BMW template
			Reason:   "requested_by_customer",
		},
		Input: &paymentsv1.PaymentRefundInput{}, // empty: Config must win
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

// TestTypedRefund_Handler_ConfigFallsBackToInput verifies that when the
// Config fields are empty, Input fields drive the refund.
func TestTypedRefund_Handler_ConfigFallsBackToInput(t *testing.T) {
	mock := setupMockModule(t, "typed-refund-fallback")
	charge, _ := mock.CreateCharge(context.Background(), chargeParamsAuto())
	result, err := handleTypedRefund(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentRefundConfig, *paymentsv1.PaymentRefundInput]{
		Config: &paymentsv1.PaymentRefundConfig{Module: "typed-refund-fallback"},
		Input: &paymentsv1.PaymentRefundInput{
			ChargeId: charge.ID,
			Amount:   250,
			Reason:   "duplicate",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.RefundId == "" {
		t.Error("expected refund_id from Input fallback path")
	}
}

// TestTypedSubscriptionCancel_Handler_ConfigFields verifies Config.SubscriptionId
// and Config.CancelAtPeriodEnd (as string) take precedence over Input.
func TestTypedSubscriptionCancel_Handler_ConfigFields(t *testing.T) {
	mock := setupMockModule(t, "typed-subcancel-cfg")
	sub, _ := mock.CreateSubscription(context.Background(), payments.SubscriptionParams{
		CustomerID: "cus_test",
		PriceID:    "price_test",
	})
	result, err := handleTypedSubscriptionCancel(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentSubscriptionCancelConfig, *paymentsv1.PaymentSubscriptionCancelInput]{
		Config: &paymentsv1.PaymentSubscriptionCancelConfig{
			Module:            "typed-subcancel-cfg",
			SubscriptionId:    sub.ID,
			CancelAtPeriodEnd: "true", // BMW renders bool as string
		},
		Input: &paymentsv1.PaymentSubscriptionCancelInput{}, // empty: Config must win
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.SubscriptionId == "" {
		t.Error("expected subscription_id from Config-driven cancel")
	}
}

// TestTypedSubscriptionCancel_Handler_ConfigFallsBackToInput verifies that
// an empty Config.SubscriptionId falls back to Input.SubscriptionId, and
// CancelAtPeriodEnd respects the Input fallback when Config string is empty.
func TestTypedSubscriptionCancel_Handler_ConfigFallsBackToInput(t *testing.T) {
	mock := setupMockModule(t, "typed-subcancel-fallback")
	sub, _ := mock.CreateSubscription(context.Background(), payments.SubscriptionParams{
		CustomerID: "cus_test",
		PriceID:    "price_test",
	})
	result, err := handleTypedSubscriptionCancel(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentSubscriptionCancelConfig, *paymentsv1.PaymentSubscriptionCancelInput]{
		Config: &paymentsv1.PaymentSubscriptionCancelConfig{Module: "typed-subcancel-fallback"},
		Input: &paymentsv1.PaymentSubscriptionCancelInput{
			SubscriptionId:    sub.ID,
			CancelAtPeriodEnd: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.SubscriptionId == "" {
		t.Error("expected subscription_id from Input fallback path")
	}
}

// --- v0.4.5: Round-3 Config field coverage (BMW comprehensive sweep) ---

// TestParseConfigFloat64 covers the helper's contract: empty/garbage→0,
// well-formed numeric string→float64.
func TestParseConfigFloat64(t *testing.T) {
	cases := map[string]float64{
		"":     0,
		"0":    0,
		"5":    5,
		"5.0":  5.0,
		"2.5":  2.5,
		"-1.5": -1.5,
		"abc":  0,
	}
	for in, want := range cases {
		if got := parseConfigFloat64(in); got != want {
			t.Errorf("parseConfigFloat64(%q) = %f, want %f", in, got, want)
		}
	}
}

// TestTypedFeeCalculate_Handler_ConfigStringPlatformFeePercent verifies that
// a string platform_fee_percent ("5.0", BMW template output) decodes correctly
// and yields a non-zero platform_fee in the result.
func TestTypedFeeCalculate_Handler_ConfigStringPlatformFeePercent(t *testing.T) {
	setupMockModule(t, "typed-fee-pct")
	result, err := handleTypedFeeCalculate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentFeeCalculateConfig, *paymentsv1.PaymentFeeCalculateInput]{
		Config: &paymentsv1.PaymentFeeCalculateConfig{
			Module:             "typed-fee-pct",
			Amount:             "1000",
			Currency:           "usd",
			PlatformFeePercent: "5.0",
		},
		Input: &paymentsv1.PaymentFeeCalculateInput{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected output error: %s", result.Output.Error)
	}
	if result.Output.PlatformFee == 0 {
		t.Error("expected non-zero platform_fee from parsed string platform_fee_percent")
	}
}

// TestTypedFeeCalculate_Handler_ConfigPlatformFeePercentInvalidFallback verifies
// that an unparseable Config.PlatformFeePercent falls back to Input.PlatformFeePercent.
func TestTypedFeeCalculate_Handler_ConfigPlatformFeePercentInvalidFallback(t *testing.T) {
	setupMockModule(t, "typed-fee-pct-bad")
	result, err := handleTypedFeeCalculate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentFeeCalculateConfig, *paymentsv1.PaymentFeeCalculateInput]{
		Config: &paymentsv1.PaymentFeeCalculateConfig{
			Module:             "typed-fee-pct-bad",
			Amount:             "1000",
			PlatformFeePercent: "not-a-number",
		},
		Input: &paymentsv1.PaymentFeeCalculateInput{PlatformFeePercent: 3.0, Currency: "usd"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected output error: %s", result.Output.Error)
	}
	if result.Output.PlatformFee == 0 {
		t.Error("expected non-zero platform_fee from Input fallback")
	}
}

// TestTypedCharge_Handler_ConfigFields verifies all v0.4.5 new Config fields
// (amount, currency, capture_method, description, customer_id) take precedence
// over Input for templated YAML configs.
func TestTypedCharge_Handler_ConfigFields(t *testing.T) {
	setupMockModule(t, "typed-charge-cfg")
	result, err := handleTypedCharge(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentChargeConfig, *paymentsv1.PaymentChargeInput]{
		Config: &paymentsv1.PaymentChargeConfig{
			Module:        "typed-charge-cfg",
			Amount:        "1500", // BMW template "{{ .total_charge }}"
			Currency:      "usd",
			CaptureMethod: "manual",
			Description:   "BuyMyWishlist contribution",
			CustomerId:    "cus_cfg",
		},
		Input: &paymentsv1.PaymentChargeInput{}, // empty: Config must win
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.ChargeId == "" {
		t.Error("expected charge_id from Config-driven charge")
	}
	if result.Output.Status != "requires_capture" {
		t.Errorf("expected status=requires_capture (manual capture), got %s", result.Output.Status)
	}
	if result.Output.Amount != 1500 {
		t.Errorf("expected amount=1500 (from Config), got %d", result.Output.Amount)
	}
}

// TestTypedCharge_Handler_ConfigFallsBackToInput verifies Input-side fields
// drive when Config is empty.
func TestTypedCharge_Handler_ConfigFallsBackToInput(t *testing.T) {
	setupMockModule(t, "typed-charge-fallback")
	result, err := handleTypedCharge(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentChargeConfig, *paymentsv1.PaymentChargeInput]{
		Config: &paymentsv1.PaymentChargeConfig{Module: "typed-charge-fallback"},
		Input: &paymentsv1.PaymentChargeInput{
			Amount:        2000,
			Currency:      "eur",
			CaptureMethod: "automatic",
			Description:   "fallback charge",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.Amount != 2000 {
		t.Errorf("expected amount=2000 from Input, got %d", result.Output.Amount)
	}
}

// TestTypedCustomerEnsure_Handler_ConfigEmail verifies Config.Email takes
// precedence over Input.Email (BMW templates email under config:).
func TestTypedCustomerEnsure_Handler_ConfigEmail(t *testing.T) {
	setupMockModule(t, "typed-cust-cfgemail")
	result, err := handleTypedCustomerEnsure(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentCustomerEnsureConfig, *paymentsv1.PaymentCustomerEnsureInput]{
		Config: &paymentsv1.PaymentCustomerEnsureConfig{
			Module: "typed-cust-cfgemail",
			Email:  "config@example.com",
			Name:   "Config Customer",
		},
		Input: &paymentsv1.PaymentCustomerEnsureInput{}, // empty: Config must win
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.CustomerId == "" {
		t.Error("expected customer_id from Config-driven ensure")
	}
	if result.Output.Email != "config@example.com" {
		t.Errorf("expected email from Config, got %s", result.Output.Email)
	}
}

// TestTypedSubscriptionCreate_Handler_ConfigInlinePricing verifies Config
// inline pricing (amount + currency + interval) drives the subscription when
// price_id is empty (BMW pattern).
func TestTypedSubscriptionCreate_Handler_ConfigInlinePricing(t *testing.T) {
	setupMockModule(t, "typed-subcreate-inline")
	result, err := handleTypedSubscriptionCreate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentSubscriptionCreateConfig, *paymentsv1.PaymentSubscriptionCreateInput]{
		Config: &paymentsv1.PaymentSubscriptionCreateConfig{
			Module:     "typed-subcreate-inline",
			CustomerId: "cus_inline",
			Amount:     "1000", // BMW renders "{{ index .steps \"calc_fees\" \"total_charge\" }}"
			Currency:   "usd",
			Interval:   "month",
		},
		Input: &paymentsv1.PaymentSubscriptionCreateInput{}, // empty: Config drives
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.SubscriptionId == "" {
		t.Error("expected subscription_id from inline-pricing path")
	}
}

// TestTypedSubscriptionCreate_Handler_ConfigPriceID verifies Config.PriceId
// path: when set, inline-pricing fields are not required.
func TestTypedSubscriptionCreate_Handler_ConfigPriceID(t *testing.T) {
	setupMockModule(t, "typed-subcreate-priceid")
	result, err := handleTypedSubscriptionCreate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentSubscriptionCreateConfig, *paymentsv1.PaymentSubscriptionCreateInput]{
		Config: &paymentsv1.PaymentSubscriptionCreateConfig{
			Module:     "typed-subcreate-priceid",
			CustomerId: "cus_pid",
			PriceId:    "price_test_123",
		},
		Input: &paymentsv1.PaymentSubscriptionCreateInput{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.SubscriptionId == "" {
		t.Error("expected subscription_id from PriceId path")
	}
}

// TestTypedSubscriptionCreate_Handler_MissingCustomerID verifies clean error
// when no customer_id is supplied via either Config or Input.
func TestTypedSubscriptionCreate_Handler_MissingCustomerID(t *testing.T) {
	setupMockModule(t, "typed-subcreate-nocust")
	result, err := handleTypedSubscriptionCreate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentSubscriptionCreateConfig, *paymentsv1.PaymentSubscriptionCreateInput]{
		Config: &paymentsv1.PaymentSubscriptionCreateConfig{Module: "typed-subcreate-nocust"},
		Input:  &paymentsv1.PaymentSubscriptionCreateInput{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error == "" {
		t.Error("expected error for missing customer_id")
	}
}

// TestTypedSubscriptionCreate_Handler_MissingPriceAndInline verifies clean
// error when customer_id is set but neither price_id nor inline-pricing.
func TestTypedSubscriptionCreate_Handler_MissingPriceAndInline(t *testing.T) {
	setupMockModule(t, "typed-subcreate-nopath")
	result, err := handleTypedSubscriptionCreate(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentSubscriptionCreateConfig, *paymentsv1.PaymentSubscriptionCreateInput]{
		Config: &paymentsv1.PaymentSubscriptionCreateConfig{
			Module:     "typed-subcreate-nopath",
			CustomerId: "cus_x",
		},
		Input: &paymentsv1.PaymentSubscriptionCreateInput{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error == "" {
		t.Error("expected error when neither price_id nor inline-pricing supplied")
	}
}

// TestTypedWebhookEndpointEnsure_Handler_ConfigURLAndEvents verifies Config
// url + events + mode take precedence over Input (BMW operator-pipeline pattern
// where the entire request is supplied via config:).
func TestTypedWebhookEndpointEnsure_Handler_ConfigURLAndEvents(t *testing.T) {
	setupMockModule(t, "typed-wh-cfgall")
	result, err := handleTypedWebhookEndpointEnsure(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentWebhookEndpointEnsureConfig, *paymentsv1.PaymentWebhookEndpointEnsureInput]{
		Config: &paymentsv1.PaymentWebhookEndpointEnsureConfig{
			Module:      "typed-wh-cfgall",
			Url:         "https://example.test/webhook",
			Events:      []string{"payment_intent.succeeded", "issuing_authorization.request"},
			Description: "config-driven webhook",
			Mode:        "ensure",
		},
		Input: &paymentsv1.PaymentWebhookEndpointEnsureInput{}, // empty: Config must win
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error != "" {
		t.Errorf("unexpected error: %s", result.Output.Error)
	}
	if result.Output.EndpointId == "" {
		t.Error("expected endpoint_id from Config-driven ensure")
	}
}

// TestTypedWebhookEndpointEnsure_Handler_MissingURL_ConfigOnly verifies clean
// error when neither Config.Url nor Input.Url is set.
func TestTypedWebhookEndpointEnsure_Handler_MissingURL_ConfigOnly(t *testing.T) {
	setupMockModule(t, "typed-wh-nourl")
	result, err := handleTypedWebhookEndpointEnsure(context.Background(), sdk.TypedStepRequest[*paymentsv1.PaymentWebhookEndpointEnsureConfig, *paymentsv1.PaymentWebhookEndpointEnsureInput]{
		Config: &paymentsv1.PaymentWebhookEndpointEnsureConfig{
			Module: "typed-wh-nourl",
			Events: []string{"payment_intent.succeeded"},
		},
		Input: &paymentsv1.PaymentWebhookEndpointEnsureInput{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.Error == "" {
		t.Error("expected error for missing url")
	}
	if !result.StopPipeline {
		t.Error("expected StopPipeline=true for missing url")
	}
}
