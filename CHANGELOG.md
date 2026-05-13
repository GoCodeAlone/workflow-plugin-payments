# Changelog

All notable changes to this plugin are documented here. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and version numbers
follow SemVer.

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
