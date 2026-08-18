package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/cam/cine-cli/internal/core"
)

const FlixHQBaseURL = "https://flixhq.to"
const FlixHQAjaxURL = "https://flixhq.to/ajax"
const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type FlixHQ struct {
	client *http.Client
}

func NewFlixHQ() *FlixHQ {
	return &FlixHQ{
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    5,
				IdleConnTimeout: 30 * time.Second,
			},
		},
	}
}

func (f *FlixHQ) Name() string  { return "flixhq" }
func (f *FlixHQ) Priority() int { return 1 }

func (f *FlixHQ) Search(ctx context.Context, query string) ([]core.MediaRef, error) {
	searchURL := fmt.Sprintf("%s/search/%s", FlixHQBaseURL, url.PathEscape(strings.ReplaceAll(query, " ", "-")))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	f.setHeaders(req)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("flixhq search: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []core.MediaRef
	doc.Find("div.flw-item").Each(func(i int, s *goquery.Selection) {
		poster := s.Find("div.film-poster")
		href, _ := poster.Find("a").Attr("href")
		if href == "" {
			return
		}
		detail := s.Find("div.film-detail")
		title := strings.TrimSpace(detail.Find("h2.film-name a").Text())
		info := strings.TrimSpace(detail.Find("div.fd-infor").Text())

		mediaType := core.MediaTypeMovie
		if strings.Contains(info, "TV") || strings.Contains(info, "Series") {
			mediaType = core.MediaTypeSeries
		}

		year := ""
		s.Find("span").Each(func(j int, span *goquery.Selection) {
			t := strings.TrimSpace(span.Text())
			if len(t) == 4 && isNumeric(t) {
				year = t
			}
		})

		img, _ := poster.Find("img").Attr("data-src")
		if img == "" {
			img, _ = poster.Find("img").Attr("src")
		}

		results = append(results, core.MediaRef{
			ProviderName: "flixhq",
			ProviderID:   href,
			Title:        title,
			MediaType:    mediaType,
			Year:         year,
			Poster:       img,
			URL:          FlixHQBaseURL + href,
		})
	})
	return results, nil
}

func (f *FlixHQ) GetStream(ctx context.Context, ref core.MediaRef) (*core.Stream, error) {
	if ref.MediaType == core.MediaTypeSeries {
		return f.getSeriesStream(ctx, ref)
	}
	return f.getMovieStream(ctx, ref)
}

func (f *FlixHQ) getMovieStream(ctx context.Context, ref core.MediaRef) (*core.Stream, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", ref.URL, nil)
	if err != nil {
		return nil, err
	}
	f.setHeaders(req)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch movie: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	linkID, _ := doc.Find("a[data-linkid]").First().Attr("data-linkid")
	if linkID == "" {
		linkID = extractMovieID(ref.URL)
		if linkID == "" {
			// try finding via the watch button
			watchURL, _ := doc.Find("a[href*='watch-']").First().Attr("href")
			if watchURL != "" {
				return f.getMovieStream(ctx, core.MediaRef{
					ProviderName: "flixhq",
					URL:          FlixHQBaseURL + watchURL,
					ProviderID:   watchURL,
				})
			}
			return nil, fmt.Errorf("no stream link found")
		}
	}
	return f.resolveLink(ctx, linkID, ref.URL)
}

func (f *FlixHQ) getSeriesStream(ctx context.Context, ref core.MediaRef) (*core.Stream, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", ref.URL, nil)
	if err != nil {
		return nil, err
	}
	f.setHeaders(req)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch series: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	epURL, _ := doc.Find("a[href*='episode']").First().Attr("href")
	if epURL == "" {
		return nil, fmt.Errorf("no episodes found")
	}

	fullURL := FlixHQBaseURL + epURL
	req2, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	f.setHeaders(req2)

	resp2, err := f.client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("fetch episode: %w", err)
	}
	defer resp2.Body.Close()

	doc2, err := goquery.NewDocumentFromReader(resp2.Body)
	if err != nil {
		return nil, err
	}

	linkID, _ := doc2.Find("a[data-linkid]").First().Attr("data-linkid")
	if linkID == "" {
		return nil, fmt.Errorf("no stream link on episode page")
	}
	return f.resolveLink(ctx, linkID, fullURL)
}

