package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	home, _ := os.UserHomeDir()

	if cfg.Provider != "vidsrc" {
		t.Errorf("expected Provider 'vidsrc', got %q", cfg.Provider)
	}
	if cfg.Player != "mpv" {
		t.Errorf("expected Player 'mpv', got %q", cfg.Player)
	}
	if cfg.Language != "es" {
		t.Errorf("expected Language 'es', got %q", cfg.Language)
	}
	if cfg.CacheTTL != DefaultCacheTTL {
		t.Errorf("expected CacheTTL %d, got %d", DefaultCacheTTL, cfg.CacheTTL)
	}
	if cfg.MaxResults != DefaultMaxResults {
		t.Errorf("expected MaxResults %d, got %d", DefaultMaxResults, cfg.MaxResults)
	}
	if cfg.MaxConcurrentDownloads != DefaultMaxDownloads {
		t.Errorf("expected MaxConcurrentDownloads %d, got %d", DefaultMaxDownloads, cfg.MaxConcurrentDownloads)
	}
	if cfg.HistoryMaxItems != DefaultHistoryMax {
		t.Errorf("expected HistoryMaxItems %d, got %d", DefaultHistoryMax, cfg.HistoryMaxItems)
	}
	if cfg.UpdateChannel != "stable" {
		t.Errorf("expected UpdateChannel 'stable', got %q", cfg.UpdateChannel)
	}
	if cfg.Theme != "auto" {
		t.Errorf("expected Theme 'auto', got %q", cfg.Theme)
	}
	if cfg.SubtitlesEnabled != true {
		t.Errorf("expected SubtitlesEnabled true")
	}
	if cfg.AutoCheckUpdates != true {
		t.Errorf("expected AutoCheckUpdates true")
	}
	if cfg.DefaultQuality != "best" {
		t.Errorf("expected DefaultQuality 'best', got %q", cfg.DefaultQuality)
	}

	expectedDataDir := filepath.Join(home, DefaultDataDir)
	if cfg.DataDir != expectedDataDir {
		t.Errorf("expected DataDir %q, got %q", expectedDataDir, cfg.DataDir)
	}
	expectedDownloadDir := filepath.Join(home, "Downloads", DefaultDownloadSub)
	if cfg.DownloadDir != expectedDownloadDir {
		t.Errorf("expected DownloadDir %q, got %q", expectedDownloadDir, cfg.DownloadDir)
	}
	expectedPluginDir := filepath.Join(home, DefaultConfigDir, DefaultPluginSub)
	if cfg.PluginDir != expectedPluginDir {
		t.Errorf("expected PluginDir %q, got %q", expectedPluginDir, cfg.PluginDir)
	}

	if len(cfg.MPVArgs) != 1 || cfg.MPVArgs[0] != "--hwdec=auto" {
		t.Errorf("expected MPVArgs [--hwdec=auto], got %v", cfg.MPVArgs)
	}
	if len(cfg.VLCArgs) != 0 {
		t.Errorf("expected empty VLCArgs, got %v", cfg.VLCArgs)
	}
	if cfg.Proxy != "" {
		t.Errorf("expected empty Proxy, got %q", cfg.Proxy)
	}
	if cfg.TMDBAPIKey != "" {
		t.Errorf("expected empty TMDBAPIKey, got %q", cfg.TMDBAPIKey)
	}

	if !cfg.SmartSelection.Enabled {
		t.Error("expected SmartSelection.Enabled true")
	}
	if cfg.SmartSelection.PreferredQuality != "best" {
		t.Errorf("expected SmartSelection.PreferredQuality 'best', got %q", cfg.SmartSelection.PreferredQuality)
	}
	if cfg.SmartSelection.MinBandwidth != 5000 {
		t.Errorf("expected SmartSelection.MinBandwidth 5000, got %d", cfg.SmartSelection.MinBandwidth)
	}

	if !cfg.PlayerDetection.Enabled {
		t.Error("expected PlayerDetection.Enabled true")
	}
	if cfg.PlayerDetection.AutoDetect != true {
		t.Error("expected PlayerDetection.AutoDetect true")
	}
	if len(cfg.PlayerDetection.Priority) != 3 {
		t.Errorf("expected 3 player detection priorities, got %d", len(cfg.PlayerDetection.Priority))
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Provider != "vidsrc" {
		t.Errorf("expected default provider, got %q", cfg.Provider)
	}
}

