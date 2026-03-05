package internal

import (
	"context"
	"encoding/json"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// invoiceStep implements step.payment_invoice_list.
type invoiceStep struct {
	name       string
	moduleName string
}

func newInvoiceStep(name string, config map[string]any) (*invoiceStep, error) {
	return &invoiceStep{
		name:       name,
		moduleName: getModuleName(config),
	}, nil
}

func (s *invoiceStep) Execute(ctx context.Context, _ map[string]any, _ map[string]map[string]any, current map[string]any, _ map[string]any, config map[string]any) (*sdk.StepResult, error) {
	provider, ok := GetProvider(s.moduleName)
	if !ok {
		return &sdk.StepResult{Output: map[string]any{"error": "payment provider not found: " + s.moduleName}}, nil
	}

	invoices, err := provider.ListInvoices(ctx, payments.InvoiceListParams{
		CustomerID: resolveValue("customer_id", current, config),
		Limit:      resolveInt64("limit", current, config),
		Status:     resolveValue("status", current, config),
	})
	if err != nil {
		return &sdk.StepResult{Output: map[string]any{"error": err.Error()}}, nil
	}

	data, _ := json.Marshal(invoices)
	return &sdk.StepResult{Output: map[string]any{
		"invoices": string(data),
		"count":    len(invoices),
	}}, nil
}
