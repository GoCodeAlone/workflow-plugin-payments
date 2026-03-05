// Command workflow-plugin-payments is a workflow engine external plugin that
// provides multi-provider payment processing (Stripe, PayPal).
// It runs as a subprocess and communicates with the host workflow engine via
// the go-plugin protocol.
package main

import (
	"github.com/GoCodeAlone/workflow-plugin-payments/internal"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

func main() {
	sdk.Serve(internal.NewPaymentsPlugin())
}