func TestLoadValidYAML(t *testing.T) {
	yamlContent := `
provider: superembed
player: vlc
quality: "1080p"
language: "es"
cache_ttl: 7200
max_results: 100
max_concurrent_downloads: 5
update_channel: beta
`
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Provider != "superembed" {
		t.Errorf("expected provider 'superembed', got %q", cfg.Provider)
	}
	if cfg.Player != "vlc" {
		t.Errorf("expected player 'vlc', got %q", cfg.Player)
	}
	if cfg.Quality != "1080p" {
		t.Errorf("expected quality '1080p', got %q", cfg.Quality)
	}
	if cfg.Language != "es" {
		t.Errorf("expected language 'es', got %q", cfg.Language)
	}
	if cfg.CacheTTL != 7200 {
		t.Errorf("expected CacheTTL 7200, got %d", cfg.CacheTTL)
	}
	if cfg.MaxResults != 100 {
		t.Errorf("expected MaxResults 100, got %d", cfg.MaxResults)
	}
	if cfg.MaxConcurrentDownloads != 5 {
		t.Errorf("expected MaxConcurrentDownloads 5, got %d", cfg.MaxConcurrentDownloads)
	}
	if cfg.UpdateChannel != "beta" {
		t.Errorf("expected UpdateChannel 'beta', got %q", cfg.UpdateChannel)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.WriteString("invalid: [[[ yaml"); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	_, err = Load(tmpFile.Name())
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(c *Config)
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid defaults",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "negative CacheTTL",
			modify: func(c *Config) {
				c.CacheTTL = -1
			},
			wantErr: true,
			errMsg:  "CacheTTL cannot be negative",
		},
		{
			name: "zero CacheTTL is valid",
			modify: func(c *Config) {
				c.CacheTTL = 0
			},
			wantErr: false,
		},
		{
			name: "negative HistoryMaxItems",
			modify: func(c *Config) {
				c.HistoryMaxItems = -1
			},
			wantErr: true,
			errMsg:  "HistoryMaxItems cannot be negative",
		},
		{
			name: "MaxResults less than 1",
			modify: func(c *Config) {
				c.MaxResults = 0
			},
			wantErr: true,
			errMsg:  "MaxResults must be at least 1",
		},
		{
			name: "MaxResults negative",
			modify: func(c *Config) {
				c.MaxResults = -5
			},
			wantErr: true,
			errMsg:  "MaxResults must be at least 1",
		},
		{
			name: "MaxConcurrentDownloads less than 1",
			modify: func(c *Config) {
				c.MaxConcurrentDownloads = 0
			},
			wantErr: true,
			errMsg:  "MaxConcurrentDownloads must be at least 1",
		},
		{
			name: "invalid UpdateChannel",
			modify: func(c *Config) {
				c.UpdateChannel = "alpha"
			},
			wantErr: true,
			errMsg:  "invalid UpdateChannel",
		},
		{
			name: "empty UpdateChannel (valid)",
			modify: func(c *Config) {
				c.UpdateChannel = ""
			},
			wantErr: false,
		},
		{
			name: "UpdateChannel nightly",
			modify: func(c *Config) {
				c.UpdateChannel = "nightly"
			},
			wantErr: false,
		},
		{
			name: "UpdateChannel beta",
			modify: func(c *Config) {
				c.UpdateChannel = "beta"
			},
			wantErr: false,
		},
		{
			name: "UpdateChannel stable",
			modify: func(c *Config) {
				c.UpdateChannel = "stable"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

func TestConfigPath(t *testing.T) {
	cfg := Default()
	path := cfg.ConfigPath()
	home, _ := os.UserHomeDir()

	if !strings.Contains(path, DefaultConfigFile) {
		t.Errorf("ConfigPath %q should contain %q", path, DefaultConfigFile)
	}
	if !strings.Contains(path, DefaultConfigDir) {
		t.Errorf("ConfigPath %q should contain %q", path, DefaultConfigDir)
	}
	if !strings.HasPrefix(path, home) {
		t.Errorf("ConfigPath %q should start with home %q", path, home)
	}
}

func TestDBPath(t *testing.T) {
	cfg := Default()
	path := cfg.DBPath()
	if !strings.HasSuffix(path, "cine-cli.db") {
		t.Errorf("DBPath %q should end with cine-cli.db", path)
	}
	if path != filepath.Join(cfg.DataDir, "cine-cli.db") {
		t.Errorf("DBPath %q should be in DataDir", path)
	}
}

func TestDownloadDirPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name    string
		setDir  string
		wantDir string
	}{
		{
			name:    "default (empty)",
			setDir:  "",
			wantDir: filepath.Join(home, "Downloads", DefaultDownloadSub),
		},
		{
			name:    "tilde expansion",
			setDir:  "~/Videos/movies",
			wantDir: filepath.Join(home, "Videos/movies"),
		},
		{
			name:    "absolute path",
			setDir:  "/data/downloads",
			wantDir: "/data/downloads",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.DownloadDir = tt.setDir
			path := cfg.DownloadDirPath()
			if path != tt.wantDir {
				t.Errorf("DownloadDirPath() = %q, want %q", path, tt.wantDir)
			}
		})
	}
}

func TestPluginDirPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name    string
		setDir  string
		wantDir string
	}{
		{
			name:    "default (empty)",
			setDir:  "",
			wantDir: filepath.Join(home, DefaultConfigDir, DefaultPluginSub),
		},
		{
			name:    "tilde expansion",
			setDir:  "~/.local/share/cine-cli/plugins",
			wantDir: filepath.Join(home, ".local/share/cine-cli/plugins"),
		},
		{
			name:    "absolute path",
			setDir:  "/opt/cine-cli/plugins",
			wantDir: "/opt/cine-cli/plugins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.PluginDir = tt.setDir
			path := cfg.PluginDirPath()
			if path != tt.wantDir {
				t.Errorf("PluginDirPath() = %q, want %q", path, tt.wantDir)
			}
		})
	}
}

func TestCacheDir(t *testing.T) {
	cfg := Default()
	dir := cfg.CacheDir()
	if !strings.HasSuffix(dir, "cache") {
		t.Errorf("CacheDir %q should end with 'cache'", dir)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("CacheDir %q should exist after call", dir)
	}
}
