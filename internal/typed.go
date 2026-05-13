package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	paymentsv1 "github.com/GoCodeAlone/workflow-plugin-payments/proto/payments/v1"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
	"google.golang.org/protobuf/types/known/anypb"
)

// parseConfigInt64 parses a string amount from a config-side proto field
// (string typed to absorb YAML template output) into int64. Returns 0 on
// empty or unparseable input — callers fall back to the Input value.
func parseConfigInt64(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parseConfigBool parses a string boolean from a config-side proto field
// (string typed to absorb YAML template output) into bool. Returns the
// supplied fallback when the value is empty or unparseable.
func parseConfigBool(s string, fallback bool) bool {
	if s == "" {
		return fallback
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return fallback
	}
	return b
}

// parseConfigFloat64 parses a string float from a config-side proto field
// (string typed to absorb YAML template output) into float64. Returns 0 on
// empty or unparseable input — callers fall back to the Input value.
func parseConfigFloat64(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// Compile-time interface checks for typed providers.
var (
	_ sdk.TypedModuleProvider = (*paymentsPlugin)(nil)
	_ sdk.TypedStepProvider   = (*paymentsPlugin)(nil)
)

// TypedModuleTypes returns the module type names this plugin provides via typed proto.
func (p *paymentsPlugin) TypedModuleTypes() []string {
	return []string{"payments.provider"}
}

// CreateTypedModule creates a module instance from a protobuf-typed config.
func (p *paymentsPlugin) CreateTypedModule(typeName, name string, config *anypb.Any) (sdk.ModuleInstance, error) {
	if typeName != "payments.provider" {
		return nil, fmt.Errorf("%w: module type %q", sdk.ErrTypedContractNotHandled, typeName)
	}
	cfg := &paymentsv1.ProviderConfig{}
	if config != nil {
		if err := config.UnmarshalTo(cfg); err != nil {
			return nil, fmt.Errorf("payments.provider %q: unpack typed config: %w", name, err)
		}
	}
	return newProviderModule(name, providerConfigProtoToMap(cfg))
}

// providerConfigProtoToMap converts a typed ProviderConfig proto to the map[string]any
// format expected by newProviderModule/newStripeProvider/newPayPalProvider.
func providerConfigProtoToMap(cfg *paymentsv1.ProviderConfig) map[string]any {
	m := map[string]any{}
	if cfg.Provider != "" {
		m["provider"] = cfg.Provider
	}
	if cfg.SecretKey != "" {
		m["secretKey"] = cfg.SecretKey
	}
	if cfg.WebhookSecret != "" {
		m["webhookSecret"] = cfg.WebhookSecret
	}
	if cfg.DefaultCurrency != "" {
		m["defaultCurrency"] = cfg.DefaultCurrency
	}
	if cfg.ClientId != "" {
		m["clientId"] = cfg.ClientId
	}
	if cfg.ClientSecret != "" {
		m["clientSecret"] = cfg.ClientSecret
	}
	if cfg.Environment != "" {
		m["environment"] = cfg.Environment
	}
	if cfg.WebhookId != "" {
		m["webhookId"] = cfg.WebhookId
	}
	return m
}

// TypedStepTypes returns the step type names this plugin provides via typed proto.
func (p *paymentsPlugin) TypedStepTypes() []string {
	return p.StepTypes()
}

// CreateTypedStep creates a typed step instance for the given type.
func (p *paymentsPlugin) CreateTypedStep(typeName, name string, config *anypb.Any) (sdk.StepInstance, error) {
	switch typeName {
	case "step.payment_charge":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentChargeConfig{},
			&paymentsv1.PaymentChargeInput{},
			handleTypedCharge,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_capture":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentCaptureConfig{},
			&paymentsv1.PaymentCaptureInput{},
			handleTypedCapture,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_refund":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentRefundConfig{},
			&paymentsv1.PaymentRefundInput{},
			handleTypedRefund,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_fee_calculate":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentFeeCalculateConfig{},
			&paymentsv1.PaymentFeeCalculateInput{},
			handleTypedFeeCalculate,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_customer_ensure":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentCustomerEnsureConfig{},
			&paymentsv1.PaymentCustomerEnsureInput{},
			handleTypedCustomerEnsure,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_subscription_create":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentSubscriptionCreateConfig{},
			&paymentsv1.PaymentSubscriptionCreateInput{},
			handleTypedSubscriptionCreate,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_subscription_update":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentSubscriptionUpdateConfig{},
			&paymentsv1.PaymentSubscriptionUpdateInput{},
			handleTypedSubscriptionUpdate,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_subscription_cancel":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentSubscriptionCancelConfig{},
			&paymentsv1.PaymentSubscriptionCancelInput{},
			handleTypedSubscriptionCancel,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_checkout_create":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentCheckoutCreateConfig{},
			&paymentsv1.PaymentCheckoutCreateInput{},
			handleTypedCheckoutCreate,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_portal_create":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentPortalCreateConfig{},
			&paymentsv1.PaymentPortalCreateInput{},
			handleTypedPortalCreate,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_webhook_verify":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentWebhookVerifyConfig{},
			&paymentsv1.PaymentWebhookVerifyInput{},
			handleTypedWebhookVerify,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_webhook_endpoint_ensure":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentWebhookEndpointEnsureConfig{},
			&paymentsv1.PaymentWebhookEndpointEnsureInput{},
			handleTypedWebhookEndpointEnsure,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_transfer":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentTransferConfig{},
			&paymentsv1.PaymentTransferInput{},
			handleTypedTransfer,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_payout":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentPayoutConfig{},
			&paymentsv1.PaymentPayoutInput{},
			handleTypedPayout,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_invoice_list":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentInvoiceListConfig{},
			&paymentsv1.PaymentInvoiceListInput{},
			handleTypedInvoiceList,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_method_attach":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentMethodAttachConfig{},
			&paymentsv1.PaymentMethodAttachInput{},
			handleTypedPaymentMethodAttach,
		).CreateTypedStep(typeName, name, config)
	case "step.payment_method_list":
		return sdk.NewTypedStepFactory(typeName,
			&paymentsv1.PaymentMethodListConfig{},
			&paymentsv1.PaymentMethodListInput{},
			handleTypedPaymentMethodList,
		).CreateTypedStep(typeName, name, config)
	default:
		return nil, fmt.Errorf("%w: step type %q", sdk.ErrTypedContractNotHandled, typeName)
	}
}

// typedModuleName extracts the module name from a typed step config.
// Falls back to "payments" if the module field is empty.
func typedModuleName(module string) string {
	if module != "" {
		return module
	}
	return "payments"
}

func handleTypedCharge(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentChargeConfig, *paymentsv1.PaymentChargeInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentChargeOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentChargeOutput]{
			Output: &paymentsv1.PaymentChargeOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	// Config takes precedence over Input for templated YAML configs (BMW pattern).
	// Config string fields absorb YAML template output; parse to typed values internally.
	amount := parseConfigInt64(req.Config.Amount)
	if amount == 0 {
		amount = req.Input.Amount
	}
	currency := req.Config.Currency
	if currency == "" {
		currency = req.Input.Currency
	}
	customerID := req.Config.CustomerId
	if customerID == "" {
		customerID = req.Input.CustomerId
	}
	captureMethod := req.Config.CaptureMethod
	if captureMethod == "" {
		captureMethod = req.Input.CaptureMethod
	}
	description := req.Config.Description
	if description == "" {
		description = req.Input.Description
	}
	if amount == 0 {
		return &sdk.TypedStepResult[*paymentsv1.PaymentChargeOutput]{
			Output: &paymentsv1.PaymentChargeOutput{Error: "amount is required"},
		}, nil
	}
	charge, err := provider.CreateCharge(ctx, payments.ChargeParams{
		Amount:        amount,
		Currency:      currency,
		CustomerID:    customerID,
		CaptureMethod: captureMethod,
		Description:   description,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentChargeOutput]{
			Output: &paymentsv1.PaymentChargeOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentChargeOutput]{
		Output: &paymentsv1.PaymentChargeOutput{
			ChargeId:     charge.ID,
			ClientSecret: charge.ClientSecret,
			Status:       charge.Status,
			Amount:       charge.Amount,
		},
	}, nil
}

func handleTypedCapture(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentCaptureConfig, *paymentsv1.PaymentCaptureInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentCaptureOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentCaptureOutput]{
			Output: &paymentsv1.PaymentCaptureOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	// Config takes precedence over Input for templated YAML configs (BMW pattern).
	chargeID := req.Config.ChargeId
	if chargeID == "" {
		chargeID = req.Input.ChargeId
	}
	amount := req.Config.Amount
	if amount == 0 {
		amount = req.Input.Amount
	}
	if chargeID == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentCaptureOutput]{
			Output: &paymentsv1.PaymentCaptureOutput{Error: "charge_id is required"},
		}, nil
	}
	charge, err := provider.CaptureCharge(ctx, chargeID, amount)
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentCaptureOutput]{
			Output: &paymentsv1.PaymentCaptureOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentCaptureOutput]{
		Output: &paymentsv1.PaymentCaptureOutput{
			Status: charge.Status,
			Amount: charge.Amount,
		},
	}, nil
}

