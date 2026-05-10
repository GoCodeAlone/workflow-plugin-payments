# workflow-plugin-payments

Multi-provider payment processing for the [GoCodeAlone/workflow](https://github.com/GoCodeAlone/workflow) engine. Stripe and PayPal today; designed so additional providers slot in behind the same `payments.provider` module type.

## Capabilities

| Surface | Purpose |
|---|---|
| `payments.provider` module | Provider-backed runtime (Stripe, PayPal). Configures secret keys + defaults. |
| 17 `step.payment_*` step types | Charge / capture / refund / customer / subscription / checkout / portal / webhook verify + ensure / transfer / payout / invoice / payment-method ops. |
| `wfctl payments` CLI | Plugin-CLI commands operators run without standing the engine up — e.g. one-shot webhook endpoint provisioning. |

## Install

The plugin ships pre-built binaries via [GoReleaser](https://github.com/GoCodeAlone/workflow-plugin-payments/releases) and is registered in [GoCodeAlone/workflow-registry](https://github.com/GoCodeAlone/workflow-registry/tree/main/plugins/payments). Install into a workflow project's plugin directory:

```sh
wfctl plugin install workflow-plugin-payments
```

This downloads the latest release tarball, extracts to `data/plugins/payments/`, and writes the installed manifest. The binary registers as `data/plugins/payments/payments` and is discovered by:

- the engine via `payments.provider` module instantiation
- `wfctl <subcommand>` via the plugin-CLI registry (requires wfctl ≥ v0.27.5)

To pin a specific version, use `<name>@<tag>`:

```sh
wfctl plugin install workflow-plugin-payments@v0.3.1
```

## Configure

Add a `payments.provider` module to your `app.yaml`. Stripe example:

```yaml
modules:
  - name: stripe                  # module name pipelines reference via step config
    type: payments.provider
    config:
      provider: stripe
      secretKey: '{{config "stripe_secret_key"}}'
      webhookSecret: '{{config "stripe_webhook_secret"}}'
```

Source the secrets via your `config.provider` schema (env vars, secrets module, etc.). The plugin reads `secretKey` lazily so an empty value at init returns `payments.ErrStripeKeyMissing` from API calls rather than failing module load — useful for dev/test environments where the key is unset.

PayPal works identically; replace `provider: stripe` with `provider: paypal` and configure `clientID` + `clientSecret`.

See [`docs/SETUP.md`](docs/SETUP.md) for the full module schema, multi-tenant configurations, and provider-specific knobs.

## Step types

```
step.payment_charge                    step.payment_subscription_create
step.payment_capture                   step.payment_subscription_update
step.payment_refund                    step.payment_subscription_cancel
step.payment_customer_ensure           step.payment_checkout_create
step.payment_fee_calculate             step.payment_portal_create
step.payment_webhook_verify            step.payment_webhook_endpoint_ensure
step.payment_transfer                  step.payment_method_attach
step.payment_payout                    step.payment_method_list
step.payment_invoice_list
```

Every step takes a `module` config field selecting the `payments.provider` module instance to dispatch to (defaults to `payments`). Outputs are scalar string/bool/number across the gRPC structpb boundary so they round-trip cleanly through pipeline state.

## CLI commands

```
wfctl payments webhook ensure --provider stripe \
  --url https://<your-host>/api/v1/webhooks/stripe/issuing \
  --events <comma-separated-event-list> \
  [--description "..."] \
  [--mode ensure|replace] \
  [--api-key-env STRIPE_SECRET_KEY]
```

Idempotently provisions a webhook endpoint and returns JSON with `endpoint_id`, `created`, `events_drift`, and `signing_secret` (populated only on fresh-create). See [`docs/WEBHOOK-PROVISIONING.md`](docs/WEBHOOK-PROVISIONING.md) for the full operator playbook including the GitHub Actions persistence pattern.

The CLI surface is registered via [the plugin-CLI registry](https://github.com/GoCodeAlone/workflow/blob/main/cmd/wfctl/plugin_cli_commands.go) — the plugin manifest declares `capabilities.cliCommands: [{name: "payments"}]` and wfctl dispatches `wfctl payments …` to the plugin binary's `RunCLI`.

## Versions

| | Version |
|---|---|
| `payments.PaymentProvider` Go interface | v0.3.x — adds `WebhookEndpointEnsure` |
| stripe-go | v82 (current) |
| Minimum workflow engine | v0.3.12 (declared in `plugin.json:minEngineVersion`) |
| Minimum wfctl for plugin-CLI dispatch | **v0.27.5** (4-fix lineage: `#591`/`#595`/`#612`/`#613`) |

The wfctl floor is real: earlier versions silently fail to dispatch `wfctl payments …` because of bugs in BuildCLIRegistry's binary-path resolution, the post-install plugin.json's stripped `cliCommands`, etc. Pin `setup-wfctl@v1` to `version: v0.27.5` or later.

## Development

```sh
go build -o bin/workflow-plugin-payments ./cmd/workflow-plugin-payments
GOWORK=off go test ./... -count=1
GOWORK=off go vet ./...
```

The plugin runs as a [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin) gRPC subprocess; the host workflow engine spawns it on demand. `cmd/workflow-plugin-payments/main.go` calls `sdk.ServePluginFull(plugin, cli, nil)` so the binary handles three modes:

- `--wfctl-cli <args>` → `CLIProvider.RunCLI` (operator commands)
- `--wfctl-hook <event>` → `HookHandler.HandleBuildHook` (none today)
- default → standard gRPC server for the engine

## License

Apache-2.0
