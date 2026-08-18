package resolver

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

type YTDlp struct {
	timeout time.Duration
}

func NewYTDlp(timeout time.Duration) *YTDlp {
	return &YTDlp{timeout: timeout}
}

func (y *YTDlp) Name() string { return "yt-dlp" }
func (y *YTDlp) Available() bool {
	_, err := exec.LookPath("yt-dlp")
	return err == nil
}

func (y *YTDlp) Resolve(ctx context.Context, embedURL string) (*StreamResult, error) {
	ctx, cancel := context.WithTimeout(ctx, y.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--no-download",
		"--get-url",
		"--no-playlist",
		"--ignore-no-formats",
		"--format", "best",
		"--user-agent", httpUA,
		embedURL,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp: %w", err)
	}

	url := strings.TrimSpace(string(out))
	if url == "" {
		return nil, fmt.Errorf("yt-dlp: empty URL")
	}

	return &StreamResult{
		URL:       url,
		Referer:   embedURL,
		UserAgent: httpUA,
		Provider:  "yt-dlp",
	}, nil
}

type YTDlpStream struct {
	timeout time.Duration
}

func NewYTDlpStream(timeout time.Duration) *YTDlpStream {
	return &YTDlpStream{timeout: timeout}
}

func (y *YTDlpStream) Name() string { return "yt-dlp-stream" }
func (y *YTDlpStream) Available() bool {
	_, err := exec.LookPath("yt-dlp")
	return err == nil
}

func (y *YTDlpStream) ResolveStream(ctx context.Context, url string) (*core.Stream, error) {
	ctx, cancel := context.WithTimeout(ctx, y.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--no-download",
		"--get-url",
		"--no-playlist",
		"--ignore-no-formats",
		"--format", "best",
		"--user-agent", httpUA,
		url,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp: %w", err)
	}

	streamURL := strings.TrimSpace(string(out))
	if streamURL == "" {
		return nil, fmt.Errorf("yt-dlp: empty URL")
	}

	return &core.Stream{
		URL:       streamURL,
		Referer:   url,
		UserAgent: httpUA,
		IsM3U8:    strings.Contains(streamURL, ".m3u8"),
		Provider:  "yt-dlp",
	}, nil
}