func handleTypedRefund(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentRefundConfig, *paymentsv1.PaymentRefundInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentRefundOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentRefundOutput]{
			Output: &paymentsv1.PaymentRefundOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	// Config takes precedence over Input for templated YAML configs (BMW pattern).
	// Config.Amount is `string` so YAML templates survive STRICT_PROTO decode.
	chargeID := req.Config.ChargeId
	if chargeID == "" {
		chargeID = req.Input.ChargeId
	}
	amount := parseConfigInt64(req.Config.Amount)
	if amount == 0 {
		amount = req.Input.Amount
	}
	reason := req.Config.Reason
	if reason == "" {
		reason = req.Input.Reason
	}
	if chargeID == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentRefundOutput]{
			Output: &paymentsv1.PaymentRefundOutput{Error: "charge_id is required"},
		}, nil
	}
	re, err := provider.RefundCharge(ctx, payments.RefundParams{
		ChargeID: chargeID,
		Amount:   amount,
		Reason:   reason,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentRefundOutput]{
			Output: &paymentsv1.PaymentRefundOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentRefundOutput]{
		Output: &paymentsv1.PaymentRefundOutput{
			RefundId: re.ID,
			Status:   re.Status,
		},
	}, nil
}

func handleTypedFeeCalculate(_ context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentFeeCalculateConfig, *paymentsv1.PaymentFeeCalculateInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentFeeCalculateOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentFeeCalculateOutput]{
			Output: &paymentsv1.PaymentFeeCalculateOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	// Config takes precedence over Input for templated YAML configs (BMW pattern).
	// Config.Amount and Config.PlatformFeePercent are `string` so YAML templates
	// ("{{ .body.amount }}", "{{ ...platform_fee_percent | default \"5.0\" }}")
	// survive STRICT_PROTO decode; parse to typed values for the provider call.
	amount := parseConfigInt64(req.Config.Amount)
	if amount == 0 {
		amount = req.Input.Amount
	}
	currency := req.Config.Currency
	if currency == "" {
		currency = req.Input.Currency
	}
	platformFeePercent := parseConfigFloat64(req.Config.PlatformFeePercent)
	if platformFeePercent == 0 {
		platformFeePercent = req.Input.PlatformFeePercent
	}
	if amount == 0 {
		return &sdk.TypedStepResult[*paymentsv1.PaymentFeeCalculateOutput]{
			Output: &paymentsv1.PaymentFeeCalculateOutput{Error: "amount is required"},
		}, nil
	}
	fees, err := provider.CalculateFees(amount, currency, platformFeePercent)
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentFeeCalculateOutput]{
			Output: &paymentsv1.PaymentFeeCalculateOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentFeeCalculateOutput]{
		Output: &paymentsv1.PaymentFeeCalculateOutput{
			ContributionAmount: fees.ContributionAmount,
			ProcessingFee:      fees.ProcessingFee,
			PlatformFee:        fees.PlatformFee,
			TotalCharge:        fees.TotalCharge,
		},
	}, nil
}

