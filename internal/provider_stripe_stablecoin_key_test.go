package internal

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	stripe "github.com/stripe/stripe-go/v85"
)

// stablecoinKeyBackend records the key supplied to the real SDK client while
// reusing the in-memory response backend; it cannot make a network request.
type stablecoinKeyBackend struct {
	stablecoinEvidenceBackend
	lastKey string
}

func (b *stablecoinKeyBackend) Call(method, path, key string, params stripe.ParamsContainer, out stripe.LastResponseSetter) error {
	b.lastKey = key
	return b.stablecoinEvidenceBackend.Call(method, path, key, params, out)
}

func TestStripeCreateStablecoinDepositIntentUsesOwnKeyWithoutChangingGlobal(t *testing.T) {
	backend := &stablecoinKeyBackend{stablecoinEvidenceBackend: stablecoinEvidenceBackend{
		response: stablecoinEvidenceResponse([]byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":"base_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":"base_token"}]}}}}}`)),
	}}
	previousBackend, previousKey := stripe.GetBackend(stripe.APIBackend), stripe.Key
	stripe.SetBackend(stripe.APIBackend, backend)
	t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, previousBackend); stripe.Key = previousKey })
	const globalKey = "sk_test_global_sentinel"
	stripe.Key = globalKey

	for _, key := range []string{"sk_test_provider_a", "sk_test_provider_b"} {
		provider := &stripeProvider{secretKey: key, defaultCurrency: "usd"}
		intent, err := provider.CreateStablecoinDepositIntent(context.Background(), payments.StablecoinDepositIntentParams{Amount: 5000, Networks: []string{"base"}})
		if err != nil {
			t.Fatal(err)
		}
		if intent == nil || intent.ID != "pi_evidence" {
			t.Fatal("expected valid backend-injected payment intent")
		}
		if backend.lastKey != key {
			t.Errorf("request did not use its own provider key")
		}
		if stripe.Key != globalKey {
			t.Errorf("stablecoin request changed global stripe.Key")
		}
	}
	if backend.calls != 2 {
		t.Fatalf("provider calls = %d, want exactly two", backend.calls)
	}
}

func TestStripeCreateStablecoinDepositIntentUsesConfiguredBackend(t *testing.T) {
	raw := []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":"base_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":"base_token"}]}}}}}`)
	configured := &stablecoinKeyBackend{stablecoinEvidenceBackend: stablecoinEvidenceBackend{response: stablecoinEvidenceResponse(raw)}}
	fallback := &stablecoinKeyBackend{stablecoinEvidenceBackend: stablecoinEvidenceBackend{response: stablecoinEvidenceResponse(raw)}}
	previousBackend, previousKey := stripe.GetBackend(stripe.APIBackend), stripe.Key
	stripe.SetBackend(stripe.APIBackend, fallback)
	t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, previousBackend); stripe.Key = previousKey })
	provider := &stripeProvider{secretKey: "sk_test_configured_provider", defaultCurrency: "usd", backends: stripe.Backends{API: configured}}
	intent, err := provider.CreateStablecoinDepositIntent(context.Background(), payments.StablecoinDepositIntentParams{Amount: 5000, Networks: []string{"base"}})
	if err != nil || intent == nil {
		t.Fatalf("configured backend call failed: %v", err)
	}
	if configured.calls != 1 || fallback.calls != 0 {
		t.Fatalf("configured calls = %d, global fallback calls = %d", configured.calls, fallback.calls)
	}
	if configured.lastKey != provider.secretKey {
		t.Fatal("configured backend did not receive its provider's own key")
	}
}
