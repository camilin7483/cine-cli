package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/cam/cine-cli/internal/core"
	"github.com/cam/cine-cli/internal/i18n"
	"github.com/cam/cine-cli/internal/player/detect"
	"github.com/spf13/cobra"
)

func (a *App) runQuickPlay(ctx context.Context, query string) error {
	if a.Metadata == nil {
		return fmt.Errorf("TMDB API key not configured. Set tmdb_api_key in ~/.config/cine-cli/config.yaml")
	}

	results, err := a.Metadata.Search(ctx, core.SearchFilter{Query: query})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no results found for %q", query)
	}

	selected := results[0]
	fmt.Printf("\n  %s (%d) [%s]\n", selected.Title, selected.Year, selected.MediaType)

	tmdbID := fmt.Sprintf("%d", selected.TMDBID)
	providerID := tmdbID
	isSeries := selected.MediaType == core.MediaTypeSeries
	season, episode := 0, 0

	// Prefer last progress for series
	if isSeries {
		cwList, _ := a.DB.ListContinueWatching(ctx, 20)
		found := false
		for _, cw := range cwList {
			if cw.MediaID == selected.ID && !cw.Completed {
				season, episode = cw.Season, cw.Episode
				providerID = fmt.Sprintf("%s/%d/%d", tmdbID, season, episode)
				fmt.Printf("  Resuming S%dE%d from continue-watching\n", season, episode)
				found = true
				break
			}
		}
		if !found {
			seasons, err := a.Metadata.GetSeasons(ctx, selected.TMDBID)
			if err == nil && len(seasons) > 0 {
				sn := seasons[0].SeasonNumber
				if sn == 0 && len(seasons) > 1 {
					sn = seasons[1].SeasonNumber
				}
				episodes, err := a.Metadata.GetEpisodes(ctx, selected.TMDBID, sn)
				if err == nil && len(episodes) > 0 {
					season, episode = sn, episodes[0].EpisodeNumber
					providerID = fmt.Sprintf("%s/%d/%d", tmdbID, season, episode)
					fmt.Printf("  Season %d Episode %d: %s\n", season, episode, episodes[0].Name)
				}
			}
		}
	}

	// Resume position
	var resumePosition float64
	progress, err := a.DB.GetProgress(ctx, selected.ID, season, episode)
	if err == nil && progress != nil && !progress.Completed && progress.Position > 5 {
		resumePosition = progress.Position
		fmt.Printf("  Continue from %s (%d%%)\n", formatDuration(progress.Position), int(progress.Percentage))
	}

	// Resolve stream — try preferred provider first, then the rest
	providers := a.Manager.ListProviders()
	if a.Config.Provider != "" {
		providers = append([]string{a.Config.Provider}, providers...)
	}

	var stream *core.Stream
	var usedProvider string
	var bestScore = -1

	for _, pname := range providers {
		ref := core.MediaRef{
			ProviderName: pname,
			ProviderID:   providerID,
			Title:        selected.Title,
			MediaType:    selected.MediaType,
		}
		s, err := a.Manager.ResolveStream(ctx, ref)
		if err != nil || s == nil || s.URL == "" {
			continue
		}
		score := core.QualityScore(s.Quality)
		if a.Config.SmartSelection.Enabled {
			if score > bestScore || stream == nil {
				stream = s
				usedProvider = pname
				bestScore = score
			}
			// keep looking for a better quality if smart selection is on
			if score >= core.QualityScore(a.Config.SmartSelection.PreferredQuality) &&
				a.Config.SmartSelection.PreferredQuality != "best" {
				break
			}
		} else {
			stream = s
			usedProvider = pname
			break
		}
	}

	if stream == nil || stream.URL == "" {
		fmt.Printf("  No stream available, opening in browser...\n")
		url := buildQuickBrowserURL(selected)
		// no auto browser — user can open manually
		_ = url
		return nil
	}

	fmt.Printf("  Stream resolved via %s", usedProvider)
	if stream.Quality != "" {
		fmt.Printf(" [%s]", stream.Quality)
	}
	fmt.Println()

	// Auto-detect player if configured
	playerName := a.Config.Player
	if a.Config.PlayerDetection.Enabled && a.Config.PlayerDetection.AutoDetect {
		if best := detect.Best(); best != nil {
			playerName = best.Name
		}
	}

	subs := stream.Subtitles
	if a.Config.SubtitlesEnabled && a.Subs != nil {
		if extra, err := a.Subs.Find(ctx, selected.TMDBID, selected.MediaType, season, episode); err == nil && len(extra) > 0 {
			subs = append(subs, extra...)
			fmt.Printf("  Subtitles: %d file(s)\n", len(extra))
		}
	}
	opts := core.PlayOptions{
		StreamURL:     stream.URL,
		Referer:       stream.Referer,
		UserAgent:     stream.UserAgent,
		Subtitles:     subs,
		Title:         selected.Title,
		Player:        playerName,
		PreferredLang: a.Config.SmartSelection.PreferredLang,
		SubsLang:      a.Config.SubtitlesLanguage,
	}
	if resumePosition > 0 {
		opts.ExtraArgs = append(opts.ExtraArgs, fmt.Sprintf("--start=%.1f", resumePosition))
	}

	if err := a.Player.Play(ctx, opts); err != nil {
		fmt.Printf("  Playback failed: %v\n  Opening in browser...\n", err)
		_ = exec.Command("xdg-open", stream.URL).Start()
		return nil
	}

	_ = a.DB.Add(ctx, core.HistoryEntry{
		MediaID:   selected.ID,
		Title:     selected.Title,
		MediaType: selected.MediaType,
		Provider:  usedProvider,
		StreamURL: stream.URL,
	})

	// Track progress while player is running
	a.trackProgress(ctx, selected.ID, selected.Title, selected.MediaType, season, episode, usedProvider, stream.URL)

	return nil
}

