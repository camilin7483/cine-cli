package search

import (
	"context"
	"testing"

	"github.com/cam/cine-cli/internal/core"
)

type mockProviderManager struct {
	searchWithProviderFn func(ctx context.Context, providerName string, query string) ([]core.MediaRef, error)
}

func (m *mockProviderManager) SearchWithProvider(ctx context.Context, providerName string, query string) ([]core.MediaRef, error) {
	return m.searchWithProviderFn(ctx, providerName, query)
}

func (m *mockProviderManager) SearchAll(ctx context.Context, query string) map[string][]core.MediaRef {
	return nil
}

func (m *mockProviderManager) ResolveStream(ctx context.Context, ref core.MediaRef) (*core.Stream, error) {
	return nil, nil
}

func TestNewEngine(t *testing.T) {
	mgr := &mockProviderManager{}
	engine := NewEngine(mgr)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine.manager == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestDeduplicateRefs(t *testing.T) {
	refs := []core.MediaRef{
		{Title: "The Matrix", Year: "1999", MediaType: core.MediaTypeMovie},
		{Title: "The Matrix", Year: "1999", MediaType: core.MediaTypeMovie},
		{Title: "The Matrix", Year: "1999", MediaType: core.MediaTypeMovie},
		{Title: "Inception", Year: "2010", MediaType: core.MediaTypeMovie},
		{Title: "Inception", Year: "2010", MediaType: core.MediaTypeMovie},
		{Title: "The Matrix", Year: "1999", MediaType: core.MediaTypeSeries},
	}

	result := DeduplicateRefs(refs)
	if len(result) != 3 {
		t.Errorf("expected 3 unique refs, got %d", len(result))
	}

	titles := make(map[string]bool)
	for _, r := range result {
		titles[r.Title] = true
	}
	if !titles["The Matrix"] || !titles["Inception"] {
		t.Errorf("missing expected titles in result: %v", result)
	}
}

func TestDeduplicateRefsEmpty(t *testing.T) {
	result := DeduplicateRefs(nil)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}

	result = DeduplicateRefs([]core.MediaRef{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

func TestDeduplicateRefsCaseInsensitive(t *testing.T) {
	refs := []core.MediaRef{
		{Title: "Breaking Bad", Year: "2008", MediaType: core.MediaTypeSeries},
		{Title: "breaking bad", Year: "2008", MediaType: core.MediaTypeSeries},
		{Title: "BREAKING BAD", Year: "2008", MediaType: core.MediaTypeSeries},
	}

	result := DeduplicateRefs(refs)
	if len(result) != 1 {
		t.Errorf("expected 1 unique ref, got %d", len(result))
	}
}

func TestSortByRelevance(t *testing.T) {
	results := []core.MediaRef{
		{Title: "The Godfather Part II", Year: "1974"},
		{Title: "The Godfather", Year: "1972"},
		{Title: "Goodfellas", Year: "1990"},
		{Title: "The Godfather Part III", Year: "1990"},
		{Title: "Godzilla", Year: "1998"},
	}

	SortByRelevance(results, "The Godfather")

	if results[0].Title != "The Godfather" {
		t.Errorf("expected exact match 'The Godfather' first, got %q", results[0].Title)
	}

	foundExact := false
	for _, r := range results {
		if r.Title == "The Godfather" {
			foundExact = true
		}
	}
	if !foundExact {
		t.Error("exact match not found in results")
	}
}

func TestSortByRelevanceExactFirst(t *testing.T) {
	results := []core.MediaRef{
		{Title: "Star Wars: The Force Awakens", Year: "2015"},
		{Title: "Star Wars", Year: "1977"},
		{Title: "Star Wars: The Last Jedi", Year: "2017"},
		{Title: "Starman", Year: "1984"},
	}

	SortByRelevance(results, "Star Wars")

	if results[0].Title != "Star Wars" {
		t.Errorf("expected exact match 'Star Wars' first, got %q", results[0].Title)
	}
}

func TestSortByRelevancePrefixBeforeContains(t *testing.T) {
	results := []core.MediaRef{
		{Title: "Avengers: Endgame", Year: "2019"},
		{Title: "The Avengers", Year: "2012"},
		{Title: "Avengers", Year: "2019"},
	}

	SortByRelevance(results, "Avengers")

	if results[0].Title != "Avengers" {
		t.Errorf("expected exact match 'Avengers' first, got %q", results[0].Title)
	}

	foundPrefix := false
	foundContains := false
	for _, r := range results {
		if r.Title == "Avengers: Endgame" {
			foundPrefix = true
		}
		if r.Title == "The Avengers" {
			foundContains = true
		}
	}
	if !foundPrefix || !foundContains {
		t.Error("missing expected titles after sort")
	}
}

func TestFilterResults(t *testing.T) {
	results := []core.MediaRef{
		{Title: "The Matrix", Year: "1999", MediaType: core.MediaTypeMovie},
		{Title: "The Matrix Reloaded", Year: "2003", MediaType: core.MediaTypeMovie},
		{Title: "Breaking Bad", Year: "2008", MediaType: core.MediaTypeSeries},
		{Title: "Better Call Saul", Year: "2015", MediaType: core.MediaTypeSeries},
	}

	filtered := FilterResults(results, func(ref core.MediaRef) bool {
		return ref.MediaType == core.MediaTypeMovie
	})

	if len(filtered) != 2 {
		t.Errorf("expected 2 movies, got %d", len(filtered))
	}
	for _, r := range filtered {
		if r.MediaType != core.MediaTypeMovie {
			t.Errorf("expected only movies, got %s", r.MediaType)
		}
	}
}

func TestFilterResultsMultiple(t *testing.T) {
	results := []core.MediaRef{
		{Title: "The Matrix", Year: "1999", MediaType: core.MediaTypeMovie},
		{Title: "The Matrix Reloaded", Year: "2003", MediaType: core.MediaTypeMovie},
		{Title: "Breaking Bad", Year: "2008", MediaType: core.MediaTypeSeries},
		{Title: "Better Call Saul", Year: "2015", MediaType: core.MediaTypeSeries},
	}

	filtered := FilterResults(results,
		func(ref core.MediaRef) bool {
			return ref.MediaType == core.MediaTypeSeries
		},
		func(ref core.MediaRef) bool {
			return ref.Year >= "2010"
		},
	)

	if len(filtered) != 1 {
		t.Errorf("expected 1 result, got %d", len(filtered))
	}
	if filtered[0].Title != "Better Call Saul" {
		t.Errorf("expected 'Better Call Saul', got %q", filtered[0].Title)
	}
}

func TestFilterResultsEmpty(t *testing.T) {
	results := []core.MediaRef{
		{Title: "The Matrix", Year: "1999"},
	}

	filtered := FilterResults(results, func(ref core.MediaRef) bool {
		return false
	})

	if len(filtered) != 0 {
		t.Errorf("expected 0 results, got %d", len(filtered))
	}
}

func TestFilterResultsNoFilters(t *testing.T) {
	results := []core.MediaRef{
		{Title: "The Matrix", Year: "1999"},
		{Title: "Inception", Year: "2010"},
	}

	filtered := FilterResults(results)
	if len(filtered) != 2 {
		t.Errorf("expected all 2 results, got %d", len(filtered))
	}
}
