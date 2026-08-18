package resolver

import (
	"context"
	"fmt"
)

type StreamResult struct {
	URL       string
	Referer   string
	UserAgent string
	Subtitles []SubData
	Provider  string
}

type SubData struct {
	URL  string
	Lang string
}

type Resolver interface {
	Name() string
	Available() bool
	Resolve(ctx context.Context, url string) (*StreamResult, error)
}

type Chain struct {
	resolvers []Resolver
}

func NewChain(resolvers ...Resolver) *Chain {
	return &Chain{resolvers: resolvers}
}

func (c *Chain) Resolve(ctx context.Context, url string) (*StreamResult, error) {
	var errs []string
	for _, r := range c.resolvers {
		if !r.Available() {
			continue
		}
		result, err := r.Resolve(ctx, url)
		if err == nil && result != nil && result.URL != "" {
			return result, nil
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.Name(), err))
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("all resolvers failed: %v", errs)
	}
	return nil, fmt.Errorf("no resolver available for %s", url)
}