func (f *FlixHQ) resolveLink(ctx context.Context, linkID, referer string) (*core.Stream, error) {
	srcURL := fmt.Sprintf("%s/episode/servers/%s", FlixHQAjaxURL, linkID)
	req, err := http.NewRequestWithContext(ctx, "GET", srcURL, nil)
	if err != nil {
		return nil, err
	}
	f.setHeaders(req)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", referer)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch servers: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	serverDoc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	var streamURL string
	var subtitles []core.Subtitle

	serverDoc.Find("a[data-id]").Each(func(i int, s *goquery.Selection) {
		if streamURL != "" {
			return
		}
		dataID, ok := s.Attr("data-id")
		if !ok {
			return
		}
		embedURL := fmt.Sprintf("%s/episode/sources/%s", FlixHQAjaxURL, dataID)
		er, err := f.client.Get(embedURL)
		if err != nil {
			return
		}
		defer er.Body.Close()
		var ed struct {
			Link string `json:"link"`
		}
		if json.NewDecoder(er.Body).Decode(&ed) != nil {
			return
		}
		streamURL = ed.Link
	})

	if streamURL == "" {
		return nil, fmt.Errorf("could not resolve embed URL")
	}

	return f.resolveEmbedStream(ctx, streamURL, referer, subtitles)
}

func (f *FlixHQ) resolveEmbedStream(ctx context.Context, embedURL, referer string, subs []core.Subtitle) (*core.Stream, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", embedURL, nil)
	if err != nil {
		return nil, err
	}
	f.setHeaders(req)
	req.Header.Set("Referer", referer)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch embed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(body)

	if m3u8 := extractM3U8URL(html); m3u8 != "" {
		return &core.Stream{
			URL:       m3u8,
			Referer:   embedURL,
			UserAgent: userAgent,
			IsM3U8:    true,
			Provider:  "flixhq",
		}, nil
	}

	if m3u8, extraSubs := extractMegacloud(html); m3u8 != "" {
		return &core.Stream{
			URL:       m3u8,
			Referer:   embedURL,
			UserAgent: userAgent,
			Subtitles: append(subs, extraSubs...),
			IsM3U8:    true,
			Provider:  "flixhq",
		}, nil
	}

	return &core.Stream{
		URL:      embedURL,
		Referer:  referer,
		Provider: "flixhq",
		IsM3U8:   strings.Contains(embedURL, ".m3u8"),
	}, nil
}

func (f *FlixHQ) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
}

var m3u8Regex = regexp.MustCompile(`https?://[^"'\s]+\.m3u8[^"'\s]*`)

func extractM3U8URL(html string) string {
	matches := m3u8Regex.FindString(html)
	return matches
}

var megaConfigRegex = regexp.MustCompile(`sources:\s*\[\{file:\s*"([^"]+)"`)
var megaFileRegex = regexp.MustCompile(`file:\s*"([^"]+\.m3u8[^"]*)"`)

func extractMegacloud(html string) (string, []core.Subtitle) {
	if m := megaConfigRegex.FindStringSubmatch(html); len(m) > 1 {
		return m[1], extractSubtitles(html)
	}
	if m := megaFileRegex.FindStringSubmatch(html); len(m) > 1 {
		return m[1], extractSubtitles(html)
	}
	return "", nil
}

var subRegex = regexp.MustCompile(`(?:subtitle|sub):\s*"([^"]+)"(?:\s*,\s*"([^"]+)")?`)

func extractSubtitles(html string) []core.Subtitle {
	var subs []core.Subtitle
	matches := subRegex.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 && m[1] != "" {
			lang := "en"
			if len(m) > 2 && m[2] != "" {
				lang = m[2]
			}
			subs = append(subs, core.Subtitle{URL: m[1], Lang: lang})
		}
	}
	return subs
}

func extractMovieID(s string) string {
	parts := strings.Split(s, "-")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
