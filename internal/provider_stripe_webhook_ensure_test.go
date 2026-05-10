package internal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

// stripeWebhookHandler is a minimal mock of the Stripe webhook-endpoints API.
// It tracks endpoints by URL key and serves the four operations the test
// matrix needs: list, create, update, delete.
type stripeWebhookHandler struct {
	mu             sync.Mutex
	endpointsByURL map[string]map[string]any // url → object
	idCounter      int
	listCalls      int
	newCalls       int
	updateCalls    int
	delCalls       int
	listErr        bool
}

func newStripeWebhookHandler() *stripeWebhookHandler {
	return &stripeWebhookHandler{endpointsByURL: map[string]map[string]any{}}
}

func (h *stripeWebhookHandler) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		defer h.mu.Unlock()

		path := r.URL.Path
		switch {
		case r.Method == "GET" && path == "/v1/webhook_endpoints":
			h.listCalls++
			if h.listErr {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"message":"forced list error","type":"api_error"}}`))
				return
			}
			data := make([]map[string]any, 0, len(h.endpointsByURL))
			for _, ep := range h.endpointsByURL {
				data = append(data, ep)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object":   "list",
				"data":     data,
				"has_more": false,
			})
		case r.Method == "POST" && path == "/v1/webhook_endpoints":
			h.newCalls++
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			h.idCounter++
			id := "we_test_" + intToStr(h.idCounter)
			url := r.PostForm.Get("url")
			ep := map[string]any{
				"id":             id,
				"object":         "webhook_endpoint",
				"url":            url,
				"enabled_events": collectEvents(r.PostForm["enabled_events[]"]),
				"secret":         "whsec_fresh_" + id,
				"description":    r.PostForm.Get("description"),
				"livemode":       false,
				"status":         "enabled",
			}
			h.endpointsByURL[url] = ep
			_ = json.NewEncoder(w).Encode(ep)
		case r.Method == "POST" && strings.HasPrefix(path, "/v1/webhook_endpoints/"):
			h.updateCalls++
			id := strings.TrimPrefix(path, "/v1/webhook_endpoints/")
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			for url, ep := range h.endpointsByURL {
				if ep["id"] == id {
					ep["enabled_events"] = collectEvents(r.PostForm["enabled_events[]"])
					if d := r.PostForm.Get("description"); d != "" {
						ep["description"] = d
					}
					h.endpointsByURL[url] = ep
					// Update path returns endpoint without the once-only secret.
					responseEp := copyMap(ep)
					delete(responseEp, "secret")
					_ = json.NewEncoder(w).Encode(responseEp)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "DELETE" && strings.HasPrefix(path, "/v1/webhook_endpoints/"):
			h.delCalls++
			id := strings.TrimPrefix(path, "/v1/webhook_endpoints/")
			for url, ep := range h.endpointsByURL {
				if ep["id"] == id {
					delete(h.endpointsByURL, url)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"id":      id,
						"object":  "webhook_endpoint",
						"deleted": true,
					})
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func collectEvents(form []string) []string {
	out := make([]string, 0, len(form))
	for _, e := range form {
		e = strings.TrimSpace(e)
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestWebhookEndpointEnsure_FreshCreate(t *testing.T) {
	h := newStripeWebhookHandler()
	p, _ := newTestStripeProvider(t, h.handler(t))

	out, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL:    "https://example.com/api/v1/webhooks/stripe/issuing",
		Events: []string{"issuing_authorization.request"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Created {
		t.Errorf("expected created=true on fresh-create")
	}
	if out.SigningSecret == "" {
		t.Errorf("expected signing_secret populated on fresh create")
	}
	if out.EndpointID == "" {
		t.Errorf("expected endpoint_id populated")
	}
	if h.newCalls != 1 {
		t.Errorf("expected 1 New call, got %d", h.newCalls)
	}
}

func TestWebhookEndpointEnsure_IdempotentNoOp(t *testing.T) {
	h := newStripeWebhookHandler()
	// Pre-populate an endpoint with the events the test will request.
	h.endpointsByURL["https://example.com/api/v1/webhooks/stripe/issuing"] = map[string]any{
		"id":             "we_existing",
		"object":         "webhook_endpoint",
		"url":            "https://example.com/api/v1/webhooks/stripe/issuing",
		"enabled_events": []string{"a", "b"},
		"livemode":       false,
		"status":         "enabled",
	}
	p, _ := newTestStripeProvider(t, h.handler(t))

	out, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL:    "https://example.com/api/v1/webhooks/stripe/issuing",
		Events: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Created {
		t.Errorf("V1: expected created=false on no-op")
	}
	if out.EventsDrift {
		t.Errorf("V1: expected events_drift=false on no-op")
	}
	if out.SigningSecret != "" {
		t.Errorf("V3: expected signing_secret='' on no-op, got %q", out.SigningSecret)
	}
	if h.newCalls+h.updateCalls+h.delCalls != 0 {
		t.Errorf("V1: expected zero write calls, got new=%d update=%d del=%d", h.newCalls, h.updateCalls, h.delCalls)
	}
}

func TestWebhookEndpointEnsure_EventsDriftEnsure(t *testing.T) {
	h := newStripeWebhookHandler()
	h.endpointsByURL["https://example.com/api/v1/webhooks/stripe/issuing"] = map[string]any{
		"id":             "we_existing",
		"object":         "webhook_endpoint",
		"url":            "https://example.com/api/v1/webhooks/stripe/issuing",
		"enabled_events": []string{"a"},
		"livemode":       false,
		"status":         "enabled",
	}
	p, _ := newTestStripeProvider(t, h.handler(t))

	out, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL:    "https://example.com/api/v1/webhooks/stripe/issuing",
		Events: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Created {
		t.Errorf("V2: expected created=false on update")
	}
	if !out.EventsDrift {
		t.Errorf("V2: expected events_drift=true on update")
	}
	if out.SigningSecret != "" {
		t.Errorf("V3: expected signing_secret='' on update path, got %q", out.SigningSecret)
	}
	if h.updateCalls != 1 {
		t.Errorf("V2: expected exactly 1 Update call, got %d", h.updateCalls)
	}
	if h.newCalls+h.delCalls != 0 {
		t.Errorf("V2: expected no create/delete calls, got new=%d del=%d", h.newCalls, h.delCalls)
	}
}

func TestWebhookEndpointEnsure_ReplaceRotatesSecret(t *testing.T) {
	h := newStripeWebhookHandler()
	h.endpointsByURL["https://example.com/api/v1/webhooks/stripe/issuing"] = map[string]any{
		"id":             "we_old",
		"object":         "webhook_endpoint",
		"url":            "https://example.com/api/v1/webhooks/stripe/issuing",
		"enabled_events": []string{"a"},
		"livemode":       false,
		"status":         "enabled",
	}
	p, _ := newTestStripeProvider(t, h.handler(t))

	out, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL:    "https://example.com/api/v1/webhooks/stripe/issuing",
		Events: []string{"a"},
		Mode:   "replace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Created {
		t.Errorf("V8: expected created=true on replace")
	}
	if out.SigningSecret == "" {
		t.Errorf("V8: expected new signing_secret on replace")
	}
	if h.delCalls != 1 || h.newCalls != 1 {
		t.Errorf("V8: expected del=1 new=1 on replace, got del=%d new=%d", h.delCalls, h.newCalls)
	}
}

func TestWebhookEndpointEnsure_EventsOrderIndependent(t *testing.T) {
	h := newStripeWebhookHandler()
	h.endpointsByURL["https://example.com/api/v1/webhooks/stripe/issuing"] = map[string]any{
		"id":             "we_x",
		"object":         "webhook_endpoint",
		"url":            "https://example.com/api/v1/webhooks/stripe/issuing",
		"enabled_events": []string{"b", "a", "c"},
		"livemode":       false,
		"status":         "enabled",
	}
	p, _ := newTestStripeProvider(t, h.handler(t))

	out, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL: "https://example.com/api/v1/webhooks/stripe/issuing",
		// reordered with duplicates
		Events: []string{"c", "a", "b", "a"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.EventsDrift {
		t.Errorf("V9: reorder+dup should not register as drift")
	}
	if h.updateCalls != 0 {
		t.Errorf("V9: expected zero update calls, got %d", h.updateCalls)
	}
}

func TestWebhookEndpointEnsure_RejectsEmptyKey(t *testing.T) {
	p := &stripeProvider{secretKey: ""}
	_, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL:    "https://example.com/api/v1/webhooks/stripe/issuing",
		Events: []string{"x"},
	})
	if !errors.Is(err, payments.ErrStripeKeyMissing) {
		t.Errorf("expected ErrStripeKeyMissing, got %v", err)
	}
}

func TestWebhookEndpointEnsure_RejectsEmptyURL(t *testing.T) {
	h := newStripeWebhookHandler()
	p, _ := newTestStripeProvider(t, h.handler(t))
	_, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		Events: []string{"x"},
	})
	if err == nil || !strings.Contains(err.Error(), "url") {
		t.Errorf("expected url-required error, got %v", err)
	}
}

func TestWebhookEndpointEnsure_RejectsEmptyEvents(t *testing.T) {
	h := newStripeWebhookHandler()
	p, _ := newTestStripeProvider(t, h.handler(t))
	_, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL: "https://example.com/api/v1/webhooks/stripe/issuing",
	})
	if err == nil || !strings.Contains(err.Error(), "events") {
		t.Errorf("expected events-required error, got %v", err)
	}
}

func TestWebhookEndpointEnsure_RejectsInvalidMode(t *testing.T) {
	h := newStripeWebhookHandler()
	p, _ := newTestStripeProvider(t, h.handler(t))
	_, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL:    "https://example.com/api/v1/webhooks/stripe/issuing",
		Events: []string{"x"},
		Mode:   "delete",
	})
	if err == nil || !strings.Contains(err.Error(), "mode") {
		t.Errorf("expected mode error, got %v", err)
	}
}

func TestWebhookEndpointEnsure_StripeListError(t *testing.T) {
	h := newStripeWebhookHandler()
	h.listErr = true
	p, _ := newTestStripeProvider(t, h.handler(t))
	out, err := p.WebhookEndpointEnsure(context.Background(), payments.WebhookEndpointEnsureParams{
		URL:    "https://example.com/api/v1/webhooks/stripe/issuing",
		Events: []string{"x"},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "list webhook endpoints") {
		t.Errorf("expected wrapped list error, got %v", err)
	}
	if out != nil {
		t.Errorf("expected nil result on error, got %+v", out)
	}
}

func TestNormalizeWebhookEvents(t *testing.T) {
	got := normalizeWebhookEvents([]string{"  Issuing.B  ", "issuing.a", "issuing.B", "", "issuing.A"})
	want := []string{"issuing.a", "issuing.b"}
	if len(got) != len(want) {
		t.Fatalf("normalizeWebhookEvents length: got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("normalizeWebhookEvents[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}