func handleTypedCustomerEnsure(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentCustomerEnsureConfig, *paymentsv1.PaymentCustomerEnsureInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentCustomerEnsureOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentCustomerEnsureOutput]{
			Output: &paymentsv1.PaymentCustomerEnsureOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	// Config takes precedence over Input for templated YAML configs (BMW pattern).
	email := req.Config.Email
	if email == "" {
		email = req.Input.Email
	}
	name := req.Config.Name
	if name == "" {
		name = req.Input.Name
	}
	if email == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentCustomerEnsureOutput]{
			Output: &paymentsv1.PaymentCustomerEnsureOutput{Error: "email is required"},
		}, nil
	}
	cust, err := provider.EnsureCustomer(ctx, payments.CustomerParams{
		Email: email,
		Name:  name,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentCustomerEnsureOutput]{
			Output: &paymentsv1.PaymentCustomerEnsureOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentCustomerEnsureOutput]{
		Output: &paymentsv1.PaymentCustomerEnsureOutput{
			CustomerId: cust.ID,
			Email:      cust.Email,
			Name:       cust.Name,
		},
	}, nil
}

func handleTypedSubscriptionCreate(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentSubscriptionCreateConfig, *paymentsv1.PaymentSubscriptionCreateInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCreateOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCreateOutput]{
			Output: &paymentsv1.PaymentSubscriptionCreateOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	// Config takes precedence over Input for templated YAML configs (BMW pattern).
	// Two pricing modes: PriceID (existing Price) or inline (amount+currency+interval).
	customerID := req.Config.CustomerId
	if customerID == "" {
		customerID = req.Input.CustomerId
	}
	priceID := req.Config.PriceId
	if priceID == "" {
		priceID = req.Input.PriceId
	}
	amount := parseConfigInt64(req.Config.Amount)
	currency := req.Config.Currency
	interval := req.Config.Interval
	if customerID == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCreateOutput]{
			Output: &paymentsv1.PaymentSubscriptionCreateOutput{Error: "customer_id is required"},
		}, nil
	}
	if priceID == "" && (amount == 0 || currency == "" || interval == "") {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCreateOutput]{
			Output: &paymentsv1.PaymentSubscriptionCreateOutput{Error: "price_id is required, or supply amount + currency + interval for inline pricing"},
		}, nil
	}
	sub, err := provider.CreateSubscription(ctx, payments.SubscriptionParams{
		CustomerID: customerID,
		PriceID:    priceID,
		Amount:     amount,
		Currency:   currency,
		Interval:   interval,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCreateOutput]{
			Output: &paymentsv1.PaymentSubscriptionCreateOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCreateOutput]{
		Output: &paymentsv1.PaymentSubscriptionCreateOutput{
			SubscriptionId: sub.ID,
			Status:         sub.Status,
		},
	}, nil
}

