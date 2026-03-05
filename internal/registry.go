package internal

import (
	"sync"

	"github.com/GoCodeAlone/workflow-plugin-payments/payments"
)

var (
	providerMu       sync.RWMutex
	providerRegistry = make(map[string]payments.PaymentProvider)
)

// RegisterProvider adds a provider to the global registry under the given name.
func RegisterProvider(name string, p payments.PaymentProvider) {
	providerMu.Lock()
	defer providerMu.Unlock()
	providerRegistry[name] = p
}

// GetProvider looks up a provider by name.
func GetProvider(name string) (payments.PaymentProvider, bool) {
	providerMu.RLock()
	defer providerMu.RUnlock()
	p, ok := providerRegistry[name]
	return p, ok
}

// UnregisterProvider removes a provider from the registry (used in tests and module stop).
func UnregisterProvider(name string) {
	providerMu.Lock()
	defer providerMu.Unlock()
	delete(providerRegistry, name)
}
