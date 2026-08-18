package tmdb

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cam/cine-cli/internal/core"
	tmdb "github.com/cyruzin/golang-tmdb"
)

type Provider struct {
	client *tmdb.Client
	lang   string
	cache  map[string]*core.Media
}

func New(apiKey string, lang string) (*Provider, error) {
	client, err := tmdb.Init(apiKey)
	if err != nil {
		return nil, fmt.Errorf("tmdb init: %w", err)
	}

	client.SetClientConfig(http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{MaxIdleConns: 10, IdleConnTimeout: 30 * time.Second},
	})

	return &Provider{
		client: client,
		lang:   lang,
		cache:  make(map[string]*core.Media),
	}, nil
}

func (p *Provider) Search(ctx context.Context, filter core.SearchFilter) ([]core.Media, error) {
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	opts := map[string]string{"language": p.lang, "page": strconv.Itoa(page)}

	switch filter.MediaType {
	case core.MediaTypeMovie:
		return p.searchMovies(ctx, filter.Query, opts)
	case core.MediaTypeSeries:
		return p.searchSeries(ctx, filter.Query, opts)
	default:
		movies, _ := p.searchMovies(ctx, filter.Query, opts)
		series, _ := p.searchSeries(ctx, filter.Query, opts)
		return append(movies, series...), nil
	}
}

func (p *Provider) searchMovies(ctx context.Context, query string, opts map[string]string) ([]core.Media, error) {
	result, err := p.client.GetSearchMovies(query, opts)
	if err != nil {
		return nil, fmt.Errorf("tmdb search movies: %w", err)
	}

	media := make([]core.Media, 0)
	for _, r := range result.Results {
		m := convertMovieResult(r)
		media = append(media, m)
		p.cache[m.ID] = &m
	}
	return media, nil
}

func (p *Provider) searchSeries(ctx context.Context, query string, opts map[string]string) ([]core.Media, error) {
	result, err := p.client.GetSearchTVShow(query, opts)
	if err != nil {
		return nil, fmt.Errorf("tmdb search tv: %w", err)
	}

	media := make([]core.Media, 0)
	for _, r := range result.Results {
		m := convertTVResult(r)
		media = append(media, m)
		p.cache[m.ID] = &m
	}
	return media, nil
}

func (p *Provider) GetDetails(ctx context.Context, tmdbID int, mediaType core.MediaType) (*core.Media, error) {
	cacheKey := fmt.Sprintf("detail:%d:%s", tmdbID, mediaType)
	if cached, ok := p.cache[cacheKey]; ok {
		return cached, nil
	}

	switch mediaType {
	case core.MediaTypeMovie:
		movie, err := p.client.GetMovieDetails(tmdbID, map[string]string{"language": p.lang})
		if err != nil {
			return nil, fmt.Errorf("tmdb movie details: %w", err)
		}
		m := convertMovieDetails(movie)
		p.cache[cacheKey] = &m
		return &m, nil

	case core.MediaTypeSeries:
		tv, err := p.client.GetTVDetails(tmdbID, map[string]string{"language": p.lang})
		if err != nil {
			return nil, fmt.Errorf("tmdb tv details: %w", err)
		}
		m := convertTVDetails(tv)
		p.cache[cacheKey] = &m
		return &m, nil
	}
	return nil, fmt.Errorf("unknown media type: %s", mediaType)
}

func (p *Provider) GetSeasons(ctx context.Context, seriesID int) ([]core.Season, error) {
	tv, err := p.client.GetTVDetails(seriesID, map[string]string{"language": p.lang})
	if err != nil {
		return nil, fmt.Errorf("tmdb seasons: %w", err)
	}

	seasons := make([]core.Season, 0)
	for _, s := range tv.Seasons {
		seasons = append(seasons, core.Season{
			SeasonNumber: s.SeasonNumber,
			Name:         s.Name,
			EpisodeCount: s.EpisodeCount,
			PosterURL:    posterURL(s.PosterPath),
			Overview:     s.Overview,
		})
	}
	return seasons, nil
}

func (p *Provider) GetEpisodes(ctx context.Context, seriesID int, seasonNum int) ([]core.Episode, error) {
	season, err := p.client.GetTVSeasonDetails(seriesID, seasonNum, map[string]string{"language": p.lang})
	if err != nil {
		return nil, fmt.Errorf("tmdb episodes: %w", err)
	}

	episodes := make([]core.Episode, 0)
	for _, e := range season.Episodes {
		episodes = append(episodes, core.Episode{
			EpisodeNumber: e.EpisodeNumber,
			SeasonNumber:  e.SeasonNumber,
			Name:          e.Name,
			Overview:      e.Overview,
			StillURL:      posterURL(e.StillPath),
			AirDate:       e.AirDate,
		})
	}
	return episodes, nil
}

