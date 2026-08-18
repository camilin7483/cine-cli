package resolver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const httpUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type HTTP struct {
	client *http.Client
}

func NewHTTP() *HTTP {
	return &HTTP{
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    5,
				IdleConnTimeout: 30 * time.Second,
			},
		},
	}
}

func (h *HTTP) Name() string    { return "http" }
func (h *HTTP) Available() bool { return true }

func (h *HTTP) Resolve(ctx context.Context, embedURL string) (*StreamResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", embedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, err
	}

	html := string(body)

	if m3u8 := extractM3U8FromString(html); m3u8 != "" {
		return &StreamResult{
			URL:       m3u8,
			Referer:   embedURL,
			UserAgent: httpUA,
			Provider:  "http",
		}, nil
	}

	iframeSrc := extractIframeSrc(html)
	if iframeSrc != "" {
		return h.resolveIframe(ctx, iframeSrc, embedURL)
	}

	return nil, fmt.Errorf("http: no stream URL found")
}

func (h *HTTP) resolveIframe(ctx context.Context, iframeSrc, referer string) (*StreamResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", iframeSrc, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpUA)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, err
	}

	html := string(body)

	if m3u8 := extractM3U8FromString(html); m3u8 != "" {
		return &StreamResult{
			URL:       m3u8,
			Referer:   iframeSrc,
			UserAgent: httpUA,
			Provider:  "http",
		}, nil
	}

	return nil, fmt.Errorf("http iframe: no stream")
}
