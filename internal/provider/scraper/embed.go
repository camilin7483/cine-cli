package scraper

import (
	"context"
	"fmt"
	"time"

	"github.com/cam/cine-cli/internal/core"
	"github.com/cam/cine-cli/internal/provider/resolver"
)

type embedProvider struct {
	name     string
	priority int
	resolver *resolver.Chain
	ytdlp    *resolver.YTDlpStream
	api      *resolver.VidsrcAPI
}

func (p *embedProvider) Name() string  { return p.name }
func (p *embedProvider) Priority() int { return p.priority }

func (p *embedProvider) Search(ctx context.Context, query string) ([]core.MediaRef, error) {
	return nil, nil
}

func (p *embedProvider) buildEmbedURL(ref core.MediaRef) string {
	id := ref.ProviderID
	mediaType := ref.MediaType

	switch p.name {
	case "vidsrc":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://vidsrc.to/embed/tv/%s/%d/%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://vidsrc.to/embed/movie/%s", id)
	case "2embed":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://www.2embed.cc/embedtv/%s&s=%d&e=%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://www.2embed.cc/embed/%s", id)
	case "vidlink":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://vidlink.pro/tv/%s/%d/%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://vidlink.pro/movie/%s", id)
	case "vidsrcme":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://vidsrc.me/embed/tv?tmdb=%s&season=%d&episode=%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://vidsrc.me/embed/movie?tmdb=%s", id)
	case "superembed":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://multiembed.mov/direct.php?video_id=%s&s=%d&e=%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://multiembed.mov/direct.php?video_id=%s", id)
	case "multiem":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://multiembed.mov/embed/tv/%s/%d/%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://multiembed.mov/embed/movie/%s", id)
	case "autoembed":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://autoembed.cc/tv/%s/%d/%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://autoembed.cc/movie/%s", id)
	case "vidsrcxyz":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://vidsrc.xyz/embed/tv?tmdb=%s&season=%d&episode=%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://vidsrc.xyz/embed/movie?tmdb=%s", id)
	case "moviesapi":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://moviesapi.club/tv/%s-%d-%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://moviesapi.club/movie/%s", id)
	case "embedsu":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://embed.su/embed/tv/%s/%d/%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://embed.su/embed/movie/%s", id)
	case "vidplay":
		if mediaType == core.MediaTypeSeries {
			s, ep := parseSE(id)
			return fmt.Sprintf("https://vidsrc.cc/v2/embed/tv/%s/%d/%d", tmdbPart(id), s, ep)
		}
		return fmt.Sprintf("https://vidsrc.cc/v2/embed/movie/%s", id)
	default:
		return fmt.Sprintf("https://vidsrc.to/embed/movie/%s", id)
	}
}

func (p *embedProvider) GetStream(ctx context.Context, ref core.MediaRef) (*core.Stream, error) {
	embedURL := p.buildEmbedURL(ref)

	// Fast path: official data API + WASM decrypt + CDN JWT (requires Chrome)
	if p.api != nil && p.api.Available() {
		mediaType := "movie"
		season, episode := 0, 0
		tmdbID := 0
		_, _ = fmt.Sscanf(tmdbPart(ref.ProviderID), "%d", &tmdbID)
		if ref.MediaType == core.MediaTypeSeries {
			mediaType = "tv"
			season, episode = parseSE(ref.ProviderID)
		}
		if tmdbID > 0 {
			if result, err := p.api.ResolveAPI(ctx, mediaType, tmdbID, season, episode); err == nil && result != nil && result.URL != "" {
				return &core.Stream{
					URL:       result.URL,
					Referer:   result.Referer,
					UserAgent: result.UserAgent,
					Provider:  p.name,
					IsM3U8:    isM3U8(result.URL),
					Quality:   "auto",
				}, nil
			}
		}
	}

	result, err := p.resolver.Resolve(ctx, embedURL)
	if err == nil && result != nil && result.URL != "" {
		return &core.Stream{
			URL:       result.URL,
			Referer:   result.Referer,
			UserAgent: result.UserAgent,
			Subtitles: toSubtitles(result.Subtitles),
			Provider:  p.name,
			IsM3U8:    isM3U8(result.URL),
		}, nil
	}

	if p.ytdlp != nil && p.ytdlp.Available() {
		stream, yterr := p.ytdlp.ResolveStream(ctx, embedURL)
		if yterr == nil && stream != nil && stream.URL != "" {
			stream.Provider = p.name
			return stream, nil
		}
	}
	if br := resolver.NewBrowser(15 * time.Second); br.Available() {
		if result, berr := br.Resolve(ctx, embedURL); berr == nil && result != nil && result.URL != "" {
			return &core.Stream{
				URL: result.URL, Referer: result.Referer, UserAgent: result.UserAgent,
				Subtitles: toSubtitles(result.Subtitles), Provider: p.name, IsM3U8: isM3U8(result.URL),
			}, nil
		}
	}
	return nil, fmt.Errorf("provider %s: could not resolve stream for %s", p.name, embedURL)
}

