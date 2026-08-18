package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const chromeUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type Browser struct {
	timeout time.Duration
}

func NewBrowser(timeout time.Duration) *Browser {
	return &Browser{timeout: timeout}
}

func (b *Browser) Name() string { return "browser" }

func (b *Browser) Available() bool {
	for _, name := range []string{"google-chrome-stable", "chromium", "chromium-browser", "brave-browser"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

// Resolve loads the embed URL, captures the iframe chain from network requests,
// then navigates directly to the final video page to capture the m3u8.
func (b *Browser) Resolve(ctx context.Context, embedURL string) (*StreamResult, error) {
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, b.allocOpts()...)
	defer allocCancel()

	tabCtx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, b.timeout)
	defer timeoutCancel()

	var mu sync.Mutex
	var foundM3U8 string
	var foundSubs []SubData
	var cloudURL string
	var allNetURLs []string

	// Capture ALL network requests to find:
	// 1. The cloudorchestranova/cloudnestra iframe URL (Document requests)
	// 2. The m3u8 URL (Media/XHR requests)
	chromedp.ListenTarget(timeoutCtx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			u := ev.Request.URL
			mu.Lock()
			allNetURLs = append(allNetURLs, u)
			mu.Unlock()

			// Check for m3u8/mp4
			if isStreamURL(u) {
				mu.Lock()
				if foundM3U8 == "" {
					foundM3U8 = u
				}
				mu.Unlock()
			}

			// Capture cloudorchestranova/cloudnestra iframe URLs
			if (strings.Contains(u, "cloudorchestranova.com") || strings.Contains(u, "cloudnestra.com")) &&
				strings.Contains(u, "/rcp/") && cloudURL == "" {
				mu.Lock()
				cloudURL = u
				mu.Unlock()
			}

		case *network.EventResponseReceived:
			u := ev.Response.URL
			if isStreamURL(u) {
				mu.Lock()
				if foundM3U8 == "" {
					foundM3U8 = u
				}
				mu.Unlock()
			}
			lu := strings.ToLower(u)
			if strings.HasSuffix(lu, ".vtt") || strings.HasSuffix(lu, ".srt") {
				mu.Lock()
				foundSubs = append(foundSubs, SubData{URL: u, Lang: ""})
				mu.Unlock()
			}
		}
	})

	// Step 1: Load the embed page and wait for iframes to be created
	err := chromedp.Run(timeoutCtx,
		network.Enable(),
		chromedp.Navigate(embedURL),
		chromedp.Sleep(5*time.Second),
	)

	mu.Lock()
	capturedCloudURL := cloudURL
	capturedM3U8 := foundM3U8
	mu.Unlock()

	if capturedM3U8 != "" {
		return &StreamResult{
			URL:       capturedM3U8,
			Referer:   embedURL,
			UserAgent: chromeUA,
			Subtitles: foundSubs,
			Provider:  "chrome",
		}, nil
	}

	// Step 2: If we found a cloudorchestranova URL, navigate to it directly
	// This allows us to capture the m3u8 from that page's network requests
	if capturedCloudURL != "" {
		mu.Lock()
		foundM3U8 = "" // reset for the new page
		mu.Unlock()

		err = chromedp.Run(timeoutCtx,
			chromedp.Navigate(capturedCloudURL),
			chromedp.Sleep(8*time.Second),
		)

		// Try clicking play buttons
		var clicked bool
		_ = chromedp.Run(timeoutCtx, chromedp.Evaluate(`(function(){
			var sels = ['.play-button','#play','.vjs-big-play-button','button[class*="play"]','.jw-icon-display','.video-play','#playButton','.play-btn','button','[onclick*="play"]','a[href*="play"]'];
			for (var i=0; i<sels.length; i++) {
				var el = document.querySelector(sels[i]);
				if (el) { el.click(); return true; }
			}
			return false;
		})()`, &clicked))

		if clicked {
			 _ = chromedp.Run(timeoutCtx, chromedp.Sleep(5*time.Second))
		}

		mu.Lock()
		capturedM3U8 = foundM3U8
		mu.Unlock()

		if capturedM3U8 != "" {
			return &StreamResult{
				URL:       capturedM3U8,
				Referer:   capturedCloudURL,
				UserAgent: chromeUA,
				Subtitles: foundSubs,
				Provider:  "chrome",
			}, nil
		}

		// Also try extracting from DOM
		var pageInfo string
		_ = chromedp.Run(timeoutCtx, chromedp.Evaluate(`(function(){
			var data = {videos:[], bodyHTML:'', scripts:''};
			var vids = document.querySelectorAll('video source, video');
			for (var i=0; i<vids.length && i<5; i++) {
				data.videos.push(vids[i].src || '');
			}
			if (document.body) {
				data.bodyHTML = document.body.outerHTML.substring(0, 80000);
			}
			var scripts = document.querySelectorAll('script');
			for (var i=0; i<scripts.length && i<20; i++) {
				data.scripts += scripts[i].textContent.substring(0, 5000) + '\n';
			}
			return JSON.stringify(data);
		})()`, &pageInfo))

		if pageInfo != "" {
			var data struct {
				Videos   []string `json:"videos"`
				BodyHTML string   `json:"bodyHTML"`
				Scripts  string   `json:"scripts"`
			}
			if err := json.Unmarshal([]byte(pageInfo), &data); err == nil {
				for _, v := range data.Videos {
					if isStreamURL(v) {
						return &StreamResult{URL: v, Referer: capturedCloudURL, UserAgent: chromeUA, Provider: "chrome"}, nil
					}
				}
				combined := data.BodyHTML + data.Scripts
				if m3u8 := extractM3U8FromString(combined); m3u8 != "" {
					return &StreamResult{URL: m3u8, Referer: capturedCloudURL, UserAgent: chromeUA, Provider: "chrome"}, nil
				}
			}
		}
	}

	// Step 3: Try extracting iframe src from DOM and following it
	var iframeSrc string
	_ = chromedp.Run(timeoutCtx, chromedp.Evaluate(`(function(){
		var ifs = document.querySelectorAll('iframe');
		for (var i=0; i<ifs.length; i++) {
			var src = ifs[i].src || '';
			if (src && src.indexOf('cloud') !== -1) return src;
			if (src && src.indexOf('http') === 0 && src.indexOf('ad') === -1) return src;
		}
		return '';
	})()`, &iframeSrc))

	if iframeSrc != "" && iframeSrc != capturedCloudURL {
		mu.Lock()
		foundM3U8 = ""
		mu.Unlock()

		err = chromedp.Run(timeoutCtx,
			chromedp.Navigate(iframeSrc),
			chromedp.Sleep(8*time.Second),
		)

		mu.Lock()
		capturedM3U8 = foundM3U8
		mu.Unlock()

		if capturedM3U8 != "" {
			return &StreamResult{
				URL:       capturedM3U8,
				Referer:   iframeSrc,
				UserAgent: chromeUA,
				Provider:  "chrome",
			}, nil
		}
	}

	_ = allNetURLs // keep for debugging

	if err != nil {
		return nil, fmt.Errorf("browser: %w", err)
	}
	return nil, fmt.Errorf("browser: no stream found for %s", embedURL)
}

