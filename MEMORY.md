# cine-cli — Memory

## 2026-07-16: Professional upgrade

### What was done
Complete overhaul of cine-cli adding ~15 new features while preserving existing architecture.

### New features implemented

**Core infrastructure:**
- i18n system (6 locales: ES, EN, PT, FR, DE, IT) with embedded JSON files
- Enhanced config with subtitles, download dir, smart selection, player detection, keybindings, proxy
- Database migration system (v1→v2→v3) with continue_watching + downloads tables
- Continue Watching: tracks position/duration/percentage per media_id+season+episode

**CLI commands (16 total):**
- `cine search|watch|browse|history|trending|popular|providers|config` — all enhanced
- `cine favorites` — list, add, remove, export/import JSON, backup, restore, dedup
- `cine watchlist` — same as favorites + status management
- `cine download` — list, pause, resume, cancel, progress, cleanup
- `cine plugin` — list, enable, disable, discover, doc
- `cine update` — check, download, verify checksum, replace binary, restart
- `cine setup` — interactive wizard for language, player, provider, quality, subtitles, TMDB key, cache, downloads
- `cine completion bash|zsh|fish|powershell` — shell completion

**JSON output:**
- `--json` flag on all query commands for scripting

**Parallel search:**
- Multi-provider search with goroutines, context, timeout, dedup, relevance sorting

**Player detection:**
- Auto-detect mpv, vlc, celluloid, iina, mpc-hc, potplayer with priority config

**Smart selection:**
- Quality scoring engine, bandwidth filtering, language preferences, custom rules

**TUI enhancements:**
- Continue Watching section on search screen
- Favorites toggle (f key) with ♥ indicator
- Download from TUI (d key)
- Enhanced detail block (genres, rating, long overview)
- Updated help screen with all keybindings

**Downloads system:**
- Concurrent download manager with semaphore
- HTTP Range support for pause/resume
- Progress tracking (bytes, speed, percentage)
- File naming: `Title (Year) [Quality].mp4`

**Plugin system:**
- Go plugin support (plugin.so with Provider symbol)
- External script plugins (script.sh)
- Manifest-based discovery (manifest.json)
- Enable/disable per plugin

### Files changed/created
- `internal/i18n/` — NEW (7 files: engine + 6 locale JSONs)
- `internal/core/download.go` — NEW (Download types + interfaces)
- `internal/core/selection.go` — NEW (Smart selection config + quality scoring)
- `internal/core/media.go` — ADDED (ContinueWatching, HistoryFilter types)
- `internal/core/repository.go` — ADDED (new store interfaces)
- `internal/database/store.go` — UPDATED (v3 migration + downloads table)
- `internal/database/history.go` — UPDATED (Search, ListWithFilters, UpdatePosition, etc.)
- `internal/database/favorites.go` — UPDATED (export/import/backup/restore/dedup)
- `internal/database/watchlist.go` — UPDATED (same + status/query)
- `internal/database/continue_watching.go` — NEW
- `internal/download/` — NEW (manager.go, store.go)
- `internal/plugin/plugin.go` — NEW (registry, Go plugin, external plugin, doc)
- `internal/update/update.go` — NEW (GitHub release checker, downloader, replacer)
- `internal/search/engine.go` — NEW (parallel search, dedup, sorting, filtering)
- `internal/player/detect/detect.go` — NEW (player auto-detection)
- `internal/config/config.go` — UPDATED (all new fields + methods)
- `internal/cli/root.go` — UPDATED (all new commands + services)
- `internal/cli/search.go` — UPDATED (JSON flags, enhanced history)
- `internal/cli/play.go` — UPDATED (continue watching integration)
- `internal/cli/tui.go` — UPDATED (continue watching, favorites, download)
- `internal/cli/tui_views.go` — UPDATED (continue watching section, fav indicator)
- `internal/cli/tui_styles.go` — UPDATED (subtitle style)
- `internal/cli/favorites.go` — NEW
- `internal/cli/watchlist.go` — NEW
- `internal/cli/download.go` — NEW
- `internal/cli/plugin.go` — NEW
- `internal/cli/update.go` — NEW
- `internal/cli/setup.go` — NEW
- `internal/cli/completion.go` — NEW

