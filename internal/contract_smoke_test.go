package internal_test

import (
	"testing"

	internal "github.com/GoCodeAlone/workflow-plugin-payments/internal"
)

// TestPluginRegistersUnderV0_51_2 verifies the exported factory satisfies the
// strict-cutover SDK contract: NewPaymentsPlugin() must return a non-nil
// sdk.PluginProvider.  A nil return means the factory guard-clause fired,
// which would cause the gRPC plugin process to exit 1 at startup.
func TestPluginRegistersUnderV0_51_2(t *testing.T) {
	p := internal.NewPaymentsPlugin()
	if p == nil {
		t.Fatal("internal.NewPaymentsPlugin() returned nil — strict-cutover SDK contract not satisfied")
	}
}