func (b *Browser) allocOpts() []chromedp.ExecAllocatorOption {
	return []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-features", "TranslateUI,VizDisplayCompositor"),
		chromedp.UserAgent(chromeUA),
		chromedp.WindowSize(1920, 1080),
	}
}

func isStreamURL(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, ".m3u8") ||
		(strings.Contains(lower, ".mp4") && !strings.Contains(lower, ".html"))
}


var m3u8Re = regexp.MustCompile(`https?://[^"'\s<>]+\.m3u8[^"'\s<>]*`)
var iframeRe = regexp.MustCompile(`<iframe[^>]+src=["']([^"']+)["']`)

func extractM3U8FromString(s string) string {
	matches := m3u8Re.FindString(s)
	if matches != "" {
		return cleanM3U8(matches)
	}

	idx := strings.Index(s, ".m3u8")
	if idx < 0 {
		return ""
	}
	start := idx
	for start > 0 && s[start] != '"' && s[start] != '\'' && s[start] != ' ' && s[start] != '\n' {
		start--
	}
	if s[start] == '"' || s[start] == '\'' {
		start++
	}
	end := idx + 5
	for end < len(s) && s[end] != '"' && s[end] != '\'' && s[end] != ' ' && s[end] != '\n' && s[end] != '&' {
		end++
	}
	return cleanM3U8(s[start:end])
}

func cleanM3U8(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\\/", "/")
	s = strings.TrimRight(s, "\\")
	return s
}

func extractIframeSrc(html string) string {
	matches := iframeRe.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 {
			src := m[1]
			if strings.Contains(src, "ads") || strings.Contains(src, "tracking") || strings.Contains(src, "histats") {
				continue
			}
			if strings.HasPrefix(src, "http") || strings.HasPrefix(src, "//") || strings.HasPrefix(src, "/") {
				return src
			}
		}
	}
	return ""
}