func handleTypedSubscriptionUpdate(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentSubscriptionUpdateConfig, *paymentsv1.PaymentSubscriptionUpdateInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionUpdateOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionUpdateOutput]{
			Output: &paymentsv1.PaymentSubscriptionUpdateOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	if req.Input.SubscriptionId == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionUpdateOutput]{
			Output: &paymentsv1.PaymentSubscriptionUpdateOutput{Error: "subscription_id is required"},
		}, nil
	}
	sub, err := provider.UpdateSubscription(ctx, req.Input.SubscriptionId, payments.SubscriptionUpdateParams{
		PriceID: req.Input.PriceId,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionUpdateOutput]{
			Output: &paymentsv1.PaymentSubscriptionUpdateOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionUpdateOutput]{
		Output: &paymentsv1.PaymentSubscriptionUpdateOutput{
			SubscriptionId: sub.ID,
			Status:         sub.Status,
		},
	}, nil
}

func handleTypedSubscriptionCancel(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentSubscriptionCancelConfig, *paymentsv1.PaymentSubscriptionCancelInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCancelOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCancelOutput]{
			Output: &paymentsv1.PaymentSubscriptionCancelOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	// Config takes precedence over Input for templated YAML configs (BMW pattern).
	// Config.CancelAtPeriodEnd is `string` so YAML templates ("true") survive
	// STRICT_PROTO decode; parse to bool, falling back to the Input value.
	subscriptionID := req.Config.SubscriptionId
	if subscriptionID == "" {
		subscriptionID = req.Input.SubscriptionId
	}
	cancelAtPeriodEnd := parseConfigBool(req.Config.CancelAtPeriodEnd, req.Input.CancelAtPeriodEnd)
	if subscriptionID == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCancelOutput]{
			Output: &paymentsv1.PaymentSubscriptionCancelOutput{Error: "subscription_id is required"},
		}, nil
	}
	sub, err := provider.CancelSubscription(ctx, subscriptionID, cancelAtPeriodEnd)
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCancelOutput]{
			Output: &paymentsv1.PaymentSubscriptionCancelOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentSubscriptionCancelOutput]{
		Output: &paymentsv1.PaymentSubscriptionCancelOutput{
			SubscriptionId: sub.ID,
			Status:         sub.Status,
		},
	}, nil
}

