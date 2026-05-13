# Changelog

All notable changes to this plugin are documented here. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and version numbers
follow SemVer.

## [0.4.5] - 2026-05-13

### Fixed
- **Strict-proto Config-field gaps round 3** — comprehensive sweep of every
  `step.payment_*` callsite in BMW `app.yaml` (post-v0.4.4 smoke). Rounds 1
  and 2 each missed entire Config messages; this round adds them all and
  audits the full surface.
  - `PaymentFeeCalculateConfig.platform_fee_percent` — type change from
    `double → string` to match BMW template output
    (`'{{ ...platform_fee_percent | default "5.0" }}'`). Handler parses via
    `strconv.ParseFloat`, falls back to Input on empty/invalid. **Breaking
    for any caller that set PlatformFeePercent programmatically as float64
    — set it as the decimal string equivalent.**
  - `PaymentChargeConfig` — added `amount` (string), `currency`,
    `capture_method`, `description`, `customer_id`. BMW supplies all under
    `config:` (~`app.yaml` L3733-L3739). Handler parses amount via
    `strconv.ParseInt`. Config-when-set wins over Input.
  - `PaymentSubscriptionCreateConfig` — added `customer_id`, `price_id`,
    `amount` (string), `currency`, `interval`. Two pricing modes: existing
    `price_id`, or inline pricing via `amount + currency + interval`
    (BMW pattern at `app.yaml` L5847-L5853). Stripe provider extended to
    create inline `price_data` when `PriceID` is empty.
  - `PaymentCustomerEnsureConfig` — added `email`, `name`. BMW supplies
    `email` via Config at `app.yaml` L5842-L5845.
  - `PaymentWebhookEndpointEnsureConfig` — added `url`, `events` (repeated
    string), `mode`. The provision-stripe-issuing-webhook operator pipeline
    in BMW (`app.yaml` L14785-L14802) supplies the entire request via
    `config:`, so `url`/`events`/`mode` were dropped under STRICT_PROTO
    dispatch before this fix. Handlers prefer Config when non-empty.

### Added
- `parseConfigFloat64` helper mirrors the v0.4.4 `parseConfigInt64` /
  `parseConfigBool` pattern for string→float64 with empty/invalid → 0.
- Unit tests covering every new Config field across all five messages,
  Config-takes-precedence and Input-fallback paths, missing-required-field
  errors, and parse-helper edge cases.
- `example/compat-check.yaml` extended with a new `compat-customer-and-subscription`
  pipeline exercising every v0.4.5 Config field for the workflow-compat CI gate.

### BMW coverage table (every `step.payment_*` callsite in `app.yaml`)
| step type | YAML line | config fields supplied | now covered |
|---|---|---|---|
| `step.payment_fee_calculate` | L3299, L3718, L5835 | module, amount, currency, platform_fee_percent | YES |
| `step.payment_charge` | L3733 | module, amount, currency, capture_method, description | YES |
| `step.payment_capture` | L3805, L4927 | module, charge_id | YES (v0.4.3) |
| `step.payment_refund` | L5164, L8463 | module, charge_id, amount, reason | YES (v0.4.4) |
| `step.payment_customer_ensure` | L5842 | module, email | YES |
| `step.payment_subscription_create` | L5847 | module, customer_id, amount, currency, interval | YES |
| `step.payment_subscription_cancel` | L5908, L8522, L9508 | module, subscription_id, (camelCase cancelAtPeriodEnd at L5912 — BMW-side fix) | YES (v0.4.4) |
| `step.payment_portal_create` | L8571 | module | YES (no Config fields) |
| `step.payment_webhook_verify` | L7941 | module | YES (no Config fields) |
| `step.payment_webhook_endpoint_ensure` | L14792 | module, url, events, description | YES |

### Notes
- BMW line 5912 uses `cancelAtPeriodEnd` (camelCase). protojson maps proto
  `cancel_at_period_end` to lowerCamelCase `cancelAtPeriodEnd`, so this
  callsite actually decodes correctly under STRICT_PROTO. The v0.4.4 note
  to the contrary was incorrect — no BMW fix needed for this one.
- `step.payment_subscription_create` previously required `price_id`; v0.4.5
  introduces inline-pricing mode (`amount + currency + interval`) to support
  BMW's recurring-contribution flow where no provider Price object exists.

## [0.4.4] - 2026-05-13

### Fixed
- **Strict-proto Config-field gaps round 2** surfaced by BMW local smoke
  against workflow v0.51.5 (v0.4.3 missed these three).
  - `PaymentRefundConfig` — added `charge_id`, `amount` (string), `reason`.
    BMW templates `amount: "{{ .body.amount }}"` for partial refunds. Handler
    parses the string via `strconv.ParseInt`, falls back to Input on
    empty/invalid.
  - `PaymentSubscriptionCancelConfig` — added `subscription_id` and
    `cancel_at_period_end` (string). BMW templates both fields. Handler
    parses the bool via `strconv.ParseBool`, falls back to Input on
    empty/invalid.
  - `PaymentFeeCalculateConfig.amount` — type change from `int64` to
    `string` to match BMW template output. Handler parses to int64
    internally. **Breaking for any caller that set Amount programmatically
    as int64** — set it as the decimal string equivalent.

### Notes
- BMW callsites in `app.yaml` that still use the non-canonical camelCase
  forms (`subscriptionID`, `cancelAtPeriodEnd`, `customerID`) will continue
  to fail under STRICT_PROTO because protojson maps `subscription_id` to
  `subscriptionId` (lowerCamelCase), not `subscriptionID`. Fix is BMW-side.
- `step.payment_charge` and `step.payment_subscription_create` both have
  Config fields supplied by BMW (`amount`, `currency`, `capture_method`,
  `description`, `customer_id`, `interval`) that are not on the Config
  protos. Round 3 should add them. Out of scope for v0.4.4.

## [0.4.3] - 2026-05-13

### Fixed
- **Strict-proto Config-field gaps** surfaced by BMW local smoke against
  workflow v0.51.5. Under STRICT_PROTO dispatch, fields supplied in step
  `config:` were dropped because they did not exist on the Config proto
  messages. Added the missing fields and made the handlers prefer Config
  when set so templated YAML works end-to-end:
  - `PaymentFeeCalculateConfig` — added `amount`, `currency`,
    `platform_fee_percent`. Handler now uses Config first, falls back to
    Input.
  - `PaymentCaptureConfig` — added `charge_id`, `amount`. Handler now uses
    Config first, falls back to Input.
  - `PaymentWebhookEndpointEnsureConfig` — added `description`. Handler now
    uses Config first, falls back to Input.

### Added
- `.github/workflows/workflow-compat.yml` — daily + per-PR compat gate that
  builds the plugin against the latest GoCodeAlone/workflow release and runs
  `wfctl validate --plugin-dir <build-dir> --strict example/compat-check.yaml`.
  Fails the PR when an upstream change or a missing field would break a
  consumer config.
- `example/compat-check.yaml` — minimal config exercising every
  `step.payment_*` config field this plugin advertises.

## [0.4.2] - prior release

- ContractProvider implementation for STRICT_PROTO typed config dispatch
  (Bug 3 fix).
