package player

import (
	"net/url"

	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// normalizeStream expands CDN proxy URLs that carry host/headers query params
// and returns (playURL, header map suitable for mpv --http-header-fields).
func normalizeStream(raw, referer, ua string) (string, map[string]string) {
	headers := map[string]string{}
	if ua == "" {
		ua = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	}
	headers["User-Agent"] = ua

	u, err := url.Parse(raw)
	if err != nil {
		if referer != "" {
			headers["Referer"] = referer
			headers["Origin"] = originOf(referer)
		}
		return raw, headers
	}

	q := u.Query()

	// headers= JSON blob (url-encoded)
	if hraw := q.Get("headers"); hraw != "" && hraw != "{}" {
		var extra map[string]string
		if json.Unmarshal([]byte(hraw), &extra) == nil {
			for k, v := range extra {
				if v != "" {
					headers[k] = v
				}
			}
		}
	}

	// host= real CDN origin — use as Referer/Origin and Host-related hints
	if hostParam := q.Get("host"); hostParam != "" {
		hostParam, _ = url.QueryUnescape(hostParam)
		if !strings.HasPrefix(hostParam, "http") {
			hostParam = "https://" + hostParam
		}
		if referer == "" {
			referer = hostParam + "/"
		}
		headers["Referer"] = referer
		headers["Origin"] = originOf(hostParam)
		// Some gateways want the Host header of the real CDN
		if hu, err := url.Parse(hostParam); err == nil && hu.Host != "" {
			headers["Host"] = hu.Host
		}
	}

	if referer != "" {
		if _, ok := headers["Referer"]; !ok {
			headers["Referer"] = referer
		}
		if _, ok := headers["Origin"]; !ok {
			headers["Origin"] = originOf(referer)
		}
	} else if u.Scheme != "" && u.Host != "" {
		headers["Referer"] = u.Scheme + "://" + u.Host + "/"
		headers["Origin"] = u.Scheme + "://" + u.Host
	}

	// Common CDN preconditions
	headers["Accept"] = "*/*"
	headers["Accept-Language"] = "en-US,en;q=0.9"

	return raw, headers
}

func headerFieldsArg(headers map[string]string) string {
	var parts []string
	for k, v := range headers {
		// mpv sets User-Agent separately; Host is special — skip both here if desired
		if strings.EqualFold(k, "User-Agent") {
			continue
		}
		parts = append(parts, k+": "+v)
	}
	return strings.Join(parts, ",")
}

// preflightStream does a ranged GET to verify the URL is playable before opening a window.
// Returns nil if looks OK (2xx/3xx), error otherwise.
func preflightStream(ctx context.Context, streamURL string, headers map[string]string) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", streamURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", "bytes=0-1023")
	for k, v := range headers {
		if strings.EqualFold(k, "Host") {
			// Go's client sets Host from URL; use req.Host
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		return nil
	case resp.StatusCode == 403:
		return fmt.Errorf("CDN bloqueó el stream (403 Forbidden) — prueba otra fuente")
	case resp.StatusCode == 428:
		return fmt.Errorf("CDN exige headers (428) — esta fuente no es reproducible en mpv")
	case resp.StatusCode == 404:
		return fmt.Errorf("stream no encontrado (404)")
	default:
		return fmt.Errorf("stream HTTP %d", resp.StatusCode)
	}
}

func originOf(referer string) string {
	u, err := url.Parse(referer)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return referer
	}
	return u.Scheme + "://" + u.Host
}