// trackProgress polls the player Position/Duration while it is running and
// periodically persists continue-watching data.
func (a *App) trackProgress(ctx context.Context, mediaID, title string, mediaType core.MediaType, season, episode int, provider, streamURL string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastPos float64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !a.Player.Running() {
				// Final save
				pos, _ := a.Player.Position()
				dur, _ := a.Player.Duration()
				posSec := pos.Seconds()
				durSec := dur.Seconds()
				if posSec < 1 && lastPos > 0 {
					posSec = lastPos
				}
				_ = a.DB.SaveProgress(ctx, mediaID, title, mediaType, season, episode, posSec, durSec, provider, streamURL)
				if durSec > 0 && posSec/durSec >= 0.92 {
					_ = a.DB.MarkCompleted(ctx, mediaID, season, episode)
				}
				return
			}
			pos, err := a.Player.Position()
			if err != nil {
				continue
			}
			dur, _ := a.Player.Duration()
			posSec := pos.Seconds()
			durSec := dur.Seconds()
			lastPos = posSec
			_ = a.DB.SaveProgress(ctx, mediaID, title, mediaType, season, episode, posSec, durSec, provider, streamURL)
		}
	}
}

func buildQuickBrowserURL(m core.Media) string {
	id := fmt.Sprintf("%d", m.TMDBID)
	if m.MediaType == core.MediaTypeSeries {
		return fmt.Sprintf("https://vidsrc.to/embed/tv/%s/1/1", id)
	}
	return fmt.Sprintf("https://vidsrc.to/embed/movie/%s", id)
}

func (a *App) runTUI(ctx context.Context) error {
	tui := NewTUI(a)
	_, err := tui.Run()
	return err
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func (a *App) doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose installation and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runDoctor(cmd.Context())
		},
	}
}

func (a *App) runDoctor(ctx context.Context) error {
	ok := true
	fmt.Println("cine-cli doctor")
	fmt.Println("===============")

	// Version
	fmt.Printf("Version:     %s\n", version)

	// Config
	cfgPath := a.Config.ConfigPath()
	fmt.Printf("Config:      %s\n", cfgPath)
	if _, err := os.Stat(cfgPath); err != nil {
		fmt.Println("  ⚠ config file not found (using defaults)")
	} else {
		fmt.Println("  ✓ config file present")
	}

	// TMDB
	if a.Config.TMDBAPIKey == "" {
		fmt.Println("TMDB key:    ✗ missing — search/metadata will fail")
		ok = false
	} else {
		fmt.Printf("TMDB key:    ✓ set (%s)\n", maskKey(a.Config.TMDBAPIKey))
		if a.Metadata != nil {
			fmt.Println("  ✓ metadata client initialized")
		}
	}

	// Players
	fmt.Println("Players:")
	players := detect.Available()
	if len(players) == 0 {
		fmt.Println("  ✗ no supported players found (mpv/vlc/...)")
		ok = false
	} else {
		for _, p := range players {
			fmt.Printf("  ✓ %s (%s)\n", p.Name, p.Binary)
		}
		if best := detect.Best(); best != nil {
			fmt.Printf("  → preferred: %s\n", best.Name)
		}
	}

	// DB
	dbPath := a.Config.DBPath()
	fmt.Printf("Database:    %s\n", dbPath)
	if a.DB != nil {
		fmt.Println("  ✓ SQLite store open")
	} else {
		fmt.Println("  ✗ database not open")
		ok = false
	}

	// Providers
	provs := a.Manager.ListProviders()
	fmt.Printf("Providers:   %d registered\n", len(provs))
	for _, p := range provs {
		fmt.Printf("  • %s\n", p)
	}
	if len(provs) == 0 {
		ok = false
	}

	// Plugins
	plugs := a.Plugins.List()
	fmt.Printf("Plugins:     %d discovered\n", len(plugs))
	for _, p := range plugs {
		status := "disabled"
		if p.Manifest.Enabled {
			status = "enabled"
		}
		loaded := "no impl"
		if p.Impl != nil {
			loaded = "loaded"
		}
		fmt.Printf("  • %s [%s, %s]\n", p.Manifest.Name, status, loaded)
	}

	// Downloads dir
	dlDir := a.Config.DownloadDirPath()
	fmt.Printf("Downloads:   %s\n", dlDir)
	if st, err := os.Stat(dlDir); err == nil && st.IsDir() {
		fmt.Println("  ✓ writable dir")
	} else {
		if err := os.MkdirAll(dlDir, 0755); err != nil {
			fmt.Println("  ✗ cannot create download dir")
			ok = false
		} else {
			fmt.Println("  ✓ created")
		}
	}

	// Language
	fmt.Printf("Language:    %s (locale %s)\n", a.Config.Language, i18n.CurrentLocale())

	// Subtitles
	if a.Config.SubtitlesEnabled {
		if a.Config.OpenSubtitlesAPIKey != "" {
			fmt.Printf("Subtitles:   ✓ enabled (OpenSubtitles key set, lang=%s)\n", a.Config.SubtitlesLanguage)
		} else {
			fmt.Printf("Subtitles:   ~ enabled but no opensubtitles_api_key (stream-embedded only)\n")
		}
	} else {
		fmt.Println("Subtitles:   disabled")
	}

	fmt.Println()
	if ok {
		fmt.Println("All critical checks passed.")
	} else {
		fmt.Println("Some issues found. Fix the ✗ items above.")
	}
	return nil
}
