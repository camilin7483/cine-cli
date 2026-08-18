package core

import "context"

type SearchEngine interface {
	Search(ctx context.Context, filter SearchFilter) ([]Media, error)
	SearchWithProviders(ctx context.Context, query string, providers []string) (map[string][]MediaRef, error)
	FuzzyFilter(results []Media, query string) []Media
}

type SearchIndex interface {
	Index(media []Media) error
	Query(query string) ([]Media, error)
}
