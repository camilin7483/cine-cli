package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// VidsrcAPI decrypts stream_urls via the official vsdec.js (Chrome) and
// attaches a short-lived JWT from the CDN /generate.php endpoint.
type VidsrcAPI struct {
	timeout time.Duration
	mu      sync.Mutex
}

func NewVidsrcAPI(timeout time.Duration) *VidsrcAPI {
	if timeout <= 0 {
		timeout = 35 * time.Second
	}
	return &VidsrcAPI{timeout: timeout}
}

func (v *VidsrcAPI) Name() string { return "vidsrc-api" }

func (v *VidsrcAPI) Available() bool {
	b := NewBrowser(5 * time.Second)
	return b.Available()
}

// ResolveAPI fetches playable m3u8 URLs for a TMDB id.
// mediaType: "movie" or "tv". For tv, season/episode are required.
func (v *VidsrcAPI) ResolveAPI(ctx context.Context, mediaType string, tmdbID, season, episode int) (*StreamResult, error) {
	if !v.Available() {
		return nil, fmt.Errorf("vidsrc-api: chrome/chromium required")
	}

	api := fmt.Sprintf("https://data.vidsrcme.ru/api.php?type=%s&tmdb=%d&stream_urls", mediaType, tmdbID)
	if mediaType == "tv" {
		api = fmt.Sprintf("https://data.vidsrcme.ru/api.php?type=tv&tmdb=%d&season=%d&episode=%d&stream_urls", tmdbID, season, episode)
	}

	urls, err := v.decryptStreamURLs(ctx, api)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("vidsrc-api: no stream urls")
	}

	// Prefer first URL; attach CDN token
	playURL, err := attachCDNToken(ctx, urls[0])
	if err != nil {
		// still return without token — preflight may fail later
		playURL = urls[0]
	}

	return &StreamResult{
		URL:       playURL,
		Referer:   "https://cloudorchestranova.com/",
		UserAgent: chromeUA,
		Provider:  "vidsrc-api",
	}, nil
}

// Resolve implements StreamResolver for embed URLs (extracts tmdb from path).
func (v *VidsrcAPI) Resolve(ctx context.Context, embedURL string) (*StreamResult, error) {
	mediaType, tmdbID, season, episode, err := parseEmbedMeta(embedURL)
	if err != nil {
		return nil, err
	}
	return v.ResolveAPI(ctx, mediaType, tmdbID, season, episode)
}

func parseEmbedMeta(embedURL string) (mediaType string, tmdbID, season, episode int, err error) {
	u, err := url.Parse(embedURL)
	if err != nil {
		return "", 0, 0, 0, err
	}
	q := u.Query()
	path := u.Path
	mediaType = "movie"
	if strings.Contains(path, "/tv") || q.Get("type") == "tv" {
		mediaType = "tv"
	}
	// patterns: /movie/27205  /tv/1396/1/1  ?tmdb=27205
	if id := q.Get("tmdb"); id != "" {
		_, _ = fmt.Sscanf(id, "%d", &tmdbID)
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == "movie" && i+1 < len(parts) {
			_, _ = fmt.Sscanf(parts[i+1], "%d", &tmdbID)
			mediaType = "movie"
		}
		if p == "tv" && i+1 < len(parts) {
			mediaType = "tv"
			_, _ = fmt.Sscanf(parts[i+1], "%d", &tmdbID)
			if i+2 < len(parts) {
				_, _ = fmt.Sscanf(parts[i+2], "%d", &season)
			}
			if i+3 < len(parts) {
				_, _ = fmt.Sscanf(parts[i+3], "%d", &episode)
			}
		}
	}
	if season == 0 {
		_, _ = fmt.Sscanf(q.Get("season"), "%d", &season)
	}
	if episode == 0 {
		_, _ = fmt.Sscanf(q.Get("episode"), "%d", &episode)
	}
	if tmdbID == 0 {
		return "", 0, 0, 0, fmt.Errorf("cannot parse tmdb id from %s", embedURL)
	}
	if mediaType == "tv" && (season == 0 || episode == 0) {
		season, episode = 1, 1
	}
	return mediaType, tmdbID, season, episode, nil
}

func (v *VidsrcAPI) decryptStreamURLs(ctx context.Context, apiURL string) ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.UserAgent(chromeUA),
	)
	alloc, acancel := chromedp.NewExecAllocator(ctx, opts...)
	defer acancel()
	tab, tcancel := chromedp.NewContext(alloc)
	defer tcancel()

	var result string
	js := fmt.Sprintf(`(async () => {
		try {
			const load = (src) => new Promise((res, rej) => {
				const s = document.createElement('script');
				s.src = src; s.onload = res; s.onerror = rej; document.head.appendChild(s);
			});
			if (typeof window.vsFetchJSON !== 'function') {
				await load('/embed/iframe_player/assets/vsdec.js?v=1');
				await new Promise(r => setTimeout(r, 1000));
			}
			if (typeof window.vsFetchJSON !== 'function') return JSON.stringify({error:'no vsFetchJSON'});
			const j = await window.vsFetchJSON(%q);
			if (!j || j.status_code == 404 || j.status_code === '404') return JSON.stringify({error:'not found'});
			const urls = j.data && j.data.stream_urls;
			if (Array.isArray(urls)) return JSON.stringify({urls: urls});
			return JSON.stringify({error:'encrypted or empty', type: typeof urls});
		} catch (e) { return JSON.stringify({error: e.message}); }
	})()`, apiURL)

	err := chromedp.Run(tab,
		chromedp.Navigate("https://cloudorchestranova.com/embed/movie/27205"),
		chromedp.Sleep(1200*time.Millisecond),
		chromedp.Evaluate(js, &result, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("vidsrc-api chrome: %w", err)
	}

	var parsed struct {
		URLs  []string `json:"urls"`
		Error string   `json:"error"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		return nil, fmt.Errorf("vidsrc-api parse: %w (%s)", err, result)
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("vidsrc-api: %s", parsed.Error)
	}
	return parsed.URLs, nil
}

func attachCDNToken(ctx context.Context, streamURL string) (string, error) {
	u, err := url.Parse(streamURL)
	if err != nil {
		return streamURL, err
	}
	tokenURL := u.Scheme + "://" + u.Host + "/generate.php"
	req, err := http.NewRequestWithContext(ctx, "GET", tokenURL, nil)
	if err != nil {
		return streamURL, err
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Referer", "https://cloudorchestranova.com/")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return streamURL, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return streamURL, err
	}
	token := strings.TrimSpace(string(body))
	if token == "" || !strings.Contains(token, ".") {
		return streamURL, fmt.Errorf("empty token")
	}
	// Append token query param
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
