package core

import "context"

type Provider interface {
	Name() string
	Priority() int
	Search(ctx context.Context, query string) ([]MediaRef, error)
	GetStream(ctx context.Context, ref MediaRef) (*Stream, error)
}

type ProviderRegistry interface {
	Register(p Provider)
	Get(name string) (Provider, bool)
	All() []Provider
	ByPriority() []Provider
}

type ProviderManager interface {
	SearchAll(ctx context.Context, query string) map[string][]MediaRef
	SearchWithProvider(ctx context.Context, providerName string, query string) ([]MediaRef, error)
	ResolveStream(ctx context.Context, ref MediaRef) (*Stream, error)
}