func newEmbedProvider(name string, priority int, chain *resolver.Chain, yt *resolver.YTDlpStream, api *resolver.VidsrcAPI) *embedProvider {
	return &embedProvider{
		name:     name,
		priority: priority,
		resolver: chain,
		ytdlp:    yt,
		api:      api,
	}
}

func RegisterAll(registry core.ProviderRegistry) {
	registerEmbedProviders(registry)
}

func registerEmbedProviders(registry core.ProviderRegistry) {
	vidsrcChain := resolver.NewVidsrcResolver()
	httpFb := resolver.NewHTTP()
	chain := resolver.NewChain(vidsrcChain, httpFb)
	yt := resolver.NewYTDlpStream(15 * time.Second)
	api := resolver.NewVidsrcAPI(35 * time.Second)

	// Only register a few primary providers — all share the same working API path.
	// Extra names kept for compatibility / embed URL variety as fallbacks.
	// Only primary providers use the Chrome decrypt API (expensive).
	registry.Register(newEmbedProvider("vidsrc", 1, chain, yt, api))
	registry.Register(newEmbedProvider("vidsrcme", 2, chain, yt, api))
	registry.Register(newEmbedProvider("vidlink", 3, chain, yt, nil))
	registry.Register(newEmbedProvider("2embed", 4, chain, yt, nil))
	registry.Register(newEmbedProvider("vidsrcxyz", 5, chain, yt, nil))
	registry.Register(newEmbedProvider("autoembed", 6, chain, yt, nil))
	registry.Register(newEmbedProvider("moviesapi", 7, chain, yt, nil))
	registry.Register(newEmbedProvider("embedsu", 8, chain, yt, nil))
	registry.Register(newEmbedProvider("vidplay", 9, chain, yt, nil))
}

func parseSE(providerID string) (int, int) {
	s, ep := 1, 1
	parts := splitSlash(providerID)
	if len(parts) >= 3 {
		s = atoiOr(parts[1], 1)
		ep = atoiOr(parts[2], 1)
	}
	return s, ep
}

func tmdbPart(providerID string) string {
	parts := splitSlash(providerID)
	if len(parts) > 0 {
		return parts[0]
	}
	return providerID
}

func splitSlash(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == '/' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func atoiOr(s string, def int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return def
	}
	return n
}

func isM3U8(url string) bool {
	for i := 0; i < len(url)-4; i++ {
		if url[i] == '.' && url[i+1] == 'm' && url[i+2] == '3' && url[i+3] == 'u' && i+4 < len(url) && url[i+4] == '8' {
			return true
		}
	}
	return false
}

func toSubtitles(subs []resolver.SubData) []core.Subtitle {
	result := make([]core.Subtitle, 0, len(subs))
	for _, s := range subs {
		result = append(result, core.Subtitle{URL: s.URL, Lang: s.Lang})
	}
	return result
}