func handleTypedCheckoutCreate(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentCheckoutCreateConfig, *paymentsv1.PaymentCheckoutCreateInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentCheckoutCreateOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentCheckoutCreateOutput]{
			Output: &paymentsv1.PaymentCheckoutCreateOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	sess, err := provider.CreateCheckoutSession(ctx, payments.CheckoutParams{
		CustomerID: req.Input.CustomerId,
		PriceID:    req.Input.PriceId,
		SuccessURL: req.Input.SuccessUrl,
		CancelURL:  req.Input.CancelUrl,
		Mode:       req.Input.Mode,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentCheckoutCreateOutput]{
			Output: &paymentsv1.PaymentCheckoutCreateOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentCheckoutCreateOutput]{
		Output: &paymentsv1.PaymentCheckoutCreateOutput{
			Url:       sess.URL,
			SessionId: sess.ID,
		},
	}, nil
}

func handleTypedPortalCreate(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentPortalCreateConfig, *paymentsv1.PaymentPortalCreateInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentPortalCreateOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentPortalCreateOutput]{
			Output: &paymentsv1.PaymentPortalCreateOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	if req.Input.CustomerId == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentPortalCreateOutput]{
			Output: &paymentsv1.PaymentPortalCreateOutput{Error: "customer_id is required"},
		}, nil
	}
	sess, err := provider.CreatePortalSession(ctx, req.Input.CustomerId, req.Input.ReturnUrl)
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentPortalCreateOutput]{
			Output: &paymentsv1.PaymentPortalCreateOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentPortalCreateOutput]{
		Output: &paymentsv1.PaymentPortalCreateOutput{
			Url:       sess.URL,
			SessionId: sess.ID,
		},
	}, nil
}

func handleTypedWebhookVerify(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentWebhookVerifyConfig, *paymentsv1.PaymentWebhookVerifyInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentWebhookVerifyOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentWebhookVerifyOutput]{
			Output: &paymentsv1.PaymentWebhookVerifyOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	payload := []byte(req.Input.RequestBody)
	if len(payload) == 0 {
		return &sdk.TypedStepResult[*paymentsv1.PaymentWebhookVerifyOutput]{
			Output: &paymentsv1.PaymentWebhookVerifyOutput{Error: "missing webhook payload (request_body)"},
		}, nil
	}
	headers := http.Header{}
	if req.Input.StripeSignature != "" {
		headers.Set("Stripe-Signature", req.Input.StripeSignature)
	}
	if req.Input.PaypalTransmissionId != "" {
		headers.Set("Paypal-Transmission-Id", req.Input.PaypalTransmissionId)
	}
	if req.Input.PaypalTransmissionSig != "" {
		headers.Set("Paypal-Transmission-Sig", req.Input.PaypalTransmissionSig)
	}
	if req.Input.PaypalCertUrl != "" {
		headers.Set("Paypal-Cert-Url", req.Input.PaypalCertUrl)
	}
	if req.Input.PaypalAuthAlgo != "" {
		headers.Set("Paypal-Auth-Algo", req.Input.PaypalAuthAlgo)
	}
	if req.Input.PaypalTransmissionTime != "" {
		headers.Set("Paypal-Transmission-Time", req.Input.PaypalTransmissionTime)
	}
	event, err := provider.VerifyWebhook(ctx, payload, headers)
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentWebhookVerifyOutput]{
			Output: &paymentsv1.PaymentWebhookVerifyOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentWebhookVerifyOutput]{
		Output: &paymentsv1.PaymentWebhookVerifyOutput{
			EventType: event.Type,
			EventId:   event.ID,
		},
	}, nil
}