func (p *Provider) GetTrending(ctx context.Context, mediaType core.MediaType, page int) ([]core.Media, error) {
	opts := map[string]string{"language": p.lang, "page": strconv.Itoa(page)}
	timeWindow := "week"
	mt := "movie"
	if mediaType == core.MediaTypeSeries {
		mt = "tv"
	}

	trending, err := p.client.GetTrending(mt, timeWindow, opts)
	if err != nil {
		return nil, err
	}

	results := make([]core.Media, 0)
	for _, r := range trending.Results {
		if r.MediaType == "person" {
			continue
		}
		m := core.Media{
			TMDBID:    int(r.ID),
			Title:     firstNonEmpty(r.Title, r.Name),
			MediaType: ifType(r.MediaType),
			Year:      extractYear(firstNonEmpty(r.ReleaseDate, r.FirstAirDate)),
			Overview:  r.Overview,
			PosterURL: posterURL(r.PosterPath),
			Rating:    float64(r.Popularity),
		}
		results = append(results, m)
	}
	return results, nil
}

func (p *Provider) GetRecommendations(ctx context.Context, tmdbID int, mediaType core.MediaType) ([]core.Media, error) {
	opts := map[string]string{"language": p.lang}
	switch mediaType {
	case core.MediaTypeMovie:
		resp, err := p.client.GetMovieRecommendations(tmdbID, opts)
		if err != nil {
			return nil, err
		}
		results := make([]core.Media, 0)
		for _, r := range resp.Results {
			results = append(results, core.Media{
				ID:        fmt.Sprintf("tmdb-%d", r.ID),
				TMDBID:    int(r.ID),
				Title:     r.Title,
				MediaType: core.MediaTypeMovie,
				Year:      extractYear(r.ReleaseDate),
				Overview:  r.Overview,
				PosterURL: posterURL(r.PosterPath),
				Rating:    float64(r.VoteAverage),
			})
		}
		return results, nil
	case core.MediaTypeSeries:
		resp, err := p.client.GetTVRecommendations(tmdbID, opts)
		if err != nil {
			return nil, err
		}
		results := make([]core.Media, 0)
		for _, r := range resp.Results {
			results = append(results, core.Media{
				ID:        fmt.Sprintf("tmdb-%d", r.ID),
				TMDBID:    int(r.ID),
				Title:     r.Name,
				MediaType: core.MediaTypeSeries,
				Year:      extractYear(r.FirstAirDate),
				Overview:  r.Overview,
				PosterURL: posterURL(r.PosterPath),
				Rating:    float64(r.VoteAverage),
			})
		}
		return results, nil
	}
	return nil, nil
}

func convertMovieResult(r tmdb.MovieResult) core.Media {
	return core.Media{
		ID:        fmt.Sprintf("tmdb-%d", r.ID),
		TMDBID:    int(r.ID),
		Title:     r.Title,
		MediaType: core.MediaTypeMovie,
		Year:      extractYear(r.ReleaseDate),
		Overview:  r.Overview,
		PosterURL: posterURL(r.PosterPath),
		Rating:    float64(r.VoteAverage),
	}
}

func convertMovieDetails(m *tmdb.MovieDetails) core.Media {
	media := core.Media{
		ID:          fmt.Sprintf("tmdb-%d", m.ID),
		TMDBID:      int(m.ID),
		IMDBID:      m.IMDbID,
		Title:       m.Title,
		MediaType:   core.MediaTypeMovie,
		Year:        extractYear(m.ReleaseDate),
		Overview:    m.Overview,
		PosterURL:   posterURL(m.PosterPath),
		BackdropURL: posterURL(m.BackdropPath),
		Rating:      float64(m.VoteAverage),
		Runtime:     m.Runtime,
		Status:      m.Status,
		Tagline:     m.Tagline,
	}
	for _, g := range m.Genres {
		media.Genres = append(media.Genres, g.Name)
	}
	return media
}

func convertTVResult(r tmdb.TVShowResult) core.Media {
	return core.Media{
		ID:        fmt.Sprintf("tmdb-%d", r.ID),
		TMDBID:    int(r.ID),
		Title:     r.Name,
		MediaType: core.MediaTypeSeries,
		Year:      extractYear(r.FirstAirDate),
		Overview:  r.Overview,
		PosterURL: posterURL(r.PosterPath),
		Rating:    float64(r.VoteAverage),
	}
}

func convertTVDetails(tv *tmdb.TVDetails) core.Media {
	media := core.Media{
		ID:          fmt.Sprintf("tmdb-%d", tv.ID),
		TMDBID:      int(tv.ID),
		Title:       tv.Name,
		MediaType:   core.MediaTypeSeries,
		Year:        extractYear(tv.FirstAirDate),
		Overview:    tv.Overview,
		PosterURL:   posterURL(tv.PosterPath),
		BackdropURL: posterURL(tv.BackdropPath),
		Rating:      float64(tv.VoteAverage),
		Status:      tv.Status,
	}
	for _, g := range tv.Genres {
		media.Genres = append(media.Genres, g.Name)
	}
	return media
}

func posterURL(path string) string {
	if path == "" {
		return ""
	}
	return "https://image.tmdb.org/t/p/w500" + path
}

func extractYear(date string) int {
	if len(date) >= 4 {
		y, err := strconv.Atoi(date[:4])
		if err == nil {
			return y
		}
	}
	return 0
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func ifType(t string) core.MediaType {
	if t == "tv" {
		return core.MediaTypeSeries
	}
	return core.MediaTypeMovie
}
