package adapter

import (
	"fmt"
	"sync"
)

// Registry resolves provider adapters by name.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]RateProvider
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]RateProvider),
	}
}

// Register adds a provider adapter to the registry.
func (r *Registry) Register(provider RateProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[provider.Name()] = provider
}

// ReplaceAll atomically swaps the whole adapter set (used by the config reload loop, SRS 3.1.2).
func (r *Registry) ReplaceAll(providers []RateProvider) {
	adapters := make(map[string]RateProvider, len(providers))
	for _, p := range providers {
		adapters[p.Name()] = p
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters = adapters
}

// Get returns a provider adapter by name.
func (r *Registry) Get(name string) (RateProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("provider adapter not found: %s", name)
	}
	return provider, nil
}