func handleTypedWebhookEndpointEnsure(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentWebhookEndpointEnsureConfig, *paymentsv1.PaymentWebhookEndpointEnsureInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentWebhookEndpointEnsureOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentWebhookEndpointEnsureOutput]{
			Output: &paymentsv1.PaymentWebhookEndpointEnsureOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	// Config takes precedence over Input for templated YAML configs (BMW pattern).
	url := req.Config.Url
	if url == "" {
		url = req.Input.Url
	}
	events := req.Config.Events
	if len(events) == 0 {
		events = req.Input.Events
	}
	description := req.Config.Description
	if description == "" {
		description = req.Input.Description
	}
	mode := req.Config.Mode
	if mode == "" {
		mode = req.Input.Mode
	}
	if url == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentWebhookEndpointEnsureOutput]{
			Output:       &paymentsv1.PaymentWebhookEndpointEnsureOutput{Error: "url is required"},
			StopPipeline: true,
		}, nil
	}
	if len(events) == 0 {
		return &sdk.TypedStepResult[*paymentsv1.PaymentWebhookEndpointEnsureOutput]{
			Output:       &paymentsv1.PaymentWebhookEndpointEnsureOutput{Error: "events list is required"},
			StopPipeline: true,
		}, nil
	}
	out, err := provider.WebhookEndpointEnsure(ctx, payments.WebhookEndpointEnsureParams{
		URL:         url,
		Events:      events,
		Description: description,
		Mode:        mode,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentWebhookEndpointEnsureOutput]{
			Output:       &paymentsv1.PaymentWebhookEndpointEnsureOutput{Error: fmt.Sprintf("webhook ensure: %v", err)},
			StopPipeline: true,
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentWebhookEndpointEnsureOutput]{
		Output: &paymentsv1.PaymentWebhookEndpointEnsureOutput{
			EndpointId:    out.EndpointID,
			Created:       out.Created,
			EventsDrift:   out.EventsDrift,
			SigningSecret: out.SigningSecret,
		},
	}, nil
}

func handleTypedTransfer(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentTransferConfig, *paymentsv1.PaymentTransferInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentTransferOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentTransferOutput]{
			Output: &paymentsv1.PaymentTransferOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	if req.Input.Amount == 0 {
		return &sdk.TypedStepResult[*paymentsv1.PaymentTransferOutput]{
			Output: &paymentsv1.PaymentTransferOutput{Error: "amount is required"},
		}, nil
	}
	if req.Input.DestinationAccountId == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentTransferOutput]{
			Output: &paymentsv1.PaymentTransferOutput{Error: "destination_account_id is required"},
		}, nil
	}
	t, err := provider.CreateTransfer(ctx, payments.TransferParams{
		Amount:               req.Input.Amount,
		Currency:             req.Input.Currency,
		DestinationAccountID: req.Input.DestinationAccountId,
		Description:          req.Input.Description,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentTransferOutput]{
			Output: &paymentsv1.PaymentTransferOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentTransferOutput]{
		Output: &paymentsv1.PaymentTransferOutput{
			TransferId: t.ID,
			Status:     t.Status,
		},
	}, nil
}

