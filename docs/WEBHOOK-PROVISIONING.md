# Webhook endpoint provisioning

`wfctl payments webhook ensure` provisions a provider webhook endpoint idempotently. Reusable from operator shells, GitHub Actions, or any automation that has a workflow project on disk and a release-channel API key.

## Why a CLI

Webhook endpoint provisioning is one-shot and operator-presence. Routing it through the workflow engine + a `step.payment_webhook_endpoint_ensure` pipeline works (and is supported), but spinning the engine up just to make four Stripe API calls is overkill in CI. The CLI path skips the engine entirely — the plugin binary handles the request via its `RunCLI` entry, talks to the provider's REST API directly, and exits.

## Command

```
wfctl payments webhook ensure \
  --provider stripe \
  --url https://<your-host>/api/v1/webhooks/<endpoint-path> \
  --events <comma-separated-event-list> \
  [--description "..."] \
  [--mode ensure|replace] \
  [--api-key-env STRIPE_SECRET_KEY]
```

| Flag | Notes |
|---|---|
| `--provider` | `stripe` today. `--provider paypal` is rejected by the CLI with `paypal CLI provider not implemented (see PaymentProvider.WebhookEndpointEnsure stub)` — the engine-side `payments.PaymentProvider.WebhookEndpointEnsure` for PayPal returns `payments.ErrUnsupported`; the CLI fast-fails before reaching it. PayPal webhook-management integration is tracked as a follow-up. |
| `--url` | Full https URL. Provider does not validate the URL is reachable at create-time. |
| `--events` | Comma-separated provider event names. Sort + dedup + lowercase happen automatically; reorder/duplicate input does not register as drift. |
| `--description` | Optional human-readable label stored on the endpoint. |
| `--mode` | `ensure` (default, idempotent) or `replace` (deletes any existing endpoint at the same URL and creates fresh — **rotates the signing secret**). |
| `--api-key-env` | Override the default env var. Default: `STRIPE_SECRET_KEY` for stripe, `PAYPAL_CLIENT_SECRET` for paypal. |

The API key itself never crosses the command line — only the env var name does. The CLI reads the resolved value from the process environment.

## Output

Stdout receives a single JSON object on success:

```json
{
  "endpoint_id":    "we_…",
  "created":        true,
  "events_drift":   false,
  "signing_secret": "whsec_…"
}
```

| Field | Meaning |
|---|---|
| `endpoint_id` | Provider-assigned endpoint ID (`we_…` for Stripe). Always populated. |
| `created` | `true` only on the fresh-create branch. `false` for idempotent no-op AND for events-drift updates. |
| `events_drift` | `true` when an existing endpoint's event list was patched to match the requested set. |
| `signing_secret` | Populated **only when `created == true`**. Stripe returns the signing secret once at creation. On every other branch this is the empty string so downstream secret-write steps can short-circuit and avoid clobbering an already-stored secret with empty. |

Errors land on stderr with non-zero exit; the JSON output stream stays clean.

## Behaviour matrix

| Existing endpoint at URL? | Events match? | `--mode` | API operations | `created` | `signing_secret` |
|---|---|---|---|---|---|
| no | — | `ensure` | `POST /v1/webhook_endpoints` | `true` | populated |
| yes | yes | `ensure` | (none — list only) | `false` | `""` |
| yes | no | `ensure` | `POST /v1/webhook_endpoints/{id}` (update events) | `false` | `""` (drift, but secret unchanged) |
| yes | — | `replace` | `DELETE` + `POST` (rotates secret) | `true` | populated (new) |
| no | — | `replace` | `POST /v1/webhook_endpoints` | `true` | populated |

`replace` mode is destructive — any in-flight webhook deliveries to the old endpoint will fail signature verification once the new secret is in use. Use sparingly (rotation events, emergency revocation, etc.).

## GitHub Actions integration

The canonical pattern: a `workflow_dispatch`-triggered job that runs the CLI, masks the returned signing secret, and persists it to the target environment's secrets via `gh secret set`. Sketch:

