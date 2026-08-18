package search

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

type Result struct {
	Provider string
	Results  []core.MediaRef
	Error    error
	Duration time.Duration
}

type Engine struct {
	manager core.ProviderManager
}

func NewEngine(manager core.ProviderManager) *Engine {
	return &Engine{manager: manager}
}

func (e *Engine) ParallelSearch(ctx context.Context, query string, providerNames []string, timeout time.Duration) ([]Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	resultsCh := make(chan Result, len(providerNames))

	for _, name := range providerNames {
		wg.Add(1)
		go func(pname string) {
			defer wg.Done()
			start := time.Now()

			refs, err := e.manager.SearchWithProvider(ctx, pname, query)
			resultsCh <- Result{
				Provider: pname,
				Results:  refs,
				Error:    err,
				Duration: time.Since(start),
			}
		}(name)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	results := make([]Result, 0)
	for r := range resultsCh {
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool {
		return len(results[i].Results) > len(results[j].Results)
	})

	return results, nil
}

func (e *Engine) MergedSearch(ctx context.Context, query string, providerNames []string, timeout time.Duration) ([]core.MediaRef, error) {
	results, err := e.ParallelSearch(ctx, query, providerNames, timeout)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var merged []core.MediaRef

	for _, r := range results {
		for _, ref := range r.Results {
			key := strings.ToLower(ref.Title + ref.Year)
			if !seen[key] {
				seen[key] = true
				merged = append(merged, ref)
			}
		}
	}

	return merged, nil
}

type FilterFunc func(ref core.MediaRef) bool

func FilterResults(results []core.MediaRef, filters ...FilterFunc) []core.MediaRef {
	var filtered []core.MediaRef
	for _, ref := range results {
		include := true
		for _, f := range filters {
			if !f(ref) {
				include = false
				break
			}
		}
		if include {
			filtered = append(filtered, ref)
		}
	}
	return filtered
}

func SortByRelevance(results []core.MediaRef, query string) {
	query = strings.ToLower(query)
	sort.Slice(results, func(i, j int) bool {
		ti := strings.ToLower(results[i].Title)
		tj := strings.ToLower(results[j].Title)

		// Exact match first
		if ti == query && tj != query {
			return true
		}
		if tj == query && ti != query {
			return false
		}

		// Starts with query
		si := strings.HasPrefix(ti, query)
		sj := strings.HasPrefix(tj, query)
		if si && !sj {
			return true
		}
		if sj && !si {
			return false
		}

		// Contains query
		ci := strings.Contains(ti, query)
		cj := strings.Contains(tj, query)
		if ci && !cj {
			return true
		}
		if cj && !ci {
			return false
		}

		return ti < tj
	})
}

func DeduplicateRefs(refs []core.MediaRef) []core.MediaRef {
	seen := make(map[string]bool)
	result := make([]core.MediaRef, 0)

	for _, ref := range refs {
		key := strings.ToLower(fmt.Sprintf("%s-%s-%s", ref.Title, ref.Year, ref.MediaType))
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, ref)
	}

	return result
}
