package internal

import (
	"context"
	"testing"
)

func TestProviderModule_MissingProvider(t *testing.T) {
	_, err := newProviderModule("test", map[string]any{})
	if err == nil {
		t.Error("expected error for missing provider config")
	}
}

func TestProviderModule_UnknownProvider(t *testing.T) {
	m, err := newProviderModule("test", map[string]any{"provider": "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Init(); err == nil {
		t.Error("expected error for unknown provider type")
	}
}

func TestProviderModule_StopUnregisters(t *testing.T) {
	// Register a mock provider directly.
	mock := newMockProvider()
	RegisterProvider("mod-stop-test", mock)

	m := &providerModule{name: "mod-stop-test", provider: "stripe"}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, ok := GetProvider("mod-stop-test"); ok {
		t.Error("provider should be unregistered after Stop()")
	}
}

func TestProviderModule_StartIsNoop(t *testing.T) {
	m := &providerModule{name: "test", provider: "stripe"}
	if err := m.Start(context.Background()); err != nil {
		t.Errorf("unexpected error from Start: %v", err)
	}
}
