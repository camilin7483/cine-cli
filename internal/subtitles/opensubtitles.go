package subtitles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

// Client talks to the OpenSubtitles REST API (api.opensubtitles.com).
// Requires a free API key: https://www.opensubtitles.com/en/consumers
type Client struct {
	apiKey string
	lang   string
	cache  string
	http   *http.Client
}

func New(apiKey, lang, cacheDir string) *Client {
	if lang == "" {
		lang = "en"
	}
	_ = os.MkdirAll(cacheDir, 0755)
	return &Client{
		apiKey: apiKey,
		lang:   lang,
		cache:  cacheDir,
		http:   &http.Client{Timeout: 12 * time.Second},
	}
}

type searchResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Language      string `json:"language"`
			DownloadCount int    `json:"download_count"`
			Files         []struct {
				FileID   int    `json:"file_id"`
				FileName string `json:"file_name"`
			} `json:"files"`
			FeatureDetails struct {
				Title string `json:"title"`
			} `json:"feature_details"`
		} `json:"attributes"`
	} `json:"data"`
}

type downloadResponse struct {
	Link string `json:"link"`
}

// Find returns subtitle entries for a TMDB id (movie or series episode).
func (c *Client) Find(ctx context.Context, tmdbID int, mediaType core.MediaType, season, episode int) ([]core.Subtitle, error) {
	if c.apiKey == "" {
		return nil, nil
	}

	q := url.Values{}
	q.Set("tmdb_id", strconv.Itoa(tmdbID))
	q.Set("languages", c.lang)
	if mediaType == core.MediaTypeSeries {
		q.Set("type", "episode")
		if season > 0 {
			q.Set("season_number", strconv.Itoa(season))
		}
		if episode > 0 {
			q.Set("episode_number", strconv.Itoa(episode))
		}
	} else {
		q.Set("type", "movie")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.opensubtitles.com/api/v1/subtitles?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("opensubtitles search %d: %s", resp.StatusCode, string(body))
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	if len(sr.Data) == 0 {
		return nil, nil
	}

	// Take top 2 by download count order (API usually sorts)
	var out []core.Subtitle
	for i, item := range sr.Data {
		if i >= 2 {
			break
		}
		if len(item.Attributes.Files) == 0 {
			continue
		}
		fileID := item.Attributes.Files[0].FileID
		path, err := c.downloadFile(ctx, fileID, item.Attributes.Files[0].FileName)
		if err != nil {
			continue
		}
		out = append(out, core.Subtitle{
			URL:  path,
			Lang: item.Attributes.Language,
		})
	}
	return out, nil
}

func (c *Client) downloadFile(ctx context.Context, fileID int, name string) (string, error) {
	body := fmt.Sprintf(`{"file_id":%d}`, fileID)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.opensubtitles.com/api/v1/download", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("opensubtitles download %d: %s", resp.StatusCode, string(b))
	}

	var dr downloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return "", err
	}
	if dr.Link == "" {
		return "", fmt.Errorf("empty download link")
	}

	// Fetch the actual subtitle content
	sreq, err := http.NewRequestWithContext(ctx, "GET", dr.Link, nil)
	if err != nil {
		return "", err
	}
	sresp, err := c.http.Do(sreq)
	if err != nil {
		return "", err
	}
	defer sresp.Body.Close()
	if sresp.StatusCode != 200 {
		return "", fmt.Errorf("subtitle file HTTP %d", sresp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(sresp.Body, 2<<20))
	if err != nil {
		return "", err
	}

	safe := sanitize(name)
	if safe == "" {
		safe = fmt.Sprintf("sub-%d.srt", fileID)
	}
	path := filepath.Join(c.cache, safe)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("User-Agent", "cine-cli v0.5")
	req.Header.Set("Accept", "application/json")
}

func sanitize(name string) string {
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	return name
}