```yaml
name: Provision Stripe Webhook

on:
  workflow_dispatch:
    inputs:
      environment: {description: "Target env", required: true, type: choice, options: [staging, prod]}
      mode:        {description: "ensure|replace", required: true, default: ensure, type: choice, options: [ensure, replace]}

permissions:
  contents: read

# Serialize per-environment so two operator triggers don't race each other.
# Critical under mode=replace where a race rotates the signing secret twice
# and leaves the persisted value ambiguous.
concurrency:
  group: provision-webhook-${{ inputs.environment }}
  cancel-in-progress: false

jobs:
  provision:
    runs-on: ubuntu-latest
    environment: ${{ inputs.environment }}
    steps:
      - uses: actions/checkout@v4

      # wfctl >= v0.27.5 required for plugin-CLI dispatch; earlier versions
      # silently fail with `unknown command: payments`.
      - uses: GoCodeAlone/setup-wfctl@v1
        with:
          version: v0.27.5

      - uses: ./.github/actions/setup-plugins  # installs workflow-plugin-payments
        id: setup-plugins
        with:
          app-id:          ${{ vars.PLUGIN_INSTALLER_APP_ID }}
          app-private-key: ${{ secrets.PLUGIN_INSTALLER_APP_PRIVATE_KEY }}
          owner:           ${{ github.repository_owner }}

      - name: Provision endpoint
        id: provision
        env:
          WFCTL_PLUGIN_DIR:  ${{ github.workspace }}/data/plugins
          STRIPE_SECRET_KEY: ${{ secrets.STRIPE_SECRET_KEY }}
          PUBLIC_HOST:       ${{ vars.PUBLIC_HOST }}
          MODE:              ${{ inputs.mode }}
        run: |
          set -euo pipefail
          wfctl payments webhook ensure \
            --provider stripe \
            --url "https://${PUBLIC_HOST}/api/v1/webhooks/<endpoint-path>" \
            --events "<comma-separated-event-list>" \
            --description "Stripe webhook" \
            --mode "${MODE}" \
            > /tmp/result.json

          # Mask BEFORE echoing structured outputs so the value can't leak via subsequent step logs.
          SIGNING_SECRET=$(jq -r '.signing_secret // ""' /tmp/result.json)
          if [ -n "${SIGNING_SECRET}" ]; then
            echo "::add-mask::${SIGNING_SECRET}"
          fi
          {
            echo "endpoint_id=$(jq -r '.endpoint_id' /tmp/result.json)"
            echo "created=$(jq -r '.created' /tmp/result.json)"
            echo "events_drift=$(jq -r '.events_drift' /tmp/result.json)"
            echo "signing_secret<<EOF"
            echo "${SIGNING_SECRET}"
            echo "EOF"
          } >> "$GITHUB_OUTPUT"

      - name: Persist signing secret (only on fresh-create)
        if: steps.provision.outputs.created == 'true'
        env:
          GH_TOKEN:       ${{ steps.setup-plugins.outputs.app-token }}
          SIGNING_SECRET: ${{ steps.provision.outputs.signing_secret }}
        run: |
          set -euo pipefail
          if [ -z "${SIGNING_SECRET}" ]; then
            echo "::error::created=true but signing_secret is empty — refusing to write" >&2
            exit 1
          fi
          printf '%s' "${SIGNING_SECRET}" | gh secret set STRIPE_WEBHOOK_SIGNING_SECRET \
            --repo "${{ github.repository }}" \
            --env "${{ inputs.environment }}"
```

### Secret-name + scope convention

Choose the persisted name and scope based on what the deploy job that consumes it can read:

| Deploy job has `environment:` scope? | Persist target |
|---|---|
| Yes (e.g. `environment: prod`) | `gh secret set <NAME> --env <env-name>` (env-scoped secret) |
| No (no `environment:` on the job) | `gh secret set <NAME> --repo …` (repo-level secret) |

Match what your existing deploy reads. A common convention when the staging deploy is repo-level and prod is env-scoped: write `<NAME>_STAGING` at repo level, `<NAME>` at the prod env scope.

