package internal

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCLI_RejectsMissingURL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runCLI([]string{"payments", "webhook", "ensure", "--events", "a"}, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("expected non-zero exit, got 0")
	}
	if !strings.Contains(stderr.String(), "url") {
		t.Errorf("expected url-required error in stderr, got %q", stderr.String())
	}
}

func TestCLI_RejectsMissingEvents(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runCLI([]string{
		"payments", "webhook", "ensure",
		"--url", "https://example.com/api/v1/webhooks/stripe/issuing",
	}, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("expected non-zero exit, got 0")
	}
	if !strings.Contains(stderr.String(), "events") {
		t.Errorf("expected events-required error, got %q", stderr.String())
	}
}

func TestCLI_RejectsMissingAPIKeyEnv(t *testing.T) {
	t.Setenv("STRIPE_SECRET_KEY", "")
	var stdout, stderr bytes.Buffer
	rc := runCLI([]string{
		"payments", "webhook", "ensure",
		"--url", "https://example.com/api/v1/webhooks/stripe/issuing",
		"--events", "a,b",
	}, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("expected non-zero exit when STRIPE_SECRET_KEY missing")
	}
	if !strings.Contains(stderr.String(), "STRIPE_SECRET_KEY") {
		t.Errorf("expected STRIPE_SECRET_KEY mention in error, got %q", stderr.String())
	}
}

func TestCLI_HappyPath_PrintsJSONResult(t *testing.T) {
	// Wire the test stripe HTTP server so webhookendpoint.New hits it.
	h := newStripeWebhookHandler()
	_, _ = newTestStripeProvider(t, h.handler(t))
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_fake")

	var stdout, stderr bytes.Buffer
	rc := runCLI([]string{
		"payments", "webhook", "ensure",
		"--url", "https://example.com/api/v1/webhooks/stripe/issuing",
		"--events", "issuing_authorization.request,issuing_card.updated",
		"--description", "Test webhook",
	}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", rc, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode JSON: %v\nstdout=%q", err, stdout.String())
	}
	if got["created"] != true {
		t.Errorf("expected created=true, got %v", got["created"])
	}
	if id, _ := got["endpoint_id"].(string); id == "" {
		t.Errorf("expected endpoint_id, got empty")
	}
	if secret, _ := got["signing_secret"].(string); secret == "" {
		t.Errorf("expected signing_secret on fresh-create")
	}
}

func TestCLI_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runCLI([]string{"payments", "vacuum", "everything"}, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("expected non-zero exit on unknown subcommand")
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Errorf("expected unknown-subcommand error, got %q", stderr.String())
	}
}

func TestCLI_NotPaymentsRootCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runCLI([]string{"some-other-cmd"}, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("expected non-zero exit when args[0] is not 'payments'")
	}
}

func TestCLI_ReplaceModePropagates(t *testing.T) {
	h := newStripeWebhookHandler()
	// Pre-populate so replace path fires (DELETE+POST).
	h.endpointsByURL["https://example.com/api/v1/webhooks/stripe/issuing"] = map[string]any{
		"id":             "we_old",
		"object":         "webhook_endpoint",
		"url":            "https://example.com/api/v1/webhooks/stripe/issuing",
		"enabled_events": []string{"a"},
		"livemode":       false,
		"status":         "enabled",
	}
	_, _ = newTestStripeProvider(t, h.handler(t))
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_fake")

	var stdout, stderr bytes.Buffer
	rc := runCLI([]string{
		"payments", "webhook", "ensure",
		"--url", "https://example.com/api/v1/webhooks/stripe/issuing",
		"--events", "a",
		"--mode", "replace",
	}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", rc, stderr.String())
	}
	if h.delCalls != 1 || h.newCalls != 1 {
		t.Errorf("expected replace-mode del=1 new=1, got del=%d new=%d", h.delCalls, h.newCalls)
	}
}

func TestSplitEvents(t *testing.T) {
	got := splitEvents("  a , b,, c  ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitEvents length: got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("splitEvents[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}
