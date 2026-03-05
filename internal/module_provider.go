package internal

import (
	"context"
	"fmt"
)

// providerModule creates the appropriate PaymentProvider and registers it.
type providerModule struct {
	name     string
	config   map[string]any
	provider string // "stripe" or "paypal"
}

func newProviderModule(name string, config map[string]any) (*providerModule, error) {
	providerType, _ := config["provider"].(string)
	if providerType == "" {
		return nil, fmt.Errorf("payments.provider %q: config.provider is required (\"stripe\" or \"paypal\")", name)
	}
	return &providerModule{
		name:     name,
		config:   config,
		provider: providerType,
	}, nil
}

// Init creates the payment provider and registers it in the global registry.
func (m *providerModule) Init() error {
	switch m.provider {
	case "stripe":
		p, err := newStripeProvider(m.config)
		if err != nil {
			return fmt.Errorf("payments.provider %q: %w", m.name, err)
		}
		RegisterProvider(m.name, p)
	case "paypal":
		p, err := newPayPalProvider(m.config)
		if err != nil {
			return fmt.Errorf("payments.provider %q: %w", m.name, err)
		}
		RegisterProvider(m.name, p)
	default:
		return fmt.Errorf("payments.provider %q: unknown provider %q (supported: stripe, paypal)", m.name, m.provider)
	}
	return nil
}

// Start is a no-op for this module.
func (m *providerModule) Start(_ context.Context) error { return nil }

// Stop unregisters the provider.
func (m *providerModule) Stop(_ context.Context) error {
	UnregisterProvider(m.name)
	return nil
}

// Name returns the module name.
func (m *providerModule) Name() string { return m.name }
