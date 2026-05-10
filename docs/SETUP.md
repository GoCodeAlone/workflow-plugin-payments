# Setup

End-to-end install and configuration walkthrough for `workflow-plugin-payments`.

## 1. Install the plugin

```sh
# In your workflow project directory (the one with app.yaml)
wfctl plugin install workflow-plugin-payments
```

`wfctl plugin install` reads from [workflow-registry](https://github.com/GoCodeAlone/workflow-registry/tree/main/plugins/payments), downloads the matching release binary from GitHub, and extracts it to `data/plugins/payments/`. It also writes a `wfctl.yaml` entry + `.wfctl-lock.yaml` line so subsequent `wfctl plugin install` calls (e.g. in CI) are deterministic.

Pin a specific version:

```sh
wfctl plugin install workflow-plugin-payments@v0.3.1
```

In CI, install is usually delegated to a composite action that reads the lockfile and bulk-installs all declared plugins:

```yaml
- uses: ./.github/actions/setup-plugins
  with:
    app-id:           ${{ vars.PLUGIN_INSTALLER_APP_ID }}
    app-private-key:  ${{ secrets.PLUGIN_INSTALLER_APP_PRIVATE_KEY }}
    owner:            ${{ github.repository_owner }}
```

## 2. Declare the provider module

Add a `payments.provider` module to your `app.yaml`. The module name (`stripe` below) is what pipeline steps reference via the step's `module:` config field.

### Stripe

```yaml
modules:
  - name: stripe
    type: payments.provider
    config:
      provider: stripe
      secretKey: '{{config "stripe_secret_key"}}'        # sk_live_… / sk_test_…
      webhookSecret: '{{config "stripe_webhook_secret"}}' # whsec_… (charges/disputes endpoint)
      defaultCurrency: usd                                # optional, defaults to "usd"
```

`secretKey` is intentionally optional at module init: an empty value loads successfully and only fails at API call sites with `payments.ErrStripeKeyMissing`. Use this when the same `app.yaml` runs in environments where Stripe is intentionally disabled (local dev, ephemeral preview deploys).

### PayPal

```yaml
modules:
  - name: paypal
    type: payments.provider
    config:
      provider: paypal
      clientID: '{{config "paypal_client_id"}}'
      clientSecret: '{{config "paypal_client_secret"}}'
      webhookID: '{{config "paypal_webhook_id"}}'  # required for VerifyWebhook
      mode: live                                    # "live" or "sandbox"
```

### Multiple providers

Run multiple `payments.provider` modules side by side under different names:

```yaml
modules:
  - name: stripe
    type: payments.provider
    config: {provider: stripe, secretKey: '{{config "stripe_secret_key"}}'}
  - name: paypal
    type: payments.provider
    config: {provider: paypal, clientID: '{{config "paypal_client_id"}}', clientSecret: '{{config "paypal_client_secret"}}'}
```

Pipelines select between them via the step's `module: stripe` or `module: paypal`:

```yaml
- name: charge_with_stripe
  type: step.payment_charge
  config:
    module: stripe
    amount: 1000
    currency: usd
    customer_id: '{{.steps.upsert.customer_id}}'
```

## 3. Source the secrets

The `'{{config "stripe_secret_key"}}'` expressions above resolve via the engine's `config.provider` module. Declare each key in the schema:

```yaml
modules:
  - name: config-provider
    type: config.provider
    config:
      sources:
        - type: defaults
        - type: env
      schema:
        stripe_secret_key:
          env: STRIPE_SECRET_KEY
          required: false
          sensitive: true
          desc: "Stripe API secret key"
        stripe_webhook_secret:
          env: STRIPE_WEBHOOK_SECRET
          required: false
          sensitive: true
          desc: "Stripe webhook signing secret (charges/disputes endpoint)"
```

In production, source from a secrets-management plugin (`workflow-plugin-secrets`, AWS SecretsManager, GCP Secret Manager, Vault, etc.) instead of `type: env`. The provider reads the resolved value at module init.

## 4. Verify install

```sh
wfctl validate app.yaml
```

If the validator reports `unknown module type "payments.provider"`, the plugin install didn't land — check `data/plugins/payments/plugin.json` exists and `wfctl plugin list` shows the plugin. The plugin's binary is at `data/plugins/payments/payments` (renamed from the tarball's `workflow-plugin-payments` by `wfctl plugin install`'s `ensurePluginBinary` step).

## Gotchas

- **wfctl version floor.** Plugin-CLI commands (`wfctl payments …`) require **wfctl v0.27.5+**. Earlier versions silently fail with `unknown command: payments` or fork/exec errors due to BuildCLIRegistry bugs that landed in `#591`/`#595`/`#612`/`#613`. Pin `setup-wfctl@v1` accordingly in CI.
- **Plugin install cache.** Composite actions usually cache `data/plugins/` keyed by `hashFiles('wfctl.yaml', '.wfctl-lock.yaml')`. After upgrading the plugin, invalidate the cache (touch wfctl.yaml whitespace, or delete via `gh api -X DELETE /repos/<owner>/<repo>/actions/caches/<id>`); otherwise `wfctl plugin install` no-ops on a stale plugin and the new version's `cliCommands` aren't picked up.
- **Cross-environment isolation.** Stripe `pk_test_*`/`pk_live_*` and `sk_test_*`/`sk_live_*` MUST be segregated per environment. Use environment-scoped GitHub secrets (or your secrets backend's equivalent) — never reuse a live secret across environments.
