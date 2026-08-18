package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir   = ".config/cine-cli"
	DefaultDataDir     = ".local/share/cine-cli"
	DefaultDownloadSub = "cine-cli"
	DefaultPluginSub   = "plugins"
	DefaultConfigFile  = "config.yaml"
	DefaultCacheTTL    = 3600
	DefaultMaxResults  = 50
	DefaultMaxDownloads = 3
	DefaultHistoryMax   = 500
)

type Config struct {
	Provider              string                   `yaml:"provider"`
	Player                string                   `yaml:"player"`
	Quality               string                   `yaml:"quality"`
	Language              string                   `yaml:"language"`
	TMDBAPIKey            string                   `yaml:"tmdb_api_key"`
	DataDir               string                   `yaml:"data_dir"`
	MPVArgs               []string                 `yaml:"mpv_args"`
	VLCArgs               []string                 `yaml:"vlc_args"`
	CacheTTL              int                      `yaml:"cache_ttl"`
	Theme                 string                   `yaml:"theme"`
	MaxResults            int                      `yaml:"max_results"`
	SubtitlesLanguage      string                   `yaml:"subtitles_language"`
	SubtitlesEnabled       bool                     `yaml:"subtitles_enabled"`
	OpenSubtitlesAPIKey    string                   `yaml:"opensubtitles_api_key"`
	DownloadDir            string                   `yaml:"download_dir"`
	MaxConcurrentDownloads int                      `yaml:"max_concurrent_downloads"`
	AutoCheckUpdates       bool                     `yaml:"auto_check_updates"`
	UpdateChannel          string                   `yaml:"update_channel"`
	PluginDir              string                   `yaml:"plugin_dir"`
	ThemeMode              string                   `yaml:"theme_mode"`
	Keybindings            map[string]string        `yaml:"keybindings"`
	SmartSelection         SmartSelectionConfig     `yaml:"smart_selection"`
	PlayerDetection        PlayerDetectionConfig    `yaml:"player_detection"`
	Proxy                  string                   `yaml:"proxy"`
	DefaultQuality         string                   `yaml:"default_quality"`
	HistoryMaxItems        int                      `yaml:"history_max_items"`

	Hooks HooksConfig `yaml:"hooks"`
}

type SmartSelectionConfig struct {
	Enabled          bool   `yaml:"enabled"`
	PreferredQuality string `yaml:"preferred_quality"`
	PreferredLang    string `yaml:"preferred_lang"`
	MinBandwidth     int    `yaml:"min_bandwidth"`
	AutoSubtitles    bool   `yaml:"auto_subtitles"`
}

type PlayerDetectionConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Priority   []string `yaml:"priority"`
	AutoDetect bool     `yaml:"auto_detect"`
}

type HooksConfig struct {
	OnPlay     string `yaml:"on_play"`
	OnExit     string `yaml:"on_exit"`
	OnDownload string `yaml:"on_download"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Provider:              "vidsrc",
		Player:                "mpv",
		Quality:               "",
		Language:              "es",
		TMDBAPIKey:            "",
		DataDir:               filepath.Join(home, DefaultDataDir),
		MPVArgs:               []string{"--hwdec=auto"},
		VLCArgs:               []string{},
		CacheTTL:              DefaultCacheTTL,
		Theme:                 "auto",
		MaxResults:            DefaultMaxResults,
		SubtitlesLanguage:     "es",
		SubtitlesEnabled:      true,
		OpenSubtitlesAPIKey:   "",
		DownloadDir:           filepath.Join(home, "Downloads", DefaultDownloadSub),
		MaxConcurrentDownloads: DefaultMaxDownloads,
		AutoCheckUpdates:      true,
		UpdateChannel:         "stable",
		PluginDir:             filepath.Join(home, DefaultConfigDir, DefaultPluginSub),
		ThemeMode:             "dark",
		Keybindings: map[string]string{
			"play":    "space",
			"quit":    "q",
			"search":  "/",
			"select":  "enter",
		},
		SmartSelection: SmartSelectionConfig{
			Enabled:          true,
			PreferredQuality: "best",
			PreferredLang:    "es",
			MinBandwidth:     5000,
			AutoSubtitles:    true,
		},
		PlayerDetection: PlayerDetectionConfig{
			Enabled:    true,
			Priority:   []string{"mpv", "vlc", "celluloid"},
			AutoDetect: true,
		},
		Proxy:           "",
		DefaultQuality:  "best",
		HistoryMaxItems: DefaultHistoryMax,

		Hooks: HooksConfig{},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func (c *Config) Save() error {
	path := c.ConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, DefaultConfigDir, DefaultConfigFile)
}

func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0755)
}

func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "cine-cli.db")
}

func (c *Config) CacheDir() string {
	dir := filepath.Join(c.DataDir, "cache")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func (c *Config) DownloadDirPath() string {
	if c.DownloadDir == "" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Downloads", DefaultDownloadSub)
	}
	if c.DownloadDir[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, c.DownloadDir[2:])
	}
	return c.DownloadDir
}

func (c *Config) PluginDirPath() string {
	if c.PluginDir == "" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, DefaultConfigDir, DefaultPluginSub)
	}
	if c.PluginDir[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, c.PluginDir[2:])
	}
	return c.PluginDir
}

func (c *Config) Validate() error {
	if c.MaxConcurrentDownloads < 1 {
		return fmt.Errorf("MaxConcurrentDownloads must be at least 1, got %d", c.MaxConcurrentDownloads)
	}
	if c.CacheTTL < 0 {
		return errors.New("CacheTTL cannot be negative")
	}
	if c.HistoryMaxItems < 0 {
		return errors.New("HistoryMaxItems cannot be negative")
	}
	if c.MaxResults < 1 {
		return fmt.Errorf("MaxResults must be at least 1, got %d", c.MaxResults)
	}
	validChannels := map[string]bool{"stable": true, "beta": true, "nightly": true}
	if c.UpdateChannel != "" && !validChannels[c.UpdateChannel] {
		return fmt.Errorf("invalid UpdateChannel: %q (must be stable, beta, or nightly)", c.UpdateChannel)
	}
	return nil
}
