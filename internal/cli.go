package internal

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

// CLIProvider implements sdk.CLIProvider for the payments plugin's
// "payments" top-level wfctl command. It dispatches the
// `payments webhook ensure ...` subcommand to the configured provider's
// WebhookEndpointEnsure method.
//
// Args layout (after wfctl strips the binary path + --wfctl-cli flag):
//
//	["payments", "webhook", "ensure", "--provider", "stripe", "--url", …]
//
// The CLI is invoked once per operator action and exits the process; it does
// NOT need the long-lived gRPC plugin server.
type CLIProvider struct{}

// NewCLIProvider returns the payments-plugin CLI dispatcher.
func NewCLIProvider() *CLIProvider {
	return &CLIProvider{}
}

// RunCLI is the sdk.CLIProvider entrypoint. Returns the process exit code.
func (c *CLIProvider) RunCLI(args []string) int {
	return runCLI(args, os.Stdout, os.Stderr)
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] != "payments" {
		fmt.Fprintln(stderr, "usage: wfctl payments <subcommand> [options]")
		fmt.Fprintln(stderr, "subcommands:")
		fmt.Fprintln(stderr, "  webhook ensure --url <URL> --events <CSV> [--description <S>] [--mode ensure|replace] [--provider stripe|paypal]")
		return 2
	}
	switch {
	case args[1] == "webhook" && len(args) >= 3 && args[2] == "ensure":
		return runWebhookEnsure(args[3:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %s\n", strings.Join(args[1:], " "))
		return 2
	}
}

func runWebhookEnsure(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("payments webhook ensure", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", "", "https URL the provider will POST events to (required)")
	eventsCSV := fs.String("events", "", "comma-separated event names (required)")
	description := fs.String("description", "", "optional human-readable description")
	mode := fs.String("mode", "ensure", `"ensure" (idempotent) or "replace" (rotates signing secret)`)
	provider := fs.String("provider", "stripe", "payment provider name (stripe|paypal)")
	apiKeyEnv := fs.String("api-key-env", "", "env var holding the provider API key (default: STRIPE_SECRET_KEY for stripe)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *url == "" {
		fmt.Fprintln(stderr, "error: --url is required")
		fs.Usage()
		return 2
	}
	if *eventsCSV == "" {
		fmt.Fprintln(stderr, "error: --events is required (comma-separated)")
		fs.Usage()
		return 2
	}
	events := splitEvents(*eventsCSV)
	if len(events) == 0 {
		fmt.Fprintln(stderr, "error: --events parsed to empty list")
		return 2
	}

	envName := *apiKeyEnv
	if envName == "" {
		switch *provider {
		case "stripe":
			envName = "STRIPE_SECRET_KEY"
		case "paypal":
			envName = "PAYPAL_CLIENT_SECRET"
		default:
			fmt.Fprintf(stderr, "error: unknown provider %q (expected stripe|paypal)\n", *provider)
			return 2
		}
	}
	apiKey := os.Getenv(envName)
	if apiKey == "" {
		fmt.Fprintf(stderr, "error: %s env var is empty (set it before running, or pass --api-key-env)\n", envName)
		return 2
	}

	prov, err := buildCLIProvider(*provider, apiKey)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	out, err := prov.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL:         *url,
		Events:      events,
		Description: *description,
		Mode:        *mode,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"endpoint_id":    out.EndpointID,
		"created":        out.Created,
		"events_drift":   out.EventsDrift,
		"signing_secret": out.SigningSecret,
	}); err != nil {
		fmt.Fprintf(stderr, "error: encode result: %v\n", err)
		return 1
	}
	return 0
}

// buildCLIProvider builds a one-shot provider instance from a single API key.
// CLI invocations don't have access to the host workflow engine's
// payments.provider module registry, so we instantiate directly.
func buildCLIProvider(name, apiKey string) (payments.PaymentProvider, error) {
	switch name {
	case "stripe":
		return newStripeProviderFromKey(apiKey), nil
	case "paypal":
		return nil, fmt.Errorf("paypal CLI provider not implemented (see PaymentProvider.WebhookEndpointEnsure stub)")
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}

// newStripeProviderFromKey constructs a stripeProvider with just an API key —
// all other config (defaultCurrency, webhookSecret, backends) is irrelevant
// for the webhook-ensure code path.
func newStripeProviderFromKey(secretKey string) *stripeProvider {
	return &stripeProvider{secretKey: secretKey}
}

func splitEvents(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