### Idempotency guarantees

Re-running the same workflow with `mode=ensure` is safe — the second run reports `created=false` and the persistence step's `if: created == 'true'` guard skips. Useful properties this gives you:

- Operators can re-run on every deploy without rotating secrets.
- A failed mid-run (e.g. a network blip after the Stripe call but before secret-write) is recoverable — re-run resumes idempotently as long as the signing secret was captured by Stripe's API response.

The only case where re-running needs care: if the original run successfully created the endpoint but FAILED to persist the signing secret (e.g. `gh secret set` 403). Stripe will not re-emit the secret on subsequent `--mode ensure` calls. Recovery is `--mode replace` to delete + recreate, accepting the consequences (any in-flight deliveries to the old endpoint will fail).

## Sandbox provisioning

Both Stripe and PayPal expose sandboxes. Use a sandbox API key (`sk_test_…` for Stripe) and an https-reachable URL pointing at your staging deployment. Stripe does not validate URL reachability at create-time, so a not-yet-deployed URL is fine — but no events will deliver until the endpoint actually serves the path.

Per-environment endpoint isolation is enforced by the provider: Stripe issues distinct signing secrets for staging vs production endpoints, even when registered against the same account. Use distinct API keys (`sk_test_…` for staging, `sk_live_…` for prod) to keep test and live data segregated.

## Provider notes

### Stripe

Implementation: `internal/provider_stripe.go`. Calls `webhookendpoint.{New,Update,Del,List}` from `github.com/stripe/stripe-go/v82`. Per-call client (`webhookendpoint.Client{B: ..., Key: secretKey}`) so concurrent module instances don't race the package-level `stripe.Key` global.

### PayPal

The CLI dispatch path for PayPal is unimplemented — `wfctl payments webhook ensure --provider paypal` exits with `paypal CLI provider not implemented (see PaymentProvider.WebhookEndpointEnsure stub)`. The engine-side `PaymentProvider.WebhookEndpointEnsure` for PayPal returns `payments.ErrUnsupported`; the CLI never reaches it because the provider-selection switch fast-fails first. The PayPal webhook-management API (`/v1/notifications/webhooks`) has different idempotency semantics from Stripe's `/v1/webhook_endpoints`; integration tracked as a follow-up. PayPal webhook **verification** (`step.payment_webhook_verify`) is fully supported.

## Troubleshooting

**`unknown command: payments`**

wfctl version is older than v0.27.5. Plugin-CLI dispatch went through four upstream fix iterations; the dispatch path only works end-to-end at v0.27.5+. Bump `setup-wfctl@v1` to `version: v0.27.5` (or later) in the workflow.

**`fork/exec data/plugins/payments/workflow-plugin-payments: no such file or directory`**

wfctl version is between v0.27.2 and v0.27.4. The binary post-install lives at `data/plugins/payments/payments` (renamed by `ensurePluginBinary`); v0.27.4 and earlier looked for it at `…/payments/workflow-plugin-payments`. Fixed in v0.27.5.

**`unknown command: payments` on a fresh wfctl v0.27.4+**

Plugin-install cache may have a stale `plugin.json` from before the registry manifest declared `cliCommands`. Invalidate the GitHub Actions cache for `plugins-…` keys (delete via `gh api -X DELETE /repos/<owner>/<repo>/actions/caches/<id>`) or change the cache key (e.g. add a no-op comment to `wfctl.yaml`, which most setup-plugins composites hash).

**`api_key is required …`**

The CLI couldn't read the API key from the configured env var. Check `STRIPE_SECRET_KEY` (or your `--api-key-env` override) is exported in the calling process. The CLI never accepts the key on argv to keep it out of shell history.

**Plugin install cache hits a stale tarball after registry update**

`wfctl plugin install` skips re-extraction when the destination plugin is already at the requested version. If the registry manifest's metadata changed (e.g. added `cliCommands`) but the version tag didn't, the on-disk `plugin.json` won't refresh until the cache is invalidated. Either bump the plugin version (preferred) or force a fresh install via cache deletion.
