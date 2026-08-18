package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cam/cine-cli/internal/player/detect"
	"github.com/spf13/cobra"
)

func (a *App) setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive configuration wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSetup()
		},
	}
}

func (a *App) runSetup() error {
	reader := bufio.NewReader(os.Stdin)
	cfg := a.Config

	fmt.Println("╔════════════════════════════════════╗")
	fmt.Println("║        cine-cli Setup Wizard       ║")
	fmt.Println("╚════════════════════════════════════╝")
	fmt.Println()

	ask := func(prompt, defaultVal string) string {
		if defaultVal != "" {
			fmt.Printf("%s [%s]: ", prompt, defaultVal)
		} else {
			fmt.Printf("%s: ", prompt)
		}
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			return defaultVal
		}
		return text
	}

	askYN := func(prompt string, defaultVal bool) bool {
		def := "y"
		if !defaultVal {
			def = "n"
		}
		val := ask(prompt+" (y/n)", def)
		return strings.ToLower(val) == "y" || val == def
	}

	langs := []string{"en-US", "es-ES", "pt-BR", "fr-FR", "de-DE", "it-IT"}
	fmt.Println("Available languages:")
	for i, l := range langs {
		fmt.Printf("  %d. %s\n", i+1, l)
	}
	langIdx, _ := strconv.Atoi(ask("Select language", "1"))
	if langIdx > 0 && langIdx <= len(langs) {
		cfg.Language = langs[langIdx-1]
	}

	players := detect.Available()
	if len(players) > 0 {
		fmt.Println("\nDetected players:")
		for i, p := range players {
			fmt.Printf("  %d. %s\n", i+1, p.Name)
		}
		pIdx, _ := strconv.Atoi(ask("Select default player", "1"))
		if pIdx > 0 && pIdx <= len(players) {
			cfg.Player = players[pIdx-1].Name
		}
	} else {
		current := cfg.Player
		if current == "" {
			current = "mpv"
		}
		cfg.Player = ask("Player (mpv/vlc)", current)
	}

	providers := a.Manager.ListProviders()
	if len(providers) > 0 {
		fmt.Println("\nAvailable providers:")
		for i, p := range providers {
			fmt.Printf("  %d. %s\n", i+1, p)
		}
		pIdx, _ := strconv.Atoi(ask("Select default provider", "1"))
		if pIdx > 0 && pIdx <= len(providers) {
			cfg.Provider = providers[pIdx-1]
		}
	}

	qualities := []string{"best", "1080p", "720p", "480p"}
	fmt.Println("\nDefault quality:")
	for i, q := range qualities {
		fmt.Printf("  %d. %s\n", i+1, q)
	}
	qIdx, _ := strconv.Atoi(ask("Select quality", "1"))
	if qIdx > 0 && qIdx <= len(qualities) {
		cfg.DefaultQuality = qualities[qIdx-1]
	}

	cfg.SubtitlesEnabled = askYN("Enable subtitles by default", true)
	if cfg.SubtitlesEnabled {
		cfg.SubtitlesLanguage = ask("Default subtitle language", "en")
	}

	if cfg.TMDBAPIKey == "" {
		key := ask("TMDB API Key (get from https://themoviedb.org)", "")
		if key != "" {
			cfg.TMDBAPIKey = key
		}
	}

	maxDL := ask("Max concurrent downloads", fmt.Sprintf("%d", cfg.MaxConcurrentDownloads))
	if n, err := strconv.Atoi(maxDL); err == nil && n > 0 {
		cfg.MaxConcurrentDownloads = n
	}

	ttl := ask("Cache TTL (seconds)", fmt.Sprintf("%d", cfg.CacheTTL))
	if n, err := strconv.Atoi(ttl); err == nil && n > 0 {
		cfg.CacheTTL = n
	}

	dlDir := ask("Download directory", cfg.DownloadDir)
	if dlDir != "" {
		cfg.DownloadDir = dlDir
	}

	cfg.AutoCheckUpdates = askYN("Auto-check for updates", true)

	fmt.Println("\nSaving configuration...")
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println("Configuration saved to", cfg.ConfigPath())
	fmt.Println("\nYou can change these settings anytime with:")
	fmt.Println("  cine config")
	fmt.Println("  cine setup")

	return nil
}
