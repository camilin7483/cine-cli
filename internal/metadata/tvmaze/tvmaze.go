package tvmaze

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

const baseURL = "https://api.tvmaze.com"

type Provider struct {
	client *http.Client
	cache  map[string]*core.Media
}

func New() *Provider {
	return &Provider{
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
		},
		cache: make(map[string]*core.Media),
	}
}

type searchResult struct {
	Score float64    `json:"score"`
	Show  tvmazeShow `json:"show"`
}

type tvmazeShow struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Premiered string `json:"premiered"`
	Summary   string `json:"summary"`
	Image     *struct {
		Medium   string `json:"medium"`
		Original string `json:"original"`
	} `json:"image"`
	Rating struct {
		Average *float64 `json:"average"`
	} `json:"rating"`
	Genres  []string `json:"genres"`
	Runtime int      `json:"runtime"`
	Status  string   `json:"status"`
	Network *struct {
		Name string `json:"name"`
	} `json:"network"`
	WebChannel *struct {
		Name string `json:"name"`
	} `json:"webChannel"`
	Externals struct {
		TVRage  *int    `json:"tvrage"`
		TheTVDB *int    `json:"thetvdb"`
		IMDB    *string `json:"imdb"`
	} `json:"externals"`
	OfficialSite string `json:"officialSite"`
}

type tvmazeEpisode struct {
	ID      int    `json:"id"`
	Season  int    `json:"season"`
	Number  int    `json:"number"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Image   *struct {
		Medium   string `json:"medium"`
		Original string `json:"original"`
	} `json:"image"`
	Airdate string `json:"airdate"`
	Runtime int    `json:"runtime"`
}

func (p *Provider) Search(ctx context.Context, filter core.SearchFilter) ([]core.Media, error) {
	reqURL := fmt.Sprintf("%s/search/shows?q=%s", baseURL, url.QueryEscape(filter.Query))
	body, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("tvmaze search: %w", err)
	}

	var results []searchResult
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("tvmaze decode: %w", err)
	}

	if filter.MediaType == core.MediaTypeMovie {
		return nil, nil
	}

	var media []core.Media
	for _, r := range results {
		m := convertShow(r.Show)
		if m.Title != "" {
			media = append(media, m)
			p.cache[strconv.Itoa(r.Show.ID)] = &m
		}
	}
	return media, nil
}

func (p *Provider) GetDetails(ctx context.Context, id int, mediaType core.MediaType) (*core.Media, error) {
	if mediaType == core.MediaTypeMovie {
		return nil, fmt.Errorf("tvmaze: movies not supported")
	}

	cacheKey := strconv.Itoa(id)
	if cached, ok := p.cache[cacheKey]; ok {
		return cached, nil
	}

	reqURL := fmt.Sprintf("%s/shows/%d", baseURL, id)
	body, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("tvmaze details: %w", err)
	}

	var show tvmazeShow
	if err := json.Unmarshal(body, &show); err != nil {
		return nil, fmt.Errorf("tvmaze decode details: %w", err)
	}

	m := convertShow(show)
	p.cache[cacheKey] = &m
	return &m, nil
}

func (p *Provider) GetSeasons(ctx context.Context, seriesID int) ([]core.Season, error) {
	reqURL := fmt.Sprintf("%s/shows/%d/seasons", baseURL, seriesID)
	body, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("tvmaze seasons: %w", err)
	}

	var seasons []struct {
		ID           int    `json:"id"`
		Number       int    `json:"number"`
		Name         string `json:"name"`
		EpisodeOrder int    `json:"episodeOrder"`
		PremiereDate string `json:"premiereDate"`
		EndDate      string `json:"endDate"`
		Network      *struct {
			Name string `json:"name"`
		} `json:"network"`
		Image *struct {
			Medium   string `json:"medium"`
			Original string `json:"original"`
		} `json:"image"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(body, &seasons); err != nil {
		return nil, fmt.Errorf("tvmaze decode seasons: %w", err)
	}

	result := make([]core.Season, 0)
	for _, s := range seasons {
		poster := ""
		if s.Image != nil {
			poster = s.Image.Original
		}
		result = append(result, core.Season{
			SeasonNumber: s.Number,
			Name:         s.Name,
			EpisodeCount: s.EpisodeOrder,
			PosterURL:    poster,
			Overview:     stripHTML(s.Summary),
		})
	}
	return result, nil
}

