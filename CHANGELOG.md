# Changelog

All notable changes to this plugin are documented here. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and version numbers
follow SemVer.

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
