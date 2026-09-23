package timing

import (
	"sync"
)

// Registry manages available timing vendor adapters.
type Registry struct {
	mu        sync.RWMutex
	providers map[ProviderType]any
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[ProviderType]any),
	}
}

// DefaultRegistry creates and pre-populates a registry with standard adapters.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(ProviderRaceResult, NewRaceResultAdapter())
	r.Register(ProviderGenericCSV, NewCSVAdapter())
	r.Register(ProviderVendorAPI, NewVendorAPIAdapter())
	return r
}

// Register registers an adapter for a given provider type.
func (r *Registry) Register(provider ProviderType, adapter any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider] = adapter
}

// Get returns the adapter registered for a provider type.
func (r *Registry) Get(provider ProviderType) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.providers[provider]
	return a, ok
}

// ListProviders returns all registered provider types.
func (r *Registry) ListProviders() []ProviderType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProviderType, 0, len(r.providers))
	for k := range r.providers {
		out = append(out, k)
	}
	return out
}

// GetPassingParser returns the PassingParser for the provider if supported.
func (r *Registry) GetPassingParser(provider ProviderType) (PassingParser, bool) {
	a, ok := r.Get(provider)
	if !ok {
		return nil, false
	}
	p, ok := a.(PassingParser)
	return p, ok
}

// GetParticipantExporter returns the ParticipantExporter for the provider if supported.
func (r *Registry) GetParticipantExporter(provider ProviderType) (ParticipantExporter, bool) {
	a, ok := r.Get(provider)
	if !ok {
		return nil, false
	}
	p, ok := a.(ParticipantExporter)
	return p, ok
}

// GetFinalResultParser returns the FinalResultParser for the provider if supported.
func (r *Registry) GetFinalResultParser(provider ProviderType) (FinalResultParser, bool) {
	a, ok := r.Get(provider)
	if !ok {
		return nil, false
	}
	p, ok := a.(FinalResultParser)
	return p, ok
}

// GetMappingParser returns the MappingParser for the provider if supported.
func (r *Registry) GetMappingParser(provider ProviderType) (MappingParser, bool) {
	a, ok := r.Get(provider)
	if !ok {
		return nil, false
	}
	p, ok := a.(MappingParser)
	return p, ok
}
