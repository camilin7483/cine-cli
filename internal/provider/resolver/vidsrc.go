package resolver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const vidsrcUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// VidsrcResolver implements the full vidsrc.to → vsembed.ru → cloudorchestranova
// → prorcp → m3u8 chain using pure HTTP (no browser needed).
type VidsrcResolver struct {
	client *http.Client
}

func NewVidsrcResolver() *VidsrcResolver {
	return &VidsrcResolver{
		client: &http.Client{
			Timeout: 12 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
	}
}

func (v *VidsrcResolver) Name() string    { return "vidsrc-chain" }
func (v *VidsrcResolver) Available() bool { return true }

func (v *VidsrcResolver) Resolve(ctx context.Context, embedURL string) (*StreamResult, error) {
	// Step 1: Fetch the embed page and find the vsembed.ru iframe
	body, _, err := v.fetch(ctx, embedURL, "")
	if err != nil {
		return nil, fmt.Errorf("step1: %w", err)
	}

	vsembedURL := findIframeURL(body, "vsembed.ru", "vidsrcme.ru")
	if vsembedURL == "" {
		return nil, fmt.Errorf("no vsembed iframe found")
	}
	vsembedURL = resolveURL(vsembedURL, embedURL)

	// Step 2: Fetch vsembed.ru and find the cloudorchestranova iframe
	body, _, err = v.fetch(ctx, vsembedURL, embedURL)
	if err != nil {
		return nil, fmt.Errorf("step2: %w", err)
	}

	cloudURL := findIframeURL(body, "cloudorchestranova.com", "cloudnestra.com")
	if cloudURL == "" {
		// Try extracting from data-hash attributes
		cloudURL = findCloudURLFromHash(body)
	}
	if cloudURL == "" {
		return nil, fmt.Errorf("no cloud iframe found")
	}
	cloudURL = resolveURL(cloudURL, vsembedURL)

	// Step 3: cloud page (legacy /rcp/ or modern /embed/?vs=)
	body, _, err = v.fetch(ctx, cloudURL, vsembedURL)
	if err != nil {
		return nil, fmt.Errorf("step3: %w", err)
	}
	if m3u8URLs := findAllM3U8WithURL(body); len(m3u8URLs) > 0 {
		return &StreamResult{URL: m3u8URLs[0], Referer: cloudURL, UserAgent: vidsrcUA, Provider: "vidsrc-chain"}, nil
	}
	if pu := findPlayerURL(body); pu != "" {
		pu = resolveURL(pu, cloudURL)
		if body2, _, e2 := v.fetch(ctx, pu, cloudURL); e2 == nil {
			if m3u8URLs := findAllM3U8WithURL(body2); len(m3u8URLs) > 0 {
				return &StreamResult{URL: m3u8URLs[0], Referer: pu, UserAgent: vidsrcUA, Provider: "vidsrc-chain"}, nil
			}
			body = body2
		}
	}

	prorcpPath := findProrcpURL(body)
	if prorcpPath == "" {
		return nil, fmt.Errorf("no stream path found on cloud page")
	}

	cloudHost := extractHost(cloudURL)
	prorcpURL := cloudHost + prorcpPath

	// Step 4: Fetch /prorcp/<base64> and find m3u8 URLs
	body, _, err = v.fetch(ctx, prorcpURL, cloudURL)
	if err != nil {
		return nil, fmt.Errorf("step4: %w", err)
	}

	m3u8URLs := findAllM3U8WithURL(body)
	if len(m3u8URLs) == 0 {
		return nil, fmt.Errorf("no m3u8 URLs found in prorcp")
	}

	// Step 5: For each m3u8 URL, get the token and replace __TOKEN__
	for _, m3u8 := range m3u8URLs {
		result, err := v.resolveWithToken(ctx, m3u8, prorcpURL)
		if err == nil && result != nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("could not resolve token for any m3u8 URL")
}

func (v *VidsrcResolver) resolveWithToken(ctx context.Context, m3u8URL, referer string) (*StreamResult, error) {
	// Check if the URL needs a token
	if !strings.Contains(m3u8URL, "__TOKEN") {
		return &StreamResult{
			URL:       m3u8URL,
			Referer:   referer,
			UserAgent: vidsrcUA,
			Provider:  "vidsrc-chain",
		}, nil
	}

	// Extract host from the m3u8 URL
	parsed, err := url.Parse(m3u8URL)
	if err != nil {
		return nil, err
	}
	host := parsed.Scheme + "://" + parsed.Host

	// Fetch the token from generate.php
	tokenBody, _, err := v.fetch(ctx, host+"/generate.php", referer)
	if err != nil {
		return nil, fmt.Errorf("token fetch: %w", err)
	}

	token := strings.TrimSpace(tokenBody)
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}

	// Replace __TOKEN__ and __TOKENPG__ with the real token
	finalURL := strings.ReplaceAll(m3u8URL, "__TOKEN__", token)
	finalURL = strings.ReplaceAll(finalURL, "__TOKENPG__", token)

	return &StreamResult{
		URL:       finalURL,
		Referer:   referer,
		UserAgent: vidsrcUA,
		Provider:  "vidsrc-chain",
	}, nil
}

func (v *VidsrcResolver) fetch(ctx context.Context, targetURL, referer string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", vidsrcUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", "", err
	}

	return string(body), targetURL, nil
}

var iframeSrcRe = regexp.MustCompile(`<iframe[^>]+src=["']([^"']+)["']`)
var prorcpRe = regexp.MustCompile(`["']/prorcp/([^"']+)["']`)
var dataHashRe = regexp.MustCompile(`data-hash="([^"]+)"`)
var m3u8FullRe = regexp.MustCompile(`https?://[^"'\s<>]+\.m3u8[^"'\s<>]*`)

func findIframeURL(html string, domains ...string) string {
	matches := iframeSrcRe.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 {
			src := m[1]
			for _, domain := range domains {
				if strings.Contains(src, domain) {
					return src
				}
			}
		}
	}
	return ""
}

func findCloudURLFromHash(html string) string {
	match := dataHashRe.FindStringSubmatch(html)
	if len(match) > 1 {
		return "https://cloudorchestranova.com/rcp/" + match[1]
	}
	return ""
}

func findProrcpURL(html string) string {
	match := prorcpRe.FindStringSubmatch(html)
	if len(match) > 1 {
		return "/prorcp/" + match[1]
	}
	return ""
}

func findAllM3U8WithURL(html string) []string {
	matches := m3u8FullRe.FindAllString(html, -1)
	var result []string
	seen := make(map[string]bool)
	for _, m := range matches {
		m = cleanM3U8(m)
		if !seen[m] && strings.HasPrefix(m, "http") {
			seen[m] = true
			result = append(result, m)
		}
	}
	return result
}

func findPlayerURL(html string) string {
	re := regexp.MustCompile(`"playerUrl"\s*:\s*"([^"]+)"`)
	m := re.FindStringSubmatch(html)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func resolveURL(raw, base string) string {
	if strings.HasPrefix(raw, "http") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		parsed, _ := url.Parse(base)
		return parsed.Scheme + ":" + raw
	}
	if strings.HasPrefix(raw, "/") {
		parsed, _ := url.Parse(base)
		return parsed.Scheme + "://" + parsed.Host + raw
	}
	return raw
}

func extractHost(fullURL string) string {
	parsed, err := url.Parse(fullURL)
	if err != nil {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
