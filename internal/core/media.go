package core

import "time"

type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeSeries MediaType = "series"
)

type Media struct {
	ID            string            `json:"id"`
	TMDBID        int               `json:"tmdb_id"`
	IMDBID        string            `json:"imdb_id"`
	Title         string            `json:"title"`
	OriginalTitle string            `json:"original_title"`
	MediaType     MediaType         `json:"media_type"`
	Year          int               `json:"year"`
	Overview      string            `json:"overview"`
	PosterURL     string            `json:"poster_url"`
	BackdropURL   string            `json:"backdrop_url"`
	Rating        float64           `json:"rating"`
	Genres        []string          `json:"genres"`
	Runtime       int               `json:"runtime"`
	Status        string            `json:"status"`
	Tagline       string            `json:"tagline"`
	ProviderIDs   map[string]string `json:"provider_ids,omitempty"`
}

type MediaRef struct {
	ProviderName string    `json:"provider_name"`
	ProviderID   string    `json:"provider_id"`
	Title        string    `json:"title"`
	MediaType    MediaType `json:"media_type"`
	Year         string    `json:"year"`
	Poster       string    `json:"poster"`
	URL          string    `json:"url"`
}

type Season struct {
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	EpisodeCount int    `json:"episode_count"`
	PosterURL    string `json:"poster_url"`
	Overview     string `json:"overview"`
}

type Episode struct {
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	StillURL      string `json:"still_url"`
	AirDate       string `json:"air_date"`
}

type Stream struct {
	URL       string     `json:"url"`
	Referer   string     `json:"referer"`
	UserAgent string     `json:"user_agent"`
	Subtitles []Subtitle `json:"subtitles"`
	Quality   string     `json:"quality"`
	Provider  string     `json:"provider"`
	IsM3U8    bool       `json:"is_m3u8"`
}

type Subtitle struct {
	URL  string `json:"url"`
	Lang string `json:"lang"`
}

type Quality struct {
	URL        string `json:"url"`
	Resolution string `json:"resolution"`
	Height     int    `json:"height"`
	Bandwidth  int    `json:"bandwidth"`
	Label      string `json:"label"`
}

type HistoryEntry struct {
	ID        int64     `json:"id"`
	MediaID   string    `json:"media_id"`
	Title     string    `json:"title"`
	MediaType MediaType `json:"media_type"`
	Season    int       `json:"season"`
	Episode   int       `json:"episode"`
	Provider  string    `json:"provider"`
	StreamURL string    `json:"stream_url"`
	Position  float64   `json:"position"`
	Duration  float64   `json:"duration"`
	WatchedAt time.Time `json:"watched_at"`
}

type ContinueWatching struct {
	ID          int64     `json:"id"`
	MediaID     string    `json:"media_id"`
	Title       string    `json:"title"`
	MediaType   MediaType `json:"media_type"`
	Season      int       `json:"season"`
	Episode     int       `json:"episode"`
	Position    float64   `json:"position"`
	Duration    float64   `json:"duration"`
	Percentage  float64   `json:"percentage"`
	Provider    string    `json:"provider"`
	StreamURL   string    `json:"stream_url"`
	LastWatched time.Time `json:"last_watched"`
	Completed   bool      `json:"completed"`
}

type HistoryFilter struct {
	Query     string
	MediaType MediaType
	SortBy    string
	SortOrder string
	Limit     int
	Offset    int
}

type HistoryStats struct {
	TotalShows    int `json:"total_shows"`
	TotalMovies   int `json:"total_movies"`
	TotalEpisodes int `json:"total_episodes"`
}

type WatchlistItem struct {
	ID        int64     `json:"id"`
	MediaID   string    `json:"media_id"`
	Title     string    `json:"title"`
	MediaType MediaType `json:"media_type"`
	Season    int       `json:"season"`
	Episode   int       `json:"episode"`
	Status    string    `json:"status"`
	AddedAt   time.Time `json:"added_at"`
}

type Favorite struct {
	ID        int64     `json:"id"`
	MediaID   string    `json:"media_id"`
	Title     string    `json:"title"`
	MediaType MediaType `json:"media_type"`
	PosterURL string    `json:"poster_url"`
	AddedAt   time.Time `json:"added_at"`
}

type PlayOptions struct {
	StreamURL     string
	Referer       string
	UserAgent     string
	Subtitles     []Subtitle
	Title         string
	Player        string
	ExtraArgs     []string
	PreferredLang string
	SubsLang      string
}

type SearchFilter struct {
	Query     string
	MediaType MediaType
	Year      int
	Genre     string
	Page      int
	Language  string
}

type Recommendation struct {
	Title     string    `json:"title"`
	MediaType MediaType `json:"media_type"`
	Year      string    `json:"year"`
	Overview  string    `json:"overview"`
	TMDBID    int       `json:"tmdb_id"`
	Score     float64   `json:"score"`
	PosterURL string    `json:"poster_url"`
}
