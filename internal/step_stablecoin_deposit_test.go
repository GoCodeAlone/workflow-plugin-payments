package internal

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestStablecoinDepositIntentStepStructPBRoundTrip(t *testing.T) {
	RegisterProvider("stablecoin-structpb", newMockProvider())
	t.Cleanup(func() { UnregisterProvider("stablecoin-structpb") })
	config := map[string]any{"module": "stablecoin-structpb"}
	step, err := newStablecoinDepositIntentStep("deposit", config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := step.Execute(context.Background(), nil, nil,
		map[string]any{"amount": int64(5000), "currency": "usd", "networks": []any{"base", "solana"}, "stablecoin": "usdc"}, nil, config)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output["error"] != nil {
		t.Fatalf("unexpected step error: %v", result.Output["error"])
	}
	// The Workflow SDK calls NewStruct directly; exercising that boundary
	// catches Go container types that cannot be sent over the gRPC wire.
	encoded, err := structpb.NewStruct(result.Output)
	if err != nil {
		t.Fatalf("stablecoin output cannot cross the structpb boundary: %v", err)
	}
	wire, err := proto.Marshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded := &structpb.Struct{}
	if err := proto.Unmarshal(wire, decoded); err != nil {
		t.Fatal(err)
	}
	output := decoded.AsMap()
	if output["payment_intent_id"] != result.Output["payment_intent_id"] || output["amount"] != float64(5000) {
		t.Fatal("scalar payment intent evidence changed during round trip")
	}
	addresses, ok := output["deposit_addresses"].([]any)
	if !ok || len(addresses) != 2 {
		t.Fatal("expected two address objects after structpb round trip")
	}
	for i, network := range []string{"base", "solana"} {
		address, ok := addresses[i].(map[string]any)
		if !ok || address["network"] != network || address["address"] != network+"_deposit_address" || address["stablecoin"] != "usdc" || address["token_contract_address"] != network+"_token_contract" {
			t.Fatalf("%s address/token evidence changed during structpb round trip", network)
		}
	}
}
