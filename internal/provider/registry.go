package provider

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cam/cine-cli/internal/core"
)

type Registry struct {
	mu        sync.RWMutex
	providers map[string]core.Provider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]core.Provider),
	}
}

func (r *Registry) Register(p core.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (core.Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

func (r *Registry) All() []core.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]core.Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

func (r *Registry) ByPriority() []core.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]core.Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority() < result[j].Priority()
	})
	return result
}

type Manager struct {
	registry *Registry
}

func NewManager(registry *Registry) *Manager {
	return &Manager{registry: registry}
}

func (m *Manager) SearchAll(ctx context.Context, query string) map[string][]core.MediaRef {
	results := make(map[string][]core.MediaRef)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, p := range m.registry.ByPriority() {
		wg.Add(1)
		go func(prov core.Provider) {
			defer wg.Done()
			refs, err := prov.Search(ctx, query)
			if err != nil {
				return
			}
			mu.Lock()
			results[prov.Name()] = refs
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	return results
}

func (m *Manager) SearchWithProvider(ctx context.Context, providerName string, query string) ([]core.MediaRef, error) {
	p, ok := m.registry.Get(providerName)
	if !ok {
		return nil, fmt.Errorf("provider %q not found", providerName)
	}
	return p.Search(ctx, query)
}

func (m *Manager) ResolveStream(ctx context.Context, ref core.MediaRef) (*core.Stream, error) {
	p, ok := m.registry.Get(ref.ProviderName)
	if !ok {
		return nil, fmt.Errorf("provider %q not found", ref.ProviderName)
	}
	return p.GetStream(ctx, ref)
}

func (m *Manager) ListProviders() []string {
	providers := m.registry.ByPriority()
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}
	return names
}