### Commands
- Build: `go build -o ~/.local/bin/cine ./cmd/cine/`
- Binary: 30MB at `~/.local/bin/cine`
- Config: `~/.config/cine-cli/config.yaml`

### Architecture
Clean hexagonal:
- `internal/core/` — domain interfaces + types
- `internal/database/` — SQLite persistence
- `internal/metadata/` — TMDB provider
- `internal/provider/` — 5 stream providers + registry + resolvers
- `internal/player/` — MPV + VLC + auto-detection
- `internal/config/` — YAML config
- `internal/cache/` — two-layer cache
- `internal/i18n/` — internationalization
- `internal/download/` — concurrent download manager
- `internal/plugin/` — plugin registry
- `internal/update/` — auto-updater
- `internal/search/` — parallel search engine
- `internal/cli/` — Cobra CLI + Bubble Tea TUI (all commands)

### Not yet implemented
- Fuzzy search (needs external lib or custom algo)
- Statistics dashboard (hours watched, genre stats, etc.)
- More providers (PrimeWire, LookMovie, etc.)
- Subtitle providers (OpenSubtitles, SubDL)
- Offline mode
- Bookmarks
- Tests, CI/CD, GoReleaser, linting pipeline

## 2026-08-18: Sprint 1 — Fix base (compile + infra)

### Fixes applied
1. **Syntax error** in `internal/download/manager.go:240` — `return _ = m.store.Update(...)` replaced with proper `_ = m.store.Update(...); return nil`. Project now builds and passes `go vet`.
2. **Makefile** — targets `build`/`install` now point to `./cmd/cine` (was `./cmd/cine-cli`). Binary name set to `cine`.
3. **CI** (`.github/workflows/ci.yml`) — `setup-go` updated to Go 1.26.4 (was 1.23). Lint job also pins the same version.
4. **Goreleaser** — fixed ldflags to `-X github.com/cam/cine-cli/internal/cli.version={{.Version}}`, set `main: ./cmd/cine`, added multi-OS (linux/darwin/windows).
5. **Dead code removed** — empty `internal/tui/` and unused `pkg/types/` deleted.
6. **README** — Go requirement updated to 1.26.4+.

### Result
- `go build ./cmd/cine` succeeds
- `go test ./...` passes (existing tests)
- `go vet ./...` clean

Next: Sprint 2 (i18n completion, real Position()/continue-watching, subtitles).

## 2026-08-18: Sprint 2 — Core functional improvements

### Made functional
1. **Continue-watching real** — MPV now uses IPC (`--input-ipc-server`). `Position()` and `Duration()` query live properties. `play.go` tracks progress every 5s while playing and marks completed at ≥92%.
2. **Smart selection active** — when resolving streams, providers are scored with `QualityScore`; best quality is preferred if `SmartSelection.Enabled`.
3. **Player auto-detection** — if `PlayerDetection.AutoDetect` is on, uses `detect.Best()` instead of hard-coded config player.
4. **Series resume** — looks up continue-watching for the media and resumes the correct SxE instead of always forcing 1/1.
5. **Config.Validate()** is now called on every Load (invalid configs fail fast).
6. **Default language Spanish** — `Language=es`, `SubtitlesLanguage=es`, `PreferredLang=es`.
7. **i18n TUI** — empty states (favorites, watchlist, history), help title, browser view, TMDB key prompt now use `i18n.T()`.
8. **Subtitles from stream** — MPV receives `--sub-file=` for each resolved subtitle.

### Still pending (later sprints)
- Full i18n of every TUI string and CLI error messages
- VLC Position via RC interface
- Plugins real registration beyond type-assert
- Subtitles download from OpenSubtitles
- Offline mode / bookmarks
- Home screen, statusline, interactive source picker

## 2026-08-18: Sprint 3 — Plugins, config, doctor, updates

