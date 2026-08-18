package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cam/cine-cli/internal/cache"
	"github.com/cam/cine-cli/internal/config"
	"github.com/cam/cine-cli/internal/core"
	"github.com/cam/cine-cli/internal/database"
	"github.com/cam/cine-cli/internal/download"
	"github.com/cam/cine-cli/internal/i18n"
	"github.com/cam/cine-cli/internal/metadata/tmdb"
	"github.com/cam/cine-cli/internal/player"
	"github.com/cam/cine-cli/internal/plugin"
	"github.com/cam/cine-cli/internal/provider"
	"github.com/cam/cine-cli/internal/provider/scraper"
	"github.com/cam/cine-cli/internal/subtitles"
	"github.com/cam/cine-cli/internal/update"
	"github.com/spf13/cobra"
)

type App struct {
	Config    *config.Config
	DB        *database.Store
	Metadata  *tmdb.Provider
	Registry  *provider.Registry
	Manager   *provider.Manager
	Player    *player.PlayerSwitch
	Cache     *cache.TwoLayerCache
	Downloads *download.Manager
	Plugins   *plugin.Registry
	Updater   *update.Checker
	Subs      *subtitles.Client
	jsonOut   bool
}

var version = "1.0.0"

func NewApp() (*App, error) {
	cfg := config.Default()
	cfgPath := cfg.ConfigPath()

	loadedCfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", err)
	} else {
		cfg = loadedCfg
	}

	var loc i18n.Locale
	switch cfg.Language {
	case "en-US", "en":
		loc = i18n.LocaleEN
	case "es-ES", "es":
		loc = i18n.LocaleES
	case "pt":
		loc = i18n.LocalePT
	case "fr":
		loc = i18n.LocaleFR
	case "de":
		loc = i18n.LocaleDE
	case "it":
		loc = i18n.LocaleIT
	default:
		loc = i18n.LocaleEN
	}
	if err := i18n.SetLocale(loc); err != nil {
		fmt.Fprintf(os.Stderr, "warning: i18n: %v\n", err)
	}

	if err := cfg.EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("data dir: %w", err)
	}

	switch {
	case strings.HasPrefix(cfg.Language, "es"):
		_ = i18n.SetLocale(i18n.LocaleES)
	case strings.HasPrefix(cfg.Language, "pt"):
		_ = i18n.SetLocale(i18n.LocalePT)
	case strings.HasPrefix(cfg.Language, "fr"):
		_ = i18n.SetLocale(i18n.LocaleFR)
	case strings.HasPrefix(cfg.Language, "de"):
		_ = i18n.SetLocale(i18n.LocaleDE)
	case strings.HasPrefix(cfg.Language, "it"):
		_ = i18n.SetLocale(i18n.LocaleIT)
	default:
		_ = i18n.SetLocale(i18n.LocaleEN)
	}

	db, err := database.New(cfg.DBPath())
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	var metadata *tmdb.Provider
	if cfg.TMDBAPIKey != "" {
		var err error
		metadata, err = tmdb.New(cfg.TMDBAPIKey, cfg.Language)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: tmdb init: %v\n", err)
		}
	}

	reg := provider.NewRegistry()
	scraper.RegisterAll(reg)
	mgr := provider.NewManager(reg)

	players := player.NewPlayerSwitch(cfg.MPVArgs, cfg.VLCArgs)

	memCache := cache.NewMemoryCache(time.Duration(cfg.CacheTTL) * time.Second)
	diskCache := cache.NewTwoLayerCache(memCache, &cacheDB{db: db}, time.Duration(cfg.CacheTTL)*time.Second)

	dlStore := download.NewStore(db.DB())
	dlMgr := download.NewManager(cfg.DownloadDirPath(), cfg.MaxConcurrentDownloads, dlStore)

	pluginReg := plugin.NewRegistry(cfg.PluginDirPath())
	_ = pluginReg.Discover(context.Background())

	updater := update.New(version)

	subsCache := filepath.Join(cfg.DataDir, "subtitles")
	subsClient := subtitles.New(cfg.OpenSubtitlesAPIKey, cfg.SubtitlesLanguage, subsCache)

	app := &App{
		Config:    cfg,
		DB:        db,
		Metadata:  metadata,
		Registry:  reg,
		Manager:   mgr,
		Player:    players,
		Cache:     diskCache,
		Downloads: dlMgr,
		Plugins:   pluginReg,
		Updater:   updater,
		Subs:      subsClient,
	}

	for _, p := range pluginReg.ListByType(plugin.PluginTypeProvider) {
		if p.Impl == nil || !p.Manifest.Enabled {
			continue
		}
		if prov, ok := p.Impl.(core.Provider); ok {
			reg.Register(prov)
		}
	}

	return app, nil
}

type cacheDB struct {
	db *database.Store
}

func (c *cacheDB) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return c.db.CacheGet(ctx, key)
}

func (c *cacheDB) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.db.CacheSet(ctx, key, value, ttl)
}

func (a *App) Close() error {
	_ = a.Downloads.Cleanup(context.Background())
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

func (a *App) printJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func (a *App) RootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "cine",
		Short:   "cine-cli — Watch movies and TV shows from your terminal",
		Long:    `A modern CLI tool for discovering and playing movies and TV series.`,
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				a.jsonOut = true
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return a.runQuickPlay(cmd.Context(), args[0])
			}
			return a.runTUI(cmd.Context())
		},
	}

	root.PersistentFlags().Bool("json", false, "Output in JSON format")

	root.AddCommand(a.searchCmd())
	root.AddCommand(a.watchCmd())
	root.AddCommand(a.browseCmd())
	root.AddCommand(a.historyCmd())
	root.AddCommand(a.trendingCmd())
	root.AddCommand(a.popularCmd())
	root.AddCommand(a.recommendationsCmd())
	root.AddCommand(a.providersCmd())
	root.AddCommand(a.configCmd())
	root.AddCommand(a.favoritesCmd())
	root.AddCommand(a.watchlistCmd())
	root.AddCommand(a.downloadCmd())
	root.AddCommand(a.pluginCmd())
	root.AddCommand(a.updateCmd())
	root.AddCommand(a.setupCmd())
	root.AddCommand(a.statsCmd())
	root.AddCommand(a.completionCmd())
	root.AddCommand(a.doctorCmd())
	root.AddCommand(a.offlineCmd())
	root.AddCommand(a.bookmarksCmd())

	return root
}

func (a *App) Shutdown() {
	_ = a.Player.Stop()
	if a.Downloads != nil {
		_ = a.Downloads.Cleanup(context.Background())
	}
	a.Close()
	os.Exit(0)
}

func Run() {
	app, err := NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()

	root := app.RootCmd()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		app.Shutdown()
	}()

	cobra.AddTemplateFunc("trimString", strings.TrimSpace)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
