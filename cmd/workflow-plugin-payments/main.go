// Command workflow-plugin-payments is a workflow engine external plugin that
// provides multi-provider payment processing (Stripe, PayPal).
//
// It runs as a subprocess in three modes (per sdk.ServePluginFull dispatch):
//   - --wfctl-cli  → CLIProvider handles operator subcommands like
//     `payments webhook ensure --url …` and exits with the returned code.
//   - --wfctl-hook → no hook handler today.
//   - default      → standard go-plugin gRPC server for the host workflow
//     engine to call PaymentProvider methods through the
//     payments.provider module.
package main

import (
	"github.com/GoCodeAlone/workflow-plugin-payments/internal"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

func main() {
	sdk.ServePluginFull(internal.NewPaymentsPlugin(), internal.NewCLIProvider(), nil)
}