func handleTypedPayout(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentPayoutConfig, *paymentsv1.PaymentPayoutInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentPayoutOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentPayoutOutput]{
			Output: &paymentsv1.PaymentPayoutOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	if req.Input.Amount == 0 {
		return &sdk.TypedStepResult[*paymentsv1.PaymentPayoutOutput]{
			Output: &paymentsv1.PaymentPayoutOutput{Error: "amount is required"},
		}, nil
	}
	po, err := provider.CreatePayout(ctx, payments.PayoutParams{
		Amount:            req.Input.Amount,
		Currency:          req.Input.Currency,
		DestinationBankID: req.Input.DestinationBankId,
		Description:       req.Input.Description,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentPayoutOutput]{
			Output: &paymentsv1.PaymentPayoutOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentPayoutOutput]{
		Output: &paymentsv1.PaymentPayoutOutput{
			PayoutId: po.ID,
			Status:   po.Status,
		},
	}, nil
}

func handleTypedInvoiceList(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentInvoiceListConfig, *paymentsv1.PaymentInvoiceListInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentInvoiceListOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentInvoiceListOutput]{
			Output: &paymentsv1.PaymentInvoiceListOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	invoices, err := provider.ListInvoices(ctx, payments.InvoiceListParams{
		CustomerID: req.Input.CustomerId,
		Limit:      req.Input.Limit,
		Status:     req.Input.Status,
	})
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentInvoiceListOutput]{
			Output: &paymentsv1.PaymentInvoiceListOutput{Error: err.Error()},
		}, nil
	}
	data, _ := json.Marshal(invoices)
	return &sdk.TypedStepResult[*paymentsv1.PaymentInvoiceListOutput]{
		Output: &paymentsv1.PaymentInvoiceListOutput{
			Invoices: string(data),
			Count:    int64(len(invoices)),
		},
	}, nil
}

func handleTypedPaymentMethodAttach(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentMethodAttachConfig, *paymentsv1.PaymentMethodAttachInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentMethodAttachOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentMethodAttachOutput]{
			Output: &paymentsv1.PaymentMethodAttachOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	if req.Input.CustomerId == "" || req.Input.PaymentMethodId == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentMethodAttachOutput]{
			Output: &paymentsv1.PaymentMethodAttachOutput{Error: "customer_id and payment_method_id are required"},
		}, nil
	}
	pm, err := provider.AttachPaymentMethod(ctx, req.Input.CustomerId, req.Input.PaymentMethodId)
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentMethodAttachOutput]{
			Output: &paymentsv1.PaymentMethodAttachOutput{Error: err.Error()},
		}, nil
	}
	return &sdk.TypedStepResult[*paymentsv1.PaymentMethodAttachOutput]{
		Output: &paymentsv1.PaymentMethodAttachOutput{
			PaymentMethodId: pm.ID,
			Type:            pm.Type,
		},
	}, nil
}

func handleTypedPaymentMethodList(ctx context.Context, req sdk.TypedStepRequest[*paymentsv1.PaymentMethodListConfig, *paymentsv1.PaymentMethodListInput]) (*sdk.TypedStepResult[*paymentsv1.PaymentMethodListOutput], error) {
	moduleName := typedModuleName(req.Config.Module)
	provider, ok := GetProvider(moduleName)
	if !ok {
		return &sdk.TypedStepResult[*paymentsv1.PaymentMethodListOutput]{
			Output: &paymentsv1.PaymentMethodListOutput{Error: "payment provider not found: " + moduleName},
		}, nil
	}
	if req.Input.CustomerId == "" {
		return &sdk.TypedStepResult[*paymentsv1.PaymentMethodListOutput]{
			Output: &paymentsv1.PaymentMethodListOutput{Error: "customer_id is required"},
		}, nil
	}
	methods, err := provider.ListPaymentMethods(ctx, req.Input.CustomerId, req.Input.Type)
	if err != nil {
		return &sdk.TypedStepResult[*paymentsv1.PaymentMethodListOutput]{
			Output: &paymentsv1.PaymentMethodListOutput{Error: err.Error()},
		}, nil
	}
	data, _ := json.Marshal(methods)
	return &sdk.TypedStepResult[*paymentsv1.PaymentMethodListOutput]{
		Output: &paymentsv1.PaymentMethodListOutput{
			PaymentMethods: string(data),
			Count:          int64(len(methods)),
		},
	}, nil
}
