package core

import "context"

type MetadataProvider interface {
	Search(ctx context.Context, filter SearchFilter) ([]Media, error)
	GetDetails(ctx context.Context, tmdbID int, mediaType MediaType) (*Media, error)
	GetSeasons(ctx context.Context, seriesID int) ([]Season, error)
	GetEpisodes(ctx context.Context, seriesID int, seasonNum int) ([]Episode, error)
	GetTrending(ctx context.Context, mediaType MediaType, page int) ([]Media, error)
	GetRecommendations(ctx context.Context, tmdbID int, mediaType MediaType) ([]Media, error)
}

type MetadataCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
}
