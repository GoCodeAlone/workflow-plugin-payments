// Package internal implements the workflow-plugin-payments plugin.
package internal

import (
	"fmt"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// Version is set at build time via -ldflags
// "-X github.com/GoCodeAlone/workflow-plugin-payments/internal.Version=X.Y.Z"
var Version = "dev"

// paymentsPlugin implements sdk.PluginProvider, sdk.ModuleProvider, and sdk.StepProvider.
type paymentsPlugin struct{}

// NewPaymentsPlugin returns a new paymentsPlugin instance.
func NewPaymentsPlugin() sdk.PluginProvider {
	return &paymentsPlugin{}
}

// Manifest returns plugin metadata.
func (p *paymentsPlugin) Manifest() sdk.PluginManifest {
	return sdk.PluginManifest{
		Name:        "workflow-plugin-payments",
		Version:     Version,
		Author:      "GoCodeAlone",
		Description: "Multi-provider payment processing plugin (Stripe, PayPal)",
	}
}

// ModuleTypes returns the module type names this plugin provides.
func (p *paymentsPlugin) ModuleTypes() []string {
	return []string{"payments.provider"}
}

// CreateModule creates a module instance of the given type.
func (p *paymentsPlugin) CreateModule(typeName, name string, config map[string]any) (sdk.ModuleInstance, error) {
	switch typeName {
	case "payments.provider":
		m, err := newProviderModule(name, config)
		if err != nil {
			return nil, err
		}
		return m, nil
	default:
		return nil, fmt.Errorf("payments plugin: unknown module type %q", typeName)
	}
}

// StepTypes returns the step type names this plugin provides.
func (p *paymentsPlugin) StepTypes() []string {
	return []string{
		"step.payment_charge",
		"step.payment_capture",
		"step.payment_refund",
		"step.payment_fee_calculate",
		"step.payment_customer_ensure",
		"step.payment_subscription_create",
		"step.payment_subscription_update",
		"step.payment_subscription_cancel",
		"step.payment_checkout_create",
		"step.payment_portal_create",
		"step.payment_webhook_verify",
		"step.payment_transfer",
		"step.payment_payout",
		"step.payment_invoice_list",
		"step.payment_method_attach",
		"step.payment_method_list",
	}
}

// CreateStep creates a step instance of the given type.
func (p *paymentsPlugin) CreateStep(typeName, name string, config map[string]any) (sdk.StepInstance, error) {
	switch typeName {
	case "step.payment_charge":
		return newChargeStep(name, config)
	case "step.payment_capture":
		return newCaptureStep(name, config)
	case "step.payment_refund":
		return newRefundStep(name, config)
	case "step.payment_fee_calculate":
		return newFeeCalcStep(name, config)
	case "step.payment_customer_ensure":
		return newCustomerStep(name, config)
	case "step.payment_subscription_create":
		return newSubscriptionCreateStep(name, config)
	case "step.payment_subscription_update":
		return newSubscriptionUpdateStep(name, config)
	case "step.payment_subscription_cancel":
		return newSubscriptionCancelStep(name, config)
	case "step.payment_checkout_create":
		return newCheckoutStep(name, config)
	case "step.payment_portal_create":
		return newPortalStep(name, config)
	case "step.payment_webhook_verify":
		return newWebhookStep(name, config)
	case "step.payment_transfer":
		return newTransferStep(name, config)
	case "step.payment_payout":
		return newPayoutStep(name, config)
	case "step.payment_invoice_list":
		return newInvoiceStep(name, config)
	case "step.payment_method_attach":
		return newPaymentMethodAttachStep(name, config)
	case "step.payment_method_list":
		return newPaymentMethodListStep(name, config)
	default:
		return nil, fmt.Errorf("payments plugin: unknown step type %q", typeName)
	}
}