func (p *Provider) GetEpisodes(ctx context.Context, seriesID int, seasonNum int) ([]core.Episode, error) {
	reqURL := fmt.Sprintf("%s/shows/%d/episodes", baseURL, seriesID)
	body, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("tvmaze episodes: %w", err)
	}

	var episodes []tvmazeEpisode
	if err := json.Unmarshal(body, &episodes); err != nil {
		return nil, fmt.Errorf("tvmaze decode episodes: %w", err)
	}

	result := make([]core.Episode, 0)
	for _, e := range episodes {
		if e.Season != seasonNum {
			continue
		}
		still := ""
		if e.Image != nil {
			still = e.Image.Original
		}
		result = append(result, core.Episode{
			EpisodeNumber: e.Number,
			SeasonNumber:  e.Season,
			Name:          e.Name,
			Overview:      stripHTML(e.Summary),
			StillURL:      still,
			AirDate:       e.Airdate,
		})
	}
	return result, nil
}

func (p *Provider) GetTrending(ctx context.Context, mediaType core.MediaType, page int) ([]core.Media, error) {
	if mediaType == core.MediaTypeMovie {
		return nil, nil
	}

	y, m, d := time.Now().Date()
	dateStr := fmt.Sprintf("%d-%02d-%02d", y, m, d)
	reqURL := fmt.Sprintf("%s/schedule?country=US&date=%s", baseURL, dateStr)
	body, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("tvmaze schedule: %w", err)
	}

	var schedule []struct {
		ID       int        `json:"id"`
		Name     string     `json:"name"`
		Season   int        `json:"season"`
		Number   int        `json:"number"`
		Airdate  string     `json:"airdate"`
		AirTime  string     `json:"airtime"`
		Airstamp string     `json:"airstamp"`
		Runtime  int        `json:"runtime"`
		Summary  string     `json:"summary"`
		Show     tvmazeShow `json:"show"`
		Image    *struct {
			Medium   string `json:"medium"`
			Original string `json:"original"`
		} `json:"image"`
	}
	if err := json.Unmarshal(body, &schedule); err != nil {
		return nil, fmt.Errorf("tvmaze decode schedule: %w", err)
	}

	seen := make(map[int]bool)
	var results []core.Media
	for _, s := range schedule {
		if seen[s.Show.ID] {
			continue
		}
		seen[s.Show.ID] = true
		m := convertShow(s.Show)
		if m.Title != "" {
			results = append(results, m)
		}
	}

	return results, nil
}

func (p *Provider) GetRecommendations(ctx context.Context, id int, mediaType core.MediaType) ([]core.Media, error) {
	return nil, nil
}

func (p *Provider) get(ctx context.Context, reqURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cine-cli/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tvmaze: unexpected status %d", resp.StatusCode)
	}

	var body []byte
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}
	return body, nil
}

func convertShow(s tvmazeShow) core.Media {
	year := 0
	if len(s.Premiered) >= 4 {
		y, err := strconv.Atoi(s.Premiered[:4])
		if err == nil {
			year = y
		}
	}

	var rating float64
	if s.Rating.Average != nil {
		rating = *s.Rating.Average
	}

	poster := ""
	if s.Image != nil {
		poster = s.Image.Original
	}

	providerIDs := map[string]string{
		"tvmaze": strconv.Itoa(s.ID),
	}

	return core.Media{
		ID:          fmt.Sprintf("tvmaze-%d", s.ID),
		Title:       s.Name,
		MediaType:   core.MediaTypeSeries,
		Year:        year,
		Overview:    stripHTML(s.Summary),
		PosterURL:   poster,
		Rating:      rating,
		Genres:      s.Genres,
		Runtime:     s.Runtime,
		Status:      s.Status,
		ProviderIDs: providerIDs,
	}
}

func stripHTML(s string) string {
	result := ""
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result += string(c)
		}
	}
	return result
}
