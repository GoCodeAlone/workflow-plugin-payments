package internal

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
	stripe "github.com/stripe/stripe-go/v85"
)

// stablecoinEvidenceBackend injects an SDK response without an HTTP request.
// Embedding Backend leaves unused operations out of this focused regression.
type stablecoinEvidenceBackend struct {
	stripe.Backend
	response *stripe.PaymentIntent
	calls    int
}

func (b *stablecoinEvidenceBackend) Call(method, path, _ string, _ stripe.ParamsContainer, out stripe.LastResponseSetter) error {
	if method != "POST" || path != "/v1/payment_intents" {
		return fmt.Errorf("unexpected Stripe request: %s %s", method, path)
	}
	b.calls++
	pi, ok := out.(*stripe.PaymentIntent)
	if !ok {
		return fmt.Errorf("unexpected Stripe response type %T", out)
	}
	*pi = *b.response
	return nil
}

func stablecoinEvidenceResponse(raw []byte) *stripe.PaymentIntent {
	pi := &stripe.PaymentIntent{
		ID: "pi_evidence", ClientSecret: "private_client_secret_marker",
		Amount: 5000, Currency: "usd", Status: "requires_action",
	}
	if raw != nil {
		pi.LastResponse = &stripe.APIResponse{RawJSON: raw}
	}
	return pi
}

func TestStripeCreateStablecoinDepositIntentRejectsIncompleteEvidence(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
	}{
		{"missing_response", nil},
		{"empty_raw_json", []byte{}},
		{"malformed_json", []byte(`{"client_secret":"private_client_secret_marker",`)},
		{"missing_addresses", []byte(`{"next_action":{"type":"crypto_display_details"}}`)},
		{"empty_addresses", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{}}}}`)},
		{"malformed_addresses", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":[]}}}`)},
		{"no_tokens", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":"private_deposit_address_marker","supported_tokens":[]}}}}}`)},
		{"no_matching_token", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":"private_deposit_address_marker","supported_tokens":[{"token_currency":"usdt","token_contract_address":"0xtoken"}]}}}}}`)},
		{"empty_address", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":"","supported_tokens":[{"token_currency":"usdc","token_contract_address":"0xtoken"}]}}}}}`)},
		{"blank_address", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":" ","supported_tokens":[{"token_currency":"usdc","token_contract_address":"0xtoken"}]}}}}}`)},
		{"empty_token_contract", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":"base_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":""}]}}}}}`)},
		{"blank_token_contract", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":"base_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":" "}]}}}}}`)},
		{"unsupported_response_network", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"ethereum":{"address":"ethereum_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":"ethereum_token"}]}}}}}`)},
		{"unrequested_response_network", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"solana":{"address":"solana_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":"solana_token"}]}}}}}`)},
		{"valid_plus_unsupported_response_network", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":"base_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":"base_token"}]},"ethereum":{"address":"ethereum_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":"ethereum_token"}]}}}}}`)},
		{"valid_plus_unrequested_response_network", []byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"base":{"address":"base_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":"base_token"}]},"solana":{"address":"solana_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":"solana_token"}]}}}}}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backend := &stablecoinEvidenceBackend{response: stablecoinEvidenceResponse(tc.raw)}
			previousBackend, previousKey := stripe.GetBackend(stripe.APIBackend), stripe.Key
			stripe.SetBackend(stripe.APIBackend, backend)
			t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, previousBackend); stripe.Key = previousKey })
			p := &stripeProvider{secretKey: "sk_test_fake", defaultCurrency: "usd"}
			intent, err := p.CreateStablecoinDepositIntent(context.Background(), payments.StablecoinDepositIntentParams{
				Amount: 5000, Networks: []string{"base"}, Stablecoin: "usdc",
			})
			if err == nil {
				t.Fatal("expected fail-closed error for incomplete stablecoin deposit evidence")
			}
			if intent != nil {
				t.Fatal("incomplete evidence must not return a successful payment intent")
			}
			for _, private := range []string{"private_client_secret_marker", "private_deposit_address_marker"} {
				if strings.Contains(err.Error(), private) {
					t.Fatal("error exposes private provider response data")
				}
			}
			if backend.calls != 1 {
				t.Fatalf("provider calls = %d, want exactly one", backend.calls)
			}
		})
	}
}

func TestStripeCreateStablecoinDepositIntentPreservesValidEvidence(t *testing.T) {
	backend := &stablecoinEvidenceBackend{response: stablecoinEvidenceResponse([]byte(`{"next_action":{"crypto_display_details":{"deposit_addresses":{"solana":{"address":"solana_address","supported_tokens":[{"token_currency":"usdc","token_contract_address":"solana_token"}]},"base":{"address":"base_address","supported_tokens":[{"token_currency":"USDC","token_contract_address":"base_token"}]}}}}}`))}
	previousBackend, previousKey := stripe.GetBackend(stripe.APIBackend), stripe.Key
	stripe.SetBackend(stripe.APIBackend, backend)
	t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, previousBackend); stripe.Key = previousKey })
	p := &stripeProvider{secretKey: "sk_test_fake", defaultCurrency: "usd"}
	intent, err := p.CreateStablecoinDepositIntent(context.Background(), payments.StablecoinDepositIntentParams{Amount: 5000, Networks: []string{"base", "solana"}})
	if err != nil {
		t.Fatal(err)
	}
	if intent.ID != "pi_evidence" || len(intent.DepositAddresses) != 2 {
		t.Fatal("expected valid payment intent and two addresses")
	}
	for i, network := range []string{"base", "solana"} {
		got := intent.DepositAddresses[i]
		if got.Network != network || got.Address != network+"_address" || got.Stablecoin != "usdc" || got.TokenContractAddress != network+"_token" {
			t.Fatalf("valid %s address/token evidence was not preserved", network)
		}
	}
}