### Functional additions
1. **Plugins really load** — `Discover` now loads `.so` via `LoadGoPlugin` or `script.sh` as `ExternalProvider`. Registration accepts any `core.Provider`, not only the type-assert that never matched.
2. **`cine config set/get/path`** — real get/set for provider, player, language, tmdb key, quality, download_dir, max_dl, cache_ttl, subtitles, proxy, preferred_lang/quality. Validates before save.
3. **Update semver** — version comparison is now proper major.minor.patch (not string equality).
4. **`cine doctor`** — diagnoses TMDB key, players, DB, providers, plugins, download dir, language.
5. Version bumped to **0.3.0**.

### Still open
- Interactive source/quality picker in TUI
- Full remaining i18n of help tables and CLI errors
- VLC Position via RC
- OpenSubtitles download
- Offline mode / bookmarks
- Home screen + statusline

## 2026-08-18: Sprint 4 — Source picker + statusline

### Functional
1. **Interactive source/quality selector** — resolve collects ALL working streams from providers (deduped by URL), scores by quality, shows list. One source → auto-play; multiple → pick with ↑↓ + enter.
2. **Statusline** — bottom bar with playing state, last status/error, language, and hotkey hints on every screen.
3. **Progress tracking from TUI** — uses the same `trackProgress` as CLI after play starts.
4. **Better quit** — `q` quits (except while typing search); esc from playing stops the player first.
5. i18n for resolving/sources/playing strings (es + en).
6. Version **0.4.0**.

## 2026-08-18: Sprint 5 — VLC RC, OpenSubtitles, home, help i18n

### Functional
1. **VLC Position/Pause/Resume** via RC interface (`--extraintf rc --rc-host`). `PlayerSwitch` routes to active player.
2. **OpenSubtitles client** (`internal/subtitles`) — searches by TMDB id + language, downloads `.srt` to cache, attaches to MPV/VLC. Needs `opensubtitles_api_key` in config (`cine config set opensubtitles_api_key KEY`).
3. **Home screen** — when search is empty shows Continue Watching + Trending preview + localized hints.
4. **Help fully i18n** — all help table sections and key descriptions use `i18n.T`.
5. **Doctor** reports subtitle status.
6. Version **0.5.0**.

## 2026-08-18: Sprint 6 — Fix hang + scroll + offline/bookmarks

### Critical fixes
1. **Resolve no longer hangs** — parallel provider resolution with 10s/provider and 22s overall timeout. Always returns (sources or browser fallback).
2. **Episodes scroll** — `visibleCount()` based on terminal height so title/header stay visible with many episodes.
3. **Vidsrc chain** updated for modern `/embed/?vs=` + `playerUrl` layout; shorter HTTP timeouts.
4. **Browser fallback** in embed GetStream when Chrome is available.
5. **More providers**: vidsrcxyz, moviesapi, embedsu, vidplay (+ existing).

### New
- `cine offline list|play` — local downloads
- `cine bookmarks list|add|delete`
- Dockerfile multi-stage
- Tests: QualityScore, parseSE

Version **0.6.0**.

## 2026-08-18: Sprint 6.1 — No auto-browser, MPV headers, theme, lang

1. **MPV**: `--no-ytdl` (evita CF 403 del hook), Referer+Origin headers, UA por defecto, soft `--alang` multi-code.
2. **No auto xdg-open** on resolve/play failure — mensaje + `b` manual.
3. **Theme** GitHub dark/light (azul/neutro, sin morado saturado).
4. **Idioma**: al ciclar muestra "si el stream no lo trae, MPV usa el disponible".

## 2026-08-18: v1.0.0 release

### Working stream pipeline (tested)
1. Chrome headless + vsdec.js decrypts `data.vidsrcme.ru` stream_urls
2. CDN `/generate.php` JWT appended as `?token=`
3. Preflight returns 200 #EXTM3U for Inception + Breaking Bad S01E01
4. Young Sheldon S01E01 → upstream 404 (not in catalog)

### UX
- No auto-browser, no window flash on dead streams
- Episode list pinned header
- README complete (ani-cli style)
