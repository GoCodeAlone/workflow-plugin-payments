# workflow-compute Settlement Handoff

`workflow-plugin-payments` owns live provider calls. `workflow-compute` owns the
audited settlement state machine, payout gates, normalized evidence, and
operator/dashboard visibility. Do not move Stripe API calls or webhook signing
secrets into wfcompute.

## Stablecoin Deposit Intent

Use `step.payment_stablecoin_deposit_intent` when a wfcompute orchestration
next action asks for `workflow-plugin-payments:step.payment_stablecoin_deposit_intent`.
The Workflow step should keep provider execution in the app/plugin layer:

```yaml
- type: step.payment_stablecoin_deposit_intent
  name: create_stablecoin_deposit
  config:
    module: stripe
    amount: 2500
    currency: usd
    stablecoin: usdc
    networks:
      - base
```

The output can be submitted directly to wfcompute without writing a JSON request
file:

```sh
compute settlement import-payment-intent \
  -server "$COMPUTE_SERVER" \
  -token "$COMPUTE_TOKEN" \
  -payment-intent-id "$LOCAL_PAYMENT_ID" \
  -org "$ORG_ID" \
  -product "$PRODUCT_ID" \
  -account "$ACCOUNT_ID" \
  -external-settlement-route-id "$ROUTE_ID" \
  -settlement-wallet-id "$WALLET_ID" \
  -settlement-conversion-quote-id "$QUOTE_ID" \
  -settlement-conversion-execution-id "$EXECUTION_ID" \
  -provider stripe \
  -client-secret-ref "$CLIENT_SECRET_REF" \
  -output-payment-intent-id "$PAYMENT_INTENT_ID" \
  -output-status "$PAYMENT_INTENT_STATUS" \
  -amount "$AMOUNT" \
  -currency "$CURRENCY" \
  -stablecoin "$STABLECOIN" \
  -deposit-address "network=base,address=$BASE_ADDRESS,stablecoin=usdc,token_contract_address=$BASE_USDC_CONTRACT"
```

For an existing orchestration, prefer the orchestration-scoped command so
wfcompute derives route, wallet, quote, and execution context:

```sh
compute settlement orchestration-payment-evidence \
  -server "$COMPUTE_SERVER" \
  -token "$COMPUTE_TOKEN" \
  -id "$ORCHESTRATION_ID" \
  -expected-status awaiting_payment_intent \
  -provider stripe \
  -client-secret-ref "$CLIENT_SECRET_REF" \
  -output-payment-intent-id "$PAYMENT_INTENT_ID" \
  -output-status "$PAYMENT_INTENT_STATUS" \
  -amount "$AMOUNT" \
  -currency "$CURRENCY" \
  -stablecoin "$STABLECOIN" \
  -deposit-address "network=base,address=$BASE_ADDRESS,stablecoin=usdc,token_contract_address=$BASE_USDC_CONTRACT"
```

`client_secret` from Stripe output is transient. Store it in a secrets manager,
pass only a `client-secret-ref` to wfcompute, and never log the raw value.
`output-payment-intent-id` should be the provider PaymentIntent id returned by
Stripe; use the same value later as the webhook `provider-ref`.

## Webhook Verify

Use `step.payment_webhook_verify` to validate provider webhook signatures before
calling wfcompute. The verified output maps to the webhook reconcile command:

```yaml
- type: step.payment_webhook_verify
  name: verify_stripe_webhook
  config:
    module: stripe
    payload: '{{ input.body }}'
    signature: '{{ input.headers.Stripe-Signature }}'
```

```sh
compute settlement webhook-reconcile-payment-intent \
  -server "$COMPUTE_SERVER" \
  -token "$COMPUTE_TOKEN" \
  -id "$SETTLEMENT_PAYMENT_INTENT_ID" \
  -event-id "$EVENT_ID" \
  -event-type "$EVENT_TYPE" \
  -provider-ref "$PAYMENT_INTENT_ID" \
  -transaction-ref "$TRANSACTION_REF"
```

After reconciliation, refresh the orchestration so wfcompute re-derives the
state from stored child evidence:

```sh
compute settlement refresh-orchestration \
  -server "$COMPUTE_SERVER" \
  -token "$COMPUTE_TOKEN" \
  -id "$ORCHESTRATION_ID" \
  -expected-status awaiting_payment_settlement
```

## Boundaries

- This plugin may create Stripe deposit-mode PaymentIntents and verify webhooks.
- wfcompute stores normalized evidence and rejects raw client/webhook secrets.
- BMW or another Workflow app owns user-facing wishlist/payment business logic.
- Provider workflows should read `orchestration-next-action` before each handoff
  and use `expected-status` to fail stale retries.
- Provider workflows should use idempotency keys around Stripe calls and
  wfcompute submissions so webhook redelivery does not duplicate evidence.
